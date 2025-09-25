package repository

import (
	"context"
	cryptorand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"be-v2/internal/domain"
	"be-v2/pkg/database"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type VoteRepository struct {
	db  *database.PostgresDB
	log *zap.Logger
}

func NewVoteRepository(db *database.PostgresDB) *VoteRepository {
	return &VoteRepository{db: db, log: zap.NewNop()}
}

// WithLogger sets a logger for this repository
func (r *VoteRepository) WithLogger(log *zap.Logger) *VoteRepository {
	r.log = log
	return r
}

// CreateVote creates a new vote record with PDPA compliance
func (r *VoteRepository) CreateVote(ctx context.Context, vote *domain.Vote) error {
	query := `
		INSERT INTO votes (
			vote_id, user_id, team_id, voter_name, voter_email, voter_phone, 
			favorite_video, ip_address, user_agent, consent_timestamp, consent_ip, 
			privacy_policy_version, pdpa_consent, marketing_consent, data_retention_until
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at
	`

	start := time.Now()
	err := r.db.Pool.QueryRow(ctx, query,
		vote.VoteID,
		vote.UserID,
		vote.TeamID,
		vote.VoterName,
		vote.VoterEmail,
		vote.VoterPhone,
		vote.FavoriteVideo,
		vote.IPAddress,
		vote.UserAgent,
		vote.ConsentTimestamp,
		vote.ConsentIP,
		vote.PrivacyPolicyVersion,
		vote.ConsentPDPA,
		vote.MarketingConsent,
		vote.DataRetentionUntil,
	).Scan(&vote.ID, &vote.CreatedAt)
	dur := time.Since(start)

	if err != nil {
		r.log.Info("db_insert_votes", zap.Duration("duration", dur), zap.Error(err))
		return fmt.Errorf("failed to create vote: %w", err)
	}
	r.log.Debug("db_insert_votes", zap.Duration("duration", dur))

	// Note: Materialized view refresh moved to a periodic background task

	return nil
}

// GetVoteByUserID gets a vote by user ID
func (r *VoteRepository) GetVoteByUserID(ctx context.Context, userID string) (*domain.Vote, error) {
	var vote domain.Vote
	var voteID sql.NullString // Handle nullable vote_id
	var teamID sql.NullInt32
	var voterName sql.NullString
	var voterEmail sql.NullString
	var voterPhone sql.NullString
	var favoriteVideo sql.NullString
	var ipAddress sql.NullString
	var userAgent sql.NullString
	var consentTimestamp sql.NullTime
	var consentIP sql.NullString
	var privacyPolicyVersion sql.NullString
	var dataRetentionUntil sql.NullTime
	var welcomeAcceptedAt sql.NullTime
	var rulesVersion sql.NullString

	query := `
		SELECT id, vote_id, user_id, team_id, voter_name, voter_email, voter_phone, 
		       favorite_video, ip_address, user_agent, consent_timestamp, consent_ip,
		       privacy_policy_version, pdpa_consent, marketing_consent, 
		       data_retention_until, created_at,
		       welcome_accepted, welcome_accepted_at, rules_version
		FROM votes
		WHERE user_id = $1
	`

	start := time.Now()
	err := r.db.GetReadPool().QueryRow(ctx, query, userID).Scan(
		&vote.ID,
		&voteID, // Use nullable version
		&vote.UserID,
		&teamID,
		&voterName,
		&voterEmail,
		&voterPhone,
		&favoriteVideo,
		&ipAddress,
		&userAgent,
		&consentTimestamp,
		&consentIP,
		&privacyPolicyVersion,
		&vote.ConsentPDPA,
		&vote.MarketingConsent,
		&dataRetentionUntil,
		&vote.CreatedAt,
		&vote.WelcomeAccepted,
		&welcomeAcceptedAt,
		&rulesVersion,
	)
	dur := time.Since(start)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.log.Info("db_get_vote_by_user_id", zap.Duration("duration", dur), zap.Error(err))
		return nil, fmt.Errorf("failed to get vote: %w", err)
	}
	r.log.Debug("db_get_vote_by_user_id", zap.Duration("duration", dur))

	// Handle nullable fields
	if voteID.Valid {
		vote.VoteID = voteID.String
	}
	if teamID.Valid {
		vote.TeamID = int(teamID.Int32)
		vote.CandidateID = int(teamID.Int32)
	}
	if voterName.Valid {
		vote.VoterName = voterName.String
		// Parse full name
		names := strings.Fields(voterName.String)
		if len(names) >= 2 {
			vote.FirstName = names[0]
			vote.LastName = strings.Join(names[1:], " ")
		} else if len(names) == 1 {
			vote.FirstName = names[0]
		}
	}
	if voterEmail.Valid {
		vote.VoterEmail = voterEmail.String
		vote.Email = voterEmail.String
	}
	if voterPhone.Valid {
		vote.VoterPhone = voterPhone.String
		vote.Phone = voterPhone.String
	}
	if favoriteVideo.Valid {
		vote.FavoriteVideo = favoriteVideo.String
	}
	if ipAddress.Valid {
		vote.IPAddress = ipAddress.String
	}
	if userAgent.Valid {
		vote.UserAgent = userAgent.String
	}
	if consentTimestamp.Valid {
		vote.ConsentTimestamp = &consentTimestamp.Time
	}
	if consentIP.Valid {
		vote.ConsentIP = consentIP.String
	}
	if privacyPolicyVersion.Valid {
		vote.PrivacyPolicyVersion = privacyPolicyVersion.String
	}
	if dataRetentionUntil.Valid {
		vote.DataRetentionUntil = &dataRetentionUntil.Time
	}
	if welcomeAcceptedAt.Valid {
		vote.WelcomeAcceptedAt = &welcomeAcceptedAt.Time
	}
	if rulesVersion.Valid {
		vote.RulesVersion = rulesVersion.String
	}

	return &vote, nil
}

// GetVoteByVoteID gets a vote by vote ID
func (r *VoteRepository) GetVoteByVoteID(ctx context.Context, voteID string) (*domain.Vote, error) {
	var vote domain.Vote
	var teamID sql.NullInt32
	query := `
		SELECT id, vote_id, user_id, team_id, voter_name, voter_email, voter_phone, 
		       favorite_video, ip_address, user_agent, consent_timestamp, consent_ip,
		       privacy_policy_version, pdpa_consent, marketing_consent, 
		       data_retention_until, created_at
		FROM votes
		WHERE vote_id = $1
	`

	start := time.Now()
	err := r.db.GetReadPool().QueryRow(ctx, query, voteID).Scan(
		&vote.ID,
		&vote.VoteID,
		&vote.UserID,
		&teamID,
		&vote.VoterName,
		&vote.VoterEmail,
		&vote.VoterPhone,
		&vote.FavoriteVideo,
		&vote.IPAddress,
		&vote.UserAgent,
		&vote.ConsentTimestamp,
		&vote.ConsentIP,
		&vote.PrivacyPolicyVersion,
		&vote.ConsentPDPA,
		&vote.MarketingConsent,
		&vote.DataRetentionUntil,
		&vote.CreatedAt,
	)
	dur := time.Since(start)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.log.Info("db_get_vote_by_vote_id", zap.Duration("duration", dur), zap.Error(err))
		return nil, fmt.Errorf("failed to get vote by ID: %w", err)
	}
	r.log.Debug("db_get_vote_by_vote_id", zap.Duration("duration", dur))

	// Handle nullable team_id
	if teamID.Valid {
		vote.TeamID = int(teamID.Int32)
		vote.CandidateID = int(teamID.Int32)
	}

	return &vote, nil
}

// GetVoteByPhone gets a vote by phone number
func (r *VoteRepository) GetVoteByPhone(ctx context.Context, phone string) (*domain.Vote, error) {
	var vote domain.Vote
	var voteID sql.NullString // Handle nullable vote_id
	var teamID sql.NullInt32
	query := `
		SELECT id, vote_id, user_id, team_id, voter_name, voter_email, voter_phone, 
		       favorite_video, ip_address, user_agent, consent_timestamp, consent_ip,
		       privacy_policy_version, pdpa_consent, marketing_consent, 
		       data_retention_until, created_at
		FROM votes
		WHERE voter_phone = $1
	`

	start := time.Now()
	err := r.db.GetReadPool().QueryRow(ctx, query, phone).Scan(
		&vote.ID,
		&voteID, // Use nullable version
		&vote.UserID,
		&teamID,
		&vote.VoterName,
		&vote.VoterEmail,
		&vote.VoterPhone,
		&vote.FavoriteVideo,
		&vote.IPAddress,
		&vote.UserAgent,
		&vote.ConsentTimestamp,
		&vote.ConsentIP,
		&vote.PrivacyPolicyVersion,
		&vote.ConsentPDPA,
		&vote.MarketingConsent,
		&vote.DataRetentionUntil,
		&vote.CreatedAt,
	)
	dur := time.Since(start)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.log.Info("db_get_vote_by_phone", zap.Duration("duration", dur), zap.Error(err))
		return nil, fmt.Errorf("failed to get vote by phone: %w", err)
	}
	r.log.Debug("db_get_vote_by_phone", zap.Duration("duration", dur))

	// Handle nullable fields
	if voteID.Valid {
		vote.VoteID = voteID.String
	}
	if teamID.Valid {
		vote.TeamID = int(teamID.Int32)
		vote.CandidateID = int(teamID.Int32)
	}

	return &vote, nil
}

// GetTeamsWithVoteCounts gets all teams with their vote counts
func (r *VoteRepository) GetTeamsWithVoteCounts(ctx context.Context) ([]domain.Team, error) {
	query := `
		SELECT id, code, name, description, icon, image_filename, member_count, vote_count, last_vote_at
		FROM vote_count_summary
		ORDER BY vote_count DESC, name ASC
	`

	start := time.Now()
	rows, err := r.db.GetReadPool().Query(ctx, query)
	dur := time.Since(start)

	if err != nil {
		r.log.Info("db_get_teams_with_vote_counts", zap.Duration("duration", dur), zap.Error(err))
		return nil, fmt.Errorf("failed to get teams with vote counts: %w", err)
	}
	r.log.Debug("db_get_teams_with_vote_counts", zap.Duration("duration", dur))
	defer rows.Close()

	var teams []domain.Team
	for rows.Next() {
		var team domain.Team
		var imageFilename sql.NullString
		err := rows.Scan(
			&team.ID,
			&team.Code,
			&team.Name,
			&team.Description,
			&team.Icon,
			&imageFilename,
			&team.MemberCount,
			&team.VoteCount,
			&team.LastVoteAt,
		)
		if imageFilename.Valid {
			team.ImageFilename = imageFilename.String
		}
		if err != nil {
			r.log.Info("scan_team", zap.Error(err))
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}
		team.IsActive = true
		teams = append(teams, team)
	}

	return teams, nil
}

// GetTeamByID gets a team by ID
func (r *VoteRepository) GetTeamByID(ctx context.Context, teamID int) (*domain.Team, error) {
	var team domain.Team
	query := `
		SELECT id, code, name, description, icon, member_count, is_active, created_at, updated_at
		FROM teams
		WHERE id = $1 AND is_active = true
	`

	start := time.Now()
	err := r.db.GetReadPool().QueryRow(ctx, query, teamID).Scan(
		&team.ID,
		&team.Code,
		&team.Name,
		&team.Description,
		&team.Icon,
		&team.MemberCount,
		&team.IsActive,
		&team.CreatedAt,
		&team.UpdatedAt,
	)
	dur := time.Since(start)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.log.Info("db_get_team_by_id", zap.Duration("duration", dur), zap.Error(err))
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	r.log.Debug("db_get_team_by_id", zap.Duration("duration", dur))

	return &team, nil
}

// GetTotalVoteCount gets the total number of votes
func (r *VoteRepository) GetTotalVoteCount(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM votes`

	start := time.Now()
	err := r.db.GetReadPool().QueryRow(ctx, query).Scan(&count)
	dur := time.Since(start)

	if err != nil {
		r.log.Info("db_get_total_vote_count", zap.Duration("duration", dur), zap.Error(err))
		return 0, fmt.Errorf("failed to get total vote count: %w", err)
	}
	r.log.Debug("db_get_total_vote_count", zap.Duration("duration", dur))

	return count, nil
}

// UpsertPersonalInfo creates or updates personal information for the authenticated user
// This method handles personal info storage for users who may have already accepted welcome (have existing record)
// It ensures phone number uniqueness while allowing the current user to update their own information
func (r *VoteRepository) UpsertPersonalInfo(ctx context.Context, userID string, req *domain.PersonalInfoRequest, normalizedPhone, ipAddress, userAgent string) (*domain.PersonalInfoResponse, error) {
	consentTime := time.Now()
	retentionTime := time.Now().AddDate(1, 0, 0) // 1 year from now
	fullName := fmt.Sprintf("%s %s", req.FirstName, req.LastName)

	// First, check if the current user already has a record
	existingUserRecord, err := r.GetVoteByUserID(ctx, userID)
	if err != nil {
		r.log.Info("db_upsert_personal_info_check_existing", zap.Error(err))
		return nil, fmt.Errorf("failed to check existing user record: %w", err)
	}

	// Check if phone number is already used by another user (not the current user)
	existingPhoneUser, err := r.GetUserByPhone(ctx, normalizedPhone)
	if err != nil {
		r.log.Info("db_upsert_personal_info_check_phone_uniqueness", zap.Error(err))
		return nil, fmt.Errorf("failed to check phone uniqueness: %w", err)
	}

	// If phone is used by another user (different userID), reject the request
	if existingPhoneUser != nil && existingPhoneUser.UserID != userID {
		r.log.Info("db_upsert_personal_info_phone_already_used", zap.String("user_id", userID), zap.String("normalized_phone", normalizedPhone))
		return nil, fmt.Errorf("phone number already registered by another user")
	}

	var response domain.PersonalInfoResponse

	if existingUserRecord != nil {
		// User exists (from welcome acceptance or previous submission) - update their record
		updateQuery := `
			UPDATE votes 
			SET voter_phone = $2, voter_name = $3, voter_email = $4, favorite_video = $5, 
			    ip_address = $6, user_agent = $7, consent_timestamp = $8, consent_ip = $9,
			    pdpa_consent = $10, data_retention_until = $11
			WHERE user_id = $1
			RETURNING user_id, voter_phone, voter_name, voter_email, favorite_video, created_at, NOW()
		`

		start := time.Now()
		err = r.db.Pool.QueryRow(ctx, updateQuery,
			userID,
			normalizedPhone,
			fullName,
			req.Email,
			req.FavoriteVideo,
			ipAddress,
			userAgent,
			&consentTime,
			ipAddress,
			req.ConsentPDPA,
			&retentionTime,
		).Scan(
			&response.UserID,
			&response.Phone,
			&fullName,
			&response.Email,
			&response.FavoriteVideo,
			&response.CreatedAt,
			&response.UpdatedAt,
		)
		dur := time.Since(start)

		if err != nil {
			r.log.Info("db_upsert_personal_info_update_existing", zap.Duration("duration", dur), zap.Error(err))
			return nil, fmt.Errorf("failed to update existing user: %w", err)
		}
		r.log.Debug("db_upsert_personal_info_update_existing", zap.Duration("duration", dur))
	} else {
		// User doesn't exist - create new record WITHOUT vote_id (vote_id should only be created when actually voting)
		insertQuery := `
			INSERT INTO votes (
				user_id, voter_phone, voter_name, voter_email, favorite_video,
				ip_address, user_agent, consent_timestamp, consent_ip,
				pdpa_consent, data_retention_until
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING user_id, voter_phone, voter_name, voter_email, favorite_video, created_at, created_at
		`

		start := time.Now()
		err = r.db.Pool.QueryRow(ctx, insertQuery,
			userID,
			normalizedPhone,
			fullName,
			req.Email,
			req.FavoriteVideo,
			ipAddress,
			userAgent,
			&consentTime,
			ipAddress,
			req.ConsentPDPA,
			&retentionTime,
		).Scan(
			&response.UserID,
			&response.Phone,
			&fullName,
			&response.Email,
			&response.FavoriteVideo,
			&response.CreatedAt,
			&response.UpdatedAt,
		)
		dur := time.Since(start)

		if err != nil {
			r.log.Info("db_upsert_personal_info_insert_new", zap.Duration("duration", dur), zap.Error(err))
			return nil, fmt.Errorf("failed to insert new user: %w", err)
		}
		r.log.Debug("db_upsert_personal_info_insert_new", zap.Duration("duration", dur))
	}

	// Split the full name back
	names := strings.Fields(fullName)
	if len(names) >= 2 {
		response.FirstName = names[0]
		response.LastName = strings.Join(names[1:], " ")
	} else {
		response.FirstName = fullName
		response.LastName = ""
	}

	response.Message = "Personal information saved successfully"

	return &response, nil
}

// UpdateVoteOnly updates only the vote-related fields for an existing user
func (r *VoteRepository) UpdateVoteOnly(ctx context.Context, req *domain.VoteOnlyRequest) (*domain.VoteOnlyResponse, error) {
	// First check if user exists
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM votes WHERE user_id = $1)`
	start := time.Now()
	err := r.db.GetReadPool().QueryRow(ctx, checkQuery, req.UserID).Scan(&exists)
	dur := time.Since(start)

	if err != nil {
		r.log.Info("db_update_vote_only_check_existence", zap.Duration("duration", dur), zap.Error(err))
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}
	r.log.Debug("db_update_vote_only_check_existence", zap.Duration("duration", dur))
	if !exists {
		return nil, domain.ErrUserNotFound
	}

	// Check if user has already voted (if candidate_id is not null/0)
	var existingCandidateID *int
	checkVoteQuery := `SELECT team_id FROM votes WHERE user_id = $1 AND team_id IS NOT NULL AND team_id != 0`
	start = time.Now()
	err = r.db.GetReadPool().QueryRow(ctx, checkVoteQuery, req.UserID).Scan(&existingCandidateID)
	dur = time.Since(start)

	if err != nil && err != pgx.ErrNoRows {
		r.log.Info("db_update_vote_only_check_existing_vote", zap.Duration("duration", dur), zap.Error(err))
		return nil, fmt.Errorf("failed to check existing vote: %w", err)
	}
	r.log.Debug("db_update_vote_only_check_existing_vote", zap.Duration("duration", dur))

	// If user has already voted, prevent change (since we don't have vote_status, all votes are considered finalized)
	if existingCandidateID != nil {
		return nil, domain.ErrVoteFinalized
	}

	votedAt := time.Now()

	// Generate vote_id if not already present (for when user actually votes)
	voteID := r.generateVoteID()

	// Update only vote-related fields, generate vote_id if null
	updateQuery := `
		UPDATE votes 
		SET team_id = $2, 
		    vote_id = COALESCE(vote_id, $3)
		WHERE user_id = $1
		RETURNING team_id, created_at, vote_id
	`

	var response domain.VoteOnlyResponse
	var candidateID int
	var returnedVoteID *string

	var createdAt time.Time
	start = time.Now()
	err = r.db.Pool.QueryRow(ctx, updateQuery,
		req.UserID,
		req.CandidateID,
		voteID,
	).Scan(&candidateID, &createdAt, &returnedVoteID)
	dur = time.Since(start)

	if err != nil {
		r.log.Info("db_update_vote_only_error", zap.Duration("duration", dur), zap.Error(err))
		return nil, fmt.Errorf("failed to update vote: %w", err)
	}
	r.log.Debug("db_update_vote_only", zap.Duration("duration", dur))

	response.UserID = req.UserID
	response.CandidateID = candidateID
	response.VotedAt = votedAt
	// Set the vote ID from the returned value (either existing or newly generated)
	if returnedVoteID != nil {
		response.VoteID = *returnedVoteID
	}
	response.Message = "Vote submitted successfully"

	// Note: Materialized view refresh moved to a periodic background task

	return &response, nil
}

// GetUserByPhone retrieves user info by normalized phone number
func (r *VoteRepository) GetUserByPhone(ctx context.Context, normalizedPhone string) (*domain.Vote, error) {
	var vote domain.Vote
	query := `
		SELECT user_id, voter_phone, voter_name, voter_email, favorite_video,
		       team_id, ip_address, user_agent,
		       consent_timestamp, consent_ip, pdpa_consent,
		       data_retention_until, created_at
		FROM votes
		WHERE voter_phone = $1
	`

	var fullName string
	var teamID *int

	start := time.Now()
	err := r.db.GetReadPool().QueryRow(ctx, query, normalizedPhone).Scan(
		&vote.UserID,
		&vote.Phone,
		&fullName,
		&vote.Email,
		&vote.FavoriteVideo,
		&teamID,
		&vote.IPAddress,
		&vote.UserAgent,
		&vote.ConsentTimestamp,
		&vote.ConsentIP,
		&vote.ConsentPDPA,
		&vote.DataRetentionUntil,
		&vote.CreatedAt,
	)
	dur := time.Since(start)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.log.Info("db_get_user_by_phone", zap.Duration("duration", dur), zap.Error(err))
		return nil, fmt.Errorf("failed to get user by phone: %w", err)
	}
	r.log.Debug("db_get_user_by_phone", zap.Duration("duration", dur))

	// Parse full name
	names := strings.Fields(fullName)
	if len(names) >= 2 {
		vote.FirstName = names[0]
		vote.LastName = strings.Join(names[1:], " ")
	} else {
		vote.FirstName = fullName
		vote.LastName = ""
	}

	// Set UpdatedAt to CreatedAt since there's no separate updated timestamp in the database
	vote.UpdatedAt = vote.CreatedAt

	// Set vote fields
	if teamID != nil {
		vote.CandidateID = *teamID
		if *teamID > 0 {
			votedAt := vote.CreatedAt // Use CreatedAt as the voted time
			vote.VotedAt = &votedAt
		}
	}

	return &vote, nil
}

// generateVoteID generates a unique vote ID
func (r *VoteRepository) generateVoteID() string {
	year := time.Now().Year()
	bytes := make([]byte, 6)
	cryptorand.Read(bytes)
	random := hex.EncodeToString(bytes)
	return fmt.Sprintf("VOTE%d%s", year, strings.ToUpper(random))
}

// SaveWelcomeAcceptance saves welcome/rules acceptance to database
// Creates a new record if user doesn't exist, or updates existing record
func (r *VoteRepository) SaveWelcomeAcceptance(ctx context.Context, userID, rulesVersion string) error {
	acceptedAt := time.Now()

	// First, try to update existing record
	updateQuery := `
		UPDATE votes 
		SET welcome_accepted = true,
		    welcome_accepted_at = $2,
		    rules_version = $3
		WHERE user_id = $1
	`

	start := time.Now()
	result, err := r.db.Pool.Exec(ctx, updateQuery, userID, acceptedAt, rulesVersion)
	dur := time.Since(start)

	if err != nil {
		r.log.Info("db_save_welcome_acceptance_update", zap.Duration("duration", dur), zap.Error(err))
		return fmt.Errorf("failed to save welcome acceptance: %w", err)
	}
	r.log.Debug("db_save_welcome_acceptance_update", zap.Duration("duration", dur))

	// If no rows were affected, user doesn't exist yet - create new record
	if result.RowsAffected() == 0 {
		// DO NOT create vote_id during welcome acceptance - only when user actually votes
		// Include empty strings for required NOT NULL fields (voter_name, voter_email)
		// These will be filled when user submits personal info
		insertQuery := `
			INSERT INTO votes (
				user_id, voter_name, voter_email, voter_phone,
				welcome_accepted, welcome_accepted_at, rules_version
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`

		start := time.Now()
		_, err = r.db.Pool.Exec(ctx, insertQuery,
			userID,       // user_id
			"",           // voter_name (empty, will be filled later)
			"",           // voter_email (empty, will be filled later)
			nil,          // voter_phone (NULL, will be filled later - avoids unique constraint)
			true,         // welcome_accepted
			acceptedAt,   // welcome_accepted_at
			rulesVersion, // rules_version
		)
		dur = time.Since(start)

		if err != nil {
			r.log.Info("db_save_welcome_acceptance_insert", zap.Duration("duration", dur), zap.Error(err))
			return fmt.Errorf("failed to create welcome acceptance record: %w", err)
		}
		r.log.Debug("db_save_welcome_acceptance_insert", zap.Duration("duration", dur))
	}

	return nil
}

// GetWelcomeAcceptance retrieves welcome acceptance status from database
func (r *VoteRepository) GetWelcomeAcceptance(ctx context.Context, userID string) (*domain.WelcomeAcceptanceResponse, error) {
	query := `
		SELECT user_id, welcome_accepted, welcome_accepted_at, rules_version
		FROM votes 
		WHERE user_id = $1
	`

	var response domain.WelcomeAcceptanceResponse
	var welcomeAcceptedAt sql.NullTime
	var rulesVersion sql.NullString

	start := time.Now()
	err := r.db.GetReadPool().QueryRow(ctx, query, userID).Scan(
		&response.UserID,
		&response.WelcomeAccepted,
		&welcomeAcceptedAt,
		&rulesVersion,
	)
	dur := time.Since(start)

	if err == pgx.ErrNoRows {
		return nil, nil // User not found
	}
	if err != nil {
		r.log.Info("db_get_welcome_acceptance", zap.Duration("duration", dur), zap.Error(err))
		return nil, fmt.Errorf("failed to get welcome acceptance: %w", err)
	}
	r.log.Debug("db_get_welcome_acceptance", zap.Duration("duration", dur))

	if welcomeAcceptedAt.Valid {
		response.WelcomeAcceptedAt = welcomeAcceptedAt.Time
	}
	if rulesVersion.Valid {
		response.RulesVersion = rulesVersion.String
	}

	return &response, nil
}

// GetPersonalInfoByUserID retrieves personal info for the authenticated user
func (r *VoteRepository) GetPersonalInfoByUserID(ctx context.Context, userID string) (*domain.PersonalInfoMeResponse, error) {
	query := `
		SELECT 
			user_id, voter_phone, voter_name, voter_email, favorite_video, pdpa_consent, 
			created_at, created_at as updated_at, consent_timestamp, marketing_consent,
			welcome_accepted, welcome_accepted_at, rules_version
		FROM votes 
		WHERE user_id = $1
	`

	var response domain.PersonalInfoMeResponse
	var consentTimestamp sql.NullTime
	var voterName sql.NullString
	var voterPhone sql.NullString
	var voterEmail sql.NullString
	var favoriteVideo sql.NullString
	var welcomeAcceptedAt sql.NullTime
	var rulesVersion sql.NullString

	start := time.Now()
	err := r.db.GetReadPool().QueryRow(ctx, query, userID).Scan(
		&response.UserID,
		&voterPhone,
		&voterName,
		&voterEmail,
		&favoriteVideo,
		&response.ConsentPDPA,
		&response.CreatedAt,
		&response.UpdatedAt,
		&consentTimestamp,
		&response.MarketingConsent,
		&response.WelcomeAccepted,
		&welcomeAcceptedAt,
		&rulesVersion,
	)
	dur := time.Since(start)

	if err != nil {
		if err == pgx.ErrNoRows {
			r.log.Info("db_get_personal_info_not_found", zap.String("user_id", userID))
			return nil, fmt.Errorf("personal info not found for user_id: %s", userID)
		}
		r.log.Info("db_get_personal_info", zap.Duration("duration", dur), zap.Error(err))
		return nil, fmt.Errorf("failed to get personal info: %w", err)
	}
	r.log.Debug("db_get_personal_info", zap.Duration("duration", dur))

	// Handle nullable fields
	if voterPhone.Valid {
		response.Phone = voterPhone.String
	}
	if voterEmail.Valid {
		response.Email = voterEmail.String
	}
	if favoriteVideo.Valid {
		response.FavoriteVideo = favoriteVideo.String
	}

	// Split voter_name into first and last name
	if voterName.Valid && voterName.String != "" {
		names := strings.SplitN(voterName.String, " ", 2)
		response.FirstName = names[0]
		if len(names) > 1 {
			response.LastName = names[1]
		}
	}

	if consentTimestamp.Valid {
		response.ConsentTimestamp = &consentTimestamp.Time
	}

	// Set welcome acceptance fields
	if welcomeAcceptedAt.Valid {
		response.WelcomeAcceptedAt = &welcomeAcceptedAt.Time
	}
	if rulesVersion.Valid {
		response.RulesVersion = rulesVersion.String
	}

	return &response, nil
}

// GetRandomVoteWithTeam retrieves a random vote with team information for production use
func (r *VoteRepository) GetRandomVoteWithTeam(ctx context.Context) (*domain.RandomVoteWithTeamResponse, error) {
	// Use a more reliable random selection with TABLESAMPLE for better distribution
	voteQuery := `
		SELECT v.vote_id, v.voter_name, v.voter_email, v.voter_phone, v.team_id
		FROM votes v TABLESAMPLE BERNOULLI(1)
		WHERE
		v.team_id IS NOT NULL AND
		v.vote_id IS NOT NULL AND
		v.voter_phone IS NOT NULL AND
		v.voter_email IS NOT NULL AND
		v.voter_name IS NOT NULL
		ORDER BY RANDOM()
		LIMIT 1
	`

	var voteID, voterName, voterEmail string
	var voterPhone sql.NullString
	var teamID int

	start := time.Now()
	err := r.db.GetReadPool().QueryRow(ctx, voteQuery).Scan(
		&voteID,
		&voterName,
		&voterEmail,
		&voterPhone,
		&teamID,
	)
	voteQueryDur := time.Since(start)

	if err == pgx.ErrNoRows {
		r.log.Info("db_get_random_vote_no_results", zap.Duration("duration", voteQueryDur))
		return nil, fmt.Errorf("no votes found")
	}
	if err != nil {
		r.log.Info("db_get_random_vote_error", zap.Duration("duration", voteQueryDur), zap.Error(err))
		return nil, fmt.Errorf("failed to get random vote: %w", err)
	}

	// Second, fetch the team name for this specific team_id
	teamQuery := `SELECT name FROM teams WHERE id = $1`
	var teamName string

	teamStart := time.Now()
	err = r.db.GetReadPool().QueryRow(ctx, teamQuery, teamID).Scan(&teamName)
	teamQueryDur := time.Since(teamStart)

	if err != nil {
		r.log.Info("db_get_team_name_error",
			zap.Int("team_id", teamID),
			zap.Duration("duration", teamQueryDur),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get team name for team_id %d: %w", teamID, err)
	}

	totalDur := voteQueryDur + teamQueryDur
	r.log.Debug("db_get_random_vote_with_team_success",
		zap.Duration("vote_query_duration", voteQueryDur),
		zap.Duration("team_query_duration", teamQueryDur),
		zap.Duration("total_duration", totalDur))

	// Handle NULL voter_phone
	voterPhoneStr := ""
	if voterPhone.Valid {
		voterPhoneStr = voterPhone.String
	}

	response := &domain.RandomVoteWithTeamResponse{
		VoteID:     voteID,
		VoterName:  voterName,
		VoterEmail: voterEmail,
		VoterPhone: voterPhoneStr,
		TeamName:   teamName,
	}

	return response, nil
}

// GetRandomWinners retrieves multiple unique random winners for lottery
func (r *VoteRepository) GetRandomWinners(ctx context.Context, count int) ([]domain.WinnerInfo, error) {
	// First, get a larger pool of random votes (limit to 50 to ensure variety)
	// Then select the required number from this pool
	poolSize := 50
	if count > poolSize {
		poolSize = count // If we need more than 50, get exactly what we need
	}

	// Query to get random votes with team information
	// Using ORDER BY RANDOM() for PostgreSQL
	query := `
		SELECT
			v.vote_id,
			v.voter_name,
			v.voter_email,
			v.voter_phone,
			t.name as team_name
		FROM votes v
		JOIN teams t ON v.team_id = t.id
		WHERE v.vote_id IS NOT NULL
		AND v.vote_id != ''
		AND v.voter_phone IS NOT NULL
		AND v.voter_email IS NOT NULL
		AND v.voter_name IS NOT NULL
		AND v.team_id IS NOT NULL
		AND v.team_id = 1
		ORDER BY RANDOM()
		LIMIT $1
	`

	start := time.Now()
	rows, err := r.db.GetReadPool().Query(ctx, query, poolSize)
	dur := time.Since(start)

	if err != nil {
		r.log.Info("db_get_random_winners_error", zap.Duration("duration", dur), zap.Error(err))
		return nil, fmt.Errorf("failed to get random winners: %w", err)
	}
	defer rows.Close()

	var allWinners []domain.WinnerInfo
	for rows.Next() {
		var winner domain.WinnerInfo
		var voterPhone sql.NullString

		err := rows.Scan(
			&winner.VoteID,
			&winner.VoterName,
			&winner.VoterEmail,
			&voterPhone,
			&winner.TeamName,
		)
		if err != nil {
			r.log.Error("Failed to scan winner row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan winner: %w", err)
		}

		// Handle NULL voter_phone
		if voterPhone.Valid {
			winner.VoterPhone = &voterPhone.String
		}

		allWinners = append(allWinners, winner)
	}

	if err = rows.Err(); err != nil {
		r.log.Error("Error iterating winner rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// If we have more winners than needed, randomly select from the pool
	if len(allWinners) > count {
		// Shuffle the array using Fisher-Yates algorithm
		for i := len(allWinners) - 1; i > 0; i-- {
			j := rand.Intn(i + 1)
			allWinners[i], allWinners[j] = allWinners[j], allWinners[i]
		}
		// Return only the required number
		allWinners = allWinners[:count]
	}

	r.log.Debug("db_get_random_winners_success",
		zap.Int("pool_size", poolSize),
		zap.Int("requested", count),
		zap.Int("returned", len(allWinners)),
		zap.Duration("duration", dur))

	return allWinners, nil
}

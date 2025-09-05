package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"be-v2/internal/domain"
	"be-v2/pkg/database"

	"github.com/jackc/pgx/v5"
)

type VoteRepository struct {
	db *database.PostgresDB
}

func NewVoteRepository(db *database.PostgresDB) *VoteRepository {
	return &VoteRepository{db: db}
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

	if err != nil {
		return fmt.Errorf("failed to create vote: %w", err)
	}

	// Refresh materialized view asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = r.db.RefreshMaterializedView(ctx)
	}()

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

	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
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

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get vote: %w", err)
	}

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

	err := r.db.Pool.QueryRow(ctx, query, voteID).Scan(
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

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get vote by ID: %w", err)
	}

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

	err := r.db.Pool.QueryRow(ctx, query, phone).Scan(
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

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get vote by phone: %w", err)
	}

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
		SELECT id, code, name, description, icon, member_count, vote_count, last_vote_at
		FROM vote_summary
		ORDER BY vote_count DESC, name ASC
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams with vote counts: %w", err)
	}
	defer rows.Close()

	var teams []domain.Team
	for rows.Next() {
		var team domain.Team
		err := rows.Scan(
			&team.ID,
			&team.Code,
			&team.Name,
			&team.Description,
			&team.Icon,
			&team.MemberCount,
			&team.VoteCount,
			&team.LastVoteAt,
		)
		if err != nil {
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

	err := r.db.Pool.QueryRow(ctx, query, teamID).Scan(
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

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}

	return &team, nil
}

// GetTotalVoteCount gets the total number of votes
func (r *VoteRepository) GetTotalVoteCount(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM votes`

	err := r.db.Pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get total vote count: %w", err)
	}

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
		return nil, fmt.Errorf("failed to check existing user record: %w", err)
	}

	// Check if phone number is already used by another user (not the current user)
	existingPhoneUser, err := r.GetUserByPhone(ctx, normalizedPhone)
	if err != nil {
		return nil, fmt.Errorf("failed to check phone uniqueness: %w", err)
	}

	// If phone is used by another user (different userID), reject the request
	if existingPhoneUser != nil && existingPhoneUser.UserID != userID {
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

		if err != nil {
			return nil, fmt.Errorf("failed to update existing user: %w", err)
		}
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

		if err != nil {
			return nil, fmt.Errorf("failed to insert new user: %w", err)
		}
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
	err := r.db.Pool.QueryRow(ctx, checkQuery, req.UserID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}
	if !exists {
		return nil, domain.ErrUserNotFound
	}

	// Check if user has already voted (if candidate_id is not null/0)
	var existingCandidateID *int
	checkVoteQuery := `SELECT team_id FROM votes WHERE user_id = $1 AND team_id IS NOT NULL AND team_id != 0`
	err = r.db.Pool.QueryRow(ctx, checkVoteQuery, req.UserID).Scan(&existingCandidateID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing vote: %w", err)
	}

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
	err = r.db.Pool.QueryRow(ctx, updateQuery,
		req.UserID,
		req.CandidateID,
		voteID,
	).Scan(&candidateID, &createdAt, &returnedVoteID)

	if err != nil {
		return nil, fmt.Errorf("failed to update vote: %w", err)
	}

	response.UserID = req.UserID
	response.CandidateID = candidateID
	response.VotedAt = votedAt
	// Set the vote ID from the returned value (either existing or newly generated)
	if returnedVoteID != nil {
		response.VoteID = *returnedVoteID
	}
	response.Message = "Vote submitted successfully"

	// Refresh materialized view asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = r.db.RefreshMaterializedView(ctx)
	}()

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

	err := r.db.Pool.QueryRow(ctx, query, normalizedPhone).Scan(
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

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by phone: %w", err)
	}

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
	rand.Read(bytes)
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

	result, err := r.db.Pool.Exec(ctx, updateQuery, userID, acceptedAt, rulesVersion)
	if err != nil {
		return fmt.Errorf("failed to save welcome acceptance: %w", err)
	}

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

		_, err = r.db.Pool.Exec(ctx, insertQuery, 
			userID,        // user_id
			"",           // voter_name (empty, will be filled later)
			"",           // voter_email (empty, will be filled later)
			nil,          // voter_phone (NULL, will be filled later - avoids unique constraint)
			true,         // welcome_accepted
			acceptedAt,   // welcome_accepted_at
			rulesVersion, // rules_version
		)
		if err != nil {
			return fmt.Errorf("failed to create welcome acceptance record: %w", err)
		}
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

	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&response.UserID,
		&response.WelcomeAccepted,
		&welcomeAcceptedAt,
		&rulesVersion,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // User not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get welcome acceptance: %w", err)
	}

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

	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
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

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("personal info not found for user_id: %s", userID)
		}
		return nil, fmt.Errorf("failed to get personal info: %w", err)
	}

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

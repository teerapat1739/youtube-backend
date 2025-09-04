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
	query := `
		SELECT id, vote_id, user_id, team_id, voter_name, voter_email, voter_phone, 
		       favorite_video, ip_address, user_agent, consent_timestamp, consent_ip,
		       privacy_policy_version, pdpa_consent, marketing_consent, 
		       data_retention_until, created_at
		FROM votes
		WHERE user_id = $1
	`

	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&vote.ID,
		&vote.VoteID,
		&vote.UserID,
		&vote.TeamID,
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
		return nil, fmt.Errorf("failed to get vote: %w", err)
	}

	return &vote, nil
}

// GetVoteByVoteID gets a vote by vote ID
func (r *VoteRepository) GetVoteByVoteID(ctx context.Context, voteID string) (*domain.Vote, error) {
	var vote domain.Vote
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
		&vote.TeamID,
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

	return &vote, nil
}

// GetVoteByPhone gets a vote by phone number
func (r *VoteRepository) GetVoteByPhone(ctx context.Context, phone string) (*domain.Vote, error) {
	var vote domain.Vote
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
		&vote.VoteID,
		&vote.UserID,
		&vote.TeamID,
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

// UpsertPersonalInfo creates or updates personal information based on normalized phone number
// Returns the user_id (creates new if doesn't exist)
// This method handles personal info storage WITHOUT requiring vote_id (vote_id is nullable)
func (r *VoteRepository) UpsertPersonalInfo(ctx context.Context, req *domain.PersonalInfoRequest, normalizedPhone, ipAddress, userAgent string) (*domain.PersonalInfoResponse, error) {
	consentTime := time.Now()
	retentionTime := time.Now().AddDate(1, 0, 0) // 1 year from now
	fullName := fmt.Sprintf("%s %s", req.FirstName, req.LastName)

	// First, try the upsert approach (if unique constraint exists)
	userID := r.generateUserID()
	voteID := r.generateVoteID() // Generate vote_id to satisfy NOT NULL constraint
	query := `
		INSERT INTO votes (
			vote_id, user_id, voter_phone, voter_name, voter_email, favorite_video,
			ip_address, user_agent, consent_timestamp, consent_ip,
			pdpa_consent, data_retention_until
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (voter_phone) 
		DO UPDATE SET
			voter_name = EXCLUDED.voter_name,
			voter_email = EXCLUDED.voter_email,
			favorite_video = EXCLUDED.favorite_video,
			ip_address = EXCLUDED.ip_address,
			user_agent = EXCLUDED.user_agent,
			consent_timestamp = EXCLUDED.consent_timestamp,
			consent_ip = EXCLUDED.consent_ip,
			pdpa_consent = EXCLUDED.pdpa_consent,
			data_retention_until = EXCLUDED.data_retention_until
		RETURNING user_id, voter_phone, voter_name, voter_email, favorite_video, created_at, created_at
	`

	var response domain.PersonalInfoResponse

	err := r.db.Pool.QueryRow(ctx, query,
		voteID,
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
		&fullName, // We'll split this back
		&response.Email,
		&response.FavoriteVideo,
		&response.CreatedAt,
		&response.UpdatedAt, // This will get the same value as CreatedAt due to the RETURNING clause
	)

	// If the unique constraint doesn't exist, fall back to manual upsert logic
	if err != nil && strings.Contains(err.Error(), "no unique or exclusion constraint") {
		return r.manualUpsertPersonalInfo(ctx, req, normalizedPhone, ipAddress, userAgent)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to upsert personal info: %w", err)
	}

	// Split the full name back (simple approach)
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

// manualUpsertPersonalInfo handles upsert logic when unique constraint doesn't exist
func (r *VoteRepository) manualUpsertPersonalInfo(ctx context.Context, req *domain.PersonalInfoRequest, normalizedPhone, ipAddress, userAgent string) (*domain.PersonalInfoResponse, error) {
	consentTime := time.Now()
	retentionTime := time.Now().AddDate(1, 0, 0) // 1 year from now
	fullName := fmt.Sprintf("%s %s", req.FirstName, req.LastName)

	// First, try to find existing user by phone
	existingUser, err := r.GetUserByPhone(ctx, normalizedPhone)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	var response domain.PersonalInfoResponse

	if existingUser != nil {
		// Update existing user
		updateQuery := `
			UPDATE votes 
			SET voter_name = $2, voter_email = $3, favorite_video = $4, ip_address = $5, user_agent = $6,
			    consent_timestamp = $7, consent_ip = $8, pdpa_consent = $9, data_retention_until = $10
			WHERE voter_phone = $1
			RETURNING user_id, voter_phone, voter_name, voter_email, favorite_video, created_at, NOW()
		`

		err = r.db.Pool.QueryRow(ctx, updateQuery,
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
		// Insert new user
		userID := r.generateUserID()
		voteID := r.generateVoteID() // Generate vote_id to satisfy NOT NULL constraint
		insertQuery := `
			INSERT INTO votes (
				vote_id, user_id, voter_phone, voter_name, voter_email, favorite_video,
				ip_address, user_agent, consent_timestamp, consent_ip,
				pdpa_consent, data_retention_until
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING user_id, voter_phone, voter_name, voter_email, favorite_video, created_at, created_at
		`

		err = r.db.Pool.QueryRow(ctx, insertQuery,
			voteID,
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

	// Split the full name back (simple approach)
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

// generateUserID generates a unique user ID
func (r *VoteRepository) generateUserID() string {
	year := time.Now().Year()
	bytes := make([]byte, 4)
	rand.Read(bytes)
	random := hex.EncodeToString(bytes)
	return fmt.Sprintf("USR%d%s", year, random)
}

// generateVoteID generates a unique vote ID
func (r *VoteRepository) generateVoteID() string {
	year := time.Now().Year()
	bytes := make([]byte, 6)
	rand.Read(bytes)
	random := hex.EncodeToString(bytes)
	return fmt.Sprintf("VOTE%d%s", year, strings.ToUpper(random))
}

// GetPersonalInfoByUserID retrieves personal info for the authenticated user
func (r *VoteRepository) GetPersonalInfoByUserID(ctx context.Context, userID string) (*domain.PersonalInfoMeResponse, error) {
	query := `
		SELECT 
			user_id, voter_phone, voter_name, voter_email, favorite_video, pdpa_consent, 
			created_at, created_at as updated_at, consent_timestamp, marketing_consent
		FROM votes 
		WHERE user_id = $1
	`

	var response domain.PersonalInfoMeResponse
	var consentTimestamp sql.NullTime
	var voterName sql.NullString

	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&response.UserID,
		&response.Phone,
		&voterName,
		&response.Email,
		&response.FavoriteVideo,
		&response.ConsentPDPA,
		&response.CreatedAt,
		&response.UpdatedAt,
		&consentTimestamp,
		&response.MarketingConsent,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("personal info not found for user_id: %s", userID)
		}
		return nil, fmt.Errorf("failed to get personal info: %w", err)
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

	return &response, nil
}

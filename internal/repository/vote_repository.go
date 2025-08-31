package repository

import (
	"context"
	"fmt"
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
			ip_address, user_agent, consent_timestamp, consent_ip, 
			privacy_policy_version, pdpa_consent, marketing_consent, data_retention_until
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		vote.VoteID,
		vote.UserID,
		vote.TeamID,
		vote.VoterName,
		vote.VoterEmail,
		vote.VoterPhone,
		vote.IPAddress,
		vote.UserAgent,
		vote.ConsentTimestamp,
		vote.ConsentIP,
		vote.PrivacyPolicyVersion,
		vote.PDPAConsent,
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
		       ip_address, user_agent, consent_timestamp, consent_ip,
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
		&vote.IPAddress,
		&vote.UserAgent,
		&vote.ConsentTimestamp,
		&vote.ConsentIP,
		&vote.PrivacyPolicyVersion,
		&vote.PDPAConsent,
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
		       ip_address, user_agent, consent_timestamp, consent_ip,
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
		&vote.IPAddress,
		&vote.UserAgent,
		&vote.ConsentTimestamp,
		&vote.ConsentIP,
		&vote.PrivacyPolicyVersion,
		&vote.PDPAConsent,
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
		       ip_address, user_agent, consent_timestamp, consent_ip,
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
		&vote.IPAddress,
		&vote.UserAgent,
		&vote.ConsentTimestamp,
		&vote.ConsentIP,
		&vote.PrivacyPolicyVersion,
		&vote.PDPAConsent,
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
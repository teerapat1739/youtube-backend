package repository

import (
	"context"
	"fmt"

	"github.com/gamemini/youtube/pkg/database"
	"github.com/gamemini/youtube/pkg/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VoteRepository handles vote-related database operations
type VoteRepository struct {
	db *pgxpool.Pool
}

// NewVoteRepository creates a new vote repository instance
func NewVoteRepository() *VoteRepository {
	return &VoteRepository{
		db: database.GetDB(),
	}
}

// GetCountsByActivity returns vote counts grouped by team for a specific activity
func (r *VoteRepository) GetCountsByActivity(ctx context.Context, activityID string) (map[string]int64, error) {
	// Resolve activity ID to proper UUID
	resolvedActivityID := models.ResolveActivityID(activityID)
	
	// Validate UUID format for security
	if _, err := uuid.Parse(resolvedActivityID); err != nil {
		return nil, fmt.Errorf("invalid activity ID format: %w", err)
	}
	
	query := `
		SELECT team_id, COUNT(*) AS vote_count
		FROM votes
		WHERE activity_id = $1
		GROUP BY team_id
	`
	
	rows, err := r.db.Query(ctx, query, resolvedActivityID)
	if err != nil {
		return nil, fmt.Errorf("failed to query vote counts: %w", err)
	}
	defer rows.Close()
	
	counts := make(map[string]int64)
	for rows.Next() {
		var teamID string
		var count int64
		
		if err := rows.Scan(&teamID, &count); err != nil {
			return nil, fmt.Errorf("failed to scan vote count row: %w", err)
		}
		
		counts[teamID] = count
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating vote count rows: %w", err)
	}
	
	return counts, nil
}

// GetTotalVotesByActivity returns the total number of votes for an activity
func (r *VoteRepository) GetTotalVotesByActivity(ctx context.Context, activityID string) (int64, error) {
	// Resolve activity ID to proper UUID
	resolvedActivityID := models.ResolveActivityID(activityID)
	
	// Validate UUID format for security
	if _, err := uuid.Parse(resolvedActivityID); err != nil {
		return 0, fmt.Errorf("invalid activity ID format: %w", err)
	}
	
	query := `
		SELECT COUNT(*) 
		FROM votes
		WHERE activity_id = $1
	`
	
	var total int64
	err := r.db.QueryRow(ctx, query, resolvedActivityID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count total votes: %w", err)
	}
	
	return total, nil
}
package repository

import (
	"context"
	"fmt"

	"github.com/gamemini/youtube/pkg/database"
	"github.com/gamemini/youtube/pkg/models"
	"github.com/jackc/pgx/v5"
)

// ActivityRepository handles activity-related database operations
type ActivityRepository struct{}

// NewActivityRepository creates a new activity repository
func NewActivityRepository() *ActivityRepository {
	return &ActivityRepository{}
}

// GetActivity retrieves an activity by ID
func (r *ActivityRepository) GetActivity(ctx context.Context, activityID string) (*models.Activity, error) {
	query := `
		SELECT id, title, description, channel_id, start_date, end_date, created_at
		FROM activities
		WHERE id = $1
	`

	var activity models.Activity
	err := database.GetDB().QueryRow(ctx, query, activityID).Scan(
		&activity.ID,
		&activity.Title,
		&activity.Description,
		&activity.ChannelID,
		&activity.StartDate,
		&activity.EndDate,
		&activity.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get activity: %v", err)
	}

	return &activity, nil
}

// GetActiveActivity retrieves the currently active activity
func (r *ActivityRepository) GetActiveActivity(ctx context.Context) (*models.Activity, error) {
	query := `
		SELECT id, title, description, channel_id, start_date, end_date, created_at
		FROM activities
		WHERE status = 'active' AND NOW() BETWEEN start_date AND end_date
		ORDER BY created_at DESC
		LIMIT 1
	`

	var activity models.Activity
	err := database.GetDB().QueryRow(ctx, query).Scan(
		&activity.ID,
		&activity.Title,
		&activity.Description,
		&activity.ChannelID,
		&activity.StartDate,
		&activity.EndDate,
		&activity.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active activity: %v", err)
	}

	return &activity, nil
}

// GetTeams retrieves all teams for an activity
func (r *ActivityRepository) GetTeams(ctx context.Context, activityID string) ([]models.Team, error) {
	query := `
		SELECT id, activity_id, name, display_name, image_url, description, created_at
		FROM teams
		WHERE activity_id = $1
		ORDER BY name
	`

	rows, err := database.GetDB().Query(ctx, query, activityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams: %v", err)
	}
	defer rows.Close()

	var teams []models.Team
	for rows.Next() {
		var team models.Team
		err := rows.Scan(
			&team.ID,
			&team.ActivityID,
			&team.Name,
			&team.DisplayName,
			&team.ImageURL,
			&team.Description,
			&team.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team: %v", err)
		}
		teams = append(teams, team)
	}

	return teams, nil
}

// GetTeamsWithVoteCount retrieves teams with their vote counts
func (r *ActivityRepository) GetTeamsWithVoteCount(ctx context.Context, activityID string) ([]models.TeamWithVoteCount, error) {
	query := `
		SELECT 
			t.id, t.activity_id, t.name, t.display_name, t.image_url, t.description, t.created_at,
			COALESCE(COUNT(v.id), 0) as vote_count
		FROM teams t
		LEFT JOIN votes v ON t.id = v.team_id AND v.activity_id = $1
		WHERE t.activity_id = $1
		GROUP BY t.id, t.activity_id, t.name, t.display_name, t.image_url, t.description, t.created_at
		ORDER BY t.name
	`

	rows, err := database.GetDB().Query(ctx, query, activityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams with vote count: %v", err)
	}
	defer rows.Close()

	var teamsWithVotes []models.TeamWithVoteCount
	for rows.Next() {
		var teamWithVote models.TeamWithVoteCount
		err := rows.Scan(
			&teamWithVote.Team.ID,
			&teamWithVote.Team.ActivityID,
			&teamWithVote.Team.Name,
			&teamWithVote.Team.DisplayName,
			&teamWithVote.Team.ImageURL,
			&teamWithVote.Team.Description,
			&teamWithVote.Team.CreatedAt,
			&teamWithVote.VoteCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team with vote count: %v", err)
		}
		teamsWithVotes = append(teamsWithVotes, teamWithVote)
	}

	return teamsWithVotes, nil
}

// CreateVote creates a new vote
func (r *ActivityRepository) CreateVote(ctx context.Context, vote *models.Vote) error {
	query := `
		INSERT INTO votes (user_id, team_id, activity_id)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`

	err := database.GetDB().QueryRow(ctx, query,
		vote.UserID,
		vote.TeamID,
		vote.ActivityID,
	).Scan(&vote.ID, &vote.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create vote: %v", err)
	}

	return nil
}

// GetUserVote retrieves a user's vote for an activity
func (r *ActivityRepository) GetUserVote(ctx context.Context, userID, activityID string) (*models.Vote, error) {
	query := `
		SELECT id, user_id, team_id, activity_id, created_at
		FROM votes
		WHERE user_id = $1 AND activity_id = $2
	`

	var vote models.Vote
	err := database.GetDB().QueryRow(ctx, query, userID, activityID).Scan(
		&vote.ID,
		&vote.UserID,
		&vote.TeamID,
		&vote.ActivityID,
		&vote.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user vote: %v", err)
	}

	return &vote, nil
}

// GetVoteCount retrieves the total vote count for an activity
func (r *ActivityRepository) GetVoteCount(ctx context.Context, activityID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM votes
		WHERE activity_id = $1
	`

	var count int
	err := database.GetDB().QueryRow(ctx, query, activityID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get vote count: %v", err)
	}

	return count, nil
}

// GetTeamVoteCount retrieves the vote count for a specific team
func (r *ActivityRepository) GetTeamVoteCount(ctx context.Context, teamID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM votes
		WHERE team_id = $1
	`

	var count int
	err := database.GetDB().QueryRow(ctx, query, teamID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get team vote count: %v", err)
	}

	return count, nil
}

// GetActivityWithTeams retrieves an activity with all its teams and vote counts
func (r *ActivityRepository) GetActivityWithTeams(ctx context.Context, activityID string) (*models.ActivityWithTeams, error) {
	activity, err := r.GetActivity(ctx, activityID)
	if err != nil {
		return nil, err
	}
	if activity == nil {
		return nil, nil
	}

	teamsWithVotes, err := r.GetTeamsWithVoteCount(ctx, activityID)
	if err != nil {
		return nil, err
	}

	return &models.ActivityWithTeams{
		Activity: *activity,
		Teams:    teamsWithVotes,
	}, nil
}

// CheckUserHasVoted checks if a user has already voted in an activity
func (r *ActivityRepository) CheckUserHasVoted(ctx context.Context, userID, activityID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM votes 
			WHERE user_id = $1 AND activity_id = $2
		)
	`

	var exists bool
	err := database.GetDB().QueryRow(ctx, query, userID, activityID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if user has voted: %v", err)
	}

	return exists, nil
}

// HasUserVoted checks if user has voted and returns the team ID
func (r *ActivityRepository) HasUserVoted(userID, activityID string) (bool, *string, error) {
	query := `
		SELECT team_id FROM votes 
		WHERE user_id = $1 AND activity_id = $2
		LIMIT 1
	`

	var teamID string
	err := database.GetDB().QueryRow(context.Background(), query, userID, activityID).Scan(&teamID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("failed to check user vote: %v", err)
	}

	return true, &teamID, nil
}

// GetUserVoteWithoutContext retrieves a user's vote for an activity (without context for service compatibility)
func (r *ActivityRepository) GetUserVoteWithoutContext(userID, activityID string) (*models.Vote, error) {
	return r.GetUserVote(context.Background(), userID, activityID)
}

// GetTeamByID retrieves a team by its ID
func (r *ActivityRepository) GetTeamByID(teamID string) (*models.Team, error) {
	query := `
		SELECT id, activity_id, name, display_name, image_url, description, created_at
		FROM teams
		WHERE id = $1
	`

	var team models.Team
	err := database.GetDB().QueryRow(context.Background(), query, teamID).Scan(
		&team.ID,
		&team.ActivityID,
		&team.Name,
		&team.DisplayName,
		&team.ImageURL,
		&team.Description,
		&team.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get team: %v", err)
	}

	return &team, nil
}

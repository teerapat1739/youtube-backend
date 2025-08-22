package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gamemini/youtube/pkg/database"
	"github.com/gamemini/youtube/pkg/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TeamService provides team and voting related operations
type TeamService struct {
	db *pgxpool.Pool
}

// NewTeamService creates a new team service instance
func NewTeamService() *TeamService {
	return &TeamService{
		db: database.GetDB(),
	}
}

// GetTeamsWithVotes returns all hardcoded teams with their vote counts for a specific activity
func (s *TeamService) GetTeamsWithVotes(ctx context.Context, activityID string) ([]models.TeamWithVotes, error) {
	log.Printf("🏆 Getting teams with votes for activity: %s", activityID)
	
	// Get hardcoded teams
	teams := models.GetHardcodedTeams()
	teamsWithVotes := make([]models.TeamWithVotes, 0, len(teams))
	
	// Get vote counts for each team
	for _, team := range teams {
		voteCount, err := s.getVoteCountForTeam(ctx, team.ID, activityID)
		if err != nil {
			log.Printf("❌ Error getting vote count for team %s: %v", team.ID, err)
			// Continue with 0 count instead of failing
			voteCount = 0
		}
		
		teamsWithVotes = append(teamsWithVotes, models.TeamWithVotes{
			Team:      team,
			VoteCount: voteCount,
		})
	}
	
	log.Printf("✅ Retrieved %d teams with votes", len(teamsWithVotes))
	return teamsWithVotes, nil
}

// getVoteCountForTeam gets the vote count for a specific team and activity
func (s *TeamService) getVoteCountForTeam(ctx context.Context, teamID, activityID string) (int, error) {
	// Resolve activity ID to proper UUID
	resolvedActivityID := models.ResolveActivityID(activityID)
	
	query := `SELECT COUNT(*) FROM votes WHERE team_id = $1 AND activity_id = $2`
	
	var count int
	err := s.db.QueryRow(ctx, query, teamID, resolvedActivityID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count votes for team %s: %w", teamID, err)
	}
	
	return count, nil
}

// SubmitVote allows a user to vote for a team
func (s *TeamService) SubmitVote(ctx context.Context, userID, teamID, activityID string) (*models.VoteResponse, error) {
	log.Printf("🗳️  Submitting vote - UserID: %s, TeamID: %s, ActivityID: %s", userID, teamID, activityID)
	
	// Resolve activity ID to proper UUID
	resolvedActivityID := models.ResolveActivityID(activityID)
	log.Printf("🔍 Resolved ActivityID: %s -> %s", activityID, resolvedActivityID)
	
	// Validate team ID
	if !models.ValidateTeamID(teamID) {
		return nil, fmt.Errorf("invalid team ID: %s", teamID)
	}
	
	// Check if user has already voted for this activity
	hasVoted, err := s.HasUserVoted(ctx, userID, activityID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing vote: %w", err)
	}
	
	if hasVoted {
		return nil, fmt.Errorf("user has already voted for this activity")
	}
	
	// Begin transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	
	// Insert vote
	voteID := uuid.New().String()
	insertQuery := `
		INSERT INTO votes (id, user_id, team_id, activity_id, created_at) 
		VALUES ($1, $2, $3, $4, $5)
	`
	
	_, err = tx.Exec(ctx, insertQuery, voteID, userID, teamID, resolvedActivityID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to insert vote: %w", err)
	}
	
	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit vote transaction: %w", err)
	}
	
	// Get the team information
	team, exists := models.GetTeamByID(teamID)
	if !exists {
		return nil, fmt.Errorf("team not found: %s", teamID)
	}
	
	// Get updated vote count (use original activityID since getVoteCountForTeam will resolve it)
	voteCount, err := s.getVoteCountForTeam(ctx, teamID, activityID)
	if err != nil {
		log.Printf("❌ Failed to get updated vote count: %v", err)
		voteCount = 1 // At least we know there's the vote we just inserted
	}
	
	log.Printf("✅ Vote submitted successfully - Vote ID: %s", voteID)
	
	return &models.VoteResponse{
		Vote: models.Vote{
			ID:         voteID,
			UserID:     userID,
			TeamID:     teamID,
			ActivityID: activityID,
			CreatedAt:  time.Now(),
		},
		Message:    fmt.Sprintf("Successfully voted for %s", team.DisplayName),
		TeamName:   team.DisplayName,
		TotalVotes: voteCount,
	}, nil
}

// HasUserVoted checks if a user has already voted for a specific activity
func (s *TeamService) HasUserVoted(ctx context.Context, userID, activityID string) (bool, error) {
	// Resolve activity ID to proper UUID
	resolvedActivityID := models.ResolveActivityID(activityID)
	
	query := `SELECT EXISTS(SELECT 1 FROM votes WHERE user_id = $1 AND activity_id = $2)`
	
	var exists bool
	err := s.db.QueryRow(ctx, query, userID, resolvedActivityID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if user has voted: %w", err)
	}
	
	return exists, nil
}

// GetUserVoteStatus gets the user's voting status for an activity
func (s *TeamService) GetUserVoteStatus(ctx context.Context, userID, activityID string) (*models.VotingStatus, error) {
	// Resolve activity ID to proper UUID
	resolvedActivityID := models.ResolveActivityID(activityID)
	
	query := `
		SELECT team_id, created_at 
		FROM votes 
		WHERE user_id = $1 AND activity_id = $2
		LIMIT 1
	`
	
	var teamID string
	var votedAt time.Time
	
	err := s.db.QueryRow(ctx, query, userID, resolvedActivityID).Scan(&teamID, &votedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			// User hasn't voted
			return &models.VotingStatus{
				HasVoted: false,
			}, nil
		}
		return nil, fmt.Errorf("failed to get user vote status: %w", err)
	}
	
	return &models.VotingStatus{
		HasVoted:    true,
		VotedTeamID: teamID,
		VotedAt:     votedAt,
	}, nil
}

// GetActivityWithTeams returns an activity with its teams, vote counts, and user voting status
func (s *TeamService) GetActivityWithTeams(ctx context.Context, activityID, userID string) (*models.EnhancedActivityWithTeams, error) {
	log.Printf("🎯 Getting activity with teams - ActivityID: %s, UserID: %s", activityID, userID)
	
	// For now, create a mock activity since we're focusing on teams
	// In the future, this could be retrieved from a database or config
	activity := models.Activity{
		ID:          activityID,
		Title:       "Ananped 10M โหวตทีมที่คุณชื่นชอบ",
		Description: "ร่วมเฉลิมฉลองกับกิจกรรมพิเศษ 10 ล้าน Subscribers!",
		StartDate:   time.Now().Add(-24 * time.Hour), // Started yesterday
		EndDate:     time.Now().Add(30 * 24 * time.Hour), // Ends in 30 days
		CreatedAt:   time.Now(),
	}
	
	// Get teams with votes
	teams, err := s.GetTeamsWithVotes(ctx, activityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams with votes: %w", err)
	}
	
	// Get user vote status
	userVote, err := s.GetUserVoteStatus(ctx, userID, activityID)
	if err != nil {
		log.Printf("❌ Failed to get user vote status: %v", err)
		// Continue with default status instead of failing
		userVote = &models.VotingStatus{HasVoted: false}
	}
	
	log.Printf("✅ Retrieved activity with %d teams", len(teams))
	
	return &models.EnhancedActivityWithTeams{
		Activity: activity,
		Teams:    teams,
		UserVote: *userVote,
	}, nil
}

// GetVoteStatistics returns vote statistics for all teams in an activity
func (s *TeamService) GetVoteStatistics(ctx context.Context, activityID string) (map[string]interface{}, error) {
	teams, err := s.GetTeamsWithVotes(ctx, activityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams with votes: %w", err)
	}
	
	totalVotes := 0
	teamStats := make([]map[string]interface{}, 0, len(teams))
	
	for _, teamWithVotes := range teams {
		totalVotes += teamWithVotes.VoteCount
		
		teamStats = append(teamStats, map[string]interface{}{
			"team_id":     teamWithVotes.Team.ID,
			"team_name":   teamWithVotes.Team.DisplayName,
			"vote_count":  teamWithVotes.VoteCount,
		})
	}
	
	// Calculate percentages
	for _, stat := range teamStats {
		voteCount := stat["vote_count"].(int)
		if totalVotes > 0 {
			stat["percentage"] = float64(voteCount) / float64(totalVotes) * 100
		} else {
			stat["percentage"] = 0.0
		}
	}
	
	return map[string]interface{}{
		"activity_id":  activityID,
		"total_votes":  totalVotes,
		"team_count":   len(teams),
		"team_stats":   teamStats,
		"generated_at": time.Now(),
	}, nil
}
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/repository"
)

// ActivityService handles business logic for activities
type ActivityService struct {
	activityRepo *repository.ActivityRepository
	userRepo     *repository.UserRepository
}

// NewActivityService creates a new activity service
func NewActivityService() *ActivityService {
	return &ActivityService{
		activityRepo: repository.NewActivityRepository(),
		userRepo:     repository.NewUserRepository(),
	}
}

// GetActiveActivity retrieves the currently active activity with teams and vote counts
func (s *ActivityService) GetActiveActivity(ctx context.Context) (*models.ActivityWithTeams, error) {
	activity, err := s.activityRepo.GetActiveActivity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active activity: %v", err)
	}

	if activity == nil {
		return nil, fmt.Errorf("no active activity found")
	}

	teamsWithVotes, err := s.activityRepo.GetTeamsWithVoteCount(ctx, activity.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams with vote counts: %v", err)
	}

	return &models.ActivityWithTeams{
		Activity: *activity,
		Teams:    teamsWithVotes,
	}, nil
}

// SubmitVote submits a vote for a team
func (s *ActivityService) SubmitVote(ctx context.Context, userID, teamID, activityID string) (*models.VoteResponse, error) {
	// Check if user has already voted
	hasVoted, err := s.activityRepo.CheckUserHasVoted(ctx, userID, activityID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if user has voted: %v", err)
	}

	if hasVoted {
		return nil, fmt.Errorf("user has already voted in this activity")
	}

	// Create the vote
	vote := &models.Vote{
		UserID:     userID,
		TeamID:     teamID,
		ActivityID: activityID,
	}

	err = s.activityRepo.CreateVote(ctx, vote)
	if err != nil {
		return nil, fmt.Errorf("failed to create vote: %v", err)
	}

	// Get team name for response
	teams, err := s.activityRepo.GetTeams(ctx, activityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams: %v", err)
	}

	var teamName string
	for _, team := range teams {
		if team.ID == teamID {
			teamName = team.DisplayName
			break
		}
	}

	// Get total vote count for the activity
	totalVotes, err := s.activityRepo.GetVoteCount(ctx, activityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total vote count: %v", err)
	}

	return &models.VoteResponse{
		Vote:       *vote,
		Message:    "Vote submitted successfully!",
		TeamName:   teamName,
		TotalVotes: totalVotes,
	}, nil
}

// GetUserProfile retrieves user profile with voting information
func (s *ActivityService) GetUserProfile(ctx context.Context, userID string) (*models.UserProfileResponse, error) {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Get active activity with teams
	activityWithTeams, err := s.GetActiveActivity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active activity: %v", err)
	}

	if activityWithTeams == nil {
		return &models.UserProfileResponse{
			User:           user,
			HasVoted:       false,
			ActivityStatus: "No active activity",
		}, nil
	}

	// Check if user has voted
	hasVoted, err := s.activityRepo.CheckUserHasVoted(ctx, userID, activityWithTeams.Activity.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check if user has voted: %v", err)
	}

	var votedTeamName *string
	var votedTeamID *string

	if hasVoted {
		userVote, err := s.activityRepo.GetUserVote(ctx, userID, activityWithTeams.Activity.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user vote: %v", err)
		}

		if userVote != nil {
			// Find team name
			for _, team := range activityWithTeams.Teams {
				if team.Team.ID == userVote.TeamID {
					votedTeamName = &team.Team.DisplayName
					votedTeamID = &team.Team.ID
					break
				}
			}
		}
	}

	// Calculate time remaining
	timeRemaining := ""
	if activityWithTeams.Activity.EndDate.After(time.Now()) {
		duration := activityWithTeams.Activity.EndDate.Sub(time.Now())
		days := int(duration.Hours() / 24)
		hours := int(duration.Hours()) % 24
		minutes := int(duration.Minutes()) % 60

		if days > 0 {
			timeRemaining = fmt.Sprintf("%d days, %d hours, %d minutes", days, hours, minutes)
		} else if hours > 0 {
			timeRemaining = fmt.Sprintf("%d hours, %d minutes", hours, minutes)
		} else {
			timeRemaining = fmt.Sprintf("%d minutes", minutes)
		}
	}

	// Determine activity status based on dates
	activityStatus := "Active"
	if time.Now().After(activityWithTeams.Activity.EndDate) {
		activityStatus = "Ended"
	} else if time.Now().Before(activityWithTeams.Activity.StartDate) {
		activityStatus = "Upcoming"
	}

	return &models.UserProfileResponse{
		User:           user,
		HasVoted:       hasVoted,
		VotedTeamName:  votedTeamName,
		VotedTeamID:    votedTeamID,
		ActivityStatus: activityStatus,
		TimeRemaining:  timeRemaining,
	}, nil
}

// GetTeams retrieves teams for an activity
func (s *ActivityService) GetTeams(ctx context.Context, activityID string) ([]models.Team, error) {
	return s.activityRepo.GetTeams(ctx, activityID)
}

// GetTeamsWithVoteCount retrieves teams with their vote counts
func (s *ActivityService) GetTeamsWithVoteCount(ctx context.Context, activityID string) ([]models.TeamWithVoteCount, error) {
	return s.activityRepo.GetTeamsWithVoteCount(ctx, activityID)
}

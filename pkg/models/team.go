package models

import (
	"time"
)

// HardcodedTeam represents a hardcoded team definition (without database fields)
type HardcodedTeam struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description,omitempty"`
	ImageURL     string `json:"image_url,omitempty"`
	DisplayOrder int    `json:"display_order"`
}

// TeamWithVotes represents a team with vote count (alias for consistency)
type TeamWithVotes = TeamWithVoteCount

// VotingStatus represents a user's voting status for an activity
type VotingStatus struct {
	HasVoted    bool      `json:"has_voted"`
	VotedTeamID string    `json:"voted_team_id,omitempty"`
	VotedAt     time.Time `json:"voted_at,omitempty"`
}

// EnhancedVoteResponse represents an enhanced response after submitting a vote
type EnhancedVoteResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	VotedTeam Team   `json:"voted_team"`
	VoteCount int    `json:"vote_count"`
}

// EnhancedActivityWithTeams represents an activity with its teams, vote counts, and user voting status
type EnhancedActivityWithTeams struct {
	Activity Activity        `json:"activity"`
	Teams    []TeamWithVotes `json:"teams"`
	UserVote VotingStatus    `json:"user_vote"`
}

// GetHardcodedTeams returns the 6 hardcoded teams with fixed UUIDs
func GetHardcodedTeams() []Team {
	desc1 := "Team Crimson for the Ananped 8M celebration"
	desc2 := "Team Azure for the Ananped 8M celebration"
	desc3 := "Team Golden for the Ananped 8M celebration"
	desc4 := "Team Emerald for the Ananped 8M celebration"
	desc5 := "Team Purple for the Ananped 8M celebration"
	desc6 := "Team Silver for the Ananped 8M celebration"

	return []Team{
		{
			ID:          "550e8400-e29b-41d4-a716-446655440001", // Fixed UUID for Team A
			ActivityID:  "active",                               // Static activity ID
			Name:        "A",
			DisplayName: "Team Crimson",
			Description: &desc1,
		},
		{
			ID:          "550e8400-e29b-41d4-a716-446655440002", // Fixed UUID for Team B
			ActivityID:  "active",
			Name:        "B",
			DisplayName: "Team Azure",
			Description: &desc2,
		},
		{
			ID:          "550e8400-e29b-41d4-a716-446655440003", // Fixed UUID for Team C
			ActivityID:  "active",
			Name:        "C",
			DisplayName: "Team Golden",
			Description: &desc3,
		},
		{
			ID:          "550e8400-e29b-41d4-a716-446655440004", // Fixed UUID for Team D
			ActivityID:  "active",
			Name:        "D",
			DisplayName: "Team Emerald",
			Description: &desc4,
		},
		{
			ID:          "550e8400-e29b-41d4-a716-446655440005", // Fixed UUID for Team E
			ActivityID:  "active",
			Name:        "E",
			DisplayName: "Team Purple",
			Description: &desc5,
		},
		{
			ID:          "550e8400-e29b-41d4-a716-446655440006", // Fixed UUID for Team F
			ActivityID:  "active",
			Name:        "F",
			DisplayName: "Team Silver",
			Description: &desc6,
		},
	}
}

// GetHardcodedTeamDefinitions returns the hardcoded team definitions with display order
func GetHardcodedTeamDefinitions() []HardcodedTeam {
	return []HardcodedTeam{
		{
			ID:           "550e8400-e29b-41d4-a716-446655440001",
			Name:         "A",
			DisplayName:  "Team Crimson",
			Description:  "Team Crimson for the Ananped 8M celebration",
			DisplayOrder: 1,
		},
		{
			ID:           "550e8400-e29b-41d4-a716-446655440002",
			Name:         "B",
			DisplayName:  "Team Azure",
			Description:  "Team Azure for the Ananped 8M celebration",
			DisplayOrder: 2,
		},
		{
			ID:           "550e8400-e29b-41d4-a716-446655440003",
			Name:         "C",
			DisplayName:  "Team Golden",
			Description:  "Team Golden for the Ananped 8M celebration",
			DisplayOrder: 3,
		},
		{
			ID:           "550e8400-e29b-41d4-a716-446655440004",
			Name:         "D",
			DisplayName:  "Team Emerald",
			Description:  "Team Emerald for the Ananped 8M celebration",
			DisplayOrder: 4,
		},
		{
			ID:           "550e8400-e29b-41d4-a716-446655440005",
			Name:         "E",
			DisplayName:  "Team Purple",
			Description:  "Team Purple for the Ananped 8M celebration",
			DisplayOrder: 5,
		},
		{
			ID:           "550e8400-e29b-41d4-a716-446655440006",
			Name:         "F",
			DisplayName:  "Team Silver",
			Description:  "Team Silver for the Ananped 8M celebration",
			DisplayOrder: 6,
		},
	}
}

// GetTeamByID returns a team by its ID
func GetTeamByID(teamID string) (*Team, bool) {
	teams := GetHardcodedTeams()
	for _, team := range teams {
		if team.ID == teamID {
			return &team, true
		}
	}
	return nil, false
}

// ValidateTeamID checks if a team ID is valid
func ValidateTeamID(teamID string) bool {
	_, exists := GetTeamByID(teamID)
	return exists
}

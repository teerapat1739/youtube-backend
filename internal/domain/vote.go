package domain

import (
	"time"
)

// Vote represents a vote record with PDPA compliance
type Vote struct {
	ID                   string     `json:"id"`
	VoteID               string     `json:"vote_id"`
	UserID               string     `json:"user_id"`
	TeamID               int        `json:"team_id"`
	VoterName            string     `json:"voter_name"`
	VoterEmail           string     `json:"voter_email"`
	VoterPhone           string     `json:"voter_phone"`
	IPAddress            string     `json:"ip_address"`
	UserAgent            string     `json:"user_agent"`
	ConsentTimestamp     *time.Time `json:"consent_timestamp,omitempty"`
	ConsentIP            string     `json:"consent_ip,omitempty"`
	PrivacyPolicyVersion string     `json:"privacy_policy_version,omitempty"`
	PDPAConsent          bool       `json:"pdpa_consent"`
	MarketingConsent     bool       `json:"marketing_consent"`
	DataRetentionUntil   *time.Time `json:"data_retention_until,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

// VoteRequest represents a vote submission request
type VoteRequest struct {
	TeamID       int          `json:"team_id" validate:"required,min=1"`
	PersonalInfo PersonalInfo `json:"personal_info" validate:"required"`
	Consent      ConsentData  `json:"consent" validate:"required"`
}

// PersonalInfo represents voter's personal information
type PersonalInfo struct {
	FirstName string `json:"first_name" validate:"required,min=2,max=100"`
	LastName  string `json:"last_name" validate:"required,min=2,max=100"`
	Email     string `json:"email" validate:"required,email"`
	Phone     string `json:"phone" validate:"required,min=10,max=20"`
}

// ConsentData represents PDPA consent information
type ConsentData struct {
	PDPAConsent          bool   `json:"pdpa_consent" validate:"required,eq=true"`
	MarketingConsent     bool   `json:"marketing_consent"`
	PrivacyPolicyVersion string `json:"privacy_policy_version" validate:"required"`
}

// VoteResponse represents the response after voting
type VoteResponse struct {
	VoteID    string    `json:"vote_id"`
	TeamID    int       `json:"team_id"`
	TeamName  string    `json:"team_name"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// VotingStatus represents the current voting status
type VotingStatus struct {
	Teams        []TeamWithVoteStatus `json:"teams"`
	TotalVotes   int                  `json:"total_votes"`
	LastUpdate   time.Time            `json:"last_update"`
	UserHasVoted bool                 `json:"user_has_voted"`
	UserVoteID   string               `json:"user_vote_id,omitempty"`
}

// TeamResultWithRanking represents a team with its ranking and statistics for results display
type TeamResultWithRanking struct {
	Team
	Rank       int     `json:"rank"`
	Percentage float64 `json:"percentage"`
	IsWinner   bool    `json:"is_winner"`
}

// VotingResults represents comprehensive voting results with rankings and statistics
type VotingResults struct {
	Teams           []TeamResultWithRanking `json:"teams"`
	TotalVotes      int                     `json:"total_votes"`
	LastUpdate      time.Time               `json:"last_update"`
	VotingComplete  bool                    `json:"voting_complete"`
	Winner          *TeamResultWithRanking  `json:"winner,omitempty"`
	ParticipatedAt  *time.Time              `json:"participated_at,omitempty"`
	Statistics      VotingStatistics        `json:"statistics"`
}

// VotingStatistics provides additional voting statistics
type VotingStatistics struct {
	TotalParticipants int                          `json:"total_participants"`
	VotingPeriod      VotingPeriodInfo            `json:"voting_period"`
	TopTeams          []TeamResultWithRanking     `json:"top_teams"`
	Distribution      []VoteDistribution          `json:"distribution"`
}

// VotingPeriodInfo represents the voting period information
type VotingPeriodInfo struct {
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	Duration    string     `json:"duration"`
	IsActive    bool       `json:"is_active"`
}

// VoteDistribution represents vote distribution by percentage ranges
type VoteDistribution struct {
	Range      string  `json:"range"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}
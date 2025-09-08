package domain

import (
	"errors"
	"time"
)

// Common errors
var (
	ErrUserNotFound   = errors.New("user not found: personal info must be created first")
	ErrVoteFinalized  = errors.New("vote is finalized and cannot be changed")
	ErrDuplicatePhone = errors.New("this phone number has already been used")
)

// Vote represents a unified record that contains both personal info and voting data
type Vote struct {
	// Primary key and identifiers
	ID     string `json:"id"`
	UserID string `json:"user_id"` // This serves as the primary key for the unified table

	// Personal information fields
	Phone         string `json:"phone"` // Normalized phone number (unique)
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Email         string `json:"email"`
	FavoriteVideo string `json:"favorite_video,omitempty"` // User's favorite video (max 1000 chars, optional)

	// PDPA compliance fields
	ConsentPDPA          bool       `json:"consent_pdpa"`
	ConsentTimestamp     *time.Time `json:"consent_timestamp,omitempty"`
	ConsentIP            string     `json:"consent_ip,omitempty"`
	PrivacyPolicyVersion string     `json:"privacy_policy_version,omitempty"`
	MarketingConsent     bool       `json:"marketing_consent"`
	DataRetentionUntil   *time.Time `json:"data_retention_until,omitempty"`

	// Vote-specific fields
	CandidateID int        `json:"candidate_id,omitempty"` // 0 means no vote cast yet
	VotedAt     *time.Time `json:"voted_at,omitempty"`

	// Welcome/Rules acceptance fields
	WelcomeAccepted   bool       `json:"welcome_accepted"`
	WelcomeAcceptedAt *time.Time `json:"welcome_accepted_at,omitempty"`
	RulesVersion      string     `json:"rules_version,omitempty"`

	// Audit fields
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Legacy fields for compatibility (will be deprecated)
	VoteID     string `json:"vote_id,omitempty"`     // Deprecated: use ID
	TeamID     int    `json:"team_id,omitempty"`     // Deprecated: use CandidateID
	VoterName  string `json:"voter_name,omitempty"`  // Deprecated: use FirstName + LastName
	VoterEmail string `json:"voter_email,omitempty"` // Deprecated: use Email
	VoterPhone string `json:"voter_phone,omitempty"` // Deprecated: use Phone
}

// VoteRequest represents a vote submission request
type VoteRequest struct {
	TeamID       int          `json:"team_id" validate:"required,min=1"`
	PersonalInfo PersonalInfo `json:"personal_info" validate:"required"`
	Consent      ConsentData  `json:"consent" validate:"required"`
}

// PersonalInfo represents voter's personal information
type PersonalInfo struct {
	FirstName     string `json:"first_name" validate:"required,min=2,max=100"`
	LastName      string `json:"last_name" validate:"required,min=2,max=100"`
	Email         string `json:"email" validate:"required,email"`
	Phone         string `json:"phone" validate:"required,min=10,max=20"`
	FavoriteVideo string `json:"favorite_video,omitempty" validate:"omitempty,max=1000"`
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
	Teams          []TeamResultWithRanking `json:"teams"`
	TotalVotes     int                     `json:"total_votes"`
	LastUpdate     time.Time               `json:"last_update"`
	VotingComplete bool                    `json:"voting_complete"`
	Winner         *TeamResultWithRanking  `json:"winner,omitempty"`
	ParticipatedAt *time.Time              `json:"participated_at,omitempty"`
	Statistics     VotingStatistics        `json:"statistics"`
}

// VotingStatistics provides additional voting statistics
type VotingStatistics struct {
	TotalParticipants int                     `json:"total_participants"`
	VotingPeriod      VotingPeriodInfo        `json:"voting_period"`
	TopTeams          []TeamResultWithRanking `json:"top_teams"`
	Distribution      []VoteDistribution      `json:"distribution"`
}

// VotingPeriodInfo represents the voting period information
type VotingPeriodInfo struct {
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	Duration  string     `json:"duration"`
	IsActive  bool       `json:"is_active"`
}

// VoteDistribution represents vote distribution by percentage ranges
type VoteDistribution struct {
	Range      string  `json:"range"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// PersonalInfoRequest represents a request to create/update personal info
type PersonalInfoRequest struct {
	FirstName     string `json:"first_name" validate:"required,min=2,max=100"`
	LastName      string `json:"last_name" validate:"required,min=2,max=100"`
	Email         string `json:"email" validate:"required,email"`
	Phone         string `json:"phone" validate:"required,min=10,max=20"`
	FavoriteVideo string `json:"favorite_video,omitempty" validate:"omitempty,max=1000"`
	ConsentPDPA   bool   `json:"consent_pdpa" validate:"required,eq=true"`
}

// PersonalInfoResponse represents the response after creating/updating personal info
type PersonalInfoResponse struct {
	UserID        string    `json:"user_id"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	FavoriteVideo string    `json:"favorite_video,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Message       string    `json:"message"`
}

// WelcomeAcceptanceRequest represents a request to save welcome/rules acceptance
type WelcomeAcceptanceRequest struct {
	UserID       string `json:"user_id"`
	RulesVersion string `json:"rules_version" validate:"required"`
	IPAddress    string `json:"ip_address,omitempty"`
	UserAgent    string `json:"user_agent,omitempty"`
}

// WelcomeAcceptanceResponse represents the response after saving welcome acceptance
type WelcomeAcceptanceResponse struct {
	UserID            string    `json:"user_id"`
	WelcomeAccepted   bool      `json:"welcome_accepted"`
	WelcomeAcceptedAt time.Time `json:"welcome_accepted_at"`
	RulesVersion      string    `json:"rules_version"`
	Message           string    `json:"message"`
}

// VoteOnlyRequest represents a request to submit only the vote (no personal info)
type VoteOnlyRequest struct {
	UserID      string `json:"user_id" validate:"required"`
	CandidateID int    `json:"candidate_id" validate:"required,min=1"`
}

// VoteOnlyResponse represents the response after submitting a vote
type VoteOnlyResponse struct {
	UserID      string    `json:"user_id"`
	CandidateID int       `json:"candidate_id"`
	VoteID      string    `json:"vote_id,omitempty"`
	VotedAt     time.Time `json:"voted_at"`
	Message     string    `json:"message"`
}

// PersonalInfoMeResponse represents the response for GET /api/personal-info/me
type PersonalInfoMeResponse struct {
	UserID           string     `json:"user_id"`
	Phone            string     `json:"phone"`
	FirstName        string     `json:"first_name"`
	LastName         string     `json:"last_name"`
	Email            string     `json:"email"`
	FavoriteVideo    string     `json:"favorite_video,omitempty"`
	ConsentPDPA      bool       `json:"consent_pdpa"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ConsentTimestamp *time.Time `json:"consent_timestamp,omitempty"`
	MarketingConsent bool       `json:"marketing_consent"`

	// Voting status fields
	HasVoted       bool       `json:"has_voted"`
	VoteID         string     `json:"vote_id,omitempty"`
	VotedAt        *time.Time `json:"voted_at,omitempty"`
	SelectedTeamID *int       `json:"selected_team_id,omitempty"`

	// Welcome/Rules acceptance fields
	WelcomeAccepted   bool       `json:"welcome_accepted"`
	WelcomeAcceptedAt *time.Time `json:"welcome_accepted_at,omitempty"`
	RulesVersion      string     `json:"rules_version,omitempty"`
}

// UserStatusResponse represents the response for GET /api/user/status
type UserStatusResponse struct {
	UserID            string `json:"user_id"`
	WelcomeAccepted   bool   `json:"welcome_accepted"`
	HasPersonalInfo   bool   `json:"has_personal_info"`
	HasVoted          bool   `json:"has_voted"`
	CurrentStep       string `json:"current_step"` // welcome, personal-info, vote, complete
}

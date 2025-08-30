package models

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID                     string     `json:"id" db:"id"`
	GoogleID               string     `json:"google_id" db:"google_id"`
	Email                  string     `json:"email" db:"email"`
	FirstName              *string    `json:"first_name" db:"first_name"`
	LastName               *string    `json:"last_name" db:"last_name"`
	Phone                  *string    `json:"phone" db:"phone"`
	TermsAccepted          bool       `json:"terms_accepted" db:"terms_accepted"`
	TermsVersion           *string    `json:"terms_version" db:"terms_version"`
	PDPAAccepted           bool       `json:"pdpa_accepted" db:"pdpa_accepted"`
	PDPAVersion            *string    `json:"pdpa_version" db:"pdpa_version"`
	ProfileCompleted       bool       `json:"profile_completed" db:"profile_completed"`
	YouTubeSubscribed      bool       `json:"youtube_subscribed" db:"youtube_subscribed"`
	SubscriptionVerifiedAt *time.Time `json:"subscription_verified_at,omitempty" db:"subscription_verified_at"`
	// OAuth token fields for YouTube API access
	GoogleAccessToken  *string    `json:"-" db:"google_access_token"`                           // Hidden from JSON for security
	GoogleRefreshToken *string    `json:"-" db:"google_refresh_token"`                          // Hidden from JSON for security
	GoogleTokenExpiry  *time.Time `json:"-" db:"google_token_expiry"`                           // Hidden from JSON for security
	YouTubeChannelID   *string    `json:"youtube_channel_id,omitempty" db:"youtube_channel_id"` // User's YouTube channel ID
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at" db:"updated_at"`
}

// Team represents a team in an activity
type Team struct {
	ID          string    `json:"id" db:"id"`
	ActivityID  string    `json:"activity_id" db:"activity_id"`
	Name        string    `json:"name" db:"name"`
	DisplayName string    `json:"display_name" db:"display_name"`
	ImageURL    *string   `json:"image_url,omitempty" db:"image_url"`
	Description *string   `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Vote represents a user's vote for a team
type Vote struct {
	ID         string    `json:"id" db:"id"`
	UserID     string    `json:"user_id" db:"user_id"`
	TeamID     string    `json:"team_id" db:"team_id"`
	ActivityID string    `json:"activity_id" db:"activity_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// UserSession represents a user's session
type UserSession struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	SessionToken string    `json:"session_token" db:"session_token"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// TermsVersion represents a version of terms or PDPA
type TermsVersion struct {
	ID        string    `json:"id" db:"id"`
	Version   string    `json:"version" db:"version"`
	Type      string    `json:"type" db:"type"` // 'terms' or 'pdpa'
	Content   string    `json:"content" db:"content"`
	Active    bool      `json:"active" db:"active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// UserTermsAcceptance represents a user's acceptance of terms/PDPA
type UserTermsAcceptance struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	TermsVersion string    `json:"terms_version" db:"terms_version"`
	PDPAVersion  string    `json:"pdpa_version" db:"pdpa_version"`
	AcceptedAt   time.Time `json:"accepted_at" db:"accepted_at"`
	IPAddress    *string   `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent    *string   `json:"user_agent,omitempty" db:"user_agent"`
}

// TeamWithVoteCount represents a team with its vote count
type TeamWithVoteCount struct {
	Team      Team `json:"team"`
	VoteCount int  `json:"vote_count"`
}

// ActivityWithTeams represents an activity with its teams and vote counts
type ActivityWithTeams struct {
	Activity Activity            `json:"activity"`
	Teams    []TeamWithVoteCount `json:"teams"`
}

// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
	GoogleID    string `json:"google_id" validate:"required"`
	Email       string `json:"email" validate:"required,email"`
	FirstName   string `json:"first_name" validate:"required"`
	LastName    string `json:"last_name" validate:"required"`
	Phone       string `json:"phone" validate:"required"`
	AcceptTerms bool   `json:"accept_terms" validate:"required"`
	AcceptPDPA  bool   `json:"accept_pdpa" validate:"required"`
}

// UpdateUserProfileRequest represents the request to update user profile
type UpdateUserProfileRequest struct {
	FirstName   string `json:"first_name" validate:"required,min=2,max=50"`
	LastName    string `json:"last_name" validate:"required,min=2,max=50"`
	Phone       string `json:"phone" validate:"required,len=10"`
	AcceptTerms bool   `json:"accept_terms" validate:"required"`
	AcceptPDPA  bool   `json:"accept_pdpa" validate:"required"`
}

// UserProfileValidationRequest represents the request to validate profile data
type UserProfileValidationRequest struct {
	FirstName string `json:"first_name" validate:"required,min=2,max=50"`
	LastName  string `json:"last_name" validate:"required,min=2,max=50"`
	Phone     string `json:"phone" validate:"required,len=10"`
}

// AcceptTermsRequest represents the request to accept terms and PDPA
type AcceptTermsRequest struct {
	TermsVersion string `json:"terms_version" validate:"required"`
	PDPAVersion  string `json:"pdpa_version" validate:"required"`
	AcceptTerms  bool   `json:"accept_terms" validate:"required"`
	AcceptPDPA   bool   `json:"accept_pdpa" validate:"required"`
}

// CreateVoteRequest represents the request to create a vote
type CreateVoteRequest struct {
	TeamID     string `json:"team_id" validate:"required,uuid"`
	ActivityID string `json:"activity_id" validate:"required,uuid"`
}

// VoteResponse represents the response after voting
type VoteResponse struct {
	Vote       Vote   `json:"vote"`
	Message    string `json:"message"`
	TeamName   string `json:"team_name"`
	TotalVotes int    `json:"total_votes"`
}

// UserProfileResponse represents the user profile response
type UserProfileResponse struct {
	Exists         bool    `json:"exists"`
	User           *User   `json:"user,omitempty"`
	HasVoted       bool    `json:"has_voted"`
	VotedTeamName  *string `json:"voted_team_name,omitempty"`
	VotedTeamID    *string `json:"voted_team_id,omitempty"`
	ActivityStatus string  `json:"activity_status"`
	TimeRemaining  string  `json:"time_remaining,omitempty"`
}

// VoteStatusResponse represents the vote status response
type VoteStatusResponse struct {
	HasVoted      bool    `json:"has_voted"`
	VotedTeamID   *string `json:"voted_team_id"`
	VoteTimestamp *string `json:"vote_timestamp"`
}

// TermsResponse represents the terms and PDPA response
type TermsResponse struct {
	TermsVersion string `json:"terms_version"`
	TermsContent string `json:"terms_content"`
	PDPAVersion  string `json:"pdpa_version"`
	PDPAContent  string `json:"pdpa_content"`
}

// ValidationResponse represents validation response
type ValidationResponse struct {
	Valid  bool              `json:"valid"`
	Errors map[string]string `json:"errors,omitempty"`
}

// APIResponse represents a generic API response
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Message   string      `json:"message,omitempty"`
	ErrorCode string      `json:"error_code,omitempty"`
	Details   interface{} `json:"details,omitempty"`
}

// OAuthTokenData represents OAuth token information for a user
type OAuthTokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	TokenType    string    `json:"token_type"`
}

// YouTubeSubscriptionCheckRequest represents a subscription check request
type YouTubeSubscriptionCheckRequest struct {
	ChannelID string `json:"channel_id" validate:"required"`
	UserID    string `json:"user_id,omitempty"`
}

// YouTubeSubscriptionCheckResponse represents a subscription check response
type YouTubeSubscriptionCheckResponse struct {
	IsSubscribed       bool   `json:"is_subscribed"`
	ChannelID          string `json:"channel_id"`
	UserChannelID      string `json:"user_channel_id,omitempty"`
	SubscriptionID     string `json:"subscription_id,omitempty"`
	VerificationMethod string `json:"verification_method"` // "oauth", "api_key", "fallback"
}

// HealthCheckResponse represents the health check response
type HealthCheckResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version"`
	Services  map[string]string `json:"services"`
}

// ActivityRules represents activity rules with structured content
type ActivityRules struct {
	Version     string                 `json:"version"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Content     map[string]interface{} `json:"content"`
}

// ActivityRulesAcceptance represents a user's acceptance of activity rules
type ActivityRulesAcceptance struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	RulesVersion string    `json:"rules_version" db:"rules_version"`
	AcceptedAt   time.Time `json:"accepted_at" db:"accepted_at"`
	IPAddress    *string   `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent    *string   `json:"user_agent,omitempty" db:"user_agent"`
}

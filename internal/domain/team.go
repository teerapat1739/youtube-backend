package domain

import "time"

// Team represents a voting team
type Team struct {
	ID           int        `json:"id"`
	Code         string     `json:"code"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Icon         string     `json:"icon"`
	ImageFilename string    `json:"image_filename"`
	MemberCount  int        `json:"member_count"`
	IsActive     bool       `json:"is_active"`
	VoteCount    int        `json:"vote_count"`
	LastVoteAt   *time.Time `json:"last_vote_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TeamWithVoteStatus includes user's voting status
type TeamWithVoteStatus struct {
	Team
	UserHasVoted bool `json:"user_has_voted"`
}

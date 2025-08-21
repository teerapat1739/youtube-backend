package models

import (
	"time"
)

// Activity represents a promotional activity
type Activity struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ChannelID   string    `json:"channel_id"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	CreatedAt   time.Time `json:"created_at"`
}

// Participant represents a user who joined an activity
type Participant struct {
	ID         string    `json:"id"`
	ActivityID string    `json:"activity_id"`
	UserID     string    `json:"user_id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Phone      string    `json:"phone,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ContactForm represents the form data submitted by participants
type ContactForm struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone,omitempty"`
	AcceptTerms bool   `json:"accept_terms"`
}

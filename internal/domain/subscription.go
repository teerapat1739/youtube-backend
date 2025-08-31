package domain

import "time"

// SubscriptionStatus represents the status of a YouTube subscription
type SubscriptionStatus struct {
	IsSubscribed  bool      `json:"is_subscribed"`
	ChannelID     string    `json:"channel_id"`
	ChannelTitle  string    `json:"channel_title"`
	SubscribedAt  *time.Time `json:"subscribed_at,omitempty"`
	CheckedAt     time.Time `json:"checked_at"`
	UserID        string    `json:"user_id"`
}

// YouTubeChannel represents basic YouTube channel information
type YouTubeChannel struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"`
}

// SubscriptionCheckRequest represents a request to check subscription status
type SubscriptionCheckRequest struct {
	UserID    string `json:"user_id"`
	ChannelID string `json:"channel_id"`
}

// SubscriptionCheckResponse represents the response from subscription check
type SubscriptionCheckResponse struct {
	IsSubscribed bool           `json:"is_subscribed"`
	Channel      YouTubeChannel `json:"channel"`
	Message      string         `json:"message,omitempty"`
}
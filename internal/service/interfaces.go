package service

import (
	"be-v2/internal/domain"
	"context"
)

// AuthService defines the interface for authentication operations
type AuthService interface {
	// ValidateGoogleToken validates a Google OAuth token and returns user profile
	ValidateGoogleToken(ctx context.Context, token string) (*domain.UserProfile, error)

	// ValidateJWTToken validates a JWT token and returns auth claims
	ValidateJWTToken(ctx context.Context, token string) (*domain.AuthClaims, error)

	// GetUserProfile gets user profile from validated token
	GetUserProfile(ctx context.Context, userID string) (*domain.User, error)
}

// YouTubeService defines the interface for YouTube operations
type YouTubeService interface {
	// CheckSubscription checks if a user is subscribed to a specific channel
	CheckSubscription(ctx context.Context, accessToken string, channelID string) (*domain.SubscriptionCheckResponse, error)

	// GetChannelInfo gets basic information about a YouTube channel
	GetChannelInfo(ctx context.Context, channelID string) (*domain.YouTubeChannel, error)
}

// VisitorService defines the interface for visitor tracking operations
type VisitorService interface {
	// Start initializes the visitor service and begins periodic snapshots
	Start(ctx context.Context) error

	// Stop gracefully shuts down the visitor service
	Stop(ctx context.Context) error

	// RecordVisit records a visit from the given IP address and user agent
	RecordVisit(ctx context.Context, ipAddress, userAgent string) (*domain.RateLimitInfo, error)

	// GetStats retrieves current visitor statistics
	GetStats(ctx context.Context) (*domain.VisitorStats, error)
}

// Services aggregates all service interfaces
type Services struct {
	Auth    AuthService
	YouTube YouTubeService
	Visitor VisitorService
}

package service

import (
	"context"
	"be-v2/internal/domain"
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

// Services aggregates all service interfaces
type Services struct {
	Auth    AuthService
	YouTube YouTubeService
}
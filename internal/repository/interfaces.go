package repository

import (
	"be-v2/internal/domain"
	"context"
)

// UserRepository defines the interface for user data operations
type UserRepository interface {
	// GetByID retrieves a user by ID
	GetByID(ctx context.Context, id string) (*domain.User, error)

	// GetByEmail retrieves a user by email
	GetByEmail(ctx context.Context, email string) (*domain.User, error)

	// Create creates a new user
	Create(ctx context.Context, user *domain.User) error

	// Update updates an existing user
	Update(ctx context.Context, user *domain.User) error
}

// SubscriptionRepository defines the interface for subscription data operations
type SubscriptionRepository interface {
	// GetByUserAndChannel retrieves subscription status for a user and channel
	GetByUserAndChannel(ctx context.Context, userID, channelID string) (*domain.SubscriptionStatus, error)

	// Create creates a new subscription record
	Create(ctx context.Context, subscription *domain.SubscriptionStatus) error

	// Update updates an existing subscription record
	Update(ctx context.Context, subscription *domain.SubscriptionStatus) error
}

// Repositories aggregates all repository interfaces
type Repositories struct {
	User         UserRepository
	Subscription SubscriptionRepository
}

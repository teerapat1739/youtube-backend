package repository

import (
	"be-v2/internal/domain"
	"context"
	"time"
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

// VisitorRepository defines the interface for visitor snapshot operations
type VisitorRepository interface {
	// CreateSnapshot creates a new visitor snapshot in the database
	CreateSnapshot(ctx context.Context, snapshot *domain.VisitorSnapshot) error

	// GetLatestSnapshot retrieves the most recent visitor snapshot
	GetLatestSnapshot(ctx context.Context) (*domain.VisitorSnapshot, error)

	// GetSnapshotByDate retrieves a visitor snapshot for a specific date
	GetSnapshotByDate(ctx context.Context, date time.Time) (*domain.VisitorSnapshot, error)

	// GetHistoricalSnapshots retrieves visitor snapshots within a date range
	GetHistoricalSnapshots(ctx context.Context, startDate, endDate time.Time, limit int) ([]*domain.VisitorSnapshot, error)

	// DeleteOldSnapshots removes snapshots older than the specified retention period
	DeleteOldSnapshots(ctx context.Context, retentionDays int) (int64, error)

	// GetSnapshotCount returns the total number of snapshots in the database
	GetSnapshotCount(ctx context.Context) (int64, error)
}

// Repositories aggregates all repository interfaces
type Repositories struct {
	User         UserRepository
	Subscription SubscriptionRepository
	Visitor      VisitorRepository
}

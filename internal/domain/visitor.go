package domain

import (
	"time"
)

// VisitorSnapshot represents a snapshot of visitor statistics stored in PostgreSQL
type VisitorSnapshot struct {
	ID           int64     `json:"id" db:"id"`
	TotalVisits  int64     `json:"total_visits" db:"total_visits"`
	DailyVisits  int64     `json:"daily_visits" db:"daily_visits"`
	UniqueVisits int64     `json:"unique_visits" db:"unique_visits"`
	SnapshotDate time.Time `json:"snapshot_date" db:"snapshot_date"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// VisitorStats represents real-time visitor statistics from Redis
type VisitorStats struct {
	TotalVisits  int64     `json:"total_visits"`
	DailyVisits  int64     `json:"daily_visits"`
	UniqueVisits int64     `json:"unique_visits"`
	LastUpdated  time.Time `json:"last_updated"`
}

// VisitRequest represents a request to record a visit
type VisitRequest struct {
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Timestamp time.Time `json:"timestamp"`
}

// RateLimitInfo represents rate limiting information
type RateLimitInfo struct {
	IPAddress    string        `json:"ip_address"`
	RequestCount int64         `json:"request_count"`
	WindowStart  time.Time     `json:"window_start"`
	TTL          time.Duration `json:"ttl"`
	IsAllowed    bool          `json:"is_allowed"`
}
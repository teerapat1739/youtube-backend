package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"be-v2/internal/domain"
	"be-v2/internal/repository"
	"be-v2/pkg/logger"
	"be-v2/pkg/redis"
)

// Redis keys for visitor tracking
const (
	KeyVisitorTotal       = "visitor:total"
	KeyVisitorDaily       = "visitor:daily:%s"        // visitor:daily:2024-01-15
	KeyVisitorUnique      = "visitor:unique"          // Set of unique visitor hashes
	KeyVisitorUniqueDaily = "visitor:unique:daily:%s" // visitor:unique:daily:2024-01-15
	KeyVisitorRateLimit   = "visitor:ratelimit:%s"    // visitor:ratelimit:ip_hash
	KeyVisitorLastUpdate  = "visitor:last_update"
)

// TTL constants for visitor tracking
const (
	TTLVisitorDaily       = 25 * time.Hour  // Daily counters (kept slightly longer than 24h)
	TTLVisitorUnique      = 7 * 24 * time.Hour // Unique visitors (1 week)
	TTLVisitorUniqueDaily = 25 * time.Hour  // Daily unique visitors
	TTLVisitorRateLimit   = 1 * time.Hour   // Rate limiting window
	TTLVisitorLastUpdate  = 24 * time.Hour  // Last update timestamp
)

// Rate limiting constants
const (
	RateLimitWindow   = 1 * time.Hour // Rate limit window
	RateLimitRequests = 60            // Max requests per window per IP
)

// visitorService handles visitor tracking with Redis and PostgreSQL snapshots
type visitorService struct {
	redisClient   *redis.Client
	visitorRepo   repository.VisitorRepository
	voteRepo      *repository.VoteRepository
	logger        *logger.Logger
	snapshotTicker *time.Ticker
	stopSnapshot   chan struct{}
	mu            sync.RWMutex
	isRunning     bool
	keyPrefix     string // Environment-specific key prefix
}

// NewVisitorService creates a new visitor service
func NewVisitorService(redisClient *redis.Client, visitorRepo repository.VisitorRepository, voteRepo *repository.VoteRepository, logger *logger.Logger, environment string) VisitorService {
	service := &visitorService{
		redisClient:  redisClient,
		visitorRepo:  visitorRepo,
		voteRepo:     voteRepo,
		logger:       logger,
		stopSnapshot: make(chan struct{}),
		keyPrefix:    redisClient.KeyBuilder.GetPrefix(),
	}

	logger.WithField("key_prefix", service.keyPrefix).Info("Initialized visitor service with environment prefix")
	
	return service
}

// buildKey constructs a Redis key with the environment prefix (deprecated - use KeyBuilder)
func (s *visitorService) buildKey(key string) string {
	return s.redisClient.KeyBuilder.BuildKey(key)
}

// Start initializes the visitor service and begins periodic snapshots
func (s *visitorService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return nil
	}

	s.logger.Info("Starting visitor service...")

	// Restore from last SQL snapshot if Redis is empty
	if err := s.restoreFromSnapshot(ctx); err != nil {
		s.logger.WithError(err).Warn("Failed to restore from snapshot, continuing with fresh counters")
	}

	// Start periodic snapshot routine (every 30 seconds)
	s.snapshotTicker = time.NewTicker(30 * time.Second)
	go s.snapshotRoutine(ctx)

	s.isRunning = true
	s.logger.Info("Visitor service started successfully")
	return nil
}

// Stop gracefully shuts down the visitor service
func (s *visitorService) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return nil
	}

	s.logger.Info("Stopping visitor service...")

	// Stop the snapshot ticker
	if s.snapshotTicker != nil {
		s.snapshotTicker.Stop()
	}

	// Signal the snapshot routine to stop
	close(s.stopSnapshot)

	// Save final snapshot
	if err := s.saveSnapshot(ctx); err != nil {
		s.logger.WithError(err).Error("Failed to save final snapshot during shutdown")
	}

	s.isRunning = false
	s.logger.Info("Visitor service stopped")
	return nil
}

// RecordVisit records a visit from the given IP address
func (s *visitorService) RecordVisit(ctx context.Context, ipAddress, userAgent string) (*domain.RateLimitInfo, error) {
	// Check rate limiting first
	rateLimitInfo, err := s.checkRateLimit(ctx, ipAddress)
	if err != nil {
		s.logger.WithError(err).Error("Failed to check rate limit")
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	if !rateLimitInfo.IsAllowed {
		s.logger.WithFields(map[string]interface{}{
			"ip":            ipAddress,
			"request_count": rateLimitInfo.RequestCount,
		}).Warn("Rate limit exceeded")
		return rateLimitInfo, nil
	}

	// Create visitor hash for uniqueness tracking
	visitorHash := s.createVisitorHash(ipAddress, userAgent)
	today := time.Now().Format("2006-01-02")

	// Use Redis pipeline for atomic operations
	pipe := s.redisClient.Pipeline()

	// Increment total visits
	pipe.Incr(ctx, s.redisClient.KeyBuilder.KeyVisitorTotal())

	// Increment daily visits
	dailyKey := s.redisClient.KeyBuilder.KeyVisitorDaily(today)
	pipe.Incr(ctx, dailyKey)
	pipe.Expire(ctx, dailyKey, TTLVisitorDaily)

	// Track unique visitors (global and daily)
	uniqueKey := s.redisClient.KeyBuilder.KeyVisitorUnique()
	uniqueDailyKey := s.redisClient.KeyBuilder.KeyVisitorUniqueDaily(today)

	// Add to unique visitor sets
	pipe.SAdd(ctx, uniqueKey, visitorHash)
	pipe.Expire(ctx, uniqueKey, TTLVisitorUnique)

	pipe.SAdd(ctx, uniqueDailyKey, visitorHash)
	pipe.Expire(ctx, uniqueDailyKey, TTLVisitorUniqueDaily)

	// Update last update timestamp
	pipe.Set(ctx, s.redisClient.KeyBuilder.KeyVisitorLastUpdate(), time.Now().Unix(), TTLVisitorLastUpdate)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to record visit")
		return nil, fmt.Errorf("failed to record visit: %w", err)
	}

	s.logger.WithFields(map[string]interface{}{
		"ip":           ipAddress,
		"visitor_hash": visitorHash[:8] + "...",
	}).Debug("Visit recorded successfully")

	return rateLimitInfo, nil
}

// GetStats retrieves current visitor statistics (now returns vote count from database)
func (s *visitorService) GetStats(ctx context.Context) (*domain.VisitorStats, error) {
	// Get total vote count from the database
	totalVoteCount, err := s.voteRepo.GetTotalVoteCount(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get total vote count")
		return nil, fmt.Errorf("failed to get total vote count: %w", err)
	}

	// Create stats object with vote count as total visits
	// Keep the same structure to maintain frontend compatibility
	stats := &domain.VisitorStats{
		TotalVisits:  int64(totalVoteCount), // Use vote count as total visits
		DailyVisits:  0,                     // Not tracking daily votes currently
		UniqueVisits: int64(totalVoteCount), // Use vote count as unique visits for consistency
		LastUpdated:  time.Now(),
	}

	s.logger.WithFields(map[string]interface{}{
		"total_vote_count": totalVoteCount,
	}).Debug("Vote count retrieved successfully")

	return stats, nil
}

// checkRateLimit checks if the IP address is within rate limits
func (s *visitorService) checkRateLimit(ctx context.Context, ipAddress string) (*domain.RateLimitInfo, error) {
	ipHash := s.createIPHash(ipAddress)
	rateLimitKey := s.redisClient.KeyBuilder.KeyVisitorRateLimit(ipHash)

	// Increment the counter for this IP
	count, err := s.redisClient.Incr(ctx, rateLimitKey)
	if err != nil {
		return nil, fmt.Errorf("failed to increment rate limit counter: %w", err)
	}

	// Set expiry on first request
	if count == 1 {
		err := s.redisClient.Expire(ctx, rateLimitKey, TTLVisitorRateLimit)
		if err != nil {
			s.logger.WithError(err).Warn("Failed to set rate limit key expiry")
		}
	}

	rateLimitInfo := &domain.RateLimitInfo{
		IPAddress:    ipAddress,
		RequestCount: count,
		WindowStart:  time.Now().Truncate(RateLimitWindow),
		TTL:          TTLVisitorRateLimit,
		IsAllowed:    count <= RateLimitRequests,
	}

	return rateLimitInfo, nil
}

// restoreFromSnapshot restores counters from the latest SQL snapshot if Redis is empty
func (s *visitorService) restoreFromSnapshot(ctx context.Context) error {
	// Check if Redis already has data
	exists, err := s.redisClient.Exists(ctx, s.redisClient.KeyBuilder.KeyVisitorTotal())
	if err != nil {
		return fmt.Errorf("failed to check if Redis has visitor data: %w", err)
	}

	if exists > 0 {
		s.logger.Info("Redis already contains visitor data, skipping restore")
		return nil
	}

	// Get latest snapshot from database
	snapshot, err := s.visitorRepo.GetLatestSnapshot(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	// If no snapshot exists, initialize with zeros
	if snapshot == nil {
		s.logger.Info("No visitor snapshot found, initializing with zero counters")
		pipe := s.redisClient.Pipeline()
		pipe.Set(ctx, s.redisClient.KeyBuilder.KeyVisitorTotal(), 0, 0)
		pipe.Set(ctx, s.redisClient.KeyBuilder.KeyVisitorLastUpdate(), time.Now().Unix(), TTLVisitorLastUpdate)
		_, err = pipe.Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to initialize Redis counters: %w", err)
		}
		return nil
	}

	// Restore to Redis
	pipe := s.redisClient.Pipeline()

	pipe.Set(ctx, s.redisClient.KeyBuilder.KeyVisitorTotal(), snapshot.TotalVisits, 0) // No expiry for total
	
	// Only restore daily count if it's for today
	today := time.Now().Format("2006-01-02")
	if snapshot.SnapshotDate.Format("2006-01-02") == today {
		dailyKey := s.redisClient.KeyBuilder.KeyVisitorDaily(today)
		pipe.Set(ctx, dailyKey, snapshot.DailyVisits, TTLVisitorDaily)
	}

	pipe.Set(ctx, s.redisClient.KeyBuilder.KeyVisitorLastUpdate(), time.Now().Unix(), TTLVisitorLastUpdate)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to restore snapshot to Redis: %w", err)
	}

	s.logger.WithFields(map[string]interface{}{
		"total_visits":  snapshot.TotalVisits,
		"daily_visits":  snapshot.DailyVisits,
		"unique_visits": snapshot.UniqueVisits,
		"snapshot_date": snapshot.SnapshotDate,
	}).Info("Successfully restored visitor data from snapshot")

	return nil
}

// saveSnapshot saves current Redis counters to PostgreSQL
func (s *visitorService) saveSnapshot(ctx context.Context) error {
	stats, err := s.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current stats: %w", err)
	}

	snapshot := &domain.VisitorSnapshot{
		TotalVisits:  stats.TotalVisits,
		DailyVisits:  stats.DailyVisits,
		UniqueVisits: stats.UniqueVisits,
		SnapshotDate: time.Now(),
		CreatedAt:    time.Now(),
	}

	err = s.visitorRepo.CreateSnapshot(ctx, snapshot)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	s.logger.WithFields(map[string]interface{}{
		"total_visits":  snapshot.TotalVisits,
		"daily_visits":  snapshot.DailyVisits,
		"unique_visits": snapshot.UniqueVisits,
	}).Debug("Visitor snapshot saved successfully")

	return nil
}

// snapshotRoutine runs periodic snapshots
func (s *visitorService) snapshotRoutine(ctx context.Context) {
	for {
		select {
		case <-s.snapshotTicker.C:
			if err := s.saveSnapshot(ctx); err != nil {
				s.logger.WithError(err).Error("Failed to save periodic snapshot")
			}
		case <-s.stopSnapshot:
			s.logger.Debug("Snapshot routine stopped")
			return
		case <-ctx.Done():
			s.logger.Debug("Snapshot routine cancelled")
			return
		}
	}
}

// createVisitorHash creates a hash for visitor uniqueness tracking
func (s *visitorService) createVisitorHash(ipAddress, userAgent string) string {
	hash := sha256.Sum256([]byte(ipAddress + "|" + userAgent))
	return fmt.Sprintf("%x", hash)
}

// createIPHash creates a hash for IP address (for rate limiting privacy)
func (s *visitorService) createIPHash(ipAddress string) string {
	hash := sha256.Sum256([]byte(ipAddress))
	return fmt.Sprintf("%x", hash)[:16] // Use first 16 chars for shorter key
}
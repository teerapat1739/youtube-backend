package services

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/gamemini/youtube/pkg/cache"
	"github.com/gamemini/youtube/pkg/repository"
)

// VoteCountService handles vote count operations with minimal cache-aside pattern
type VoteCountService struct {
	voteRepo *repository.VoteRepository
}

// NewVoteCountService creates a new vote count service instance
func NewVoteCountService() *VoteCountService {
	return &VoteCountService{
		voteRepo: repository.NewVoteRepository(),
	}
}

// GetCounts retrieves vote counts for an activity using DB-read-then-cache strategy
func (s *VoteCountService) GetCounts(ctx context.Context, activityID string) (map[string]int64, string, error) {
	if activityID == "" {
		activityID = "active"
	}

	key := fmt.Sprintf("vote_counts:%s", activityID)

	// Try Redis first if available
	if cachedData, err := cache.HGetAll(ctx, key); err == nil && len(cachedData) > 0 {
		counts := make(map[string]int64, len(cachedData))
		for teamID, countStr := range cachedData {
			if count, parseErr := strconv.ParseInt(countStr, 10, 64); parseErr == nil {
				counts[teamID] = count
			}
		}
		return counts, "redis", nil
	}

	// Cache miss - fetch from database
	counts, err := s.voteRepo.GetCountsByActivity(ctx, activityID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get vote counts from database: %w", err)
	}

	// Cache the result with jittered TTL (fire-and-forget)
	go func() {
		// Create jittered TTL (9-11 seconds) to prevent thundering herd
		jitter := time.Duration(rand.Intn(3)) * time.Second
		ttl := 9*time.Second + jitter

		if err := cache.HSetAllWithTTL(context.Background(), key, counts, ttl); err != nil {
			log.Printf("⚠️ Failed to cache vote counts: %v", err)
		}
	}()

	return counts, "db", nil
}

// Increment increments the vote count for a team in an activity (best-effort cache update)
// DEPRECATED: This method is kept for backward compatibility but should not be used
// Use InvalidateCache() instead for the new DB-read-then-cache strategy
func (s *VoteCountService) Increment(ctx context.Context, activityID, teamID string) error {
	if activityID == "" {
		activityID = "active"
	}

	key := fmt.Sprintf("vote_counts:%s", activityID)

	// Best-effort increment in cache
	_ = cache.HIncrBy(ctx, key, teamID, 1) // ignore errors for best-effort

	return nil
}

// InvalidateCache is deprecated for high-concurrency scenarios
// With 100k+ concurrent requests, use pure TTL-based expiration instead
func (s *VoteCountService) InvalidateCache(ctx context.Context, activityID string) {
	// No-op - let TTL handle cache expiration naturally
	// This prevents thundering herd issues with high concurrency
}

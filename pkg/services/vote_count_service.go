package services

import (
	"context"
	"fmt"
	"strconv"

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

// GetCounts retrieves vote counts for an activity using simple cache-aside pattern
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
	
	// Fetch from database
	counts, err := s.voteRepo.GetCountsByActivity(ctx, activityID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get vote counts from database: %w", err)
	}
	
	// Best-effort cache population
	if len(counts) > 0 {
		_ = cache.HSetAll(ctx, key, counts) // ignore errors for best-effort
	}
	
	return counts, "db", nil
}

// Increment increments the vote count for a team in an activity (best-effort cache update)
func (s *VoteCountService) Increment(ctx context.Context, activityID, teamID string) error {
	if activityID == "" {
		activityID = "active"
	}
	
	key := fmt.Sprintf("vote_counts:%s", activityID)
	
	// Best-effort increment in cache
	_ = cache.HIncrBy(ctx, key, teamID, 1) // ignore errors for best-effort
	
	return nil
}
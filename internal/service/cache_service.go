package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"be-v2/internal/domain"
	"be-v2/pkg/redis"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// CacheService provides advanced caching patterns with error handling and metrics
type CacheService struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewCacheService creates a new cache service
func NewCacheService(redisClient *redis.Client, logger *zap.Logger) *CacheService {
	return &CacheService{
		redis:  redisClient,
		logger: logger,
	}
}

// GetTeamWithCache retrieves team data with cache-aside pattern and comprehensive error handling
func (c *CacheService) GetTeamWithCache(ctx context.Context, teamID int, dbFallback func(ctx context.Context, id int) (*domain.Team, error)) (*domain.Team, error) {
	cacheKey := fmt.Sprintf(redis.KeyTeamByID, teamID)
	
	// Try cache first
	cachedData, err := c.redis.Get(ctx, cacheKey)
	if err == nil && cachedData != "" {
		var team domain.Team
		if marshalErr := json.Unmarshal([]byte(cachedData), &team); marshalErr == nil {
			c.logger.Debug("Team cache hit", zap.Int("team_id", teamID))
			return &team, nil
		} else {
			// Log cache corruption but continue to database
			c.logger.Warn("Team cache corrupted, falling back to database", 
				zap.Int("team_id", teamID), 
				zap.Error(marshalErr))
		}
	} else if err != nil {
		// Log cache error but continue to database
		c.logger.Warn("Team cache error, falling back to database", 
			zap.Int("team_id", teamID), 
			zap.Error(err))
	}

	// Cache miss or error - get from database
	c.logger.Debug("Team cache miss", zap.Int("team_id", teamID))
	team, err := dbFallback(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("database fallback failed: %w", err)
	}

	// Cache the result asynchronously (fire and forget)
	if team != nil {
		go c.cacheTeamAsync(teamID, team)
	}

	return team, nil
}

// CheckPhoneUsageWithCache checks if a phone number has been used with cache-first pattern
func (c *CacheService) CheckPhoneUsageWithCache(ctx context.Context, normalizedPhone string, dbFallback func(ctx context.Context, phone string) (bool, error)) (bool, error) {
	cacheKey := fmt.Sprintf(redis.KeyPhoneVoted, normalizedPhone)
	
	// Check cache first
	exists, err := c.redis.Exists(ctx, cacheKey)
	if err == nil && exists > 0 {
		c.logger.Debug("Phone cache hit", zap.String("phone_hash", c.hashPhoneForLog(normalizedPhone)))
		return true, nil
	} else if err != nil {
		// Log cache error but continue to database
		c.logger.Warn("Phone cache error, falling back to database", 
			zap.String("phone_hash", c.hashPhoneForLog(normalizedPhone)), 
			zap.Error(err))
	}

	// Cache miss or error - check database
	c.logger.Debug("Phone cache miss", zap.String("phone_hash", c.hashPhoneForLog(normalizedPhone)))
	isUsed, err := dbFallback(ctx, normalizedPhone)
	if err != nil {
		return false, fmt.Errorf("database fallback failed: %w", err)
	}

	// Cache the result asynchronously if phone is used
	if isUsed {
		go c.cachePhoneUsageAsync(normalizedPhone)
	}

	return isUsed, nil
}

// CacheVoteSubmission caches vote-related data after successful submission
func (c *CacheService) CacheVoteSubmission(ctx context.Context, userID, normalizedPhone string, teamID int) error {
	userKey := fmt.Sprintf(redis.KeyUserVoted, userID)
	phoneKey := fmt.Sprintf(redis.KeyPhoneVoted, normalizedPhone)
	
	// Use pipeline for atomic caching
	pipe := c.redis.Pipeline()
	pipe.Set(ctx, userKey, teamID, redis.TTLUserVote)
	pipe.Set(ctx, phoneKey, "1", redis.TTLPhoneVote)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		c.logger.Error("Failed to cache vote submission", 
			zap.String("user_id", userID),
			zap.String("phone_hash", c.hashPhoneForLog(normalizedPhone)),
			zap.Int("team_id", teamID),
			zap.Error(err))
		return err
	}
	
	c.logger.Debug("Vote submission cached successfully", 
		zap.String("user_id", userID),
		zap.Int("team_id", teamID))
	return nil
}

// InvalidateVotingCaches invalidates all relevant caches after vote submission
func (c *CacheService) InvalidateVotingCaches(teamID int) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Keys to invalidate
		keysToDelete := []string{
			redis.KeyTeamsAll,
			redis.KeyVoteSummary,
			fmt.Sprintf(redis.KeyTeamCount, teamID),
		}

		// Delete specific keys
		if err := c.redis.Delete(ctx, keysToDelete...); err != nil {
			c.logger.Error("Failed to invalidate cache keys", 
				zap.Strings("keys", keysToDelete),
				zap.Error(err))
		}

		// Invalidate ETag pattern caches
		if err := c.redis.InvalidatePattern(ctx, "voting:etag:*"); err != nil {
			c.logger.Error("Failed to invalidate ETag pattern", zap.Error(err))
		}

		c.logger.Debug("Vote caches invalidated", zap.Int("team_id", teamID))
	}()
}

// HealthCheck performs a health check on the cache system
func (c *CacheService) HealthCheck(ctx context.Context) error {
	start := time.Now()
	err := c.redis.Health(ctx)
	duration := time.Since(start)
	
	if err != nil {
		c.logger.Error("Cache health check failed", 
			zap.Duration("duration", duration),
			zap.Error(err))
		return err
	}
	
	c.logger.Debug("Cache health check passed", zap.Duration("duration", duration))
	return nil
}

// cacheTeamAsync caches team data asynchronously
func (c *CacheService) cacheTeamAsync(teamID int, team *domain.Team) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	cacheKey := fmt.Sprintf(redis.KeyTeamByID, teamID)
	teamData, err := json.Marshal(team)
	if err != nil {
		c.logger.Error("Failed to marshal team for caching", 
			zap.Int("team_id", teamID),
			zap.Error(err))
		return
	}
	
	if err := c.redis.Set(ctx, cacheKey, string(teamData), redis.TTLTeamByID); err != nil {
		c.logger.Error("Failed to cache team data", 
			zap.Int("team_id", teamID),
			zap.Error(err))
	} else {
		c.logger.Debug("Team cached successfully", zap.Int("team_id", teamID))
	}
}

// cachePhoneUsageAsync caches phone usage asynchronously
func (c *CacheService) cachePhoneUsageAsync(normalizedPhone string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	cacheKey := fmt.Sprintf(redis.KeyPhoneVoted, normalizedPhone)
	if err := c.redis.Set(ctx, cacheKey, "1", redis.TTLPhoneVote); err != nil {
		c.logger.Error("Failed to cache phone usage", 
			zap.String("phone_hash", c.hashPhoneForLog(normalizedPhone)),
			zap.Error(err))
	} else {
		c.logger.Debug("Phone usage cached successfully", 
			zap.String("phone_hash", c.hashPhoneForLog(normalizedPhone)))
	}
}

// GetSubscriptionWithCache retrieves subscription status with cache-aside pattern
func (c *CacheService) GetSubscriptionWithCache(ctx context.Context, userID, channelID string, fallback func(ctx context.Context, accessToken, channelID string) (*domain.SubscriptionCheckResponse, error), accessToken string) (*domain.SubscriptionCheckResponse, error) {
	cacheKey := fmt.Sprintf(redis.KeySubscriptionCheck, userID, channelID)
	
	// Try cache first
	cachedData, err := c.redis.Get(ctx, cacheKey)
	if err == nil && cachedData != "" {
		var subscription domain.SubscriptionCheckResponse
		if marshalErr := json.Unmarshal([]byte(cachedData), &subscription); marshalErr == nil {
			c.logger.Debug("Subscription cache hit", 
				zap.String("user_id", userID),
				zap.String("channel_id", channelID))
			return &subscription, nil
		} else {
			// Log cache corruption but continue to YouTube API
			c.logger.Warn("Subscription cache corrupted, falling back to YouTube API", 
				zap.String("user_id", userID),
				zap.String("channel_id", channelID),
				zap.Error(marshalErr))
		}
	} else if err != nil && err != goredis.Nil {
		// Log cache error but continue to YouTube API (ignore Nil errors as they're expected for cache misses)
		c.logger.Warn("Subscription cache error, falling back to YouTube API", 
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
			zap.Error(err))
	}

	// Cache miss or error - get from YouTube API
	c.logger.Debug("Subscription cache miss", 
		zap.String("user_id", userID),
		zap.String("channel_id", channelID))
	
	subscription, err := fallback(ctx, accessToken, channelID)
	if err != nil {
		return nil, fmt.Errorf("YouTube API fallback failed: %w", err)
	}

	// Cache the result asynchronously (fire and forget)
	if subscription != nil {
		go c.cacheSubscriptionAsync(userID, channelID, subscription)
	}

	return subscription, nil
}

// InvalidateSubscriptionCache removes subscription cache for a specific user and channel
func (c *CacheService) InvalidateSubscriptionCache(ctx context.Context, userID, channelID string) error {
	cacheKey := fmt.Sprintf(redis.KeySubscriptionCheck, userID, channelID)
	
	if err := c.redis.Delete(ctx, cacheKey); err != nil {
		c.logger.Error("Failed to invalidate subscription cache", 
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
			zap.Error(err))
		return err
	}
	
	c.logger.Debug("Subscription cache invalidated", 
		zap.String("user_id", userID),
		zap.String("channel_id", channelID))
	return nil
}

// InvalidateUserSubscriptionCaches removes all subscription caches for a specific user
func (c *CacheService) InvalidateUserSubscriptionCaches(ctx context.Context, userID string) error {
	pattern := fmt.Sprintf(redis.KeySubscriptionCheck, userID, "*")
	
	if err := c.redis.InvalidatePattern(ctx, pattern); err != nil {
		c.logger.Error("Failed to invalidate user subscription caches", 
			zap.String("user_id", userID),
			zap.Error(err))
		return err
	}
	
	c.logger.Debug("User subscription caches invalidated", zap.String("user_id", userID))
	return nil
}

// cacheSubscriptionAsync caches subscription data asynchronously
func (c *CacheService) cacheSubscriptionAsync(userID, channelID string, subscription *domain.SubscriptionCheckResponse) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	cacheKey := fmt.Sprintf(redis.KeySubscriptionCheck, userID, channelID)
	subscriptionData, err := json.Marshal(subscription)
	if err != nil {
		c.logger.Error("Failed to marshal subscription for caching", 
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
			zap.Error(err))
		return
	}
	
	if err := c.redis.Set(ctx, cacheKey, string(subscriptionData), redis.TTLSubscription); err != nil {
		c.logger.Error("Failed to cache subscription data", 
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
			zap.Error(err))
	} else {
		c.logger.Debug("Subscription cached successfully", 
			zap.String("user_id", userID),
			zap.String("channel_id", channelID))
	}
}

// hashPhoneForLog creates a hash of phone number for safe logging (privacy)
func (c *CacheService) hashPhoneForLog(phone string) string {
	// For privacy, we only log a prefix and suffix of the phone number
	if len(phone) > 6 {
		return phone[:3] + "***" + phone[len(phone)-3:]
	}
	return "***"
}
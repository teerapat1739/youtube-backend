package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

// Cache key constants
const (
	// Voting related keys
	KeyTeamsAll      = "voting:teams:all"
	KeyTeamByID      = "voting:team:%d"           // Individual team data
	KeyTeamCount     = "voting:team:%d:count"
	KeyUserVoted     = "voting:user:%s:voted"
	KeyPhoneVoted    = "voting:phone:%s:voted"    // Phone number vote status
	KeyVoteSummary   = "voting:summary"
	KeyVotingResults = "voting:results"           // Complete voting results with rankings
	KeyLastUpdate    = "voting:last_update"
	KeyETag          = "voting:etag:%s"
	
	// Subscription related keys
	KeySubscriptionCheck = "subscription:%s:%s"   // subscription:{userID}:{channelID}
)

// TTL constants
const (
	// Voting related TTLs
	TTLTeams       = 5 * time.Minute    // Team list cache
	TTLTeamByID    = 15 * time.Minute   // Individual team cache (longer since teams change less frequently)
	TTLCounts      = 30 * time.Second   // Vote counts (short TTL for real-time updates)
	TTLUserVote    = 24 * time.Hour     // User vote status (long TTL, changes rarely)
	TTLPhoneVote   = 2 * time.Hour      // Phone vote status (moderate TTL, balance between performance and data consistency)
	TTLETag        = 5 * time.Minute    // ETag cache
	
	// Subscription related TTLs
	TTLSubscription = 24 * time.Hour    // Subscription status cache (24 hours as requested)
)

// NewClient creates a new Redis client
func NewClient(redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Configure for Cloud Run
	opts.PoolSize = 50
	opts.MinIdleConns = 5
	opts.MaxRetries = 3
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second

	rdb := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	if c.rdb != nil {
		return c.rdb.Close()
	}
	return nil
}

// Get retrieves a value from Redis
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// Set stores a value in Redis with TTL
func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// SetNX sets a value only if it doesn't exist (for duplicate vote prevention)
func (c *Client) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, value, ttl).Result()
}

// Delete removes a key from Redis
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.rdb.Exists(ctx, keys...).Result()
}

// Incr increments a counter
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, key).Result()
}

// HSet sets a hash field
func (c *Client) HSet(ctx context.Context, key string, values ...interface{}) error {
	return c.rdb.HSet(ctx, key, values...).Err()
}

// HGetAll gets all fields from a hash
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.rdb.HGetAll(ctx, key).Result()
}

// Expire sets a TTL on a key
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// Health checks the Redis connection
func (c *Client) Health(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// GetWithFallback attempts to get a value from cache, falling back to a function if not found
func (c *Client) GetWithFallback(ctx context.Context, key string, ttl time.Duration, fallback func() (interface{}, error)) (string, error) {
	// Try to get from cache first
	val, err := c.Get(ctx, key)
	if err == nil && val != "" {
		return val, nil
	}

	// Cache miss or error, use fallback
	result, err := fallback()
	if err != nil {
		return "", err
	}

	// Cache the result (fire and forget to avoid blocking)
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.Set(cacheCtx, key, result, ttl)
	}()

	// Convert result to string for return
	if str, ok := result.(string); ok {
		return str, nil
	}
	return fmt.Sprintf("%v", result), nil
}

// InvalidatePattern removes keys matching a pattern (use carefully in production)
func (c *Client) InvalidatePattern(ctx context.Context, pattern string) error {
	// Get keys matching the pattern
	keys, err := c.rdb.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}

	// Delete matching keys in batches
	if len(keys) > 0 {
		return c.rdb.Del(ctx, keys...).Err()
	}
	return nil
}

// Pipeline creates a new pipeline for batch operations
func (c *Client) Pipeline() redis.Pipeliner {
	return c.rdb.Pipeline()
}

// SetMultiple sets multiple key-value pairs with the same TTL
func (c *Client) SetMultiple(ctx context.Context, kvPairs map[string]interface{}, ttl time.Duration) error {
	pipe := c.Pipeline()
	for key, value := range kvPairs {
		pipe.Set(ctx, key, value, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}
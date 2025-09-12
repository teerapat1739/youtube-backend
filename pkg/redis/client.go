package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Client struct {
	rdb        *redis.Client
	KeyBuilder *KeyBuilder
	log        *zap.Logger
}

// Cache key constants
const (
	// Voting related keys
	KeyTeamsAll        = "voting:teams:all"
	KeyTeamByID        = "voting:team:%d" // Individual team data
	KeyTeamCount       = "voting:team:%d:count"
	KeyUserVoted       = "voting:user:%s:voted"
	KeyPhoneVoted      = "voting:phone:%s:voted" // Phone number vote status
	KeyVoteSummary     = "voting:summary"
	KeyVotingResults   = "voting:results" // Complete voting results with rankings
	KeyLastUpdate      = "voting:last_update"
	KeyETag            = "voting:etag:%s"
	KeyWelcomeAccepted = "welcome:user:%s:accepted" // Welcome acceptance status

	// Subscription related keys
	KeySubscriptionCheck = "subscription:%s:%s" // subscription:{userID}:{channelID}
)

// TTL constants
const (
	// Voting related TTLs
	TTLTeams           = 5 * time.Minute  // Team list cache
	TTLTeamByID        = 15 * time.Minute // Individual team cache (longer since teams change less frequently)
	TTLCounts          = 30 * time.Second // Vote counts (short TTL for real-time updates)
	TTLUserVote        = 24 * time.Hour   // User vote status (long TTL, changes rarely)
	TTLPhoneVote       = 2 * time.Hour    // Phone vote status (moderate TTL, balance between performance and data consistency)
	TTLETag            = 5 * time.Minute  // ETag cache
	TTLWelcomeAccepted = 24 * time.Hour   // Welcome acceptance status (long TTL, changes rarely)

	// Subscription related TTLs
	TTLSubscription = 24 * time.Hour // Subscription status cache (24 hours as requested)
)

// NewClient creates a new Redis client
func NewClient(redisURL string, environment string, log *zap.Logger) (*Client, error) {
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

	// Initialize key builder with environment
	keyBuilder := NewKeyBuilder(environment)

	return &Client{rdb: rdb, KeyBuilder: keyBuilder, log: log}, nil
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
	start := time.Now()
	val, err := c.rdb.Get(ctx, key).Result()
	dur := time.Since(start)
	if err != nil && err != redis.Nil {
		c.log.Info("redis_get",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Duration("duration", dur),
			zap.Error(err))
	} else {
		c.log.Debug("redis_get",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Duration("duration", dur))
	}
	return val, err
}

// Set stores a value in Redis with TTL
func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	err := c.rdb.Set(ctx, key, value, ttl).Err()
	dur := time.Since(start)
	if err != nil {
		c.log.Info("redis_set",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Duration("duration", dur),
			zap.Error(err))
	} else {
		c.log.Debug("redis_set",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Duration("duration", dur))
	}
	return err
}

// SetNX sets a value only if it doesn't exist (for duplicate vote prevention)
func (c *Client) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	start := time.Now()
	ok, err := c.rdb.SetNX(ctx, key, value, ttl).Result()
	dur := time.Since(start)
	if err != nil {
		c.log.Info("redis_setnx",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Duration("duration", dur),
			zap.Error(err))
	} else {
		c.log.Debug("redis_setnx",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Bool("result", ok),
			zap.Duration("duration", dur))
	}
	return ok, err
}

// Delete removes a key from Redis
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	start := time.Now()
	err := c.rdb.Del(ctx, keys...).Err()
	dur := time.Since(start)
	c.log.Debug("redis_del",
		zap.Int("keys", len(keys)),
		zap.Duration("duration", dur),
		zap.Error(err))
	return err
}

// Exists checks if a key exists
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	start := time.Now()
	n, err := c.rdb.Exists(ctx, keys...).Result()
	dur := time.Since(start)
	if err != nil {
		c.log.Info("redis_exists",
			zap.Int("keys", len(keys)),
			zap.Duration("duration", dur),
			zap.Error(err))
	} else {
		c.log.Debug("redis_exists",
			zap.Int64("result", n),
			zap.Int("keys", len(keys)),
			zap.Duration("duration", dur))
	}
	return n, err
}

// Incr increments a counter
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	start := time.Now()
	v, err := c.rdb.Incr(ctx, key).Result()
	dur := time.Since(start)
	if err != nil {
		c.log.Info("redis_incr",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Duration("duration", dur),
			zap.Error(err))
	} else {
		c.log.Debug("redis_incr",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Int64("value", v),
			zap.Duration("duration", dur))
	}
	return v, err
}

// HSet sets a hash field
func (c *Client) HSet(ctx context.Context, key string, values ...interface{}) error {
	start := time.Now()
	err := c.rdb.HSet(ctx, key, values...).Err()
	dur := time.Since(start)
	if err != nil {
		c.log.Info("redis_hset",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Int("fields", len(values)/2),
			zap.Duration("duration", dur),
			zap.Error(err))
	} else {
		c.log.Debug("redis_hset",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Int("fields", len(values)/2),
			zap.Duration("duration", dur))
	}
	return err
}

// HGetAll gets all fields from a hash
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	start := time.Now()
	m, err := c.rdb.HGetAll(ctx, key).Result()
	dur := time.Since(start)
	if err != nil {
		c.log.Info("redis_hgetall",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Duration("duration", dur),
			zap.Error(err))
	} else {
		c.log.Debug("redis_hgetall",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Int("fields", len(m)),
			zap.Duration("duration", dur))
	}
	return m, err
}

// Expire sets a TTL on a key
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	start := time.Now()
	err := c.rdb.Expire(ctx, key, ttl).Err()
	dur := time.Since(start)
	if err != nil {
		c.log.Info("redis_expire",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Duration("duration", dur),
			zap.Error(err))
	} else {
		c.log.Debug("redis_expire",
			zap.String("key_prefix", prefixForLog(key)),
			zap.Duration("duration", dur))
	}
	return err
}

// Health checks the Redis connection
func (c *Client) Health(ctx context.Context) error {
	start := time.Now()
	err := c.rdb.Ping(ctx).Err()
	dur := time.Since(start)
	if err != nil {
		c.log.Info("redis_ping",
			zap.Duration("duration", dur),
			zap.Error(err))
	} else {
		c.log.Debug("redis_ping", zap.Duration("duration", dur))
	}
	return err
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
	start := time.Now()
	_, err := pipe.Exec(ctx)
	dur := time.Since(start)
	if err != nil {
		c.log.Info("redis_set_multiple",
			zap.Int("keys", len(kvPairs)),
			zap.Duration("duration", dur),
			zap.Error(err))
	} else {
		c.log.Debug("redis_set_multiple",
			zap.Int("keys", len(kvPairs)),
			zap.Duration("duration", dur))
	}
	return err
}

// prefixForLog returns a safe prefix of a key to avoid logging PII
func prefixForLog(key string) string {
	if len(key) <= 24 {
		return key
	}
	return key[:24] + "…"
}

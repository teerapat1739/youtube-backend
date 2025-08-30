package cache

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gamemini/youtube/pkg/config"
	"github.com/redis/go-redis/v9"
)

var (
	redisClient *redis.Client
	redisOnce   sync.Once
	redisErr    error
	debugMode   bool
)

func init() {
	// Debug mode will be initialized when config is loaded
	debugMode = false
}

// GetRedis returns the singleton Redis client or nil if not configured
func GetRedis() *redis.Client {
	redisOnce.Do(func() {
		redisClient, redisErr = initRedisClient()
		if redisErr != nil && debugMode {
			log.Printf("[REDIS] Failed to initialize Redis client: %v", redisErr)
		}
	})
	return redisClient
}

// initRedisClient initializes the Redis client from centralized configuration
func initRedisClient() (*redis.Client, error) {
	appConfig := config.GetConfig()
	debugMode = appConfig.Debug
	redisURL := appConfig.RedisURL
	if redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL environment variable is required")
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse REDIS_URL: %w", err)
	}
	
	// Set reasonable defaults for cloud Redis
	opt.MaxRetries = 3
	opt.MinRetryBackoff = 100 * time.Millisecond
	opt.MaxRetryBackoff = 500 * time.Millisecond
	opt.DialTimeout = 5 * time.Second
	opt.ReadTimeout = 2 * time.Second
	opt.WriteTimeout = 2 * time.Second
	opt.PoolSize = 10
	opt.MinIdleConns = 2
	opt.MaxIdleConns = 5
	opt.ConnMaxIdleTime = 10 * time.Minute
	opt.ConnMaxLifetime = 60 * time.Minute
	
	client := redis.NewClient(opt)
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	
	if debugMode {
		log.Printf("[REDIS] Connected to Redis Cloud via REDIS_URL")
	}
	return client, nil
}

// HGetAll gets all fields and values in a hash
func HGetAll(ctx context.Context, key string) (map[string]string, error) {
	client := GetRedis()
	if client == nil {
		return nil, fmt.Errorf("redis client not available")
	}
	
	result, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis HGetAll failed: %w", err)
	}
	return result, nil
}

// HSetAll sets multiple fields in a hash from a map[string]int64
func HSetAll(ctx context.Context, key string, values map[string]int64) error {
	if len(values) == 0 {
		return nil
	}
	
	client := GetRedis()
	if client == nil {
		return fmt.Errorf("redis client not available")
	}
	
	// Convert int64 values to interface{} for Redis
	data := make(map[string]interface{}, len(values))
	for k, v := range values {
		data[k] = v
	}
	
	err := client.HSet(ctx, key, data).Err()
	if err != nil {
		return fmt.Errorf("redis HSet failed: %w", err)
	}
	return nil
}

// HIncrBy increments a hash field by the given amount
func HIncrBy(ctx context.Context, key, field string, by int64) error {
	client := GetRedis()
	if client == nil {
		return fmt.Errorf("redis client not available")
	}
	
	err := client.HIncrBy(ctx, key, field, by).Err()
	if err != nil {
		return fmt.Errorf("redis HIncrBy failed: %w", err)
	}
	return nil
}

// HSetAllWithTTL sets multiple fields in a hash with TTL
func HSetAllWithTTL(ctx context.Context, key string, values map[string]int64, ttl time.Duration) error {
	if len(values) == 0 {
		return nil
	}
	
	client := GetRedis()
	if client == nil {
		return fmt.Errorf("redis client not available")
	}
	
	// Convert int64 values to interface{} for Redis
	data := make(map[string]interface{}, len(values))
	for k, v := range values {
		data[k] = v
	}
	
	// Use pipeline for atomic operation
	pipe := client.Pipeline()
	pipe.HSet(ctx, key, data)
	pipe.Expire(ctx, key, ttl)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis pipeline failed: %w", err)
	}
	return nil
}

// Del deletes a key
func Del(ctx context.Context, key string) error {
	client := GetRedis()
	if client == nil {
		return fmt.Errorf("redis client not available")
	}
	
	err := client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("redis Del failed: %w", err)
	}
	return nil
}
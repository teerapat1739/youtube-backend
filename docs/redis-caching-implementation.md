# Redis Caching Implementation for Voting Service

This document outlines the comprehensive Redis caching implementation for the voting service in be-v2, providing high-performance, scalable, and reliable caching patterns.

## Overview

The caching implementation follows the **cache-aside pattern** with comprehensive error handling, fallback mechanisms, and cache invalidation strategies. It's designed for production environments with high concurrency and reliability requirements.

## Architecture

### Components

1. **Redis Client** (`pkg/redis/client.go`)
   - Enhanced with new cache keys and TTL constants
   - Helper methods for batch operations and pattern-based invalidation
   - Production-ready connection pooling and timeouts

2. **Cache Service** (`internal/service/cache_service.go`)
   - Centralized caching logic with comprehensive error handling
   - Structured logging for observability
   - Async cache operations to avoid blocking

3. **Voting Service** (`internal/service/voting_service.go`)
   - Integration with cache service
   - Fallback mechanisms for database access

## Caching Strategies

### 1. Team Data Caching

**Cache Key Pattern**: `voting:team:{team_id}`
**TTL**: 15 minutes (teams change infrequently)

```go
// Cache-aside pattern with fallback
team, err := s.cacheService.GetTeamWithCache(ctx, teamID,
    func(ctx context.Context, id int) (*domain.Team, error) {
        return s.voteRepo.GetTeamByID(ctx, id)
    })
```

**Benefits**:
- Reduces database load for frequently accessed team data
- Longer TTL since team information is relatively static
- Graceful degradation when cache is unavailable

### 2. Phone Number Vote Checking

**Cache Key Pattern**: `voting:phone:{normalized_phone}:voted`
**TTL**: 2 hours (balance between performance and data consistency)

```go
// Check phone usage with caching
phoneUsed, err := s.cacheService.CheckPhoneUsageWithCache(ctx, normalizedPhone, 
    func(ctx context.Context, phone string) (bool, error) {
        vote, err := s.voteRepo.GetVoteByPhone(ctx, phone)
        return vote != nil, err
    })
```

**Benefits**:
- Prevents duplicate database queries for the same phone number
- Moderate TTL ensures data consistency while providing performance benefits
- Only caches positive results (phone already used)

## Cache Keys and TTL Strategy

| Cache Key Pattern | Purpose | TTL | Rationale |
|-------------------|---------|-----|-----------|
| `voting:team:{id}` | Individual team data | 15 minutes | Teams change infrequently |
| `voting:phone:{phone}:voted` | Phone usage tracking | 2 hours | Balance performance/consistency |
| `voting:user:{user_id}:voted` | User vote status | 24 hours | Rarely changes once set |
| `voting:teams:all` | Team list with counts | 5 minutes | Real-time vote counts |
| `voting:summary` | Vote summary cache | 30 seconds | High-frequency updates |

## Error Handling and Fallback

### Cache Miss Handling
```go
// Graceful fallback to database
if cacheErr != nil {
    logger.Warn("Cache error, falling back to database", zap.Error(cacheErr))
}
// Continue with database query...
```

### Cache Corruption Handling
```go
// Detect and handle corrupted cache data
if unmarshalErr := json.Unmarshal([]byte(cachedData), &team); unmarshalErr != nil {
    logger.Warn("Cache corrupted, falling back to database", zap.Error(unmarshalErr))
    // Continue with database fallback
}
```

### Async Cache Operations
```go
// Cache operations don't block main flow
go func() {
    cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _ = c.redis.Set(cacheCtx, key, data, ttl)
}()
```

## Cache Invalidation Strategy

### 1. On Vote Submission
When a vote is successfully submitted:

```go
// Immediate cache invalidation
s.cacheService.InvalidateVotingCaches(req.TeamID)
```

**Invalidated Keys**:
- `voting:teams:all` - Team list with vote counts
- `voting:summary` - Vote summary cache
- `voting:team:{team_id}:count` - Specific team vote count
- `voting:etag:*` - All ETag caches

### 2. Atomic Cache Updates
```go
// Pipeline for atomic operations
pipe := c.redis.Pipeline()
pipe.Set(ctx, userKey, teamID, redis.TTLUserVote)
pipe.Set(ctx, phoneKey, "1", redis.TTLPhoneVote)
_, err := pipe.Exec(ctx)
```

## Performance Optimizations

### 1. Connection Pooling
```go
opts.PoolSize = 50
opts.MinIdleConns = 5
opts.MaxRetries = 3
```

### 2. Timeout Configuration
```go
opts.DialTimeout = 5 * time.Second
opts.ReadTimeout = 3 * time.Second
opts.WriteTimeout = 3 * time.Second
```

### 3. Batch Operations
```go
// Multiple cache operations in single pipeline
func (c *Client) SetMultiple(ctx context.Context, kvPairs map[string]interface{}, ttl time.Duration) error
```

## Monitoring and Observability

### Structured Logging
```go
c.logger.Debug("Team cache hit", zap.Int("team_id", teamID))
c.logger.Warn("Cache error, falling back to database", zap.Error(err))
```

### Health Checks
```go
func (s *VotingService) HealthCheck(ctx context.Context) error {
    return s.cacheService.HealthCheck(ctx)
}
```

### Privacy-Safe Logging
```go
// Phone numbers are hashed for privacy in logs
func (c *CacheService) hashPhoneForLog(phone string) string {
    if len(phone) > 6 {
        return phone[:3] + "***" + phone[len(phone)-3:]
    }
    return "***"
}
```

## Best Practices Implemented

### 1. Cache-Aside Pattern
- Application manages cache explicitly
- Database is the source of truth
- Cache misses fall back to database

### 2. Fail-Safe Design
- Cache failures don't break functionality
- Graceful degradation to database
- Async operations prevent blocking

### 3. Consistency Management
- Appropriate TTLs for different data types
- Proactive cache invalidation
- Atomic operations where needed

### 4. Security and Privacy
- Phone numbers are normalized and hashed in logs
- No sensitive data in cache keys
- Secure Redis connection configuration

## Usage Examples

### Basic Team Lookup with Caching
```go
team, err := votingService.cacheService.GetTeamWithCache(ctx, teamID,
    func(ctx context.Context, id int) (*domain.Team, error) {
        return voteRepository.GetTeamByID(ctx, id)
    })
```

### Phone Number Duplicate Check
```go
phoneUsed, err := votingService.cacheService.CheckPhoneUsageWithCache(ctx, phone,
    func(ctx context.Context, phone string) (bool, error) {
        vote, err := voteRepository.GetVoteByPhone(ctx, phone)
        return vote != nil, err
    })
```

### Vote Submission with Caching
```go
// After successful vote creation
err := votingService.cacheService.CacheVoteSubmission(ctx, userID, phone, teamID)
votingService.cacheService.InvalidateVotingCaches(teamID)
```

## Configuration

### Environment Variables
```bash
REDIS_URL=redis://localhost:6379/0  # Redis connection string
```

### Service Initialization
```go
// Initialize with logger for observability
votingService := NewVotingService(voteRepo, redisClient, logger)
```

## Testing Considerations

1. **Cache Miss Scenarios**: Test behavior when Redis is unavailable
2. **Cache Corruption**: Test handling of invalid cached data
3. **Concurrent Access**: Test race conditions and atomic operations
4. **TTL Expiration**: Test cache expiration scenarios
5. **Performance**: Measure cache hit rates and response times

This implementation provides a robust, scalable, and maintainable caching solution that enhances the voting service performance while maintaining data consistency and reliability.
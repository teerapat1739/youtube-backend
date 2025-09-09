# Redis Cache Key Management Documentation

This document provides comprehensive documentation for the Redis cache key management system in the YouTube backend, focusing on the simple, effective environment-based key prefixing strategy.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Environment-Based Key Prefixing](#environment-based-key-prefixing)
3. [Complete Redis Key Reference](#complete-redis-key-reference)
4. [Key Naming Conventions](#key-naming-conventions)
5. [TTL Configuration Strategy](#ttl-configuration-strategy)
6. [Usage Examples](#usage-examples)
7. [Best Practices](#best-practices)
8. [Troubleshooting](#troubleshooting)
9. [Adding New Cache Keys](#adding-new-cache-keys)

## Architecture Overview

The Redis cache key management system uses a simple, maintainable approach to provide complete isolation between staging and production environments.

### Current Implementation Structure

```
pkg/redis/
├── client.go           # Core Redis client with connection pooling
├── key_builder.go      # Environment-aware key prefixing (ACTIVE)
└── key_builder_test.go # Comprehensive test coverage

internal/service/
├── cache_service.go    # High-level caching patterns using KeyBuilder
├── voting_service.go   # Voting system with Redis caching
└── visitor_service.go  # Visitor tracking with Redis
```

### Core Components

1. **KeyBuilder** (`pkg/redis/key_builder.go`)
   - Simple, focused implementation
   - Automatically prefixes all keys with environment identifier
   - Service-specific methods for type-safe key generation
   - Zero configuration after initialization

2. **Redis Client** (`pkg/redis/client.go`)
   - Connection pooling (default: 50 connections)
   - Integrated KeyBuilder for automatic prefixing
   - Graceful error handling
   - Connection health monitoring

3. **Service Integration**
   - All services use KeyBuilder consistently
   - No manual prefix management needed
   - Type-safe key generation methods

### Design Philosophy

- **Simplicity First**: One clear way to do things
- **Environment Safety**: Impossible to mix staging/production data
- **Maintainability**: Easy to understand and extend
- **Performance**: Efficient connection pooling and operations

## Environment-Based Key Prefixing

### How It Works

Every Redis key is automatically prefixed based on the `ENVIRONMENT` variable:

| Environment Variable | Redis Key Prefix | Use Case |
|---------------------|------------------|----------|
| `development` | `staging:` | Local development |
| `staging` | `staging:` | Staging server |
| `production` | `prod:` | Production server |

### Examples

```
# Development/Staging
staging:voting:summary
staging:visitor:total
staging:visitor:daily:2025-01-09

# Production
prod:voting:summary
prod:visitor:total
prod:visitor:daily:2025-01-09
```

### Implementation

```go
// Automatic prefix determination
func NewKeyBuilder(environment string) *KeyBuilder {
    prefix := "prod"
    if environment == "development" || environment == "staging" {
        prefix = "staging"
    }
    return &KeyBuilder{prefix: prefix}
}
```

## Complete Redis Key Reference

### Voting Service Keys

| Key Pattern | Example | Purpose | TTL |
|------------|---------|---------|-----|
| `voting:summary` | `staging:voting:summary` | Voting summary cache | 5 minutes |
| `voting:results` | `staging:voting:results` | Voting results cache | 30 minutes |
| `voting:team:%d` | `staging:voting:team:1` | Team data by ID | 30 minutes |
| `voting:teams` | `staging:voting:teams` | All teams data | 30 minutes |
| `voting:latest` | `staging:voting:latest` | Latest votes | 5 minutes |
| `voting:user:%s:voted` | `staging:voting:user:123:voted` | User voted flag | 24 hours |
| `voting:phone:%s:voted` | `staging:voting:phone:+66891234567:voted` | Phone voted flag | 24 hours |
| `voting:user:%s:team` | `staging:voting:user:123:team` | User's voted team | 24 hours |
| `voting:team:%d:by_id` | `staging:voting:team:1:by_id` | Team lookup by ID | 30 minutes |
| `voting:lock:%s` | `staging:voting:lock:user123` | Distributed lock | 10 seconds |
| `voting:processing:%s` | `staging:voting:processing:vote123` | Processing flag | 1 minute |

### Visitor Service Keys

| Key Pattern | Example | Purpose | TTL |
|------------|---------|---------|-----|
| `visitor:total` | `staging:visitor:total` | Total visitor count | No expiry |
| `visitor:daily:%s` | `staging:visitor:daily:2025-01-09` | Daily visitor count | 25 hours |
| `visitor:unique` | `staging:visitor:unique` | Unique visitor set | 7 days |
| `visitor:unique:daily:%s` | `staging:visitor:unique:daily:2025-01-09` | Daily unique visitors | 25 hours |
| `visitor:ratelimit:%s` | `staging:visitor:ratelimit:ip_hash` | Rate limiting by IP | 1 hour |
| `visitor:last_update` | `staging:visitor:last_update` | Last update timestamp | 24 hours |

### Subscription Service Keys

| Key Pattern | Example | Purpose | TTL |
|------------|---------|---------|-----|
| `subscription:%s:%s` | `staging:subscription:user123:UC-chqi3Gpb4F7yBqedlnq5g` | YouTube subscription check | 1 hour |

### Welcome Service Keys

| Key Pattern | Example | Purpose | TTL |
|------------|---------|---------|-----|
| `welcome:accepted:%s` | `staging:welcome:accepted:user123` | Welcome dialog accepted flag | 7 days |

## Key Naming Conventions

### Hierarchical Structure

```
{prefix}:{service}:{resource}:{identifier}:{sub-resource}
```

Examples:
- `staging:voting:user:123:voted`
- `prod:visitor:daily:2025-01-09`
- `staging:subscription:user123:channel456`

### Naming Rules

1. **Use colons as separators**: Better for Redis operations
2. **Lowercase only**: Consistency across the system
3. **Service namespace first**: Groups related keys
4. **Descriptive resource names**: Clear purpose identification
5. **Include identifiers**: User IDs, dates, etc.

## TTL Configuration Strategy

### TTL Categories

| Category | Duration | Use Cases |
|----------|----------|-----------|
| **Ultra-short** | 5-10 seconds | Distributed locks |
| **Short** | 1-5 minutes | Processing flags, hot data |
| **Medium** | 30 minutes - 1 hour | API responses, summaries |
| **Long** | 24 hours | User sessions, daily data |
| **Extended** | 7 days | Unique tracking, welcome flags |
| **Permanent** | No expiry | Total counts, critical data |

### Service-Specific TTLs

```go
// Voting Service TTLs
const (
    TTLVotingSummary = 5 * time.Minute
    TTLVotingResults = 30 * time.Minute
    TTLUserVoted     = 24 * time.Hour
    TTLVotingLock    = 10 * time.Second
)

// Visitor Service TTLs
const (
    TTLVisitorDaily       = 25 * time.Hour
    TTLVisitorUnique      = 7 * 24 * time.Hour
    TTLVisitorRateLimit   = 1 * time.Hour
)
```

## Usage Examples

### Basic Key Building

```go
// Initialize KeyBuilder with environment
kb := redis.NewKeyBuilder(cfg.Environment)

// Build simple key
key := kb.BuildKey("voting:summary")
// Result: "staging:voting:summary" or "prod:voting:summary"
```

### Service-Specific Methods

```go
// Voting keys
userVotedKey := kb.KeyUserVoted("user123")
// Result: "staging:voting:user:user123:voted"

teamKey := kb.KeyTeamByID(1)
// Result: "staging:voting:team:1:by_id"

// Visitor keys
dailyKey := kb.KeyVisitorDaily("2025-01-09")
// Result: "staging:visitor:daily:2025-01-09"

// Subscription keys
subKey := kb.KeySubscriptionCheck("user123", "channel456")
// Result: "staging:subscription:user123:channel456"
```

### In Service Implementation

```go
// voting_service.go
func (s *VotingService) GetVotingSummary(ctx context.Context) (*VotingSummary, error) {
    // Key is automatically prefixed
    cacheKey := s.redis.KeyBuilder.KeyVotingSummary()
    
    // Try to get from cache
    cached, err := s.redis.Get(ctx, cacheKey)
    if err == nil && cached != "" {
        // Parse and return cached data
    }
    
    // If not cached, compute and store
    summary := computeSummary()
    s.redis.Set(ctx, cacheKey, summary, 5*time.Minute)
    
    return summary, nil
}
```

### Cache Service Pattern

```go
// cache_service.go
func (s *CacheService) GetWithFallback(ctx context.Context, key string, 
    fallback func() (interface{}, error)) (interface{}, error) {
    
    // Key is automatically prefixed via KeyBuilder
    fullKey := s.redis.KeyBuilder.BuildKey(key)
    
    // Try cache first
    if cached, err := s.redis.Get(ctx, fullKey); err == nil {
        return cached, nil
    }
    
    // Fallback to computation
    result, err := fallback()
    if err != nil {
        return nil, err
    }
    
    // Cache the result
    s.redis.Set(ctx, fullKey, result, 30*time.Minute)
    return result, nil
}
```

## Best Practices

### 1. Always Use KeyBuilder

```go
// ✅ GOOD - Uses KeyBuilder
key := s.redis.KeyBuilder.KeyVotingSummary()

// ❌ BAD - Manual key construction
key := "staging:voting:summary"  // Don't hardcode prefixes!
```

### 2. Appropriate TTL Selection

```go
// ✅ GOOD - Appropriate TTLs
s.redis.Set(ctx, lockKey, "1", 10*time.Second)      // Short for locks
s.redis.Set(ctx, summaryKey, data, 5*time.Minute)   // Medium for summaries
s.redis.Set(ctx, userKey, data, 24*time.Hour)       // Long for user data

// ❌ BAD - Inappropriate TTLs
s.redis.Set(ctx, lockKey, "1", 24*time.Hour)        // Too long for locks!
```

### 3. Handle Cache Misses Gracefully

```go
// ✅ GOOD - Graceful fallback
data, err := s.redis.Get(ctx, key)
if err != nil || data == "" {
    // Compute fresh data
    data = computeData()
    // Cache for next time
    s.redis.Set(ctx, key, data, ttl)
}
return data

// ❌ BAD - No fallback
data, err := s.redis.Get(ctx, key)
if err != nil {
    return nil, err  // Don't fail on cache miss!
}
```

### 4. Use Service-Specific Methods

```go
// ✅ GOOD - Type-safe, clear intent
key := kb.KeyUserVoted(userID)
key := kb.KeyVisitorDaily(date)

// ❌ BAD - Error-prone string formatting
key := fmt.Sprintf("voting:user:%s:voted", userID)
```

### 5. Monitor Key Patterns

```bash
# Check key distribution
redis-cli --scan --pattern "staging:*" | head -20

# Monitor key sizes
redis-cli INFO memory

# Check specific service keys
redis-cli KEYS "staging:voting:*"
```

## Troubleshooting

### Common Issues

#### 1. Environment Mismatch

**Problem**: Data not found after deployment
```bash
# Check actual keys in Redis
redis-cli KEYS "*" | head
# Shows: prod:voting:summary (but app expects staging:voting:summary)
```

**Solution**: Verify ENVIRONMENT variable
```bash
echo $ENVIRONMENT  # Should match deployment environment
```

#### 2. Cache Not Updating

**Problem**: Stale data being served
```bash
# Check TTL
redis-cli TTL "staging:voting:summary"
# Returns: -1 (no expiry set)
```

**Solution**: Ensure TTL is set on cache writes
```go
s.redis.Set(ctx, key, data, 5*time.Minute)  // Always include TTL
```

#### 3. Memory Issues

**Problem**: Redis memory full
```bash
redis-cli INFO memory
# used_memory_human:450M
# maxmemory_human:512M
```

**Solution**: Review TTLs and implement eviction
```bash
# Set eviction policy
redis-cli CONFIG SET maxmemory-policy allkeys-lru

# Review large keys
redis-cli --bigkeys
```

#### 4. Connection Pool Exhaustion

**Problem**: "connection pool timeout" errors

**Solution**: Increase pool size
```go
// In redis/client.go
opt.PoolSize = 100  // Increase from default 50
```

### Debugging Commands

```bash
# View all staging keys
redis-cli --scan --pattern "staging:*"

# View all production keys
redis-cli --scan --pattern "prod:*"

# Check specific service
redis-cli --scan --pattern "staging:voting:*"

# Monitor real-time commands
redis-cli MONITOR

# Check slow queries
redis-cli SLOWLOG GET 10

# Memory analysis
redis-cli MEMORY USAGE "staging:voting:summary"
```

## Adding New Cache Keys

### Step-by-Step Process

#### 1. Define the Key Pattern

```go
// In pkg/redis/key_builder.go

// Add new key pattern method
func (kb *KeyBuilder) KeyAnalyticsSummary(period string) string {
    return kb.BuildKey(fmt.Sprintf("analytics:summary:%s", period))
}

func (kb *KeyBuilder) KeyAnalyticsUser(userID string) string {
    return kb.BuildKey(fmt.Sprintf("analytics:user:%s", userID))
}
```

#### 2. Add to Documentation

Update this document with:
- Key pattern
- Example with prefix
- Purpose
- TTL

#### 3. Implement in Service

```go
// In internal/service/analytics_service.go

type AnalyticsService struct {
    redis *redis.Client
}

func (s *AnalyticsService) GetDailySummary(ctx context.Context) (*Summary, error) {
    // Use the new key method
    key := s.redis.KeyBuilder.KeyAnalyticsSummary("daily")
    
    // Try cache first
    if cached, err := s.redis.Get(ctx, key); err == nil {
        return parseSummary(cached), nil
    }
    
    // Compute if not cached
    summary := s.computeDailySummary()
    
    // Cache with appropriate TTL
    s.redis.Set(ctx, key, summary, 1*time.Hour)
    
    return summary, nil
}
```

#### 4. Add Tests

```go
// In pkg/redis/key_builder_test.go

func TestKeyAnalytics(t *testing.T) {
    kb := NewKeyBuilder("staging")
    
    tests := []struct {
        name     string
        method   func() string
        expected string
    }{
        {
            name:     "Analytics summary key",
            method:   func() string { return kb.KeyAnalyticsSummary("daily") },
            expected: "staging:analytics:summary:daily",
        },
        {
            name:     "Analytics user key",
            method:   func() string { return kb.KeyAnalyticsUser("user123") },
            expected: "staging:analytics:user:user123",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            assert.Equal(t, tt.expected, tt.method())
        })
    }
}
```

#### 5. Deploy and Monitor

```bash
# After deployment, verify keys are created correctly
redis-cli --scan --pattern "*analytics*"

# Check TTLs are set
redis-cli TTL "staging:analytics:summary:daily"

# Monitor memory impact
redis-cli MEMORY USAGE "staging:analytics:summary:daily"
```

## Summary

The Redis cache key management system provides:

1. **Complete environment isolation** through automatic prefixing
2. **Simple, maintainable architecture** with just the essential components
3. **Type-safe key generation** through KeyBuilder methods
4. **Consistent patterns** across all services
5. **Easy debugging** with clear key hierarchies

The system prioritizes simplicity and safety, ensuring staging and production data never mix while maintaining excellent performance and developer experience.
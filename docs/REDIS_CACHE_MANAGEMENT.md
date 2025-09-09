# Redis Cache Management API

## Overview

The backend provides development-only endpoints for managing Redis cache during testing and debugging. All Redis keys are automatically prefixed based on the environment to ensure complete isolation between staging and production data.

## Environment-Based Key Prefixing

All Redis keys are automatically prefixed based on the `ENVIRONMENT` variable:

| Environment | Key Prefix | Example Key |
|------------|------------|-------------|
| `development` | `staging:` | `staging:voting:summary` |
| `staging` | `staging:` | `staging:visitor:total` |
| `production` | `prod:` | `prod:voting:team:1` |

## Cache Management Endpoints

### Clear Redis Cache

**Endpoint:** `DELETE /api/testing/clear-redis-cache`

**Description:** Clears all Redis cache keys for the current environment. This endpoint is **only available in development environment**.

**Required Headers:**
- `X-Clear-Cache-Confirm: yes` - Confirmation header to prevent accidental cache clearing
- `Content-Type: application/json`

**Example Request:**
```bash
curl -X DELETE http://localhost:8080/api/testing/clear-redis-cache \
  -H "X-Clear-Cache-Confirm: yes" \
  -H "Content-Type: application/json"
```

**Success Response (200 OK):**
```json
{
  "status": "success",
  "message": "Successfully cleared Redis cache keys with prefix 'staging' (duration: 47.233792ms)",
  "environment": "development",
  "keys_cleared": 0,
  "timestamp": "2025-01-09T15:10:03.116678Z"
}
```

**Error Responses:**

- **403 Forbidden** - When accessed in non-development environment:
```json
{
  "status": "error",
  "message": "This endpoint is only available in development environment",
  "environment": "production",
  "keys_cleared": 0,
  "timestamp": "2025-01-09T15:10:03.116678Z"
}
```

- **400 Bad Request** - When confirmation header is missing:
```json
{
  "status": "error",
  "message": "Missing confirmation header. Add 'X-Clear-Cache-Confirm: yes' to proceed",
  "environment": "development",
  "keys_cleared": 0,
  "timestamp": "2025-01-09T15:10:03.116678Z"
}
```

## Other Testing Endpoints

### Refresh Materialized View

**Endpoint:** `POST /api/testing/refresh-materialized-view`

**Description:** Manually refreshes the voting results materialized view (development only).

**Example Request:**
```bash
curl -X POST http://localhost:8080/api/testing/refresh-materialized-view
```

### Get Materialized View Stats

**Endpoint:** `GET /api/testing/materialized-view-stats`

**Description:** Returns statistics about the materialized view (development only).

**Example Request:**
```bash
curl http://localhost:8080/api/testing/materialized-view-stats
```

## Redis Key Patterns

All keys follow the pattern: `{environment_prefix}:{service}:{resource}:{identifier}`

### Common Key Patterns

| Service | Key Pattern | Example | Purpose |
|---------|------------|---------|---------|
| Voting | `{prefix}:voting:summary` | `staging:voting:summary` | Voting summary cache |
| Voting | `{prefix}:voting:user:{id}:voted` | `staging:voting:user:123:voted` | User vote status |
| Voting | `{prefix}:voting:phone:{phone}:voted` | `staging:voting:phone:+66891234567:voted` | Phone vote status |
| Visitor | `{prefix}:visitor:total` | `staging:visitor:total` | Total visitor count |
| Visitor | `{prefix}:visitor:daily:{date}` | `staging:visitor:daily:2025-01-09` | Daily visitor count |
| Visitor | `{prefix}:visitor:unique` | `staging:visitor:unique` | Unique visitor set |

## Testing Script

A test script is provided to verify Redis key format and cache clearing:

```bash
./test_redis_keys.sh
```

This script will:
1. Test the cache clear endpoint
2. List current Redis keys with environment prefix
3. Create new visitor tracking keys
4. Verify keys are properly prefixed

## Best Practices

1. **Never manually create Redis keys** - Always use the KeyBuilder methods
2. **Test in development first** - Use the development environment for testing cache operations
3. **Monitor key patterns** - Use Redis monitoring to track key usage
4. **Clear cache sparingly** - Cache clearing should only be done during testing/debugging

## Security Considerations

1. Cache clearing endpoint is **development-only** - Returns 403 in production
2. Requires confirmation header to prevent accidental clearing
3. All operations are logged for audit purposes
4. Environment prefixes prevent cross-environment data access

## Implementation Details

The Redis key management is implemented through:

1. **KeyBuilder** (`pkg/redis/key_builder.go`) - Handles all key generation with environment prefixes
2. **TestingHandler** (`internal/handler/testing_handler.go`) - Provides development-only cache management endpoints
3. **Automatic prefixing** - All services use KeyBuilder for consistent key generation

## Troubleshooting

### Cache not clearing
- Verify you're in development environment (`ENVIRONMENT=development`)
- Check the confirmation header is included
- Review server logs for error messages

### Keys not prefixed correctly
- Ensure all Redis operations use KeyBuilder
- Check environment variable is set correctly
- Verify KeyBuilder is initialized with correct environment

### Redis connection issues
- Check Redis is running and accessible
- Verify Redis URL in configuration
- Review connection pool settings
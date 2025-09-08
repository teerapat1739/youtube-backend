# Welcome Acceptance Implementation

This document describes the backend implementation for tracking welcome/rules acceptance in the voting platform.

## Overview

The welcome acceptance feature allows the system to track when users accept the platform's welcome screen and rules. This is similar to the existing voting status tracking but focuses specifically on onboarding compliance.

## Implementation Details

### Database Changes

**Migration File:** `/migrations/add_welcome_tracking.sql`

Added the following columns to the `votes` table:
- `welcome_accepted` (boolean, default: false) - Whether user has accepted welcome/rules
- `welcome_accepted_at` (timestamp, nullable) - When acceptance occurred
- `rules_version` (varchar(50), nullable) - Version of rules that were accepted

Indexes added for performance:
- `idx_votes_welcome_accepted` on `welcome_accepted`
- `idx_votes_user_id_welcome` on `(user_id, welcome_accepted)`

### Repository Layer

**File:** `/internal/repository/vote_repository.go`

#### New Methods:
- `SaveWelcomeAcceptance(ctx, userID, rulesVersion)` - Saves welcome acceptance to database
- `GetWelcomeAcceptance(ctx, userID)` - Retrieves welcome acceptance status
- Updated `GetPersonalInfoByUserID()` to include welcome fields

### Service Layer

**File:** `/internal/service/voting_service.go`

#### New Methods:
- `SaveWelcomeAcceptance(ctx, userID, rulesVersion)` - Handles welcome acceptance with caching
- `GetWelcomeAcceptance(ctx, userID)` - Retrieves status with cache-first approach

#### Caching Strategy:
- **Cache Key:** `welcome:user:{userID}:accepted`
- **TTL:** 24 hours (long-lived since acceptance rarely changes)
- **Pattern:** Write-through caching (save to DB first, then cache)

### Handler Layer

**File:** `/internal/handler/voting_handler.go`

#### New Endpoint:
- `AcceptWelcome()` - Handles `POST /api/welcome/accept`
- Updated `GetPersonalInfoMe()` to include welcome status (already handled by repository changes)

### Routes

**File:** `/main.go`

Added route: `POST /api/welcome/accept` (requires authentication)

### Redis Configuration

**File:** `/pkg/redis/client.go`

#### New Constants:
- `KeyWelcomeAccepted = "welcome:user:%s:accepted"`
- `TTLWelcomeAccepted = 24 * time.Hour`

## API Endpoints

### POST /api/welcome/accept

**Authentication:** Required

**Request Body:**
```json
{
  "rules_version": "1.0"
}
```

**Response (200 OK):**
```json
{
  "user_id": "usr123",
  "welcome_accepted": true,
  "welcome_accepted_at": "2025-09-05T12:00:00Z",
  "rules_version": "1.0",
  "message": "Welcome acceptance saved successfully"
}
```

**Error Responses:**
- `401 Unauthorized` - Missing or invalid authentication
- `400 Bad Request` - Invalid request body or missing rules_version
- `412 Precondition Failed` - User personal info not found

### GET /api/personal-info/me

**Authentication:** Required

The existing endpoint now includes welcome acceptance fields:

**Response (200 OK):**
```json
{
  "user_id": "usr123",
  "phone": "+66123456789",
  "first_name": "John",
  "last_name": "Doe",
  "email": "john.doe@example.com",
  "favorite_video": "Some video",
  "consent_pdpa": true,
  "created_at": "2025-09-05T10:00:00Z",
  "updated_at": "2025-09-05T10:00:00Z",
  "has_voted": false,
  "welcome_accepted": true,
  "welcome_accepted_at": "2025-09-05T12:00:00Z",
  "rules_version": "1.0"
}
```

## Deployment Steps

1. **Run Database Migration:**
   ```bash
   psql -d your_database -f migrations/add_welcome_tracking.sql
   ```

2. **Deploy Application:**
   The application includes the new code and will work with existing data (default `welcome_accepted = false`).

3. **Verify Redis Configuration:**
   Ensure Redis is available for caching.

## Testing

Use the provided test script:
```bash
./scripts/test_welcome_acceptance.sh
```

## Error Handling

The implementation includes proper error handling:
- Database connection issues
- User not found scenarios
- Invalid request validation
- Redis caching failures (non-blocking)

## Performance Considerations

- **Database:** Indexed queries for efficient lookups
- **Caching:** 24-hour TTL reduces database load
- **Write-through:** Ensures data consistency
- **Async:** Non-blocking cache operations

## Security

- **Authentication:** All endpoints require valid authentication
- **Input Validation:** Rules version is required and validated
- **Audit Trail:** IP address and user agent captured
- **Rate Limiting:** Inherits from existing middleware

## Monitoring

The implementation includes structured logging with:
- User ID tracking
- Error logging with context
- Cache hit/miss logging (debug level)
- Performance metrics via existing middleware
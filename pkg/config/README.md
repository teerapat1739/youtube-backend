# Configuration System

This package provides a centralized configuration system for the YouTube backend application that works seamlessly with both local development and GCP Cloud Run deployment.

## Features

- **Environment Detection**: Automatically detects whether running locally or in GCP Cloud Run
- **Flexible Configuration Loading**: 
  - Local: Loads from `.env.local` or `.env` files
  - Production: Uses environment variables set through GCP Cloud Run UI
- **Generic Naming**: Uses generic field names instead of brand-specific terminology
- **Validation**: Validates required configuration based on environment
- **Singleton Pattern**: Provides thread-safe singleton access to configuration

## Environment Detection

The system automatically detects the environment using the following priority:

1. **Explicit ENV variable**: `ENV=production|development|local`
2. **NODE_ENV compatibility**: `NODE_ENV=production|development|local`
3. **GCP Cloud Run detection**: Checks for Cloud Run-specific environment variables:
   - `K_SERVICE` (Cloud Run service name)
   - `K_REVISION` (Cloud Run revision)
   - `K_CONFIGURATION` (Cloud Run configuration)
4. **Default**: Falls back to `local` environment for safety

## Configuration Structure

```go
type Config struct {
    // Server configuration
    Port           string
    AllowedOrigins []string
    FrontendURL    string
    Environment    Environment

    // Authentication configuration
    OAuthConfig OAuthConfig
    JWTSecret   string

    // API configuration
    YouTubeAPIKey      string
    TargetChannelID    string
    YouTubeAPIBaseURL  string

    // Database configuration
    DatabaseURL string

    // Cache configuration
    RedisURL string

    // Logging configuration
    LogLevel string
    Debug    bool
}
```

## Usage

### Load Configuration (Usually done in main.go)

```go
import "github.com/gamemini/youtube/pkg/config"

func main() {
    appConfig, err := config.LoadConfig()
    if err != nil {
        log.Fatalf("Configuration error: %v", err)
    }
    
    // Use configuration
    log.Printf("Running on port: %s", appConfig.Port)
}
```

### Access Configuration (From anywhere in the application)

```go
import "github.com/gamemini/youtube/pkg/config"

func someFunction() {
    appConfig := config.GetConfig()
    
    // Use configuration values
    if appConfig.IsProduction() {
        // Production-specific logic
    }
    
    apiKey := appConfig.YouTubeAPIKey
    jwtSecret := appConfig.JWTSecret
}
```

## Environment Variables

### Required for Production

- `DATABASE_URL`: PostgreSQL connection string
- `GOOGLE_CLIENT_ID`: Google OAuth client ID
- `GOOGLE_CLIENT_SECRET`: Google OAuth client secret
- `JWT_SECRET`: Secret for signing JWT tokens
- `YOUTUBE_API_KEY`: YouTube Data API v3 key

### Optional

- `ENV`: Environment name (local/development/production)
- `PORT`: Server port (defaults to 8080)
- `FRONTEND_URL`: Frontend application URL
- `REDIRECT_URL`: OAuth redirect URL
- `TARGET_YOUTUBE_CHANNEL_ID`: Target YouTube channel for subscription checks
- `YOUTUBE_API_BASE_URL`: YouTube API base URL (defaults to googleapis.com)
- `ALLOWED_ORIGINS`: Comma-separated CORS origins
- `REDIS_URL`: Redis connection string for caching
- `LOG_LEVEL`: Logging level (defaults to "info")
- `APP_DEBUG`: Debug mode (true/false, defaults to false)

## Local Development

Create a `.env.local` file in the project root:

```bash
# Environment
ENV=local
APP_DEBUG=true

# Server
PORT=8080
FRONTEND_URL=http://localhost:3000
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173

# Google OAuth
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
REDIRECT_URL=http://localhost:8080/auth/google/callback

# JWT
JWT_SECRET=your_local_jwt_secret

# YouTube API
YOUTUBE_API_KEY=your_youtube_api_key
TARGET_YOUTUBE_CHANNEL_ID=your_target_channel_id

# Database
DATABASE_URL=postgresql://user:password@localhost:5432/dbname

# Redis (optional)
REDIS_URL=redis://localhost:6379
```

## GCP Cloud Run Deployment

Set environment variables through the Cloud Run UI or using gcloud CLI:

```bash
gcloud run services update YOUR_SERVICE_NAME \
  --set-env-vars="ENV=production,DATABASE_URL=your_production_db_url,GOOGLE_CLIENT_ID=your_client_id" \
  --region=your-region
```

The system will automatically detect the Cloud Run environment and use these variables directly.

## Validation

The configuration system validates required fields based on the environment:

- **Production**: All authentication and API keys are required
- **Development/Local**: Missing values generate warnings but don't fail startup

## Migration from Direct os.Getenv

Old code:
```go
clientID := os.Getenv("GOOGLE_CLIENT_ID")
jwtSecret := os.Getenv("JWT_SECRET")
```

New code:
```go
appConfig := config.GetConfig()
clientID := appConfig.OAuthConfig.ClientID
jwtSecret := appConfig.JWTSecret
```

This provides better type safety, validation, and centralized configuration management.
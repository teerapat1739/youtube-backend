# be-v2

A Go backend service for the YouTube activity platform with clean architecture, dependency injection, and Google OAuth integration.

## Features

- **Clean Architecture**: Well-structured codebase with clear separation of concerns
- **Dependency Injection**: All services initialized once at startup using a container pattern
- **Google OAuth**: Token verification for user authentication
- **YouTube Integration**: Subscription checking using YouTube Data API v3
- **CORS Support**: Properly configured CORS for frontend integration
- **Comprehensive Logging**: Structured logging with different levels
- **Error Handling**: Consistent error responses with proper HTTP status codes
- **Health Checks**: Built-in health check endpoint

## Project Structure

```
be-v2/
├── cmd/
│   └── api/
│       └── main.go                 # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go              # Configuration management
│   ├── domain/
│   │   ├── user.go               # User domain models
│   │   └── subscription.go       # Subscription domain models
│   ├── repository/
│   │   └── interfaces.go         # Repository interfaces
│   ├── service/
│   │   ├── interfaces.go         # Service interfaces
│   │   ├── auth/
│   │   │   └── auth_service.go   # Google OAuth service
│   │   └── youtube/
│   │       └── youtube_service.go # YouTube API service
│   ├── handler/
│   │   ├── auth_handler.go       # Authentication handlers
│   │   ├── subscription_handler.go # Subscription handlers
│   │   └── health_handler.go     # Health check handler
│   ├── middleware/
│   │   ├── cors.go              # CORS middleware
│   │   └── auth.go              # Authentication middleware
│   └── container/
│       └── container.go         # Dependency injection container
├── pkg/
│   ├── logger/
│   │   └── logger.go            # Structured logging
│   └── errors/
│       └── errors.go            # Error handling utilities
├── go.mod
├── go.sum
├── .env.example                 # Environment variables template
└── README.md
```

## Quick Start

### Prerequisites

- Go 1.21 or higher
- Google OAuth2 credentials
- YouTube Data API v3 key

### Installation

1. Clone or navigate to the be-v2 directory:
   ```bash
   cd /Users/gamemini/workspace/youtube/be-v2
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Copy the environment template:
   ```bash
   cp .env.example .env
   ```

4. Configure environment variables in `.env`:
   ```bash
   # Required configurations
   GOOGLE_CLIENT_ID=your-google-client-id.apps.googleusercontent.com
   YOUTUBE_API_KEY=your-youtube-api-key
   
   # Optional configurations
   PORT=8080
   LOG_LEVEL=info
   ALLOWED_ORIGINS=http://localhost:5173,http://localhost:5174
   YOUTUBE_CHANNEL_ID=UC-chqi3Gpb4F7yBqedlnq5g
   ```

5. Run the server:
   ```bash
   go run cmd/api/main.go
   ```

The server will start on `http://localhost:8080` by default.

## API Endpoints

### Public Endpoints

- `GET /health` - Health check
- `GET /api/youtube/channel/{channelId}` - Get YouTube channel information

### Protected Endpoints (Require Authentication)

- `GET /api/user/profile` - Get user profile
- `GET /api/youtube/subscription-check` - Check YouTube subscription status

### Authentication

Protected endpoints require a `Bearer` token in the `Authorization` header:

```
Authorization: Bearer your-google-access-token
```

## Usage Examples

### Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "version": "1.0.0",
  "service": "be-v2"
}
```

### Get User Profile

```bash
curl -H "Authorization: Bearer your-token" \
     http://localhost:8080/api/user/profile
```

### Check YouTube Subscription

```bash
curl -H "Authorization: Bearer your-token" \
     "http://localhost:8080/api/youtube/subscription-check?channel_id=UC-chqi3Gpb4F7yBqedlnq5g"
```

## Architecture

### Dependency Injection

The application uses a container pattern for dependency injection:

- All services are initialized once at startup
- Dependencies are injected through the container
- No global variables or singletons
- Easy to test and mock

### Clean Architecture

The codebase follows clean architecture principles:

- **Domain**: Business entities and rules
- **Service**: Application business logic
- **Handler**: HTTP request/response handling
- **Repository**: Data access interfaces
- **Middleware**: Cross-cutting concerns

### Error Handling

Consistent error handling with structured error types:

- Validation errors (400)
- Authentication errors (401)
- Authorization errors (403)
- Not found errors (404)
- Internal errors (500)
- External service errors (502)

## Development

### Running Tests

```bash
go test ./...
```

### Running with Hot Reload

Install air for hot reloading:

```bash
go install github.com/cosmtrek/air@latest
air
```

### Linting

```bash
golangci-lint run
```

## Configuration

All configuration is handled through environment variables:

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `PORT` | Server port | `8080` | No |
| `LOG_LEVEL` | Logging level | `info` | No |
| `ALLOWED_ORIGINS` | CORS allowed origins | `http://localhost:5173,http://localhost:5174` | No |
| `GOOGLE_CLIENT_ID` | Google OAuth client ID | - | Yes |
| `YOUTUBE_API_KEY` | YouTube Data API key | - | Yes |
| `YOUTUBE_CHANNEL_ID` | Default YouTube channel ID | `UC-chqi3Gpb4F7yBqedlnq5g` | No |

## Deployment

The application is designed to be easily deployable:

- No external dependencies required at runtime
- Environment-based configuration
- Health check endpoint for load balancers
- Graceful shutdown handling
- Structured logging for monitoring

## Contributing

1. Follow the existing code structure
2. Add tests for new functionality
3. Update documentation as needed
4. Ensure all lint checks pass

## License

This project is part of the YouTube activity platform.
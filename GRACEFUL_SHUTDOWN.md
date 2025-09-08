# Graceful Shutdown Implementation

This document describes the graceful shutdown mechanism implemented in the YouTube backend application.

## Overview

The application implements a comprehensive graceful shutdown system that ensures all resources are properly cleaned up when the application terminates, preventing connection leaks and potential data corruption.

## Features

### Signal Handling
- Listens for `SIGINT`, `SIGTERM`, and `os.Interrupt` signals
- Handles both normal termination and interrupt signals
- Supports graceful shutdown on server errors

### Resource Management
The `Resources` struct manages cleanup of:
- HTTP server (graceful shutdown with connection draining)
- Redis connections (with health checks)
- PostgreSQL connection pool (with health checks)
- Thread-safe cleanup with mutex protection

### Cleanup Process
1. **HTTP Server Shutdown**: Stops accepting new requests and waits for existing requests to complete
2. **Redis Connection Cleanup**: Performs health check, then closes connection
3. **Database Connection Cleanup**: Performs health check, then closes connection pool
4. **Error Collection**: Collects and reports any errors during cleanup

## Implementation Details

### Resources Structure
```go
type Resources struct {
    db          *database.PostgresDB
    redisClient *redis.Client
    server      *http.Server
    log         *logger.Logger
    mu          sync.Mutex
    closed      bool
}
```

### Key Methods

#### `Cleanup(ctx context.Context) error`
- Thread-safe cleanup of all resources
- Uses context with timeout for operation limits
- Performs health checks before closing connections
- Collects and returns any errors encountered

### Signal Handling
```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
```

### Dual Cleanup Safety
- Cleanup is called both in main flow and in defer statement
- Ensures cleanup happens even if main flow is interrupted
- Mutex prevents duplicate cleanup operations

## Timeouts and Configuration

### Cleanup Timeouts
- **Main cleanup**: 25 seconds for normal shutdown
- **Defer cleanup**: 30 seconds as fallback
- **Health checks**: 2 seconds per connection
- **HTTP server**: Uses Go's standard graceful shutdown

### HTTP Server Configuration
- **ReadTimeout**: 30 seconds
- **WriteTimeout**: 30 seconds  
- **IdleTimeout**: 60 seconds

## Usage

The graceful shutdown is automatically enabled when running the application. No additional configuration is required.

### Testing Graceful Shutdown

Use the provided test script:
```bash
./test_graceful_shutdown.sh
```

Or manually test with:
```bash
# Start the application
go run .

# In another terminal, send termination signal
kill -TERM <PID>
```

## Error Handling

### Error Collection
- All cleanup errors are collected and logged
- Application exits with error code 1 if cleanup fails
- Individual component failures don't prevent other cleanups

### Logging
- Comprehensive logging at each cleanup stage
- Error details with structured logging
- Health check results before connection closure

## Best Practices Implemented

1. **Resource Ordering**: HTTP server shutdown first to stop new requests
2. **Health Checks**: Verify connection state before cleanup
3. **Timeout Management**: Prevent indefinite blocking during shutdown
4. **Error Aggregation**: Collect all errors for comprehensive reporting
5. **Thread Safety**: Mutex protection for concurrent shutdown calls
6. **Dual Safety**: Both explicit and deferred cleanup calls

## Connection Pool Configuration

### PostgreSQL (pgxpool)
- **MaxConns**: 10 connections
- **MinConns**: 2 connections
- **MaxConnLifetime**: 1 hour
- **MaxConnIdleTime**: 30 minutes
- **HealthCheckPeriod**: 1 minute

### Redis
- **PoolSize**: 50 connections
- **MinIdleConns**: 5 connections
- **MaxRetries**: 3 attempts
- **DialTimeout**: 5 seconds
- **ReadTimeout**: 3 seconds
- **WriteTimeout**: 3 seconds

## Monitoring

The implementation logs the following events:
- Application startup
- Signal reception
- Shutdown initiation
- HTTP server shutdown status
- Redis connection health and closure status
- Database connection health and closure status
- Cleanup completion with success/error status

All logs use structured logging with relevant context fields for monitoring and debugging.
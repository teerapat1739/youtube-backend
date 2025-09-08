package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"be-v2/internal/service"
	"be-v2/pkg/errors"
	"be-v2/pkg/logger"
)

// ContextKey represents keys used in request context
type ContextKey string

const (
	// UserContextKey is the key for user information in context
	UserContextKey ContextKey = "user"
	// RequestIDContextKey is the key for request ID in context
	RequestIDContextKey ContextKey = "request_id"
)

// Auth creates an authentication middleware
func Auth(authService service.AuthService, logger *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeErrorResponse(w, errors.NewAuthenticationError("Authorization header is required"), logger)
				return
			}

			// Check if header starts with "Bearer "
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeErrorResponse(w, errors.NewAuthenticationError("Invalid authorization header format"), logger)
				return
			}

			// Extract token
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				writeErrorResponse(w, errors.NewAuthenticationError("Token is required"), logger)
				return
			}

			// Validate token
			ctx := r.Context()
			userProfile, err := authService.ValidateGoogleToken(ctx, token)
			if err != nil {
				logger.WithError(err).Error("Token validation failed")
				writeErrorResponse(w, errors.NewAuthenticationError("Invalid or expired token"), logger)
				return
			}

			// Add user to context
			ctx = context.WithValue(ctx, UserContextKey, userProfile)
			r = r.WithContext(ctx)

			logger.WithField("user_id", userProfile.Sub).Debug("User authenticated successfully")

			next.ServeHTTP(w, r)
		})
	}
}

// OptionalAuth creates an optional authentication middleware
// If token is provided, it validates it, otherwise continues without authentication
func OptionalAuth(authService service.AuthService, logger *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			// If no auth header, continue without authentication
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			// If auth header is present, validate it
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeErrorResponse(w, errors.NewAuthenticationError("Invalid authorization header format"), logger)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				writeErrorResponse(w, errors.NewAuthenticationError("Token is required"), logger)
				return
			}

			ctx := r.Context()
			userProfile, err := authService.ValidateGoogleToken(ctx, token)
			if err != nil {
				logger.WithError(err).Error("Token validation failed")
				writeErrorResponse(w, errors.NewAuthenticationError("Invalid or expired token"), logger)
				return
			}

			// Add user to context
			ctx = context.WithValue(ctx, UserContextKey, userProfile)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// RequestID creates a middleware that adds a unique request ID to each request
func RequestID(logger *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate request ID (simple timestamp-based for now)
			requestID := generateRequestID()

			// Add to context
			ctx := context.WithValue(r.Context(), RequestIDContextKey, requestID)
			r = r.WithContext(ctx)

			// Add to response header
			w.Header().Set("X-Request-ID", requestID)

			// Add to logger context
			logger = logger.WithField("request_id", requestID)

			next.ServeHTTP(w, r)
		})
	}
}

// generateRequestID generates a simple request ID
func generateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().Unix(), time.Now().Nanosecond())
}

// writeErrorResponse writes an error response to the client
func writeErrorResponse(w http.ResponseWriter, appErr *errors.AppError, logger *logger.Logger) {
	logger.WithError(appErr).Error("Request error")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.StatusCode)

	response := &errors.ErrorResponse{}
	response.Error.Type = appErr.Type
	response.Error.Message = appErr.Message
	response.Error.Details = appErr.Details
	response.Error.Timestamp = time.Now().UTC().Format(time.RFC3339)

	// You would typically use json.Marshal here, but for now we'll write a simple response
	w.Write([]byte(`{"error":{"type":"` + string(appErr.Type) + `","message":"` + appErr.Message + `","timestamp":"` + response.Error.Timestamp + `"}}`))
}

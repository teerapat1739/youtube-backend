package errors

import (
	"fmt"
	"net/http"
)

// ErrorType represents different types of application errors
type ErrorType string

const (
	ErrorTypeValidation    ErrorType = "validation"
	ErrorTypeAuthentication ErrorType = "authentication"
	ErrorTypeAuthorization  ErrorType = "authorization"
	ErrorTypeNotFound      ErrorType = "not_found"
	ErrorTypeInternal      ErrorType = "internal"
	ErrorTypeExternal      ErrorType = "external"
	ErrorTypeRateLimit     ErrorType = "rate_limit"
)

// AppError represents a structured application error
type AppError struct {
	Type       ErrorType `json:"type"`
	Message    string    `json:"message"`
	StatusCode int       `json:"status_code"`
	Internal   error     `json:"-"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("%s: %s (%s)", e.Type, e.Message, e.Internal.Error())
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the wrapped error
func (e *AppError) Unwrap() error {
	return e.Internal
}

// NewValidationError creates a new validation error
func NewValidationError(message string, details map[string]interface{}) *AppError {
	return &AppError{
		Type:       ErrorTypeValidation,
		Message:    message,
		StatusCode: http.StatusBadRequest,
		Details:    details,
	}
}

// NewAuthenticationError creates a new authentication error
func NewAuthenticationError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeAuthentication,
		Message:    message,
		StatusCode: http.StatusUnauthorized,
	}
}

// NewAuthorizationError creates a new authorization error
func NewAuthorizationError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeAuthorization,
		Message:    message,
		StatusCode: http.StatusForbidden,
	}
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeNotFound,
		Message:    message,
		StatusCode: http.StatusNotFound,
	}
}

// NewInternalError creates a new internal server error
func NewInternalError(message string, internal error) *AppError {
	return &AppError{
		Type:       ErrorTypeInternal,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
		Internal:   internal,
	}
}

// NewExternalError creates a new external service error
func NewExternalError(message string, internal error) *AppError {
	return &AppError{
		Type:       ErrorTypeExternal,
		Message:    message,
		StatusCode: http.StatusBadGateway,
		Internal:   internal,
	}
}

// NewRateLimitError creates a new rate limit error
func NewRateLimitError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeRateLimit,
		Message:    message,
		StatusCode: http.StatusTooManyRequests,
	}
}

// ErrorResponse represents the JSON error response
type ErrorResponse struct {
	Error struct {
		Type       ErrorType              `json:"type"`
		Message    string                 `json:"message"`
		Details    map[string]interface{} `json:"details,omitempty"`
		RequestID  string                 `json:"request_id,omitempty"`
		Timestamp  string                 `json:"timestamp"`
	} `json:"error"`
}
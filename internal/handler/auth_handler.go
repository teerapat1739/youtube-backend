package handler

import (
	"encoding/json"
	"net/http"

	"be-v2/internal/container"
	"be-v2/internal/domain"
	"be-v2/internal/middleware"
	"be-v2/pkg/errors"
)

// AuthHandler handles authentication related requests
type AuthHandler struct {
	container *container.Container
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(container *container.Container) *AuthHandler {
	return &AuthHandler{
		container: container,
	}
}

// UserProfileResponse represents the user profile response
type UserProfileResponse struct {
	User    *domain.UserProfile `json:"user"`
	Success bool                `json:"success"`
	Message string              `json:"message"`
}

// GetProfile handles GET /api/user/profile
func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	logger := h.container.GetLogger()
	
	// Get user from context (set by auth middleware)
	user, ok := r.Context().Value(middleware.UserContextKey).(*domain.UserProfile)
	if !ok {
		logger.Error("User not found in context")
		h.writeErrorResponse(w, errors.NewAuthenticationError("User not authenticated"))
		return
	}
	
	logger.WithField("user_id", user.Sub).Debug("Getting user profile")
	
	response := UserProfileResponse{
		User:    user,
		Success: true,
		Message: "User profile retrieved successfully",
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode user profile response")
		h.writeErrorResponse(w, errors.NewInternalError("Failed to encode response", err))
		return
	}
	
	logger.WithField("user_id", user.Sub).Debug("User profile retrieved successfully")
}

// writeErrorResponse writes an error response to the client
func (h *AuthHandler) writeErrorResponse(w http.ResponseWriter, appErr *errors.AppError) {
	logger := h.container.GetLogger()
	logger.WithError(appErr).Error("Request error")
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.StatusCode)
	
	response := map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"type":    string(appErr.Type),
			"message": appErr.Message,
		},
	}
	
	if appErr.Details != nil {
		response["error"].(map[string]interface{})["details"] = appErr.Details
	}
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode error response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
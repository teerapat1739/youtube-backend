package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"be-v2/internal/container"
	"be-v2/internal/domain"
	"be-v2/internal/middleware"
	"be-v2/pkg/errors"
)

// SubscriptionHandler handles subscription related requests
type SubscriptionHandler struct {
	container *container.Container
}

// NewSubscriptionHandler creates a new subscription handler
func NewSubscriptionHandler(container *container.Container) *SubscriptionHandler {
	return &SubscriptionHandler{
		container: container,
	}
}

// SubscriptionCheckResponseWrapper wraps the subscription check response
type SubscriptionCheckResponseWrapper struct {
	Data    *domain.SubscriptionCheckResponse `json:"data"`
	Success bool                              `json:"success"`
	Message string                            `json:"message"`
}

// CheckSubscription handles GET /api/youtube/subscription-check
func (h *SubscriptionHandler) CheckSubscription(w http.ResponseWriter, r *http.Request) {
	logger := h.container.GetLogger()
	config := h.container.GetConfig()
	youtubeService := h.container.GetYouTubeService()

	// Get user from context (set by auth middleware)
	user, ok := r.Context().Value(middleware.UserContextKey).(*domain.UserProfile)
	if !ok {
		logger.Error("User not found in context")
		h.writeErrorResponse(w, errors.NewAuthenticationError("User not authenticated"))
		return
	}

	// Get access token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		h.writeErrorResponse(w, errors.NewAuthenticationError("Authorization header is required"))
		return
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		h.writeErrorResponse(w, errors.NewAuthenticationError("Invalid authorization header format"))
		return
	}

	accessToken := strings.TrimPrefix(authHeader, "Bearer ")

	// Get channel ID from query parameter or use default
	channelID := config.YouTubeChannelID

	if channelID == "" {
		h.writeErrorResponse(w, errors.NewValidationError("Channel ID is required", map[string]interface{}{
			"field": "channel_id",
		}))
		return
	}

	logger.WithFields(map[string]interface{}{
		"user_id":    user.Sub,
		"channel_id": channelID,
	}).Debug("Checking YouTube subscription")

	// Check subscription with caching if Redis is available
	var subscriptionResponse *domain.SubscriptionCheckResponse
	var err error
	
	cacheService := h.container.GetCacheService()
	if cacheService != nil {
		// Use Redis caching
		subscriptionResponse, err = cacheService.GetSubscriptionWithCache(
			r.Context(), 
			user.Sub, 
			channelID,
			youtubeService.CheckSubscription,
			accessToken,
		)
	} else {
		// Fallback to direct YouTube API call without caching
		logger.Debug("Redis not available, using direct YouTube API call")
		subscriptionResponse, err = youtubeService.CheckSubscription(r.Context(), accessToken, channelID)
	}
	
	if err != nil {
		logger.WithError(err).Error("Failed to check subscription")
		if appErr, ok := err.(*errors.AppError); ok {
			h.writeErrorResponse(w, appErr)
		} else {
			h.writeErrorResponse(w, errors.NewInternalError("Failed to check subscription", err))
		}
		return
	}

	response := SubscriptionCheckResponseWrapper{
		Data:    subscriptionResponse,
		Success: true,
		Message: "Subscription status checked successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode subscription response")
		h.writeErrorResponse(w, errors.NewInternalError("Failed to encode response", err))
		return
	}

	logger.WithFields(map[string]interface{}{
		"user_id":       user.Sub,
		"channel_id":    channelID,
		"is_subscribed": subscriptionResponse.IsSubscribed,
	}).Info("Subscription check completed successfully")
}

// InvalidateSubscriptionCache handles cache invalidation for a specific user and channel
func (h *SubscriptionHandler) InvalidateSubscriptionCache(w http.ResponseWriter, r *http.Request) {
	logger := h.container.GetLogger()
	cacheService := h.container.GetCacheService()
	
	if cacheService == nil {
		h.writeErrorResponse(w, errors.NewInternalError("Redis cache not available", nil))
		return
	}

	// Get user from context (set by auth middleware)
	user, ok := r.Context().Value(middleware.UserContextKey).(*domain.UserProfile)
	if !ok {
		logger.Error("User not found in context")
		h.writeErrorResponse(w, errors.NewAuthenticationError("User not authenticated"))
		return
	}

	// Get channel ID from query parameter or use default
	config := h.container.GetConfig()
	channelID := config.YouTubeChannelID

	if channelID == "" {
		h.writeErrorResponse(w, errors.NewValidationError("Channel ID is required", map[string]interface{}{
			"field": "channel_id",
		}))
		return
	}

	logger.WithFields(map[string]interface{}{
		"user_id":    user.Sub,
		"channel_id": channelID,
	}).Debug("Invalidating subscription cache")

	// Invalidate cache
	err := cacheService.InvalidateSubscriptionCache(r.Context(), user.Sub, channelID)
	if err != nil {
		logger.WithError(err).Error("Failed to invalidate subscription cache")
		h.writeErrorResponse(w, errors.NewInternalError("Failed to invalidate cache", err))
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Subscription cache invalidated successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode cache invalidation response")
		h.writeErrorResponse(w, errors.NewInternalError("Failed to encode response", err))
		return
	}

	logger.WithFields(map[string]interface{}{
		"user_id":    user.Sub,
		"channel_id": channelID,
	}).Info("Subscription cache invalidated successfully")
}

// GetChannelInfo handles GET /api/youtube/channel/{channelId}
func (h *SubscriptionHandler) GetChannelInfo(w http.ResponseWriter, r *http.Request) {
	logger := h.container.GetLogger()
	youtubeService := h.container.GetYouTubeService()

	// Get channel ID from URL path
	channelID := strings.TrimPrefix(r.URL.Path, "/api/youtube/channel/")
	if channelID == "" || channelID == "/api/youtube/channel/" {
		h.writeErrorResponse(w, errors.NewValidationError("Channel ID is required", map[string]interface{}{
			"field": "channel_id",
		}))
		return
	}

	logger.WithField("channel_id", channelID).Debug("Getting YouTube channel info")

	// Get channel info
	channelInfo, err := youtubeService.GetChannelInfo(r.Context(), channelID)
	if err != nil {
		logger.WithError(err).Error("Failed to get channel info")
		if appErr, ok := err.(*errors.AppError); ok {
			h.writeErrorResponse(w, appErr)
		} else {
			h.writeErrorResponse(w, errors.NewInternalError("Failed to get channel info", err))
		}
		return
	}

	response := map[string]interface{}{
		"data":    channelInfo,
		"success": true,
		"message": "Channel information retrieved successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode channel info response")
		h.writeErrorResponse(w, errors.NewInternalError("Failed to encode response", err))
		return
	}

	logger.WithField("channel_id", channelID).Debug("Channel info retrieved successfully")
}

// writeErrorResponse writes an error response to the client
func (h *SubscriptionHandler) writeErrorResponse(w http.ResponseWriter, appErr *errors.AppError) {
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

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gamemini/youtube/pkg/middleware"
	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/repository"
	"github.com/gamemini/youtube/pkg/services"
)

// AdminSubscriptionCheckRequest represents the request to check user subscription
type AdminSubscriptionCheckRequest struct {
	UserID    string `json:"user_id" validate:"required"`
	ChannelID string `json:"channel_id" validate:"required"`
}

// AdminSubscriptionCheckResponse represents the response for admin subscription check
type AdminSubscriptionCheckResponse struct {
	Success            bool                   `json:"success"`
	Message            string                 `json:"message,omitempty"`
	Data               *AdminSubscriptionData `json:"data,omitempty"`
	Error              string                 `json:"error,omitempty"`
	AdminRequester     string                 `json:"admin_requester,omitempty"`
	RequestTimestamp   time.Time              `json:"request_timestamp"`
}

// AdminSubscriptionData contains the subscription check results
type AdminSubscriptionData struct {
	UserID             string                 `json:"user_id"`
	ChannelID          string                 `json:"channel_id"`
	IsSubscribed       bool                   `json:"is_subscribed"`
	VerificationMethod string                 `json:"verification_method"`
	UserInfo           map[string]interface{} `json:"user_info,omitempty"`
	ChannelInfo        map[string]interface{} `json:"channel_info,omitempty"`
	VerifiedAt         time.Time              `json:"verified_at"`
	TokenStatus        string                 `json:"token_status,omitempty"`
	ErrorDetails       string                 `json:"error_details,omitempty"`
}

// HandleAdminSubscriptionCheck handles admin subscription verification requests
func HandleAdminSubscriptionCheck(w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now()
	log.Printf("🔐 [ADMIN-SUB-CHECK] Starting admin subscription check from %s", r.RemoteAddr)
	
	// Extract admin claims from middleware context
	adminClaims, ok := middleware.GetAdminClaims(r)
	if !ok {
		log.Printf("❌ [ADMIN-SUB-CHECK] Admin claims not found in context")
		sendAdminErrorResponse(w, "Admin authentication required", http.StatusUnauthorized, "")
		return
	}

	// Validate admin email against environment variable
	if !isValidAdminEmail(adminClaims.Email) {
		log.Printf("❌ [ADMIN-SUB-CHECK] Invalid admin email: %s", adminClaims.Email)
		sendAdminErrorResponse(w, "Admin email not authorized", http.StatusForbidden, adminClaims.Email)
		return
	}

	log.Printf("✅ [ADMIN-SUB-CHECK] Admin authenticated: %s", adminClaims.Email)

	// Parse request body
	var request AdminSubscriptionCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("❌ [ADMIN-SUB-CHECK] Invalid request body: %v", err)
		sendAdminErrorResponse(w, "Invalid request body", http.StatusBadRequest, adminClaims.Email)
		return
	}

	// Validate required fields
	if request.UserID == "" {
		sendAdminErrorResponse(w, "user_id is required", http.StatusBadRequest, adminClaims.Email)
		return
	}
	if request.ChannelID == "" {
		sendAdminErrorResponse(w, "channel_id is required", http.StatusBadRequest, adminClaims.Email)
		return
	}

	log.Printf("🎯 [ADMIN-SUB-CHECK] Checking subscription - User: %s, Channel: %s", request.UserID, request.ChannelID)

	// Initialize services
	userRepo := repository.NewUserRepository()
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-development-secret-change-in-production"
	}
	userService := services.NewUserService(userRepo, jwtSecret)

	// Get user info first
	user, err := userRepo.GetUserByID(r.Context(), request.UserID)
	if err != nil {
		log.Printf("❌ [ADMIN-SUB-CHECK] Failed to get user: %v", err)
		sendAdminErrorResponse(w, "User not found", http.StatusNotFound, adminClaims.Email)
		return
	}

	if user == nil {
		log.Printf("❌ [ADMIN-SUB-CHECK] User not found: %s", request.UserID)
		sendAdminErrorResponse(w, "User not found", http.StatusNotFound, adminClaims.Email)
		return
	}

	// Create user info map (safe for admin view)
	userInfo := map[string]interface{}{
		"id":                  user.ID,
		"google_id":           user.GoogleID,
		"email":               user.Email,
		"name":                getUserDisplayName(user),
		"profile_completed":   user.ProfileCompleted,
		"youtube_subscribed":  user.YouTubeSubscribed,
		"subscription_verified_at": user.SubscriptionVerifiedAt,
		"has_google_tokens":   user.GoogleAccessToken != nil && user.GoogleRefreshToken != nil,
		"created_at":          user.CreatedAt,
	}

	// Check subscription using OAuth tokens
	isSubscribed, verificationMethod, tokenStatus, err := checkUserSubscriptionForAdmin(r.Context(), userService, request.UserID, request.ChannelID)
	
	// Get channel info regardless of subscription status
	channelInfo, channelErr := getChannelInfoForAdmin(r.Context(), request.ChannelID)
	if channelErr != nil {
		log.Printf("⚠️ [ADMIN-SUB-CHECK] Failed to get channel info: %v", channelErr)
		channelInfo = map[string]interface{}{
			"id":    request.ChannelID,
			"error": "Could not retrieve channel info",
		}
	}

	// Prepare response data
	responseData := &AdminSubscriptionData{
		UserID:             request.UserID,
		ChannelID:          request.ChannelID,
		IsSubscribed:       isSubscribed,
		VerificationMethod: verificationMethod,
		UserInfo:           userInfo,
		ChannelInfo:        channelInfo,
		VerifiedAt:         time.Now(),
		TokenStatus:        tokenStatus,
	}

	if err != nil {
		log.Printf("⚠️ [ADMIN-SUB-CHECK] Subscription check had issues: %v", err)
		responseData.ErrorDetails = err.Error()
	}

	// Create response
	response := AdminSubscriptionCheckResponse{
		Success:          true,
		Message:          "Admin subscription check completed",
		Data:             responseData,
		AdminRequester:   adminClaims.Email,
		RequestTimestamp: requestStart,
	}

	// Log the admin action for audit purposes
	log.Printf("📊 [ADMIN-AUDIT] Admin %s checked subscription for user %s on channel %s - Result: %t (took %v)",
		adminClaims.Email, request.UserID, request.ChannelID, isSubscribed, time.Since(requestStart))

	log.Printf("✅ [ADMIN-SUB-CHECK] Admin subscription check completed successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// checkUserSubscriptionForAdmin performs subscription check specifically for admin requests
func checkUserSubscriptionForAdmin(ctx context.Context, userService *services.UserService, userID, channelID string) (bool, string, string, error) {
	log.Printf("🔍 [ADMIN-OAUTH-CHECK] Starting OAuth-based subscription check for admin request")
	
	// Check if OAuth token is expired and refresh if needed
	isExpired, err := userService.IsOAuthTokenExpired(ctx, userID)
	if err != nil {
		log.Printf("❌ [ADMIN-OAUTH-CHECK] Failed to check token expiry: %v", err)
		return false, "error", "token_check_failed", fmt.Errorf("failed to check token expiry: %w", err)
	}
	
	var tokenData *models.OAuthTokenData
	var tokenStatus string
	
	if isExpired {
		log.Printf("🔄 [ADMIN-OAUTH-CHECK] OAuth token expired, attempting refresh")
		tokenData, err = userService.RefreshUserOAuthToken(ctx, userID)
		if err != nil {
			log.Printf("❌ [ADMIN-OAUTH-CHECK] Failed to refresh OAuth token: %v", err)
			return false, "refresh_failed", "token_refresh_failed", fmt.Errorf("failed to refresh OAuth token: %w", err)
		}
		tokenStatus = "refreshed"
		log.Printf("✅ [ADMIN-OAUTH-CHECK] OAuth token refreshed successfully")
	} else {
		// Get current tokens
		tokenData, err = userService.GetUserOAuthTokens(ctx, userID)
		if err != nil {
			log.Printf("❌ [ADMIN-OAUTH-CHECK] Failed to get OAuth tokens: %v", err)
			return false, "no_tokens", "no_tokens_stored", fmt.Errorf("failed to get OAuth tokens: %w", err)
		}
		tokenStatus = "valid"
		log.Printf("✅ [ADMIN-OAUTH-CHECK] OAuth tokens retrieved successfully")
	}
	
	// Check if we have required tokens
	if tokenData.AccessToken == "" {
		return false, "no_access_token", "missing_access_token", fmt.Errorf("no access token available")
	}
	if tokenData.RefreshToken == "" {
		tokenStatus = "no_refresh_token"
		log.Printf("⚠️ [ADMIN-OAUTH-CHECK] No refresh token available - this may cause future issues")
	}
	
	// Use existing subscription check logic
	isSubscribed, verificationMethod, err := checkUserSubscriptionWithOAuth(ctx, userID, channelID)
	return isSubscribed, verificationMethod, tokenStatus, err
}

// getChannelInfoForAdmin retrieves channel information for admin requests
func getChannelInfoForAdmin(ctx context.Context, channelID string) (map[string]interface{}, error) {
	log.Printf("📊 [ADMIN-CHANNEL-INFO] Fetching channel info for admin: %s", channelID)
	
	// Get YouTube API key from environment
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("YOUTUBE_API_KEY not configured")
	}

	// Create YouTube service with API key
	youtubeService, err := createYouTubeServiceWithAPIKey(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	// Get channel info using existing function
	return getChannelInfoWithAPIKey(youtubeService, channelID)
}

// isValidAdminEmail checks if the email is authorized as admin
func isValidAdminEmail(email string) bool {
	adminEmails := os.Getenv("ADMIN_EMAILS")
	if adminEmails == "" {
		log.Printf("⚠️ [ADMIN-AUTH] ADMIN_EMAILS not configured, rejecting all admin requests")
		return false
	}

	// Split comma-separated emails and check
	emailList := strings.Split(adminEmails, ",")
	for _, adminEmail := range emailList {
		if strings.TrimSpace(adminEmail) == email {
			return true
		}
	}
	
	log.Printf("❌ [ADMIN-AUTH] Email %s not in admin list: %s", email, adminEmails)
	return false
}

// getUserDisplayName returns display name for user (safe for admin view)
func getUserDisplayName(user *models.User) string {
	if user.FirstName != nil && user.LastName != nil {
		return fmt.Sprintf("%s %s", *user.FirstName, *user.LastName)
	}
	if user.FirstName != nil {
		return *user.FirstName
	}
	// Return masked email for privacy
	if strings.Contains(user.Email, "@") {
		parts := strings.Split(user.Email, "@")
		if len(parts[0]) > 3 {
			return parts[0][:3] + "***@" + parts[1]
		}
	}
	return "User"
}

// sendAdminErrorResponse sends standardized error response for admin endpoints
func sendAdminErrorResponse(w http.ResponseWriter, message string, statusCode int, adminEmail string) {
	response := AdminSubscriptionCheckResponse{
		Success:          false,
		Message:          message,
		Error:            message,
		AdminRequester:   adminEmail,
		RequestTimestamp: time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
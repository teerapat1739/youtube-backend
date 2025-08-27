package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/repository"
	"github.com/gamemini/youtube/pkg/services"
	"github.com/gorilla/mux"
	"golang.org/x/oauth2"
)

// TestSubscriptionResponse represents the response for the test subscription endpoint
type TestSubscriptionResponse struct {
	Success       bool   `json:"success"`
	UserID        string `json:"user_id"`
	ChannelID     string `json:"channel_id"`
	IsSubscribed  bool   `json:"is_subscribed"`
	Message       string `json:"message"`
	TokenStatus   string `json:"token_status,omitempty"`
	ErrorDetails  string `json:"error_details,omitempty"`
}

// HandleTestSubscription is a testing endpoint that checks subscription without authentication
// GET /api/test/subscription/{user_id}/{channel_id}
func HandleTestSubscription(w http.ResponseWriter, r *http.Request) {
	log.Printf("🧪 [TEST-SUBSCRIPTION] Starting test subscription check from %s", r.RemoteAddr)
	log.Printf("🧪 [TEST-SUBSCRIPTION] Request URL: %s", r.URL.String())
	
	// Extract user_id from URL parameters (channel_id is now from env)
	vars := mux.Vars(r)
	userID := vars["user_id"]
	
	// Get channel ID from environment variable
	channelID := os.Getenv("TARGET_YOUTUBE_CHANNEL_ID")
	if channelID == "" {
		log.Printf("❌ [TEST-SUBSCRIPTION] TARGET_YOUTUBE_CHANNEL_ID not set in environment")
		sendTestErrorResponse(w, userID, "", "Target channel ID not configured", http.StatusInternalServerError)
		return
	}
	
	if userID == "" {
		log.Printf("❌ [TEST-SUBSCRIPTION] Missing required user_id parameter")
		sendTestErrorResponse(w, "", channelID, "Missing user_id in URL parameters", http.StatusBadRequest)
		return
	}
	
	log.Printf("🧪 [TEST-SUBSCRIPTION] Testing subscription - UserID: %s, ChannelID: %s", userID, channelID)
	
	// Initialize services
	userRepo := repository.NewUserRepository()
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-development-secret-change-in-production"
	}
	userService := services.NewUserService(userRepo, jwtSecret)
	
	// Step 1: Look up user in database
	log.Printf("🔍 [TEST-SUBSCRIPTION] Looking up user in database")
	user, err := userRepo.GetUserByID(r.Context(), userID)
	if err != nil {
		log.Printf("❌ [TEST-SUBSCRIPTION] Failed to get user: %v", err)
		sendTestErrorResponse(w, userID, channelID, "User not found in database", http.StatusNotFound)
		return
	}
	
	if user == nil {
		log.Printf("❌ [TEST-SUBSCRIPTION] User not found")
		sendTestErrorResponse(w, userID, channelID, "User not found", http.StatusNotFound)
		return
	}
	
	log.Printf("✅ [TEST-SUBSCRIPTION] User found - Email: %s, GoogleID: %s", user.Email, user.GoogleID)
	
	// Step 2: Check if user has stored OAuth tokens
	if user.GoogleRefreshToken == nil || *user.GoogleRefreshToken == "" {
		log.Printf("❌ [TEST-SUBSCRIPTION] No refresh token stored for user")
		sendTestErrorResponse(w, userID, channelID, "No refresh token available for user", http.StatusUnauthorized)
		return
	}
	
	if user.GoogleAccessToken == nil || *user.GoogleAccessToken == "" {
		log.Printf("❌ [TEST-SUBSCRIPTION] No access token stored for user")
		sendTestErrorResponse(w, userID, channelID, "No access token available for user", http.StatusUnauthorized)
		return
	}
	
	log.Printf("✅ [TEST-SUBSCRIPTION] OAuth tokens found - AccessToken length: %d, RefreshToken length: %d", 
		len(*user.GoogleAccessToken), len(*user.GoogleRefreshToken))
	
	// Step 3: Check if token is expired and refresh if needed
	isExpired, err := userService.IsOAuthTokenExpired(r.Context(), userID)
	if err != nil {
		log.Printf("❌ [TEST-SUBSCRIPTION] Failed to check token expiry: %v", err)
		sendTestErrorResponse(w, userID, channelID, fmt.Sprintf("Failed to check token expiry: %v", err), http.StatusInternalServerError)
		return
	}
	
	var tokenData *models.OAuthTokenData
	tokenStatus := "current"
	
	if isExpired {
		log.Printf("🔄 [TEST-SUBSCRIPTION] OAuth token expired, attempting refresh")
		tokenStatus = "refreshed"
		tokenData, err = userService.RefreshUserOAuthToken(r.Context(), userID)
		if err != nil {
			log.Printf("❌ [TEST-SUBSCRIPTION] Failed to refresh OAuth token: %v", err)
			sendTestErrorResponse(w, userID, channelID, fmt.Sprintf("Failed to refresh OAuth token: %v", err), http.StatusUnauthorized)
			return
		}
		log.Printf("✅ [TEST-SUBSCRIPTION] OAuth token refreshed successfully")
	} else {
		log.Printf("✅ [TEST-SUBSCRIPTION] OAuth token is still valid")
		// Get current tokens
		tokenData, err = userService.GetUserOAuthTokens(r.Context(), userID)
		if err != nil {
			log.Printf("❌ [TEST-SUBSCRIPTION] Failed to get OAuth tokens: %v", err)
			sendTestErrorResponse(w, userID, channelID, fmt.Sprintf("Failed to get OAuth tokens: %v", err), http.StatusInternalServerError)
			return
		}
	}
	
	// Step 4: Use access token to check subscription
	isSubscribed, err := checkSubscriptionWithToken(r.Context(), tokenData.AccessToken)
	if err != nil {
		log.Printf("❌ [TEST-SUBSCRIPTION] Subscription check failed: %v", err)
		sendTestErrorResponse(w, userID, channelID, fmt.Sprintf("Subscription check failed: %v", err), http.StatusInternalServerError)
		return
	}
	
	log.Printf("✅ [TEST-SUBSCRIPTION] Subscription check completed - IsSubscribed: %t", isSubscribed)
	
	// Step 5: Update user subscription status in database if subscribed
	if isSubscribed {
		err = userRepo.UpdateYouTubeSubscription(r.Context(), userID, true)
		if err != nil {
			log.Printf("⚠️ [TEST-SUBSCRIPTION] Failed to update subscription status in DB: %v", err)
			// Don't fail the request, just log the warning
		} else {
			log.Printf("✅ [TEST-SUBSCRIPTION] Updated subscription status in database")
		}
	}
	
	// Step 6: Return success response
	response := TestSubscriptionResponse{
		Success:      true,
		UserID:       userID,
		ChannelID:    channelID,
		IsSubscribed: isSubscribed,
		Message:      "Subscription check completed",
		TokenStatus:  tokenStatus,
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
	
	log.Printf("📤 [TEST-SUBSCRIPTION] Response sent successfully")
}

// checkSubscriptionWithToken checks if a user is subscribed to a channel using an access token
func checkSubscriptionWithToken(ctx context.Context, accessToken string) (bool, error) {
	// Get channel ID from environment variable
	channelID := os.Getenv("TARGET_YOUTUBE_CHANNEL_ID")
	if channelID == "" {
		log.Printf("❌ [TOKEN-SUBSCRIPTION] TARGET_YOUTUBE_CHANNEL_ID not set in environment")
		return false, fmt.Errorf("target channel ID not configured")
	}
	
	log.Printf("🔍 [TOKEN-SUBSCRIPTION] Checking subscription with access token for channel: %s", channelID)
	
	// Create OAuth2 token
	token := &oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "Bearer",
	}
	
	// Create YouTube service
	youtubeService, err := createYouTubeService(ctx, token)
	if err != nil {
		log.Printf("❌ [TOKEN-SUBSCRIPTION] Failed to create YouTube service: %v", err)
		return false, fmt.Errorf("failed to create YouTube service: %w", err)
	}
	
	log.Printf("✅ [TOKEN-SUBSCRIPTION] YouTube service created successfully")
	
	// Check subscription using the existing function
	isSubscribed, err := checkSubscription(youtubeService)
	if err != nil {
		log.Printf("❌ [TOKEN-SUBSCRIPTION] Subscription check failed: %v", err)
		return false, fmt.Errorf("subscription check failed: %w", err)
	}
	
	log.Printf("✅ [TOKEN-SUBSCRIPTION] Subscription check completed - subscribed: %t", isSubscribed)
	return isSubscribed, nil
}

// sendTestErrorResponse sends a standardized error response for test endpoint
func sendTestErrorResponse(w http.ResponseWriter, userID, channelID, message string, statusCode int) {
	response := TestSubscriptionResponse{
		Success:      false,
		UserID:       userID,
		ChannelID:    channelID,
		IsSubscribed: false,
		Message:      "Subscription check failed",
		ErrorDetails: message,
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
	
	log.Printf("📤 [TEST-SUBSCRIPTION] Error response sent: %s (status: %d)", message, statusCode)
}
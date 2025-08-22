package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/repository"
	"github.com/gamemini/youtube/pkg/services"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// ActivityResponse is the response for activity endpoints
type ActivityResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// HandleSubscriptionCheck verifies if a user is subscribed to a specific channel
func HandleSubscriptionCheck(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔍 [SUBSCRIPTION-CHECK] Starting subscription check request from %s", r.RemoteAddr)
	log.Printf("🔍 [SUBSCRIPTION-CHECK] Request URL: %s", r.URL.String())
	log.Printf("🔍 [SUBSCRIPTION-CHECK] Request method: %s", r.Method)
	log.Printf("🔍 [SUBSCRIPTION-CHECK] Request headers: %+v", r.Header)
	
	// Extract and validate JWT token to get user ID
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Printf("❌ [SUBSCRIPTION-CHECK] No authorization header found")
		sendErrorResponse(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	log.Printf("🔑 [SUBSCRIPTION-CHECK] Auth header found, length: %d", len(authHeader))
	
	// Extract the JWT token value (remove "Bearer " prefix if it exists)
	tokenValue := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenValue = authHeader[7:]
		log.Printf("🔑 [SUBSCRIPTION-CHECK] Bearer JWT token extracted, length: %d", len(tokenValue))
	}
	
	// Get user ID from JWT token
	userID, err := extractUserIDFromJWT(tokenValue)
	if err != nil {
		log.Printf("❌ [SUBSCRIPTION-CHECK] Failed to extract user ID from JWT: %v", err)
		sendErrorResponse(w, "Invalid authorization token", http.StatusUnauthorized)
		return
	}
	
	log.Printf("🔑 [SUBSCRIPTION-CHECK] User ID extracted from JWT: %s", userID)

	// Get target channel ID from request query parameters
	targetChannelID := r.URL.Query().Get("channel_id")
	if targetChannelID == "" {
		log.Printf("❌ [SUBSCRIPTION-CHECK] No channel_id provided in query parameters")
		sendErrorResponse(w, "Channel ID is required", http.StatusBadRequest)
		return
	}
	
	log.Printf("🎯 [SUBSCRIPTION-CHECK] Checking subscription for user %s to channel: %s", userID, targetChannelID)

	// Check subscription using OAuth tokens from database
	isSubscribed, verificationMethod, err := checkUserSubscriptionWithOAuth(r.Context(), userID, targetChannelID)
	if err != nil {
		log.Printf("❌ [SUBSCRIPTION-CHECK] Subscription check failed: %v", err)
		errMsg, statusCode := handleYouTubeAPIError(err)
		log.Printf("❌ [SUBSCRIPTION-CHECK] Returning error: %s (status: %d)", errMsg, statusCode)
		sendErrorResponse(w, errMsg, statusCode)
		return
	}

	log.Printf("✅ [SUBSCRIPTION-CHECK] Subscription check completed - subscribed: %t, method: %s", isSubscribed, verificationMethod)

	// Create response with detailed information
	response := ActivityResponse{
		Success: true,
		Message: "Subscription check completed",
		Data: map[string]interface{}{
			"is_subscribed":       isSubscribed,
			"channel_id":          targetChannelID,
			"user_id":             userID,
			"verification_method": verificationMethod,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	log.Printf("📤 [SUBSCRIPTION-CHECK] Response sent successfully")
}

// HandleJoinActivity handles a user joining an activity and submitting their contact information
func HandleJoinActivity(w http.ResponseWriter, r *http.Request) {
	// Extract token from request (assuming it's in the Authorization header)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		sendErrorResponse(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	// Extract the token value (remove "Bearer " prefix if it exists)
	tokenValue := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenValue = authHeader[7:]
	}

	// Create an OAuth2 token
	token := &oauth2.Token{
		AccessToken: tokenValue,
	}

	// Create YouTube service
	youtubeService, err := createYouTubeService(r.Context(), token)
	if err != nil {
		sendErrorResponse(w, "Failed to create YouTube service", http.StatusInternalServerError)
		return
	}

	// Parse activity ID from path parameters or query
	activityID := r.URL.Query().Get("activity_id")
	if activityID == "" {
		sendErrorResponse(w, "Activity ID is required", http.StatusBadRequest)
		return
	}

	// Get channel ID from database based on activity ID
	// In a real app, you would look up the activity in the database
	channelID := "xxx" // Hardcoded for demonstration, should be retrieved from database

	// Check if the user is subscribed to the specified channel
	isSubscribed, err := checkSubscription(youtubeService, channelID)
	if err != nil {
		errMsg, statusCode := handleYouTubeAPIError(err)
		sendErrorResponse(w, errMsg, statusCode)
		return
	}

	if !isSubscribed {
		sendErrorResponse(w, "You must be subscribed to the channel to join this activity", http.StatusForbidden)
		return
	}

	// Get user info from YouTube
	userInfo, err := getUserInfo(youtubeService)
	if err != nil {
		errMsg, statusCode := handleYouTubeAPIError(err)
		sendErrorResponse(w, errMsg, statusCode)
		return
	}

	// Parse the contact form data
	var contactForm models.ContactForm
	if err := json.NewDecoder(r.Body).Decode(&contactForm); err != nil {
		sendErrorResponse(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Validate form data
	if contactForm.Name == "" || contactForm.Email == "" || !contactForm.AcceptTerms {
		sendErrorResponse(w, "Required fields missing or terms not accepted", http.StatusBadRequest)
		return
	}

	// Create a participant record
	participant := models.Participant{
		ID:         uuid.New().String(),
		ActivityID: activityID,
		UserID:     userInfo["id"].(string),
		Email:      contactForm.Email,
		Name:       contactForm.Name,
		Phone:      contactForm.Phone,
		CreatedAt:  time.Now(),
	}

	// In a real application, you would save the participant to a database here
	// For demonstration, we'll just return success

	response := ActivityResponse{
		Success: true,
		Message: "Successfully joined the activity",
		Data:    participant,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// Helper functions

// sendErrorResponse sends a standardized error response
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := ActivityResponse{
		Success: false,
		Message: message,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// handleYouTubeAPIError handles specific YouTube API errors and returns appropriate HTTP status codes
func handleYouTubeAPIError(err error) (string, int) {
	if err == nil {
		return "", 0
	}

	// Check for specific YouTube API errors
	errMsg := err.Error()

	// OAuth token related errors
	if contains(errMsg, "no OAuth tokens stored") {
		return "Please sign in with Google to verify your YouTube subscriptions", http.StatusUnauthorized
	}
	if contains(errMsg, "no refresh token available") {
		return "Your authentication session has expired. Please sign in again with Google", http.StatusUnauthorized
	}
	if contains(errMsg, "failed to refresh OAuth token") {
		return "Failed to refresh authentication. Please sign in again with Google", http.StatusUnauthorized
	}
	if contains(errMsg, "failed to get OAuth tokens") {
		return "Authentication tokens not found. Please sign in with Google", http.StatusUnauthorized
	}

	// Account related errors (403)
	if contains(errMsg, "accountClosed") {
		return "Cannot retrieve subscriptions because the subscriber's account is closed", http.StatusForbidden
	}
	if contains(errMsg, "accountSuspended") {
		return "Cannot retrieve subscriptions because the subscriber's account is suspended", http.StatusForbidden
	}
	if contains(errMsg, "subscriptionForbidden") {
		return "The requester is not allowed to access the requested subscriptions", http.StatusForbidden
	}
	if contains(errMsg, "ACCESS_TOKEN_SCOPE_INSUFFICIENT") {
		return "Insufficient permissions. Please re-authenticate with proper YouTube access.", http.StatusForbidden
	}

	// Authentication/authorization errors (401)
	if contains(errMsg, "invalid_grant") || contains(errMsg, "invalid_token") {
		return "Your authentication has expired. Please sign in again with Google", http.StatusUnauthorized
	}
	if contains(errMsg, "invalid credentials") || contains(errMsg, "Invalid Credentials") {
		return "Invalid authentication credentials. Please sign in again", http.StatusUnauthorized
	}

	// Not found errors (404)
	if contains(errMsg, "subscriberNotFound") {
		return "The subscriber specified in the request was not found", http.StatusNotFound
	}

	// API key related errors (should be rare now with OAuth)
	if contains(errMsg, "API key not valid") || contains(errMsg, "Invalid API key") {
		return "Invalid YouTube API key configuration", http.StatusInternalServerError
	}
	if contains(errMsg, "Daily Limit Exceeded") || contains(errMsg, "Quota exceeded") {
		return "YouTube API quota exceeded. Please try again later.", http.StatusTooManyRequests
	}
	if contains(errMsg, "API key blocked") {
		return "YouTube API key is blocked", http.StatusInternalServerError
	}

	// Service creation errors
	if contains(errMsg, "service_creation_failed") {
		return "Failed to connect to YouTube API. Please try again", http.StatusInternalServerError
	}
	if contains(errMsg, "api_call_failed") {
		return "YouTube API request failed. Please try again", http.StatusInternalServerError
	}

	// Default to internal server error
	return "YouTube API error: " + errMsg, http.StatusInternalServerError
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(substr) == 0 || str[:len(substr)] == substr || str[len(str)-len(substr):] == substr)
}

// createYouTubeService creates a YouTube API service using the provided OAuth token (legacy function)
func createYouTubeService(ctx context.Context, token *oauth2.Token) (*youtube.Service, error) {
	log.Printf("🔗 [YOUTUBE-SERVICE] Creating YouTube service with token...")
	log.Printf("🔗 [YOUTUBE-SERVICE] Token has access token: %t", token.AccessToken != "")
	log.Printf("🔗 [YOUTUBE-SERVICE] Token expiry: %v", token.Expiry)
	log.Printf("🔗 [YOUTUBE-SERVICE] Token type: %s", token.TokenType)
	
	// In a real application, you would use the OAuth2 config from your Google auth package
	// For simplicity, we're just using the token directly
	service, err := youtube.NewService(ctx, option.WithTokenSource(oauth2.StaticTokenSource(token)))
	if err != nil {
		log.Printf("❌ [YOUTUBE-SERVICE] Failed to create service: %v", err)
		return nil, err
	}
	
	log.Printf("✅ [YOUTUBE-SERVICE] YouTube service created successfully")
	return service, nil
}

// createYouTubeServiceWithAPIKey creates a YouTube API service using API key authentication
func createYouTubeServiceWithAPIKey(ctx context.Context, apiKey string) (*youtube.Service, error) {
	log.Printf("🔗 [YOUTUBE-SERVICE-API] Creating YouTube service with API key...")
	log.Printf("🔗 [YOUTUBE-SERVICE-API] API key length: %d", len(apiKey))
	
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	
	// Create YouTube service with API key
	service, err := youtube.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Printf("❌ [YOUTUBE-SERVICE-API] Failed to create service: %v", err)
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}
	
	log.Printf("✅ [YOUTUBE-SERVICE-API] YouTube service created successfully with API key")
	return service, nil
}

// checkSubscription checks if the authenticated user is subscribed to the specified channel (legacy function)
func checkSubscription(service *youtube.Service, channelID string) (bool, error) {
	log.Printf("🔍 [CHECK-SUBSCRIPTION] Starting subscription check for channel: %s", channelID)
	
	// Call the YouTube API to check subscriptions using the subscriptions.list API
	// Reference: https://developers.google.com/youtube/v3/docs/subscriptions/list

	log.Printf("🔍 [CHECK-SUBSCRIPTION] Building YouTube API call...")
	call := service.Subscriptions.List([]string{"snippet"})
	call = call.Mine(true)
	call = call.ForChannelId(channelID)

	log.Printf("🔍 [CHECK-SUBSCRIPTION] Executing YouTube API call...")
	response, err := call.Do()
	if err != nil {
		log.Printf("❌ [CHECK-SUBSCRIPTION] YouTube API call failed: %v", err)
		log.Printf("❌ [CHECK-SUBSCRIPTION] Error type: %T", err)
		log.Printf("❌ [CHECK-SUBSCRIPTION] Error details: %+v", err)
		return false, err
	}

	log.Printf("✅ [CHECK-SUBSCRIPTION] YouTube API call successful")
	log.Printf("📊 [CHECK-SUBSCRIPTION] Response items count: %d", len(response.Items))
	
	if len(response.Items) > 0 {
		log.Printf("🎯 [CHECK-SUBSCRIPTION] User is subscribed to channel %s", channelID)
		for i, item := range response.Items {
			if item.Snippet != nil {
				log.Printf("📋 [CHECK-SUBSCRIPTION] Subscription %d: Channel Title: %s", i+1, item.Snippet.Title)
			}
		}
	} else {
		log.Printf("❌ [CHECK-SUBSCRIPTION] User is NOT subscribed to channel %s", channelID)
	}

	// If we get any items back, it means the user is subscribed to the channel
	isSubscribed := len(response.Items) > 0
	log.Printf("🏁 [CHECK-SUBSCRIPTION] Final result: subscribed = %t", isSubscribed)
	return isSubscribed, nil
}

// Note: checkSubscriptionWithAPIKey function removed as we now use OAuth-based subscription checking
// This eliminates the previous limitation where API keys couldn't verify user subscriptions

// Note: Legacy token-based helper functions removed as we now use database-stored OAuth tokens
// The new implementation extracts user ID from JWT and retrieves stored OAuth tokens from database

// getUserInfo gets basic user information from the YouTube API
func getUserInfo(service *youtube.Service) (map[string]interface{}, error) {
	// Call the YouTube API to get channel information for "mine"
	// Reference: https://developers.google.com/youtube/v3/docs/channels/list

	call := service.Channels.List([]string{"snippet"})
	call = call.Mine(true)

	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	if len(response.Items) == 0 {
		return nil, errors.New("no channel found for authenticated user")
	}

	// Get the first (and typically only) channel for the authenticated user
	channel := response.Items[0]

	return map[string]interface{}{
		"id":    channel.Id,
		"name":  channel.Snippet.Title,
		"email": "", // Email is not available through the channels API
	}, nil
}

// extractUserIDFromJWT extracts the user ID from a JWT token
func extractUserIDFromJWT(tokenString string) (string, error) {
	log.Printf("🔑 [JWT-EXTRACT] Extracting user ID from JWT token")
	
	// Get JWT secret from environment
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", fmt.Errorf("JWT_SECRET not configured")
	}
	
	// Parse and validate JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
	
	if err != nil {
		log.Printf("❌ [JWT-EXTRACT] JWT verification failed: %v", err)
		return "", fmt.Errorf("invalid JWT token: %v", err)
	}
	
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID := fmt.Sprintf("%v", claims["user_id"])
		if userID == "" {
			return "", fmt.Errorf("invalid token: missing user_id claim")
		}
		
		log.Printf("✅ [JWT-EXTRACT] User ID extracted successfully: %s", userID)
		return userID, nil
	}
	
	return "", fmt.Errorf("invalid token claims")
}

// checkUserSubscriptionWithOAuth checks if a user is subscribed using OAuth tokens from database
func checkUserSubscriptionWithOAuth(ctx context.Context, userID, targetChannelID string) (bool, string, error) {
	log.Printf("🔍 [OAUTH-SUBSCRIPTION] Starting OAuth-based subscription check for user %s", userID)
	
	// Initialize services
	userRepo := repository.NewUserRepository()
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-development-secret-change-in-production"
	}
	userService := services.NewUserService(userRepo, jwtSecret)
	
	// Check if OAuth token is expired and refresh if needed
	isExpired, err := userService.IsOAuthTokenExpired(ctx, userID)
	if err != nil {
		log.Printf("❌ [OAUTH-SUBSCRIPTION] Failed to check token expiry: %v", err)
		return false, "error", fmt.Errorf("failed to check token expiry: %w", err)
	}
	
	var tokenData *models.OAuthTokenData
	if isExpired {
		log.Printf("🔄 [OAUTH-SUBSCRIPTION] OAuth token expired, attempting refresh")
		tokenData, err = userService.RefreshUserOAuthToken(ctx, userID)
		if err != nil {
			log.Printf("❌ [OAUTH-SUBSCRIPTION] Failed to refresh OAuth token: %v", err)
			return false, "refresh_failed", fmt.Errorf("failed to refresh OAuth token: %w", err)
		}
		log.Printf("✅ [OAUTH-SUBSCRIPTION] OAuth token refreshed successfully")
	} else {
		// Get current tokens
		tokenData, err = userService.GetUserOAuthTokens(ctx, userID)
		if err != nil {
			log.Printf("❌ [OAUTH-SUBSCRIPTION] Failed to get OAuth tokens: %v", err)
			return false, "no_tokens", fmt.Errorf("failed to get OAuth tokens: %w", err)
		}
		log.Printf("✅ [OAUTH-SUBSCRIPTION] OAuth tokens retrieved successfully")
	}
	
	// Create OAuth2 token for YouTube API
	token := &oauth2.Token{
		AccessToken:  tokenData.AccessToken,
		RefreshToken: tokenData.RefreshToken,
		Expiry:       tokenData.Expiry,
		TokenType:    tokenData.TokenType,
	}
	
	// Create YouTube service with OAuth token
	youtubeService, err := createYouTubeService(ctx, token)
	if err != nil {
		log.Printf("❌ [OAUTH-SUBSCRIPTION] Failed to create YouTube service: %v", err)
		return false, "service_creation_failed", fmt.Errorf("failed to create YouTube service: %w", err)
	}
	
	log.Printf("✅ [OAUTH-SUBSCRIPTION] YouTube service created successfully")
	
	// Check subscription using the authenticated YouTube API
	isSubscribed, err := checkSubscription(youtubeService, targetChannelID)
	if err != nil {
		log.Printf("❌ [OAUTH-SUBSCRIPTION] Subscription check failed: %v", err)
		return false, "api_call_failed", fmt.Errorf("subscription check failed: %w", err)
	}
	
	log.Printf("✅ [OAUTH-SUBSCRIPTION] Subscription check completed - subscribed: %t", isSubscribed)
	
	// Update user's subscription status in database if verified
	if isSubscribed {
		err = userRepo.UpdateYouTubeSubscription(ctx, userID, true)
		if err != nil {
			log.Printf("⚠️ [OAUTH-SUBSCRIPTION] Failed to update subscription status in DB: %v", err)
			// Don't fail the request, just log the warning
		}
	}
	
	return isSubscribed, "oauth", nil
}

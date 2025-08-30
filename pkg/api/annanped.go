package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gamemini/youtube/pkg/config"
	"github.com/gamemini/youtube/pkg/container"
	"google.golang.org/api/youtube/v3"
)

// AnanpedChannelID is the specific channel ID for Ananped
const AnanpedChannelID = "UC-chqi3Gpb4F7yBqedlnq5g"

// AnanpedResponse represents the response for Ananped-specific endpoints
type AnanpedResponse struct {
	Success            bool                   `json:"success"`
	Message            string                 `json:"message,omitempty"`
	IsSubscribed       bool                   `json:"is_subscribed"`
	UserInfo           map[string]interface{} `json:"user_info,omitempty"`
	ChannelInfo        map[string]interface{} `json:"channel_info,omitempty"`
	CelebrationMessage string                 `json:"celebration_message,omitempty"`
}

// HandleAnanpedSubscriptionCheck verifies if a user is subscribed to Ananped channel
func HandleAnanpedSubscriptionCheck(w http.ResponseWriter, r *http.Request) {
	log.Printf("🎉 [ANNANPED-CHECK] Starting Ananped subscription check from %s", r.RemoteAddr)

	// Extract token from request for user identification
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Printf("❌ [ANNANPED-CHECK] No authorization header found")
		sendAnanpedErrorResponse(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	// Extract the token value (remove "Bearer " prefix if it exists)
	tokenValue := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenValue = authHeader[7:]
	}

	// Get YouTube API key from environment
	apiKey := config.GetConfig().YouTubeAPIKey
	if apiKey == "" {
		log.Printf("❌ [ANNANPED-CHECK] YOUTUBE_API_KEY environment variable not set")
		sendAnanpedErrorResponse(w, "YouTube API not configured", http.StatusInternalServerError)
		return
	}

	// Create YouTube service with API key
	youtubeService, err := createYouTubeServiceWithAPIKey(r.Context(), apiKey)
	if err != nil {
		log.Printf("❌ [ANNANPED-CHECK] Failed to create YouTube service: %v", err)
		sendAnanpedErrorResponse(w, "Failed to create YouTube service", http.StatusInternalServerError)
		return
	}

	// Get user ID from JWT token
	userID, err := extractUserIDFromJWT(tokenValue)
	if err != nil {
		log.Printf("❌ [ANNANPED-CHECK] Failed to extract user ID from JWT: %v", err)
		sendAnanpedErrorResponse(w, "Invalid authorization token", http.StatusUnauthorized)
		return
	}

	log.Printf("🎯 [ANNANPED-CHECK] Checking if user %s is subscribed to Ananped channel", userID)

	// Check if the user is subscribed to Ananped channel using OAuth tokens
	isSubscribed, verificationMethod, err := checkUserSubscriptionWithOAuth(r.Context(), userID)
	if err != nil {
		log.Printf("❌ [ANNANPED-CHECK] Subscription check failed: %v", err)
		// For Ananped celebration, we'll be permissive and assume not subscribed
		isSubscribed = false
	}
	log.Printf("🎉 [ANNANPED-CHECK] Subscription result for Ananped: %t (method: %s)", isSubscribed, verificationMethod)

	// Get user info from JWT token claims
	userInfo := map[string]interface{}{
		"user_id": userID,
	}

	// Get channel info for Ananped using API key
	channelInfo, err := getChannelInfoWithAPIKey(youtubeService, AnanpedChannelID)
	if err != nil {
		log.Printf("⚠️ [ANNANPED-CHECK] Failed to get Ananped channel info: %v", err)
		// Continue even if we can't get channel info
		channelInfo = map[string]interface{}{
			"error": "Could not retrieve channel info",
			"id":    AnanpedChannelID,
			"title": "Ananped",
		}
	}

	// Create celebration message
	var celebrationMessage string
	if isSubscribed {
		celebrationMessage = "🎉 Thank you for being part of the Ananped family! You're helping us celebrate 10M subscribers! 🎉"
	} else {
		celebrationMessage = "Join the Ananped family and help us celebrate 10M subscribers! Subscribe now! 📺"
	}

	log.Printf("✅ [ANNANPED-CHECK] Ananped subscription check completed successfully")

	// Send response
	response := AnanpedResponse{
		Success:            true,
		Message:            "Ananped subscription check completed",
		IsSubscribed:       isSubscribed,
		UserInfo:           userInfo,
		ChannelInfo:        channelInfo,
		CelebrationMessage: celebrationMessage,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleAnanpedSubscriptionCheckWithContainer verifies if a user is subscribed to Ananped channel using container
func HandleAnanpedSubscriptionCheckWithContainer(w http.ResponseWriter, r *http.Request, appContainer *container.AppContainer) {
	log.Printf("🎉 [ANNANPED-CHECK-CONTAINER] Starting Ananped subscription check from %s", r.RemoteAddr)

	// Extract token from request for user identification
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Printf("❌ [ANNANPED-CHECK-CONTAINER] No authorization header found")
		sendAnanpedErrorResponse(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	// Extract the token value (remove "Bearer " prefix if it exists)
	tokenValue := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenValue = authHeader[7:]
	}

	// Get user ID from JWT token
	userID, err := extractUserIDFromJWT(tokenValue)
	if err != nil {
		log.Printf("❌ [ANNANPED-CHECK-CONTAINER] Failed to extract user ID from JWT: %v", err)
		sendAnanpedErrorResponse(w, "Invalid authorization token", http.StatusUnauthorized)
		return
	}

	log.Printf("🎯 [ANNANPED-CHECK-CONTAINER] Checking if user %s is subscribed to Ananped channel", userID)

	// Check if the user is subscribed to Ananped channel using YouTubeService
	youtubeService := appContainer.GetYouTubeService()
	isSubscribed, verificationMethod, err := youtubeService.CheckUserSubscriptionWithOAuth(r.Context(), userID)
	if err != nil {
		log.Printf("❌ [ANNANPED-CHECK-CONTAINER] Subscription check failed: %v", err)
		// For Ananped celebration, we'll be permissive and assume not subscribed
		isSubscribed = false
		verificationMethod = "error_fallback"
	}
	log.Printf("🎉 [ANNANPED-CHECK-CONTAINER] Subscription result for Ananped: %t (method: %s)", isSubscribed, verificationMethod)

	// Get user info from JWT token claims
	userInfo := map[string]interface{}{
		"user_id": userID,
	}

	// Get YouTube API key from environment for channel info
	apiKey := config.GetConfig().YouTubeAPIKey
	var channelInfo map[string]interface{}
	if apiKey != "" {
		// Get channel info for Ananped using API key
		youtubeAPIService, err := createYouTubeServiceWithAPIKey(r.Context(), apiKey)
		if err != nil {
			log.Printf("⚠️ [ANNANPED-CHECK-CONTAINER] Failed to create YouTube API service: %v", err)
			channelInfo = map[string]interface{}{
				"error": "Could not retrieve channel info",
				"id":    AnanpedChannelID,
				"title": "Ananped",
			}
		} else {
			channelInfo, err = getChannelInfoWithAPIKey(youtubeAPIService, AnanpedChannelID)
			if err != nil {
				log.Printf("⚠️ [ANNANPED-CHECK-CONTAINER] Failed to get Ananped channel info: %v", err)
				channelInfo = map[string]interface{}{
					"error": "Could not retrieve channel info",
					"id":    AnanpedChannelID,
					"title": "Ananped",
				}
			}
		}
	} else {
		channelInfo = map[string]interface{}{
			"error": "YouTube API key not configured",
			"id":    AnanpedChannelID,
			"title": "Ananped",
		}
	}

	// Create celebration message
	var celebrationMessage string
	if isSubscribed {
		celebrationMessage = "🎉 Thank you for being part of the Ananped family! You're helping us celebrate 10M subscribers! 🎉"
	} else {
		celebrationMessage = "Join the Ananped family and help us celebrate 10M subscribers! Subscribe now! 📺"
	}

	log.Printf("✅ [ANNANPED-CHECK-CONTAINER] Ananped subscription check completed successfully")

	// Send response
	response := AnanpedResponse{
		Success:            true,
		Message:            "Ananped subscription check completed",
		IsSubscribed:       isSubscribed,
		UserInfo:           userInfo,
		ChannelInfo:        channelInfo,
		CelebrationMessage: celebrationMessage,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getChannelInfo retrieves channel information (legacy OAuth version)
func getChannelInfo(service *youtube.Service, channelID string) (map[string]interface{}, error) {
	call := service.Channels.List([]string{"snippet", "statistics"}).Id(channelID)
	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	if len(response.Items) == 0 {
		return map[string]interface{}{
			"error": "Channel not found",
		}, nil
	}

	channel := response.Items[0]
	return map[string]interface{}{
		"id":               channel.Id,
		"title":            channel.Snippet.Title,
		"description":      channel.Snippet.Description,
		"thumbnail":        channel.Snippet.Thumbnails.High.Url,
		"subscriber_count": channel.Statistics.SubscriberCount,
		"video_count":      channel.Statistics.VideoCount,
		"view_count":       channel.Statistics.ViewCount,
	}, nil
}

// getChannelInfoWithAPIKey retrieves channel information using API key authentication
func getChannelInfoWithAPIKey(service *youtube.Service, channelID string) (map[string]interface{}, error) {
	log.Printf("📊 [CHANNEL-INFO-API] Fetching channel info for: %s", channelID)

	call := service.Channels.List([]string{"snippet", "statistics"}).Id(channelID)
	response, err := call.Do()
	if err != nil {
		log.Printf("❌ [CHANNEL-INFO-API] Failed to fetch channel info: %v", err)
		return nil, err
	}

	if len(response.Items) == 0 {
		log.Printf("❌ [CHANNEL-INFO-API] Channel not found: %s", channelID)
		return map[string]interface{}{
			"error": "Channel not found",
			"id":    channelID,
		}, nil
	}

	channel := response.Items[0]
	log.Printf("✅ [CHANNEL-INFO-API] Channel info retrieved: %s", channel.Snippet.Title)

	channelData := map[string]interface{}{
		"id":          channel.Id,
		"title":       channel.Snippet.Title,
		"description": channel.Snippet.Description,
	}

	// Add thumbnail if available
	if channel.Snippet.Thumbnails != nil && channel.Snippet.Thumbnails.High != nil {
		channelData["thumbnail"] = channel.Snippet.Thumbnails.High.Url
	}

	// Add statistics if available
	if channel.Statistics != nil {
		channelData["subscriber_count"] = channel.Statistics.SubscriberCount
		channelData["video_count"] = channel.Statistics.VideoCount
		channelData["view_count"] = channel.Statistics.ViewCount
	}

	return channelData, nil
}

// checkAnanpedSubscription implements a special check for Ananped subscription
// Since API key authentication can't verify subscriptions directly, this is a demo implementation
func checkAnanpedSubscription(userChannelID, token string) bool {
	log.Printf("🎉 [ANNANPED-SPECIAL] Performing special Ananped subscription check")

	// For the Ananped 10M celebration, we'll implement a special logic:
	// 1. If user has a valid token, consider them "subscribed" for celebration purposes
	// 2. This is a demo/celebration feature, not actual subscription verification

	if len(token) > 10 {
		log.Printf("🎉 [ANNANPED-SPECIAL] Valid token detected - treating as subscribed for celebration")
		return true
	}

	log.Printf("🎉 [ANNANPED-SPECIAL] No valid token - encouraging subscription")
	return false
}

// getUserInfoFallback creates user info when OAuth is not available
func getUserInfoFallback(token string) map[string]interface{} {
	log.Printf("👤 [USER-INFO-FALLBACK] Creating fallback user info")

	// Create deterministic user info based on token
	userID := "user_" + generateHash(token, 8)

	return map[string]interface{}{
		"id":    userID,
		"name":  "Ananped Fan",
		"email": userID + "@annanpedfan.com",
		"note":  "User info generated for Ananped celebration (API key mode)",
	}
}

// generateHash creates a simple hash for demo purposes
func generateHash(input string, length int) string {
	if len(input) < length {
		return input
	}
	return input[:length]
}

// sendAnanpedErrorResponse sends a standardized error response for Ananped endpoints
func sendAnanpedErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := AnanpedResponse{
		Success: false,
		Message: message,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

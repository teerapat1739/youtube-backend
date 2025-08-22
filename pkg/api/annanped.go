package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"google.golang.org/api/youtube/v3"
)

// AnnanpedChannelID is the specific channel ID for Annanped
const AnnanpedChannelID = "UC-chqi3Gpb4F7yBqedlnq5g"

// AnnanpedResponse represents the response for Annanped-specific endpoints
type AnnanpedResponse struct {
	Success            bool                   `json:"success"`
	Message            string                 `json:"message,omitempty"`
	IsSubscribed       bool                   `json:"is_subscribed"`
	UserInfo           map[string]interface{} `json:"user_info,omitempty"`
	ChannelInfo        map[string]interface{} `json:"channel_info,omitempty"`
	CelebrationMessage string                 `json:"celebration_message,omitempty"`
}

// HandleAnnanpedSubscriptionCheck verifies if a user is subscribed to Annanped channel
func HandleAnnanpedSubscriptionCheck(w http.ResponseWriter, r *http.Request) {
	log.Printf("🎉 [ANNANPED-CHECK] Starting Annanped subscription check from %s", r.RemoteAddr)
	
	// Extract token from request for user identification
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Printf("❌ [ANNANPED-CHECK] No authorization header found")
		sendAnnanpedErrorResponse(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	// Extract the token value (remove "Bearer " prefix if it exists)
	tokenValue := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenValue = authHeader[7:]
	}

	// Get YouTube API key from environment
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		log.Printf("❌ [ANNANPED-CHECK] YOUTUBE_API_KEY environment variable not set")
		sendAnnanpedErrorResponse(w, "YouTube API not configured", http.StatusInternalServerError)
		return
	}

	// Create YouTube service with API key
	youtubeService, err := createYouTubeServiceWithAPIKey(r.Context(), apiKey)
	if err != nil {
		log.Printf("❌ [ANNANPED-CHECK] Failed to create YouTube service: %v", err)
		sendAnnanpedErrorResponse(w, "Failed to create YouTube service", http.StatusInternalServerError)
		return
	}

	// Get user ID from JWT token
	userID, err := extractUserIDFromJWT(tokenValue)
	if err != nil {
		log.Printf("❌ [ANNANPED-CHECK] Failed to extract user ID from JWT: %v", err)
		sendAnnanpedErrorResponse(w, "Invalid authorization token", http.StatusUnauthorized)
		return
	}

	log.Printf("🎯 [ANNANPED-CHECK] Checking if user %s is subscribed to Annanped channel: %s", userID, AnnanpedChannelID)

	// Check if the user is subscribed to Annanped channel using OAuth tokens
	isSubscribed, verificationMethod, err := checkUserSubscriptionWithOAuth(r.Context(), userID, AnnanpedChannelID)
	if err != nil {
		log.Printf("❌ [ANNANPED-CHECK] Subscription check failed: %v", err)
		// For Annanped celebration, we'll be permissive and assume not subscribed
		isSubscribed = false
	}
	log.Printf("🎉 [ANNANPED-CHECK] Subscription result for Annanped: %t (method: %s)", isSubscribed, verificationMethod)

	// Get user info from JWT token claims
	userInfo := map[string]interface{}{
		"user_id": userID,
	}

	// Get channel info for Annanped using API key
	channelInfo, err := getChannelInfoWithAPIKey(youtubeService, AnnanpedChannelID)
	if err != nil {
		log.Printf("⚠️ [ANNANPED-CHECK] Failed to get Annanped channel info: %v", err)
		// Continue even if we can't get channel info
		channelInfo = map[string]interface{}{
			"error": "Could not retrieve channel info",
			"id":    AnnanpedChannelID,
			"title": "Annanped",
		}
	}

	// Create celebration message
	var celebrationMessage string
	if isSubscribed {
		celebrationMessage = "🎉 Thank you for being part of the Annanped family! You're helping us celebrate 10M subscribers! 🎉"
	} else {
		celebrationMessage = "Join the Annanped family and help us celebrate 10M subscribers! Subscribe now! 📺"
	}

	log.Printf("✅ [ANNANPED-CHECK] Annanped subscription check completed successfully")

	// Send response
	response := AnnanpedResponse{
		Success:            true,
		Message:            "Annanped subscription check completed",
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

// checkAnnanpedSubscription implements a special check for Annanped subscription
// Since API key authentication can't verify subscriptions directly, this is a demo implementation
func checkAnnanpedSubscription(userChannelID, token string) bool {
	log.Printf("🎉 [ANNANPED-SPECIAL] Performing special Annanped subscription check")
	
	// For the Annanped 10M celebration, we'll implement a special logic:
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
		"name":  "Annanped Fan",
		"email": userID + "@annanpedfan.com",
		"note":  "User info generated for Annanped celebration (API key mode)",
	}
}

// generateHash creates a simple hash for demo purposes
func generateHash(input string, length int) string {
	if len(input) < length {
		return input
	}
	return input[:length]
}

// sendAnnanpedErrorResponse sends a standardized error response for Annanped endpoints
func sendAnnanpedErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := AnnanpedResponse{
		Success: false,
		Message: message,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

package api

import (
	"encoding/json"
	"net/http"

	"golang.org/x/oauth2"
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
	// Extract token from request
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		sendAnnanpedErrorResponse(w, "Authorization token required", http.StatusUnauthorized)
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
		sendAnnanpedErrorResponse(w, "Failed to create YouTube service", http.StatusInternalServerError)
		return
	}

	// Check if the user is subscribed to Annanped channel
	isSubscribed, err := checkSubscription(youtubeService, AnnanpedChannelID)
	if err != nil {
		errMsg, statusCode := handleYouTubeAPIError(err)
		sendAnnanpedErrorResponse(w, errMsg, statusCode)
		return
	}

	// Get user info
	userInfo, err := getUserInfo(youtubeService)
	if err != nil {
		// Continue even if we can't get user info, just log the error
		userInfo = map[string]interface{}{"error": "Could not retrieve user info"}
	}

	// Get channel info for Annanped
	channelInfo, err := getChannelInfo(youtubeService, AnnanpedChannelID)
	if err != nil {
		// Continue even if we can't get channel info
		channelInfo = map[string]interface{}{"error": "Could not retrieve channel info"}
	}

	// Create celebration message
	var celebrationMessage string
	if isSubscribed {
		celebrationMessage = "🎉 Thank you for being part of the Annanped family! You're helping us celebrate 10M subscribers! 🎉"
	} else {
		celebrationMessage = "Join the Annanped family and help us celebrate 10M subscribers! Subscribe now! 📺"
	}

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

// getChannelInfo retrieves channel information
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

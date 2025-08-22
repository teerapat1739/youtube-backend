package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gamemini/youtube/pkg/models"
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

	// Get channel ID from request query parameters
	channelID := r.URL.Query().Get("channel_id")
	if channelID == "" {
		sendErrorResponse(w, "Channel ID is required", http.StatusBadRequest)
		return
	}

	// Check if the user is subscribed to the specified channel
	isSubscribed, err := checkSubscription(youtubeService, channelID)
	if err != nil {
		errMsg, statusCode := handleYouTubeAPIError(err)
		sendErrorResponse(w, errMsg, statusCode)
		return
	}

	// Send response
	response := ActivityResponse{
		Success: isSubscribed,
		Message: "Subscription check completed",
		Data: map[string]bool{
			"is_subscribed": isSubscribed,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

	// Not found errors (404)
	if contains(errMsg, "subscriberNotFound") {
		return "The subscriber specified in the request was not found", http.StatusNotFound
	}

	// Default to internal server error
	return "YouTube API error: " + errMsg, http.StatusInternalServerError
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(substr) == 0 || str[:len(substr)] == substr || str[len(str)-len(substr):] == substr)
}

// createYouTubeService creates a YouTube API service using the provided OAuth token
func createYouTubeService(ctx context.Context, token *oauth2.Token) (*youtube.Service, error) {
	// In a real application, you would use the OAuth2 config from your Google auth package
	// For simplicity, we're just using the token directly
	return youtube.NewService(ctx, option.WithTokenSource(oauth2.StaticTokenSource(token)))
}

// checkSubscription checks if the authenticated user is subscribed to the specified channel
func checkSubscription(service *youtube.Service, channelID string) (bool, error) {
	// Call the YouTube API to check subscriptions using the subscriptions.list API
	// Reference: https://developers.google.com/youtube/v3/docs/subscriptions/list

	call := service.Subscriptions.List([]string{"snippet"})
	call = call.Mine(true)
	call = call.ForChannelId(channelID)

	response, err := call.Do()
	if err != nil {
		return false, err
	}

	// If we get any items back, it means the user is subscribed to the channel
	return len(response.Items) > 0, nil
}

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

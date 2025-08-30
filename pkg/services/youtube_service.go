package services

import (
	"context"
	"fmt"
	"log"

	"github.com/gamemini/youtube/pkg/config"
	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/repository"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// YouTubeService handles YouTube-related business logic
type YouTubeService struct {
	userRepo    *repository.UserRepository
	userService *UserService
	config      *config.Config
}

// NewYouTubeService creates a new YouTube service
func NewYouTubeService(userRepo *repository.UserRepository, userService *UserService, cfg *config.Config) *YouTubeService {
	return &YouTubeService{
		userRepo:    userRepo,
		userService: userService,
		config:      cfg,
	}
}

// CheckUserSubscriptionWithOAuth checks if a user is subscribed using OAuth tokens from database
// This function handles:
// 1. Checking if OAuth token is expired and refreshing if needed
// 2. Creating YouTube service with OAuth token
// 3. Checking YouTube subscription status
// 4. Updating subscription status in database
func (s *YouTubeService) CheckUserSubscriptionWithOAuth(ctx context.Context, userID string) (bool, string, error) {
	log.Printf("🔍 [OAUTH-SUBSCRIPTION] Starting OAuth-based subscription check for user %s", userID)

	// Check if OAuth token is expired and refresh if needed
	isExpired, err := s.userService.IsOAuthTokenExpired(ctx, userID)
	if err != nil {
		log.Printf("❌ [OAUTH-SUBSCRIPTION] Failed to check token expiry: %v", err)
		return false, "error", fmt.Errorf("failed to check token expiry: %w", err)
	}

	var tokenData *models.OAuthTokenData
	if isExpired {
		log.Printf("🔄 [OAUTH-SUBSCRIPTION] OAuth token expired, attempting refresh")
		tokenData, err = s.userService.RefreshUserOAuthToken(ctx, userID)
		if err != nil {
			log.Printf("❌ [OAUTH-SUBSCRIPTION] Failed to refresh OAuth token: %v", err)
			return false, "refresh_failed", fmt.Errorf("failed to refresh OAuth token: %w", err)
		}
		log.Printf("✅ [OAUTH-SUBSCRIPTION] OAuth token refreshed successfully")
	} else {
		// Get current tokens
		tokenData, err = s.userService.GetUserOAuthTokens(ctx, userID)
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
	youtubeService, err := s.createYouTubeService(ctx, token)
	if err != nil {
		log.Printf("❌ [OAUTH-SUBSCRIPTION] Failed to create YouTube service: %v", err)
		return false, "service_creation_failed", fmt.Errorf("failed to create YouTube service: %w", err)
	}

	log.Printf("✅ [OAUTH-SUBSCRIPTION] YouTube service created successfully")

	// Check subscription using the authenticated YouTube API
	isSubscribed, err := s.checkSubscription(youtubeService)
	if err != nil {
		log.Printf("❌ [OAUTH-SUBSCRIPTION] Subscription check failed: %v", err)
		return false, "api_call_failed", fmt.Errorf("subscription check failed: %w", err)
	}

	log.Printf("✅ [OAUTH-SUBSCRIPTION] Subscription check completed - subscribed: %t", isSubscribed)

	// Update user's subscription status in database
	err = s.userRepo.UpdateYouTubeSubscription(ctx, userID, isSubscribed)
	if err != nil {
		log.Printf("⚠️ [OAUTH-SUBSCRIPTION] Failed to update subscription status in DB: %v", err)
		// Don't fail the request, just log the warning
	}

	return isSubscribed, "oauth", nil
}

// createYouTubeService creates a YouTube API service using the provided OAuth token
func (s *YouTubeService) createYouTubeService(ctx context.Context, token *oauth2.Token) (*youtube.Service, error) {
	log.Printf("🔗 [YOUTUBE-SERVICE] Creating YouTube service with token...")
	log.Printf("🔗 [YOUTUBE-SERVICE] Token has access token: %t", token.AccessToken != "")
	log.Printf("🔗 [YOUTUBE-SERVICE] Token expiry: %v", token.Expiry)
	log.Printf("🔗 [YOUTUBE-SERVICE] Token type: %s", token.TokenType)

	service, err := youtube.NewService(ctx, option.WithTokenSource(oauth2.StaticTokenSource(token)))
	if err != nil {
		log.Printf("❌ [YOUTUBE-SERVICE] Failed to create service: %v", err)
		return nil, err
	}

	log.Printf("✅ [YOUTUBE-SERVICE] YouTube service created successfully")
	return service, nil
}

// checkSubscription checks if the authenticated user is subscribed to the specified channel
func (s *YouTubeService) checkSubscription(service *youtube.Service) (bool, error) {
	// Get channel ID from environment variable
	channelID := s.config.TargetChannelID
	if channelID == "" {
		log.Printf("❌ [CHECK-SUBSCRIPTION] TARGET_YOUTUBE_CHANNEL_ID not set in environment")
		return false, fmt.Errorf("target channel ID not configured")
	}

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

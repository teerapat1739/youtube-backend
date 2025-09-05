package youtube

import (
	"context"
	"fmt"
	"net/http"

	"be-v2/internal/domain"
	"be-v2/internal/service"
	"be-v2/pkg/errors"
	"be-v2/pkg/logger"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// Service implements the YouTubeService interface
type Service struct {
	apiKey     string
	httpClient *http.Client
	logger     *logger.Logger
}

// NewService creates a new YouTube service
func NewService(apiKey string, logger *logger.Logger) service.YouTubeService {
	return &Service{
		apiKey:     apiKey,
		httpClient: &http.Client{},
		logger:     logger,
	}
}

// CheckSubscription checks if a user is subscribed to a specific channel
func (s *Service) CheckSubscription(ctx context.Context, accessToken string, channelID string) (*domain.SubscriptionCheckResponse, error) {
	s.logger.WithFields(map[string]interface{}{
		"channel_id": channelID,
	}).Debug("Checking YouTube subscription")

	// Create YouTube service with user's access token
	token := &oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "Bearer",
	}

	oauth2Config := &oauth2.Config{}
	client := oauth2Config.Client(ctx, token)

	youtubeService, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		s.logger.WithError(err).Error("Failed to create YouTube service")
		return nil, errors.NewInternalError("Failed to initialize YouTube service", err)
	}

	// Get channel information first
	channelInfo, err := s.GetChannelInfo(ctx, channelID)
	if err != nil {
		return nil, err
	}

	// Check subscription
	subscriptionsCall := youtubeService.Subscriptions.List([]string{"id", "snippet"}).
		ForChannelId(channelID).
		Mine(true)

	subscriptionsResponse, err := subscriptionsCall.Do()
	if err != nil {
		s.logger.WithError(err).Error("Failed to check subscription")
		return nil, errors.NewExternalError("Failed to check YouTube subscription", err)
	}

	isSubscribed := len(subscriptionsResponse.Items) > 0

	response := &domain.SubscriptionCheckResponse{
		IsSubscribed: isSubscribed,
		Channel:      *channelInfo,
	}

	if isSubscribed {
		response.Message = fmt.Sprintf("User is subscribed to channel %s", channelInfo.Title)
		s.logger.WithField("channel_title", channelInfo.Title).Info("User is subscribed to channel")
	} else {
		response.Message = fmt.Sprintf("User is not subscribed to channel %s", channelInfo.Title)
		s.logger.WithField("channel_title", channelInfo.Title).Info("User is not subscribed to channel")
	}

	return response, nil
}

// GetChannelInfo gets basic information about a YouTube channel
func (s *Service) GetChannelInfo(ctx context.Context, channelID string) (*domain.YouTubeChannel, error) {
	s.logger.WithField("channel_id", channelID).Debug("Getting YouTube channel info")

	// Create YouTube service with API key
	youtubeService, err := youtube.NewService(ctx, option.WithAPIKey(s.apiKey))
	if err != nil {
		s.logger.WithError(err).Error("Failed to create YouTube service")
		return nil, errors.NewInternalError("Failed to initialize YouTube service", err)
	}

	// Get channel information
	channelsCall := youtubeService.Channels.List([]string{"id", "snippet"}).
		Id(channelID)

	channelsResponse, err := channelsCall.Do()
	if err != nil {
		s.logger.WithError(err).Error("Failed to get channel info")
		return nil, errors.NewExternalError("Failed to get YouTube channel information", err)
	}

	if len(channelsResponse.Items) == 0 {
		s.logger.WithField("channel_id", channelID).Error("Channel not found")
		return nil, errors.NewNotFoundError("YouTube channel not found")
	}

	channel := channelsResponse.Items[0]

	// Get thumbnail URL
	thumbnail := ""
	if channel.Snippet.Thumbnails != nil {
		if channel.Snippet.Thumbnails.Default != nil {
			thumbnail = channel.Snippet.Thumbnails.Default.Url
		} else if channel.Snippet.Thumbnails.Medium != nil {
			thumbnail = channel.Snippet.Thumbnails.Medium.Url
		} else if channel.Snippet.Thumbnails.High != nil {
			thumbnail = channel.Snippet.Thumbnails.High.Url
		}
	}

	channelInfo := &domain.YouTubeChannel{
		ID:          channel.Id,
		Title:       channel.Snippet.Title,
		Description: channel.Snippet.Description,
		Thumbnail:   thumbnail,
	}

	s.logger.WithFields(map[string]interface{}{
		"channel_id":    channelInfo.ID,
		"channel_title": channelInfo.Title,
	}).Debug("Retrieved YouTube channel info")

	return channelInfo, nil
}

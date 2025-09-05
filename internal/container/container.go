package container

import (
	"be-v2/internal/config"
	"be-v2/internal/service"
	"be-v2/internal/service/auth"
	"be-v2/internal/service/youtube"
	"be-v2/pkg/logger"
	"be-v2/pkg/redis"
)

// Container holds all application dependencies
type Container struct {
	Config      *config.Config
	Logger      *logger.Logger
	RedisClient *redis.Client
	Services    *service.Services
}

// New creates a new dependency injection container
func New(cfg *config.Config, logger *logger.Logger) (*Container, error) {
	// Initialize Redis client if Redis URL is configured
	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		client, err := redis.NewClient(cfg.RedisURL)
		if err != nil {
			logger.WithError(err).Warn("Failed to initialize Redis client, proceeding without caching")
		} else {
			redisClient = client
			logger.Info("Redis client initialized successfully")
		}
	} else {
		logger.Info("Redis URL not configured, proceeding without caching")
	}

	// Initialize services
	authService := auth.NewService(cfg.GoogleClientID, logger)
	youtubeService := youtube.NewService(cfg.YouTubeAPIKey, logger)

	services := &service.Services{
		Auth:    authService,
		YouTube: youtubeService,
	}

	return &Container{
		Config:      cfg,
		Logger:      logger,
		RedisClient: redisClient,
		Services:    services,
	}, nil
}

// GetAuthService returns the auth service
func (c *Container) GetAuthService() service.AuthService {
	return c.Services.Auth
}

// GetYouTubeService returns the YouTube service
func (c *Container) GetYouTubeService() service.YouTubeService {
	return c.Services.YouTube
}

// GetLogger returns the logger
func (c *Container) GetLogger() *logger.Logger {
	return c.Logger
}

// GetConfig returns the configuration
func (c *Container) GetConfig() *config.Config {
	return c.Config
}

// GetRedisClient returns the Redis client (may be nil if not configured)
func (c *Container) GetRedisClient() *redis.Client {
	return c.RedisClient
}

// HasRedis returns true if Redis client is available
func (c *Container) HasRedis() bool {
	return c.RedisClient != nil
}

// GetCacheService returns a cache service instance (returns nil if Redis is not available)
func (c *Container) GetCacheService() *service.CacheService {
	if c.RedisClient == nil {
		return nil
	}
	return service.NewCacheService(c.RedisClient, c.Logger.Logger)
}

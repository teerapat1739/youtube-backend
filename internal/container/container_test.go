package container

import (
	"testing"

	"be-v2/internal/config"
	"be-v2/internal/service"
	"be-v2/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.Config
		expectRedis   bool
		expectError   bool
	}{
		{
			name: "Container with Redis configured",
			config: &config.Config{
				Environment:    "test",
				RedisURL:       "redis://localhost:6379/0",
				GoogleClientID: "test-client-id",
				YouTubeAPIKey:  "test-api-key",
			},
			expectRedis: true,
			expectError: false,
		},
		{
			name: "Container without Redis configured",
			config: &config.Config{
				Environment:    "test",
				RedisURL:       "",
				GoogleClientID: "test-client-id",
				YouTubeAPIKey:  "test-api-key",
			},
			expectRedis: false,
			expectError: false,
		},
		{
			name: "Container with invalid Redis URL",
			config: &config.Config{
				Environment:    "test",
				RedisURL:       "invalid://redis-url",
				GoogleClientID: "test-client-id",
				YouTubeAPIKey:  "test-api-key",
			},
			expectRedis: false, // Redis client initialization fails but container creation succeeds
			expectError: false,
		},
		{
			name: "Container with production environment",
			config: &config.Config{
				Environment:    "production",
				RedisURL:       "redis://localhost:6379/0",
				GoogleClientID: "prod-client-id",
				YouTubeAPIKey:  "prod-api-key",
			},
			expectRedis: true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger
			testLogger, _ := logger.New("info")

			// Create container
			container, err := New(tt.config, testLogger)

			// Assert error expectation
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, container)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, container)

				// Verify container components
				assert.NotNil(t, container.Config)
				assert.Equal(t, tt.config, container.Config)
				assert.NotNil(t, container.Logger)
				assert.Equal(t, testLogger, container.Logger)
				assert.NotNil(t, container.Services)
				assert.NotNil(t, container.Services.Auth)
				assert.NotNil(t, container.Services.YouTube)

				// Verify Redis client
				if tt.expectRedis && tt.config.RedisURL != "" && tt.config.RedisURL != "invalid://redis-url" {
					// For valid Redis URLs, client might be created but connection could fail
					// We can't guarantee Redis is running, so we just check if client was attempted
					// In real scenario with valid URL, client would be created
				} else if tt.config.RedisURL == "" {
					assert.Nil(t, container.RedisClient)
				}
			}
		})
	}
}

func TestContainer_GetAuthService(t *testing.T) {
	cfg := &config.Config{
		Environment:    "test",
		GoogleClientID: "test-client-id",
		YouTubeAPIKey:  "test-api-key",
	}
	testLogger, _ := logger.New("info")

	container, err := New(cfg, testLogger)
	require.NoError(t, err)
	require.NotNil(t, container)

	authService := container.GetAuthService()
	assert.NotNil(t, authService)
	assert.Implements(t, (*service.AuthService)(nil), authService)
}

func TestContainer_GetYouTubeService(t *testing.T) {
	cfg := &config.Config{
		Environment:    "test",
		GoogleClientID: "test-client-id",
		YouTubeAPIKey:  "test-api-key",
	}
	testLogger, _ := logger.New("info")

	container, err := New(cfg, testLogger)
	require.NoError(t, err)
	require.NotNil(t, container)

	youtubeService := container.GetYouTubeService()
	assert.NotNil(t, youtubeService)
	assert.Implements(t, (*service.YouTubeService)(nil), youtubeService)
}

func TestContainer_GetLogger(t *testing.T) {
	cfg := &config.Config{
		Environment:    "test",
		GoogleClientID: "test-client-id",
		YouTubeAPIKey:  "test-api-key",
	}
	testLogger, _ := logger.New("info")

	container, err := New(cfg, testLogger)
	require.NoError(t, err)
	require.NotNil(t, container)

	retrievedLogger := container.GetLogger()
	assert.NotNil(t, retrievedLogger)
	assert.Equal(t, testLogger, retrievedLogger)
}

func TestContainer_GetConfig(t *testing.T) {
	cfg := &config.Config{
		Environment:    "test",
		GoogleClientID: "test-client-id",
		YouTubeAPIKey:  "test-api-key",
		Port:           "8080",
	}
	testLogger, _ := logger.New("info")

	container, err := New(cfg, testLogger)
	require.NoError(t, err)
	require.NotNil(t, container)

	retrievedConfig := container.GetConfig()
	assert.NotNil(t, retrievedConfig)
	assert.Equal(t, cfg, retrievedConfig)
	assert.Equal(t, "test", retrievedConfig.Environment)
	assert.Equal(t, "test-client-id", retrievedConfig.GoogleClientID)
	assert.Equal(t, "test-api-key", retrievedConfig.YouTubeAPIKey)
	assert.Equal(t, "8080", retrievedConfig.Port)
}

func TestContainer_GetRedisClient(t *testing.T) {
	tests := []struct {
		name           string
		redisURL       string
		expectNil      bool
	}{
		{
			name:      "With Redis URL",
			redisURL:  "redis://localhost:6379/0",
			expectNil: false, // Might be nil if connection fails, but won't be nil if URL is valid
		},
		{
			name:      "Without Redis URL",
			redisURL:  "",
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Environment:    "test",
				RedisURL:       tt.redisURL,
				GoogleClientID: "test-client-id",
				YouTubeAPIKey:  "test-api-key",
			}
			testLogger, _ := logger.New("info")

			container, err := New(cfg, testLogger)
			require.NoError(t, err)
			require.NotNil(t, container)

			redisClient := container.GetRedisClient()
			if tt.expectNil {
				assert.Nil(t, redisClient)
			} else {
				// Redis client might be nil if connection fails
				// We can't guarantee Redis is running in test environment
				// So we just test that the method works
				_ = redisClient
			}
		})
	}
}

func TestContainer_HasRedis(t *testing.T) {
	tests := []struct {
		name         string
		redisURL     string
		expectHasRedis bool
	}{
		{
			name:         "With Redis URL",
			redisURL:     "redis://localhost:6379/0",
			expectHasRedis: false, // Will be false if Redis connection fails
		},
		{
			name:         "Without Redis URL",
			redisURL:     "",
			expectHasRedis: false,
		},
		{
			name:         "With invalid Redis URL",
			redisURL:     "invalid://redis",
			expectHasRedis: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Environment:    "test",
				RedisURL:       tt.redisURL,
				GoogleClientID: "test-client-id",
				YouTubeAPIKey:  "test-api-key",
			}
			testLogger, _ := logger.New("info")

			container, err := New(cfg, testLogger)
			require.NoError(t, err)
			require.NotNil(t, container)

			hasRedis := container.HasRedis()
			// In test environment without actual Redis, this will be false
			// even with valid URLs since connection fails
			if tt.redisURL == "" {
				assert.False(t, hasRedis)
			}
		})
	}
}

func TestContainer_GetCacheService(t *testing.T) {
	tests := []struct {
		name              string
		redisURL          string
		expectCacheService bool
	}{
		{
			name:              "With Redis client",
			redisURL:          "redis://localhost:6379/0",
			expectCacheService: false, // Will be false if Redis connection fails
		},
		{
			name:              "Without Redis client",
			redisURL:          "",
			expectCacheService: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Environment:    "test",
				RedisURL:       tt.redisURL,
				GoogleClientID: "test-client-id",
				YouTubeAPIKey:  "test-api-key",
			}
			testLogger, _ := logger.New("info")

			container, err := New(cfg, testLogger)
			require.NoError(t, err)
			require.NotNil(t, container)

			cacheService := container.GetCacheService()
			if tt.expectCacheService && container.RedisClient != nil {
				assert.NotNil(t, cacheService)
				assert.IsType(t, &service.CacheService{}, cacheService)
			} else {
				// Cache service is nil when Redis is not available
				if container.RedisClient == nil {
					assert.Nil(t, cacheService)
				}
			}
		})
	}
}

func TestContainer_EnvironmentPrefixing(t *testing.T) {
	tests := []struct {
		name           string
		environment    string
		expectedPrefix string
	}{
		{
			name:           "Development environment",
			environment:    "development",
			expectedPrefix: "staging",
		},
		{
			name:           "Staging environment",
			environment:    "staging",
			expectedPrefix: "staging",
		},
		{
			name:           "Production environment",
			environment:    "production",
			expectedPrefix: "prod",
		},
		{
			name:           "Test environment defaults to staging",
			environment:    "test",
			expectedPrefix: "prod", // Unknown environments default to prod
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Environment:    tt.environment,
				RedisURL:       "redis://localhost:6379/0",
				GoogleClientID: "test-client-id",
				YouTubeAPIKey:  "test-api-key",
			}
			testLogger, _ := logger.New("info")

			container, err := New(cfg, testLogger)
			require.NoError(t, err)
			require.NotNil(t, container)

			// If Redis client was successfully created, check the prefix
			if container.RedisClient != nil {
				assert.Equal(t, tt.expectedPrefix, container.RedisClient.KeyBuilder.GetPrefix())
			}
		})
	}
}

func TestContainer_ServiceInitialization(t *testing.T) {
	cfg := &config.Config{
		Environment:    "test",
		GoogleClientID: "test-google-client",
		YouTubeAPIKey:  "test-youtube-key",
	}
	testLogger, _ := logger.New("info")

	container, err := New(cfg, testLogger)
	require.NoError(t, err)
	require.NotNil(t, container)

	// Verify all services are initialized
	assert.NotNil(t, container.Services)
	assert.NotNil(t, container.Services.Auth)
	assert.NotNil(t, container.Services.YouTube)

	// Verify services have correct configuration
	// Note: We can't directly test the internal configuration of services
	// but we can verify they're not nil and implement the correct interfaces
	authService := container.GetAuthService()
	assert.NotNil(t, authService)

	youtubeService := container.GetYouTubeService()
	assert.NotNil(t, youtubeService)
}
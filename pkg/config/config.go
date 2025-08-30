package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// Environment represents the application environment
type Environment string

const (
	EnvLocal       Environment = "local"
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
)

// Config holds all application configuration with generic naming
type Config struct {
	// Server configuration
	Port           string
	AllowedOrigins []string
	FrontendURL    string
	Environment    Environment

	// Authentication configuration
	OAuthConfig OAuthConfig
	JWTSecret   string

	// API configuration
	YouTubeAPIKey      string
	TargetChannelID    string
	YouTubeAPIBaseURL  string

	// Database configuration
	DatabaseURL string

	// Cache configuration
	RedisURL string

	// Logging configuration
	LogLevel string
	Debug    bool
}

// OAuthConfig holds OAuth configuration (generic naming)
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// LoadConfig loads and validates all application configuration
// It automatically detects the environment and loads configuration accordingly
func LoadConfig() (*Config, error) {
	log.Println("🔧 Loading application configuration...")

	// Detect environment first
	env := detectEnvironment()
	log.Printf("🌍 Detected environment: %s", env)

	// Load environment variables based on environment
	if err := loadEnvironmentVariables(env); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Build configuration
	config, err := buildConfig(env)
	if err != nil {
		return nil, fmt.Errorf("failed to build configuration: %w", err)
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	log.Printf("✅ Configuration loaded successfully for %s environment", env)
	return config, nil
}

// detectEnvironment automatically detects the environment
func detectEnvironment() Environment {
	// Priority 1: Explicit ENV setting
	if env := os.Getenv("ENV"); env != "" {
		switch strings.ToLower(env) {
		case "production", "prod":
			return EnvProduction
		case "development", "dev":
			return EnvDevelopment
		case "local":
			return EnvLocal
		}
	}

	// Priority 2: NODE_ENV for compatibility
	if env := os.Getenv("NODE_ENV"); env != "" {
		switch strings.ToLower(env) {
		case "production", "prod":
			return EnvProduction
		case "development", "dev":
			return EnvDevelopment
		case "local":
			return EnvLocal
		}
	}

	// Priority 3: GCP Cloud Run indicators
	if isGCPCloudRun() {
		log.Println("🌐 GCP Cloud Run environment detected")
		return EnvProduction
	}

	// Default to local for safety
	log.Println("🏠 Defaulting to local environment")
	return EnvLocal
}

// isGCPCloudRun detects if running in GCP Cloud Run
func isGCPCloudRun() bool {
	// GCP Cloud Run sets these environment variables automatically
	gcpIndicators := []string{
		"K_SERVICE",        // Cloud Run service name
		"K_REVISION",       // Cloud Run revision
		"K_CONFIGURATION",  // Cloud Run configuration
	}

	for _, indicator := range gcpIndicators {
		if os.Getenv(indicator) != "" {
			return true
		}
	}

	// Additional check: PORT is always set by Cloud Run
	if port := os.Getenv("PORT"); port != "" {
		// Also check for absence of typical local development indicators
		if os.Getenv("HOME") == "" || strings.Contains(os.Getenv("PWD"), "/app") {
			return true
		}
	}

	return false
}

// loadEnvironmentVariables loads environment variables based on the environment
func loadEnvironmentVariables(env Environment) error {
	switch env {
	case EnvLocal, EnvDevelopment:
		return loadFromEnvFiles()
	case EnvProduction:
		log.Println("📊 Production environment - using system environment variables")
		return nil // Production uses environment variables set by GCP Cloud Run UI
	default:
		return loadFromEnvFiles() // Default to local behavior
	}
}

// loadFromEnvFiles loads environment variables from .env files
func loadFromEnvFiles() error {
	// Try to load from .env.local first (highest priority)
	if err := loadEnvFile(".env.local"); err == nil {
		log.Println("📁 Loaded .env.local file")
		return nil
	}

	// Fallback to .env file
	if err := loadEnvFile(".env"); err == nil {
		log.Println("📁 Loaded .env file")
		return nil
	}

	log.Println("ℹ️  No .env files found, using system environment variables")
	return nil
}

// loadEnvFile loads environment variables from a specific file
func loadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Parse key=value pairs
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				
				// Remove quotes if present
				if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				   (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
					value = value[1 : len(value)-1]
				}
				
				// Only set if not already set (allows override via system env)
				if os.Getenv(key) == "" {
					os.Setenv(key, value)
				}
			}
		}
	}
	
	return scanner.Err()
}

// buildConfig builds the configuration struct from environment variables
func buildConfig(env Environment) (*Config, error) {
	config := &Config{
		Environment: env,
	}

	// Server configuration
	config.Port = getEnvWithDefault("PORT", "8080")
	config.FrontendURL = os.Getenv("FRONTEND_URL")

	// Parse allowed origins
	allowedOrigins, err := parseAllowedOrigins()
	if err != nil {
		return nil, fmt.Errorf("failed to parse allowed origins: %w", err)
	}
	config.AllowedOrigins = allowedOrigins

	// Authentication configuration
	config.OAuthConfig = OAuthConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("REDIRECT_URL"),
	}
	config.JWTSecret = os.Getenv("JWT_SECRET")

	// API configuration
	config.YouTubeAPIKey = os.Getenv("YOUTUBE_API_KEY")
	config.TargetChannelID = os.Getenv("TARGET_YOUTUBE_CHANNEL_ID")
	config.YouTubeAPIBaseURL = getEnvWithDefault("YOUTUBE_API_BASE_URL", "https://www.googleapis.com/youtube/v3")

	// Database configuration
	config.DatabaseURL = os.Getenv("DATABASE_URL")

	// Cache configuration
	config.RedisURL = os.Getenv("REDIS_URL")

	// Logging configuration
	config.LogLevel = getEnvWithDefault("LOG_LEVEL", "info")
	config.Debug = getBoolEnv("APP_DEBUG", false)

	return config, nil
}

// parseAllowedOrigins parses ALLOWED_ORIGINS environment variable
func parseAllowedOrigins() ([]string, error) {
	envOrigins := os.Getenv("ALLOWED_ORIGINS")
	if envOrigins == "" {
		// Provide development defaults
		log.Println("⚠️  ALLOWED_ORIGINS not set, using development defaults")
		return []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"https://localhost:3000",
		}, nil
	}

	// Parse comma-separated origins
	origins := make([]string, 0)
	for _, origin := range strings.Split(envOrigins, ",") {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	if len(origins) == 0 {
		return nil, fmt.Errorf("ALLOWED_ORIGINS contains no valid origins")
	}

	log.Printf("🌐 Configured CORS origins: %v", origins)
	return origins, nil
}

// validateConfig validates required configuration values
func validateConfig(config *Config) error {
	var errors []string

	// Validate required fields based on environment
	if len(config.AllowedOrigins) == 0 {
		errors = append(errors, "at least one allowed origin must be configured")
	}

	if config.DatabaseURL == "" {
		errors = append(errors, "DATABASE_URL is required")
	}

	// Production-specific validations
	if config.Environment == EnvProduction {
		if config.OAuthConfig.ClientID == "" {
			errors = append(errors, "GOOGLE_CLIENT_ID is required in production")
		}
		if config.JWTSecret == "" {
			errors = append(errors, "JWT_SECRET is required in production")
		}
		if config.YouTubeAPIKey == "" {
			errors = append(errors, "YOUTUBE_API_KEY is required in production")
		}
	} else {
		// Development warnings (non-blocking)
		if config.OAuthConfig.ClientID == "" {
			log.Println("⚠️  GOOGLE_CLIENT_ID not set - Google OAuth will not work")
		}
		if config.JWTSecret == "" {
			log.Println("⚠️  JWT_SECRET not set - JWT authentication will not work")
		}
		if config.YouTubeAPIKey == "" {
			log.Println("⚠️  YOUTUBE_API_KEY not set - YouTube API calls will not work")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// GetConfig returns a singleton configuration instance
var globalConfig *Config

func GetConfig() *Config {
	if globalConfig == nil {
		config, err := LoadConfig()
		if err != nil {
			log.Fatalf("❌ Failed to load configuration: %v", err)
		}
		globalConfig = config
	}
	return globalConfig
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Environment == EnvProduction
}

// IsLocal returns true if running in local environment
func (c *Config) IsLocal() bool {
	return c.Environment == EnvLocal
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.Environment == EnvDevelopment
}

// PrintSummary prints a summary of the loaded configuration
func (c *Config) PrintSummary() {
	log.Printf("🌍 Environment: %s", c.Environment)
	log.Printf("🚀 Server port: %s", c.Port)
	log.Printf("🌐 CORS origins: %v", c.AllowedOrigins)
	log.Printf("🌐 Frontend URL: %s", c.FrontendURL)
	log.Printf("🔑 OAuth Client ID: %s", maskSecret(c.OAuthConfig.ClientID))
	log.Printf("🔑 OAuth Client Secret: %s", maskSecret(c.OAuthConfig.ClientSecret))
	log.Printf("🔑 JWT Secret: %s", maskSecret(c.JWTSecret))
	log.Printf("🔑 YouTube API Key: %s", maskSecret(c.YouTubeAPIKey))
	log.Printf("🔑 Target Channel ID: %s", c.TargetChannelID)
	log.Printf("🗄️  Database URL: %s", maskDatabaseURL(c.DatabaseURL))
	if c.RedisURL != "" {
		log.Printf("🔴 Redis URL: %s", maskDatabaseURL(c.RedisURL))
	}
	log.Printf("📊 Log Level: %s", c.LogLevel)
	log.Printf("🐛 Debug Mode: %t", c.Debug)
}

// Helper functions

// getEnvWithDefault gets an environment variable with a default value
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getBoolEnv gets a boolean environment variable with a default value
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// maskSecret masks sensitive configuration values for logging
func maskSecret(secret string) string {
	if secret == "" {
		return "(not set)"
	}
	if len(secret) <= 10 {
		return strings.Repeat("*", len(secret))
	}
	return secret[:10] + "..."
}

// maskDatabaseURL masks sensitive parts of database URLs for logging
func maskDatabaseURL(url string) string {
	if url == "" {
		return "(not set)"
	}
	
	// Hide password in database URLs
	if strings.Contains(url, "@") {
		parts := strings.Split(url, "@")
		if len(parts) >= 2 {
			// Mask everything before @ except the scheme
			beforeAt := parts[0]
			if strings.Contains(beforeAt, "://") {
				schemeParts := strings.Split(beforeAt, "://")
				if len(schemeParts) == 2 {
					return schemeParts[0] + "://***@" + strings.Join(parts[1:], "@")
				}
			}
		}
	}
	
	return url
}
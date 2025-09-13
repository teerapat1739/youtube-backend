package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration values for the application
type Config struct {
	Port              string
	AllowedOrigins    []string
	GoogleClientID    string
	YouTubeAPIKey     string
	YouTubeChannelID  string
	LogLevel          string
	DatabaseURL       string
	DatabaseReadURL   string // Read replica URL for SELECT queries
	RedisURL          string
	SupabaseURL       string
	SupabaseAnonKey   string
	SupabaseJWTSecret string
	Environment       string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	return &Config{
		Port:              getEnv("PORT", "8080"),
		AllowedOrigins:    parseOrigins(getEnv("ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:5174")),
		GoogleClientID:    getEnv("GOOGLE_CLIENT_ID", ""),
		YouTubeAPIKey:     getEnv("YOUTUBE_API_KEY", ""),
		YouTubeChannelID:  getEnv("YOUTUBE_CHANNEL_ID", "UC-chqi3Gpb4F7yBqedlnq5g"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		DatabaseURL:       getEnv("DATABASE_URL", ""),
		DatabaseReadURL:   getEnv("DATABASE_READ_URL", getEnv("DATABASE_URL", "")), // Falls back to write DB if not set
		RedisURL:          getEnv("REDIS_URL", ""),
		SupabaseURL:       getEnv("SUPABASE_URL", ""),
		SupabaseAnonKey:   getEnv("SUPABASE_ANON_KEY", ""),
		SupabaseJWTSecret: getEnv("SUPABASE_JWT_SECRET", ""),
		Environment:       getEnv("ENVIRONMENT", "production"),
	}, nil
}

// getEnv gets an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// parseOrigins parses comma-separated origins into a slice
func parseOrigins(origins string) []string {
	if origins == "" {
		return []string{}
	}

	parts := strings.Split(origins, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// getBoolEnv gets a boolean environment variable with a fallback value
func getBoolEnv(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return fallback
}

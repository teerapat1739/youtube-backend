package database

import (
	"context"
	"fmt"
	"time"

	"github.com/gamemini/youtube/pkg/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is the global database connection pool
var DB *pgxpool.Pool

// InitDB initializes the database connection based on centralized configuration
func InitDB() error {
	appConfig := config.GetConfig()

	switch appConfig.Environment {
	case config.EnvProduction:
		return InitProductionDB()
	case config.EnvDevelopment, config.EnvLocal:
		return InitLocalDB()
	default:
		return InitLocalDB() // Default to local for safety
	}
}

// InitLocalDB initializes local database connection
func InitLocalDB() error {
	databaseURL := config.GetConfig().DatabaseURL
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable is required")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("unable to parse DATABASE_URL: %v", err)
	}

	// Local development configuration
	config.MaxConns = 20
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	// Disable prepared statement cache to avoid conflicts
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	DB, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("unable to create connection pool: %v", err)
	}

	// Test connection
	if err := DB.Ping(ctx); err != nil {
		return fmt.Errorf("unable to ping database: %v", err)
	}

	fmt.Println("✅ Connected to local database")
	return nil
}

// InitProductionDB initializes production database connection
func InitProductionDB() error {
	databaseURL := config.GetConfig().DatabaseURL
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable is required")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("unable to parse DATABASE_URL: %v", err)
	}

	// Production configuration for high concurrency
	config.MaxConns = 100
	config.MinConns = 20
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	// Disable prepared statement cache to avoid conflicts
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	// GCP-specific configurations
	config.ConnConfig.RuntimeParams["application_name"] = "activity-landing-page"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	DB, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("unable to create connection pool: %v", err)
	}

	// Test connection
	if err := DB.Ping(ctx); err != nil {
		return fmt.Errorf("unable to ping database: %v", err)
	}

	fmt.Println("✅ Connected to production database")
	return nil
}

// CloseDB closes the database connection
func CloseDB() {
	if DB != nil {
		DB.Close()
		fmt.Println("🔌 Database connection closed")
	}
}

// GetDB returns the database connection pool
func GetDB() *pgxpool.Pool {
	return DB
}

// HealthCheck checks if the database is healthy
func HealthCheck() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return DB.Ping(ctx)
}

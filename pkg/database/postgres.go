package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDB struct {
	Pool     *pgxpool.Pool // Write pool (primary database)
	ReadPool *pgxpool.Pool // Read pool (read replica)
}

// NewPostgresDB creates a new PostgreSQL connection pool with optional read replica
func NewPostgresDB(ctx context.Context, databaseURL, readDatabaseURL string) (*PostgresDB, error) {
	// Create write pool
	writeConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure connection pool for Cloud Run
	writeConfig.MaxConns = 10
	writeConfig.MinConns = 2
	writeConfig.MaxConnLifetime = time.Hour
	writeConfig.MaxConnIdleTime = time.Minute * 30
	writeConfig.HealthCheckPeriod = time.Minute
	writeConfig.ConnConfig.ConnectTimeout = time.Second * 5

	writePool, err := pgxpool.NewWithConfig(ctx, writeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create write connection pool: %w", err)
	}

	// Test the write connection
	if err := writePool.Ping(ctx); err != nil {
		writePool.Close()
		return nil, fmt.Errorf("failed to ping write database: %w", err)
	}

	db := &PostgresDB{Pool: writePool}

	// Create read pool if read URL is provided and different from write URL
	if readDatabaseURL != "" && readDatabaseURL != databaseURL {
		readConfig, err := pgxpool.ParseConfig(readDatabaseURL)
		if err != nil {
			writePool.Close()
			return nil, fmt.Errorf("failed to parse read database URL: %w", err)
		}

		// Configure read pool with more connections since it handles most queries
		readConfig.MaxConns = 15
		readConfig.MinConns = 3
		readConfig.MaxConnLifetime = time.Hour
		readConfig.MaxConnIdleTime = time.Minute * 30
		readConfig.HealthCheckPeriod = time.Minute
		readConfig.ConnConfig.ConnectTimeout = time.Second * 5

		readPool, err := pgxpool.NewWithConfig(ctx, readConfig)
		if err != nil {
			writePool.Close()
			return nil, fmt.Errorf("failed to create read connection pool: %w", err)
		}

		// Test the read connection
		if err := readPool.Ping(ctx); err != nil {
			writePool.Close()
			readPool.Close()
			return nil, fmt.Errorf("failed to ping read database: %w", err)
		}

		db.ReadPool = readPool
	} else {
		// If no read replica, use write pool for reads
		db.ReadPool = writePool
	}

	return db, nil
}

// Close closes the database connection pools
func (db *PostgresDB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
	if db.ReadPool != nil && db.ReadPool != db.Pool {
		db.ReadPool.Close()
	}
}

// Health checks the database connection
func (db *PostgresDB) Health(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// GetReadPool returns the appropriate pool for read operations
func (db *PostgresDB) GetReadPool() *pgxpool.Pool {
	if db.ReadPool != nil {
		return db.ReadPool
	}
	return db.Pool
}

// GetWritePool returns the pool for write operations
func (db *PostgresDB) GetWritePool() *pgxpool.Pool {
	return db.Pool
}

// RefreshMaterializedView refreshes the vote_count_summary materialized view
func (db *PostgresDB) RefreshMaterializedView(ctx context.Context) error {
	// Note: Using non-concurrent refresh as the view doesn't have a unique index
	// CONCURRENTLY requires a unique index on the materialized view
	_, err := db.Pool.Exec(ctx, "REFRESH MATERIALIZED VIEW vote_count_summary")
	return err
}
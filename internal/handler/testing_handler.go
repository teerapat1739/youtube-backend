package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"be-v2/internal/container"
	"be-v2/pkg/database"
	"be-v2/pkg/redis"
)

// TestingHandler handles testing/development requests
type TestingHandler struct {
	container   *container.Container
	db          *database.PostgresDB
	redisClient *redis.Client
	environment string
}

// NewTestingHandler creates a new testing handler
func NewTestingHandler(container *container.Container, db *database.PostgresDB, redisClient *redis.Client) *TestingHandler {
	cfg := container.GetConfig()
	return &TestingHandler{
		container:   container,
		db:          db,
		redisClient: redisClient,
		environment: cfg.Environment,
	}
}

// RefreshResponse represents the response for refresh operations
type RefreshResponse struct {
	Status      string    `json:"status"`
	Message     string    `json:"message"`
	Environment string    `json:"environment"`
	Timestamp   time.Time `json:"timestamp"`
}

// ClearCacheResponse represents the response for cache clearing operations
type ClearCacheResponse struct {
	Status      string    `json:"status"`
	Message     string    `json:"message"`
	Environment string    `json:"environment"`
	KeysCleared int       `json:"keys_cleared"`
	Timestamp   time.Time `json:"timestamp"`
}

// RefreshMaterializedView handles POST /api/testing/refresh-materialized-view
// This endpoint is only available in development environment
func (h *TestingHandler) RefreshMaterializedView(w http.ResponseWriter, r *http.Request) {
	logger := h.container.GetLogger()

	// Check if we're in development environment
	if h.environment != "development" {
		logger.Warn("Attempted to access testing endpoint in non-development environment")
		
		response := RefreshResponse{
			Status:      "error",
			Message:     "This endpoint is only available in development environment",
			Environment: h.environment,
			Timestamp:   time.Now().UTC(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(response)
		return
	}

	logger.Info("Testing: Refreshing materialized view requested")

	// Create a context with timeout for the refresh operation
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Execute the refresh
	startTime := time.Now()
	err := h.db.RefreshMaterializedView(ctx)
	duration := time.Since(startTime)

	if err != nil {
		logger.WithError(err).Error("Testing: Failed to refresh materialized view")
		
		response := RefreshResponse{
			Status:      "error",
			Message:     "Failed to refresh materialized view: " + err.Error(),
			Environment: h.environment,
			Timestamp:   time.Now().UTC(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	logger.WithField("duration_ms", duration.Milliseconds()).Info("Testing: Materialized view refreshed successfully")

	response := RefreshResponse{
		Status:      "success",
		Message:     "Materialized view refreshed successfully (duration: " + duration.String() + ")",
		Environment: h.environment,
		Timestamp:   time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Testing: Failed to encode refresh response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// GetMaterializedViewStats handles GET /api/testing/materialized-view-stats
// Returns statistics about the materialized view (development only)
func (h *TestingHandler) GetMaterializedViewStats(w http.ResponseWriter, r *http.Request) {
	logger := h.container.GetLogger()

	// Check if we're in development environment
	if h.environment != "development" {
		logger.Warn("Attempted to access testing endpoint in non-development environment")
		http.Error(w, "This endpoint is only available in development environment", http.StatusForbidden)
		return
	}

	logger.Debug("Testing: Materialized view stats requested")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Query to get stats about the materialized view
	query := `
		SELECT 
			COUNT(*) as row_count,
			MIN(created_at) as oldest_entry,
			MAX(created_at) as newest_entry,
			COUNT(DISTINCT candidate_id) as unique_candidates
		FROM vote_count_summary
	`

	var stats struct {
		RowCount         int        `json:"row_count"`
		OldestEntry      *time.Time `json:"oldest_entry"`
		NewestEntry      *time.Time `json:"newest_entry"`
		UniqueCandidates int        `json:"unique_candidates"`
	}

	err := h.db.Pool.QueryRow(ctx, query).Scan(
		&stats.RowCount,
		&stats.OldestEntry,
		&stats.NewestEntry,
		&stats.UniqueCandidates,
	)

	if err != nil {
		logger.WithError(err).Error("Testing: Failed to get materialized view stats")
		http.Error(w, "Failed to get stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get materialized view metadata
	metaQuery := `
		SELECT 
			schemaname,
			matviewname,
			hasindexes,
			ispopulated
		FROM pg_matviews 
		WHERE matviewname = 'vote_count_summary'
	`

	var meta struct {
		SchemaName  string `json:"schema_name"`
		ViewName    string `json:"view_name"`
		HasIndexes  bool   `json:"has_indexes"`
		IsPopulated bool   `json:"is_populated"`
	}

	err = h.db.Pool.QueryRow(ctx, metaQuery).Scan(
		&meta.SchemaName,
		&meta.ViewName,
		&meta.HasIndexes,
		&meta.IsPopulated,
	)

	if err != nil {
		logger.WithError(err).Warn("Testing: Failed to get materialized view metadata")
	}

	response := map[string]interface{}{
		"environment": h.environment,
		"stats":       stats,
		"metadata":    meta,
		"timestamp":   time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Testing: Failed to encode stats response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	logger.Debug("Testing: Materialized view stats returned successfully")
}

// ClearRedisCache handles DELETE /api/testing/clear-redis-cache
// This endpoint clears all Redis cache keys for the current environment (development only)
func (h *TestingHandler) ClearRedisCache(w http.ResponseWriter, r *http.Request) {
	logger := h.container.GetLogger()

	// Check if we're in development environment
	if h.environment != "development" {
		logger.Warn("Attempted to clear Redis cache in non-development environment")
		
		response := ClearCacheResponse{
			Status:      "error",
			Message:     "This endpoint is only available in development environment",
			Environment: h.environment,
			KeysCleared: 0,
			Timestamp:   time.Now().UTC(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Optional: Check for a special header for additional protection
	if r.Header.Get("X-Clear-Cache-Confirm") != "yes" {
		logger.Warn("Testing: Cache clear request missing confirmation header")
		
		response := ClearCacheResponse{
			Status:      "error",
			Message:     "Missing confirmation header. Add 'X-Clear-Cache-Confirm: yes' to proceed",
			Environment: h.environment,
			KeysCleared: 0,
			Timestamp:   time.Now().UTC(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	logger.Info("Testing: Redis cache clear requested")

	// Create a context with timeout for the clear operation
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	startTime := time.Now()
	
	// Get the environment prefix to clear only keys for this environment
	prefix := h.redisClient.KeyBuilder.GetPrefix()
	pattern := prefix + ":*"

	// Clear all keys with the environment prefix
	// The InvalidatePattern method will handle counting internally
	err := h.redisClient.InvalidatePattern(ctx, pattern)
	if err != nil {
		logger.WithError(err).Error("Testing: Failed to clear Redis cache")
		
		response := ClearCacheResponse{
			Status:      "error",
			Message:     "Failed to clear Redis cache: " + err.Error(),
			Environment: h.environment,
			KeysCleared: -1, // Indicates error occurred
			Timestamp:   time.Now().UTC(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	duration := time.Since(startTime)

	logger.WithFields(map[string]interface{}{
		"duration_ms":   duration.Milliseconds(),
		"prefix":        prefix,
		"pattern":       pattern,
	}).Info("Testing: Redis cache cleared successfully")

	response := ClearCacheResponse{
		Status:      "success",
		Message:     fmt.Sprintf("Successfully cleared Redis cache keys with prefix '%s' (duration: %s)", prefix, duration.String()),
		Environment: h.environment,
		KeysCleared: 0, // We don't track the exact count with current implementation
		Timestamp:   time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Testing: Failed to encode clear cache response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gamemini/youtube/pkg/database"
)

// HandleHealthCheck handles the health check endpoint
func HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
		"services": map[string]interface{}{
			"database": CheckDatabaseHealth(),
			"api":      "running",
		},
	}

	json.NewEncoder(w).Encode(health)
}

// CheckDatabaseHealth checks if the database is healthy
func CheckDatabaseHealth() map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	db := database.GetDB()
	if db == nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  "database connection not initialized",
		}
	}

	if err := db.Ping(ctx); err != nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	}

	return map[string]interface{}{
		"status": "healthy",
	}
}
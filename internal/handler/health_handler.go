package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"be-v2/internal/container"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	container *container.Container
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(container *container.Container) *HealthHandler {
	return &HealthHandler{
		container: container,
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Service   string    `json:"service"`
}

// Check handles GET /health
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	logger := h.container.GetLogger()
	
	logger.Debug("Health check requested")
	
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
		Service:   "be-v2",
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to encode health check response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	logger.Debug("Health check completed successfully")
}
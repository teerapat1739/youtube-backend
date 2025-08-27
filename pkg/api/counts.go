package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gamemini/youtube/pkg/services"
	"github.com/gorilla/mux"
)

// CountsResponse represents the response for vote counts endpoint
type CountsResponse struct {
	Success bool       `json:"success"`
	Data    CountsData `json:"data"`
}

// CountsData contains the vote count data
type CountsData struct {
	ActivityID  string           `json:"activity_id"`
	Counts      map[string]int64 `json:"counts"`
	GeneratedAt string           `json:"generated_at"`
	Source      string           `json:"source"`
}

// HandleGetVoteCounts handles GET requests for vote counts with ETag support and minimal error handling
func HandleGetVoteCounts(w http.ResponseWriter, r *http.Request) {
	// Extract activity ID from path
	vars := mux.Vars(r)
	activityID := vars["id"]
	if activityID == "" {
		activityID = "active"
	}
	
	// Create context with 2s timeout
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	
	// Get counts from service
	voteCountService := services.NewVoteCountService()
	counts, source, err := voteCountService.GetCounts(ctx, activityID)
	
	// Handle errors - only 503 for genuine failures
	if err != nil {
		log.Printf("[API-COUNTS] Failed to get vote counts: %v", err)
		w.Header().Set("Retry-After", "5")
		http.Error(w, "Failed to retrieve vote counts", http.StatusServiceUnavailable)
		return
	}
	
	// Ensure counts is not nil
	if counts == nil {
		counts = make(map[string]int64)
	}
	
	// Create response data
	generatedAt := time.Now().UTC()
	responseData := CountsData{
		ActivityID:  activityID,
		Counts:      counts,
		GeneratedAt: generatedAt.Format(time.RFC3339),
		Source:      source,
	}
	
	// Generate stable JSON for ETag computation
	stableJSON := generateStableJSON(counts)
	
	// Compute ETag using SHA256
	hash := sha256.New()
	hash.Write([]byte(stableJSON))
	etag := fmt.Sprintf(`"%x"`, hash.Sum(nil)[:16]) // Use first 16 bytes for shorter ETag
	
	// Check If-None-Match header
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch == etag {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "public, s-maxage=10, stale-while-revalidate=30")
		w.WriteHeader(http.StatusNotModified)
		return
	}
	
	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, s-maxage=10, stale-while-revalidate=30")
	
	// Create and send response
	response := CountsResponse{
		Success: true,
		Data:    responseData,
	}
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[API-COUNTS] Failed to encode response: %v", err)
	}
}

// generateStableJSON creates a stable JSON representation of counts for ETag generation
func generateStableJSON(counts map[string]int64) string {
	// Sort keys for stable ordering
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	// Build stable JSON string
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf(`"%s":%d`, k, counts[k]))
	}
	
	return "{" + strings.Join(parts, ",") + "}"
}
package handler

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"be-v2/internal/config"
	"be-v2/internal/domain"
	"be-v2/internal/service"
	"be-v2/pkg/logger"

	"github.com/go-chi/chi/v5"
)

// VisitorHandler handles visitor tracking HTTP requests
type VisitorHandler struct {
	visitorService service.VisitorService
	votingService  *service.VotingService
	logger         *logger.Logger
	config         *config.Config
	supabaseClient *service.SupabaseClient
}

// NewVisitorHandler creates a new visitor handler
func NewVisitorHandler(visitorService service.VisitorService, votingService *service.VotingService, logger *logger.Logger, config *config.Config) *VisitorHandler {
	supabaseClient := service.NewSupabaseClient(config, logger)
	return &VisitorHandler{
		visitorService: visitorService,
		votingService:  votingService,
		logger:         logger,
		config:         config,
		supabaseClient: supabaseClient,
	}
}

// VisitResponse represents the response for visit recording
type VisitResponse struct {
	Success     bool                   `json:"success"`
	Message     string                 `json:"message"`
	RateLimit   *domain.RateLimitInfo  `json:"rate_limit,omitempty"`
	Error       *ErrorResponse         `json:"error,omitempty"`
}

// StatsResponse represents the response for visitor/vote statistics
// Using interface{} for Data to support both VisitorStats and VoteStats
type StatsResponse struct {
	Success bool           `json:"success"`
	Data    interface{}    `json:"data,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}


// RecordVisit handles POST /api/visitor/visit
func (h *VisitorHandler) RecordVisit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get real IP address
	ipAddress := h.getRealIPAddress(r)
	userAgent := r.UserAgent()

	// Record the visit
	rateLimitInfo, err := h.visitorService.RecordVisit(ctx, ipAddress, userAgent)
	if err != nil {
		h.logger.WithError(err).Error("Failed to record visit")
		h.sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to record visit")
		return
	}

	// Check if rate limited
	if !rateLimitInfo.IsAllowed {
		response := VisitResponse{
			Success:   false,
			Message:   "Rate limit exceeded. Please try again later.",
			RateLimit: rateLimitInfo,
		}
		
		// Set rate limit headers
		h.setRateLimitHeaders(w, rateLimitInfo)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.WithError(err).Error("Failed to encode rate limit response")
		}
		return
	}

	// Success response
	response := VisitResponse{
		Success:   true,
		Message:   "Visit recorded successfully",
		RateLimit: rateLimitInfo,
	}

	// Set rate limit headers
	h.setRateLimitHeaders(w, rateLimitInfo)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithError(err).Error("Failed to encode visit response")
		h.sendErrorResponse(w, http.StatusInternalServerError, "encoding_error", "Failed to encode response")
		return
	}

	h.logger.WithFields(map[string]interface{}{
		"ip":            ipAddress,
		"user_agent":    userAgent,
		"request_count": rateLimitInfo.RequestCount,
	}).Debug("Visit recorded successfully")
}

// GetStats handles GET /api/visitor/stats
// Now returns voting statistics from Supabase accumulate-slots function
func (h *VisitorHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Call Supabase accumulate-slots function to update stats with current vote data
	supabaseStats, err := h.callSupabaseAccumulate(ctx)
	if err != nil {
		h.logger.WithError(err).Error("Failed to call Supabase accumulate function")
		h.sendErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to update voting statistics")
		return
	}

	// Create response with the accumulated total from Supabase
	// The total represents the accumulated visits/votes
	voteStats := &domain.VoteStats{
		TotalVisits:  int64(supabaseStats.Total),
		DailyVisits:  0, // Not provided by Supabase function currently
		UniqueVisits: int64(supabaseStats.Total), // Using total as unique for now
		LastUpdated:  time.Now(),
	}

	response := StatsResponse{
		Success: true,
		Data:    voteStats,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithError(err).Error("Failed to encode stats response")
		h.sendErrorResponse(w, http.StatusInternalServerError, "encoding_error", "Failed to encode response")
		return
	}

	h.logger.WithFields(map[string]interface{}{
		"total_votes":   voteStats.TotalVisits,
		"daily_votes":   voteStats.DailyVisits,
		"unique_voters": voteStats.UniqueVisits,
		"last_updated":  voteStats.LastUpdated,
	}).Debug("Vote stats retrieved successfully from Supabase")
}

// GetHistoricalStats handles GET /api/visitor/historical
func (h *VisitorHandler) GetHistoricalStats(w http.ResponseWriter, r *http.Request) {
	// This could be extended to show historical data from PostgreSQL snapshots
	// For now, we'll just return current stats
	h.GetStats(w, r)
}

// HealthCheck handles GET /api/visitor/health
func (h *VisitorHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"success":   true,
		"service":   "visitor",
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithError(err).Error("Failed to encode health check response")
		h.sendErrorResponse(w, http.StatusInternalServerError, "encoding_error", "Failed to encode response")
		return
	}
}

// getRealIPAddress extracts the real IP address from the request
func (h *VisitorHandler) getRealIPAddress(r *http.Request) string {
	// Check for IP in various headers (in order of preference)
	headers := []string{
		"CF-Connecting-IP",    // Cloudflare
		"X-Forwarded-For",     // Standard proxy header
		"X-Real-IP",           // Nginx proxy
		"X-Client-IP",         // Apache proxy
		"X-Forwarded",         // Less common
		"Forwarded-For",       // Less common
		"Forwarded",           // Less common
	}

	for _, header := range headers {
		if ip := r.Header.Get(header); ip != "" {
			// X-Forwarded-For can contain multiple IPs, take the first one
			if header == "X-Forwarded-For" {
				if firstIP := getFirstIP(ip); firstIP != "" {
					return firstIP
				}
			} else {
				return ip
			}
		}
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// getFirstIP extracts the first IP from a comma-separated list
func getFirstIP(ips string) string {
	for i, char := range ips {
		if char == ',' || char == ' ' {
			return ips[:i]
		}
	}
	return ips
}

// setRateLimitHeaders sets standard rate limit headers
func (h *VisitorHandler) setRateLimitHeaders(w http.ResponseWriter, rateLimitInfo *domain.RateLimitInfo) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(60)) // 60 requests per hour
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(max(0, 60-rateLimitInfo.RequestCount), 10))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(rateLimitInfo.WindowStart.Add(rateLimitInfo.TTL).Unix(), 10))
}

// sendErrorResponse sends a standardized error response
func (h *VisitorHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, errorType, message string) {
	response := map[string]interface{}{
		"success": false,
		"error": ErrorResponse{
			Type:    errorType,
			Message: message,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithError(err).Error("Failed to encode error response")
	}
}

// max returns the maximum of two int64 values
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// callSupabaseAccumulate calls the Supabase accumulate-slots function with current stats
func (h *VisitorHandler) callSupabaseAccumulate(ctx context.Context) (*service.SupabaseAccumulateResponse, error) {
	// First, get the current vote stats from the voting service
	voteStats, err := h.getVoteStats(ctx)
	if err != nil {
		return nil, err
	}

	// Prepare the request body with current stats
	requestBody := map[string]interface{}{
		"total_visits":  voteStats.TotalVisits,
		"unique_visits": voteStats.UniqueVisits,
	}

	// Use the shared Supabase client
	return h.supabaseClient.FetchAccumulateSlots(ctx, requestBody)
}

// getVoteStats retrieves voting statistics and formats them for backward compatibility
// This method leverages the existing caching in VotingService.GetVotingStatus()
func (h *VisitorHandler) getVoteStats(ctx context.Context) (*domain.VoteStats, error) {
	// Get current voting status which includes total vote count
	// This call already uses Redis caching with TTL in the voting service
	votingStatus, err := h.votingService.GetVotingStatus(ctx, "")
	if err != nil {
		h.logger.WithError(err).Error("Failed to get voting status for stats")
		return nil, err
	}

	// Create VoteStats with the same structure as VisitorStats for backward compatibility
	voteStats := &domain.VoteStats{
		TotalVisits:  int64(votingStatus.TotalVotes), // Total votes cast
		DailyVisits:  0,                              // Daily votes - could be implemented later
		UniqueVisits: int64(votingStatus.TotalVotes), // Same as total votes in our voting system
		LastUpdated:  votingStatus.LastUpdate,        // Use the actual last update time from voting status
	}

	h.logger.WithFields(map[string]interface{}{
		"total_votes": voteStats.TotalVisits,
		"last_update": voteStats.LastUpdated,
	}).Debug("Vote stats generated from voting status")

	return voteStats, nil
}

// RegisterRoutes registers visitor handler routes with the router
func (h *VisitorHandler) RegisterRoutes(r chi.Router) {
	r.Route("/visitor", func(r chi.Router) {
		// Public endpoints
		r.Post("/visit", h.RecordVisit)
		r.Get("/stats", h.GetStats)
		r.Get("/health", h.HealthCheck)
	})
}
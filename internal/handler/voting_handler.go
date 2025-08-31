package handler

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"be-v2/internal/domain"
	"be-v2/internal/middleware"
	"be-v2/internal/service"
	"github.com/go-chi/chi/v5"
)

type VotingHandler struct {
	votingService *service.VotingService
}

func NewVotingHandler(votingService *service.VotingService) *VotingHandler {
	return &VotingHandler{
		votingService: votingService,
	}
}

// GetVotingStatus handles GET /api/v1/voting/status (polling endpoint)
func (h *VotingHandler) GetVotingStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get user ID from context (set by auth middleware)
	userID := h.getUserID(r)
	if userID == "" {
		userID = "anonymous"
	}

	// Get voting status
	status, err := h.votingService.GetVotingStatus(ctx, userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "Failed to get voting status")
		return
	}

	// Generate ETag based on content
	etag := h.generateETag(status)
	
	// Check If-None-Match header
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Set ETag and Cache-Control headers
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=10")
	
	h.respondJSON(w, http.StatusOK, status)
}

// SubmitVote handles POST /api/v1/voting/vote
func (h *VotingHandler) SubmitVote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from auth context
	userID := h.getUserID(r)
	if userID == "" {
		h.respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Parse request body
	var req domain.VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := h.validateVoteRequest(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get client IP and User-Agent
	ipAddress := h.getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	// Submit vote
	response, err := h.votingService.SubmitVote(ctx, userID, &req, ipAddress, userAgent)
	if err != nil {
		// Log the actual error for debugging
		fmt.Printf("Vote submission error: %v\n", err)
		
		if strings.Contains(err.Error(), "already voted") {
			h.respondError(w, http.StatusConflict, "You have already voted")
			return
		}
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, "Team not found")
			return
		}
		h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to submit vote: %v", err))
		return
	}

	h.respondJSON(w, http.StatusCreated, response)
}

// VerifyVote handles GET /api/v1/voting/verify/:voteId
func (h *VotingHandler) VerifyVote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	voteID := chi.URLParam(r, "voteId")

	if voteID == "" {
		h.respondError(w, http.StatusBadRequest, "Vote ID is required")
		return
	}

	vote, err := h.votingService.VerifyVote(ctx, voteID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, "Vote not found")
			return
		}
		h.respondError(w, http.StatusInternalServerError, "Failed to verify vote")
		return
	}

	h.respondJSON(w, http.StatusOK, vote)
}

// GetMyVoteStatus handles GET /api/v1/voting/my-status
func (h *VotingHandler) GetMyVoteStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from auth context
	userID := h.getUserID(r)
	if userID == "" {
		h.respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	vote, err := h.votingService.GetUserVoteStatus(ctx, userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "Failed to get vote status")
		return
	}

	if vote == nil {
		h.respondJSON(w, http.StatusOK, map[string]interface{}{
			"has_voted": false,
		})
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"has_voted": true,
		"vote_id":   vote.VoteID,
		"team_id":   vote.TeamID,
		"voted_at":  vote.CreatedAt,
	})
}

// GetVotingResults handles GET /api/v1/voting/results
func (h *VotingHandler) GetVotingResults(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get voting results
	results, err := h.votingService.GetVotingResults(ctx)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "Failed to get voting results")
		return
	}

	// Generate ETag based on content
	etag := h.generateETag(results)
	
	// Check If-None-Match header for caching
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Set caching headers
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=30") // Cache for 30 seconds
	
	h.respondJSON(w, http.StatusOK, results)
}

// Helper methods

func (h *VotingHandler) getUserID(r *http.Request) string {
	// Get user from context (set by auth middleware)
	if user, ok := r.Context().Value(middleware.UserContextKey).(*domain.UserProfile); ok && user != nil {
		return user.Sub // This is the actual user ID from the token
	}
	// Return empty string if no authenticated user
	// This ensures voting endpoints require proper authentication
	return ""
}

func (h *VotingHandler) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for Cloud Run)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	
	// Handle IPv6 addresses with brackets
	if strings.HasPrefix(ip, "[") {
		// IPv6 address like [::1]:port
		if idx := strings.LastIndex(ip, "]:"); idx != -1 {
			// Remove brackets and port
			ip = ip[1:idx]
		}
	} else {
		// IPv4 address like 127.0.0.1:port
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}
	}
	
	// Convert ::1 to 127.0.0.1 for localhost
	if ip == "::1" {
		return "127.0.0.1"
	}
	
	return ip
}

func (h *VotingHandler) validateVoteRequest(req *domain.VoteRequest) error {
	if req.TeamID <= 0 {
		return fmt.Errorf("invalid team ID")
	}
	
	// Validate personal info
	if req.PersonalInfo.FirstName == "" || len(req.PersonalInfo.FirstName) < 2 {
		return fmt.Errorf("first name is required (min 2 characters)")
	}
	
	if req.PersonalInfo.LastName == "" || len(req.PersonalInfo.LastName) < 2 {
		return fmt.Errorf("last name is required (min 2 characters)")
	}
	
	if req.PersonalInfo.Email == "" || !strings.Contains(req.PersonalInfo.Email, "@") {
		return fmt.Errorf("valid email is required")
	}
	
	if req.PersonalInfo.Phone == "" || len(req.PersonalInfo.Phone) < 10 {
		return fmt.Errorf("phone number is required (min 10 digits)")
	}
	
	// Validate PDPA consent
	if !req.Consent.PDPAConsent {
		return fmt.Errorf("PDPA consent is required to proceed")
	}
	
	if req.Consent.PrivacyPolicyVersion == "" {
		return fmt.Errorf("privacy policy version is required")
	}
	
	return nil
}

func (h *VotingHandler) generateETag(data interface{}) string {
	jsonData, _ := json.Marshal(data)
	hash := md5.Sum(jsonData)
	return fmt.Sprintf(`"%x"`, hash)
}

func (h *VotingHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *VotingHandler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{
		"error": message,
	})
}
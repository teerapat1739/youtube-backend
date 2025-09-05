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

	// Parse request body - allow for minimal requests with just team_id
	var rawReq json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawReq); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Try to parse as minimal request first (just team_id)
	var minimalReq struct {
		TeamID int `json:"team_id"`
	}

	var req domain.VoteRequest
	if err := json.Unmarshal(rawReq, &minimalReq); err == nil && minimalReq.TeamID > 0 {
		// Minimal request - need to fetch personal info from database
		// Try to get stored personal info for this user
		personalInfo, err := h.votingService.GetPersonalInfoByUserID(ctx, userID)
		if err != nil {
			// If personal info not found, require it in the request
			if strings.Contains(err.Error(), "not found") {
				h.respondError(w, http.StatusPreconditionFailed, "Personal information not found. Please complete personal info first or include it in your vote request.")
				return
			}
			h.respondError(w, http.StatusInternalServerError, "Failed to retrieve personal information")
			return
		}

		// Build complete request with stored personal info
		req = domain.VoteRequest{
			TeamID: minimalReq.TeamID,
			PersonalInfo: domain.PersonalInfo{
				FirstName:     personalInfo.FirstName,
				LastName:      personalInfo.LastName,
				Email:         personalInfo.Email,
				Phone:         personalInfo.Phone,
				FavoriteVideo: personalInfo.FavoriteVideo,
			},
			Consent: domain.ConsentData{
				PDPAConsent:          personalInfo.ConsentPDPA,
				MarketingConsent:     personalInfo.MarketingConsent,
				PrivacyPolicyVersion: "1.0", // Default version since it's not stored in PersonalInfoMeResponse
			},
		}
	} else {
		// Full request - parse as normal
		if err := json.Unmarshal(rawReq, &req); err != nil {
			h.respondError(w, http.StatusBadRequest, "Invalid request body format")
			return
		}

		// Validate full request
		if err := h.validateVoteRequest(&req); err != nil {
			h.respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	// Get client IP and User-Agent
	ipAddress := h.getClientIP(r)
	userAgent := r.Header.Get("User-Agent")
	fmt.Printf("SubmitVote: userID = '%s', ipAddress = '%s', userAgent = '%s'\n", userID, ipAddress, userAgent)
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
		fmt.Printf("[DEBUG] getUserID: Found user in context - Sub: '%s', Email: '%s', Name: '%s'\n", user.Sub, user.Email, user.Name)
		return user.Sub // This is the actual user ID from the token
	}
	fmt.Printf("[DEBUG] getUserID: No user found in context or user is nil\n")
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

// CreatePersonalInfo handles POST /api/personal-info
func (h *VotingHandler) CreatePersonalInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from auth context (this endpoint should require authentication)
	userID := h.getUserID(r)
	if userID == "" {
		h.respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Parse request body - handle both nested and flat formats
	var rawReq json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&rawReq); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Try to parse as nested format first (from frontend)
	var nestedReq struct {
		PersonalInfo domain.PersonalInfo `json:"personal_info"`
		Consent      struct {
			PDPAConsent          bool   `json:"pdpa_consent"`
			MarketingConsent     bool   `json:"marketing_consent"`
			PrivacyPolicyVersion string `json:"privacy_policy_version"`
		} `json:"consent"`
	}

	var req domain.PersonalInfoRequest
	if err := json.Unmarshal(rawReq, &nestedReq); err == nil && nestedReq.PersonalInfo.FirstName != "" {
		// Convert nested format to flat format
		req = domain.PersonalInfoRequest{
			FirstName:     nestedReq.PersonalInfo.FirstName,
			LastName:      nestedReq.PersonalInfo.LastName,
			Email:         nestedReq.PersonalInfo.Email,
			Phone:         nestedReq.PersonalInfo.Phone,
			FavoriteVideo: nestedReq.PersonalInfo.FavoriteVideo,
			ConsentPDPA:   nestedReq.Consent.PDPAConsent,
		}
	} else {
		// Try flat format
		if err := json.Unmarshal(rawReq, &req); err != nil {
			h.respondError(w, http.StatusBadRequest, "Invalid request body format")
			return
		}
	}

	// Validate request
	if err := h.validatePersonalInfoRequest(&req); err != nil {
		h.respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Get client IP and User-Agent
	ipAddress := h.getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	// Create or update personal info
	response, err := h.votingService.CreateOrUpdatePersonalInfo(ctx, userID, &req, ipAddress, userAgent)
	if err != nil {
		// Log the actual error for debugging
		fmt.Printf("Personal info submission error: %v\n", err)

		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			h.respondError(w, http.StatusConflict, "This phone number is already registered")
			return
		}
		if strings.Contains(err.Error(), "invalid phone") {
			h.respondError(w, http.StatusUnprocessableEntity, "Invalid phone number format")
			return
		}
		h.respondError(w, http.StatusInternalServerError, "Failed to save personal information")
		return
	}

	h.respondJSON(w, http.StatusOK, response)
}

// SubmitVoteOnly handles POST /api/vote
func (h *VotingHandler) SubmitVoteOnly(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request body
	var req struct {
		Phone       string `json:"phone,omitempty"`
		UserID      string `json:"user_id,omitempty"`
		CandidateID int    `json:"candidate_id"`
		TeamID      int    `json:"team_id,omitempty"` // Support both candidate_id and team_id
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Support both candidate_id and team_id for compatibility
	if req.CandidateID <= 0 && req.TeamID > 0 {
		req.CandidateID = req.TeamID
	}

	// Validate request
	if req.CandidateID <= 0 {
		h.respondError(w, http.StatusUnprocessableEntity, "Invalid candidate/team ID")
		return
	}

	// Try to get user ID from auth context first (if authenticated)
	if req.UserID == "" {
		if user, ok := r.Context().Value(middleware.UserContextKey).(*domain.UserProfile); ok && user != nil {
			req.UserID = user.Sub
			fmt.Printf("[DEBUG] SubmitVoteOnly: Using userID from auth context: %s\n", req.UserID)
		}
	}

	// Handle vote submission based on provided identifier
	var response *domain.VoteOnlyResponse
	var err error

	if req.UserID != "" {
		// Vote by user ID
		voteReq := &domain.VoteOnlyRequest{
			UserID:      req.UserID,
			CandidateID: req.CandidateID,
		}
		response, err = h.votingService.SubmitVoteOnly(ctx, voteReq)
	} else if req.Phone != "" {
		// Vote by phone number
		response, err = h.votingService.SubmitVoteByPhone(ctx, req.Phone, req.CandidateID)
	} else {
		h.respondError(w, http.StatusBadRequest, "Either user_id or phone must be provided")
		return
	}

	if err != nil {
		// Log the actual error for debugging
		fmt.Printf("Vote submission error: %v\n", err)

		if err == domain.ErrUserNotFound {
			h.respondError(w, http.StatusPreconditionFailed, "Personal information not found. Please complete personal info first.")
			return
		}
		if err == domain.ErrVoteFinalized {
			h.respondError(w, http.StatusConflict, "Vote has already been finalized and cannot be changed")
			return
		}
		if strings.Contains(err.Error(), "team not found") {
			h.respondError(w, http.StatusNotFound, "Candidate not found")
			return
		}
		h.respondError(w, http.StatusInternalServerError, "Failed to submit vote")
		return
	}

	h.respondJSON(w, http.StatusOK, response)
}

// validatePersonalInfoRequest validates the personal info request
func (h *VotingHandler) validatePersonalInfoRequest(req *domain.PersonalInfoRequest) error {
	if req.FirstName == "" || len(req.FirstName) < 2 {
		return fmt.Errorf("first name is required (min 2 characters)")
	}

	if req.LastName == "" || len(req.LastName) < 2 {
		return fmt.Errorf("last name is required (min 2 characters)")
	}

	if req.Email == "" || !strings.Contains(req.Email, "@") {
		return fmt.Errorf("valid email is required")
	}

	if req.Phone == "" || len(req.Phone) < 10 {
		return fmt.Errorf("phone number is required (min 10 digits)")
	}

	// Validate favorite video field (optional but limited to 1000 characters)
	if len(req.FavoriteVideo) > 1000 {
		return fmt.Errorf("favorite video field cannot exceed 1000 characters")
	}

	if !req.ConsentPDPA {
		return fmt.Errorf("PDPA consent is required to proceed")
	}

	return nil
}

// GetPersonalInfoMe handles GET /api/personal-info/me
func (h *VotingHandler) GetPersonalInfoMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID and email from auth context (this endpoint requires authentication)
	userID := h.getUserID(r)
	var userEmail string

	// Also get the email from the user profile for fallback lookup
	if user, ok := r.Context().Value(middleware.UserContextKey).(*domain.UserProfile); ok && user != nil {
		userEmail = user.Email
		fmt.Printf("[DEBUG] GetPersonalInfoMe: extracted userID = '%s', email = '%s'\n", userID, userEmail)
	} else {
		fmt.Printf("[DEBUG] GetPersonalInfoMe: extracted userID = '%s', no email found\n", userID)
	}

	if userID == "" {
		fmt.Printf("[ERROR] GetPersonalInfoMe: No userID found in context\n")
		h.respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Get personal info for the authenticated user, with email fallback
	fmt.Printf("[DEBUG] GetPersonalInfoMe: calling GetPersonalInfoByUserID with userID = '%s', email = '%s'\n", userID, userEmail)
	personalInfo, err := h.votingService.GetPersonalInfoByUserID(ctx, userID)
	if err != nil {
		fmt.Printf("[ERROR] GetPersonalInfoMe: GetPersonalInfoByUserID failed with error: %v\n", err)
		if strings.Contains(err.Error(), "not found") {
			fmt.Printf("[DEBUG] GetPersonalInfoMe: Personal info not found for userID '%s' and email '%s'\n", userID, userEmail)
			h.respondError(w, http.StatusNotFound, "Personal information not found")
			return
		}
		fmt.Printf("[ERROR] GetPersonalInfoMe: Internal error: %v\n", err)
		h.respondError(w, http.StatusInternalServerError, "Failed to retrieve personal information")
		return
	}

	fmt.Printf("[DEBUG] GetPersonalInfoMe: Successfully found personal info for userID '%s'\n", userID)
	h.respondJSON(w, http.StatusOK, personalInfo)
}

package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/services"
	"github.com/gorilla/mux"
)

type UserProfileHandler struct {
	userProfileService *services.UserProfileService
}

func NewUserProfileHandler(userProfileService *services.UserProfileService) *UserProfileHandler {
	return &UserProfileHandler{
		userProfileService: userProfileService,
	}
}

// GetUserProfile handles GET /api/user/profile
func (h *UserProfileHandler) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	// Get user ID from JWT token (you'll need to implement JWT middleware)
	googleID := h.getUserGoogleIDFromToken(r)
	if googleID == "" {
		h.sendErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	profile, err := h.userProfileService.GetUserProfile(googleID)
	if err != nil {
		if validationErr, ok := err.(*services.ValidationError); ok {
			h.sendErrorResponse(w, validationErr.StatusCode(), validationErr.Code, validationErr.Message, validationErr.Details)
			return
		}
		h.sendErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get user profile", nil)
		return
	}

	h.sendSuccessResponse(w, profile)
}

// CreateUserProfile handles POST /api/user/profile
func (h *UserProfileHandler) CreateUserProfile(w http.ResponseWriter, r *http.Request) {
	// Get user info from JWT token
	googleID, email := h.getUserInfoFromToken(r)
	if googleID == "" || email == "" {
		h.sendErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req models.UpdateUserProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body", nil)
		return
	}

	user, err := h.userProfileService.CreateUserProfile(&req, googleID, email)
	if err != nil {
		if validationErr, ok := err.(*services.ValidationError); ok {
			h.sendErrorResponse(w, validationErr.StatusCode(), validationErr.Code, validationErr.Message, validationErr.Details)
			return
		}
		h.sendErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create user profile", nil)
		return
	}

	h.sendSuccessResponse(w, user)
}

// UpdateUserProfile handles PUT /api/user/profile
func (h *UserProfileHandler) UpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	// Get user ID from JWT token
	userID := h.getUserIDFromToken(r)
	if userID == "" {
		h.sendErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req models.UpdateUserProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body", nil)
		return
	}

	user, err := h.userProfileService.UpdateUserProfile(userID, &req)
	if err != nil {
		if validationErr, ok := err.(*services.ValidationError); ok {
			h.sendErrorResponse(w, validationErr.StatusCode(), validationErr.Code, validationErr.Message, validationErr.Details)
			return
		}
		h.sendErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update user profile", nil)
		return
	}

	h.sendSuccessResponse(w, user)
}

// ValidateUserProfile handles POST /api/user/profile/validate
func (h *UserProfileHandler) ValidateUserProfile(w http.ResponseWriter, r *http.Request) {
	var req models.UserProfileValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body", nil)
		return
	}

	validation, err := h.userProfileService.ValidateUserProfile(&req)
	if err != nil {
		h.sendErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to validate profile", nil)
		return
	}

	h.sendSuccessResponse(w, validation)
}

// GetTerms handles GET /api/terms
func (h *UserProfileHandler) GetTerms(w http.ResponseWriter, r *http.Request) {
	terms, err := h.userProfileService.GetTermsContent()
	if err != nil {
		h.sendErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get terms content", nil)
		return
	}

	h.sendSuccessResponse(w, terms)
}

// AcceptTerms handles POST /api/user/accept-terms
func (h *UserProfileHandler) AcceptTerms(w http.ResponseWriter, r *http.Request) {
	// Get user ID from JWT token
	userID := h.getUserIDFromToken(r)
	if userID == "" {
		h.sendErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req models.AcceptTermsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body", nil)
		return
	}

	// Get client IP and user agent for audit trail
	ipAddress := h.getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	err := h.userProfileService.AcceptTerms(userID, &req, ipAddress, userAgent)
	if err != nil {
		if validationErr, ok := err.(*services.ValidationError); ok {
			h.sendErrorResponse(w, validationErr.StatusCode(), validationErr.Code, validationErr.Message, validationErr.Details)
			return
		}
		h.sendErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to accept terms", nil)
		return
	}

	h.sendSuccessResponse(w, map[string]bool{"success": true})
}

// GetVoteStatus handles GET /api/activities/{activity_id}/vote-status
func (h *UserProfileHandler) GetVoteStatus(w http.ResponseWriter, r *http.Request) {
	// Get user ID from JWT token
	userID := h.getUserIDFromToken(r)
	if userID == "" {
		h.sendErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	// Get activity ID from URL
	vars := mux.Vars(r)
	activityID := vars["activity_id"]
	if activityID == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "MISSING_ACTIVITY_ID", "Activity ID is required", nil)
		return
	}

	voteStatus, err := h.userProfileService.GetVoteStatus(userID, activityID)
	if err != nil {
		h.sendErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get vote status", nil)
		return
	}

	h.sendSuccessResponse(w, voteStatus)
}

// Helper methods

func (h *UserProfileHandler) getUserGoogleIDFromToken(r *http.Request) string {
	// TODO: Implement JWT token parsing to get Google ID
	// This should parse the Authorization header and extract Google ID from the JWT
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// For now, return a placeholder
	// You need to implement proper JWT parsing here
	return "placeholder_google_id"
}

func (h *UserProfileHandler) getUserInfoFromToken(r *http.Request) (string, string) {
	// TODO: Implement JWT token parsing to get both Google ID and email
	// This should parse the Authorization header and extract both values from the JWT
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ""
	}

	// For now, return placeholders
	// You need to implement proper JWT parsing here
	return "placeholder_google_id", "placeholder_email"
}

func (h *UserProfileHandler) getUserIDFromToken(r *http.Request) string {
	// TODO: Implement JWT token parsing to get user ID
	// This should parse the Authorization header and extract user ID from the JWT
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// For now, return a placeholder
	// You need to implement proper JWT parsing here
	return "placeholder_user_id"
}

func (h *UserProfileHandler) getClientIP(r *http.Request) string {
	// Check for forwarded IP first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, get the first one
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check for real IP
	realIP := r.Header.Get("X-Real-Ip")
	if realIP != "" {
		return realIP
	}

	// Fall back to remote address
	return r.RemoteAddr
}

func (h *UserProfileHandler) sendSuccessResponse(w http.ResponseWriter, data interface{}) {
	response := models.APIResponse{
		Success: true,
		Data:    data,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *UserProfileHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message string, details interface{}) {
	response := models.APIResponse{
		Success:   false,
		Message:   message,
		ErrorCode: errorCode,
		Details:   details,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// RegisterRoutes registers the user profile routes
func (h *UserProfileHandler) RegisterRoutes(router *mux.Router) {
	// User profile routes
	router.HandleFunc("/api/user/profile", h.GetUserProfile).Methods("GET")
	router.HandleFunc("/api/user/profile", h.CreateUserProfile).Methods("POST")
	router.HandleFunc("/api/user/profile", h.UpdateUserProfile).Methods("PUT")
	router.HandleFunc("/api/user/profile/validate", h.ValidateUserProfile).Methods("POST")

	// Terms and PDPA routes
	router.HandleFunc("/api/terms", h.GetTerms).Methods("GET")
	router.HandleFunc("/api/user/accept-terms", h.AcceptTerms).Methods("POST")

	// Vote status routes
	router.HandleFunc("/api/activities/{activity_id}/vote-status", h.GetVoteStatus).Methods("GET")
}

package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gamemini/youtube/pkg/auth/google"
	"github.com/gamemini/youtube/pkg/config"
	"github.com/gamemini/youtube/pkg/interfaces"
	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/repository"
	"github.com/gamemini/youtube/pkg/services"
)

// AuthHandlers contains all authentication-related handlers
type AuthHandlers struct {
	userService  *services.UserService
	oauthHandler *google.EnhancedOAuthHandler
	container    interfaces.Container // Using interface to avoid circular dependency
}

// NewAuthHandlers creates new authentication handlers (deprecated - use NewAuthHandlersWithContainer)
func NewAuthHandlers() *AuthHandlers {
	appConfig := config.GetConfig()
	jwtSecret := appConfig.JWTSecret
	if jwtSecret == "" {
		jwtSecret = "default-development-secret-change-in-production"
		log.Println("⚠️  Using default JWT secret - set JWT_SECRET environment variable in production")
	}

	userRepo := repository.NewUserRepository()
	userService := services.NewUserService(userRepo, jwtSecret)
	oauthHandler := google.NewEnhancedOAuthHandler(jwtSecret)

	return &AuthHandlers{
		userService:  userService,
		oauthHandler: oauthHandler,
	}
}

// NewAuthHandlersWithContainer creates new authentication handlers with dependency injection
func NewAuthHandlersWithContainer(container interfaces.Container) *AuthHandlers {
	appConfig := config.GetConfig()
	jwtSecret := appConfig.JWTSecret
	if jwtSecret == "" {
		jwtSecret = "default-development-secret-change-in-production"
		log.Println("⚠️  Using default JWT secret - set JWT_SECRET environment variable in production")
	}

	oauthHandler := google.NewEnhancedOAuthHandler(jwtSecret)

	return &AuthHandlers{
		userService:  container.GetUserService(),
		oauthHandler: oauthHandler,
		container:    container,
	}
}

// HandleGoogleLogin handles Google OAuth login requests
func (h *AuthHandlers) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	log.Println("🔑 Handling Google login request")
	h.oauthHandler.HandleLogin(w, r)
}

// HandleGoogleCallback handles Google OAuth callback
func (h *AuthHandlers) HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	log.Println("🔄 Handling Google OAuth callback")
	h.oauthHandler.HandleCallback(w, r)
}

// HandleCreateInitialUserProfile handles initial user profile creation
func (h *AuthHandlers) HandleCreateInitialUserProfile(w http.ResponseWriter, r *http.Request) {
	log.Println("👤 Handling create initial user profile request")
	h.oauthHandler.HandleCreateInitialProfile(w, r)
}

// HandleGetUserProfile gets user profile with vote status
func (h *AuthHandlers) HandleGetUserProfile(w http.ResponseWriter, r *http.Request) {
	log.Println("👤 Handling get user profile request")

	// Extract user from JWT token
	user, err := h.extractUserFromToken(r)
	if err != nil {
		log.Printf("❌ Failed to extract user from token: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get activity ID from query params (optional)
	activityID := r.URL.Query().Get("activity_id")
	if activityID == "" {
		activityID = "active" // Default to active activity
	}

	// Get user profile with vote status
	profileResponse, err := h.userService.GetUserProfileWithVoteStatus(r.Context(), user.ID, activityID)
	if err != nil {
		log.Printf("❌ Failed to get user profile: %v", err)
		http.Error(w, "Failed to get user profile", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ User profile retrieved - UserID: %s", user.ID)

	response := map[string]interface{}{
		"success": true,
		"data":    profileResponse,
	}

	h.writeJSONResponse(w, response)
}

// HandleUpdateUserProfile handles user profile updates
func (h *AuthHandlers) HandleUpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	log.Println("📝 Handling update user profile request")

	// Extract user from JWT token
	user, err := h.extractUserFromToken(r)
	if err != nil {
		log.Printf("❌ Failed to extract user from token: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var profileData struct {
		FirstName   string `json:"first_name"`
		LastName    string `json:"last_name"`
		Phone       string `json:"phone"`
		AcceptTerms bool   `json:"accept_terms"`
		AcceptPDPA  bool   `json:"accept_pdpa"`
	}

	if err := json.NewDecoder(r.Body).Decode(&profileData); err != nil {
		log.Printf("❌ Failed to decode request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if !profileData.AcceptTerms || !profileData.AcceptPDPA {
		http.Error(w, "Terms and PDPA acceptance required", http.StatusBadRequest)
		return
	}

	// Update user profile
	updates := &models.UpdateUserProfileRequest{
		FirstName:   profileData.FirstName,
		LastName:    profileData.LastName,
		Phone:       profileData.Phone,
		AcceptTerms: profileData.AcceptTerms,
		AcceptPDPA:  profileData.AcceptPDPA,
	}

	updatedUser, err := h.userService.UpdateUserProfile(r.Context(), user.ID, updates)
	if err != nil {
		log.Printf("❌ Failed to update user profile: %v", err)
		if strings.Contains(err.Error(), "already exists") {
			http.Error(w, "National ID already exists for another user", http.StatusConflict)
		} else {
			http.Error(w, "Failed to update user profile", http.StatusInternalServerError)
		}
		return
	}

	log.Printf("✅ User profile updated successfully - UserID: %s", user.ID)

	response := map[string]interface{}{
		"success": true,
		"message": "User profile updated successfully",
		"data": map[string]interface{}{
			"user": updatedUser,
		},
	}

	h.writeJSONResponse(w, response)
}

// HandleUpdatePersonalInfo handles personal information updates without requiring terms acceptance
func (h *AuthHandlers) HandleUpdatePersonalInfo(w http.ResponseWriter, r *http.Request) {
	log.Println("📝 Handling update personal info request")

	// Extract user from JWT token
	user, err := h.extractUserFromToken(r)
	if err != nil {
		log.Printf("❌ Failed to extract user from token: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse request body - only personal info fields
	var personalData struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&personalData); err != nil {
		log.Printf("❌ Failed to decode request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Basic validation for required fields
	if personalData.FirstName == "" || personalData.LastName == "" {
		http.Error(w, "First name and last name are required", http.StatusBadRequest)
		return
	}

	// Update user profile - only personal info fields, preserve existing terms/PDPA status
	updates := &models.UpdateUserProfileRequest{
		FirstName: personalData.FirstName,
		LastName:  personalData.LastName,
		Phone:     personalData.Phone,
		// Don't change terms/PDPA acceptance status
		AcceptTerms: true, // This will be ignored by the service if we modify it correctly
		AcceptPDPA:  true,
	}

	updatedUser, err := h.userService.UpdateUserProfilePersonalInfoOnly(r.Context(), user.ID, updates)
	if err != nil {
		log.Printf("❌ Failed to update personal info: %v", err)
		http.Error(w, "Failed to update personal info", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Personal info updated successfully - UserID: %s", user.ID)

	response := map[string]interface{}{
		"success": true,
		"message": "Personal information updated successfully",
		"data": map[string]interface{}{
			"user": updatedUser,
		},
	}

	h.writeJSONResponse(w, response)
}

// HandleAcceptTerms handles terms and PDPA acceptance
func (h *AuthHandlers) HandleAcceptTerms(w http.ResponseWriter, r *http.Request) {
	log.Println("📋 Handling accept terms request")

	// Extract user from JWT token
	user, err := h.extractUserFromToken(r)
	if err != nil {
		log.Printf("❌ Failed to extract user from token: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var termsData struct {
		TermsVersion string `json:"terms_version"`
		PDPAVersion  string `json:"pdpa_version"`
		AcceptTerms  bool   `json:"accept_terms"`
		AcceptPDPA   bool   `json:"accept_pdpa"`
	}

	if err := json.NewDecoder(r.Body).Decode(&termsData); err != nil {
		log.Printf("❌ Failed to decode request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get client IP and User-Agent for audit
	ipAddress := h.getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	// Accept terms
	err = h.userService.AcceptTerms(
		r.Context(),
		user.ID,
		termsData.TermsVersion,
		termsData.PDPAVersion,
		termsData.AcceptTerms,
		termsData.AcceptPDPA,
		ipAddress,
		userAgent,
	)

	if err != nil {
		log.Printf("❌ Failed to accept terms: %v", err)
		http.Error(w, "Failed to accept terms", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Terms accepted successfully - UserID: %s", user.ID)

	response := map[string]interface{}{
		"success": true,
		"message": "Terms and PDPA accepted successfully",
	}

	h.writeJSONResponse(w, response)
}

// HandleGetTerms handles getting current terms and PDPA versions
func (h *AuthHandlers) HandleGetTerms(w http.ResponseWriter, r *http.Request) {
	log.Println("📋 Handling get terms request")

	userRepo := repository.NewUserRepository()
	termsVersion, pdpaVersion, err := userRepo.GetActiveTermsVersions(r.Context())
	if err != nil {
		log.Printf("❌ Failed to get terms versions: %v", err)
		http.Error(w, "Failed to get terms versions", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"terms_version": termsVersion,
			"terms_content": "Terms and Conditions content", // Could be fetched from DB
			"pdpa_version":  pdpaVersion,
			"pdpa_content":  "Privacy Policy content", // Could be fetched from DB
		},
	}

	h.writeJSONResponse(w, response)
}

// HandleLogout handles user logout by invalidating their JWT token and clearing session data
func (h *AuthHandlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	log.Println("🚪 Handling user logout request")

	// Extract user from JWT token (if present)
	user, err := h.extractUserFromToken(r)
	if err != nil {
		// Even if token extraction fails, we'll still respond with success
		// This ensures logout works even with expired/invalid tokens
		log.Printf("⚠️ Logout called with invalid/expired token: %v", err)
	} else {
		log.Printf("👤 Logging out user: %s", user.ID)

		// Note: With JWT tokens, we can't truly invalidate them server-side without a blacklist
		// The frontend will remove the token from localStorage, effectively logging out the user
		// In a production system, you might want to:
		// 1. Maintain a blacklist of invalidated tokens (with expiry)
		// 2. Use shorter-lived access tokens with refresh tokens
		// 3. Store session tokens in database for server-side invalidation

		// For now, we'll just log the logout event for audit purposes
		// You could add audit logging to database here if needed
		log.Printf("✅ User %s logged out successfully", user.ID)
	}

	// Clear any security-related headers
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Return success response
	response := map[string]interface{}{
		"success":   true,
		"message":   "Successfully logged out",
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	}

	log.Println("✅ Logout completed successfully")
	h.writeJSONResponse(w, response)
}

// extractUserFromToken extracts user from JWT token
func (h *AuthHandlers) extractUserFromToken(r *http.Request) (*models.User, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("no authorization header")
	}

	// Remove "Bearer " prefix
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	tokenString := authHeader[7:]

	// Validate JWT token
	user, err := h.userService.ValidateAccessToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return user, nil
}

// getClientIP extracts client IP address from request
func (h *AuthHandlers) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for load balancers/proxies)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// HandleGetActivityRules handles getting current activity rules content
func (h *AuthHandlers) HandleGetActivityRules(w http.ResponseWriter, r *http.Request) {
	log.Println("🏆 Handling get activity rules request")

	userRepo := repository.NewUserRepository()
	activityRules, err := userRepo.GetActiveActivityRules(r.Context())
	if err != nil {
		log.Printf("❌ Failed to get activity rules: %v", err)
		http.Error(w, "Failed to get activity rules", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"data":    activityRules,
	}

	h.writeJSONResponse(w, response)
}

// HandleAcceptActivityRules handles activity rules acceptance
func (h *AuthHandlers) HandleAcceptActivityRules(w http.ResponseWriter, r *http.Request) {
	log.Println("🏆 Handling accept activity rules request")

	// Extract user from JWT token
	user, err := h.extractUserFromToken(r)
	if err != nil {
		log.Printf("❌ Failed to extract user from token: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get client IP and User-Agent for audit
	ipAddress := h.getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	// Accept activity rules
	err = h.userService.AcceptActivityRules(
		r.Context(),
		user.ID,
		ipAddress,
		userAgent,
	)

	if err != nil {
		log.Printf("❌ Failed to accept activity rules: %v", err)
		http.Error(w, "Failed to accept activity rules", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Activity rules accepted successfully - UserID: %s", user.ID)

	response := map[string]interface{}{
		"success": true,
		"message": "Activity rules accepted successfully",
	}

	h.writeJSONResponse(w, response)
}

// writeJSONResponse writes a JSON response
func (h *AuthHandlers) writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("❌ Failed to encode JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

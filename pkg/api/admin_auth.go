package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gamemini/youtube/pkg/middleware"
	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/repository"
	"github.com/golang-jwt/jwt/v5"
)

// AdminLoginRequest represents admin login request
type AdminLoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password,omitempty"` // Optional for OAuth-based admin auth
}

// AdminLoginResponse represents admin login response
type AdminLoginResponse struct {
	Success      bool                     `json:"success"`
	Message      string                   `json:"message,omitempty"`
	AccessToken  string                   `json:"access_token,omitempty"`
	ExpiresAt    time.Time                `json:"expires_at,omitempty"`
	AdminInfo    *middleware.AdminClaims  `json:"admin_info,omitempty"`
	Error        string                   `json:"error,omitempty"`
}

// HandleAdminLogin handles admin authentication and JWT token generation
func HandleAdminLogin(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔐 [ADMIN-LOGIN] Starting admin login from %s", r.RemoteAddr)
	
	var request AdminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("❌ [ADMIN-LOGIN] Invalid request body: %v", err)
		sendAdminLoginErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.Email == "" {
		sendAdminLoginErrorResponse(w, "Email is required", http.StatusBadRequest)
		return
	}

	// Validate admin email
	if !isValidAdminEmailForLogin(request.Email) {
		log.Printf("❌ [ADMIN-LOGIN] Invalid admin email: %s", request.Email)
		sendAdminLoginErrorResponse(w, "Email not authorized for admin access", http.StatusForbidden)
		return
	}

	log.Printf("✅ [ADMIN-LOGIN] Valid admin email: %s", request.Email)

	// For this implementation, we'll generate admin tokens for valid admin emails
	// In production, you would verify credentials against your admin user store
	adminUser, err := getOrCreateAdminUser(request.Email)
	if err != nil {
		log.Printf("❌ [ADMIN-LOGIN] Failed to get/create admin user: %v", err)
		sendAdminLoginErrorResponse(w, "Failed to authenticate admin", http.StatusInternalServerError)
		return
	}

	// Generate admin JWT token
	adminToken, expiresAt, err := generateAdminJWTToken(adminUser)
	if err != nil {
		log.Printf("❌ [ADMIN-LOGIN] Failed to generate admin token: %v", err)
		sendAdminLoginErrorResponse(w, "Failed to generate admin token", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ [ADMIN-LOGIN] Admin token generated successfully for %s", request.Email)

	// Create response
	response := AdminLoginResponse{
		Success:     true,
		Message:     "Admin authenticated successfully",
		AccessToken: adminToken,
		ExpiresAt:   expiresAt,
		AdminInfo: &middleware.AdminClaims{
			UserID:  adminUser.ID,
			Email:   adminUser.Email,
			Roles:   []string{"admin"},
			IsAdmin: true,
			GoogleID: adminUser.GoogleID,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// generateAdminJWTToken creates a JWT token for admin users
func generateAdminJWTToken(adminUser *models.User) (string, time.Time, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-development-secret-change-in-production"
	}

	// Set expiration to 24 hours for admin tokens
	expiresAt := time.Now().Add(24 * time.Hour)

	// Create admin-specific JWT claims
	claims := &middleware.AdminClaims{
		UserID:   adminUser.ID,
		Email:    adminUser.Email,
		Roles:    []string{"admin"},
		IsAdmin:  true,
		GoogleID: adminUser.GoogleID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "youtube-activity-platform",
			Subject:   adminUser.ID,
			Audience:  []string{"youtube-activity-admin"},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign admin token: %w", err)
	}

	log.Printf("🔑 [ADMIN-TOKEN] Generated admin JWT token (expires: %s)", expiresAt.Format(time.RFC3339))
	return tokenString, expiresAt, nil
}

// getOrCreateAdminUser gets or creates an admin user record
func getOrCreateAdminUser(email string) (*models.User, error) {
	// Initialize services
	userRepo := repository.NewUserRepository()
	
	// Try to find existing user by email
	// Note: This is a simplified approach - in production you'd have a separate admin user table
	ctx := context.Background()
	
	// Generate a consistent Google ID for admin users based on email
	googleID := "admin-" + strings.ReplaceAll(email, "@", "-at-")
	googleID = strings.ReplaceAll(googleID, ".", "-dot-")
	
	// Use the existing upsert function to create/update the admin user
	user, _, err := userRepo.UpsertUserFromOAuth(ctx, googleID, email)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert admin user: %w", err)
	}
	
	log.Printf("✅ [ADMIN-USER] Admin user retrieved/created: %s", email)
	return user, nil
}

// isValidAdminEmailForLogin checks if email is valid for admin login
func isValidAdminEmailForLogin(email string) bool {
	return isValidAdminEmail(email)
}

// sendAdminLoginErrorResponse sends error response for admin login
func sendAdminLoginErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := AdminLoginResponse{
		Success: false,
		Message: message,
		Error:   message,
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
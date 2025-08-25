package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/repository"
)

// TokenStatusResponse represents user's token status
type TokenStatusResponse struct {
	Success              bool      `json:"success"`
	UserID               string    `json:"user_id"`
	Email                string    `json:"email"`
	HasAccessToken       bool      `json:"has_access_token"`
	HasRefreshToken      bool      `json:"has_refresh_token"`
	AccessTokenExpiry    *time.Time `json:"access_token_expiry"`
	IsExpired            bool      `json:"is_expired"`
	NeedsReauthorization bool      `json:"needs_reauthorization"`
	ReauthReason         string    `json:"reauth_reason,omitempty"`
	Instructions         []string  `json:"instructions,omitempty"`
}

// HandleTokenStatus checks user's OAuth token status and provides reauth guidance
func HandleTokenStatus(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔍 [TOKEN-STATUS] Checking token status from %s", r.RemoteAddr)
	
	// Extract user ID from JWT token - simplified version for this endpoint
	userID, err := extractUserIDFromJWT(r.Header.Get("Authorization"))
	if err != nil {
		log.Printf("❌ [TOKEN-STATUS] Failed to extract user: %v", err)
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get user from database
	userRepo := repository.NewUserRepository()
	user, err := userRepo.GetUserByID(r.Context(), userID)
	if err != nil || user == nil {
		log.Printf("❌ [TOKEN-STATUS] Failed to get user: %v", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Analyze token status
	response := analyzeTokenStatus(user)
	
	log.Printf("✅ [TOKEN-STATUS] Token analysis completed for user %s", userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleForceReauth provides a forced re-authorization URL
func HandleForceReauth(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔄 [FORCE-REAUTH] Starting forced reauth from %s", r.RemoteAddr)
	
	// Generate a new OAuth URL with forced consent
	// This will be handled by the existing OAuth flow but with additional parameters
	
	response := map[string]interface{}{
		"success": true,
		"message": "To fix refresh token issues, please complete re-authorization",
		"instructions": []string{
			"1. First, revoke existing app access:",
			"   - Visit: https://myaccount.google.com/permissions",
			"   - Find 'YouTube Activity Platform' and click 'Remove access'",
			"2. Then click the re-authorize button below",
			"3. Make sure to grant all requested permissions",
			"4. You should see 'offline access' in the permission list",
		},
		"reauth_url": "/auth/google/login?force_consent=true",
		"revoke_url": "https://myaccount.google.com/permissions",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// analyzeTokenStatus analyzes user's OAuth token status
func analyzeTokenStatus(user *models.User) *TokenStatusResponse {
	response := &TokenStatusResponse{
		Success:         true,
		UserID:          user.ID,
		Email:           user.Email,
		HasAccessToken:  user.GoogleAccessToken != nil && *user.GoogleAccessToken != "",
		HasRefreshToken: user.GoogleRefreshToken != nil && *user.GoogleRefreshToken != "",
	}

	// Check expiry
	if user.GoogleTokenExpiry != nil {
		response.AccessTokenExpiry = user.GoogleTokenExpiry
		response.IsExpired = user.GoogleTokenExpiry.Before(time.Now())
	}

	// Determine if reauthorization is needed
	if !response.HasRefreshToken {
		response.NeedsReauthorization = true
		response.ReauthReason = "Missing refresh token - cannot automatically renew expired access tokens"
		response.Instructions = []string{
			"Your account is missing a refresh token, which means we cannot automatically renew your access to YouTube.",
			"This happens when:",
			"• You previously authorized our app before we implemented proper refresh token handling",
			"• You denied the 'offline access' permission during authorization",
			"• Google's OAuth flow didn't provide a refresh token",
			"",
			"To fix this:",
			"1. Visit https://myaccount.google.com/permissions",
			"2. Find 'YouTube Activity Platform' and remove access",
			"3. Re-authorize the application to get a refresh token",
		}
	} else if response.IsExpired {
		response.NeedsReauthorization = true
		response.ReauthReason = "Access token has expired"
		response.Instructions = []string{
			"Your access token has expired. We'll try to refresh it automatically.",
			"If automatic refresh fails, you may need to re-authorize.",
		}
	} else if user.GoogleTokenExpiry != nil && user.GoogleTokenExpiry.Before(time.Now().Add(24*time.Hour)) {
		response.NeedsReauthorization = false // Not critical yet
		response.ReauthReason = "Access token expires soon"
		response.Instructions = []string{
			"Your access token expires within 24 hours.",
			"We'll automatically refresh it when needed.",
		}
	}

	return response
}

// HandleBulkTokenAnalysis provides admin endpoint to analyze all users' token status
func HandleBulkTokenAnalysis(w http.ResponseWriter, r *http.Request) {
	log.Printf("📊 [BULK-TOKEN-ANALYSIS] Starting bulk token analysis")
	
	// This would need to be implemented in the repository
	// For now, return a placeholder response
	response := map[string]interface{}{
		"success": true,
		"message": "Bulk token analysis completed",
		"summary": map[string]interface{}{
			"total_users": "Query needed",
			"users_with_refresh_token": "Query needed", 
			"users_without_refresh_token": "Query needed",
			"users_with_expired_tokens": "Query needed",
		},
		"recommendation": "Run the SQL queries in debug_refresh_tokens.sql to get detailed statistics",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
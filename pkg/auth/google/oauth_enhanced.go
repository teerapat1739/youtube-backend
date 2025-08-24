package google

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/repository"
	"github.com/gamemini/youtube/pkg/services"
	"golang.org/x/oauth2"
)

// EnhancedOAuthHandler provides enhanced OAuth handling with proper user creation
type EnhancedOAuthHandler struct {
	userService *services.UserService
	oauthConfig *oauth2.Config
}

// NewEnhancedOAuthHandler creates a new enhanced OAuth handler
func NewEnhancedOAuthHandler(jwtSecret string) *EnhancedOAuthHandler {
	userRepo := repository.NewUserRepository()
	userService := services.NewUserService(userRepo, jwtSecret)

	return &EnhancedOAuthHandler{
		userService: userService,
		oauthConfig: GetOAuthConfig(),
	}
}

// HandleLogin redirects to Google OAuth
func (h *EnhancedOAuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	log.Println("🔑 Starting enhanced Google OAuth login flow")

	// Generate state for CSRF protection
	state := generateSecureState()

	// Store state in session/cookie for validation (simplified for now)
	// In production, you'd want to store this securely

	url := h.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	log.Printf("🔄 Redirecting to Google OAuth: %s", url)

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// HandleCallback processes the OAuth callback and creates/updates user
func (h *EnhancedOAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	log.Println("🔄 Processing enhanced Google OAuth callback")

	ctx := r.Context()
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		log.Println("❌ No authorization code in callback")
		http.Redirect(w, r, frontendURL+"/?error=no_code", http.StatusTemporaryRedirect)
		return
	}

	// Exchange code for token
	token, err := h.oauthConfig.Exchange(ctx, code)
	if err != nil {
		log.Printf("❌ Failed to exchange code for token: %v", err)
		http.Redirect(w, r, frontendURL+"/?error=token_exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	// Get user info from Google
	userInfo, err := getUserInfo(token.AccessToken)
	if err != nil {
		log.Printf("❌ Failed to get user info from Google: %v", err)
		http.Redirect(w, r, frontendURL+"/?error=user_info_failed", http.StatusTemporaryRedirect)
		return
	}

	log.Printf("✅ Retrieved user info from Google - ID: %s, Email: %s", userInfo.ID, userInfo.Email)

	// Create OAuth token data
	oauthTokenData := &models.OAuthTokenData{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
		TokenType:    token.TokenType,
	}

	// Create or update user in our database with OAuth tokens
	authData, err := h.userService.CreateOrUpdateUserFromOAuthWithTokens(ctx, userInfo.ID, userInfo.Email, oauthTokenData)
	if err != nil {
		log.Printf("❌ Failed to create/update user with OAuth tokens: %v", err)
		http.Redirect(w, r, frontendURL+"/?error=user_creation_failed", http.StatusTemporaryRedirect)
		return
	}

	log.Printf("✅ User created/updated successfully - UserID: %s", authData.User.ID)

	// Determine the appropriate redirect route based on user status
	var targetRoute string
	if authData.User.ProfileCompleted && authData.User.TermsAccepted && authData.User.PDPAAccepted {
		// User has completed onboarding, redirect to home
		targetRoute = "/home"
	} else {
		// User needs to complete onboarding, redirect to appropriate step
		if !authData.User.ProfileCompleted {
			targetRoute = "/onboarding?step=profile"
		} else if !authData.User.TermsAccepted || !authData.User.PDPAAccepted {
			targetRoute = "/onboarding?step=terms"
		} else {
			targetRoute = "/onboarding?step=subscription"
		}
	}

	// Redirect to frontend with both JWT token and Google access token
	// JWT token for backend authentication, Google token for YouTube API
	redirectURL := fmt.Sprintf("%s%s?token=%s&google_token=%s&user_id=%s",
		frontendURL,
		targetRoute,
		authData.AccessToken,
		token.AccessToken,
		authData.User.ID)

	log.Printf("🔄 Redirecting to frontend target route: %s", redirectURL)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// HandleUserInfo provides user information using JWT token
func (h *EnhancedOAuthHandler) HandleUserInfo(w http.ResponseWriter, r *http.Request) {
	log.Println("👤 Handling user info request")

	// Extract and validate JWT token
	user, err := h.extractUserFromJWT(r)
	if err != nil {
		log.Printf("❌ Failed to extract user from JWT: %v", err)
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	log.Printf("✅ User info retrieved - UserID: %s, Email: %s", user.ID, user.Email)

	// Return user information
	response := map[string]interface{}{
		"id":                user.GoogleID,
		"email":             user.Email,
		"verified_email":    true,
		"name":              getUserDisplayName(user),
		"picture":           "", // Could be added later
		"user_id":           user.ID,
		"profile_completed": user.ProfileCompleted,
		"terms_accepted":    user.TermsAccepted,
		"pdpa_accepted":     user.PDPAAccepted,
	}

	writeJSONResponse(w, response)
}

// HandleCreateInitialProfile creates an initial user profile (called after OAuth)
func (h *EnhancedOAuthHandler) HandleCreateInitialProfile(w http.ResponseWriter, r *http.Request) {
	log.Println("👤 Handling create initial profile request")

	// Extract and validate JWT token
	user, err := h.extractUserFromJWT(r)
	if err != nil {
		log.Printf("❌ Failed to extract user from JWT: %v", err)
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	log.Printf("✅ User found for profile creation - UserID: %s", user.ID)

	// Return user data (user already exists from OAuth callback)
	response := map[string]interface{}{
		"success": true,
		"message": "User record verified",
		"data": map[string]interface{}{
			"user": map[string]interface{}{
				"id":                user.ID,
				"google_id":         user.GoogleID,
				"email":             user.Email,
				"first_name":        user.FirstName,
				"last_name":         user.LastName,
				"national_id":       user.NationalID,
				"phone":             user.Phone,
				"terms_accepted":    user.TermsAccepted,
				"pdpa_accepted":     user.PDPAAccepted,
				"profile_completed": user.ProfileCompleted,
				"created_at":        user.CreatedAt.Format(time.RFC3339),
				"updated_at":        user.UpdatedAt.Format(time.RFC3339),
			},
			"created": false, // User was created in OAuth callback
		},
	}

	writeJSONResponse(w, response)
}

// extractUserFromJWT extracts user from JWT token
func (h *EnhancedOAuthHandler) extractUserFromJWT(r *http.Request) (*models.User, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("no authorization header")
	}

	// Remove "Bearer " prefix
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	tokenString := authHeader[7:]

	// Validate JWT token and get user
	user, err := h.userService.ValidateAccessToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return user, nil
}

// generateSecureState generates a secure state parameter for CSRF protection
func generateSecureState() string {
	// In production, generate a cryptographically secure random state
	return fmt.Sprintf("state_%d", time.Now().UnixNano())
}

// getUserDisplayName returns a display name for the user
func getUserDisplayName(user *models.User) string {
	if user.FirstName != nil && user.LastName != nil {
		return fmt.Sprintf("%s %s", *user.FirstName, *user.LastName)
	}
	if user.FirstName != nil {
		return *user.FirstName
	}
	// Fallback to email username
	return user.Email
}

// writeJSONResponse writes a JSON response
func writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")

	// Use proper JSON encoding
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("❌ Failed to encode JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

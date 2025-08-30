package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gamemini/youtube/pkg/config"
	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/repository"
	"github.com/golang-jwt/jwt/v5"
)

// UserService handles user-related business logic
type UserService struct {
	userRepo *repository.UserRepository
	jwtSecret string
}

// NewUserService creates a new user service
func NewUserService(userRepo *repository.UserRepository, jwtSecret string) *UserService {
	return &UserService{
		userRepo: userRepo,
		jwtSecret: jwtSecret,
	}
}

// UserAuthData represents authentication data for a user
type UserAuthData struct {
	User         *models.User `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time    `json:"expires_at"`
}

// CreateOrUpdateUserFromOAuth creates or updates a user from OAuth data with proper upsert pattern
func (s *UserService) CreateOrUpdateUserFromOAuth(ctx context.Context, googleID, email string) (*UserAuthData, error) {
	log.Printf("🔐 Creating/updating user from OAuth - GoogleID: %s, Email: %s", googleID, email)

	// Use atomic upsert operation to handle race conditions
	user, isNewUser, err := s.userRepo.UpsertUserFromOAuth(ctx, googleID, email)
	if err != nil {
		log.Printf("❌ Failed to upsert user: %v", err)
		return nil, fmt.Errorf("failed to create/update user: %w", err)
	}

	if isNewUser {
		log.Printf("✅ New user created - ID: %s, Email: %s", user.ID, user.Email)
	} else {
		log.Printf("✅ Existing user updated - ID: %s, Email: %s", user.ID, user.Email)
	}

	// Generate JWT tokens
	accessToken, expiresAt, err := s.GenerateAccessToken(user)
	if err != nil {
		log.Printf("❌ Failed to generate access token: %v", err)
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Create session for tracking
	session := &models.UserSession{
		UserID:       user.ID,
		SessionToken: accessToken,
		ExpiresAt:    expiresAt,
	}

	if err := s.userRepo.CreateUserSession(ctx, session); err != nil {
		log.Printf("❌ Failed to create user session: %v", err)
		// Check if it's the varchar(255) issue specifically
		if strings.Contains(err.Error(), "value too long for type character varying(255)") {
			log.Printf("🔧 Detected session token length issue - run migration 008_fix_session_token_length.sql")
			return nil, fmt.Errorf("session creation failed due to token length limitation - database migration required: %w", err)
		}
		// For other session creation issues, log but continue (temporary degraded service)
		log.Printf("⚠️  Session creation failed but continuing with authentication - sessions may not be properly tracked")
	} else {
		log.Printf("✅ User session created successfully - SessionID: %s", session.ID)
	}

	return &UserAuthData{
		User:        user,
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
	}, nil
}

// CreateOrUpdateUserFromOAuthWithTokens creates or updates a user from OAuth data and stores OAuth tokens
func (s *UserService) CreateOrUpdateUserFromOAuthWithTokens(ctx context.Context, googleID, email string, oauthTokenData *models.OAuthTokenData) (*UserAuthData, error) {
	log.Printf("🔐 Creating/updating user from OAuth with tokens - GoogleID: %s, Email: %s", googleID, email)

	// Use atomic upsert operation to handle race conditions and store OAuth tokens
	user, isNewUser, err := s.userRepo.UpsertUserFromOAuthWithTokens(ctx, googleID, email, oauthTokenData)
	if err != nil {
		log.Printf("❌ Failed to upsert user with tokens: %v", err)
		return nil, fmt.Errorf("failed to create/update user with tokens: %w", err)
	}

	if isNewUser {
		log.Printf("✅ New user created with OAuth tokens - ID: %s, Email: %s", user.ID, user.Email)
	} else {
		log.Printf("✅ Existing user updated with OAuth tokens - ID: %s, Email: %s", user.ID, user.Email)
	}

	// Generate JWT tokens
	accessToken, expiresAt, err := s.GenerateAccessToken(user)
	if err != nil {
		log.Printf("❌ Failed to generate access token: %v", err)
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Create session for tracking
	session := &models.UserSession{
		UserID:       user.ID,
		SessionToken: accessToken,
		ExpiresAt:    expiresAt,
	}

	if err := s.userRepo.CreateUserSession(ctx, session); err != nil {
		log.Printf("❌ Failed to create user session: %v", err)
		// Check if it's the varchar(255) issue specifically
		if strings.Contains(err.Error(), "value too long for type character varying(255)") {
			log.Printf("🔧 Detected session token length issue - run migration 008_fix_session_token_length.sql")
			return nil, fmt.Errorf("session creation failed due to token length limitation - database migration required: %w", err)
		}
		// For other session creation issues, log but continue (temporary degraded service)
		log.Printf("⚠️  Session creation failed but continuing with authentication - sessions may not be properly tracked")
	} else {
		log.Printf("✅ User session created successfully - SessionID: %s", session.ID)
	}

	return &UserAuthData{
		User:        user,
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
	}, nil
}

// UpdateUserProfile updates user profile information with validation
func (s *UserService) UpdateUserProfile(ctx context.Context, userID string, updates *models.UpdateUserProfileRequest) (*models.User, error) {
	log.Printf("📝 Updating user profile - UserID: %s", userID)

	// Validate the update request
	if err := s.validateUpdateRequest(updates); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}


	// Update user profile atomically
	err := s.userRepo.UpdateUserProfileAtomic(ctx, userID, updates)
	if err != nil {
		log.Printf("❌ Failed to update user profile: %v", err)
		return nil, fmt.Errorf("failed to update user profile: %w", err)
	}

	// Fetch updated user
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		log.Printf("❌ Failed to fetch updated user: %v", err)
		return nil, fmt.Errorf("failed to fetch updated user: %w", err)
	}

	log.Printf("✅ User profile updated successfully - UserID: %s", userID)
	return user, nil
}

// UpdateUserProfilePersonalInfoOnly updates only personal information fields without requiring terms acceptance
func (s *UserService) UpdateUserProfilePersonalInfoOnly(ctx context.Context, userID string, updates *models.UpdateUserProfileRequest) (*models.User, error) {
	log.Printf("📝 Updating personal info only - UserID: %s", userID)

	// Basic validation for personal info fields only
	if updates.FirstName == "" {
		return nil, fmt.Errorf("first name is required")
	}
	if updates.LastName == "" {
		return nil, fmt.Errorf("last name is required")
	}

	// Optional validation for phone if provided
	if updates.Phone != "" && len(updates.Phone) != 10 {
		return nil, fmt.Errorf("phone number must be 10 digits")
	}


	// Update only personal information fields
	err := s.userRepo.UpdateUserPersonalInfoOnly(ctx, userID, updates.FirstName, updates.LastName, updates.Phone)
	if err != nil {
		log.Printf("❌ Failed to update personal info: %v", err)
		return nil, fmt.Errorf("failed to update personal info: %w", err)
	}

	// Fetch updated user
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		log.Printf("❌ Failed to fetch updated user: %v", err)
		return nil, fmt.Errorf("failed to fetch updated user: %w", err)
	}

	log.Printf("✅ Personal info updated successfully - UserID: %s", userID)
	return user, nil
}

// AcceptTerms records terms and PDPA acceptance with audit trail
func (s *UserService) AcceptTerms(ctx context.Context, userID, termsVersion, pdpaVersion string, acceptTerms, acceptPDPA bool, ipAddress, userAgent string) error {
	log.Printf("📋 Recording terms acceptance - UserID: %s, Terms: %s, PDPA: %s", userID, termsVersion, pdpaVersion)

	// Update user record with terms acceptance
	err := s.userRepo.UpdateTermsAcceptance(ctx, userID, termsVersion, pdpaVersion, acceptTerms, acceptPDPA)
	if err != nil {
		log.Printf("❌ Failed to update terms acceptance: %v", err)
		return fmt.Errorf("failed to update terms acceptance: %w", err)
	}

	// Create audit record
	acceptance := &models.UserTermsAcceptance{
		UserID:       userID,
		TermsVersion: termsVersion,
		PDPAVersion:  pdpaVersion,
		AcceptedAt:   time.Now(),
	}
	
	if ipAddress != "" {
		acceptance.IPAddress = &ipAddress
	}
	if userAgent != "" {
		acceptance.UserAgent = &userAgent
	}

	if err := s.userRepo.CreateTermsAcceptanceRecord(ctx, acceptance); err != nil {
		log.Printf("⚠️  Failed to create audit record (continuing): %v", err)
		// Don't fail the request if audit record creation fails
	}

	log.Printf("✅ Terms acceptance recorded successfully - UserID: %s", userID)
	return nil
}

// GenerateAccessToken generates a JWT access token for the user
func (s *UserService) GenerateAccessToken(user *models.User) (string, time.Time, error) {
	// Extended expiration to 7 days for better UX (reduced auth interruptions)
	// Frontend should handle token refresh before expiration
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days expiration

	now := time.Now()
	claims := jwt.MapClaims{
		"user_id":   user.ID,
		"google_id": user.GoogleID,
		"email":     user.Email,
		"exp":       expiresAt.Unix(),
		"iat":       now.Unix(),
		"nbf":       now.Unix(), // Not before - prevents token use before issued
		"iss":       "youtube-activity-platform",
		"aud":       "youtube-activity-frontend", // Audience - specific to our frontend
		"sub":       user.ID, // Subject - standard JWT claim for user identity
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	log.Printf("🔑 Generated JWT token for user %s (expires: %s)", user.ID, expiresAt.Format(time.RFC3339))
	return tokenString, expiresAt, nil
}

// ValidateAccessToken validates and parses a JWT access token
func (s *UserService) ValidateAccessToken(tokenString string) (*models.User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		// Provide more specific error messages for common JWT issues
		// Check for specific error types in jwt/v5
		errorMsg := err.Error()
		switch {
		case strings.Contains(errorMsg, "token is malformed"):
			return nil, fmt.Errorf("token is malformed")
		case strings.Contains(errorMsg, "token has expired") || strings.Contains(errorMsg, "token is expired"):
			return nil, fmt.Errorf("token has expired")
		case strings.Contains(errorMsg, "token used before valid") || strings.Contains(errorMsg, "not valid yet"):
			return nil, fmt.Errorf("token is not valid yet")
		case strings.Contains(errorMsg, "issuer") || strings.Contains(errorMsg, "audience"):
			return nil, fmt.Errorf("token has invalid claims: issuer or audience mismatch")
		case strings.Contains(errorMsg, "signature is invalid"):
			return nil, fmt.Errorf("token has invalid signature")
		default:
			return nil, fmt.Errorf("invalid token: %w", err)
		}
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Validate required claims
		userID := fmt.Sprintf("%v", claims["user_id"])
		if userID == "" {
			return nil, fmt.Errorf("invalid token: missing user_id claim")
		}

		// Validate issuer if present
		if iss, ok := claims["iss"].(string); ok {
			if iss != "youtube-activity-platform" {
				return nil, fmt.Errorf("invalid token: invalid issuer")
			}
		}

		// Validate audience if present
		if aud, ok := claims["aud"].(string); ok {
			if aud != "youtube-activity-frontend" {
				return nil, fmt.Errorf("invalid token: invalid audience")
			}
		}

		// Fetch current user data from database
		user, err := s.userRepo.GetUserByID(context.Background(), userID)
		if err != nil {
			return nil, fmt.Errorf("user not found: %w", err)
		}

		log.Printf("✅ Token validated successfully for user %s", userID)
		return user, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// GenerateRefreshToken generates a secure refresh token
func (s *UserService) GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// validateUpdateRequest validates user profile update request
func (s *UserService) validateUpdateRequest(updates *models.UpdateUserProfileRequest) error {
	if updates.FirstName == "" {
		return fmt.Errorf("first name is required")
	}
	if updates.LastName == "" {
		return fmt.Errorf("last name is required")
	}
	if updates.Phone == "" {
		return fmt.Errorf("phone number is required")
	}
	if len(updates.Phone) != 10 {
		return fmt.Errorf("phone number must be 10 digits")
	}

	return nil
}

// GetUserProfileWithVoteStatus retrieves user profile with voting status
func (s *UserService) GetUserProfileWithVoteStatus(ctx context.Context, userID string, activityID string) (*models.UserProfileResponse, error) {
	log.Printf("👤 Getting user profile with vote status - UserID: %s, ActivityID: %s", userID, activityID)

	// Get user profile
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return &models.UserProfileResponse{
			Exists: false,
		}, nil
	}

	response := &models.UserProfileResponse{
		Exists: true,
		User:   user,
	}

	// Check vote status if activity ID provided
	if activityID != "" && activityID != "active" {
		vote, err := s.userRepo.GetUserVote(ctx, userID, activityID)
		if err != nil {
			log.Printf("⚠️  Error getting vote status: %v", err)
		} else if vote != nil {
			response.HasVoted = true
			response.VotedTeamID = &vote.TeamID
		}
	}

	log.Printf("✅ User profile retrieved - UserID: %s, HasVoted: %t", userID, response.HasVoted)
	return response, nil
}

// CleanupExpiredSessions removes expired user sessions and returns count deleted
func (s *UserService) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	log.Println("🧹 Cleaning up expired sessions...")
	
	deletedCount, err := s.userRepo.CleanExpiredSessions(ctx)
	if err != nil {
		log.Printf("❌ Failed to cleanup expired sessions: %v", err)
		return 0, fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	
	if deletedCount > 0 {
		log.Printf("✅ Cleaned up %d expired sessions", deletedCount)
	} else {
		log.Println("✅ No expired sessions to clean up")
	}
	
	return deletedCount, nil
}

// StartSessionCleanupScheduler starts a background goroutine that periodically cleans expired sessions
func (s *UserService) StartSessionCleanupScheduler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Hour // Default to 1 hour if invalid interval
	}
	
	log.Printf("🚀 Starting session cleanup scheduler (interval: %v)", interval)
	
	// Initial cleanup on startup
	go func() {
		if count, err := s.CleanupExpiredSessions(ctx); err != nil {
			log.Printf("⚠️  Initial session cleanup failed: %v", err)
		} else if count > 0 {
			log.Printf("🧹 Initial cleanup removed %d expired sessions", count)
		}
	}()
	
	// Periodic cleanup
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("🛑 Session cleanup scheduler stopped due to context cancellation")
				return
			case <-ticker.C:
				if count, err := s.CleanupExpiredSessions(context.Background()); err != nil {
					log.Printf("⚠️  Scheduled session cleanup failed: %v", err)
				} else if count > 0 {
					log.Printf("🧹 Scheduled cleanup removed %d expired sessions", count)
				}
			}
		}
	}()
}

// GetUserOAuthTokens retrieves stored OAuth tokens for a user
func (s *UserService) GetUserOAuthTokens(ctx context.Context, userID string) (*models.OAuthTokenData, error) {
	log.Printf("🔑 Getting OAuth tokens for user - UserID: %s", userID)
	
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		log.Printf("❌ Failed to get user: %v", err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	if user.GoogleAccessToken == nil || user.GoogleRefreshToken == nil {
		log.Printf("❌ No OAuth tokens stored for user - UserID: %s", userID)
		return nil, fmt.Errorf("no OAuth tokens stored for user")
	}
	
	tokenData := &models.OAuthTokenData{
		AccessToken:  *user.GoogleAccessToken,
		RefreshToken: *user.GoogleRefreshToken,
		TokenType:    "Bearer",
	}
	
	if user.GoogleTokenExpiry != nil {
		tokenData.Expiry = *user.GoogleTokenExpiry
	}
	
	log.Printf("✅ OAuth tokens retrieved for user - UserID: %s", userID)
	return tokenData, nil
}

// RefreshUserOAuthToken refreshes OAuth tokens for a user using Google's OAuth2 refresh endpoint
func (s *UserService) RefreshUserOAuthToken(ctx context.Context, userID string) (*models.OAuthTokenData, error) {
	log.Printf("🔄 Refreshing OAuth token for user - UserID: %s", userID)
	
	// Get current tokens
	tokenData, err := s.GetUserOAuthTokens(ctx, userID)
	if err != nil {
		log.Printf("❌ Failed to get current tokens: %v", err)
		return nil, fmt.Errorf("failed to get current tokens: %w", err)
	}
	
	if tokenData.RefreshToken == "" {
		log.Printf("❌ No refresh token available for user - UserID: %s", userID)
		return nil, fmt.Errorf("no refresh token available for user")
	}
	
	// Import the Google OAuth config here to avoid circular imports
	// Use similar approach as in google/auth.go
	refreshedTokenData, err := s.refreshTokenWithGoogle(ctx, tokenData.RefreshToken)
	if err != nil {
		log.Printf("❌ Failed to refresh token with Google: %v", err)
		return nil, fmt.Errorf("failed to refresh token with Google: %w", err)
	}
	
	// Keep the original refresh token since Google doesn't always provide a new one
	if refreshedTokenData.RefreshToken == "" {
		refreshedTokenData.RefreshToken = tokenData.RefreshToken
	}
	
	// Update stored tokens
	err = s.userRepo.UpdateUserOAuthTokens(ctx, userID, refreshedTokenData)
	if err != nil {
		log.Printf("❌ Failed to update OAuth tokens: %v", err)
		return nil, fmt.Errorf("failed to update OAuth tokens: %w", err)
	}
	
	log.Printf("✅ OAuth token refreshed for user - UserID: %s", userID)
	return refreshedTokenData, nil
}

// refreshTokenWithGoogle calls Google's token refresh endpoint
func (s *UserService) refreshTokenWithGoogle(ctx context.Context, refreshToken string) (*models.OAuthTokenData, error) {
	log.Printf("🔄 Calling Google token refresh endpoint")
	
	// Get OAuth configuration
	appConfig := config.GetConfig()
	clientID := appConfig.OAuthConfig.ClientID
	clientSecret := appConfig.OAuthConfig.ClientSecret
	
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("missing Google OAuth credentials")
	}
	
	// Prepare token refresh request
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")
	
	// Make HTTP request to Google's token endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	log.Printf("🌐 Making token refresh request to Google")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token with Google: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refresh response: %w", err)
	}
	
	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Google token refresh failed with status %d: %s", resp.StatusCode, string(body))
		
		// Parse error response for better error messages
		var errorResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		
		if json.Unmarshal(body, &errorResp) == nil && errorResp.Error != "" {
			switch errorResp.Error {
			case "invalid_grant":
				return nil, fmt.Errorf("refresh token is invalid or expired, user needs to re-authorize")
			case "invalid_client":
				return nil, fmt.Errorf("invalid OAuth client credentials")
			default:
				return nil, fmt.Errorf("token refresh failed: %s - %s", errorResp.Error, errorResp.ErrorDescription)
			}
		}
		
		return nil, fmt.Errorf("token refresh failed with status %d", resp.StatusCode)
	}
	
	// Parse successful response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"` // Google may rotate refresh token
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope,omitempty"`
	}
	
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}
	
	// Validate required fields
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token in refresh response")
	}
	
	// Calculate expiry time
	newExpiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	
	// Use new refresh token if provided, otherwise keep the original
	newRefreshToken := refreshToken
	if tokenResp.RefreshToken != "" {
		newRefreshToken = tokenResp.RefreshToken
		log.Printf("🔄 Google provided new refresh token")
	}
	
	// Create refreshed token data
	refreshedData := &models.OAuthTokenData{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: newRefreshToken,
		Expiry:       newExpiry,
		TokenType:    tokenResp.TokenType,
	}
	
	log.Printf("✅ Token refreshed successfully with Google - Expires: %s", newExpiry.Format(time.RFC3339))
	return refreshedData, nil
}

// IsOAuthTokenExpired checks if a user's OAuth token is expired
func (s *UserService) IsOAuthTokenExpired(ctx context.Context, userID string) (bool, error) {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return true, fmt.Errorf("failed to get user: %w", err)
	}
	
	if user.GoogleTokenExpiry == nil {
		return true, nil // No expiry means expired
	}
	
	// Token is expired if expiry time is before now (with 5 minute buffer)
	return user.GoogleTokenExpiry.Before(time.Now().Add(5*time.Minute)), nil
}

// AcceptActivityRules handles activity rules acceptance for a user
func (s *UserService) AcceptActivityRules(ctx context.Context, userID, ipAddress, userAgent string) error {
	log.Printf("🏆 Processing activity rules acceptance - UserID: %s", userID)

	// Get current active activity rules version
	currentRules, err := s.userRepo.GetActiveActivityRules(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current activity rules: %w", err)
	}

	// Accept the activity rules
	err = s.userRepo.AcceptActivityRules(ctx, userID, currentRules.Version, ipAddress, userAgent)
	if err != nil {
		return fmt.Errorf("failed to accept activity rules: %w", err)
	}

	log.Printf("✅ Activity rules accepted successfully - UserID: %s, Version: %s", userID, currentRules.Version)
	return nil
}
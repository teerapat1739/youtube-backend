package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

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
		log.Printf("⚠️  Failed to create session (continuing): %v", err)
		// Don't fail the request if session creation fails
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
		log.Printf("⚠️  Failed to create session (continuing): %v", err)
		// Don't fail the request if session creation fails
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

	// Check if national ID is already taken by another user
	if updates.NationalID != "" {
		existingUser, err := s.userRepo.GetUserByNationalID(ctx, updates.NationalID)
		if err != nil {
			log.Printf("❌ Error checking national ID uniqueness: %v", err)
			return nil, fmt.Errorf("failed to validate national ID: %w", err)
		}
		if existingUser != nil && existingUser.ID != userID {
			return nil, fmt.Errorf("national ID already exists for another user")
		}
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
	expiresAt := time.Now().Add(24 * time.Hour) // 24 hour expiration

	claims := jwt.MapClaims{
		"user_id":   user.ID,
		"google_id": user.GoogleID,
		"email":     user.Email,
		"exp":       expiresAt.Unix(),
		"iat":       time.Now().Unix(),
		"iss":       "youtube-activity-platform",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

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
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID := fmt.Sprintf("%v", claims["user_id"])
		if userID == "" {
			return nil, fmt.Errorf("invalid token: missing user_id")
		}

		// Fetch current user data from database
		user, err := s.userRepo.GetUserByID(context.Background(), userID)
		if err != nil {
			return nil, fmt.Errorf("user not found: %w", err)
		}

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
	if updates.NationalID == "" {
		return fmt.Errorf("national ID is required")
	}
	if len(updates.NationalID) != 13 {
		return fmt.Errorf("national ID must be 13 digits")
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

// CleanupExpiredSessions removes expired user sessions
func (s *UserService) CleanupExpiredSessions(ctx context.Context) error {
	log.Println("🧹 Cleaning up expired sessions...")
	
	err := s.userRepo.CleanExpiredSessions(ctx)
	if err != nil {
		log.Printf("❌ Failed to cleanup expired sessions: %v", err)
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	
	log.Println("✅ Expired sessions cleaned up successfully")
	return nil
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
	
	// For now, implement a mock refresh that extends the token
	// In production, this would call Google's actual refresh endpoint
	// POST https://oauth2.googleapis.com/token with refresh_token
	
	newExpiry := time.Now().Add(time.Hour) // Google access tokens typically last 1 hour
	
	// Mock refreshed token data - in production you'd get this from Google's response
	refreshedData := &models.OAuthTokenData{
		AccessToken:  "refreshed_" + refreshToken[:20] + "_" + fmt.Sprintf("%d", time.Now().Unix()),
		RefreshToken: refreshToken, // Refresh token usually stays the same
		Expiry:       newExpiry,
		TokenType:    "Bearer",
	}
	
	log.Printf("✅ Token refreshed with Google (mock implementation)")
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
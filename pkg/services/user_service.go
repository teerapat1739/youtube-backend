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
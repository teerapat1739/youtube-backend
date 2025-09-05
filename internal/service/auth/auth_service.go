package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"be-v2/internal/domain"
	"be-v2/internal/service"
	"be-v2/pkg/errors"
	"be-v2/pkg/logger"
	"github.com/golang-jwt/jwt/v5"
)

// Service implements the AuthService interface
type Service struct {
	clientID   string
	httpClient *http.Client
	logger     *logger.Logger
}

// NewService creates a new auth service
func NewService(clientID string, logger *logger.Logger) service.AuthService {
	return &Service{
		clientID: clientID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// ValidateGoogleToken validates a Google OAuth token and returns user profile
// This function can handle both Google access tokens and Supabase JWT tokens
func (s *Service) ValidateGoogleToken(ctx context.Context, token string) (*domain.UserProfile, error) {
	s.logger.Debug("Validating token")

	// Check if this is a Google access token (starts with "ya29.")
	if isGoogleAccessToken(token) {
		s.logger.Debug("Token identified as Google access token")
		return s.validateGoogleAccessToken(ctx, token)
	}

	// Check if this is a JWT token (has 3 segments separated by dots)
	if isJWTToken(token) {
		s.logger.Debug("Token identified as JWT, trying Supabase validation")
		return s.validateSupabaseJWT(ctx, token)
	}

	// If we can't identify the token format, return an error
	s.logger.Error("Unrecognized token format")
	return nil, errors.NewAuthenticationError("Unrecognized token format")
}

// validateGoogleAccessToken validates a Google OAuth access token
func (s *Service) validateGoogleAccessToken(ctx context.Context, token string) (*domain.UserProfile, error) {
	s.logger.WithField("token_prefix", token[:20]+"...").Debug("Validating Google access token")

	// Use Google's tokeninfo endpoint to validate the access token
	url := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?access_token=%s", token)

	s.logger.WithField("url", url[:60]+"...").Debug("Making tokeninfo request")

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create tokeninfo request")
		return nil, errors.NewInternalError("Failed to create validation request", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.WithError(err).Error("Failed to call Google tokeninfo endpoint")
		return nil, errors.NewAuthenticationError("Failed to validate token")
	}
	defer resp.Body.Close()

	s.logger.WithField("status_code", resp.StatusCode).Debug("Received tokeninfo response")

	if resp.StatusCode == http.StatusUnauthorized {
		s.logger.WithField("status_code", resp.StatusCode).Error("Google access token is invalid or expired")
		return nil, errors.NewAuthenticationError("Invalid or expired Google token")
	}

	if resp.StatusCode != http.StatusOK {
		// Read response body for more detailed error information
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			s.logger.WithError(err).WithField("status_code", resp.StatusCode).Error("Failed to read error response from Google tokeninfo")
		} else {
			s.logger.WithField("status_code", resp.StatusCode).WithField("response_body", string(body)).Error("Google tokeninfo returned error")
		}
		return nil, errors.NewAuthenticationError("Token validation failed")
	}

	var tokenInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		s.logger.WithError(err).Error("Failed to decode tokeninfo response")
		return nil, errors.NewInternalError("Failed to decode token information", err)
	}

	s.logger.WithField("token_info", tokenInfo).Debug("Decoded tokeninfo response")

	// Verify the audience matches our client ID (if present in response)
	// Note: Google access tokens may not always include 'aud' field, unlike ID tokens
	if aud, ok := tokenInfo["aud"].(string); ok && aud != "" {
		if aud != s.clientID {
			s.logger.WithField("expected_audience", s.clientID).WithField("actual_audience", aud).Error("Token audience mismatch")
			return nil, errors.NewAuthenticationError("Token not intended for this application")
		}
		s.logger.WithField("audience_verification", "passed").Debug("Audience validation successful")
	} else {
		// Access tokens may not have audience, log for debugging but don't fail
		s.logger.Debug("No audience field in token response (normal for access tokens)")
	}

	// Extract user information from tokeninfo response
	profile := &domain.UserProfile{
		Sub:           getStringValue(tokenInfo, "sub"),
		Email:         getStringValue(tokenInfo, "email"),
		EmailVerified: getBoolValue(tokenInfo, "email_verified"),
		Picture:       getStringValue(tokenInfo, "picture"),
	}

	// If sub is not present, try to use email as identifier
	if profile.Sub == "" && profile.Email != "" {
		profile.Sub = profile.Email
	}

	// Ensure we have at least an identifier
	if profile.Sub == "" {
		s.logger.Error("No user identifier found in token response")
		return nil, errors.NewAuthenticationError("Invalid token: no user identifier")
	}

	// Note: Access tokens don't always include name fields, unlike ID tokens
	// These fields might be empty, which is expected behavior
	profile.Name = getStringValue(tokenInfo, "name")
	profile.GivenName = getStringValue(tokenInfo, "given_name")
	profile.FamilyName = getStringValue(tokenInfo, "family_name")
	profile.Locale = getStringValue(tokenInfo, "locale")

	s.logger.WithFields(map[string]interface{}{
		"user_id":        profile.Sub,
		"email":          profile.Email,
		"email_verified": profile.EmailVerified,
		"has_picture":    profile.Picture != "",
		"has_name":       profile.Name != "",
	}).Info("Google access token validated successfully")
	return profile, nil
}

// validateSupabaseJWT validates a Supabase JWT token with proper signature verification
func (s *Service) validateSupabaseJWT(ctx context.Context, tokenString string) (*domain.UserProfile, error) {
	s.logger.Debug("Validating Supabase JWT token with signature verification")

	// Get Supabase JWT secret from environment
	jwtSecret := os.Getenv("SUPABASE_JWT_SECRET")
	if jwtSecret == "" {
		s.logger.Error("SUPABASE_JWT_SECRET not configured")
		return nil, errors.NewAuthenticationError("JWT validation not configured")
	}

	// Parse and validate the JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing algorithm
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		s.logger.WithError(err).Error("Failed to parse/validate JWT token")
		return nil, errors.NewAuthenticationError("Invalid JWT token")
	}

	if !token.Valid {
		s.logger.Error("JWT token is not valid")
		return nil, errors.NewAuthenticationError("Invalid JWT token")
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		s.logger.Error("Failed to extract JWT claims")
		return nil, errors.NewAuthenticationError("Invalid JWT token")
	}

	// Check token expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			s.logger.Error("JWT token has expired")
			return nil, errors.NewAuthenticationError("Token has expired")
		}
	}

	// Extract user information from Supabase JWT claims
	profile := &domain.UserProfile{
		Sub:           getStringValue(claims, "sub"),
		Email:         getStringValue(claims, "email"),
		EmailVerified: getBoolValue(claims, "email_verified"),
	}

	// Get user metadata if present
	if userMeta, ok := claims["user_metadata"].(map[string]interface{}); ok {
		profile.Name = getStringValue(userMeta, "name")
		profile.Picture = getStringValue(userMeta, "avatar_url")
		profile.GivenName = getStringValue(userMeta, "given_name")
		profile.FamilyName = getStringValue(userMeta, "family_name")
	}

	// Ensure we have at least an identifier
	if profile.Sub == "" {
		s.logger.Error("No user identifier found in JWT token")
		return nil, errors.NewAuthenticationError("Invalid JWT token: no user identifier")
	}

	s.logger.WithField("user_id", profile.Sub).Debug("Supabase JWT token validated successfully")
	return profile, nil
}

// ValidateJWTToken validates a JWT token and returns auth claims
func (s *Service) ValidateJWTToken(ctx context.Context, token string) (*domain.AuthClaims, error) {
	s.logger.Debug("Validating JWT token")

	// For now, we'll use Google's tokeninfo endpoint to validate JWT tokens
	// In a production environment, you'd want to use a proper JWT library
	url := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", token)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.NewInternalError("Failed to create tokeninfo request", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.WithError(err).Error("Failed to call Google tokeninfo endpoint")
		return nil, errors.NewAuthenticationError("Failed to validate JWT token")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.WithField("status_code", resp.StatusCode).Error("Google tokeninfo returned error")
		return nil, errors.NewAuthenticationError("Invalid or expired JWT token")
	}

	var tokenInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		s.logger.WithError(err).Error("Failed to decode tokeninfo response")
		return nil, errors.NewInternalError("Failed to decode token information", err)
	}

	// Verify the audience (client ID)
	if aud, ok := tokenInfo["aud"].(string); !ok || aud != s.clientID {
		s.logger.WithField("audience", tokenInfo["aud"]).Error("Token audience mismatch")
		return nil, errors.NewAuthenticationError("Token audience mismatch")
	}

	// Convert to auth claims
	claims := &domain.AuthClaims{
		Sub:           getStringValue(tokenInfo, "sub"),
		Email:         getStringValue(tokenInfo, "email"),
		Name:          getStringValue(tokenInfo, "name"),
		Picture:       getStringValue(tokenInfo, "picture"),
		EmailVerified: getBoolValue(tokenInfo, "email_verified"),
		Aud:           getStringValue(tokenInfo, "aud"),
		Iss:           getStringValue(tokenInfo, "iss"),
		Iat:           getInt64Value(tokenInfo, "iat"),
		Exp:           getInt64Value(tokenInfo, "exp"),
	}

	s.logger.WithField("user_id", claims.Sub).Debug("JWT token validated successfully")
	return claims, nil
}

// GetUserProfile gets user profile from validated token
func (s *Service) GetUserProfile(ctx context.Context, userID string) (*domain.User, error) {
	s.logger.WithField("user_id", userID).Debug("Getting user profile")

	// This would typically fetch from a database
	// For now, we'll return a placeholder
	user := &domain.User{
		ID:            userID,
		Email:         "",    // Would be populated from database
		Name:          "",    // Would be populated from database
		Picture:       "",    // Would be populated from database
		EmailVerified: false, // Would be populated from database
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	return user, nil
}

// Helper functions for token format detection
func isGoogleAccessToken(token string) bool {
	// Google access tokens start with "ya29."
	return len(token) > 5 && token[:5] == "ya29."
}

func isJWTToken(token string) bool {
	// JWT tokens have exactly 3 segments separated by dots
	segments := len(token) > 0
	if !segments {
		return false
	}

	dotCount := 0
	for _, char := range token {
		if char == '.' {
			dotCount++
		}
	}
	return dotCount == 2
}

// Helper functions to safely extract values from tokenInfo map
func getStringValue(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getBoolValue(m map[string]interface{}, key string) bool {
	if val, ok := m[key].(bool); ok {
		return val
	}
	return false
}

func getInt64Value(m map[string]interface{}, key string) int64 {
	if val, ok := m[key].(float64); ok {
		return int64(val)
	}
	return 0
}

package be

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gamemini/youtube/pkg/config"
)

// Credentials represents the user credentials for BE login
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Response represents the authentication response
type Response struct {
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
}

// User represents a user in the system
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	// Additional user fields can be added here
}

// getJWTSecret returns the JWT secret from configuration or default
func getJWTSecret() []byte {
	appConfig := config.GetConfig()
	secret := appConfig.JWTSecret
	if secret == "" {
		secret = "your-secret-key" // Default for development
	}
	return []byte(secret)
}

// Login authenticates a user with BE credentials
func Login(w http.ResponseWriter, r *http.Request) {
	var creds Credentials

	// Decode JSON request
	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{Message: "Invalid request format"})
		return
	}

	// Validate credentials (in a real app, check against database)
	// This is just a simple example
	if !validateCredentials(creds.Username, creds.Password) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(Response{Message: "Invalid credentials"})
		return
	}

	// Generate JWT token
	token, err := generateToken(creds.Username)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Message: "Error generating token"})
		return
	}

	// Return the token
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{Token: token, Message: "Login successful"})
}

// validateCredentials checks if the provided credentials are valid
// In a real application, this would check against a database
func validateCredentials(username, password string) bool {
	// Example validation - replace with actual validation logic
	// This is just for demonstration
	return username == "testuser" && password == "password123"
}

// generateToken creates a new JWT token for the authenticated user
func generateToken(username string) (string, error) {
	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(), // 24 hour expiration
	})

	// Sign the token with our secret
	tokenString, err := token.SignedString(getJWTSecret())
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// VerifyToken validates a JWT token and returns the user information
func VerifyToken(tokenString string) (*User, error) {
	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return getJWTSecret(), nil
	})

	if err != nil {
		return nil, err
	}

	// Check if token is valid
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		username := claims["username"].(string)

		// In a real app, you'd fetch the full user from database
		user := &User{
			ID:       "user-id", // This would come from the database
			Username: username,
		}

		return user, nil
	}

	return nil, err
}

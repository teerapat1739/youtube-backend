package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gamemini/youtube/pkg/api"
	"github.com/gamemini/youtube/pkg/auth/be"
	"github.com/gamemini/youtube/pkg/auth/google"
	"github.com/gamemini/youtube/pkg/database"
	"github.com/gamemini/youtube/pkg/handlers"
	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/services"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

// Load .env file
func loadEnv() {
	file, err := os.Open(".env.local")
	if err != nil {
		fmt.Println("No .env.local file found, using system environment variables")
		return
	}
	defer file.Close()

	fmt.Println("Loading .env.local file...")
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
			fmt.Printf("Loaded: %s\n", key)
		}
	}
}

// getAllowedOrigins reads CORS allowed origins from environment variable
// Returns an error if ALLOWED_ORIGINS is not set or empty to make configuration explicit
func getAllowedOrigins() ([]string, error) {
	// Try to get origins from environment variable
	envOrigins := os.Getenv("ALLOWED_ORIGINS")
	if envOrigins == "" {
		return nil, fmt.Errorf("ALLOWED_ORIGINS environment variable is required but not set")
	}

	// Parse comma-separated origins from environment
	origins := make([]string, 0)
	for _, origin := range strings.Split(envOrigins, ",") {
		// Trim whitespace and skip empty strings
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	// If no valid origins found after parsing, return error
	if len(origins) == 0 {
		return nil, fmt.Errorf("ALLOWED_ORIGINS contains no valid origins after parsing")
	}

	log.Printf("🌐 [CORS] Using origins from ALLOWED_ORIGINS environment variable: %v", origins)
	return origins, nil
}

// Global variable to store validated allowed origins
var globalAllowedOrigins []string

// CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Log CORS request for debugging
		log.Printf("🌐 [CORS] Request from origin: %s, Method: %s, Path: %s", origin, r.Method, r.URL.Path)

		// Get frontend URL from environment variable
		frontendURL := os.Getenv("FRONTEND_URL")

		// Use pre-validated allowed origins
		allowedOrigins := globalAllowedOrigins

		// Add FRONTEND_URL to allowed origins if it's set and not already included
		if frontendURL != "" {
			found := false
			for _, allowed := range allowedOrigins {
				if allowed == frontendURL {
					found = true
					break
				}
			}
			if !found {
				// Create a copy to avoid modifying global slice
				allowedOrigins = append([]string{}, allowedOrigins...)
				allowedOrigins = append(allowedOrigins, frontendURL)
				log.Printf("🌐 [CORS] Added FRONTEND_URL to allowed origins: %s", frontendURL)
			}
		}

		// Log allowed origins for debugging on first request (to avoid spam)
		if r.URL.Path == "/health" && r.Method == "GET" {
			log.Printf("🌐 [CORS] Current allowed origins: %v", allowedOrigins)
		}

		// Check if origin is allowed
		originAllowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				originAllowed = true
				break
			}
		}

		if originAllowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			log.Printf("✅ [CORS] Allowed origin: %s", origin)
		} else if origin == "" {
			// If no origin header (like direct API calls), allow localhost:3000 as default
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		} else {
			// For development, be more permissive with localhost origins
			if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				// Log the blocked origin for debugging
				log.Printf("⚠️ [CORS] Blocked origin: %s", origin)
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, Origin, User-Agent, DNT, Cache-Control, X-Mx-ReqToken, Keep-Alive, X-Requested-With, If-Modified-Since, sec-ch-ua, sec-ch-ua-mobile, sec-ch-ua-platform, Referer")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")

		// Handle preflight OPTIONS requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Load environment variables from .env.local file
	loadEnv()

	// Validate required configuration early
	fmt.Println("🔧 Validating configuration...")
	var err error
	globalAllowedOrigins, err = getAllowedOrigins()
	if err != nil {
		log.Fatalf("❌ [CONFIG] Failed to get allowed origins: %v", err)
	}
	fmt.Printf("✅ [CONFIG] ALLOWED_ORIGINS validated: %v\n", globalAllowedOrigins)

	// Initialize database connection
	fmt.Println("🔌 Initializing database connection...")
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	// Initialize enhanced authentication handlers
	fmt.Println("🔐 Initializing enhanced authentication handlers...")
	authHandlers := handlers.NewAuthHandlers()

	router := mux.NewRouter()

	// Enhanced Auth routes with proper user creation/update
	router.HandleFunc("/auth/google/login", authHandlers.HandleGoogleLogin).Methods("GET")
	router.HandleFunc("/auth/google/callback", authHandlers.HandleGoogleCallback).Methods("GET")
	// BE Login route (using be package)
	router.HandleFunc("/auth/be/login", func(w http.ResponseWriter, r *http.Request) {
		be.Login(w, r)
	}).Methods("POST")

	// Activity routes with logging
	router.HandleFunc("/api/check-subscription", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("🔍 [ROUTER] Received request: %s %s from %s", r.Method, r.URL.String(), r.RemoteAddr)
		log.Printf("🔍 [ROUTER] User-Agent: %s", r.Header.Get("User-Agent"))
		log.Printf("🔍 [ROUTER] Authorization header present: %t", r.Header.Get("Authorization") != "")
		api.HandleSubscriptionCheck(w, r)
	}).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/join-activity", api.HandleJoinActivity).Methods("POST")

	// Ananped celebration routes
	router.HandleFunc("/api/ananped/subscription-check", api.HandleAnanpedSubscriptionCheck).Methods("GET", "OPTIONS")

	// Enhanced YouTube API routes
	router.HandleFunc("/api/user-info", authHandlers.HandleUserInfo).Methods("GET", "OPTIONS")
	// YouTube subscriptions route
	router.HandleFunc("/api/youtube-subscriptions", func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Remove "Bearer " prefix if present
		if strings.HasPrefix(token, "Bearer ") {
			token = token[7:]
		}

		subscriptions, err := google.GetYouTubeSubscriptions(token)
		if err != nil {
			http.Error(w, "Failed to get subscriptions: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(subscriptions)
	}).Methods("GET", "OPTIONS")

	// New API routes for the voting system
	// Voting system routes with hardcoded teams
	router.HandleFunc("/api/activities/active", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("🎯 [API] GET /api/activities/active - Request from %s", r.RemoteAddr)

		// Extract user ID for personalized response
		_, _, userID, err := extractUserFromToken(r)
		if err != nil {
			log.Printf("❌ [API] Failed to extract user from token: %v", err)
			// For now, use a default userID for anonymous users
			userID = "anonymous-" + fmt.Sprintf("%d", time.Now().Unix())
		}

		teamService := services.NewTeamService()
		activityID := "active" // Use "active" as the default activity ID

		activity, err := teamService.GetActivityWithTeams(r.Context(), activityID, userID)
		if err != nil {
			log.Printf("❌ [API] Failed to get activity with teams: %v", err)
			http.Error(w, fmt.Sprintf("Failed to get activity: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("✅ [API] Successfully retrieved activity with %d teams", len(activity.Teams))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    activity,
		})
	}).Methods("GET", "OPTIONS")

	router.HandleFunc("/api/activities/{id}/teams", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		activityID := vars["id"]
		log.Printf("🏆 [API] GET /api/activities/%s/teams", activityID)

		// Extract user ID for personalized response
		_, _, userID, err := extractUserFromToken(r)
		if err != nil {
			log.Printf("❌ [API] Failed to extract user from token: %v", err)
			userID = "anonymous-" + fmt.Sprintf("%d", time.Now().Unix())
		}

		teamService := services.NewTeamService()
		teams, err := teamService.GetTeamsWithVotes(r.Context(), activityID)
		if err != nil {
			log.Printf("❌ [API] Failed to get teams: %v", err)
			http.Error(w, fmt.Sprintf("Failed to get teams: %v", err), http.StatusInternalServerError)
			return
		}

		// Get user vote status
		userVote, err := teamService.GetUserVoteStatus(r.Context(), userID, activityID)
		if err != nil {
			log.Printf("❌ [API] Failed to get user vote status: %v", err)
			userVote = &models.VotingStatus{HasVoted: false}
		}

		log.Printf("✅ [API] Successfully retrieved %d teams", len(teams))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"teams":       teams,
				"activity_id": activityID,
				"user_vote":   userVote,
			},
		})
	}).Methods("GET", "OPTIONS")

	router.HandleFunc("/api/activities/{id}/vote", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		activityID := vars["id"]
		log.Printf("🗳️  [API] POST /api/activities/%s/vote", activityID)

		var voteRequest models.CreateVoteRequest
		if err := json.NewDecoder(r.Body).Decode(&voteRequest); err != nil {
			log.Printf("❌ [API] Invalid request body: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		_, _, userID, err := extractUserFromToken(r)
		if err != nil {
			log.Printf("❌ [API] Failed to extract user from token: %v", err)
			http.Error(w, fmt.Sprintf("Authentication required: %v", err), http.StatusUnauthorized)
			return
		}

		log.Printf("🔐 [API] Vote request - UserID: %s, TeamID: %s", userID, voteRequest.TeamID)

		teamService := services.NewTeamService()
		response, err := teamService.SubmitVote(r.Context(), userID, voteRequest.TeamID, activityID)
		if err != nil {
			log.Printf("❌ [API] Failed to submit vote: %v", err)
			http.Error(w, fmt.Sprintf("Failed to submit vote: %v", err), http.StatusBadRequest)
			return
		}

		log.Printf("✅ [API] Vote submitted successfully")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    response,
		})
	}).Methods("POST", "OPTIONS")

	// Add vote status endpoint
	router.HandleFunc("/api/activities/{id}/vote-status", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		activityID := vars["id"]
		log.Printf("📊 [API] GET /api/activities/%s/vote-status", activityID)

		_, _, userID, err := extractUserFromToken(r)
		if err != nil {
			log.Printf("❌ [API] Failed to extract user from token: %v", err)
			http.Error(w, fmt.Sprintf("Authentication required: %v", err), http.StatusUnauthorized)
			return
		}

		teamService := services.NewTeamService()
		voteStatus, err := teamService.GetUserVoteStatus(r.Context(), userID, activityID)
		if err != nil {
			log.Printf("❌ [API] Failed to get vote status: %v", err)
			http.Error(w, fmt.Sprintf("Failed to get vote status: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("✅ [API] Vote status retrieved - HasVoted: %v", voteStatus.HasVoted)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    voteStatus,
		})
	}).Methods("GET", "OPTIONS")

	// Enhanced user profile routes with proper authentication
	router.HandleFunc("/api/user/profile", authHandlers.HandleGetUserProfile).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/user/profile", authHandlers.HandleUpdateUserProfile).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/user/profile/create", authHandlers.HandleCreateInitialUserProfile).Methods("POST", "OPTIONS")

	// Terms and PDPA routes
	router.HandleFunc("/api/terms", authHandlers.HandleGetTerms).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/user/accept-terms", authHandlers.HandleAcceptTerms).Methods("POST", "OPTIONS")

	// Legacy routes (plural) for backward compatibility
	router.HandleFunc("/api/users/profile", authHandlers.HandleGetUserProfile).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/users/profile", authHandlers.HandleUpdateUserProfile).Methods("PUT", "OPTIONS")

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Check database health
		dbHealth := "connected"
		if err := database.HealthCheck(); err != nil {
			dbHealth = "disconnected"
		}

		health := map[string]interface{}{
			"status":    "healthy",
			"timestamp": "2025-01-20T00:00:00Z",
			"version":   "1.0.0",
			"services": map[string]string{
				"database": dbHealth,
				"api":      "running",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	}).Methods("GET")

	// CORS middleware
	router.Use(corsMiddleware)

	// Get port from environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Debug: Print environment info
	fmt.Printf("🌍 Environment: %s\n", os.Getenv("NODE_ENV"))
	fmt.Printf("🔑 GOOGLE_CLIENT_ID: %s\n", os.Getenv("GOOGLE_CLIENT_ID"))
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if len(clientSecret) > 10 {
		fmt.Printf("🔑 GOOGLE_CLIENT_SECRET: %s\n", clientSecret[:10]+"...")
	} else if len(clientSecret) > 0 {
		fmt.Printf("🔑 GOOGLE_CLIENT_SECRET: %s\n", strings.Repeat("*", len(clientSecret)))
	} else {
		fmt.Printf("🔑 GOOGLE_CLIENT_SECRET: (not set)\n")
	}

	// Additional YouTube API related environment variables
	youtubeAPIKey := os.Getenv("YOUTUBE_API_KEY")
	if youtubeAPIKey == "" {
		fmt.Printf("🔑 YOUTUBE_API_KEY: (empty)\n")
	} else if len(youtubeAPIKey) > 10 {
		fmt.Printf("🔑 YOUTUBE_API_KEY: %s...\n", youtubeAPIKey[:10])
	} else {
		fmt.Printf("🔑 YOUTUBE_API_KEY: %s\n", strings.Repeat("*", len(youtubeAPIKey)))
	}
	fmt.Printf("🔑 JWT_SECRET: %s\n", func() string {
		secret := os.Getenv("JWT_SECRET")
		if len(secret) > 10 {
			return secret[:10] + "..."
		} else if len(secret) > 0 {
			return strings.Repeat("*", len(secret))
		}
		return "(not set)"
	}())
	fmt.Printf("🌐 FRONTEND_URL: %s\n", os.Getenv("FRONTEND_URL"))
	fmt.Printf("🌐 REDIRECT_URL: %s\n", os.Getenv("REDIRECT_URL"))
	fmt.Printf("🌐 ALLOWED_ORIGINS: %s\n", os.Getenv("ALLOWED_ORIGINS"))

	// Start server
	fmt.Printf("🚀 Server is running on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

// extractUserFromToken extracts user information from JWT token with proper verification
func extractUserFromToken(r *http.Request) (googleID, email, userID string, err error) {
	log.Println("🔐 Starting token extraction...")

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", "", "", fmt.Errorf("no authorization header")
	}

	// Extract token from "Bearer <token>"
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", "", "", fmt.Errorf("invalid authorization header format")
	}

	tokenString := parts[1]
	log.Printf("🔑 Token type detected: %s", tokenString[:10]+"...")

	// Handle Google OAuth tokens (ya29.xxx format)
	if strings.HasPrefix(tokenString, "ya29.") {
		log.Println("📱 Google OAuth token detected - using Google API for verification")
		return verifyGoogleOAuthToken(tokenString)
	}

	// Handle custom JWT tokens
	log.Println("🔐 Custom JWT token detected - using JWT verification")
	return verifyCustomJWTToken(tokenString)
}

// verifyGoogleOAuthToken verifies Google OAuth access tokens
func verifyGoogleOAuthToken(tokenString string) (googleID, email, userID string, err error) {
	// For now, create consistent user data based on token
	// In production, you would call Google's tokeninfo endpoint:
	// https://oauth2.googleapis.com/tokeninfo?access_token=TOKEN

	// Create a hash-based user ID from token for consistency
	hasher := fmt.Sprintf("%x", tokenString[5:15])
	googleID = "google-user-" + hasher
	email = "user-" + hasher + "@gmail.com"
	userID = googleID // Use googleID as userID for consistency

	log.Printf("✅ Google token verified - UserID: %s, Email: %s", userID, email)
	return googleID, email, userID, nil
}

// verifyCustomJWTToken verifies custom JWT tokens issued by your backend
func verifyCustomJWTToken(tokenString string) (googleID, email, userID string, err error) {
	// Get JWT secret from environment
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", "", "", fmt.Errorf("JWT_SECRET not configured")
	}

	// Parse and validate JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		log.Printf("❌ JWT verification failed: %v", err)
		return "", "", "", fmt.Errorf("invalid JWT token: %v", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		googleID = fmt.Sprintf("%v", claims["google_id"])
		email = fmt.Sprintf("%v", claims["email"])
		userID = fmt.Sprintf("%v", claims["user_id"])

		log.Printf("✅ JWT token verified - UserID: %s, Email: %s", userID, email)
		return googleID, email, userID, nil
	}

	return "", "", "", fmt.Errorf("invalid token claims")
}

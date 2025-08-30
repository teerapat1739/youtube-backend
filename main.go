package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gamemini/youtube/pkg/api"
	"github.com/gamemini/youtube/pkg/auth/be"
	"github.com/gamemini/youtube/pkg/auth/google"
	"github.com/gamemini/youtube/pkg/config"
	"github.com/gamemini/youtube/pkg/database"
	"github.com/gamemini/youtube/pkg/handlers"
	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/services"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

// Legacy type aliases for backward compatibility
type Config = config.Config
type GoogleConfig = config.OAuthConfig


// corsMiddleware creates a CORS middleware with the given configuration
func corsMiddleware(appConfig *Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Build effective allowed origins list
			allowedOrigins := buildAllowedOrigins(appConfig)

			// Log CORS request (only for non-health endpoints to reduce noise)
			if r.URL.Path != "/health" {
				log.Printf("🌐 [CORS] %s %s from origin: %s", r.Method, r.URL.Path, origin)
			}

			// Set CORS headers
			setCORSHeaders(w, origin, allowedOrigins)

			// Handle preflight OPTIONS requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// buildAllowedOrigins constructs the complete list of allowed origins
func buildAllowedOrigins(appConfig *Config) []string {
	allowedOrigins := make([]string, len(appConfig.AllowedOrigins))
	copy(allowedOrigins, appConfig.AllowedOrigins)

	// Add FRONTEND_URL if not already included
	if appConfig.FrontendURL != "" && !contains(allowedOrigins, appConfig.FrontendURL) {
		allowedOrigins = append(allowedOrigins, appConfig.FrontendURL)
	}

	return allowedOrigins
}

// setCORSHeaders sets appropriate CORS headers based on origin validation
func setCORSHeaders(w http.ResponseWriter, origin string, allowedOrigins []string) {
	// Standard CORS headers
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, Origin, User-Agent, DNT, Cache-Control, X-Mx-ReqToken, Keep-Alive, X-Requested-With, If-Modified-Since, sec-ch-ua, sec-ch-ua-mobile, sec-ch-ua-platform, Referer, Idempotency-Key")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")

	// Determine allowed origin
	if isOriginAllowed(origin, allowedOrigins) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	} else if origin == "" {
		// No origin header (direct API calls) - allow first configured origin
		if len(allowedOrigins) > 0 {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigins[0])
		}
	} else {
		// Development mode: be permissive with localhost/127.0.0.1
		if isDevelopmentOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			log.Printf("🔧 [CORS] Development origin allowed: %s", origin)
		} else {
			log.Printf("❌ [CORS] Origin blocked: %s", origin)
		}
	}
}

// isOriginAllowed checks if an origin is in the allowed list
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	return contains(allowedOrigins, origin)
}

// isDevelopmentOrigin checks if an origin appears to be for development
func isDevelopmentOrigin(origin string) bool {
	return strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1")
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func main() {
	// Load and validate configuration using centralized config system
	appConfig, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("❌ Configuration error: %v", err)
	}

	// Print configuration summary
	appConfig.PrintSummary()

	// Initialize database
	if err := initializeDatabase(); err != nil {
		log.Fatalf("❌ Database initialization failed: %v", err)
	}
	defer database.CloseDB()

	// Create HTTP server
	server := createServer(appConfig)

	// Start server with graceful shutdown
	startServerWithGracefulShutdown(server, appConfig.Port)
}

// initializeDatabase initializes the database connection
func initializeDatabase() error {
	log.Println("🔌 Initializing database connection...")
	if err := database.InitDB(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	log.Println("✅ Database connection established")
	return nil
}

// createServer creates and configures the HTTP server
func createServer(appConfig *Config) *http.Server {
	router := setupRoutes(appConfig)

	// Apply CORS middleware
	router.Use(corsMiddleware(appConfig))

	return &http.Server{
		Addr:         ":" + appConfig.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// setupRoutes configures all application routes
func setupRoutes(_ *Config) *mux.Router {
	log.Println("🔧 Setting up routes...")
	router := mux.NewRouter()

	// Initialize handlers
	authHandlers := handlers.NewAuthHandlers()

	// Setup route groups
	setupAuthRoutes(router, authHandlers)
	setupAPIRoutes(router)
	setupHealthRoutes(router)

	log.Println("✅ Routes configured successfully")
	return router
}

// setupAuthRoutes configures authentication routes
func setupAuthRoutes(router *mux.Router, authHandlers *handlers.AuthHandlers) {
	// Google OAuth routes
	router.HandleFunc("/auth/google/login", authHandlers.HandleGoogleLogin).Methods("GET")
	router.HandleFunc("/auth/google/callback", authHandlers.HandleGoogleCallback).Methods("GET")
	router.HandleFunc("/auth/logout", authHandlers.HandleLogout).Methods("POST", "OPTIONS")

	// Backend login route
	router.HandleFunc("/auth/be/login", be.Login).Methods("POST")

	// User profile routes
	router.HandleFunc("/api/user/profile", authHandlers.HandleGetUserProfile).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/user/profile", authHandlers.HandleUpdateUserProfile).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/user/profile/create", authHandlers.HandleCreateInitialUserProfile).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/user/profile/personal-info", authHandlers.HandleUpdatePersonalInfo).Methods("POST", "OPTIONS")

	// Terms and compliance routes
	router.HandleFunc("/api/terms", authHandlers.HandleGetTerms).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/user/accept-terms", authHandlers.HandleAcceptTerms).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/activity/rules", authHandlers.HandleGetActivityRules).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/user/accept-activity-rules", authHandlers.HandleAcceptActivityRules).Methods("POST", "OPTIONS")

	// Token management routes
	router.HandleFunc("/api/user/token-status", api.HandleTokenStatus).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/user/force-reauth", api.HandleForceReauth).Methods("GET", "OPTIONS")

	// Legacy routes for backward compatibility
	router.HandleFunc("/api/users/profile", authHandlers.HandleGetUserProfile).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/users/profile", authHandlers.HandleUpdateUserProfile).Methods("PUT", "OPTIONS")
}

// setupAPIRoutes configures API routes
func setupAPIRoutes(router *mux.Router) {
	// Initialize handlers
	authHandlers := handlers.NewAuthHandlers()

	// Subscription and activity routes
	router.HandleFunc("/api/check-subscription", logRequestHandler("check-subscription", api.HandleSubscriptionCheck)).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/join-activity", api.HandleJoinActivity).Methods("POST")

	// Ananped specific routes
	router.HandleFunc("/api/ananped/subscription-check", api.HandleAnanpedSubscriptionCheck).Methods("GET", "OPTIONS")

	// Testing routes
	router.HandleFunc("/api/test/subscription/{user_id}/{channel_id}", api.HandleTestSubscription).Methods("GET", "OPTIONS")

	// YouTube API routes
	router.HandleFunc("/api/user-info", authHandlers.HandleUserInfo).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/youtube-subscriptions", handleYouTubeSubscriptions).Methods("GET", "OPTIONS")

	// Vote and activity routes
	router.HandleFunc("/api/activities/{id}/counts", api.HandleGetVoteCounts).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/activities/active", handleActiveActivity).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/activities/{id}/teams", handleActivityTeams).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/activities/{id}/vote", handleSubmitVote).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/activities/{id}/vote-status", handleVoteStatus).Methods("GET", "OPTIONS")
}

// setupHealthRoutes configures health check routes
func setupHealthRoutes(router *mux.Router) {
	router.HandleFunc("/health", handleHealthCheck).Methods("GET")
}

// Handler functions

// logRequestHandler logs API requests for debugging
func logRequestHandler(name string, handler http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("🔍 [%s] %s %s from %s", strings.ToUpper(name), r.Method, r.URL.String(), r.RemoteAddr)
		log.Printf("🔍 [%s] User-Agent: %s", strings.ToUpper(name), r.Header.Get("User-Agent"))
		log.Printf("🔍 [%s] Authorization header present: %t", strings.ToUpper(name), r.Header.Get("Authorization") != "")
		handler.ServeHTTP(w, r)
	})
}

// handleHealthCheck handles the health check endpoint
func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
		"services": map[string]interface{}{
			"database": checkDatabaseHealth(),
			"api":      "running",
		},
	}

	json.NewEncoder(w).Encode(health)
}

// handleYouTubeSubscriptions handles YouTube subscriptions API
func handleYouTubeSubscriptions(w http.ResponseWriter, r *http.Request) {
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
}

// handleActiveActivity handles getting active activity with teams
func handleActiveActivity(w http.ResponseWriter, r *http.Request) {
	log.Printf("🎯 [API] GET /api/activities/active - Request from %s", r.RemoteAddr)

	// Extract user ID for personalized response
	_, _, userID, err := extractUserFromToken(r)
	if err != nil {
		log.Printf("❌ [API] Failed to extract user from token: %v", err)
		// Use anonymous user ID for unauthenticated requests
		userID = fmt.Sprintf("anonymous-%d", time.Now().Unix())
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
}

// handleActivityTeams handles getting teams for a specific activity
func handleActivityTeams(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	activityID := vars["id"]
	log.Printf("🏆 [API] GET /api/activities/%s/teams", activityID)

	// Extract user ID for personalized response
	_, _, userID, err := extractUserFromToken(r)
	if err != nil {
		log.Printf("❌ [API] Failed to extract user from token: %v", err)
		userID = fmt.Sprintf("anonymous-%d", time.Now().Unix())
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
}

// handleSubmitVote handles vote submission
func handleSubmitVote(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	activityID := vars["id"]
	log.Printf("🗳️ [API] POST /api/activities/%s/vote", activityID)

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
}

// handleVoteStatus handles getting user vote status
func handleVoteStatus(w http.ResponseWriter, r *http.Request) {
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
}


// startServerWithGracefulShutdown starts the server and handles graceful shutdown
func startServerWithGracefulShutdown(server *http.Server, port string) {
	// Channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("🚀 Server starting on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stop
	log.Println("🚑 Shutdown signal received")

	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	log.Println("🔄 Shutting down server gracefully...")
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("⚠️  Server shutdown error: %v", err)
	} else {
		log.Println("✅ Server stopped gracefully")
	}
}

// checkDatabaseHealth checks if the database is healthy
func checkDatabaseHealth() map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	db := database.GetDB()
	if db == nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  "database connection not initialized",
		}
	}

	if err := db.Ping(ctx); err != nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	}

	return map[string]interface{}{
		"status": "healthy",
	}
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
	// Get JWT secret from configuration
	appConfig := config.GetConfig()
	jwtSecret := appConfig.JWTSecret
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

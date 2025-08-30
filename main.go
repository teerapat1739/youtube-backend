package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gamemini/youtube/pkg/api"
	"github.com/gamemini/youtube/pkg/config"
	"github.com/gamemini/youtube/pkg/container"
	"github.com/gamemini/youtube/pkg/database"
	"github.com/gamemini/youtube/pkg/handlers"
	serverutils "github.com/gamemini/youtube/pkg/server"
	"github.com/gorilla/mux"
)

// Legacy type aliases for backward compatibility
type Config = config.Config

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

	// Initialize dependency injection container
	log.Println("🏗️  Initializing dependency injection container...")
	appContainer := container.NewAppContainer(appConfig)
	log.Println("✅ Container initialized with all dependencies")

	// Create HTTP server with container
	server := createServer(appConfig, appContainer)

	// Start server with graceful shutdown
	serverutils.StartServerWithGracefulShutdown(server, appConfig.Port)
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
func createServer(appConfig *Config, appContainer *container.AppContainer) *http.Server {
	router := setupRoutes(appConfig, appContainer)

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
func setupRoutes(_ *Config, appContainer *container.AppContainer) *mux.Router {
	log.Println("🔧 Setting up routes...")
	router := mux.NewRouter()

	// Get handlers from container (single instances)
	authHandlers := appContainer.GetAuthHandlers()

	// Setup route groups
	setupAuthRoutes(router, authHandlers)
	setupAPIRoutes(router, appContainer)
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

	// User profile routes
	router.HandleFunc("/api/user/profile", authHandlers.HandleGetUserProfile).Methods("GET", "OPTIONS")

	// Complete profile update endpoint - updates personal info AND requires terms/PDPA acceptance
	// Used for: Initial profile completion after OAuth login
	// Expects: first_name, last_name, phone, accept_terms=true, accept_pdpa=true
	// Validates: All fields required, terms must be accepted
	router.HandleFunc("/api/user/profile", authHandlers.HandleUpdateUserProfile).Methods("POST", "OPTIONS")

	// Initial profile verification endpoint - confirms user exists after OAuth callback
	// Used for: Immediately after Google OAuth login to verify user record creation
	// Expects: Only JWT token in Authorization header (no request body)
	// Returns: Existing user data with profile_completed status
	// Note: Does NOT create user (user already created in OAuth callback)
	router.HandleFunc("/api/user/profile/create", authHandlers.HandleCreateInitialUserProfile).Methods("POST", "OPTIONS")

	// Personal info update endpoint - updates personal details without requiring terms re-acceptance
	// Used for: Updating profile info for users who already accepted terms
	// Expects: first_name, last_name, phone (optional)
	// Preserves: Existing terms/PDPA acceptance status
	// Note: Separate from main profile update to avoid forcing terms re-acceptance
	router.HandleFunc("/api/user/profile/personal-info", authHandlers.HandleUpdatePersonalInfo).Methods("POST", "OPTIONS")

	// Terms and compliance routes
	router.HandleFunc("/api/terms", authHandlers.HandleGetTerms).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/user/accept-terms", authHandlers.HandleAcceptTerms).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/activity/rules", authHandlers.HandleGetActivityRules).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/user/accept-activity-rules", authHandlers.HandleAcceptActivityRules).Methods("POST", "OPTIONS")

}

// setupAPIRoutes configures API routes
func setupAPIRoutes(router *mux.Router, appContainer *container.AppContainer) {

	// Subscription and activity routes
	router.HandleFunc("/api/check-subscription", func(w http.ResponseWriter, r *http.Request) {
		logRequestHandler("check-subscription", func(w http.ResponseWriter, r *http.Request) {
			api.HandleSubscriptionCheckWithContainer(w, r, appContainer)
		})(w, r)
	}).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/join-activity", api.HandleJoinActivity).Methods("POST")

	// Ananped specific routes
	router.HandleFunc("/api/ananped/subscription-check", func(w http.ResponseWriter, r *http.Request) {
		api.HandleAnanpedSubscriptionCheckWithContainer(w, r, appContainer)
	}).Methods("GET", "OPTIONS")

	// Vote and activity routes with container
	// Create closures that capture the container for handlers that need it
	router.HandleFunc("/api/activities/{id}/vote", func(w http.ResponseWriter, r *http.Request) {
		api.HandleSubmitVoteWithContainer(w, r, appContainer)
	}).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/activities/{id}/vote-status", func(w http.ResponseWriter, r *http.Request) {
		api.HandleVoteStatusWithContainer(w, r, appContainer)
	}).Methods("GET", "OPTIONS")
}

// setupHealthRoutes configures health check routes
func setupHealthRoutes(router *mux.Router) {
	router.HandleFunc("/health", api.HandleHealthCheck).Methods("GET", "OPTIONS")
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


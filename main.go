package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"be-v2/internal/config"
	"be-v2/internal/container"
	"be-v2/internal/handler"
	"be-v2/internal/middleware"
	"be-v2/internal/repository"
	"be-v2/internal/service"
	"be-v2/pkg/database"
	"be-v2/pkg/logger"
	"be-v2/pkg/redis"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	log.WithFields(map[string]interface{}{
		"port":        cfg.Port,
		"log_level":   cfg.LogLevel,
		"environment": "development",
	}).Info("Starting be-v2 server")

	// Create dependency injection container
	container, err := container.New(cfg, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create container")
	}

	// Initialize database connection
	ctx := context.Background()
	db, err := database.NewPostgresDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// Initialize Redis connection
	redisClient, err := redis.NewClient(cfg.RedisURL)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to Redis")
	}
	defer redisClient.Close()

	// Initialize repositories and services
	voteRepo := repository.NewVoteRepository(db)
	votingService := service.NewVotingService(voteRepo, redisClient, log.Logger)

	// Setup router
	router := setupRouter(container, votingService)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info("Server starting on port " + cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		log.WithError(err).Fatal("Server forced to shutdown")
	}

	log.Info("Server exited")
}

// setupRouter configures and returns the HTTP router
func setupRouter(container *container.Container, votingService *service.VotingService) *chi.Mux {
	cfg := container.GetConfig()
	log := container.GetLogger()
	authService := container.GetAuthService()

	// Create router
	r := chi.NewRouter()

	// Setup CORS middleware
	corsConfig := &middleware.CORSConfig{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
		ExposedHeaders:   []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           86400,
	}

	// Setup middlewares
	r.Use(middleware.CORS(corsConfig, log))
	r.Use(middleware.RequestID(log))
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(60 * time.Second))

	// Create handlers
	healthHandler := handler.NewHealthHandler(container)
	authHandler := handler.NewAuthHandler(container)
	subscriptionHandler := handler.NewSubscriptionHandler(container)
	votingHandler := handler.NewVotingHandler(votingService)

	// Setup routes

	// Health check (no auth required)
	r.Get("/health", healthHandler.Check)

	// Public API routes
	r.Route("/api", func(r chi.Router) {
		// YouTube channel info (no auth required)
		r.Get("/youtube/channel/{channelId}", subscriptionHandler.GetChannelInfo)

		// Voting routes
		r.Route("/v1/voting", func(r chi.Router) {
			// Public endpoints (no authentication required)
			r.Get("/status", votingHandler.GetVotingStatus)
			r.Get("/results", votingHandler.GetVotingResults)
			
			// Protected voting endpoints (require authentication)
			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(authService, log))
				
				r.Post("/vote", votingHandler.SubmitVote)
				r.Get("/my-status", votingHandler.GetMyVoteStatus)
				r.Get("/verify/{voteId}", votingHandler.VerifyVote)
			})
		})

		// Protected routes (require authentication)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(authService, log))

			// User routes
			r.Route("/user", func(r chi.Router) {
				r.Get("/profile", authHandler.GetProfile)
			})

			// YouTube routes
			r.Route("/youtube", func(r chi.Router) {
				r.Get("/subscription-check", subscriptionHandler.CheckSubscription)
			})
		})
	})

	// 404 handler
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"success":false,"error":{"type":"not_found","message":"Endpoint not found"}}`))
	})

	log.Info("Router configured successfully")
	return r
}
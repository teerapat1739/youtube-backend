package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
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

// Resources holds all resources that need cleanup
type Resources struct {
	db             *database.PostgresDB
	redisClient    *redis.Client
	visitorService service.VisitorService
	server         *http.Server
	log            *logger.Logger
	mu             sync.Mutex
	closed         bool
}

// Cleanup gracefully closes all resources
func (r *Resources) Cleanup(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true

	var errors []error

	r.log.Info("Starting graceful shutdown...")

	// Shutdown HTTP server first to stop accepting new requests
	if r.server != nil {
		r.log.Info("Shutting down HTTP server...")
		if err := r.server.Shutdown(ctx); err != nil {
			r.log.WithError(err).Error("Failed to shutdown HTTP server")
			errors = append(errors, fmt.Errorf("HTTP server shutdown: %w", err))
		} else {
			r.log.Info("HTTP server shutdown complete")
		}
	}

	// Stop visitor service (saves final snapshot)
	if r.visitorService != nil {
		r.log.Info("Stopping visitor service...")
		if err := r.visitorService.Stop(ctx); err != nil {
			r.log.WithError(err).Error("Failed to stop visitor service")
			errors = append(errors, fmt.Errorf("visitor service shutdown: %w", err))
		} else {
			r.log.Info("Visitor service stopped successfully")
		}
	}

	// Close Redis connection with health check
	if r.redisClient != nil {
		r.log.Info("Closing Redis connection...")

		// Quick health check before closing (with short timeout)
		healthCtx, healthCancel := context.WithTimeout(ctx, 2*time.Second)
		if err := r.redisClient.Health(healthCtx); err != nil {
			r.log.WithError(err).Warn("Redis health check failed before closing")
		}
		healthCancel()

		if err := r.redisClient.Close(); err != nil {
			r.log.WithError(err).Error("Failed to close Redis connection")
			errors = append(errors, fmt.Errorf("Redis close: %w", err))
		} else {
			r.log.Info("Redis connection closed successfully")
		}
	}

	// Close database connection pool with health check
	if r.db != nil {
		r.log.Info("Closing database connection pool...")

		// Quick health check before closing (with short timeout)
		healthCtx, healthCancel := context.WithTimeout(ctx, 2*time.Second)
		if err := r.db.Health(healthCtx); err != nil {
			r.log.WithError(err).Warn("Database health check failed before closing")
		}
		healthCancel()

		r.db.Close()
		r.log.Info("Database connection pool closed successfully")
	}

	if len(errors) > 0 {
		r.log.WithField("error_count", len(errors)).Error("Cleanup completed with errors")
		return fmt.Errorf("cleanup completed with %d errors: %v", len(errors), errors)
	}

	r.log.Info("Graceful shutdown completed successfully")
	return nil
}

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
		"environment": cfg.Environment,
	}).Info("Starting be-v2 server")

	// Create dependency injection container
	container, err := container.New(cfg, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create container")
	}

	// Initialize database connection
	ctx := context.Background()
	db, err := database.NewPostgresDB(ctx, cfg.DatabaseURL, cfg.DatabaseReadURL)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}

	// Initialize Redis connection
	redisClient, err := redis.NewClient(cfg.RedisURL, cfg.Environment)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to Redis")
	}

	// Initialize repositories and services
	voteRepo := repository.NewVoteRepository(db)
	votingService := service.NewVotingService(voteRepo, redisClient, log.Logger)

	// Initialize visitor service
	visitorRepo := repository.NewVisitorRepository(db)
	visitorService := service.NewVisitorService(redisClient, visitorRepo, log, cfg.Environment)

	// Start visitor service
	if err := visitorService.Start(ctx); err != nil {
		log.WithError(err).Fatal("Failed to start visitor service")
	}

	// Setup router
	router := setupRouter(container, votingService, visitorService, db)

	// Create HTTP server with optimized timeouts for high load
	server := &http.Server{
		Addr:           ":" + cfg.Port,
		Handler:        router,
		ReadTimeout:    10 * time.Second,  // Reduced for faster failure detection
		WriteTimeout:   60 * time.Second,  // Increased to align with upstream timeouts
		IdleTimeout:    120 * time.Second, // Increased for connection reuse
		MaxHeaderBytes: 1 << 20,           // 1MB max header size
	}

	// Create resources manager for cleanup
	resources := &Resources{
		db:             db,
		redisClient:    redisClient,
		visitorService: visitorService,
		server:         server,
		log:            log,
	}

	// Setup graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	// Setup cleanup function that will be called regardless of how the program exits
	defer func() {
		// Create context with timeout for cleanup operations
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := resources.Cleanup(cleanupCtx); err != nil {
			log.WithError(err).Error("Cleanup completed with errors")
		}
	}()

	// Start server in a goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		log.Info("Server starting on port " + cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("Server error occurred")
			serverErrChan <- err
		}
	}()

	// Wait for interrupt signal or server error
	select {
	case sig := <-quit:
		log.WithField("signal", sig.String()).Info("Received shutdown signal")
	case err := <-serverErrChan:
		log.WithError(err).Error("Server failed, initiating shutdown")
	}

	log.Info("Initiating graceful shutdown...")

	// Create context with timeout for shutdown operations
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Perform cleanup - this will be called here and also in defer for safety
	if err := resources.Cleanup(shutdownCtx); err != nil {
		log.WithError(err).Error("Graceful shutdown completed with errors")
		os.Exit(1)
	}

	log.Info("Application shutdown complete")
}

// setupRouter configures and returns the HTTP router
func setupRouter(container *container.Container, votingService *service.VotingService, visitorService service.VisitorService, db *database.PostgresDB) *chi.Mux {
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
	r.Use(chiMiddleware.Compress(5)) // Add gzip compression with level 5 (balanced)
	r.Use(chiMiddleware.Timeout(60 * time.Second))

	// Create handlers
	healthHandler := handler.NewHealthHandler(container)
	authHandler := handler.NewAuthHandler(container)
	subscriptionHandler := handler.NewSubscriptionHandler(container)
	votingHandler := handler.NewVotingHandler(votingService)
	visitorHandler := handler.NewVisitorHandler(visitorService, log)
	testingHandler := handler.NewTestingHandler(container, db)

	// Setup routes

	// Health check (no auth required)
	r.Get("/health", healthHandler.Check)

	// Public API routes
	r.Route("/api", func(r chi.Router) {
		// YouTube channel info (no auth required)
		r.Get("/youtube/channel/{channelId}", subscriptionHandler.GetChannelInfo)

		// Visitor tracking routes (no auth required)
		visitorHandler.RegisterRoutes(r)

		// Voting routes (legacy endpoints)
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

			// Personal info and voting endpoints (auth required)
			r.Post("/personal-info", votingHandler.CreatePersonalInfo)
			r.Post("/vote", votingHandler.SubmitVoteOnly)
			r.Get("/personal-info/me", votingHandler.GetPersonalInfoMe)

			// Welcome/Rules acceptance endpoint
			r.Post("/welcome/accept", votingHandler.AcceptWelcome)

			// Add v1/user routes for frontend compatibility (auth required)
			r.Route("/v1/user", func(r chi.Router) {
				r.Post("/personal-info", votingHandler.CreatePersonalInfo)
				r.Post("/vote", votingHandler.SubmitVoteOnly)
			})

			// User routes
			r.Route("/user", func(r chi.Router) {
				r.Get("/profile", authHandler.GetProfile)
				r.Get("/status", votingHandler.GetUserStatus)
			})

			// YouTube routes
			r.Route("/youtube", func(r chi.Router) {
				r.Get("/subscription-check", subscriptionHandler.CheckSubscription)
			})
		})

		// Testing routes (development environment only, no auth required)
		r.Route("/testing", func(r chi.Router) {
			// These endpoints are only available in development environment
			// The handler itself will check the environment and return 403 if not in development
			r.Post("/refresh-materialized-view", testingHandler.RefreshMaterializedView)
			r.Get("/materialized-view-stats", testingHandler.GetMaterializedViewStats)
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

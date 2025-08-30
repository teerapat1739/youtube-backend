package container

import (
	"github.com/gamemini/youtube/pkg/config"
	"github.com/gamemini/youtube/pkg/handlers"
	"github.com/gamemini/youtube/pkg/repository"
	"github.com/gamemini/youtube/pkg/services"
)

// AppContainer holds all application dependencies
// This implements the Dependency Injection pattern to avoid creating
// new instances of services and repositories on every request
// It implements the interfaces.Container interface
type AppContainer struct {
	// Configuration
	Config *config.Config

	// Repositories - Data access layer
	UserRepo *repository.UserRepository
	// Add other repositories as needed
	// TeamRepo *repository.TeamRepository
	// VoteRepo *repository.VoteRepository

	// Services - Business logic layer
	UserService    *services.UserService
	TeamService    *services.TeamService
	YouTubeService *services.YouTubeService
	// Add other services as needed

	// Handlers - HTTP handlers
	AuthHandlers *handlers.AuthHandlers
}

// NewAppContainer creates and initializes all application dependencies
// This should be called once at application startup
func NewAppContainer(cfg *config.Config) *AppContainer {
	// Initialize repositories (single instances)
	userRepo := repository.NewUserRepository()

	// Initialize services with their dependencies (single instances)
	userService := services.NewUserService(userRepo, cfg.JWTSecret)
	teamService := services.NewTeamService()
	youtubeService := services.NewYouTubeService(userRepo, userService, cfg)

	// Create container with all dependencies
	container := &AppContainer{
		Config:         cfg,
		UserRepo:       userRepo,
		UserService:    userService,
		TeamService:    teamService,
		YouTubeService: youtubeService,
	}

	// Initialize handlers with container
	container.AuthHandlers = handlers.NewAuthHandlersWithContainer(container)

	return container
}

// GetUserRepo returns the singleton UserRepository instance
func (c *AppContainer) GetUserRepo() *repository.UserRepository {
	return c.UserRepo
}

// GetUserService returns the singleton UserService instance
func (c *AppContainer) GetUserService() *services.UserService {
	return c.UserService
}

// GetTeamService returns the singleton TeamService instance
func (c *AppContainer) GetTeamService() *services.TeamService {
	return c.TeamService
}

// GetAuthHandlers returns the singleton AuthHandlers instance
func (c *AppContainer) GetAuthHandlers() *handlers.AuthHandlers {
	return c.AuthHandlers
}

// GetYouTubeService returns the singleton YouTubeService instance
func (c *AppContainer) GetYouTubeService() *services.YouTubeService {
	return c.YouTubeService
}

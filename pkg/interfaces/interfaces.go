package interfaces

import (
	"github.com/gamemini/youtube/pkg/repository"
	"github.com/gamemini/youtube/pkg/services"
)

// Container defines the interface for dependency injection container
// This interface helps avoid circular dependencies
type Container interface {
	GetUserRepo() *repository.UserRepository
	GetUserService() *services.UserService
	GetTeamService() *services.TeamService
}

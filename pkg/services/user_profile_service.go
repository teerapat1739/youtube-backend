package services

import (
	"fmt"
	"net/http"

	"github.com/gamemini/youtube/pkg/models"
	"github.com/gamemini/youtube/pkg/repository"
)

type UserProfileService struct {
	userProfileRepo *repository.UserProfileRepository
	activityRepo    *repository.ActivityRepository
}

func NewUserProfileService(userProfileRepo *repository.UserProfileRepository, activityRepo *repository.ActivityRepository) *UserProfileService {
	return &UserProfileService{
		userProfileRepo: userProfileRepo,
		activityRepo:    activityRepo,
	}
}

// GetUserProfile gets user profile by Google ID
func (s *UserProfileService) GetUserProfile(googleID string) (*models.UserProfileResponse, error) {
	user, err := s.userProfileRepo.GetUserByGoogleID(googleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}

	response := &models.UserProfileResponse{
		Exists:         user != nil,
		User:           user,
		HasVoted:       false,
		ActivityStatus: "active",
	}

	// If user exists, check vote status
	if user != nil {
		// Check if user has voted
		hasVoted, votedTeamID, err := s.activityRepo.HasUserVoted(user.ID, "active")
		if err != nil {
			return nil, fmt.Errorf("failed to check vote status: %w", err)
		}

		response.HasVoted = hasVoted
		if hasVoted && votedTeamID != nil {
			response.VotedTeamID = votedTeamID
			// Get team name
			team, err := s.activityRepo.GetTeamByID(*votedTeamID)
			if err == nil && team != nil {
				response.VotedTeamName = &team.DisplayName
			}
		}
	}

	return response, nil
}

// CreateUserProfile creates a new user profile
func (s *UserProfileService) CreateUserProfile(req *models.UpdateUserProfileRequest, googleID, email string) (*models.User, error) {
	// Validate that terms and PDPA are accepted
	if !req.AcceptTerms {
		return nil, &ValidationError{
			Code:    "TERMS_NOT_ACCEPTED",
			Message: "Terms and conditions must be accepted",
			Details: map[string]string{"accept_terms": "Terms must be accepted"},
		}
	}

	if !req.AcceptPDPA {
		return nil, &ValidationError{
			Code:    "PDPA_NOT_ACCEPTED",
			Message: "PDPA must be accepted",
			Details: map[string]string{"accept_pdpa": "PDPA must be accepted"},
		}
	}

	// Check if user already exists
	existingUser, err := s.userProfileRepo.GetUserByGoogleID(googleID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	if existingUser != nil {
		// Update existing user
		return s.userProfileRepo.UpdateUserProfile(existingUser.ID, req)
	}

	// Create new user
	return s.userProfileRepo.CreateUserProfile(req, googleID, email)
}

// UpdateUserProfile updates an existing user profile
func (s *UserProfileService) UpdateUserProfile(userID string, req *models.UpdateUserProfileRequest) (*models.User, error) {
	// Validate that terms and PDPA are accepted
	if !req.AcceptTerms {
		return nil, &ValidationError{
			Code:    "TERMS_NOT_ACCEPTED",
			Message: "Terms and conditions must be accepted",
			Details: map[string]string{"accept_terms": "Terms must be accepted"},
		}
	}

	if !req.AcceptPDPA {
		return nil, &ValidationError{
			Code:    "PDPA_NOT_ACCEPTED",
			Message: "PDPA must be accepted",
			Details: map[string]string{"accept_pdpa": "PDPA must be accepted"},
		}
	}

	return s.userProfileRepo.UpdateUserProfile(userID, req)
}

// ValidateUserProfile validates user profile data
func (s *UserProfileService) ValidateUserProfile(req *models.UserProfileValidationRequest) (*models.ValidationResponse, error) {
	err := s.userProfileRepo.ValidateUserProfileData(req)
	if err != nil {
		// Parse validation errors
		errors := make(map[string]string)
		if validationErr, ok := err.(*ValidationError); ok {
			if details, ok := validationErr.Details.(map[string]string); ok {
				errors = details
			}
		} else {
			errors["general"] = err.Error()
		}

		return &models.ValidationResponse{
			Valid:  false,
			Errors: errors,
		}, nil
	}

	return &models.ValidationResponse{
		Valid: true,
	}, nil
}

// GetTermsContent gets current terms and PDPA content
func (s *UserProfileService) GetTermsContent() (*models.TermsResponse, error) {
	return s.userProfileRepo.GetTermsContent()
}

// AcceptTerms handles terms and PDPA acceptance
func (s *UserProfileService) AcceptTerms(userID string, req *models.AcceptTermsRequest, ipAddress, userAgent string) error {
	// Validate request
	if !req.AcceptTerms {
		return &ValidationError{
			Code:    "TERMS_NOT_ACCEPTED",
			Message: "Terms and conditions must be accepted",
			Details: map[string]string{"accept_terms": "Terms must be accepted"},
		}
	}

	if !req.AcceptPDPA {
		return &ValidationError{
			Code:    "PDPA_NOT_ACCEPTED",
			Message: "PDPA must be accepted",
			Details: map[string]string{"accept_pdpa": "PDPA must be accepted"},
		}
	}

	// Verify versions exist
	currentTermsVersion, currentPDPAVersion, err := s.userProfileRepo.GetCurrentTermsVersions()
	if err != nil {
		return fmt.Errorf("failed to get current terms versions: %w", err)
	}

	if req.TermsVersion != currentTermsVersion {
		return &ValidationError{
			Code:    "INVALID_TERMS_VERSION",
			Message: "Invalid terms version",
			Details: map[string]string{"terms_version": "Version mismatch"},
		}
	}

	if req.PDPAVersion != currentPDPAVersion {
		return &ValidationError{
			Code:    "INVALID_PDPA_VERSION",
			Message: "Invalid PDPA version",
			Details: map[string]string{"pdpa_version": "Version mismatch"},
		}
	}

	return s.userProfileRepo.AcceptTerms(userID, req, ipAddress, userAgent)
}

// GetVoteStatus checks if user has voted in a specific activity
func (s *UserProfileService) GetVoteStatus(userID, activityID string) (*models.VoteStatusResponse, error) {
	hasVoted, votedTeamID, err := s.activityRepo.HasUserVoted(userID, activityID)
	if err != nil {
		return nil, fmt.Errorf("failed to check vote status: %w", err)
	}

	response := &models.VoteStatusResponse{
		HasVoted:    hasVoted,
		VotedTeamID: votedTeamID,
	}

	// Get vote timestamp if voted
	if hasVoted {
		vote, err := s.activityRepo.GetUserVoteWithoutContext(userID, activityID)
		if err == nil && vote != nil {
			timestamp := vote.CreatedAt.Format("2006-01-02T15:04:05Z")
			response.VoteTimestamp = &timestamp
		}
	}

	return response, nil
}

// Custom error types
type ValidationError struct {
	Code    string
	Message string
	Details interface{}
}

func (e *ValidationError) Error() string {
	return e.Message
}

func (e *ValidationError) StatusCode() int {
	switch e.Code {
	case "TERMS_NOT_ACCEPTED", "PDPA_NOT_ACCEPTED":
		return http.StatusBadRequest
	case "PROFILE_INCOMPLETE":
		return http.StatusBadRequest
	case "INVALID_NATIONAL_ID", "DUPLICATE_NATIONAL_ID", "INVALID_PHONE":
		return http.StatusBadRequest
	case "INVALID_TERMS_VERSION", "INVALID_PDPA_VERSION":
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}

// Helper function to check if user profile is complete
func (s *UserProfileService) IsProfileComplete(user *models.User) bool {
	return user.FirstName != nil && *user.FirstName != "" &&
		user.LastName != nil && *user.LastName != "" &&
		user.Phone != nil && *user.Phone != "" &&
		user.TermsAccepted && user.PDPAAccepted
}

// Helper function to check if user can vote
func (s *UserProfileService) CanUserVote(user *models.User) (bool, string) {
	if !s.IsProfileComplete(user) {
		return false, "PROFILE_INCOMPLETE"
	}

	if !user.TermsAccepted {
		return false, "TERMS_NOT_ACCEPTED"
	}

	if !user.PDPAAccepted {
		return false, "PDPA_NOT_ACCEPTED"
	}

	return true, ""
}

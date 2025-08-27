package repository

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/gamemini/youtube/pkg/models"
	"github.com/jmoiron/sqlx"
)

type UserProfileRepository struct {
	db *sqlx.DB
}

func NewUserProfileRepository(db *sqlx.DB) *UserProfileRepository {
	return &UserProfileRepository{db: db}
}

// GetUserProfile gets user profile by user ID
func (r *UserProfileRepository) GetUserProfile(userID string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, google_id, email, first_name, last_name, phone,
			   terms_accepted, terms_version, pdpa_accepted, pdpa_version,
			   profile_completed, youtube_subscribed, subscription_verified_at,
			   created_at, updated_at
		FROM users WHERE id = $1
	`
	err := r.db.Get(user, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}
	return user, nil
}

// GetUserByGoogleID gets user by Google ID
func (r *UserProfileRepository) GetUserByGoogleID(googleID string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, google_id, email, first_name, last_name, phone,
			   terms_accepted, terms_version, pdpa_accepted, pdpa_version,
			   profile_completed, youtube_subscribed, subscription_verified_at,
			   created_at, updated_at
		FROM users WHERE google_id = $1
	`
	err := r.db.Get(user, query, googleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not found
		}
		return nil, fmt.Errorf("failed to get user by google_id: %w", err)
	}
	return user, nil
}

// CreateUserProfile creates a new user profile
func (r *UserProfileRepository) CreateUserProfile(req *models.UpdateUserProfileRequest, googleID, email string) (*models.User, error) {
	// Validate data before creating
	if err := r.ValidateUpdateUserProfileData(req); err != nil {
		return nil, err
	}

	// Get current terms versions
	termsVersion, pdpaVersion, err := r.GetCurrentTermsVersions()
	if err != nil {
		return nil, fmt.Errorf("failed to get terms versions: %w", err)
	}

	query := `
		INSERT INTO users (google_id, email, first_name, last_name, phone,
						   terms_accepted, terms_version, pdpa_accepted, pdpa_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, google_id, email, first_name, last_name, phone,
				  terms_accepted, terms_version, pdpa_accepted, pdpa_version,
				  profile_completed, youtube_subscribed, subscription_verified_at,
				  created_at, updated_at
	`

	user := &models.User{}
	err = r.db.Get(user, query,
		googleID, email, req.FirstName, req.LastName, req.Phone,
		req.AcceptTerms, termsVersion, req.AcceptPDPA, pdpaVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to create user profile: %w", err)
	}

	// Log terms acceptance
	if req.AcceptTerms && req.AcceptPDPA {
		err = r.LogTermsAcceptance(user.ID, termsVersion, pdpaVersion, "", "")
		if err != nil {
			// Log error but don't fail the user creation
			fmt.Printf("Warning: failed to log terms acceptance: %v\n", err)
		}
	}

	return user, nil
}

// UpdateUserProfile updates an existing user profile
func (r *UserProfileRepository) UpdateUserProfile(userID string, req *models.UpdateUserProfileRequest) (*models.User, error) {
	// Validate data before updating
	if err := r.ValidateUpdateUserProfileData(req); err != nil {
		return nil, err
	}

	// Get current terms versions
	termsVersion, pdpaVersion, err := r.GetCurrentTermsVersions()
	if err != nil {
		return nil, fmt.Errorf("failed to get terms versions: %w", err)
	}

	query := `
		UPDATE users 
		SET first_name = $2, last_name = $3, phone = $4,
			terms_accepted = $5, terms_version = $6, pdpa_accepted = $7, pdpa_version = $8,
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, google_id, email, first_name, last_name, phone,
				  terms_accepted, terms_version, pdpa_accepted, pdpa_version,
				  profile_completed, youtube_subscribed, subscription_verified_at,
				  created_at, updated_at
	`

	user := &models.User{}
	err = r.db.Get(user, query,
		userID, req.FirstName, req.LastName, req.Phone,
		req.AcceptTerms, termsVersion, req.AcceptPDPA, pdpaVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to update user profile: %w", err)
	}

	// Log terms acceptance
	if req.AcceptTerms && req.AcceptPDPA {
		err = r.LogTermsAcceptance(userID, termsVersion, pdpaVersion, "", "")
		if err != nil {
			// Log error but don't fail the update
			fmt.Printf("Warning: failed to log terms acceptance: %v\n", err)
		}
	}

	return user, nil
}

// ValidateUserProfileData validates user profile data
func (r *UserProfileRepository) ValidateUserProfileData(req *models.UserProfileValidationRequest) error {
	errors := make(map[string]string)

	// Validate first name
	if err := r.validateThaiName(req.FirstName, "first_name"); err != nil {
		errors["first_name"] = err.Error()
	}

	// Validate last name
	if err := r.validateThaiName(req.LastName, "last_name"); err != nil {
		errors["last_name"] = err.Error()
	}

	// Validate phone number
	if err := r.validateThaiPhone(req.Phone); err != nil {
		errors["phone"] = err.Error()
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed: %v", errors)
	}

	return nil
}

// ValidateUpdateUserProfileData for UpdateUserProfileRequest
func (r *UserProfileRepository) ValidateUpdateUserProfileData(req *models.UpdateUserProfileRequest) error {
	validation := &models.UserProfileValidationRequest{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
	}
	return r.ValidateUserProfileData(validation)
}

// GetCurrentTermsVersions gets the current active terms and PDPA versions
func (r *UserProfileRepository) GetCurrentTermsVersions() (string, string, error) {
	var termsVersion, pdpaVersion string

	// Get terms version
	err := r.db.Get(&termsVersion, "SELECT version FROM terms_versions WHERE type = 'terms' AND active = TRUE LIMIT 1")
	if err != nil {
		return "", "", fmt.Errorf("failed to get terms version: %w", err)
	}

	// Get PDPA version
	err = r.db.Get(&pdpaVersion, "SELECT version FROM terms_versions WHERE type = 'pdpa' AND active = TRUE LIMIT 1")
	if err != nil {
		return "", "", fmt.Errorf("failed to get PDPA version: %w", err)
	}

	return termsVersion, pdpaVersion, nil
}

// GetTermsContent gets the current terms and PDPA content
func (r *UserProfileRepository) GetTermsContent() (*models.TermsResponse, error) {
	var terms, pdpa models.TermsVersion

	// Get terms
	err := r.db.Get(&terms, "SELECT version, content FROM terms_versions WHERE type = 'terms' AND active = TRUE LIMIT 1")
	if err != nil {
		return nil, fmt.Errorf("failed to get terms content: %w", err)
	}

	// Get PDPA
	err = r.db.Get(&pdpa, "SELECT version, content FROM terms_versions WHERE type = 'pdpa' AND active = TRUE LIMIT 1")
	if err != nil {
		return nil, fmt.Errorf("failed to get PDPA content: %w", err)
	}

	return &models.TermsResponse{
		TermsVersion: terms.Version,
		TermsContent: terms.Content,
		PDPAVersion:  pdpa.Version,
		PDPAContent:  pdpa.Content,
	}, nil
}

// LogTermsAcceptance logs user's acceptance of terms and PDPA
func (r *UserProfileRepository) LogTermsAcceptance(userID, termsVersion, pdpaVersion, ipAddress, userAgent string) error {
	query := `
		INSERT INTO user_terms_acceptance (user_id, terms_version, pdpa_version, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, terms_version, pdpa_version) DO NOTHING
	`
	_, err := r.db.Exec(query, userID, termsVersion, pdpaVersion,
		sql.NullString{String: ipAddress, Valid: ipAddress != ""},
		sql.NullString{String: userAgent, Valid: userAgent != ""})
	if err != nil {
		return fmt.Errorf("failed to log terms acceptance: %w", err)
	}
	return nil
}

// Private validation methods

func (r *UserProfileRepository) validateThaiName(name, field string) error {
	if len(name) < 2 {
		return fmt.Errorf("%s must be at least 2 characters", field)
	}
	if len(name) > 50 {
		return fmt.Errorf("%s must not exceed 50 characters", field)
	}

	// Allow Thai, English, spaces, and hyphens
	matched, _ := regexp.MatchString(`^[ก-๏a-zA-Z\s\-]+$`, name)
	if !matched {
		return fmt.Errorf("%s contains invalid characters", field)
	}
	return nil
}

func (r *UserProfileRepository) validateThaiPhone(phone string) error {
	// Remove non-digits
	clean := regexp.MustCompile(`[^0-9]`).ReplaceAllString(phone, "")

	if len(clean) != 10 {
		return fmt.Errorf("phone number must be exactly 10 digits")
	}

	if !strings.HasPrefix(clean, "0") {
		return fmt.Errorf("phone number must start with 0")
	}

	// Check valid mobile prefixes
	prefix := clean[:2]
	validPrefixes := []string{"08", "09", "06"}
	isValid := false
	for _, validPrefix := range validPrefixes {
		if prefix == validPrefix {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid phone number prefix")
	}

	return nil
}

// AcceptTerms updates user's terms acceptance
func (r *UserProfileRepository) AcceptTerms(userID string, req *models.AcceptTermsRequest, ipAddress, userAgent string) error {
	// Update user's terms acceptance
	query := `
		UPDATE users 
		SET terms_accepted = $2, terms_version = $3, pdpa_accepted = $4, pdpa_version = $5, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(query, userID, req.AcceptTerms, req.TermsVersion, req.AcceptPDPA, req.PDPAVersion)
	if err != nil {
		return fmt.Errorf("failed to update terms acceptance: %w", err)
	}

	// Log the acceptance
	return r.LogTermsAcceptance(userID, req.TermsVersion, req.PDPAVersion, ipAddress, userAgent)
}

package repository

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/gamemini/youtube/pkg/database"
	"github.com/gamemini/youtube/pkg/models"
	"github.com/jackc/pgx/v5"
)

// UserRepository handles user-related database operations
type UserRepository struct{}

// NewUserRepository creates a new user repository
func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

// CreateUser creates a new user
func (r *UserRepository) CreateUser(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (google_id, email, first_name, last_name, national_id, phone, terms_accepted, pdpa_accepted, profile_completed)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`

	err := database.GetDB().QueryRow(ctx, query,
		user.GoogleID,
		user.Email,
		user.FirstName,
		user.LastName,
		user.NationalID,
		user.Phone,
		user.TermsAccepted,
		user.PDPAAccepted,
		user.ProfileCompleted,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	return nil
}

// GetUserByGoogleID retrieves a user by Google ID
func (r *UserRepository) GetUserByGoogleID(ctx context.Context, googleID string) (*models.User, error) {
	query := `
		SELECT id, google_id, email, first_name, last_name, national_id, phone,
		       youtube_subscribed, subscription_verified_at, created_at, updated_at
		FROM users
		WHERE google_id = $1
	`

	var user models.User
	err := database.GetDB().QueryRow(ctx, query, googleID).Scan(
		&user.ID,
		&user.GoogleID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.NationalID,
		&user.Phone,
		&user.YouTubeSubscribed,
		&user.SubscriptionVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by Google ID: %v", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (r *UserRepository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	query := `
		SELECT id, google_id, email, first_name, last_name, national_id, phone,
		       terms_accepted, terms_version, pdpa_accepted, pdpa_version, profile_completed,
		       youtube_subscribed, subscription_verified_at, 
		       google_access_token, google_refresh_token, google_token_expiry, youtube_channel_id,
		       created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := database.GetDB().QueryRow(ctx, query, userID).Scan(
		&user.ID,
		&user.GoogleID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.NationalID,
		&user.Phone,
		&user.TermsAccepted,
		&user.TermsVersion,
		&user.PDPAAccepted,
		&user.PDPAVersion,
		&user.ProfileCompleted,
		&user.YouTubeSubscribed,
		&user.SubscriptionVerifiedAt,
		&user.GoogleAccessToken,
		&user.GoogleRefreshToken,
		&user.GoogleTokenExpiry,
		&user.YouTubeChannelID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by ID: %v", err)
	}

	return &user, nil
}

// GetUserByNationalID retrieves a user by National ID
func (r *UserRepository) GetUserByNationalID(ctx context.Context, nationalID string) (*models.User, error) {
	query := `
		SELECT id, google_id, email, first_name, last_name, national_id, phone,
		       youtube_subscribed, subscription_verified_at, created_at, updated_at
		FROM users WHERE national_id = $1
	`

	var user models.User
	err := database.GetDB().QueryRow(ctx, query, nationalID).Scan(
		&user.ID,
		&user.GoogleID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.NationalID,
		&user.Phone,
		&user.YouTubeSubscribed,
		&user.SubscriptionVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // User not found
		}
		return nil, fmt.Errorf("failed to get user by national ID: %w", err)
	}

	return &user, nil
}

// UpdateUser updates a user
func (r *UserRepository) UpdateUser(ctx context.Context, userID string, updates *models.UpdateUserProfileRequest) error {
	query := "UPDATE users SET "
	args := []interface{}{}
	argCount := 0
	setParts := []string{}

	if updates.FirstName != "" {
		argCount++
		setParts = append(setParts, fmt.Sprintf("first_name = $%d", argCount))
		args = append(args, updates.FirstName)
	}

	if updates.LastName != "" {
		argCount++
		setParts = append(setParts, fmt.Sprintf("last_name = $%d", argCount))
		args = append(args, updates.LastName)
	}

	if updates.NationalID != "" {
		argCount++
		setParts = append(setParts, fmt.Sprintf("national_id = $%d", argCount))
		args = append(args, updates.NationalID)
	}

	if updates.Phone != "" {
		argCount++
		setParts = append(setParts, fmt.Sprintf("phone = $%d", argCount))
		args = append(args, updates.Phone)
	}

	// Always update the timestamp
	setParts = append(setParts, "updated_at = NOW()")

	// Build the final query
	query += strings.Join(setParts, ", ")
	argCount++
	query += fmt.Sprintf(" WHERE id = $%d", argCount)
	args = append(args, userID)

	_, err := database.GetDB().Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update user: %v", err)
	}

	return nil
}

// UpdateYouTubeSubscription updates the YouTube subscription status
func (r *UserRepository) UpdateYouTubeSubscription(ctx context.Context, userID string, subscribed bool) error {
	query := `
		UPDATE users
		SET youtube_subscribed = $2, subscription_verified_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	_, err := database.GetDB().Exec(ctx, query, userID, subscribed)
	if err != nil {
		return fmt.Errorf("failed to update YouTube subscription: %v", err)
	}

	return nil
}

// CreateUserSession creates a new user session
func (r *UserRepository) CreateUserSession(ctx context.Context, session *models.UserSession) error {
	query := `
		INSERT INTO user_sessions (user_id, session_token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`

	err := database.GetDB().QueryRow(ctx, query,
		session.UserID,
		session.SessionToken,
		session.ExpiresAt,
	).Scan(&session.ID, &session.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create user session: %v", err)
	}

	return nil
}

// GetUserSession retrieves a user session by token
func (r *UserRepository) GetUserSession(ctx context.Context, sessionToken string) (*models.UserSession, error) {
	query := `
		SELECT id, user_id, session_token, expires_at, created_at
		FROM user_sessions
		WHERE session_token = $1 AND expires_at > NOW()
	`

	var session models.UserSession
	err := database.GetDB().QueryRow(ctx, query, sessionToken).Scan(
		&session.ID,
		&session.UserID,
		&session.SessionToken,
		&session.ExpiresAt,
		&session.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user session: %v", err)
	}

	return &session, nil
}

// DeleteUserSession deletes a user session
func (r *UserRepository) DeleteUserSession(ctx context.Context, sessionToken string) error {
	query := `DELETE FROM user_sessions WHERE session_token = $1`

	_, err := database.GetDB().Exec(ctx, query, sessionToken)
	if err != nil {
		return fmt.Errorf("failed to delete user session: %v", err)
	}

	return nil
}

// CleanExpiredSessions cleans up expired user sessions
func (r *UserRepository) CleanExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM user_sessions WHERE expires_at <= NOW()`

	_, err := database.GetDB().Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to clean expired sessions: %v", err)
	}

	return nil
}

// UpsertUserFromOAuth creates or updates a user from OAuth data with proper race condition handling
func (r *UserRepository) UpsertUserFromOAuth(ctx context.Context, googleID, email string) (*models.User, bool, error) {
	log.Printf("🔄 Attempting to upsert user - GoogleID: %s, Email: %s", googleID, email)

	// Use PostgreSQL's UPSERT (INSERT ... ON CONFLICT) for atomic operation
	query := `
		INSERT INTO users (google_id, email, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (google_id) 
		DO UPDATE SET 
			email = EXCLUDED.email,
			updated_at = NOW()
		RETURNING id, google_id, email, first_name, last_name, national_id, phone,
				terms_accepted, terms_version, pdpa_accepted, pdpa_version, profile_completed,
				youtube_subscribed, subscription_verified_at, created_at, updated_at,
				(xmax = 0) as is_new_user
	`

	var user models.User
	var isNewUser bool

	err := database.GetDB().QueryRow(ctx, query, googleID, email).Scan(
		&user.ID,
		&user.GoogleID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.NationalID,
		&user.Phone,
		&user.TermsAccepted,
		&user.TermsVersion,
		&user.PDPAAccepted,
		&user.PDPAVersion,
		&user.ProfileCompleted,
		&user.YouTubeSubscribed,
		&user.SubscriptionVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&isNewUser,
	)

	if err != nil {
		log.Printf("❌ Failed to upsert user: %v", err)
		return nil, false, fmt.Errorf("failed to upsert user: %w", err)
	}

	log.Printf("✅ User upserted successfully - ID: %s, IsNew: %t", user.ID, isNewUser)
	return &user, isNewUser, nil
}

// UpsertUserFromOAuthWithTokens creates or updates a user from OAuth data with tokens
func (r *UserRepository) UpsertUserFromOAuthWithTokens(ctx context.Context, googleID, email string, tokenData *models.OAuthTokenData) (*models.User, bool, error) {
	log.Printf("🔄 Attempting to upsert user with OAuth tokens - GoogleID: %s, Email: %s", googleID, email)

	// Use PostgreSQL's UPSERT (INSERT ... ON CONFLICT) for atomic operation
	query := `
		INSERT INTO users (google_id, email, google_access_token, google_refresh_token, google_token_expiry, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (google_id) 
		DO UPDATE SET 
			email = EXCLUDED.email,
			google_access_token = EXCLUDED.google_access_token,
			google_refresh_token = EXCLUDED.google_refresh_token,
			google_token_expiry = EXCLUDED.google_token_expiry,
			updated_at = NOW()
		RETURNING id, google_id, email, first_name, last_name, national_id, phone,
				terms_accepted, terms_version, pdpa_accepted, pdpa_version, profile_completed,
				youtube_subscribed, subscription_verified_at,
				google_access_token, google_refresh_token, google_token_expiry, youtube_channel_id,
				created_at, updated_at,
				(xmax = 0) as is_new_user
	`

	var user models.User
	var isNewUser bool

	err := database.GetDB().QueryRow(ctx, query, googleID, email, tokenData.AccessToken, tokenData.RefreshToken, tokenData.Expiry).Scan(
		&user.ID,
		&user.GoogleID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.NationalID,
		&user.Phone,
		&user.TermsAccepted,
		&user.TermsVersion,
		&user.PDPAAccepted,
		&user.PDPAVersion,
		&user.ProfileCompleted,
		&user.YouTubeSubscribed,
		&user.SubscriptionVerifiedAt,
		&user.GoogleAccessToken,
		&user.GoogleRefreshToken,
		&user.GoogleTokenExpiry,
		&user.YouTubeChannelID,
		&user.CreatedAt,
		&user.UpdatedAt,
		&isNewUser,
	)

	if err != nil {
		log.Printf("❌ Failed to upsert user with tokens: %v", err)
		return nil, false, fmt.Errorf("failed to upsert user with tokens: %w", err)
	}

	log.Printf("✅ User with tokens upserted successfully - ID: %s, IsNew: %t", user.ID, isNewUser)
	return &user, isNewUser, nil
}

// UpdateUserOAuthTokens updates OAuth tokens for a user
func (r *UserRepository) UpdateUserOAuthTokens(ctx context.Context, userID string, tokenData *models.OAuthTokenData) error {
	log.Printf("🔄 Updating OAuth tokens for user - UserID: %s", userID)

	query := `
		UPDATE users SET 
			google_access_token = $2,
			google_refresh_token = $3,
			google_token_expiry = $4,
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := database.GetDB().Exec(ctx, query,
		userID,
		tokenData.AccessToken,
		tokenData.RefreshToken,
		tokenData.Expiry,
	)

	if err != nil {
		log.Printf("❌ Failed to update OAuth tokens: %v", err)
		return fmt.Errorf("failed to update OAuth tokens: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	log.Printf("✅ OAuth tokens updated successfully - UserID: %s", userID)
	return nil
}

// UpdateUserProfileAtomic updates user profile atomically with validation
func (r *UserRepository) UpdateUserProfileAtomic(ctx context.Context, userID string, updates *models.UpdateUserProfileRequest) error {
	log.Printf("📝 Updating user profile atomically - UserID: %s", userID)

	// Begin transaction for atomic update
	tx, err := database.GetDB().Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Update user profile
	query := `
		UPDATE users SET 
			first_name = $2,
			last_name = $3,
			national_id = $4,
			phone = $5,
			profile_completed = TRUE,
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := tx.Exec(ctx, query,
		userID,
		updates.FirstName,
		updates.LastName,
		updates.NationalID,
		updates.Phone,
	)

	if err != nil {
		log.Printf("❌ Failed to update user profile: %v", err)
		return fmt.Errorf("failed to update user profile: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		log.Printf("❌ Failed to commit transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("✅ User profile updated atomically - UserID: %s", userID)
	return nil
}

// UpdateTermsAcceptance updates terms and PDPA acceptance
func (r *UserRepository) UpdateTermsAcceptance(ctx context.Context, userID, termsVersion, pdpaVersion string, acceptTerms, acceptPDPA bool) error {
	log.Printf("📋 Updating terms acceptance - UserID: %s", userID)

	query := `
		UPDATE users SET 
			terms_accepted = $2,
			terms_version = $3,
			pdpa_accepted = $4,
			pdpa_version = $5,
			updated_at = NOW()
		WHERE id = $1
	`

	result, err := database.GetDB().Exec(ctx, query,
		userID,
		acceptTerms,
		termsVersion,
		acceptPDPA,
		pdpaVersion,
	)

	if err != nil {
		log.Printf("❌ Failed to update terms acceptance: %v", err)
		return fmt.Errorf("failed to update terms acceptance: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	log.Printf("✅ Terms acceptance updated - UserID: %s", userID)
	return nil
}

// CreateTermsAcceptanceRecord creates an audit record for terms acceptance
func (r *UserRepository) CreateTermsAcceptanceRecord(ctx context.Context, acceptance *models.UserTermsAcceptance) error {
	log.Printf("📋 Creating terms acceptance audit record - UserID: %s", acceptance.UserID)

	query := `
		INSERT INTO user_terms_acceptance (user_id, terms_version, pdpa_version, accepted_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`

	err := database.GetDB().QueryRow(ctx, query,
		acceptance.UserID,
		acceptance.TermsVersion,
		acceptance.PDPAVersion,
		acceptance.AcceptedAt,
		acceptance.IPAddress,
		acceptance.UserAgent,
	).Scan(&acceptance.ID)

	if err != nil {
		log.Printf("❌ Failed to create terms acceptance record: %v", err)
		return fmt.Errorf("failed to create terms acceptance record: %w", err)
	}

	log.Printf("✅ Terms acceptance audit record created - UserID: %s, ID: %s", acceptance.UserID, acceptance.ID)
	return nil
}

// GetUserVote retrieves a user's vote for a specific activity
func (r *UserRepository) GetUserVote(ctx context.Context, userID, activityID string) (*models.Vote, error) {
	query := `
		SELECT id, user_id, team_id, activity_id, created_at
		FROM votes
		WHERE user_id = $1 AND activity_id = $2
	`

	var vote models.Vote
	err := database.GetDB().QueryRow(ctx, query, userID, activityID).Scan(
		&vote.ID,
		&vote.UserID,
		&vote.TeamID,
		&vote.ActivityID,
		&vote.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user vote: %w", err)
	}

	return &vote, nil
}

// GetActiveTermsVersions retrieves active terms and PDPA versions
func (r *UserRepository) GetActiveTermsVersions(ctx context.Context) (termsVersion, pdpaVersion string, err error) {
	query := `
		SELECT version FROM terms_versions 
		WHERE type = $1 AND active = TRUE 
		ORDER BY created_at DESC 
		LIMIT 1
	`

	err = database.GetDB().QueryRow(ctx, query, "terms").Scan(&termsVersion)
	if err != nil {
		return "", "", fmt.Errorf("failed to get active terms version: %w", err)
	}

	err = database.GetDB().QueryRow(ctx, query, "pdpa").Scan(&pdpaVersion)
	if err != nil {
		return "", "", fmt.Errorf("failed to get active PDPA version: %w", err)
	}

	return termsVersion, pdpaVersion, nil
}

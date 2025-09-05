package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"be-v2/internal/domain"
	"be-v2/internal/repository"
	"be-v2/pkg/redis"
	"be-v2/pkg/utils"

	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

type VotingService struct {
	voteRepo     *repository.VoteRepository
	redis        *redis.Client
	cacheService *CacheService
	logger       *zap.Logger
}

func NewVotingService(voteRepo *repository.VoteRepository, redisClient *redis.Client, logger *zap.Logger) *VotingService {
	cacheService := NewCacheService(redisClient, logger)
	return &VotingService{
		voteRepo:     voteRepo,
		redis:        redisClient,
		cacheService: cacheService,
		logger:       logger,
	}
}

// SubmitVote handles vote submission with duplicate prevention
func (s *VotingService) SubmitVote(ctx context.Context, userID string, req *domain.VoteRequest, ipAddress, userAgent string) (*domain.VoteResponse, error) {
	// Normalize and validate phone number
	normalizedPhone, err := utils.NormalizePhoneNumber(req.PersonalInfo.Phone)
	if err != nil {
		return nil, fmt.Errorf("invalid phone number format: %w", err)
	}

	// Validate Thai mobile number
	if !utils.ValidateThaiPhoneNumber(normalizedPhone) {
		return nil, fmt.Errorf("phone number must be a valid Thai mobile number")
	}

	// Check if user has already voted using Redis
	voteKey := fmt.Sprintf(redis.KeyUserVoted, userID)
	exists, err := s.redis.Exists(ctx, voteKey)
	if err == nil && exists > 0 {
		return nil, fmt.Errorf("user has already voted")
	}

	// Check database as fallback
	existingVote, err := s.voteRepo.GetVoteByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing vote: %w", err)
	}
	if existingVote != nil {
		// Cache the vote status
		_ = s.redis.Set(ctx, voteKey, existingVote.TeamID, redis.TTLUserVote)
		return nil, fmt.Errorf("user has already voted")
	}

	// Check for duplicate phone number with Redis caching
	phoneUsed, err := s.cacheService.CheckPhoneUsageWithCache(ctx, normalizedPhone,
		func(ctx context.Context, phone string) (bool, error) {
			vote, err := s.voteRepo.GetVoteByPhone(ctx, phone)
			return vote != nil, err
		})
	if err != nil {
		return nil, fmt.Errorf("failed to check phone number: %w", err)
	}
	if phoneUsed {
		return nil, fmt.Errorf("this phone number has already been used to vote")
	}

	// Verify team exists with Redis caching
	team, err := s.cacheService.GetTeamWithCache(ctx, req.TeamID,
		func(ctx context.Context, id int) (*domain.Team, error) {
			return s.voteRepo.GetTeamByID(ctx, id)
		})
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("team not found")
	}

	// Generate unique vote ID
	voteID := s.generateVoteID()

	// Calculate data retention (1 year from now)
	retentionTime := time.Now().AddDate(1, 0, 0)
	consentTime := time.Now()

	// Create vote record with PDPA compliance and normalized phone
	vote := &domain.Vote{
		VoteID:               voteID,
		UserID:               userID,
		TeamID:               req.TeamID,
		VoterName:            fmt.Sprintf("%s %s", req.PersonalInfo.FirstName, req.PersonalInfo.LastName),
		VoterEmail:           req.PersonalInfo.Email,
		VoterPhone:           normalizedPhone, // Store normalized phone number
		IPAddress:            ipAddress,
		UserAgent:            userAgent,
		ConsentTimestamp:     &consentTime,
		ConsentIP:            ipAddress,
		PrivacyPolicyVersion: req.Consent.PrivacyPolicyVersion,
		ConsentPDPA:          req.Consent.PDPAConsent,
		MarketingConsent:     req.Consent.MarketingConsent,
		DataRetentionUntil:   &retentionTime,
	}

	// Save to database with error handling for unique constraint violations
	if err := s.voteRepo.CreateVote(ctx, vote); err != nil {
		// Check for unique constraint violation on phone number
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23505" { // Unique violation error code
				if strings.Contains(pgErr.ConstraintName, "phone") {
					return nil, fmt.Errorf("this phone number has already been used to vote")
				}
				if strings.Contains(pgErr.ConstraintName, "user_id") {
					return nil, fmt.Errorf("user has already voted")
				}
			}
		}
		return nil, fmt.Errorf("failed to save vote: %w", err)
	}

	// Cache user vote status and phone usage with error handling
	if err := s.cacheService.CacheVoteSubmission(ctx, userID, normalizedPhone, req.TeamID); err != nil {
		s.logger.Warn("Failed to cache vote submission",
			zap.String("user_id", userID),
			zap.Error(err))
		// Continue execution - caching failure shouldn't fail the vote
	}

	// Invalidate relevant caches for consistency
	s.cacheService.InvalidateVotingCaches(req.TeamID)

	return &domain.VoteResponse{
		VoteID:    voteID,
		TeamID:    req.TeamID,
		TeamName:  team.Name,
		Timestamp: vote.CreatedAt,
		Message:   "Vote submitted successfully",
	}, nil
}

// GetVotingStatus returns the current voting status
func (s *VotingService) GetVotingStatus(ctx context.Context, userID string) (*domain.VotingStatus, error) {
	// Try to get from cache first
	cachedData, err := s.redis.Get(ctx, redis.KeyVoteSummary)
	if err == nil && cachedData != "" {
		var status domain.VotingStatus
		if err := json.Unmarshal([]byte(cachedData), &status); err == nil {
			// Add user-specific voting status
			s.addUserVoteStatus(ctx, &status, userID)
			return &status, nil
		}
	}

	// Get from database
	teams, err := s.voteRepo.GetTeamsWithVoteCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams: %w", err)
	}

	// Get total vote count
	totalVotes, _ := s.voteRepo.GetTotalVoteCount(ctx)

	// Check if user has voted
	userVote, _ := s.voteRepo.GetVoteByUserID(ctx, userID)

	// Build response
	status := &domain.VotingStatus{
		Teams:        make([]domain.TeamWithVoteStatus, 0, len(teams)),
		TotalVotes:   totalVotes,
		LastUpdate:   time.Now(),
		UserHasVoted: userVote != nil,
	}

	if userVote != nil {
		status.UserVoteID = userVote.VoteID
	}

	// Add vote status to teams
	for _, team := range teams {
		teamWithStatus := domain.TeamWithVoteStatus{
			Team:         team,
			UserHasVoted: userVote != nil && userVote.TeamID == team.ID,
		}
		status.Teams = append(status.Teams, teamWithStatus)
	}

	// Cache the result (without user-specific data)
	cacheData := domain.VotingStatus{
		Teams:      status.Teams,
		TotalVotes: status.TotalVotes,
		LastUpdate: status.LastUpdate,
	}
	if data, err := json.Marshal(cacheData); err == nil {
		_ = s.redis.Set(ctx, redis.KeyVoteSummary, string(data), redis.TTLCounts)
	}

	return status, nil
}

// VerifyVote verifies a vote by vote ID
func (s *VotingService) VerifyVote(ctx context.Context, voteID string) (*domain.Vote, error) {
	vote, err := s.voteRepo.GetVoteByVoteID(ctx, voteID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify vote: %w", err)
	}
	if vote == nil {
		return nil, fmt.Errorf("vote not found")
	}
	return vote, nil
}

// GetUserVoteStatus checks if a user has voted
func (s *VotingService) GetUserVoteStatus(ctx context.Context, userID string) (*domain.Vote, error) {
	// Check cache first
	voteKey := fmt.Sprintf(redis.KeyUserVoted, userID)
	if exists, _ := s.redis.Exists(ctx, voteKey); exists > 0 {
		// User has voted, get details from database
		return s.voteRepo.GetVoteByUserID(ctx, userID)
	}

	// Check database
	vote, err := s.voteRepo.GetVoteByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Cache the result if user has voted
	if vote != nil {
		_ = s.redis.Set(ctx, voteKey, vote.TeamID, redis.TTLUserVote)
	}

	return vote, nil
}

// generateVoteID generates a unique vote ID
func (s *VotingService) generateVoteID() string {
	year := time.Now().Year()
	bytes := make([]byte, 4)
	rand.Read(bytes)
	random := hex.EncodeToString(bytes)
	return fmt.Sprintf("AC%d%s", year, random)
}

// addUserVoteStatus adds user-specific voting status to the response
func (s *VotingService) addUserVoteStatus(ctx context.Context, status *domain.VotingStatus, userID string) {
	userVote, _ := s.GetUserVoteStatus(ctx, userID)
	status.UserHasVoted = userVote != nil
	if userVote != nil {
		status.UserVoteID = userVote.VoteID
		// Update team voting status
		for i := range status.Teams {
			status.Teams[i].UserHasVoted = userVote.TeamID == status.Teams[i].ID
		}
	}
}

// GetVotingResults returns comprehensive voting results with rankings and statistics
func (s *VotingService) GetVotingResults(ctx context.Context) (*domain.VotingResults, error) {
	// Try to get from cache first
	cachedData, err := s.redis.Get(ctx, redis.KeyVotingResults)
	if err == nil && cachedData != "" {
		var results domain.VotingResults
		if err := json.Unmarshal([]byte(cachedData), &results); err == nil {
			return &results, nil
		}
	}

	// Get from database
	teams, err := s.voteRepo.GetTeamsWithVoteCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get teams with vote counts: %w", err)
	}

	// Get total vote count
	totalVotes, err := s.voteRepo.GetTotalVoteCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get total vote count: %w", err)
	}

	// Calculate rankings and percentages
	teamsWithRankings := s.buildTeamRankings(teams, totalVotes)

	// Determine winner (highest votes)
	var winner *domain.TeamResultWithRanking
	if len(teamsWithRankings) > 0 {
		winner = &teamsWithRankings[0]
		winner.IsWinner = true
	}

	// Build statistics
	statistics := s.buildVotingStatistics(teamsWithRankings, totalVotes)

	// Build response
	results := &domain.VotingResults{
		Teams:          teamsWithRankings,
		TotalVotes:     totalVotes,
		LastUpdate:     time.Now(),
		VotingComplete: totalVotes > 0, // Consider voting complete if there are votes
		Winner:         winner,
		Statistics:     statistics,
	}

	// Cache the results
	if data, err := json.Marshal(results); err == nil {
		_ = s.redis.Set(ctx, redis.KeyVotingResults, string(data), redis.TTLCounts)
	}

	return results, nil
}

// buildTeamRankings creates ranked team results with percentages
func (s *VotingService) buildTeamRankings(teams []domain.Team, totalVotes int) []domain.TeamResultWithRanking {
	if len(teams) == 0 {
		return []domain.TeamResultWithRanking{}
	}

	// Sort teams by vote count (descending)
	sortedTeams := make([]domain.Team, len(teams))
	copy(sortedTeams, teams)

	// Sort by vote count descending
	for i := 0; i < len(sortedTeams)-1; i++ {
		for j := i + 1; j < len(sortedTeams); j++ {
			if sortedTeams[j].VoteCount > sortedTeams[i].VoteCount {
				sortedTeams[i], sortedTeams[j] = sortedTeams[j], sortedTeams[i]
			}
		}
	}

	// Build ranked results
	rankedTeams := make([]domain.TeamResultWithRanking, len(sortedTeams))
	for i, team := range sortedTeams {
		percentage := 0.0
		if totalVotes > 0 {
			percentage = float64(team.VoteCount) / float64(totalVotes) * 100
		}

		rankedTeams[i] = domain.TeamResultWithRanking{
			Team:       team,
			Rank:       i + 1,
			Percentage: percentage,
			IsWinner:   i == 0 && team.VoteCount > 0,
		}
	}

	return rankedTeams
}

// buildVotingStatistics creates detailed voting statistics
func (s *VotingService) buildVotingStatistics(teams []domain.TeamResultWithRanking, totalVotes int) domain.VotingStatistics {
	// Get top 3 teams
	topTeams := make([]domain.TeamResultWithRanking, 0, 3)
	for i := 0; i < len(teams) && i < 3; i++ {
		topTeams = append(topTeams, teams[i])
	}

	// Build distribution (simplified version)
	distribution := s.buildVoteDistribution(teams)

	return domain.VotingStatistics{
		TotalParticipants: totalVotes, // In this system, one person = one vote
		VotingPeriod: domain.VotingPeriodInfo{
			Duration: "Active", // You can customize this based on actual voting period
			IsActive: true,     // You can implement actual voting period logic
		},
		TopTeams:     topTeams,
		Distribution: distribution,
	}
}

// buildVoteDistribution creates vote distribution by percentage ranges
func (s *VotingService) buildVoteDistribution(teams []domain.TeamResultWithRanking) []domain.VoteDistribution {
	if len(teams) == 0 {
		return []domain.VoteDistribution{}
	}

	// Define percentage ranges
	ranges := []struct {
		min, max float64
		label    string
	}{
		{50.0, 100.0, "50%+"},
		{25.0, 49.9, "25-50%"},
		{10.0, 24.9, "10-25%"},
		{1.0, 9.9, "1-10%"},
		{0.0, 0.9, "<1%"},
	}

	distribution := make([]domain.VoteDistribution, 0, len(ranges))
	totalTeams := len(teams)

	for _, r := range ranges {
		count := 0
		for _, team := range teams {
			if team.Percentage >= r.min && team.Percentage <= r.max {
				count++
			}
		}

		percentage := 0.0
		if totalTeams > 0 {
			percentage = float64(count) / float64(totalTeams) * 100
		}

		distribution = append(distribution, domain.VoteDistribution{
			Range:      r.label,
			Count:      count,
			Percentage: percentage,
		})
	}

	return distribution
}

// HealthCheck performs a comprehensive health check including cache health
func (s *VotingService) HealthCheck(ctx context.Context) error {
	// Check cache health
	if err := s.cacheService.HealthCheck(ctx); err != nil {
		return fmt.Errorf("cache health check failed: %w", err)
	}

	s.logger.Info("Voting service health check passed")
	return nil
}

// CreateOrUpdatePersonalInfo handles creating or updating personal information
func (s *VotingService) CreateOrUpdatePersonalInfo(ctx context.Context, userID string, req *domain.PersonalInfoRequest, ipAddress, userAgent string) (*domain.PersonalInfoResponse, error) {
	// Normalize and validate phone number
	normalizedPhone, err := utils.NormalizePhoneNumber(req.Phone)
	if err != nil {
		return nil, fmt.Errorf("invalid phone number format: %w", err)
	}

	// Validate Thai mobile number
	if !utils.ValidateThaiPhoneNumber(normalizedPhone) {
		return nil, fmt.Errorf("phone number must be a valid Thai mobile number")
	}

	// Create or update personal info
	response, err := s.voteRepo.UpsertPersonalInfo(ctx, userID, req, normalizedPhone, ipAddress, userAgent)
	if err != nil {
		s.logger.Error("Failed to upsert personal info",
			zap.String("phone", normalizedPhone),
			zap.Error(err))
		return nil, fmt.Errorf("failed to save personal information: %w", err)
	}

	// Cache the phone usage to prevent duplicate voting attempts
	phoneKey := fmt.Sprintf("phone_used:%s", normalizedPhone)
	_ = s.redis.Set(ctx, phoneKey, response.UserID, redis.TTLUserVote)

	s.logger.Info("Personal info saved successfully",
		zap.String("user_id", response.UserID),
		zap.String("phone", normalizedPhone))

	return response, nil
}

// SubmitVoteOnly handles vote submission for users who already have personal info
func (s *VotingService) SubmitVoteOnly(ctx context.Context, req *domain.VoteOnlyRequest) (*domain.VoteOnlyResponse, error) {
	// Validate team exists
	team, err := s.cacheService.GetTeamWithCache(ctx, req.CandidateID,
		func(ctx context.Context, id int) (*domain.Team, error) {
			return s.voteRepo.GetTeamByID(ctx, id)
		})
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("team not found")
	}

	// Submit vote
	response, err := s.voteRepo.UpdateVoteOnly(ctx, req)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return nil, err
		}
		if err == domain.ErrVoteFinalized {
			return nil, err
		}
		s.logger.Error("Failed to submit vote",
			zap.String("user_id", req.UserID),
			zap.Int("candidate_id", req.CandidateID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to submit vote: %w", err)
	}

	// Cache user vote status
	voteKey := fmt.Sprintf(redis.KeyUserVoted, req.UserID)
	_ = s.redis.Set(ctx, voteKey, req.CandidateID, redis.TTLUserVote)

	// Invalidate relevant caches for consistency
	s.cacheService.InvalidateVotingCaches(req.CandidateID)

	// Refresh materialized view asynchronously
	// Note: This is already handled in the repository UpdateVoteOnly method

	s.logger.Info("Vote submitted successfully",
		zap.String("user_id", req.UserID),
		zap.Int("candidate_id", req.CandidateID))

	return response, nil
}

// SubmitVoteByPhone handles vote submission using phone number for identification
func (s *VotingService) SubmitVoteByPhone(ctx context.Context, phone string, candidateID int) (*domain.VoteOnlyResponse, error) {
	// Normalize and validate phone number
	normalizedPhone, err := utils.NormalizePhoneNumber(phone)
	if err != nil {
		return nil, fmt.Errorf("invalid phone number format: %w", err)
	}

	// Get user by phone
	user, err := s.voteRepo.GetUserByPhone(ctx, normalizedPhone)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, domain.ErrUserNotFound
	}

	// Submit vote using user ID
	req := &domain.VoteOnlyRequest{
		UserID:      user.UserID,
		CandidateID: candidateID,
	}

	return s.SubmitVoteOnly(ctx, req)
}

// GetPersonalInfoByUserID retrieves personal info for the authenticated user
func (s *VotingService) GetPersonalInfoByUserID(ctx context.Context, userID string) (*domain.PersonalInfoMeResponse, error) {
	return s.voteRepo.GetPersonalInfoByUserID(ctx, userID)
}

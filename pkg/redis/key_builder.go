package redis

import "fmt"

// KeyBuilder provides environment-aware Redis key building functionality
type KeyBuilder struct {
	prefix string // Environment prefix (staging/prod)
}

// NewKeyBuilder creates a new key builder with environment-based prefix
func NewKeyBuilder(environment string) *KeyBuilder {
	// Set key prefix based on environment
	prefix := "prod"
	if environment == "development" || environment == "staging" {
		prefix = "staging"
	}

	return &KeyBuilder{
		prefix: prefix,
	}
}

// BuildKey constructs a Redis key with the environment prefix
func (kb *KeyBuilder) BuildKey(key string) string {
	return fmt.Sprintf("%s:%s", kb.prefix, key)
}

// GetPrefix returns the current environment prefix
func (kb *KeyBuilder) GetPrefix() string {
	return kb.prefix
}

// Voting key builders
func (kb *KeyBuilder) KeyTeamsAll() string {
	return kb.BuildKey(KeyTeamsAll)
}

func (kb *KeyBuilder) KeyTeamByID(teamID int) string {
	return kb.BuildKey(fmt.Sprintf(KeyTeamByID, teamID))
}

func (kb *KeyBuilder) KeyTeamCount(teamID int) string {
	return kb.BuildKey(fmt.Sprintf(KeyTeamCount, teamID))
}

func (kb *KeyBuilder) KeyUserVoted(userID string) string {
	return kb.BuildKey(fmt.Sprintf(KeyUserVoted, userID))
}

func (kb *KeyBuilder) KeyPhoneVoted(phone string) string {
	return kb.BuildKey(fmt.Sprintf(KeyPhoneVoted, phone))
}

func (kb *KeyBuilder) KeyVoteSummary() string {
	return kb.BuildKey(KeyVoteSummary)
}

func (kb *KeyBuilder) KeyVotingResults() string {
	return kb.BuildKey(KeyVotingResults)
}

func (kb *KeyBuilder) KeyLastUpdate() string {
	return kb.BuildKey(KeyLastUpdate)
}

func (kb *KeyBuilder) KeyETag(etag string) string {
	return kb.BuildKey(fmt.Sprintf(KeyETag, etag))
}

func (kb *KeyBuilder) KeyWelcomeAccepted(userID string) string {
	return kb.BuildKey(fmt.Sprintf(KeyWelcomeAccepted, userID))
}

// Subscription key builders
func (kb *KeyBuilder) KeySubscriptionCheck(userID, channelID string) string {
	return kb.BuildKey(fmt.Sprintf(KeySubscriptionCheck, userID, channelID))
}

// Visitor key builders
func (kb *KeyBuilder) KeyVisitorTotal() string {
	return kb.BuildKey("visitor:total")
}

func (kb *KeyBuilder) KeyVisitorDaily(date string) string {
	return kb.BuildKey(fmt.Sprintf("visitor:daily:%s", date))
}

func (kb *KeyBuilder) KeyVisitorUnique() string {
	return kb.BuildKey("visitor:unique")
}

func (kb *KeyBuilder) KeyVisitorUniqueDaily(date string) string {
	return kb.BuildKey(fmt.Sprintf("visitor:unique:daily:%s", date))
}

func (kb *KeyBuilder) KeyVisitorRateLimit(ipHash string) string {
	return kb.BuildKey(fmt.Sprintf("visitor:ratelimit:%s", ipHash))
}

func (kb *KeyBuilder) KeyVisitorLastUpdate() string {
	return kb.BuildKey("visitor:last_update")
}

// Generic key builders for custom patterns
func (kb *KeyBuilder) KeyCustom(pattern string, args ...interface{}) string {
	key := fmt.Sprintf(pattern, args...)
	return kb.BuildKey(key)
}
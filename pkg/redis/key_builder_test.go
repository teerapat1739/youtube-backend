package redis

import (
	"testing"
)

func TestKeyBuilder_Environment_Prefixes(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		expectedPrefix string
	}{
		{
			name:        "Production environment should use prod prefix",
			environment: "production",
			expectedPrefix: "prod",
		},
		{
			name:        "Development environment should use staging prefix",
			environment: "development",
			expectedPrefix: "staging",
		},
		{
			name:        "Staging environment should use staging prefix",
			environment: "staging",
			expectedPrefix: "staging",
		},
		{
			name:        "Unknown environment should default to prod prefix",
			environment: "unknown",
			expectedPrefix: "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := NewKeyBuilder(tt.environment)
			if kb.GetPrefix() != tt.expectedPrefix {
				t.Errorf("NewKeyBuilder(%s).GetPrefix() = %s, want %s", 
					tt.environment, kb.GetPrefix(), tt.expectedPrefix)
			}
		})
	}
}

func TestKeyBuilder_KeyGeneration(t *testing.T) {
	kb := NewKeyBuilder("production")
	
	tests := []struct {
		name     string
		method   func() string
		expected string
	}{
		{
			name:     "VotingSummary key",
			method:   kb.KeyVoteSummary,
			expected: "prod:voting:summary",
		},
		{
			name:     "UserVoted key",
			method:   func() string { return kb.KeyUserVoted("user123") },
			expected: "prod:voting:user:user123:voted",
		},
		{
			name:     "TeamByID key",
			method:   func() string { return kb.KeyTeamByID(5) },
			expected: "prod:voting:team:5",
		},
		{
			name:     "VisitorTotal key",
			method:   kb.KeyVisitorTotal,
			expected: "prod:visitor:total",
		},
		{
			name:     "VisitorDaily key",
			method:   func() string { return kb.KeyVisitorDaily("2024-01-15") },
			expected: "prod:visitor:daily:2024-01-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method()
			if result != tt.expected {
				t.Errorf("%s = %s, want %s", tt.name, result, tt.expected)
			}
		})
	}
}

func TestKeyBuilder_EnvironmentSeparation(t *testing.T) {
	prodKB := NewKeyBuilder("production")
	stagingKB := NewKeyBuilder("development")
	
	prodKey := prodKB.KeyVoteSummary()
	stagingKey := stagingKB.KeyVoteSummary()
	
	if prodKey == stagingKey {
		t.Errorf("Production and staging keys should be different. Got: prod=%s, staging=%s", 
			prodKey, stagingKey)
	}
	
	expectedProd := "prod:voting:summary"
	expectedStaging := "staging:voting:summary"
	
	if prodKey != expectedProd {
		t.Errorf("Production key = %s, want %s", prodKey, expectedProd)
	}
	
	if stagingKey != expectedStaging {
		t.Errorf("Staging key = %s, want %s", stagingKey, expectedStaging)
	}
}

func TestKeyBuilder_AllVotingKeys(t *testing.T) {
	kb := NewKeyBuilder("staging")
	
	tests := []struct {
		name     string
		method   func() string
		expected string
	}{
		{
			name:     "TeamsAll key",
			method:   kb.KeyTeamsAll,
			expected: "staging:voting:teams:all",
		},
		{
			name:     "TeamByID key",
			method:   func() string { return kb.KeyTeamByID(42) },
			expected: "staging:voting:team:42",
		},
		{
			name:     "TeamCount key",
			method:   func() string { return kb.KeyTeamCount(100) },
			expected: "staging:voting:team:100:count",
		},
		{
			name:     "UserVoted key",
			method:   func() string { return kb.KeyUserVoted("test-user-id") },
			expected: "staging:voting:user:test-user-id:voted",
		},
		{
			name:     "PhoneVoted key",
			method:   func() string { return kb.KeyPhoneVoted("+66891234567") },
			expected: "staging:voting:phone:+66891234567:voted",
		},
		{
			name:     "VoteSummary key",
			method:   kb.KeyVoteSummary,
			expected: "staging:voting:summary",
		},
		{
			name:     "VotingResults key",
			method:   kb.KeyVotingResults,
			expected: "staging:voting:results",
		},
		{
			name:     "LastUpdate key",
			method:   kb.KeyLastUpdate,
			expected: "staging:voting:last_update",
		},
		{
			name:     "ETag key",
			method:   func() string { return kb.KeyETag("etag-123") },
			expected: "staging:voting:etag:etag-123",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method()
			if result != tt.expected {
				t.Errorf("%s = %s, want %s", tt.name, result, tt.expected)
			}
		})
	}
}

func TestKeyBuilder_AllVisitorKeys(t *testing.T) {
	kb := NewKeyBuilder("production")
	
	tests := []struct {
		name     string
		method   func() string
		expected string
	}{
		{
			name:     "VisitorTotal key",
			method:   kb.KeyVisitorTotal,
			expected: "prod:visitor:total",
		},
		{
			name:     "VisitorDaily key",
			method:   func() string { return kb.KeyVisitorDaily("2025-01-09") },
			expected: "prod:visitor:daily:2025-01-09",
		},
		{
			name:     "VisitorUnique key",
			method:   kb.KeyVisitorUnique,
			expected: "prod:visitor:unique",
		},
		{
			name:     "VisitorUniqueDaily key",
			method:   func() string { return kb.KeyVisitorUniqueDaily("2025-01-09") },
			expected: "prod:visitor:unique:daily:2025-01-09",
		},
		{
			name:     "VisitorRateLimit key",
			method:   func() string { return kb.KeyVisitorRateLimit("ip-hash-abc123") },
			expected: "prod:visitor:ratelimit:ip-hash-abc123",
		},
		{
			name:     "VisitorLastUpdate key",
			method:   kb.KeyVisitorLastUpdate,
			expected: "prod:visitor:last_update",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method()
			if result != tt.expected {
				t.Errorf("%s = %s, want %s", tt.name, result, tt.expected)
			}
		})
	}
}

func TestKeyBuilder_OtherServiceKeys(t *testing.T) {
	kb := NewKeyBuilder("development")
	
	tests := []struct {
		name     string
		method   func() string
		expected string
	}{
		{
			name:     "WelcomeAccepted key",
			method:   func() string { return kb.KeyWelcomeAccepted("user-456") },
			expected: "staging:welcome:user:user-456:accepted",
		},
		{
			name:     "SubscriptionCheck key",
			method:   func() string { return kb.KeySubscriptionCheck("user-789", "channel-ABC") },
			expected: "staging:subscription:user-789:channel-ABC",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method()
			if result != tt.expected {
				t.Errorf("%s = %s, want %s", tt.name, result, tt.expected)
			}
		})
	}
}

func TestKeyBuilder_CustomKey(t *testing.T) {
	kb := NewKeyBuilder("production")
	
	tests := []struct {
		name     string
		pattern  string
		args     []interface{}
		expected string
	}{
		{
			name:     "Custom key with no args",
			pattern:  "custom:key",
			args:     nil,
			expected: "prod:custom:key",
		},
		{
			name:     "Custom key with string arg",
			pattern:  "custom:%s:data",
			args:     []interface{}{"test"},
			expected: "prod:custom:test:data",
		},
		{
			name:     "Custom key with multiple args",
			pattern:  "custom:%s:%d:%s",
			args:     []interface{}{"user", 123, "action"},
			expected: "prod:custom:user:123:action",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := kb.KeyCustom(tt.pattern, tt.args...)
			if result != tt.expected {
				t.Errorf("KeyCustom(%s, %v) = %s, want %s", tt.pattern, tt.args, result, tt.expected)
			}
		})
	}
}

func TestKeyBuilder_BuildKey(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		key         string
		expected    string
	}{
		{
			name:        "Production simple key",
			environment: "production",
			key:         "test:key",
			expected:    "prod:test:key",
		},
		{
			name:        "Staging simple key",
			environment: "staging",
			key:         "test:key",
			expected:    "staging:test:key",
		},
		{
			name:        "Development simple key",
			environment: "development",
			key:         "test:key",
			expected:    "staging:test:key",
		},
		{
			name:        "Unknown environment defaults to prod",
			environment: "qa",
			key:         "test:key",
			expected:    "prod:test:key",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb := NewKeyBuilder(tt.environment)
			result := kb.BuildKey(tt.key)
			if result != tt.expected {
				t.Errorf("BuildKey(%s) with env %s = %s, want %s", 
					tt.key, tt.environment, result, tt.expected)
			}
		})
	}
}
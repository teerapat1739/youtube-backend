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
package auth

import (
	"testing"
)

func TestIsGoogleAccessToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "Valid Google access token",
			token:    "ya29.A0AS3H6NexampleGoogleAccessToken",
			expected: true,
		},
		{
			name:     "Invalid token - too short",
			token:    "ya29",
			expected: false,
		},
		{
			name:     "Invalid token - wrong prefix",
			token:    "xa29.A0AS3H6NexampleToken",
			expected: false,
		},
		{
			name:     "JWT token",
			token:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expected: false,
		},
		{
			name:     "Empty token",
			token:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isGoogleAccessToken(tt.token)
			if result != tt.expected {
				t.Errorf("isGoogleAccessToken(%s) = %v, want %v", tt.token, result, tt.expected)
			}
		})
	}
}

func TestIsJWTToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "Valid JWT token",
			token:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expected: true,
		},
		{
			name:     "Google access token",
			token:    "ya29.A0AS3H6NexampleGoogleAccessToken",
			expected: false,
		},
		{
			name:     "Token with too few segments",
			token:    "header.payload",
			expected: false,
		},
		{
			name:     "Token with too many segments",
			token:    "header.payload.signature.extra",
			expected: false,
		},
		{
			name:     "Token with no segments",
			token:    "nosegments",
			expected: false,
		},
		{
			name:     "Empty token",
			token:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isJWTToken(tt.token)
			if result != tt.expected {
				t.Errorf("isJWTToken(%s) = %v, want %v", tt.token, result, tt.expected)
			}
		})
	}
}

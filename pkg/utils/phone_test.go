package utils

import (
	"testing"
)

func TestNormalizePhoneNumber(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		shouldError bool
	}{
		{
			name:     "formatted Thai mobile",
			input:    "090-930-0861",
			expected: "0909300861",
		},
		{
			name:     "unformatted Thai mobile",
			input:    "0891234567",
			expected: "0891234567",
		},
		{
			name:     "international format +66",
			input:    "+66909300861",
			expected: "0909300861",
		},
		{
			name:     "international format 66",
			input:    "66909300861",
			expected: "0909300861",
		},
		{
			name:     "with spaces",
			input:    "090 930 0861",
			expected: "0909300861",
		},
		{
			name:     "with parentheses",
			input:    "(090) 930-0861",
			expected: "0909300861",
		},
		{
			name:        "too short",
			input:       "090930",
			expected:    "",
			shouldError: true,
		},
		{
			name:        "too long",
			input:       "090930086123",
			expected:    "",
			shouldError: true,
		},
		{
			name:        "doesn't start with 0",
			input:       "190930086",
			expected:    "",
			shouldError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expected:    "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizePhoneNumber(tt.input)
			
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestValidateThaiPhoneNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid mobile 09",
			input:    "0909300861",
			expected: true,
		},
		{
			name:     "valid mobile 08",
			input:    "0891234567",
			expected: true,
		},
		{
			name:     "valid mobile 06",
			input:    "0651234567",
			expected: true,
		},
		{
			name:     "formatted valid mobile",
			input:    "090-930-0861",
			expected: true,
		},
		{
			name:     "landline 02 (invalid for voting)",
			input:    "0212345678",
			expected: false,
		},
		{
			name:     "landline 034 (invalid for voting)",
			input:    "0341234567",
			expected: false,
		},
		{
			name:     "international format",
			input:    "+66909300861",
			expected: true,
		},
		{
			name:     "invalid format",
			input:    "invalid-phone",
			expected: false,
		},
		{
			name:     "too short",
			input:    "090930",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateThaiPhoneNumber(tt.input)
			if result != tt.expected {
				t.Errorf("expected %t, got %t for input %s", tt.expected, result, tt.input)
			}
		})
	}
}

func TestFormatPhoneNumberForDisplay(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid 10 digit number",
			input:    "0909300861",
			expected: "090-930-0861",
		},
		{
			name:     "valid 10 digit number 08",
			input:    "0891234567",
			expected: "089-123-4567",
		},
		{
			name:     "invalid length - too short",
			input:    "090930",
			expected: "090930", // returns as-is
		},
		{
			name:     "invalid length - too long",
			input:    "090930086123",
			expected: "090930086123", // returns as-is
		},
		{
			name:     "empty string",
			input:    "",
			expected: "", // returns as-is
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatPhoneNumberForDisplay(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
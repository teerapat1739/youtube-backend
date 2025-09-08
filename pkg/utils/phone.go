package utils

import (
	"errors"
	"regexp"
	"strings"
)

var (
	// Regex to match and extract digits from Thai phone numbers
	phoneRegex = regexp.MustCompile(`^0[0-9]{8,9}$`)
	// Regex to remove non-digit characters
	digitsOnlyRegex = regexp.MustCompile(`[^0-9]`)
)

// NormalizePhoneNumber normalizes a phone number by removing all non-digit characters
// and ensures it follows Thai phone number format
func NormalizePhoneNumber(phone string) (string, error) {
	if phone == "" {
		return "", errors.New("phone number cannot be empty")
	}

	// Remove all non-digit characters (hyphens, spaces, parentheses, etc.)
	normalized := digitsOnlyRegex.ReplaceAllString(phone, "")

	// Handle international format (+66)
	if strings.HasPrefix(normalized, "66") && len(normalized) >= 10 {
		normalized = "0" + normalized[2:] // Convert +66XXXXXXXXX to 0XXXXXXXXX
	}

	// Validate Thai phone number format
	if !phoneRegex.MatchString(normalized) {
		return "", errors.New("invalid Thai phone number format")
	}

	return normalized, nil
}

// FormatPhoneNumberForDisplay formats a normalized phone number for display
// Example: "0909300861" -> "090-930-0861"
func FormatPhoneNumberForDisplay(phone string) string {
	if len(phone) != 10 {
		return phone // Return as-is if not a valid 10-digit number
	}

	// Format as XXX-XXX-XXXX
	return phone[:3] + "-" + phone[3:6] + "-" + phone[6:]
}

// ValidateThaiPhoneNumber validates if a phone number is a valid Thai mobile number
func ValidateThaiPhoneNumber(phone string) bool {
	normalized, err := NormalizePhoneNumber(phone)
	if err != nil {
		return false
	}

	// Check if it's a valid Thai mobile number (starts with 06, 08, 09)
	if len(normalized) == 10 {
		prefix := normalized[:2]
		return prefix == "06" || prefix == "08" || prefix == "09"
	}

	return false
}
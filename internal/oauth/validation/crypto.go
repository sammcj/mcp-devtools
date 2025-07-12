package validation

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

// generateCodeVerifier generates a cryptographically secure PKCE code verifier
// According to RFC7636, it should be 43-128 characters long
func generateCodeVerifier() (string, error) {
	// Generate 32 random bytes (will result in 43 characters when base64url encoded)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode using base64url without padding
	verifier := base64.RawURLEncoding.EncodeToString(bytes)
	
	// Ensure it meets RFC7636 requirements (43-128 unreserved characters)
	if len(verifier) < 43 || len(verifier) > 128 {
		return "", fmt.Errorf("generated verifier length %d is outside valid range", len(verifier))
	}

	return verifier, nil
}

// base64URLDecode decodes a base64url encoded string
func base64URLDecode(s string) ([]byte, error) {
	// Add padding if necessary
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	
	// Replace URL-safe characters with standard base64 characters
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	
	return base64.StdEncoding.DecodeString(s)
}

// generateClientSecret generates a cryptographically secure client secret
func generateClientSecret() (string, error) {
	// Generate 32 random bytes for a strong client secret
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode as base64url without padding for URL safety
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// generateClientID generates a unique client ID
func generateClientID() (string, error) {
	// Generate 16 random bytes for client ID
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode as base64url without padding
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// GenerateClientID generates a unique client ID
func GenerateClientID() (string, error) {
	return generateClientID()
}

// GenerateClientSecret generates a cryptographically secure client secret
func GenerateClientSecret() (string, error) {
	return generateClientSecret()
}

// isValidCodeVerifier validates a PKCE code verifier according to RFC7636
func isValidCodeVerifier(verifier string) bool {
	// Check length (43-128 characters)
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}

	// Check that it only contains unreserved characters
	// unreserved = ALPHA / DIGIT / "-" / "." / "_" / "~"
	for _, char := range verifier {
		if !((char >= 'A' && char <= 'Z') || 
			 (char >= 'a' && char <= 'z') || 
			 (char >= '0' && char <= '9') || 
			 char == '-' || char == '.' || char == '_' || char == '~') {
			return false
		}
	}

	return true
}
package client

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/sammcj/mcp-devtools/internal/oauth/types"
)

// GeneratePKCEChallenge generates a PKCE code challenge and verifier according to RFC7636
func GeneratePKCEChallenge() (*types.PKCEChallenge, error) {
	// Generate a cryptographically random code verifier
	// RFC7636 recommends 43-128 characters, we'll use 64 for good entropy
	verifierBytes := make([]byte, 48) // 48 bytes = 64 base64url characters
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	// Encode as base64url without padding
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	// Generate code challenge using S256 method (SHA256)
	challengeBytes := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])

	return &types.PKCEChallenge{
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: "S256",
		CodeVerifier:        codeVerifier,
		CreatedAt:           time.Now(),
	}, nil
}

// ValidatePKCEChallenge validates a PKCE challenge against a verifier
func ValidatePKCEChallenge(challenge *types.PKCEChallenge, verifier string) error {
	if challenge == nil {
		return fmt.Errorf("PKCE challenge is nil")
	}

	if challenge.CodeVerifier != verifier {
		return fmt.Errorf("code verifier does not match")
	}

	// Verify the challenge was created correctly
	switch challenge.CodeChallengeMethod {
	case "S256":
		challengeBytes := sha256.Sum256([]byte(verifier))
		expectedChallenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])
		if challenge.CodeChallenge != expectedChallenge {
			return fmt.Errorf("code challenge does not match verifier")
		}
	case "plain":
		// Plain method is discouraged but supported
		if challenge.CodeChallenge != verifier {
			return fmt.Errorf("code challenge does not match verifier (plain method)")
		}
	default:
		return fmt.Errorf("unsupported code challenge method: %s", challenge.CodeChallengeMethod)
	}

	// Check if challenge is not too old (optional security measure)
	if time.Since(challenge.CreatedAt) > 10*time.Minute {
		return fmt.Errorf("PKCE challenge has expired")
	}

	return nil
}

// GenerateState generates a cryptographically secure state parameter for OAuth flow
func GenerateState() (string, error) {
	stateBytes := make([]byte, 32) // 32 bytes = 256 bits of entropy
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("failed to generate state parameter: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(stateBytes), nil
}

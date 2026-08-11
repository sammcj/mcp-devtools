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

// GenerateState generates a cryptographically secure state parameter for OAuth flow
func GenerateState() (string, error) {
	stateBytes := make([]byte, 32) // 32 bytes = 256 bits of entropy
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("failed to generate state parameter: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(stateBytes), nil
}

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewState generates an unguessable state parameter, which binds an
// authorisation response to the request this process made.
func NewState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// PKCE holds the code verifier and challenge for OAuth PKCE flow.
type PKCE struct {
	Verifier  string
	Challenge string
	Method    string
}

// NewPKCE generates a new PKCE code verifier and challenge.
func NewPKCE() (*PKCE, error) {
	// Generate 32 bytes of random data
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, err
	}

	// Base64 URL encode the verifier
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	// Create SHA256 hash of verifier for challenge
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &PKCE{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}, nil
}

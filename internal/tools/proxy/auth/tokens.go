package auth

import (
	"time"
)

// Tokens holds OAuth tokens.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresIn    int       `json:"expires_in,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope,omitempty"`
}

// IsExpired returns true if the access token is expired.
func (t *Tokens) IsExpired() bool {
	if t.ExpiresAt.IsZero() {
		return false
	}
	// Consider expired 30 seconds before actual expiry
	return time.Now().Add(30 * time.Second).After(t.ExpiresAt)
}

// SaveTokens persists tokens to the cache directory.
func SaveTokens(cacheDir, serverHash string, tokens *Tokens) error {
	return WriteJSON(cacheDir, serverHash, "tokens.json", tokens)
}

// LoadTokens loads tokens from the cache directory.
func LoadTokens(cacheDir, serverHash string) (*Tokens, error) {
	var tokens Tokens
	if err := ReadJSON(cacheDir, serverHash, "tokens.json", &tokens); err != nil {
		return nil, err
	}
	return &tokens, nil
}

// DeleteTokens removes stored tokens.
func DeleteTokens(cacheDir, serverHash string) error {
	return DeleteFile(cacheDir, serverHash, "tokens.json")
}

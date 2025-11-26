package tools

import (
	"testing"
	"time"

	"github.com/sammcj/mcp-devtools/internal/tools/proxy/auth"
)

func TestNewPKCE(t *testing.T) {
	pkce, err := auth.NewPKCE()
	if err != nil {
		t.Fatalf("NewPKCE failed: %v", err)
	}

	if pkce.Verifier == "" {
		t.Error("verifier is empty")
	}

	if pkce.Challenge == "" {
		t.Error("challenge is empty")
	}

	if pkce.Method != "S256" {
		t.Errorf("unexpected method: %s (expected S256)", pkce.Method)
	}

	// Verifier and challenge should be different
	if pkce.Verifier == pkce.Challenge {
		t.Error("verifier and challenge should be different")
	}

	// Each call should produce different values
	pkce2, err := auth.NewPKCE()
	if err != nil {
		t.Fatalf("second NewPKCE failed: %v", err)
	}

	if pkce.Verifier == pkce2.Verifier {
		t.Error("multiple PKCE calls produced same verifier")
	}
}

func TestTokens_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		tokens   *auth.Tokens
		expected bool
	}{
		{
			name: "not expired",
			tokens: &auth.Tokens{
				ExpiresAt: time.Now().Add(5 * time.Minute),
			},
			expected: false,
		},
		{
			name: "expired",
			tokens: &auth.Tokens{
				ExpiresAt: time.Now().Add(-5 * time.Minute),
			},
			expected: true,
		},
		{
			name: "expiring soon (within 30s)",
			tokens: &auth.Tokens{
				ExpiresAt: time.Now().Add(15 * time.Second),
			},
			expected: true, // Consider expired 30s before actual expiry
		},
		{
			name: "no expiry set",
			tokens: &auth.Tokens{
				ExpiresAt: time.Time{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tokens.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerMetadata_ValidateScopes(t *testing.T) {
	metadata := &auth.ServerMetadata{
		ScopesSupported: []string{"read", "write", "admin"},
	}

	tests := []struct {
		name      string
		requested []string
		expected  []string
	}{
		{
			name:      "all valid",
			requested: []string{"read", "write"},
			expected:  []string{"read", "write"},
		},
		{
			name:      "some invalid",
			requested: []string{"read", "invalid", "write"},
			expected:  []string{"read", "write"},
		},
		{
			name:      "all invalid",
			requested: []string{"invalid1", "invalid2"},
			expected:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := metadata.ValidateScopes(tt.requested)
			if len(got) != len(tt.expected) {
				t.Errorf("ValidateScopes() returned %d scopes, want %d", len(got), len(tt.expected))
			}
		})
	}
}

func TestServerMetadata_SupportsPKCE(t *testing.T) {
	tests := []struct {
		name     string
		metadata *auth.ServerMetadata
		expected bool
	}{
		{
			name: "explicitly supports S256",
			metadata: &auth.ServerMetadata{
				CodeChallengeMethodsSupported: []string{"S256"},
			},
			expected: true,
		},
		{
			name: "supports s256 (lowercase)",
			metadata: &auth.ServerMetadata{
				CodeChallengeMethodsSupported: []string{"s256"},
			},
			expected: true,
		},
		{
			name: "only supports plain",
			metadata: &auth.ServerMetadata{
				CodeChallengeMethodsSupported: []string{"plain"},
			},
			expected: false,
		},
		{
			name: "no methods specified (assume supported)",
			metadata: &auth.ServerMetadata{
				CodeChallengeMethodsSupported: []string{},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.metadata.SupportsPKCE(); got != tt.expected {
				t.Errorf("SupportsPKCE() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTokenStorage(t *testing.T) {
	tmpDir := t.TempDir()
	serverHash := "test-hash"

	tokens := &auth.Tokens{
		AccessToken:  "access-token-123",
		TokenType:    "Bearer",
		RefreshToken: "refresh-token-456",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour),
		Scope:        "read write",
	}

	// Save tokens
	err := auth.SaveTokens(tmpDir, serverHash, tokens)
	if err != nil {
		t.Fatalf("SaveTokens failed: %v", err)
	}

	// Load tokens
	loaded, err := auth.LoadTokens(tmpDir, serverHash)
	if err != nil {
		t.Fatalf("LoadTokens failed: %v", err)
	}

	if loaded.AccessToken != tokens.AccessToken {
		t.Errorf("access token mismatch: got %s, want %s", loaded.AccessToken, tokens.AccessToken)
	}

	if loaded.RefreshToken != tokens.RefreshToken {
		t.Errorf("refresh token mismatch: got %s, want %s", loaded.RefreshToken, tokens.RefreshToken)
	}

	// Delete tokens
	err = auth.DeleteTokens(tmpDir, serverHash)
	if err != nil {
		t.Fatalf("DeleteTokens failed: %v", err)
	}

	// Verify deletion
	_, err = auth.LoadTokens(tmpDir, serverHash)
	if err == nil {
		t.Error("expected error loading deleted tokens, got nil")
	}
}

func TestClientInfoStorage(t *testing.T) {
	tmpDir := t.TempDir()
	serverHash := "test-hash"

	clientInfo := &auth.ClientInfo{
		ClientID:                "client-123",
		ClientSecret:            "secret-456",
		RedirectURIs:            []string{"http://localhost:3334/callback"},
		TokenEndpointAuthMethod: "none",
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		ClientName:              "Test Client",
		Scope:                   "read write",
	}

	// Save client info
	err := auth.SaveClientInfo(tmpDir, serverHash, clientInfo)
	if err != nil {
		t.Fatalf("SaveClientInfo failed: %v", err)
	}

	// Load client info
	loaded, err := auth.LoadClientInfo(tmpDir, serverHash)
	if err != nil {
		t.Fatalf("LoadClientInfo failed: %v", err)
	}

	if loaded.ClientID != clientInfo.ClientID {
		t.Errorf("client ID mismatch: got %s, want %s", loaded.ClientID, clientInfo.ClientID)
	}

	if loaded.ClientSecret != clientInfo.ClientSecret {
		t.Errorf("client secret mismatch: got %s, want %s", loaded.ClientSecret, clientInfo.ClientSecret)
	}

	// Delete client info
	err = auth.DeleteClientInfo(tmpDir, serverHash)
	if err != nil {
		t.Fatalf("DeleteClientInfo failed: %v", err)
	}

	// Verify deletion
	_, err = auth.LoadClientInfo(tmpDir, serverHash)
	if err == nil {
		t.Error("expected error loading deleted client info, got nil")
	}
}

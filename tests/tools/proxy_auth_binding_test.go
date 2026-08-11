package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sammcj/mcp-devtools/internal/tools/proxy/auth"
)

// metadataServer serves an authorisation server metadata document declaring
// itself as the issuer, which is what validateIssuerMatchesURL requires.
func metadataServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
		}); err != nil {
			t.Errorf("failed to encode metadata: %v", err)
		}
	})

	return srv
}

func provider(t *testing.T, serverURL string, tokens *auth.Tokens) *auth.Provider {
	t.Helper()

	cacheDir := t.TempDir()
	const hash = "testhash"
	if err := auth.SaveTokens(cacheDir, hash, tokens); err != nil {
		t.Fatalf("failed to save tokens: %v", err)
	}

	return auth.NewProvider(&auth.ProviderConfig{
		ServerURL:  serverURL,
		ServerHash: hash,
		CacheDir:   cacheDir,
		ClientName: "test",
	})
}

// An unexpired access token cached against one issuer must not be used once the
// upstream reports a different one. Initialise previously returned as soon as it
// found an unexpired token, so the issuer was never compared.
func TestInitialiseRejectsCachedTokenFromAnotherIssuer(t *testing.T) {
	srv := metadataServer(t)

	p := provider(t, srv.URL, &auth.Tokens{
		AccessToken: "stale-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
		Issuer:      "https://a-different-issuer.example.com",
	})

	if err := p.Initialise(t.Context()); err == nil {
		t.Fatal("Initialise accepted a token bound to a different issuer")
	}

	if _, err := p.GetAccessToken(t.Context()); err == nil {
		t.Fatal("GetAccessToken handed out a token bound to a different issuer")
	}
}

func TestInitialiseAcceptsCachedTokenFromTheSameIssuer(t *testing.T) {
	srv := metadataServer(t)

	p := provider(t, srv.URL, &auth.Tokens{
		AccessToken: "good-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
		Issuer:      srv.URL,
	})

	if err := p.Initialise(t.Context()); err != nil {
		t.Fatalf("Initialise rejected a token from the current issuer: %v", err)
	}

	token, err := p.GetAccessToken(t.Context())
	if err != nil {
		t.Fatalf("GetAccessToken failed: %v", err)
	}
	if token != "good-token" {
		t.Errorf("unexpected token: %q", token)
	}
}

// A token cached before issuer binding existed records no issuer. Those are
// accepted so an upgrade does not force everyone to reauthenticate, and the
// grace must not cost a discovery round trip: the URL here refuses connections.
func TestInitialiseAcceptsCachedTokenWithNoRecordedIssuer(t *testing.T) {
	p := provider(t, "http://127.0.0.1:1/mcp", &auth.Tokens{
		AccessToken: "legacy-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	})

	if err := p.Initialise(t.Context()); err != nil {
		t.Fatalf("Initialise rejected a pre-binding token: %v", err)
	}
}

// An authorisation server outage says nothing about who issued a cached token,
// so discovery failing must not break an upstream that is otherwise working.
func TestInitialiseKeepsCachedTokenWhenDiscoveryFails(t *testing.T) {
	p := provider(t, "http://127.0.0.1:1/mcp", &auth.Tokens{
		AccessToken: "cached-token",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
		Issuer:      "https://issuer.example.com",
	})

	if err := p.Initialise(t.Context()); err != nil {
		t.Fatalf("Initialise failed when discovery was unreachable: %v", err)
	}
}

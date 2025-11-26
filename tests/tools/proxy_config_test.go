package tools

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/proxy"
	"github.com/sammcj/mcp-devtools/internal/tools/proxy/types"
)

func TestParseConfig_JSONArray(t *testing.T) {
	// Set up test environment
	upstreams := []types.UpstreamConfig{
		{
			Name:      "server1",
			URL:       "https://api.example.com/mcp",
			Transport: "http-first",
			OAuth: &types.OAuthConfig{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
			},
			Headers: map[string]string{
				"X-Custom-Header": "value",
			},
			IgnoreTools: []string{"dangerous_*", "admin_*"},
		},
		{
			Name:      "server2",
			URL:       "https://api2.example.com/sse",
			Transport: "sse-first",
		},
	}

	upstreamsJSON, err := json.Marshal(upstreams)
	if err != nil {
		t.Fatalf("failed to marshal test upstreams: %v", err)
	}

	os.Setenv("PROXY_UPSTREAMS", string(upstreamsJSON))
	defer os.Unsetenv("PROXY_UPSTREAMS")

	config, err := proxy.ParseConfig()
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	if len(config.Upstreams) != 2 {
		t.Errorf("expected 2 upstreams, got %d", len(config.Upstreams))
	}

	// Check first upstream
	if config.Upstreams[0].Name != "server1" {
		t.Errorf("expected name 'server1', got %s", config.Upstreams[0].Name)
	}
	if config.Upstreams[0].URL != "https://api.example.com/mcp" {
		t.Errorf("unexpected URL: %s", config.Upstreams[0].URL)
	}
	if config.Upstreams[0].Transport != "http-first" {
		t.Errorf("unexpected transport: %s", config.Upstreams[0].Transport)
	}
	if config.Upstreams[0].OAuth == nil || config.Upstreams[0].OAuth.ClientID != "test-client" {
		t.Error("OAuth config not parsed correctly")
	}
	if len(config.Upstreams[0].IgnoreTools) != 2 {
		t.Errorf("expected 2 ignore tools, got %d", len(config.Upstreams[0].IgnoreTools))
	}

	// Check second upstream
	if config.Upstreams[1].Name != "server2" {
		t.Errorf("expected name 'server2', got %s", config.Upstreams[1].Name)
	}
}

func TestParseConfig_SimplifiedFormat(t *testing.T) {
	os.Setenv("PROXY_URL", "https://mcp.example.com/sse")
	os.Setenv("PROXY_TRANSPORT", "sse-only")
	os.Setenv("PROXY_IGNORE_TOOLS", "dangerous_*, admin_*")
	defer func() {
		os.Unsetenv("PROXY_URL")
		os.Unsetenv("PROXY_TRANSPORT")
		os.Unsetenv("PROXY_IGNORE_TOOLS")
	}()

	config, err := proxy.ParseConfig()
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}

	if len(config.Upstreams) != 1 {
		t.Errorf("expected 1 upstream, got %d", len(config.Upstreams))
	}

	upstream := config.Upstreams[0]
	if upstream.Name != "default" {
		t.Errorf("expected name 'default', got %s", upstream.Name)
	}
	if upstream.URL != "https://mcp.example.com/sse" {
		t.Errorf("unexpected URL: %s", upstream.URL)
	}
	if upstream.Transport != "sse-only" {
		t.Errorf("unexpected transport: %s", upstream.Transport)
	}
	if len(upstream.IgnoreTools) != 2 {
		t.Errorf("expected 2 ignore tools, got %d", len(upstream.IgnoreTools))
	}
}

func TestParseConfig_NoConfig(t *testing.T) {
	// Ensure no proxy env vars are set
	os.Unsetenv("PROXY_UPSTREAMS")
	os.Unsetenv("PROXY_URL")

	_, err := proxy.ParseConfig()
	if err == nil {
		t.Error("expected error when no configuration is provided")
	}
}

func TestValidate_DuplicateNames(t *testing.T) {
	config := &types.ProxyConfig{
		Upstreams: []types.UpstreamConfig{
			{Name: "test", URL: "https://example.com/mcp"},
			{Name: "test", URL: "https://example2.com/mcp"},
		},
		CallbackPort: 3334,
	}

	err := config.Validate()
	if err == nil {
		t.Error("expected error for duplicate upstream names")
	}
}

func TestValidate_InvalidURL(t *testing.T) {
	config := &types.ProxyConfig{
		Upstreams: []types.UpstreamConfig{
			{Name: "test", URL: "not-a-url"},
		},
		CallbackPort: 3334,
	}

	err := config.Validate()
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestValidate_InvalidTransport(t *testing.T) {
	config := &types.ProxyConfig{
		Upstreams: []types.UpstreamConfig{
			{Name: "test", URL: "https://example.com/mcp", Transport: "invalid"},
		},
		CallbackPort: 3334,
	}

	err := config.Validate()
	if err == nil {
		t.Error("expected error for invalid transport strategy")
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	config := &types.ProxyConfig{
		Upstreams: []types.UpstreamConfig{
			{Name: "test", URL: "https://example.com/mcp"},
		},
		CallbackPort: 99999,
	}

	err := config.Validate()
	if err == nil {
		t.Error("expected error for invalid callback port")
	}
}

func TestUpstreamHash(t *testing.T) {
	upstream1 := &types.UpstreamConfig{
		URL: "https://example.com/mcp",
	}

	upstream2 := &types.UpstreamConfig{
		URL: "https://example.com/mcp",
	}

	upstream3 := &types.UpstreamConfig{
		URL:     "https://example.com/mcp",
		Headers: map[string]string{"X-Custom": "value"},
	}

	hash1 := types.UpstreamHash(upstream1)
	hash2 := types.UpstreamHash(upstream2)
	hash3 := types.UpstreamHash(upstream3)

	// Same config should produce same hash
	if hash1 != hash2 {
		t.Error("same upstreams produced different hashes")
	}

	// Different config should produce different hash
	if hash1 == hash3 {
		t.Error("different upstreams produced same hash")
	}

	// Hash should be a valid hex string
	if len(hash1) != 32 {
		t.Errorf("unexpected hash length: %d (expected 32)", len(hash1))
	}
}

func TestEnsureCacheDir(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := tmpDir + "/test-cache"

	err := proxy.EnsureCacheDir(cacheDir)
	if err != nil {
		t.Fatalf("EnsureCacheDir failed: %v", err)
	}

	// Check directory was created
	info, err := os.Stat(cacheDir)
	if err != nil {
		t.Fatalf("cache dir not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("cache path is not a directory")
	}

	// Check permissions
	if info.Mode().Perm() != 0700 {
		t.Errorf("unexpected permissions: %v (expected 0700)", info.Mode().Perm())
	}
}

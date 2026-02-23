package tools

import (
	"context"
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sammcj/mcp-devtools/internal/tools/proxy"
)

func TestRegisterUpstreamTools_NotEnabled(t *testing.T) {
	// Save original env vars
	origEnabled := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	origUpstreams := os.Getenv("PROXY_UPSTREAMS")
	defer func() {
		if origEnabled != "" {
			os.Setenv("ENABLE_ADDITIONAL_TOOLS", origEnabled)
		} else {
			os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		}
		if origUpstreams != "" {
			os.Setenv("PROXY_UPSTREAMS", origUpstreams)
		} else {
			os.Unsetenv("PROXY_UPSTREAMS")
		}
	}()

	// Set up test: proxy configured but NOT enabled
	os.Setenv("PROXY_UPSTREAMS", "[{\"name\":\"test\",\"url\":\"https://example.com/mcp\"}]")
	os.Setenv("ENABLE_ADDITIONAL_TOOLS", "github,search_packages") // Proxy not included
	tools.ResetEnabledToolsCache()

	ctx := context.Background()
	result := proxy.RegisterUpstreamTools(ctx, true)

	if result {
		t.Error("RegisterUpstreamTools should return false when proxy is not enabled")
	}
}

func TestRegisterUpstreamTools_EnabledButNotConfigured(t *testing.T) {
	// Save original env vars
	origEnabled := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	origUpstreams := os.Getenv("PROXY_UPSTREAMS")
	origURL := os.Getenv("PROXY_URL")
	defer func() {
		if origEnabled != "" {
			os.Setenv("ENABLE_ADDITIONAL_TOOLS", origEnabled)
		} else {
			os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		}
		if origUpstreams != "" {
			os.Setenv("PROXY_UPSTREAMS", origUpstreams)
		} else {
			os.Unsetenv("PROXY_UPSTREAMS")
		}
		if origURL != "" {
			os.Setenv("PROXY_URL", origURL)
		} else {
			os.Unsetenv("PROXY_URL")
		}
	}()

	// Set up test: proxy enabled but NOT configured
	os.Unsetenv("PROXY_UPSTREAMS")
	os.Unsetenv("PROXY_URL")
	os.Setenv("ENABLE_ADDITIONAL_TOOLS", "proxy")
	tools.ResetEnabledToolsCache()

	ctx := context.Background()
	result := proxy.RegisterUpstreamTools(ctx, true)

	if result {
		t.Error("RegisterUpstreamTools should return false when proxy is not configured")
	}
}

func TestRegisterUpstreamTools_EnabledViaAll(t *testing.T) {
	// Save original env vars
	origEnabled := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	origUpstreams := os.Getenv("PROXY_UPSTREAMS")
	defer func() {
		if origEnabled != "" {
			os.Setenv("ENABLE_ADDITIONAL_TOOLS", origEnabled)
		} else {
			os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		}
		if origUpstreams != "" {
			os.Setenv("PROXY_UPSTREAMS", origUpstreams)
		} else {
			os.Unsetenv("PROXY_UPSTREAMS")
		}
	}()

	// Set up test: proxy enabled via "all"
	os.Setenv("PROXY_UPSTREAMS", "[{\"name\":\"test\",\"url\":\"https://example.com/mcp\"}]")
	os.Setenv("ENABLE_ADDITIONAL_TOOLS", "all")
	tools.ResetEnabledToolsCache()

	ctx := context.Background()
	// This will fail because we can't actually connect, but it should pass the enablement check
	// and return false due to connection failure, not enablement check
	result := proxy.RegisterUpstreamTools(ctx, true)

	// We expect false here because the connection will fail (no real upstream),
	// but it should have passed the enablement check
	if result {
		t.Error("RegisterUpstreamTools should return false when connection fails (but enablement check should pass)")
	}
}

func TestRegisterUpstreamTools_EnabledExplicitly(t *testing.T) {
	// Save original env vars
	origEnabled := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	origUpstreams := os.Getenv("PROXY_UPSTREAMS")
	defer func() {
		if origEnabled != "" {
			os.Setenv("ENABLE_ADDITIONAL_TOOLS", origEnabled)
		} else {
			os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		}
		if origUpstreams != "" {
			os.Setenv("PROXY_UPSTREAMS", origUpstreams)
		} else {
			os.Unsetenv("PROXY_UPSTREAMS")
		}
	}()

	// Set up test: proxy explicitly enabled
	os.Setenv("PROXY_UPSTREAMS", "[{\"name\":\"test\",\"url\":\"https://example.com/mcp\"}]")
	os.Setenv("ENABLE_ADDITIONAL_TOOLS", "github,proxy,search_packages")
	tools.ResetEnabledToolsCache()

	ctx := context.Background()
	// This will fail because we can't actually connect, but it should pass the enablement check
	result := proxy.RegisterUpstreamTools(ctx, true)

	// We expect false here because the connection will fail (no real upstream),
	// but it should have passed the enablement check
	if result {
		t.Error("RegisterUpstreamTools should return false when connection fails (but enablement check should pass)")
	}
}

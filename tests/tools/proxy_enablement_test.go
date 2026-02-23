package tools

import (
	"context"
	"os"
	"testing"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sammcj/mcp-devtools/internal/tools/proxy"
	"github.com/sirupsen/logrus"
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

// saveProxyEnv saves the proxy-related environment variables and returns a restore function.
func saveProxyEnv(t *testing.T) func() {
	t.Helper()
	keys := []string{"ENABLE_ADDITIONAL_TOOLS", "PROXY_UPSTREAMS", "PROXY_URL"}
	saved := make(map[string]string, len(keys))
	present := make(map[string]bool, len(keys))
	for _, k := range keys {
		v, ok := os.LookupEnv(k)
		saved[k] = v
		present[k] = ok
	}
	return func() {
		for _, k := range keys {
			if present[k] {
				os.Setenv(k, saved[k])
			} else {
				os.Unsetenv(k)
			}
		}
	}
}

func TestRegisterUpstreamToolsAsync_NotEnabled(t *testing.T) {
	restore := saveProxyEnv(t)
	defer restore()

	os.Setenv("PROXY_UPSTREAMS", `[{"name":"test","url":"https://example.com/mcp"}]`)
	os.Setenv("ENABLE_ADDITIONAL_TOOLS", "github,search_packages")
	tools.ResetEnabledToolsCache()

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	srv := mcpserver.NewMCPServer("test", "1.0")

	// Should return immediately without spawning a goroutine
	proxy.RegisterUpstreamToolsAsync(t.Context(), srv, logger, "stdio")

	// Give a brief window for any goroutine to run (it shouldn't)
	time.Sleep(50 * time.Millisecond)
}

func TestRegisterUpstreamToolsAsync_EnabledButNotConfigured(t *testing.T) {
	restore := saveProxyEnv(t)
	defer restore()

	os.Unsetenv("PROXY_UPSTREAMS")
	os.Unsetenv("PROXY_URL")
	os.Setenv("ENABLE_ADDITIONAL_TOOLS", "proxy")
	tools.ResetEnabledToolsCache()

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	srv := mcpserver.NewMCPServer("test", "1.0")

	// Should return immediately without spawning a goroutine
	proxy.RegisterUpstreamToolsAsync(t.Context(), srv, logger, "stdio")

	time.Sleep(50 * time.Millisecond)
}

func TestRegisterUpstreamToolsAsync_EnabledAndConfigured(t *testing.T) {
	restore := saveProxyEnv(t)
	defer restore()

	// Proxy enabled and configured, but upstream is unreachable
	os.Setenv("PROXY_UPSTREAMS", `[{"name":"test","url":"https://192.0.2.1/mcp"}]`)
	os.Setenv("ENABLE_ADDITIONAL_TOOLS", "proxy")
	tools.ResetEnabledToolsCache()

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	srv := mcpserver.NewMCPServer("test", "1.0")

	// Use a context with a short deadline so the goroutine doesn't wait 5 minutes
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	proxy.RegisterUpstreamToolsAsync(ctx, srv, logger, "stdio")

	// Wait for the goroutine to hit the timeout and exit
	time.Sleep(3 * time.Second)
}

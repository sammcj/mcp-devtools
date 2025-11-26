package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sammcj/mcp-devtools/internal/tools/proxy/types"
)

// ParseConfig parses proxy configuration from environment variables.
func ParseConfig() (*types.ProxyConfig, error) {
	config := &types.ProxyConfig{
		CallbackHost: getEnvOrDefault("PROXY_OAUTH_CALLBACK_HOST", "localhost"),
		CallbackPort: getEnvIntOrDefault("PROXY_OAUTH_CALLBACK_PORT", 3334),
		CacheDir:     getEnvOrDefault("PROXY_CACHE_DIR", ""),
	}

	// Set default cache dir if not specified
	if config.CacheDir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user cache dir: %w", err)
		}
		config.CacheDir = filepath.Join(cacheDir, "mcp-devtools", "proxy")
	} else {
		// Expand tilde in user-provided path
		if strings.HasPrefix(config.CacheDir, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get home dir: %w", err)
			}
			config.CacheDir = filepath.Join(home, config.CacheDir[2:])
		}
	}

	// Try to parse PROXY_UPSTREAMS JSON array first
	upstreamsJSON := os.Getenv("PROXY_UPSTREAMS")
	if upstreamsJSON != "" {
		if err := json.Unmarshal([]byte(upstreamsJSON), &config.Upstreams); err != nil {
			return nil, fmt.Errorf("failed to parse PROXY_UPSTREAMS: %w", err)
		}
	} else {
		// Fall back to simplified single-upstream format
		proxyURL := os.Getenv("PROXY_URL")
		if proxyURL == "" {
			return nil, fmt.Errorf("no proxy configuration found: set PROXY_UPSTREAMS or PROXY_URL")
		}

		upstream := types.UpstreamConfig{
			Name:      "default",
			URL:       proxyURL,
			Transport: getEnvOrDefault("PROXY_TRANSPORT", "http-first"),
		}

		// Parse include tools
		includeTools := os.Getenv("PROXY_INCLUDE_TOOLS")
		if includeTools != "" {
			upstream.IncludeTools = strings.Split(includeTools, ",")
			for i := range upstream.IncludeTools {
				upstream.IncludeTools[i] = strings.TrimSpace(upstream.IncludeTools[i])
			}
		}

		// Parse ignore tools
		ignoreTools := os.Getenv("PROXY_IGNORE_TOOLS")
		if ignoreTools != "" {
			upstream.IgnoreTools = strings.Split(ignoreTools, ",")
			for i := range upstream.IgnoreTools {
				upstream.IgnoreTools[i] = strings.TrimSpace(upstream.IgnoreTools[i])
			}
		}

		config.Upstreams = []types.UpstreamConfig{upstream}
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// EnsureCacheDir creates the cache directory if it doesn't exist.
func EnsureCacheDir(cacheDir string) error {
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return fmt.Errorf("failed to create cache dir: %w", err)
	}
	return nil
}

// getEnvOrDefault returns the environment variable value or a default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault returns the environment variable as an integer or a default.
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

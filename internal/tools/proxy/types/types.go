package types

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// UpstreamConfig represents configuration for a single upstream MCP server.
type UpstreamConfig struct {
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Transport   string            `json:"transport"` // http-first, sse-first, http-only, sse-only
	OAuth       *OAuthConfig      `json:"oauth,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	IgnoreTools []string          `json:"ignore_tools,omitempty"`
}

// OAuthConfig holds OAuth-specific configuration.
type OAuthConfig struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
}

// ProxyConfig holds the complete proxy tool configuration.
type ProxyConfig struct {
	Upstreams    []UpstreamConfig
	CallbackHost string
	CallbackPort int
	CacheDir     string
}

// UpstreamHash generates a unique hash for an upstream configuration.
// This is used to isolate OAuth sessions per upstream.
func UpstreamHash(upstream *UpstreamConfig) string {
	parts := []string{upstream.URL}

	// Include headers in hash to isolate sessions with different headers
	if len(upstream.Headers) > 0 {
		keys := make([]string, 0, len(upstream.Headers))
		for k := range upstream.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		data, _ := json.Marshal(upstream.Headers)
		parts = append(parts, string(data))
	}

	// Include OAuth client ID if static (to isolate different client credentials)
	if upstream.OAuth != nil && upstream.OAuth.ClientID != "" {
		parts = append(parts, upstream.OAuth.ClientID)
	}

	hash := md5.Sum([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(hash[:])
}

// Validate checks if the configuration is valid.
func (c *ProxyConfig) Validate() error {
	if len(c.Upstreams) == 0 {
		return fmt.Errorf("no upstreams configured")
	}

	seenNames := make(map[string]bool)
	for i, upstream := range c.Upstreams {
		// Check name is unique
		if seenNames[upstream.Name] {
			return fmt.Errorf("duplicate upstream name: %s", upstream.Name)
		}
		seenNames[upstream.Name] = true

		// Check URL is present
		if upstream.URL == "" {
			return fmt.Errorf("upstream %d: URL is required", i)
		}

		// Check URL scheme
		if !strings.HasPrefix(upstream.URL, "http://") && !strings.HasPrefix(upstream.URL, "https://") {
			return fmt.Errorf("upstream %s: URL must start with http:// or https://", upstream.Name)
		}

		// Validate transport strategy
		validTransports := []string{"http-first", "sse-first", "http-only", "sse-only"}
		transport := upstream.Transport
		if transport == "" {
			transport = "http-first" // default
		}
		valid := slices.Contains(validTransports, transport)
		if !valid {
			return fmt.Errorf("upstream %s: invalid transport strategy %s (must be one of: %s)",
				upstream.Name, transport, strings.Join(validTransports, ", "))
		}
	}

	// Validate callback port
	if c.CallbackPort < 1 || c.CallbackPort > 65535 {
		return fmt.Errorf("invalid callback port: %d (must be 1-65535)", c.CallbackPort)
	}

	return nil
}

package telemetry

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

const (
	// Sanitisation thresholds
	minTokenLength    = 20  // Minimum length for alphanumeric strings to be considered tokens
	tokenPrefixLength = 8   // Length of prefix to show for redacted tokens
	maxCacheKeyLength = 100 // Maximum length for cache keys before truncation
)

// Sensitive patterns that should never appear in trace attributes
var (
	// API key patterns
	apiKeyPattern = regexp.MustCompile(`(?i)(api[_-]?key|apikey|token|secret|password|passwd|pwd|auth|authorization)[\s:=]+["']?([^\s"']+)`)

	// Common secret environment variable names
	secretEnvVars = map[string]bool{
		"api_key":       true,
		"apikey":        true,
		"token":         true,
		"secret":        true,
		"password":      true,
		"passwd":        true,
		"pwd":           true,
		"auth":          true,
		"authorization": true,
		"client_secret": true,
		"oauth_token":   true,
		"access_token":  true,
		"refresh_token": true,
		"private_key":   true,
		"ssh_key":       true,
		"certificate":   true,
		"credentials":   true,
	}

	// URL query parameters that might contain secrets
	sensitiveQueryParams = map[string]bool{
		"api_key":      true,
		"apikey":       true,
		"token":        true,
		"access_token": true,
		"secret":       true,
		"key":          true,
		"password":     true,
		"auth":         true,
	}
)

// SanitiseURL removes sensitive query parameters and credentials from URLs
func SanitiseURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" {
		// If we can't parse it or it has no scheme, return a redacted placeholder
		return "[INVALID_URL]"
	}

	// Remove user info (username:password)
	parsedURL.User = nil

	// Sanitise query parameters
	if parsedURL.RawQuery != "" {
		query := parsedURL.Query()
		for key := range query {
			keyLower := strings.ToLower(key)
			if sensitiveQueryParams[keyLower] || strings.Contains(keyLower, "key") || strings.Contains(keyLower, "token") {
				query.Set(key, "[REDACTED]")
			}
		}
		parsedURL.RawQuery = query.Encode()
	}

	return parsedURL.String()
}

// SanitiseArguments sanitises tool arguments by removing sensitive values
// Returns a JSON string of the sanitised arguments, or an error message if serialisation fails
// toolName is optional - if provided, enables tool-specific redaction rules
func SanitiseArguments(args map[string]any, toolName string) string {
	if len(args) == 0 {
		return "{}"
	}

	// Create a copy of the arguments to avoid modifying the original
	sanitised := make(map[string]any)
	for key, value := range args {
		keyLower := strings.ToLower(key)

		// Tool-specific redaction: think tool's thought content
		// Truncate long thoughts to avoid bloating traces with large reasoning text
		if toolName == "think" && keyLower == "thought" {
			if str, ok := value.(string); ok && len(str) > 100 {
				// Show first 80 characters for context
				if len(str) > 80 {
					sanitised[key] = str[:80] + "...[TRUNCATED (tracing)]"
				} else {
					sanitised[key] = str
				}
				continue
			}
		}

		// Check if this key is sensitive
		if secretEnvVars[keyLower] || strings.Contains(keyLower, "key") || strings.Contains(keyLower, "token") || strings.Contains(keyLower, "secret") || strings.Contains(keyLower, "password") {
			sanitised[key] = "[REDACTED]"
			continue
		}

		// Recursively sanitise nested maps
		switch v := value.(type) {
		case map[string]any:
			sanitised[key] = sanitiseMap(v)
		case string:
			// Sanitise string values that might contain secrets
			sanitised[key] = sanitiseString(v)
		default:
			sanitised[key] = value
		}
	}

	// Convert to JSON string
	jsonBytes, err := json.Marshal(sanitised)
	if err != nil {
		return "{\"error\": \"failed to serialise arguments\"}"
	}

	return string(jsonBytes)
}

// sanitiseMap recursively sanitises nested maps
func sanitiseMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}

	sanitised := make(map[string]any)
	for key, value := range m {
		keyLower := strings.ToLower(key)

		if secretEnvVars[keyLower] || strings.Contains(keyLower, "key") || strings.Contains(keyLower, "token") || strings.Contains(keyLower, "secret") || strings.Contains(keyLower, "password") {
			sanitised[key] = "[REDACTED]"
			continue
		}

		switch v := value.(type) {
		case map[string]any:
			sanitised[key] = sanitiseMap(v)
		case string:
			sanitised[key] = sanitiseString(v)
		default:
			sanitised[key] = value
		}
	}

	return sanitised
}

// sanitiseString removes sensitive patterns from strings
func sanitiseString(s string) string {
	if s == "" {
		return s
	}

	// Check if the string looks like it contains an API key pattern
	if apiKeyPattern.MatchString(s) {
		return apiKeyPattern.ReplaceAllString(s, "$1=[REDACTED]")
	}

	// Check if the entire string looks like a token (long alphanumeric string)
	if len(s) > minTokenLength && isAlphanumeric(s) {
		// Might be a token, show only prefix
		if len(s) > tokenPrefixLength {
			return s[:4] + "..." + "[REDACTED]"
		}
		return "[REDACTED]"
	}

	return s
}

// isAlphanumeric checks if a string contains only alphanumeric characters and common token characters
func isAlphanumeric(s string) bool {
	for _, char := range s {
		isValid := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.'
		if !isValid {
			return false
		}
	}
	return true
}

// SanitiseCacheKey sanitises cache keys to avoid leaking sensitive information
func SanitiseCacheKey(key string) string {
	if key == "" {
		return ""
	}

	// If the key contains sensitive patterns, redact it
	if strings.Contains(strings.ToLower(key), "token") || strings.Contains(strings.ToLower(key), "key") || strings.Contains(strings.ToLower(key), "secret") {
		// Show only a hash or prefix
		if len(key) > 8 {
			return key[:4] + "...[REDACTED]"
		}
		return "[REDACTED]"
	}

	// Limit length to prevent massive cache keys in traces
	if len(key) > maxCacheKeyLength {
		return key[:maxCacheKeyLength-3] + "..."
	}

	return key
}

// TruncateString truncates a string to a maximum length with ellipsis
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}

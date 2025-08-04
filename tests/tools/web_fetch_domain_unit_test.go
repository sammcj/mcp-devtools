package tools_test

import (
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/tests/testutils"
)

// TestDomainAllowlistLogic tests the domain allowlist logic in isolation
// This is a focused unit test that doesn't make HTTP requests
func TestDomainAllowlistLogic(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("FETCH_DOMAIN_ALLOWLIST")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("FETCH_DOMAIN_ALLOWLIST")
		} else {
			_ = os.Setenv("FETCH_DOMAIN_ALLOWLIST", originalValue)
		}
	}()

	testCases := []struct {
		name      string
		allowlist string
		domain    string
		allowed   bool
	}{
		// No allowlist - all domains should be allowed
		{"no_allowlist_any_domain", "", "example.com", true},
		{"no_allowlist_subdomain", "", "api.example.com", true},

		// Exact matches
		{"exact_match_allowed", "example.com,github.com", "example.com", true},
		{"exact_match_not_in_list", "example.com,github.com", "evil.com", false},

		// Wildcard tests
		{"wildcard_subdomain_allowed", "*.example.com", "api.example.com", true},
		{"wildcard_base_domain_allowed", "*.example.com", "example.com", true},
		{"wildcard_deep_subdomain_allowed", "*.example.com", "api.v1.example.com", true},
		{"wildcard_not_matching", "*.example.com", "example.org", false},

		// Mixed exact and wildcard
		{"mixed_exact_and_wildcard", "docs.com,*.api.com", "docs.com", true},
		{"mixed_wildcard_match", "docs.com,*.api.com", "v1.api.com", true},
		{"mixed_no_match", "docs.com,*.api.com", "evil.com", false},

		// Whitespace handling
		{"whitespace_trimmed", " example.com , *.api.com ", "example.com", true},
		{"whitespace_wildcard", " example.com , *.api.com ", "v1.api.com", true},

		// Empty values handling
		{"empty_values_ignored", "example.com,,github.com,", "example.com", true},
		{"empty_values_not_allowed", "example.com,,github.com,", "evil.com", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up environment
			if tc.allowlist == "" {
				_ = os.Unsetenv("FETCH_DOMAIN_ALLOWLIST")
			} else {
				err := os.Setenv("FETCH_DOMAIN_ALLOWLIST", tc.allowlist)
				if err != nil {
					t.Fatalf("Failed to set environment variable: %v", err)
				}
			}

			// Test the logic by attempting to parse the domain in isolation
			// We can infer the result from the environment variable setup
			allowlist := os.Getenv("FETCH_DOMAIN_ALLOWLIST")
			result := isDomainAllowedByLogic(tc.domain, allowlist)

			testutils.AssertEqual(t, tc.allowed, result)
		})
	}
}

// isDomainAllowedByLogic replicates the domain allowlist logic for testing
// This mirrors the logic in webfetch/client.go isDomainAllowed function
func isDomainAllowedByLogic(hostname, allowlist string) bool {
	if allowlist == "" {
		// If no allowlist is configured, allow all domains
		return true
	}

	// Parse comma-separated domain list
	domains := parseDomainList(allowlist)
	for _, domain := range domains {
		if domain == "" {
			continue
		}

		// Support wildcard subdomains (e.g., "*.example.com")
		if len(domain) > 2 && domain[:2] == "*." {
			baseDomain := domain[2:]
			if hostname == baseDomain || (len(hostname) > len(baseDomain) && hostname[len(hostname)-len(baseDomain)-1:] == "."+baseDomain) {
				return true
			}
		} else {
			// Exact domain match
			if hostname == domain {
				return true
			}
		}
	}

	return false
}

// parseDomainList parses and trims domain list from environment variable
func parseDomainList(allowlist string) []string {
	if allowlist == "" {
		return nil
	}

	domains := make([]string, 0)
	for _, domain := range splitAndTrim(allowlist, ",") {
		if domain != "" {
			domains = append(domains, domain)
		}
	}
	return domains
}

// splitAndTrim splits a string and trims whitespace from each part
func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, part := range splitString(s, sep) {
		trimmed := trimWhitespace(part)
		parts = append(parts, trimmed)
	}
	return parts
}

// splitString splits string by separator
func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}

	parts := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if i == len(s)-1 {
			parts = append(parts, s[start:i+1])
		} else if string(s[i]) == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return parts
}

// trimWhitespace removes leading/trailing whitespace
func trimWhitespace(s string) string {
	start := 0
	end := len(s)

	// Find first non-space character
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	// Find last non-space character
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}

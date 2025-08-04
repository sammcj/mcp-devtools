package tools_test

import (
	"context"
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/webfetch"
	"github.com/sammcj/mcp-devtools/tests/testutils"
	"github.com/sirupsen/logrus"
)

func TestFetchURLTool_DomainAllowlist_NoConfiguration(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("FETCH_DOMAIN_ALLOWLIST")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("FETCH_DOMAIN_ALLOWLIST")
		} else {
			_ = os.Setenv("FETCH_DOMAIN_ALLOWLIST", originalValue)
		}
	}()

	// Clear environment variable to test default behavior
	_ = os.Unsetenv("FETCH_DOMAIN_ALLOWLIST")

	client := webfetch.NewWebClient()
	logger := logrus.New()

	// Test that any domain is allowed when no allowlist is configured
	// We'll test with a non-existent domain to avoid actual network calls
	_, err := client.FetchContent(context.Background(), logger, "https://non-existent-domain-12345.example")

	// The error should be a network error, not a domain restriction error
	if err != nil && err.Error() == "domain not allowed: non-existent-domain-12345.example (check FETCH_DOMAIN_ALLOWLIST environment variable)" {
		t.Errorf("Domain should be allowed when no allowlist is configured, but got domain restriction error")
	}
}

func TestFetchURLTool_DomainAllowlist_ExactMatch(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("FETCH_DOMAIN_ALLOWLIST")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("FETCH_DOMAIN_ALLOWLIST")
		} else {
			_ = os.Setenv("FETCH_DOMAIN_ALLOWLIST", originalValue)
		}
	}()

	// Set allowlist with exact domain
	err := os.Setenv("FETCH_DOMAIN_ALLOWLIST", "allowed1.com,allowed2.com")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	client := webfetch.NewWebClient()
	logger := logrus.New()

	// Test disallowed domain - should get domain restriction error immediately
	// Use .invalid TLD to ensure no network request is made
	_, err = client.FetchContent(context.Background(), logger, "https://blocked.invalid/test")
	if err == nil || err.Error() != "domain not allowed: blocked.invalid (check FETCH_DOMAIN_ALLOWLIST environment variable)" {
		t.Errorf("Domain blocked.invalid should be blocked, but got: %v", err)
	}

	// Test another disallowed domain
	_, err = client.FetchContent(context.Background(), logger, "https://notallowed.invalid/test")
	if err == nil || err.Error() != "domain not allowed: notallowed.invalid (check FETCH_DOMAIN_ALLOWLIST environment variable)" {
		t.Errorf("Domain notallowed.invalid should be blocked, but got: %v", err)
	}
}

func TestFetchURLTool_DomainAllowlist_WildcardSubdomains(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("FETCH_DOMAIN_ALLOWLIST")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("FETCH_DOMAIN_ALLOWLIST")
		} else {
			_ = os.Setenv("FETCH_DOMAIN_ALLOWLIST", originalValue)
		}
	}()

	// Set allowlist with wildcard subdomain - use .invalid TLD to prevent network calls
	err := os.Setenv("FETCH_DOMAIN_ALLOWLIST", "*.allowed.invalid,docs.allowed.invalid")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	client := webfetch.NewWebClient()
	logger := logrus.New()

	// Test disallowed domain - should be blocked immediately without network call
	_, err = client.FetchContent(context.Background(), logger, "https://blocked.invalid/test")
	if err == nil || err.Error() != "domain not allowed: blocked.invalid (check FETCH_DOMAIN_ALLOWLIST environment variable)" {
		t.Errorf("Domain blocked.invalid should be blocked, but got: %v", err)
	}

	// Test different blocked domain
	_, err = client.FetchContent(context.Background(), logger, "https://notallowed.invalid/test")
	if err == nil || err.Error() != "domain not allowed: notallowed.invalid (check FETCH_DOMAIN_ALLOWLIST environment variable)" {
		t.Errorf("Domain notallowed.invalid should be blocked, but got: %v", err)
	}
}

func TestFetchURLTool_DomainAllowlist_WhitespaceHandling(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("FETCH_DOMAIN_ALLOWLIST")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("FETCH_DOMAIN_ALLOWLIST")
		} else {
			_ = os.Setenv("FETCH_DOMAIN_ALLOWLIST", originalValue)
		}
	}()

	// Set allowlist with extra whitespace - use .invalid TLD to prevent network calls
	err := os.Setenv("FETCH_DOMAIN_ALLOWLIST", " allowed1.invalid , allowed2.invalid , *.wildcard.invalid ")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	client := webfetch.NewWebClient()
	logger := logrus.New()

	// Test disallowed domain - should be blocked immediately without network call
	_, err = client.FetchContent(context.Background(), logger, "https://blocked.invalid/test")
	if err == nil || err.Error() != "domain not allowed: blocked.invalid (check FETCH_DOMAIN_ALLOWLIST environment variable)" {
		t.Errorf("Domain blocked.invalid should be blocked, but got: %v", err)
	}

	// Test another disallowed domain
	_, err = client.FetchContent(context.Background(), logger, "https://notallowed.invalid/test")
	if err == nil || err.Error() != "domain not allowed: notallowed.invalid (check FETCH_DOMAIN_ALLOWLIST environment variable)" {
		t.Errorf("Domain notallowed.invalid should be blocked, but got: %v", err)
	}
}

func TestFetchURLTool_DomainAllowlist_EmptyValues(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("FETCH_DOMAIN_ALLOWLIST")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("FETCH_DOMAIN_ALLOWLIST")
		} else {
			_ = os.Setenv("FETCH_DOMAIN_ALLOWLIST", originalValue)
		}
	}()

	// Set allowlist with empty values (should be ignored) - use .invalid TLD to prevent network calls
	err := os.Setenv("FETCH_DOMAIN_ALLOWLIST", "allowed1.invalid,,allowed2.invalid,")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	client := webfetch.NewWebClient()
	logger := logrus.New()

	// Test that disallowed domains are blocked immediately without network call
	_, err = client.FetchContent(context.Background(), logger, "https://blocked.invalid/test")
	if err == nil || err.Error() != "domain not allowed: blocked.invalid (check FETCH_DOMAIN_ALLOWLIST environment variable)" {
		t.Errorf("Domain blocked.invalid should be blocked, but got: %v", err)
	}

	// Test another disallowed domain
	_, err = client.FetchContent(context.Background(), logger, "https://notallowed.invalid/test")
	if err == nil || err.Error() != "domain not allowed: notallowed.invalid (check FETCH_DOMAIN_ALLOWLIST environment variable)" {
		t.Errorf("Domain notallowed.invalid should be blocked, but got: %v", err)
	}
}

func TestWebFetchConstants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, "FETCH_DOMAIN_ALLOWLIST", webfetch.FetchDomainAllowlistEnvVar)
}

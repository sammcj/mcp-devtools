package tools_test

import (
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestInternetSearchRateLimitedHTTPClient_DefaultRateLimit(t *testing.T) {
	// Test client creation and basic functionality without making HTTP requests
	client := internetsearch.NewRateLimitedHTTPClient()
	testutils.AssertNotNil(t, client)

	// Just verify the client was created successfully - no need to make real HTTP calls
}

func TestInternetSearchRateLimitedHTTPClient_CustomRateLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("INTERNET_SEARCH_RATE_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("INTERNET_SEARCH_RATE_LIMIT")
		} else {
			_ = os.Setenv("INTERNET_SEARCH_RATE_LIMIT", originalValue)
		}
	}()

	// Set custom rate limit
	err := os.Setenv("INTERNET_SEARCH_RATE_LIMIT", "5")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	// Test client creation with custom rate limit
	client := internetsearch.NewRateLimitedHTTPClient()
	testutils.AssertNotNil(t, client)

	// Just verify the client was created successfully - no need to make real HTTP calls
}

func TestInternetSearchRateLimitedHTTPClient_InvalidEnvironmentVariable(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("INTERNET_SEARCH_RATE_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("INTERNET_SEARCH_RATE_LIMIT")
		} else {
			_ = os.Setenv("INTERNET_SEARCH_RATE_LIMIT", originalValue)
		}
	}()

	// Set invalid rate limit (negative number)
	err := os.Setenv("INTERNET_SEARCH_RATE_LIMIT", "-1")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	// Should fall back to default rate limit
	client := internetsearch.NewRateLimitedHTTPClient()
	testutils.AssertNotNil(t, client)

	// Reset to test non-numeric value
	err = os.Setenv("INTERNET_SEARCH_RATE_LIMIT", "invalid")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	// Should fall back to default rate limit
	client = internetsearch.NewRateLimitedHTTPClient()
	testutils.AssertNotNil(t, client)
}

func TestInternetSearchConstants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, 1, internetsearch.DefaultInternetSearchRateLimit)
	testutils.AssertEqual(t, "INTERNET_SEARCH_RATE_LIMIT", internetsearch.InternetSearchRateLimitEnvVar)
}

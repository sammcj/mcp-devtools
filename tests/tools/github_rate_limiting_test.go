package tools_test

import (
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/github"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestGitHubRateLimit_DefaultLimits(t *testing.T) {
	// Save original environment variables
	originalCore := os.Getenv("GITHUB_CORE_API_RATE_LIMIT")
	originalSearch := os.Getenv("GITHUB_SEARCH_API_RATE_LIMIT")
	defer func() {
		if originalCore == "" {
			_ = os.Unsetenv("GITHUB_CORE_API_RATE_LIMIT")
		} else {
			_ = os.Setenv("GITHUB_CORE_API_RATE_LIMIT", originalCore)
		}
		if originalSearch == "" {
			_ = os.Unsetenv("GITHUB_SEARCH_API_RATE_LIMIT")
		} else {
			_ = os.Setenv("GITHUB_SEARCH_API_RATE_LIMIT", originalSearch)
		}
	}()

	// Clear environment variables to test defaults
	_ = os.Unsetenv("GITHUB_CORE_API_RATE_LIMIT")
	_ = os.Unsetenv("GITHUB_SEARCH_API_RATE_LIMIT")

	// Test default rate limits
	coreLimit := github.GetEnvRateLimit(github.GitHubCoreAPIRateLimitEnvVar, github.DefaultCoreAPIRateLimit)
	searchLimit := github.GetEnvRateLimit(github.GitHubSearchAPIRateLimitEnvVar, github.DefaultSearchAPIRateLimit)

	testutils.AssertEqual(t, github.DefaultCoreAPIRateLimit, coreLimit)
	testutils.AssertEqual(t, github.DefaultSearchAPIRateLimit, searchLimit)
}

func TestGitHubRateLimit_CustomLimits(t *testing.T) {
	// Save original environment variables
	originalCore := os.Getenv("GITHUB_CORE_API_RATE_LIMIT")
	originalSearch := os.Getenv("GITHUB_SEARCH_API_RATE_LIMIT")
	defer func() {
		if originalCore == "" {
			_ = os.Unsetenv("GITHUB_CORE_API_RATE_LIMIT")
		} else {
			_ = os.Setenv("GITHUB_CORE_API_RATE_LIMIT", originalCore)
		}
		if originalSearch == "" {
			_ = os.Unsetenv("GITHUB_SEARCH_API_RATE_LIMIT")
		} else {
			_ = os.Setenv("GITHUB_SEARCH_API_RATE_LIMIT", originalSearch)
		}
	}()

	// Set custom rate limits
	err := os.Setenv("GITHUB_CORE_API_RATE_LIMIT", "100")
	if err != nil {
		t.Fatalf("Failed to set core API rate limit: %v", err)
	}
	err = os.Setenv("GITHUB_SEARCH_API_RATE_LIMIT", "30")
	if err != nil {
		t.Fatalf("Failed to set search API rate limit: %v", err)
	}

	// Test custom rate limits
	coreLimit := github.GetEnvRateLimit(github.GitHubCoreAPIRateLimitEnvVar, github.DefaultCoreAPIRateLimit)
	searchLimit := github.GetEnvRateLimit(github.GitHubSearchAPIRateLimitEnvVar, github.DefaultSearchAPIRateLimit)

	testutils.AssertEqual(t, 100, coreLimit)
	testutils.AssertEqual(t, 30, searchLimit)
}

func TestGitHubRateLimit_InvalidEnvironmentVariables(t *testing.T) {
	// Save original environment variables
	originalCore := os.Getenv("GITHUB_CORE_API_RATE_LIMIT")
	originalSearch := os.Getenv("GITHUB_SEARCH_API_RATE_LIMIT")
	defer func() {
		if originalCore == "" {
			_ = os.Unsetenv("GITHUB_CORE_API_RATE_LIMIT")
		} else {
			_ = os.Setenv("GITHUB_CORE_API_RATE_LIMIT", originalCore)
		}
		if originalSearch == "" {
			_ = os.Unsetenv("GITHUB_SEARCH_API_RATE_LIMIT")
		} else {
			_ = os.Setenv("GITHUB_SEARCH_API_RATE_LIMIT", originalSearch)
		}
	}()

	// Set invalid environment variables
	err := os.Setenv("GITHUB_CORE_API_RATE_LIMIT", "invalid")
	if err != nil {
		t.Fatalf("Failed to set core API rate limit: %v", err)
	}
	err = os.Setenv("GITHUB_SEARCH_API_RATE_LIMIT", "not-a-number")
	if err != nil {
		t.Fatalf("Failed to set search API rate limit: %v", err)
	}

	// Should fall back to defaults when invalid values are provided
	coreLimit := github.GetEnvRateLimit(github.GitHubCoreAPIRateLimitEnvVar, github.DefaultCoreAPIRateLimit)
	searchLimit := github.GetEnvRateLimit(github.GitHubSearchAPIRateLimitEnvVar, github.DefaultSearchAPIRateLimit)

	testutils.AssertEqual(t, github.DefaultCoreAPIRateLimit, coreLimit)
	testutils.AssertEqual(t, github.DefaultSearchAPIRateLimit, searchLimit)
}

func TestGitHubRateLimit_ZeroEnvironmentVariables(t *testing.T) {
	// Save original environment variables
	originalCore := os.Getenv("GITHUB_CORE_API_RATE_LIMIT")
	originalSearch := os.Getenv("GITHUB_SEARCH_API_RATE_LIMIT")
	defer func() {
		if originalCore == "" {
			_ = os.Unsetenv("GITHUB_CORE_API_RATE_LIMIT")
		} else {
			_ = os.Setenv("GITHUB_CORE_API_RATE_LIMIT", originalCore)
		}
		if originalSearch == "" {
			_ = os.Unsetenv("GITHUB_SEARCH_API_RATE_LIMIT")
		} else {
			_ = os.Setenv("GITHUB_SEARCH_API_RATE_LIMIT", originalSearch)
		}
	}()

	// Set zero environment variables
	err := os.Setenv("GITHUB_CORE_API_RATE_LIMIT", "0")
	if err != nil {
		t.Fatalf("Failed to set core API rate limit: %v", err)
	}
	err = os.Setenv("GITHUB_SEARCH_API_RATE_LIMIT", "0")
	if err != nil {
		t.Fatalf("Failed to set search API rate limit: %v", err)
	}

	// Should fall back to defaults when zero values are provided
	coreLimit := github.GetEnvRateLimit(github.GitHubCoreAPIRateLimitEnvVar, github.DefaultCoreAPIRateLimit)
	searchLimit := github.GetEnvRateLimit(github.GitHubSearchAPIRateLimitEnvVar, github.DefaultSearchAPIRateLimit)

	testutils.AssertEqual(t, github.DefaultCoreAPIRateLimit, coreLimit)
	testutils.AssertEqual(t, github.DefaultSearchAPIRateLimit, searchLimit)
}

func TestGitHubRateLimit_NegativeEnvironmentVariables(t *testing.T) {
	// Save original environment variables
	originalCore := os.Getenv("GITHUB_CORE_API_RATE_LIMIT")
	originalSearch := os.Getenv("GITHUB_SEARCH_API_RATE_LIMIT")
	defer func() {
		if originalCore == "" {
			_ = os.Unsetenv("GITHUB_CORE_API_RATE_LIMIT")
		} else {
			_ = os.Setenv("GITHUB_CORE_API_RATE_LIMIT", originalCore)
		}
		if originalSearch == "" {
			_ = os.Unsetenv("GITHUB_SEARCH_API_RATE_LIMIT")
		} else {
			_ = os.Setenv("GITHUB_SEARCH_API_RATE_LIMIT", originalSearch)
		}
	}()

	// Set negative environment variables
	err := os.Setenv("GITHUB_CORE_API_RATE_LIMIT", "-10")
	if err != nil {
		t.Fatalf("Failed to set core API rate limit: %v", err)
	}
	err = os.Setenv("GITHUB_SEARCH_API_RATE_LIMIT", "-5")
	if err != nil {
		t.Fatalf("Failed to set search API rate limit: %v", err)
	}

	// Should fall back to defaults when negative values are provided
	coreLimit := github.GetEnvRateLimit(github.GitHubCoreAPIRateLimitEnvVar, github.DefaultCoreAPIRateLimit)
	searchLimit := github.GetEnvRateLimit(github.GitHubSearchAPIRateLimitEnvVar, github.DefaultSearchAPIRateLimit)

	testutils.AssertEqual(t, github.DefaultCoreAPIRateLimit, coreLimit)
	testutils.AssertEqual(t, github.DefaultSearchAPIRateLimit, searchLimit)
}

func TestGitHubConstants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, "GITHUB_CORE_API_RATE_LIMIT", github.GitHubCoreAPIRateLimitEnvVar)
	testutils.AssertEqual(t, "GITHUB_SEARCH_API_RATE_LIMIT", github.GitHubSearchAPIRateLimitEnvVar)
	testutils.AssertEqual(t, 80, github.DefaultCoreAPIRateLimit)
	testutils.AssertEqual(t, 25, github.DefaultSearchAPIRateLimit)
}

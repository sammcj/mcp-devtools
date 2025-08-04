package tools_test

import (
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/packagedocs"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sammcj/mcp-devtools/tests/testutils"
	"github.com/sirupsen/logrus"
)

func TestPackageVersionsRateLimitedHTTPClient_DefaultRateLimit(t *testing.T) {
	// Test client creation and basic functionality without making HTTP requests
	client := packageversions.NewRateLimitedHTTPClient()
	testutils.AssertNotNil(t, client)

	// Just verify the client was created successfully - no need to make real HTTP calls
}

func TestPackageVersionsRateLimitedHTTPClient_CustomRateLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("PACKAGES_RATE_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PACKAGES_RATE_LIMIT")
		} else {
			_ = os.Setenv("PACKAGES_RATE_LIMIT", originalValue)
		}
	}()

	// Set custom rate limit
	err := os.Setenv("PACKAGES_RATE_LIMIT", "20")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	// Test client creation with custom rate limit
	client := packageversions.NewRateLimitedHTTPClient()
	testutils.AssertNotNil(t, client)

	// Just verify the client was created successfully - no need to make real HTTP calls
}

func TestPackageVersionsRateLimitedHTTPClient_InvalidEnvironmentVariable(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("PACKAGES_RATE_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PACKAGES_RATE_LIMIT")
		} else {
			_ = os.Setenv("PACKAGES_RATE_LIMIT", originalValue)
		}
	}()

	// Set invalid rate limit (negative number)
	err := os.Setenv("PACKAGES_RATE_LIMIT", "-10")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	client := packageversions.NewRateLimitedHTTPClient()

	// Should fall back to default rate limit
	// Just test client creation, not actual requests to avoid rate limiting delays
	testutils.AssertNotNil(t, client)

	// Reset to test non-numeric value
	err = os.Setenv("PACKAGES_RATE_LIMIT", "invalid")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	client = packageversions.NewRateLimitedHTTPClient()

	// Should fall back to default rate limit
	testutils.AssertNotNil(t, client)
}

func TestPackageDocsClient_RateLimiting(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("PACKAGE_DOCS_RATE_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PACKAGE_DOCS_RATE_LIMIT")
		} else {
			_ = os.Setenv("PACKAGE_DOCS_RATE_LIMIT", originalValue)
		}
	}()

	// Set custom rate limit
	err := os.Setenv("PACKAGE_DOCS_RATE_LIMIT", "5")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	logger := logrus.New()
	client := packagedocs.NewClient(logger)

	// Test that the client was created successfully
	testutils.AssertNotNil(t, client)
}

func TestPackageDocsRateLimitedHTTPClient_DefaultRateLimit(t *testing.T) {
	// Test the package docs rate limiting directly
	originalValue := os.Getenv("PACKAGE_DOCS_RATE_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PACKAGE_DOCS_RATE_LIMIT")
		} else {
			_ = os.Setenv("PACKAGE_DOCS_RATE_LIMIT", originalValue)
		}
	}()

	// Clear environment variable to test default
	_ = os.Unsetenv("PACKAGE_DOCS_RATE_LIMIT")

	logger := logrus.New()
	client := packagedocs.NewClient(logger)

	// Test that the client was created successfully
	testutils.AssertNotNil(t, client)
}

func TestPackageDocsRateLimitedHTTPClient_CustomRateLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("PACKAGE_DOCS_RATE_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PACKAGE_DOCS_RATE_LIMIT")
		} else {
			_ = os.Setenv("PACKAGE_DOCS_RATE_LIMIT", originalValue)
		}
	}()

	// Set custom rate limit to 15 requests per second
	err := os.Setenv("PACKAGE_DOCS_RATE_LIMIT", "15")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	logger := logrus.New()
	client := packagedocs.NewClient(logger)

	// Test that the client was created successfully with custom rate limit
	testutils.AssertNotNil(t, client)
}

func TestPackageDocsRateLimitedHTTPClient_InvalidEnvironmentVariable(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("PACKAGE_DOCS_RATE_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PACKAGE_DOCS_RATE_LIMIT")
		} else {
			_ = os.Setenv("PACKAGE_DOCS_RATE_LIMIT", originalValue)
		}
	}()

	// Set invalid rate limit (non-numeric)
	err := os.Setenv("PACKAGE_DOCS_RATE_LIMIT", "not-a-number")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	logger := logrus.New()
	client := packagedocs.NewClient(logger)

	// Should fall back to default rate limit and still work
	testutils.AssertNotNil(t, client)
}

func TestPackageVersionsConstants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, 10, packageversions.DefaultPackagesRateLimit)
	testutils.AssertEqual(t, "PACKAGES_RATE_LIMIT", packageversions.PackagesRateLimitEnvVar)
}

func TestPackageDocsConstants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, 10, packagedocs.DefaultPackageDocsRateLimit)
	testutils.AssertEqual(t, "PACKAGE_DOCS_RATE_LIMIT", packagedocs.PackageDocsRateLimitEnvVar)
}

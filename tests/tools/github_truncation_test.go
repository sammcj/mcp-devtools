package tools

import (
	"context"
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/github"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestFileTruncation tests the automatic file truncation behaviour
func TestFileTruncation(t *testing.T) {
	// Set a small max lines for testing
	_ = os.Setenv("GITHUB_MAX_LINES", "10")
	defer func() { _ = os.Unsetenv("GITHUB_MAX_LINES") }()

	// Note: This is a unit test for the truncation logic
	// We're testing the behaviour, not making real API calls
	// The actual truncation happens in GetFileContents which requires GitHub API access

	// Test 1: Verify GetEnvInt works correctly
	maxLines := github.GetEnvInt("GITHUB_MAX_LINES", 3000)
	assert.Equal(t, 10, maxLines, "GetEnvInt should read from environment variable")

	// Test 2: Verify default value when env var not set
	_ = os.Unsetenv("GITHUB_MAX_LINES")
	maxLines = github.GetEnvInt("GITHUB_MAX_LINES", 3000)
	assert.Equal(t, 3000, maxLines, "GetEnvInt should use default when env var not set")
}

// TestGetEnvInt tests the GetEnvInt helper function
func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		envVar       string
		envValue     string
		defaultValue int
		expected     int
	}{
		{
			name:         "Valid environment variable",
			envVar:       "TEST_INT_VAR",
			envValue:     "500",
			defaultValue: 3000,
			expected:     500,
		},
		{
			name:         "Missing environment variable",
			envVar:       "MISSING_VAR",
			envValue:     "",
			defaultValue: 3000,
			expected:     3000,
		},
		{
			name:         "Invalid (non-numeric) environment variable",
			envVar:       "TEST_INVALID_VAR",
			envValue:     "not-a-number",
			defaultValue: 3000,
			expected:     3000,
		},
		{
			name:         "Zero value environment variable",
			envVar:       "TEST_ZERO_VAR",
			envValue:     "0",
			defaultValue: 3000,
			expected:     3000,
		},
		{
			name:         "Negative environment variable",
			envVar:       "TEST_NEG_VAR",
			envValue:     "-50",
			defaultValue: 3000,
			expected:     3000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				_ = os.Setenv(tt.envVar, tt.envValue)
				defer func() { _ = os.Unsetenv(tt.envVar) }()
			}

			result := github.GetEnvInt(tt.envVar, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGitHubClientCreation tests that GitHub client can be created
// This is a smoke test to ensure our changes don't break basic functionality
func TestGitHubClientCreation(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	// This will work with or without GITHUB_TOKEN
	// If token is missing, it will create an unauthenticated client
	ctx := context.Background()
	_, err := github.NewGitHubClientWrapper(ctx, logger)

	// We expect this to succeed (unauthenticated client is fine for testing)
	assert.NoError(t, err, "Should be able to create GitHub client")
}

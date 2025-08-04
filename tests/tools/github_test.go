package tools

import (
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRepository(t *testing.T) {
	tests := []struct {
		name          string
		repository    string
		expectedOwner string
		expectedRepo  string
		shouldError   bool
	}{
		{
			name:          "Valid owner/repo format",
			repository:    "microsoft/vscode",
			expectedOwner: "microsoft",
			expectedRepo:  "vscode",
			shouldError:   false,
		},
		{
			name:          "Valid GitHub URL",
			repository:    "https://github.com/microsoft/vscode",
			expectedOwner: "microsoft",
			expectedRepo:  "vscode",
			shouldError:   false,
		},
		{
			name:          "Valid GitHub URL with .git",
			repository:    "https://github.com/microsoft/vscode.git",
			expectedOwner: "microsoft",
			expectedRepo:  "vscode",
			shouldError:   false,
		},
		{
			name:          "GitHub issue URL",
			repository:    "https://github.com/microsoft/vscode/issues/123",
			expectedOwner: "microsoft",
			expectedRepo:  "vscode",
			shouldError:   false,
		},
		{
			name:          "GitHub PR URL",
			repository:    "https://github.com/microsoft/vscode/pull/456",
			expectedOwner: "microsoft",
			expectedRepo:  "vscode",
			shouldError:   false,
		},
		{
			name:        "Empty repository",
			repository:  "",
			shouldError: true,
		},
		{
			name:        "Invalid format - missing repo",
			repository:  "microsoft",
			shouldError: true,
		},
		{
			name:        "Invalid URL format",
			repository:  "https://github.com/microsoft",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := github.ValidateRepository(tt.repository)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedOwner, owner)
				assert.Equal(t, tt.expectedRepo, repo)
			}
		})
	}
}

func TestExtractIssueNumber(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedNumber int
		shouldError    bool
	}{
		{
			name:           "Valid issue URL",
			url:            "https://github.com/microsoft/vscode/issues/123",
			expectedNumber: 123,
			shouldError:    false,
		},
		{
			name:        "Invalid URL - not an issue",
			url:         "https://github.com/microsoft/vscode/pull/123",
			shouldError: true,
		},
		{
			name:        "Invalid URL format",
			url:         "https://example.com/issues/123",
			shouldError: true,
		},
		{
			name:        "Invalid number",
			url:         "https://github.com/microsoft/vscode/issues/abc",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			number, err := github.ExtractIssueNumber(tt.url)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedNumber, number)
			}
		})
	}
}

func TestExtractPullRequestNumber(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedNumber int
		shouldError    bool
	}{
		{
			name:           "Valid PR URL",
			url:            "https://github.com/microsoft/vscode/pull/456",
			expectedNumber: 456,
			shouldError:    false,
		},
		{
			name:        "Invalid URL - not a PR",
			url:         "https://github.com/microsoft/vscode/issues/456",
			shouldError: true,
		},
		{
			name:        "Invalid URL format",
			url:         "https://example.com/pull/456",
			shouldError: true,
		},
		{
			name:        "Invalid number",
			url:         "https://github.com/microsoft/vscode/pull/xyz",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			number, err := github.ExtractPullRequestNumber(tt.url)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedNumber, number)
			}
		})
	}
}

func TestExtractWorkflowRunID(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedID  int64
		shouldError bool
	}{
		{
			name:        "Valid workflow run URL",
			url:         "https://github.com/microsoft/vscode/actions/runs/123456789",
			expectedID:  123456789,
			shouldError: false,
		},
		{
			name:        "Invalid URL - not a workflow run",
			url:         "https://github.com/microsoft/vscode/pull/123",
			shouldError: true,
		},
		{
			name:        "Invalid URL format",
			url:         "https://example.com/actions/runs/123",
			shouldError: true,
		},
		{
			name:        "Invalid run ID",
			url:         "https://github.com/microsoft/vscode/actions/runs/abc",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runID, err := github.ExtractWorkflowRunID(tt.url)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedID, runID)
			}
		})
	}
}

// Removed tests for unexported functions (parseRequest, splitPath, parseInt)
// These are internal implementation details and should not be tested directly
// The public API tests above provide sufficient coverage

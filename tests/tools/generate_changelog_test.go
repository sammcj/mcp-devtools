package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/generatechangelog"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateChangelogTool_Definition(t *testing.T) {
	tool := &generatechangelog.GenerateChangelogTool{}
	def := tool.Definition()

	// Test tool name
	assert.Equal(t, "generate_changelog", def.Name)

	// Test description is present
	assert.Contains(t, def.Description, "Generate changelogs from")
	assert.Contains(t, def.Description, "git repositories")
	assert.Contains(t, def.Description, "commit history")

	// Test required parameters
	require.NotNil(t, def.InputSchema.Properties)

	// Check repository_path is required
	repoPathProp, exists := def.InputSchema.Properties["repository_path"]
	assert.True(t, exists, "repository_path parameter should exist")
	assert.Contains(t, def.InputSchema.Required, "repository_path")

	// Verify repository_path is a string with description
	repoPathMap, ok := repoPathProp.(map[string]any)
	require.True(t, ok, "repository_path should be a property map")
	assert.Equal(t, "string", repoPathMap["type"])
	assert.Contains(t, repoPathMap["description"].(string), "Absolute path to local Git repository or subdirectory")

	// Test optional parameters exist with defaults
	outputFormatProp, exists := def.InputSchema.Properties["output_format"]
	assert.True(t, exists, "output_format parameter should exist")
	outputFormatMap, ok := outputFormatProp.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "markdown", outputFormatMap["default"])

	// Check enum values for output_format
	enum, exists := outputFormatMap["enum"]
	assert.True(t, exists, "output_format should have enum values")
	enumSlice, ok := enum.([]string)
	require.True(t, ok)
	assert.Contains(t, enumSlice, "markdown")
	assert.Contains(t, enumSlice, "json")

	// Test boolean parameter
	speculateProp, exists := def.InputSchema.Properties["speculate_next_version"]
	assert.True(t, exists, "speculate_next_version parameter should exist")
	speculateMap, ok := speculateProp.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "boolean", speculateMap["type"])
	assert.Equal(t, false, speculateMap["default"])

	// Test GitHub integration parameter
	githubProp, exists := def.InputSchema.Properties["enable_github_integration"]
	assert.True(t, exists, "enable_github_integration parameter should exist")
	githubMap, ok := githubProp.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "boolean", githubMap["type"])
	assert.Equal(t, false, githubMap["default"])
	assert.Contains(t, githubMap["description"].(string), "GitHub integration")

}

func TestGenerateChangelogTool_ToolDisabled(t *testing.T) {
	// Ensure generate_changelog tool is disabled by default
	_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")

	tool := &generatechangelog.GenerateChangelogTool{}
	ctx := context.Background()
	logger := logrus.New()
	cache := &sync.Map{}

	args := map[string]any{
		"repository_path": "/path/to/repo",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "generate_changelog tool is not enabled")
	assert.Contains(t, err.Error(), "Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'generate_changelog'")
	assert.Nil(t, result)
}

func TestGenerateChangelogTool_ParseRequest(t *testing.T) {
	// Enable the generate_changelog tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "generate_changelog")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &generatechangelog.GenerateChangelogTool{}

	t.Run("valid minimal request", func(t *testing.T) {
		args := map[string]any{
			"repository_path": "/path/to/repo",
		}

		// Use reflection to access private method for testing
		// In a real implementation, you might want to make parseRequest public or test through Execute
		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		// Since parseRequest is private, we test through Execute which will validate input
		// For this test, we expect it to fail on repository validation, but that means parsing succeeded
		result, err := tool.Execute(ctx, logger, cache, args)

		// We expect an error about repository validation, not parameter parsing
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid repository") // Repository validation should fail
		assert.Nil(t, result)                                 // No result due to validation error
	})

	t.Run("missing required parameter", func(t *testing.T) {
		args := map[string]any{
			"output_format": "json",
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		result, err := tool.Execute(ctx, logger, cache, args)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing or invalid required parameter: repository_path")
		assert.Nil(t, result)
	})

	t.Run("invalid output format", func(t *testing.T) {
		args := map[string]any{
			"repository_path": "/path/to/repo",
			"output_format":   "xml",
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		result, err := tool.Execute(ctx, logger, cache, args)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "output_format must be 'markdown' or 'json'")
		assert.Nil(t, result)
	})

}

func TestGenerateChangelogTool_RepositoryValidation(t *testing.T) {
	// Enable the generate_changelog tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "generate_changelog")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &generatechangelog.GenerateChangelogTool{}

	t.Run("non-existent directory", func(t *testing.T) {
		args := map[string]any{
			"repository_path": "/nonexistent/path/to/repo",
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		result, err := tool.Execute(ctx, logger, cache, args)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository path does not exist")
		assert.Nil(t, result)
	})

	t.Run("file instead of directory", func(t *testing.T) {
		// Create a temporary file
		tmpFile, err := os.CreateTemp("", "test_file")
		require.NoError(t, err)
		defer func() { _ = os.Remove(tmpFile.Name()) }()
		_ = tmpFile.Close()

		args := map[string]any{
			"repository_path": tmpFile.Name(),
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		result, err := tool.Execute(ctx, logger, cache, args)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository path is not a directory")
		assert.Nil(t, result)
	})

	t.Run("directory without .git", func(t *testing.T) {
		// Create a temporary directory without .git
		tmpDir, err := os.MkdirTemp("", "test_dir")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		args := map[string]any{
			"repository_path": tmpDir,
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		result, err := tool.Execute(ctx, logger, cache, args)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path is not within a git repository")
		assert.Nil(t, result)
	})

	t.Run("valid git repository", func(t *testing.T) {
		// Create a temporary directory with .git subdirectory
		tmpDir, err := os.MkdirTemp("", "test_git_repo")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		gitDir := filepath.Join(tmpDir, ".git")
		err = os.Mkdir(gitDir, 0700)
		require.NoError(t, err)

		args := map[string]any{
			"repository_path": tmpDir,
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		// This should pass repository validation and fall back to local git summarizer
		// since we don't have a real git repository with remotes/commits
		result, err := tool.Execute(ctx, logger, cache, args)

		// Since we fall back to local git now, we expect this to either succeed with minimal content
		// or fail on git log (depending on whether the temp directory has git history)
		if err != nil {
			// Should fail on changelog generation, not repository validation
			assert.NotContains(t, err.Error(), "repository path")
			assert.NotContains(t, err.Error(), "not a git repository")
		} else {
			// If it succeeds, we should get a result
			assert.NotNil(t, result)
		}
	})
}

func TestGenerateChangelogTool_PlaceholderOutput(t *testing.T) {
	// Enable the generate_changelog tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "generate_changelog")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &generatechangelog.GenerateChangelogTool{}

	// Since we can't easily test the full Chronicle integration without a real GitHub repo,
	// let's test the placeholder functionality by creating a mock scenario

	t.Run("placeholder markdown output", func(t *testing.T) {
		// Create a temporary directory with .git subdirectory
		tmpDir, err := os.MkdirTemp("", "test_changelog")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		gitDir := filepath.Join(tmpDir, ".git")
		err = os.Mkdir(gitDir, 0700)
		require.NoError(t, err)

		args := map[string]any{
			"repository_path": tmpDir,
			"output_format":   "markdown",
			"title":           "Test Changelog",
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		// This will fail on Chronicle changelog generation but return a structured error response
		result, err := tool.Execute(ctx, logger, cache, args)

		// The tool now follows MCP pattern: no error, but result contains error info
		assert.NoError(t, err)
		assert.NotNil(t, result)

		// The result should be JSON formatted with error details
		if result != nil && len(result.Content) > 0 {
			textContent, ok := result.Content[0].(mcp.TextContent)
			assert.True(t, ok)
			// Should contain error about changelog generation failure
			assert.Contains(t, textContent.Text, "error")
		}
	})

	t.Run("placeholder json output", func(t *testing.T) {
		// Create a temporary directory with .git subdirectory
		tmpDir, err := os.MkdirTemp("", "test_changelog")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		gitDir := filepath.Join(tmpDir, ".git")
		err = os.Mkdir(gitDir, 0700)
		require.NoError(t, err)

		args := map[string]any{
			"repository_path": tmpDir,
			"output_format":   "json",
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		result, err := tool.Execute(ctx, logger, cache, args)

		// The tool now follows MCP pattern: no error, but result contains error info
		assert.NoError(t, err)
		assert.NotNil(t, result)

		// The result should be JSON formatted with error details
		if result != nil && len(result.Content) > 0 {
			textContent, ok := result.Content[0].(mcp.TextContent)
			assert.True(t, ok)
			// Should contain error about changelog generation failure
			assert.Contains(t, textContent.Text, "error")
		}
	})
}

func TestGenerateChangelogTool_GitHubIntegration(t *testing.T) {
	// Enable the generate_changelog tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "generate_changelog")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &generatechangelog.GenerateChangelogTool{}

	t.Run("github integration disabled by default", func(t *testing.T) {
		// Create a temporary directory with .git subdirectory
		tmpDir, err := os.MkdirTemp("", "test_github_disabled")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		gitDir := filepath.Join(tmpDir, ".git")
		err = os.Mkdir(gitDir, 0700)
		require.NoError(t, err)

		args := map[string]any{
			"repository_path":           tmpDir,
			"output_format":             "markdown",
			"enable_github_integration": false,
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		result, err := tool.Execute(ctx, logger, cache, args)

		// Should not error on GitHub integration parameter parsing
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("github integration enabled without token", func(t *testing.T) {
		// Save original environment variable
		originalToken := os.Getenv("GITHUB_TOKEN")
		defer func() {
			if originalToken != "" {
				_ = os.Setenv("GITHUB_TOKEN", originalToken)
			} else {
				_ = os.Unsetenv("GITHUB_TOKEN")
			}
		}()

		// Ensure no GitHub token is set
		_ = os.Unsetenv("GITHUB_TOKEN")

		// Create a temporary directory with .git subdirectory
		tmpDir, err := os.MkdirTemp("", "test_github_no_token")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		gitDir := filepath.Join(tmpDir, ".git")
		err = os.Mkdir(gitDir, 0700)
		require.NoError(t, err)

		args := map[string]any{
			"repository_path":           tmpDir,
			"output_format":             "markdown",
			"enable_github_integration": true,
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		result, err := tool.Execute(ctx, logger, cache, args)

		// Should not error (graceful fallback to local git)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("github integration enabled with token", func(t *testing.T) {
		// Save original environment variable
		originalToken := os.Getenv("GITHUB_TOKEN")
		defer func() {
			if originalToken != "" {
				_ = os.Setenv("GITHUB_TOKEN", originalToken)
			} else {
				_ = os.Unsetenv("GITHUB_TOKEN")
			}
		}()

		// Set a mock GitHub token (doesn't need to be valid for this test)
		_ = os.Setenv("GITHUB_TOKEN", "mock_token_for_testing")

		// Create a temporary directory with .git subdirectory
		tmpDir, err := os.MkdirTemp("", "test_github_with_token")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		gitDir := filepath.Join(tmpDir, ".git")
		err = os.Mkdir(gitDir, 0700)
		require.NoError(t, err)

		args := map[string]any{
			"repository_path":           tmpDir,
			"output_format":             "markdown",
			"enable_github_integration": true,
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		result, err := tool.Execute(ctx, logger, cache, args)

		// Should not error (should gracefully fall back to local git even with token)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("github integration parameter parsing", func(t *testing.T) {
		// Test various parameter combinations
		testCases := []struct {
			name    string
			args    map[string]any
			wantErr bool
		}{
			{
				name: "github integration true",
				args: map[string]any{
					"repository_path":           "/tmp/test",
					"enable_github_integration": true,
				},
				wantErr: false, // Should not error on parameter parsing
			},
			{
				name: "github integration false",
				args: map[string]any{
					"repository_path":           "/tmp/test",
					"enable_github_integration": false,
				},
				wantErr: false,
			},
			{
				name: "github integration omitted (defaults to false)",
				args: map[string]any{
					"repository_path": "/tmp/test",
				},
				wantErr: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.Background()
				logger := logrus.New()
				cache := &sync.Map{}

				result, err := tool.Execute(ctx, logger, cache, tc.args)

				if tc.wantErr {
					assert.Error(t, err)
					assert.Nil(t, result)
				} else {
					// We expect repository validation to fail, not parameter parsing
					if err != nil {
						assert.Contains(t, err.Error(), "invalid repository")
					}
					// The result might be nil due to repository validation failure,
					// but we don't assert on it since it depends on whether validation passes
					_ = result
				}
			})
		}
	})
}

func TestGenerateChangelogTool_ExtendedHelp(t *testing.T) {
	tool := &generatechangelog.GenerateChangelogTool{}

	// Test that the tool implements ExtendedHelpProvider
	helpProvider, ok := any(tool).(interface {
		ProvideExtendedInfo() any
	})

	if !ok {
		t.Skip("Tool does not implement ExtendedHelpProvider interface")
	}

	help := helpProvider.ProvideExtendedInfo()
	assert.NotNil(t, help, "Extended help should not be nil")

	// Basic validation that help contains expected information
	helpJSON, err := json.Marshal(help)
	require.NoError(t, err)

	helpStr := string(helpJSON)
	assert.Contains(t, helpStr, "repository_path")
	assert.Contains(t, helpStr, "GitHub")
	assert.Contains(t, helpStr, "changelog")
}

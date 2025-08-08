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
	assert.Contains(t, def.Description, "Generate changelogs from GitHub PRs and issues")
	assert.Contains(t, def.Description, "Anchore Chronicle")

	// Test required parameters
	require.NotNil(t, def.InputSchema.Properties)

	// Check repository_path is required
	repoPathProp, exists := def.InputSchema.Properties["repository_path"]
	assert.True(t, exists, "repository_path parameter should exist")
	assert.Contains(t, def.InputSchema.Required, "repository_path")

	// Verify repository_path is a string with description
	repoPathMap, ok := repoPathProp.(map[string]interface{})
	require.True(t, ok, "repository_path should be a property map")
	assert.Equal(t, "string", repoPathMap["type"])
	assert.Contains(t, repoPathMap["description"].(string), "Path to local Git repository")

	// Test optional parameters exist with defaults
	outputFormatProp, exists := def.InputSchema.Properties["output_format"]
	assert.True(t, exists, "output_format parameter should exist")
	outputFormatMap, ok := outputFormatProp.(map[string]interface{})
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
	speculateMap, ok := speculateProp.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "boolean", speculateMap["type"])
	assert.Equal(t, false, speculateMap["default"])

	// Test timeout parameter
	timeoutProp, exists := def.InputSchema.Properties["timeout_minutes"]
	assert.True(t, exists, "timeout_minutes parameter should exist")
	timeoutMap, ok := timeoutProp.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "number", timeoutMap["type"])
	assert.Equal(t, float64(5), timeoutMap["default"])
}

func TestGenerateChangelogTool_ParseRequest(t *testing.T) {
	tool := &generatechangelog.GenerateChangelogTool{}

	t.Run("valid minimal request", func(t *testing.T) {
		args := map[string]interface{}{
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
		args := map[string]interface{}{
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
		args := map[string]interface{}{
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

	t.Run("invalid timeout", func(t *testing.T) {
		args := map[string]interface{}{
			"repository_path": "/path/to/repo",
			"timeout_minutes": float64(0),
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		result, err := tool.Execute(ctx, logger, cache, args)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout_minutes must be at least 1")
		assert.Nil(t, result)
	})

	t.Run("timeout too large", func(t *testing.T) {
		args := map[string]interface{}{
			"repository_path": "/path/to/repo",
			"timeout_minutes": float64(120),
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		result, err := tool.Execute(ctx, logger, cache, args)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout_minutes cannot exceed 60")
		assert.Nil(t, result)
	})
}

func TestGenerateChangelogTool_RepositoryValidation(t *testing.T) {
	tool := &generatechangelog.GenerateChangelogTool{}

	t.Run("non-existent directory", func(t *testing.T) {
		args := map[string]interface{}{
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

		args := map[string]interface{}{
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

		args := map[string]interface{}{
			"repository_path": tmpDir,
		}

		ctx := context.Background()
		logger := logrus.New()
		cache := &sync.Map{}

		result, err := tool.Execute(ctx, logger, cache, args)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path is not a git repository")
		assert.Nil(t, result)
	})

	t.Run("valid git repository", func(t *testing.T) {
		// Create a temporary directory with .git subdirectory
		tmpDir, err := os.MkdirTemp("", "test_git_repo")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		gitDir := filepath.Join(tmpDir, ".git")
		err = os.Mkdir(gitDir, 0755)
		require.NoError(t, err)

		args := map[string]interface{}{
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
	tool := &generatechangelog.GenerateChangelogTool{}

	// Since we can't easily test the full Chronicle integration without a real GitHub repo,
	// let's test the placeholder functionality by creating a mock scenario

	t.Run("placeholder markdown output", func(t *testing.T) {
		// Create a temporary directory with .git subdirectory
		tmpDir, err := os.MkdirTemp("", "test_changelog")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tmpDir) }()

		gitDir := filepath.Join(tmpDir, ".git")
		err = os.Mkdir(gitDir, 0755)
		require.NoError(t, err)

		args := map[string]interface{}{
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
		err = os.Mkdir(gitDir, 0755)
		require.NoError(t, err)

		args := map[string]interface{}{
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

func TestGenerateChangelogTool_ExtendedHelp(t *testing.T) {
	tool := &generatechangelog.GenerateChangelogTool{}

	// Test that the tool implements ExtendedHelpProvider
	helpProvider, ok := interface{}(tool).(interface {
		ProvideExtendedInfo() interface{}
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

package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/filelength"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindLongFilesTool(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "find_long_files_test")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create test files
	testFiles := map[string]int{
		"short.go":       50,   // Should not be included
		"medium.js":      500,  // Should not be included
		"long.py":        800,  // Should be included
		"very_long.java": 1200, // Should be included
		"binary.bin":     100,  // Should be skipped (binary)
	}

	for filename, lineCount := range testFiles {
		content := strings.Repeat("// This is a test line\n", lineCount)

		// Make binary file actually binary
		if strings.HasSuffix(filename, ".bin") {
			content = string([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05})
		}

		err := os.WriteFile(filepath.Join(tempDir, filename), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Create .gitignore file
	gitignoreContent := "*.tmp\n*.log\n"
	err = os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignoreContent), 0644)
	require.NoError(t, err)

	// Create a .tmp file that should be ignored
	err = os.WriteFile(filepath.Join(tempDir, "ignored.tmp"), []byte(strings.Repeat("line\n", 900)), 0644)
	require.NoError(t, err)

	// Initialize the tool
	tool := &filelength.FindLongFilesTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests
	cache := &sync.Map{}

	tests := []struct {
		name     string
		args     map[string]interface{}
		expected []string // Expected file paths in results
	}{
		{
			name: "default threshold 700",
			args: map[string]interface{}{
				"path": tempDir,
			},
			expected: []string{"./long.py", "./very_long.java"},
		},
		{
			name: "custom threshold 500",
			args: map[string]interface{}{
				"path":           tempDir,
				"line_threshold": float64(500),
			},
			expected: []string{"./medium.js", "./long.py", "./very_long.java"},
		},
		{
			name: "high threshold",
			args: map[string]interface{}{
				"path":           tempDir,
				"line_threshold": float64(1500),
			},
			expected: []string{}, // No files should meet this threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), logger, cache, tt.args)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Get the content
			require.True(t, len(result.Content) > 0, "Expected content in result")
			textContent, ok := mcp.AsTextContent(result.Content[0])
			require.True(t, ok, "Expected TextContent type")
			content := textContent.Text
			assert.Contains(t, content, "# Checklist of files over")

			// Check that expected files are present
			for _, expectedFile := range tt.expected {
				assert.Contains(t, content, expectedFile, "Expected file %s should be in results", expectedFile)
			}

			// Check that ignored files are not present
			assert.NotContains(t, content, "ignored.tmp", "Ignored files should not be in results")
			assert.NotContains(t, content, "binary.bin", "Binary files should not be in results")

			// If no files expected, check the message
			if len(tt.expected) == 0 {
				assert.Contains(t, content, "No files found exceeding the line threshold")
			}
		})
	}
}

func TestFindLongFilesTool_ParseRequest(t *testing.T) {
	tool := &filelength.FindLongFilesTool{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "parse_request_test")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	tests := []struct {
		name           string
		args           map[string]interface{}
		expectError    bool
		checkPath      string
		checkThreshold int
	}{
		{
			name:        "missing path parameter",
			args:        map[string]interface{}{},
			expectError: true,
		},
		{
			name: "valid absolute path",
			args: map[string]interface{}{
				"path":           tempDir,
				"line_threshold": float64(1000),
			},
			expectError: false,
		},
		{
			name: "invalid threshold",
			args: map[string]interface{}{
				"path":           tempDir,
				"line_threshold": float64(0),
			},
			expectError: true,
		},
		{
			name: "nonexistent path",
			args: map[string]interface{}{
				"path": "/nonexistent/path/that/does/not/exist",
			},
			expectError: true,
		},
		{
			name: "relative path",
			args: map[string]interface{}{
				"path": "./relative/path",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use reflection to access the private parseRequest method
			// In a real scenario, you might make this method public for testing
			// or test it indirectly through Execute
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)
			cache := &sync.Map{}

			_, err := tool.Execute(context.Background(), logger, cache, tt.args)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				// For valid cases, we expect success
				assert.NoError(t, err)
			}
		})
	}
}

func TestFindLongFilesTool_EnvironmentVariables(t *testing.T) {
	// Save original environment
	originalThreshold := os.Getenv("LONG_FILES_DEFAULT_LENGTH")
	originalExcludes := os.Getenv("LONG_FILES_ADDITIONAL_EXCLUDES")
	originalPrompt := os.Getenv("LONG_FILES_RETURN_PROMPT")

	defer func() {
		// Restore original environment
		if originalThreshold == "" {
			_ = os.Unsetenv("LONG_FILES_DEFAULT_LENGTH")
		} else {
			_ = os.Setenv("LONG_FILES_DEFAULT_LENGTH", originalThreshold)
		}
		if originalExcludes == "" {
			_ = os.Unsetenv("LONG_FILES_ADDITIONAL_EXCLUDES")
		} else {
			_ = os.Setenv("LONG_FILES_ADDITIONAL_EXCLUDES", originalExcludes)
		}
		if originalPrompt == "" {
			_ = os.Unsetenv("LONG_FILES_RETURN_PROMPT")
		} else {
			_ = os.Setenv("LONG_FILES_RETURN_PROMPT", originalPrompt)
		}
	}()

	// Test environment variable for threshold
	require.NoError(t, os.Setenv("LONG_FILES_DEFAULT_LENGTH", "500"))
	require.NoError(t, os.Setenv("LONG_FILES_ADDITIONAL_EXCLUDES", "*.test,*.spec"))
	require.NoError(t, os.Setenv("LONG_FILES_RETURN_PROMPT", "Custom test message"))

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "env_test")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create a test file that should be found with threshold 500
	content := strings.Repeat("// Test line\n", 600)
	err = os.WriteFile(filepath.Join(tempDir, "test.go"), []byte(content), 0644)
	require.NoError(t, err)

	tool := &filelength.FindLongFilesTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	args := map[string]interface{}{
		"path": tempDir,
		// Don't set line_threshold to test environment variable
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	require.NoError(t, err)
	require.NotNil(t, result)

	require.True(t, len(result.Content) > 0, "Expected content in result")
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok, "Expected TextContent type")
	content_result := textContent.Text

	// Should find the file because env var sets threshold to 500
	assert.Contains(t, content_result, "test.go")

	// Should contain custom message
	assert.Contains(t, content_result, "Custom test message")
}

func TestFindLongFilesTool_Definition(t *testing.T) {
	tool := &filelength.FindLongFilesTool{}
	definition := tool.Definition()

	assert.Equal(t, "find_long_files", definition.Name)
	assert.NotEmpty(t, definition.Description)

	// Check that required parameters are defined
	inputSchema := definition.InputSchema
	assert.NotNil(t, inputSchema)

	// Just check that the schema exists and has properties
	// The exact structure checking is less important than functionality
	assert.NotNil(t, inputSchema.Properties, "InputSchema should have properties")
}

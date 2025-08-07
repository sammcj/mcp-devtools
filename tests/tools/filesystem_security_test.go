package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/filesystem"
	"github.com/sammcj/mcp-devtools/tests/testutils"
	"github.com/sirupsen/logrus"
)

func TestFilesystem_DefaultLimits(t *testing.T) {
	// Enable the filesystem tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "filesystem")

	// Save original environment variables
	originalMaxFileSize := os.Getenv("FILESYSTEM_MAX_FILE_SIZE")
	originalSecurePermissions := os.Getenv("FILESYSTEM_SECURE_PERMISSIONS")
	defer func() {
		if originalMaxFileSize == "" {
			_ = os.Unsetenv("FILESYSTEM_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("FILESYSTEM_MAX_FILE_SIZE", originalMaxFileSize)
		}
		if originalSecurePermissions == "" {
			_ = os.Unsetenv("FILESYSTEM_SECURE_PERMISSIONS")
		} else {
			_ = os.Setenv("FILESYSTEM_SECURE_PERMISSIONS", originalSecurePermissions)
		}
	}()

	// Clear environment variables to test defaults
	_ = os.Unsetenv("FILESYSTEM_MAX_FILE_SIZE")
	_ = os.Unsetenv("FILESYSTEM_SECURE_PERMISSIONS")

	// Create test directory
	testDir := t.TempDir()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise during tests

	tool := &filesystem.FileSystemTool{}
	tool.LoadSecurityConfig()
	tool.SetAllowedDirectories([]string{testDir})

	// Test that we can create and write files with default security settings
	ctx := context.Background()
	cache := &sync.Map{}

	// Test small file write (should succeed)
	smallContent := "test content"
	args := map[string]interface{}{
		"function": "write_file",
		"options": map[string]interface{}{
			"path":    filepath.Join(testDir, "test_small.txt"),
			"content": smallContent,
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Verify file was created with secure permissions (0600)
	info, err := os.Stat(filepath.Join(testDir, "test_small.txt"))
	testutils.AssertNoError(t, err)
	testutils.AssertEqual(t, os.FileMode(0600), info.Mode().Perm())
}

func TestFilesystem_CustomFileSizeLimit(t *testing.T) {
	// Enable the filesystem tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "filesystem")

	// Save original environment variable
	originalMaxFileSize := os.Getenv("FILESYSTEM_MAX_FILE_SIZE")
	defer func() {
		if originalMaxFileSize == "" {
			_ = os.Unsetenv("FILESYSTEM_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("FILESYSTEM_MAX_FILE_SIZE", originalMaxFileSize)
		}
	}()

	// Set custom file size limit (1KB = 1024 bytes)
	err := os.Setenv("FILESYSTEM_MAX_FILE_SIZE", "1024")
	testutils.AssertNoError(t, err)

	// Create test directory
	testDir := t.TempDir()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	tool := &filesystem.FileSystemTool{}
	tool.LoadSecurityConfig()
	tool.SetAllowedDirectories([]string{testDir})

	ctx := context.Background()
	cache := &sync.Map{}

	// Test small content (should succeed)
	smallContent := "small"
	args := map[string]interface{}{
		"function": "write_file",
		"options": map[string]interface{}{
			"path":    filepath.Join(testDir, "small.txt"),
			"content": smallContent,
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Test large content (should fail)
	largeContent := strings.Repeat("a", 2048) // 2KB, exceeds 1KB limit
	args = map[string]interface{}{
		"function": "write_file",
		"options": map[string]interface{}{
			"path":    filepath.Join(testDir, "large.txt"),
			"content": largeContent,
		},
	}

	_, err = tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertTrue(t, strings.Contains(err.Error(), "content size validation failed"))
	testutils.AssertTrue(t, strings.Contains(err.Error(), "exceeds maximum allowed size"))
}

func TestFilesystem_CustomFilePermissions(t *testing.T) {
	// Save original environment variable
	originalSecurePermissions := os.Getenv("FILESYSTEM_SECURE_PERMISSIONS")
	defer func() {
		if originalSecurePermissions == "" {
			_ = os.Unsetenv("FILESYSTEM_SECURE_PERMISSIONS")
		} else {
			_ = os.Setenv("FILESYSTEM_SECURE_PERMISSIONS", originalSecurePermissions)
		}
	}()

	// Set custom file permissions (0644 - read/write for owner, read for others)
	err := os.Setenv("FILESYSTEM_SECURE_PERMISSIONS", "644")
	testutils.AssertNoError(t, err)

	// Create test directory
	testDir := t.TempDir()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	tool := &filesystem.FileSystemTool{}
	tool.LoadSecurityConfig()
	tool.SetAllowedDirectories([]string{testDir})

	ctx := context.Background()
	cache := &sync.Map{}

	// Write a file
	args := map[string]interface{}{
		"function": "write_file",
		"options": map[string]interface{}{
			"path":    filepath.Join(testDir, "custom_perms.txt"),
			"content": "test content",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Verify file was created with custom permissions (0644)
	info, err := os.Stat(filepath.Join(testDir, "custom_perms.txt"))
	testutils.AssertNoError(t, err)
	testutils.AssertEqual(t, os.FileMode(0644), info.Mode().Perm())
}

func TestFilesystem_ReadFileSizeValidation(t *testing.T) {
	// Save original environment variable
	originalMaxFileSize := os.Getenv("FILESYSTEM_MAX_FILE_SIZE")
	defer func() {
		if originalMaxFileSize == "" {
			_ = os.Unsetenv("FILESYSTEM_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("FILESYSTEM_MAX_FILE_SIZE", originalMaxFileSize)
		}
	}()

	// Set small file size limit for testing
	err := os.Setenv("FILESYSTEM_MAX_FILE_SIZE", "50")
	testutils.AssertNoError(t, err)

	// Create test directory
	testDir := t.TempDir()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	tool := &filesystem.FileSystemTool{}
	tool.LoadSecurityConfig()
	tool.SetAllowedDirectories([]string{testDir})

	// Create a large file manually (bypassing our size limits)
	largeFilePath := filepath.Join(testDir, "large_file.txt")
	largeContent := strings.Repeat("a", 100) // 100 bytes, exceeds 50 byte limit
	err = os.WriteFile(largeFilePath, []byte(largeContent), 0644)
	testutils.AssertNoError(t, err)

	ctx := context.Background()
	cache := &sync.Map{}

	// Try to read the large file (should fail)
	args := map[string]interface{}{
		"function": "read_file",
		"options": map[string]interface{}{
			"path": largeFilePath,
		},
	}

	_, err = tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertTrue(t, strings.Contains(err.Error(), "file size validation failed"))
}

func TestFilesystem_EditFileSizeValidation(t *testing.T) {
	// Save original environment variable
	originalMaxFileSize := os.Getenv("FILESYSTEM_MAX_FILE_SIZE")
	defer func() {
		if originalMaxFileSize == "" {
			_ = os.Unsetenv("FILESYSTEM_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("FILESYSTEM_MAX_FILE_SIZE", originalMaxFileSize)
		}
	}()

	// Set file size limit
	err := os.Setenv("FILESYSTEM_MAX_FILE_SIZE", "100")
	testutils.AssertNoError(t, err)

	// Create test directory
	testDir := t.TempDir()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	tool := &filesystem.FileSystemTool{}
	tool.LoadSecurityConfig()
	tool.SetAllowedDirectories([]string{testDir})

	// Create a file with content
	testFilePath := filepath.Join(testDir, "edit_test.txt")
	originalContent := "short content"
	err = os.WriteFile(testFilePath, []byte(originalContent), 0644)
	testutils.AssertNoError(t, err)

	ctx := context.Background()
	cache := &sync.Map{}

	// Try to edit file to make it exceed size limit
	largeReplacement := strings.Repeat("x", 150) // 150 bytes, exceeds 100 byte limit
	args := map[string]interface{}{
		"function": "edit_file",
		"options": map[string]interface{}{
			"path": testFilePath,
			"edits": []interface{}{
				map[string]interface{}{
					"oldText": "short content",
					"newText": largeReplacement,
				},
			},
		},
	}

	_, err = tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertTrue(t, strings.Contains(err.Error(), "modified content size validation failed"))
}

func TestFilesystem_InvalidEnvironmentVariables(t *testing.T) {
	// Save original environment variables
	originalMaxFileSize := os.Getenv("FILESYSTEM_MAX_FILE_SIZE")
	originalSecurePermissions := os.Getenv("FILESYSTEM_SECURE_PERMISSIONS")
	defer func() {
		if originalMaxFileSize == "" {
			_ = os.Unsetenv("FILESYSTEM_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("FILESYSTEM_MAX_FILE_SIZE", originalMaxFileSize)
		}
		if originalSecurePermissions == "" {
			_ = os.Unsetenv("FILESYSTEM_SECURE_PERMISSIONS")
		} else {
			_ = os.Setenv("FILESYSTEM_SECURE_PERMISSIONS", originalSecurePermissions)
		}
	}()

	// Set invalid values - should fall back to defaults
	err := os.Setenv("FILESYSTEM_MAX_FILE_SIZE", "invalid")
	testutils.AssertNoError(t, err)
	err = os.Setenv("FILESYSTEM_SECURE_PERMISSIONS", "invalid")
	testutils.AssertNoError(t, err)

	// Create test directory
	testDir := t.TempDir()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	tool := &filesystem.FileSystemTool{}
	tool.LoadSecurityConfig()
	tool.SetAllowedDirectories([]string{testDir})

	ctx := context.Background()
	cache := &sync.Map{}

	// Should work with default settings despite invalid environment variables
	args := map[string]interface{}{
		"function": "write_file",
		"options": map[string]interface{}{
			"path":    filepath.Join(testDir, "test_invalid_env.txt"),
			"content": "test content",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Should use default permissions (0600)
	info, err := os.Stat(filepath.Join(testDir, "test_invalid_env.txt"))
	testutils.AssertNoError(t, err)
	testutils.AssertEqual(t, os.FileMode(0600), info.Mode().Perm())
}

func TestFilesystem_ZeroValues(t *testing.T) {
	// Save original environment variables
	originalMaxFileSize := os.Getenv("FILESYSTEM_MAX_FILE_SIZE")
	originalSecurePermissions := os.Getenv("FILESYSTEM_SECURE_PERMISSIONS")
	defer func() {
		if originalMaxFileSize == "" {
			_ = os.Unsetenv("FILESYSTEM_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("FILESYSTEM_MAX_FILE_SIZE", originalMaxFileSize)
		}
		if originalSecurePermissions == "" {
			_ = os.Unsetenv("FILESYSTEM_SECURE_PERMISSIONS")
		} else {
			_ = os.Setenv("FILESYSTEM_SECURE_PERMISSIONS", originalSecurePermissions)
		}
	}()

	// Set zero/negative values - should fall back to defaults
	err := os.Setenv("FILESYSTEM_MAX_FILE_SIZE", "0")
	testutils.AssertNoError(t, err)
	err = os.Setenv("FILESYSTEM_SECURE_PERMISSIONS", "0")
	testutils.AssertNoError(t, err)

	// Create test directory
	testDir := t.TempDir()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	tool := &filesystem.FileSystemTool{}
	tool.LoadSecurityConfig()
	tool.SetAllowedDirectories([]string{testDir})

	ctx := context.Background()
	cache := &sync.Map{}

	// Should work with default settings
	args := map[string]interface{}{
		"function": "write_file",
		"options": map[string]interface{}{
			"path":    filepath.Join(testDir, "test_zero_values.txt"),
			"content": "test content",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Should use custom permissions (0000 in this case)
	info, err := os.Stat(filepath.Join(testDir, "test_zero_values.txt"))
	testutils.AssertNoError(t, err)
	testutils.AssertEqual(t, os.FileMode(0000), info.Mode().Perm())
}

func TestFilesystem_Constants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, "FILESYSTEM_MAX_FILE_SIZE", filesystem.FilesystemMaxFileSizeEnvVar)
	testutils.AssertEqual(t, "FILESYSTEM_SECURE_PERMISSIONS", filesystem.FilesystemSecurePermissionsVar)
	testutils.AssertEqual(t, int64(2*1024*1024*1024), filesystem.DefaultMaxFileSize)
	testutils.AssertEqual(t, int(0600), int(filesystem.DefaultSecureFilePermissions))
}

func TestFilesystem_MultipleFilesSizeValidation(t *testing.T) {
	// Save original environment variable
	originalMaxFileSize := os.Getenv("FILESYSTEM_MAX_FILE_SIZE")
	defer func() {
		if originalMaxFileSize == "" {
			_ = os.Unsetenv("FILESYSTEM_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("FILESYSTEM_MAX_FILE_SIZE", originalMaxFileSize)
		}
	}()

	// Set small file size limit
	err := os.Setenv("FILESYSTEM_MAX_FILE_SIZE", "50")
	testutils.AssertNoError(t, err)

	// Create test directory
	testDir := t.TempDir()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	tool := &filesystem.FileSystemTool{}
	tool.LoadSecurityConfig()
	tool.SetAllowedDirectories([]string{testDir})

	// Create files with different sizes
	smallFilePath := filepath.Join(testDir, "small.txt")
	largeFilePath := filepath.Join(testDir, "large.txt")

	err = os.WriteFile(smallFilePath, []byte("small"), 0644)
	testutils.AssertNoError(t, err)

	err = os.WriteFile(largeFilePath, []byte(strings.Repeat("a", 100)), 0644)
	testutils.AssertNoError(t, err)

	ctx := context.Background()
	cache := &sync.Map{}

	// Try to read multiple files including one that exceeds size limit
	args := map[string]interface{}{
		"function": "read_multiple_files",
		"options": map[string]interface{}{
			"paths": []interface{}{smallFilePath, largeFilePath},
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err) // Should not fail completely
	testutils.AssertNotNil(t, result)

	// Should contain error message for the large file but succeed for small file
	textContent, ok := mcp.AsTextContent(result.Content[0])
	testutils.AssertTrue(t, ok)
	resultText := textContent.Text
	testutils.AssertTrue(t, strings.Contains(resultText, "small"))
	testutils.AssertTrue(t, strings.Contains(resultText, "file size validation failed"))
}

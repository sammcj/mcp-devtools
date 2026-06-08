package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/filesystem"
	"github.com/sirupsen/logrus"
)

// setupFilesystemTool creates a filesystem tool for testing with proper environment setup
func setupFilesystemTool(tempDir string) *filesystem.FileSystemTool {
	// Set environment variable to enable the tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "filesystem") // Ignore error in tests

	// Create tool and set allowed directories for testing
	tool := &filesystem.FileSystemTool{}
	tool.LoadSecurityConfig()
	tool.SetAllowedDirectories([]string{tempDir})
	return tool
}

// Helper function to extract text content from MCP result
func getTextContent(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}

	textContent, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		return ""
	}

	return textContent.Text
}

func TestFileSystemTool_Definition(t *testing.T) {
	tool := &filesystem.FileSystemTool{}
	def := tool.Definition()

	if def.Name != "filesystem" {
		t.Errorf("Expected tool name 'filesystem', got '%s'", def.Name)
	}

	if def.Description == "" {
		t.Error("Expected non-empty description")
	}
}

func TestFileSystemTool_ListAllowedDirectories(t *testing.T) {
	// Set environment variable to enable the tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "filesystem") // Ignore error in tests
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &filesystem.FileSystemTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests
	cache := &sync.Map{}

	args := map[string]any{
		"function": "list_allowed_directories",
		"options":  map[string]any{},
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check that result contains some directories
	content := getTextContent(result)
	if !strings.Contains(content, "Allowed directories:") {
		t.Errorf("Expected result to contain 'Allowed directories:', got: %s", content)
	}
}

func TestFileSystemTool_CreateAndReadFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filesystem_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create tool with temp directory as allowed
	tool := setupFilesystemTool(tempDir)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Test file path
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!\nThis is a test file."

	// Test write_file
	writeArgs := map[string]any{
		"function": "write_file",
		"options": map[string]any{
			"path":    testFile,
			"content": testContent,
		},
	}

	result, err := tool.Execute(context.Background(), logger, cache, writeArgs)
	if err != nil {
		t.Fatalf("Write file failed: %v", err)
	}

	content := getTextContent(result)
	if !strings.Contains(content, "Successfully wrote to") {
		t.Errorf("Expected success message, got: %s", content)
	}

	// Test read_file
	readArgs := map[string]any{
		"function": "read_file",
		"options": map[string]any{
			"path": testFile,
		},
	}

	result, err = tool.Execute(context.Background(), logger, cache, readArgs)
	if err != nil {
		t.Fatalf("Read file failed: %v", err)
	}

	content = getTextContent(result)
	if content != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, content)
	}
}

func TestFileSystemTool_CreateDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filesystem_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	tool := setupFilesystemTool(tempDir)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Test directory path
	testDir := filepath.Join(tempDir, "subdir", "nested")

	// Test create_directory
	args := map[string]any{
		"function": "create_directory",
		"options": map[string]any{
			"path": testDir,
		},
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	if err != nil {
		t.Fatalf("Create directory failed: %v", err)
	}

	content := getTextContent(result)
	if !strings.Contains(content, "Successfully created directory") {
		t.Errorf("Expected success message, got: %s", content)
	}

	// Verify directory was created
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}
}

func TestFileSystemTool_ListDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filesystem_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create some test files and directories
	testFile := filepath.Join(tempDir, "test.txt")
	testSubDir := filepath.Join(tempDir, "subdir")
	testIgnoredDir := filepath.Join(tempDir, ".venv")
	testIgnoredFile := filepath.Join(tempDir, "debug.log")
	testGitDir := filepath.Join(tempDir, ".git")
	testGitignoreFile := filepath.Join(tempDir, ".gitignore")

	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := os.Mkdir(testSubDir, 0700); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	if err := os.Mkdir(testIgnoredDir, 0700); err != nil {
		t.Fatalf("Failed to create ignored test directory: %v", err)
	}
	if err := os.Mkdir(testGitDir, 0700); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}
	if err := os.WriteFile(testIgnoredFile, []byte("ignored"), 0600); err != nil {
		t.Fatalf("Failed to create ignored test file: %v", err)
	}
	if err := os.WriteFile(testGitignoreFile, []byte(".venv/\n*.log\n"), 0600); err != nil {
		t.Fatalf("Failed to create .gitignore file: %v", err)
	}

	tool := setupFilesystemTool(tempDir)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Test list_directory
	args := map[string]any{
		"function": "list_directory",
		"options": map[string]any{
			"path": tempDir,
		},
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	if err != nil {
		t.Fatalf("List directory failed: %v", err)
	}

	content := getTextContent(result)
	if !strings.Contains(content, "[FILE] test.txt") {
		t.Errorf("Expected to find '[FILE] test.txt' in output: %s", content)
	}

	if !strings.Contains(content, "[DIR] subdir") {
		t.Errorf("Expected to find '[DIR] subdir' in output: %s", content)
	}
	if !strings.Contains(content, "[FILE] .gitignore") {
		t.Errorf("Expected '.gitignore' to be listed, got: %s", content)
	}
	if strings.Contains(content, ".venv") {
		t.Errorf("Expected '.venv' to be filtered by .gitignore, got: %s", content)
	}
	if strings.Contains(content, "debug.log") {
		t.Errorf("Expected 'debug.log' to be filtered by .gitignore, got: %s", content)
	}
	if strings.Contains(content, "[DIR] .git") {
		t.Errorf("Expected '.git' directory to be filtered, got: %s", content)
	}
}

func TestFileSystemTool_ListDirectoryWithSizes_RespectsGitignore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filesystem_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	if err := os.WriteFile(filepath.Join(tempDir, "visible.txt"), []byte("visible"), 0600); err != nil {
		t.Fatalf("Failed to create visible file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "ignore.log"), []byte("ignored"), 0600); err != nil {
		t.Fatalf("Failed to create ignored file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte("*.log\n"), 0600); err != nil {
		t.Fatalf("Failed to create .gitignore file: %v", err)
	}

	tool := setupFilesystemTool(tempDir)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	args := map[string]any{
		"function": "list_directory_with_sizes",
		"options": map[string]any{
			"path": tempDir,
		},
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	if err != nil {
		t.Fatalf("List directory with sizes failed: %v", err)
	}

	content := getTextContent(result)
	if !strings.Contains(content, "visible.txt") {
		t.Errorf("Expected to find 'visible.txt' in output: %s", content)
	}
	if strings.Contains(content, "ignore.log") {
		t.Errorf("Expected 'ignore.log' to be filtered by .gitignore, got: %s", content)
	}
}

// TestFileSystemTool_ListDirectory_InheritsParentGitignore verifies that a
// .gitignore in a parent directory (within the allowed boundary) is applied
// when listing a nested subdirectory.
func TestFileSystemTool_ListDirectory_InheritsParentGitignore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filesystem_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	subDir := filepath.Join(tempDir, "sub")
	if err := os.Mkdir(subDir, 0700); err != nil {
		t.Fatalf("Failed to create sub directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte("*.log\n"), 0600); err != nil {
		t.Fatalf("Failed to create parent .gitignore file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "app.log"), []byte("log"), 0600); err != nil {
		t.Fatalf("Failed to create ignored file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "main.go"), []byte("package main"), 0600); err != nil {
		t.Fatalf("Failed to create visible file: %v", err)
	}

	tool := setupFilesystemTool(tempDir)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	args := map[string]any{
		"function": "list_directory",
		"options": map[string]any{
			"path": subDir,
		},
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	if err != nil {
		t.Fatalf("List directory failed: %v", err)
	}

	content := getTextContent(result)
	if !strings.Contains(content, "main.go") {
		t.Errorf("Expected to find 'main.go' in output: %s", content)
	}
	if strings.Contains(content, "app.log") {
		t.Errorf("Expected 'app.log' to be filtered by parent .gitignore, got: %s", content)
	}
}

func TestFileSystemTool_GetFileInfo(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filesystem_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!"

	if err := os.WriteFile(testFile, []byte(testContent), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := setupFilesystemTool(tempDir)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Test get_file_info
	args := map[string]any{
		"function": "get_file_info",
		"options": map[string]any{
			"path": testFile,
		},
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	if err != nil {
		t.Fatalf("Get file info failed: %v", err)
	}

	content := getTextContent(result)
	expectedStrings := []string{
		"Path:",
		"Size:",
		"Type: File",
		"Permissions:",
		"Modified:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(content, expected) {
			t.Errorf("Expected to find '%s' in output: %s", expected, content)
		}
	}
}

func TestFileSystemTool_ReadFileHead(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "filesystem_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a test file with multiple lines
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"

	if err := os.WriteFile(testFile, []byte(testContent), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := setupFilesystemTool(tempDir)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Test read_file with head option
	args := map[string]any{
		"function": "read_file",
		"options": map[string]any{
			"path": testFile,
			"head": float64(3), // JSON numbers are float64
		},
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	if err != nil {
		t.Fatalf("Read file head failed: %v", err)
	}

	expected := "Line 1\nLine 2\nLine 3"
	content := getTextContent(result)
	if content != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, content)
	}
}

func TestFileSystemTool_InvalidFunction(t *testing.T) {
	tool := &filesystem.FileSystemTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	args := map[string]any{
		"function": "invalid_function",
		"options":  map[string]any{},
	}

	_, err := tool.Execute(context.Background(), logger, cache, args)
	if err == nil {
		t.Error("Expected error for invalid function")
	}

	if !strings.Contains(err.Error(), "unknown function") {
		t.Errorf("Expected 'unknown function' error, got: %v", err)
	}
}

func TestFileSystemTool_MissingParameters(t *testing.T) {
	tool := &filesystem.FileSystemTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Test missing function parameter
	args := map[string]any{
		"options": map[string]any{},
	}

	_, err := tool.Execute(context.Background(), logger, cache, args)
	if err == nil {
		t.Error("Expected error for missing function parameter")
	}

	// Test missing path parameter for read_file
	args = map[string]any{
		"function": "read_file",
		"options":  map[string]any{},
	}

	_, err = tool.Execute(context.Background(), logger, cache, args)
	if err == nil {
		t.Error("Expected error for missing path parameter")
	}
}

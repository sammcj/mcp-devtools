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
	tool := &filesystem.FileSystemTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests
	cache := &sync.Map{}

	args := map[string]interface{}{
		"function": "list_allowed_directories",
		"options":  map[string]interface{}{},
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
	tool := &filesystem.FileSystemTool{}
	tool.SetAllowedDirectories([]string{tempDir}) // Allow access to temp directory
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Test file path
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!\nThis is a test file."

	// Test write_file
	writeArgs := map[string]interface{}{
		"function": "write_file",
		"options": map[string]interface{}{
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
	readArgs := map[string]interface{}{
		"function": "read_file",
		"options": map[string]interface{}{
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

	tool := &filesystem.FileSystemTool{}
	tool.SetAllowedDirectories([]string{tempDir}) // Allow access to temp directory
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Test directory path
	testDir := filepath.Join(tempDir, "subdir", "nested")

	// Test create_directory
	args := map[string]interface{}{
		"function": "create_directory",
		"options": map[string]interface{}{
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

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := os.Mkdir(testSubDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tool := &filesystem.FileSystemTool{}
	tool.SetAllowedDirectories([]string{tempDir}) // Allow access to temp directory
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Test list_directory
	args := map[string]interface{}{
		"function": "list_directory",
		"options": map[string]interface{}{
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

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := &filesystem.FileSystemTool{}
	tool.SetAllowedDirectories([]string{tempDir}) // Allow access to temp directory
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Test get_file_info
	args := map[string]interface{}{
		"function": "get_file_info",
		"options": map[string]interface{}{
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

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := &filesystem.FileSystemTool{}
	tool.SetAllowedDirectories([]string{tempDir}) // Allow access to temp directory
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Test read_file with head option
	args := map[string]interface{}{
		"function": "read_file",
		"options": map[string]interface{}{
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

	args := map[string]interface{}{
		"function": "invalid_function",
		"options":  map[string]interface{}{},
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
	args := map[string]interface{}{
		"options": map[string]interface{}{},
	}

	_, err := tool.Execute(context.Background(), logger, cache, args)
	if err == nil {
		t.Error("Expected error for missing function parameter")
	}

	// Test missing path parameter for read_file
	args = map[string]interface{}{
		"function": "read_file",
		"options":  map[string]interface{}{},
	}

	_, err = tool.Execute(context.Background(), logger, cache, args)
	if err == nil {
		t.Error("Expected error for missing path parameter")
	}
}

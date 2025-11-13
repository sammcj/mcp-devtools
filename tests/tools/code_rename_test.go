package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/code_rename"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestCodeRenameTool_Definition(t *testing.T) {
	tool := &code_rename.CodeRenameTool{}
	definition := tool.Definition()

	testutils.AssertEqual(t, "code_rename", definition.Name)
	testutils.AssertNotNil(t, definition.Description)

	// Test that description contains key phrases
	desc := definition.Description
	if !testutils.Contains(desc, "rename") && !testutils.Contains(desc, "symbol") {
		t.Errorf("Expected description to contain 'rename' or 'symbol', got: %s", desc)
	}

	// Test input schema exists
	testutils.AssertNotNil(t, definition.InputSchema)
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filePath string
		expected string
	}{
		{"/path/to/main.go", "go"},
		{"/path/to/script.ts", "typescript"},
		{"/path/to/app.tsx", "typescript"},
		{"/path/to/index.js", "javascript"},
		{"/path/to/component.jsx", "javascript"},
		{"/path/to/app.py", "python"},
		{"/path/to/lib.rs", "rust"},
		{"/path/to/script.sh", "bash"},
		{"/path/to/index.html", "html"},
		{"/path/to/unknown.txt", ""},
	}

	for _, test := range tests {
		t.Run(test.filePath, func(t *testing.T) {
			result := code_rename.DetectLanguage(test.filePath)
			testutils.AssertEqual(t, test.expected, result)
		})
	}
}

func TestServerCache(t *testing.T) {
	cache := code_rename.NewServerCache()

	// Initially empty
	_, exists := cache.IsAvailable("gopls")
	testutils.AssertEqual(t, false, exists)

	// Set available
	cache.SetAvailable("gopls", true)
	available, exists := cache.IsAvailable("gopls")
	testutils.AssertEqual(t, true, exists)
	testutils.AssertEqual(t, true, available)

	// Set unavailable
	cache.SetAvailable("typescript-language-server", false)
	available, exists = cache.IsAvailable("typescript-language-server")
	testutils.AssertEqual(t, true, exists)
	testutils.AssertEqual(t, false, available)
}

func TestCodeRenameTool_Execute_MissingParameters(t *testing.T) {
	tool := &code_rename.CodeRenameTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tests := []struct {
		name        string
		args        map[string]any
		expectedErr string
	}{
		{
			name:        "missing file_path",
			args:        map[string]any{},
			expectedErr: "missing required parameter: file_path",
		},
		{
			name: "missing old_name",
			args: map[string]any{
				"file_path": "/path/to/file.go",
			},
			expectedErr: "missing required parameter: old_name",
		},
		{
			name: "missing new_name",
			args: map[string]any{
				"file_path": "/path/to/file.go",
				"old_name":  "oldSymbol",
			},
			expectedErr: "missing required parameter: new_name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := tool.Execute(ctx, logger, cache, test.args)
			testutils.AssertError(t, err)
			testutils.AssertErrorContains(t, err, test.expectedErr)
		})
	}
}

func TestCodeRenameTool_Execute_FileNotFound(t *testing.T) {
	tool := &code_rename.CodeRenameTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"file_path": "/nonexistent/file.go",
		"old_name":  "oldName",
		"new_name":  "newName",
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "file not found")
}

func TestCodeRenameTool_Execute_UnsupportedFileType(t *testing.T) {
	tool := &code_rename.CodeRenameTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create a temporary file with unsupported extension
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0600); err != nil {
		t.Fatal(err)
	}

	args := map[string]any{
		"file_path": tmpFile,
		"old_name":  "testSymbol",
		"new_name":  "newName",
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "unsupported file type")
}

// TestCodeRenameTool_Execute_NoLSPServer tests the scenario where no LSP server is available
func TestCodeRenameTool_Execute_RealRename(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LSP integration test in short mode")
	}

	tool := &code_rename.CodeRenameTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := context.Background()

	// Create a proper Go module workspace
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module testmodule\n\ngo 1.21\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create main.go with a function
	mainFile := filepath.Join(tmpDir, "main.go")
	mainContent := `package main

func calculateTotal(x int) int {
	return x * 2
}

func main() {
	result := calculateTotal(5)
	println(result)
}
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Create helper.go that also uses the function
	helperFile := filepath.Join(tmpDir, "helper.go")
	helperContent := `package main

func processValue() int {
	return calculateTotal(10)
}
`
	if err := os.WriteFile(helperFile, []byte(helperContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Test renaming calculateTotal to computeTotal
	args := map[string]any{
		"file_path": mainFile,
		"old_name":  "calculateTotal",
		"new_name":  "computeTotal",
		"preview":   true,
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	// If gopls is not installed, skip the test
	if err != nil && strings.Contains(err.Error(), "no LSP server available") {
		t.Skip("gopls not installed, skipping test")
	}

	// Otherwise, we expect success
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Verify the result structure - it's a *code_rename.RenameResult
	renameResult, ok := result.StructuredContent.(*code_rename.RenameResult)
	if !ok {
		t.Fatalf("Expected StructuredContent to be *code_rename.RenameResult, got %T", result.StructuredContent)
	}

	// Should have modified both files
	if renameResult.FilesModified != 2 {
		t.Errorf("Expected 2 files modified, got %d", renameResult.FilesModified)
	}

	// Should have at least 3 replacements (definition + 2 usages)
	if renameResult.TotalReplacements < 3 {
		t.Errorf("Expected at least 3 replacements, got %d", renameResult.TotalReplacements)
	}

	// Should not be applied (preview mode)
	if renameResult.Applied {
		t.Error("Expected applied to be false in preview mode")
	}

	// Should not have any errors
	if renameResult.Error != "" {
		t.Errorf("Expected no error, got: %s", renameResult.Error)
	}

	t.Logf("Rename successful: %d files, %d replacements", renameResult.FilesModified, renameResult.TotalReplacements)
}

func TestGetAvailableLanguages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LSP detection test in short mode")
	}

	ctx := context.Background()
	logger := testutils.CreateTestLogger()

	languages := code_rename.GetAvailableLanguages(ctx, logger)

	// Should return a list (possibly empty if no LSP servers installed)
	testutils.AssertNotNil(t, languages)

	// Log available languages for debugging
	t.Logf("Available LSP languages: %v", languages)
}

func TestCodeRenameTool_Execute_InvalidNewName(t *testing.T) {
	tool := &code_rename.CodeRenameTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")
	content := `package main

func main() {
	x := 42
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		newName     string
		expectedErr string
	}{
		{
			name:        "name with spaces",
			newName:     "new name",
			expectedErr: "invalid new_name",
		},
		{
			name:        "name starting with digit",
			newName:     "1name",
			expectedErr: "invalid new_name",
		},
		{
			name:        "name with special characters",
			newName:     "name-with-dash",
			expectedErr: "invalid new_name",
		},
		{
			name:        "empty name",
			newName:     "",
			expectedErr: "missing required parameter: new_name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := map[string]any{
				"file_path": tmpFile,
				"old_name":  "x",
				"new_name":  test.newName,
			}

			_, err := tool.Execute(ctx, logger, cache, args)
			testutils.AssertError(t, err)
			testutils.AssertErrorContains(t, err, test.expectedErr)
		})
	}
}

func TestCodeRenameTool_ProvideExtendedInfo(t *testing.T) {
	tool := &code_rename.CodeRenameTool{}
	info := tool.ProvideExtendedInfo()

	testutils.AssertNotNil(t, info)
	testutils.AssertTrue(t, len(info.Examples) > 0)
	testutils.AssertTrue(t, len(info.CommonPatterns) > 0)
	testutils.AssertTrue(t, len(info.Troubleshooting) > 0)
	testutils.AssertTrue(t, len(info.ParameterDetails) > 0)
	testutils.AssertTrue(t, info.WhenToUse != "")
	testutils.AssertTrue(t, info.WhenNotToUse != "")
}

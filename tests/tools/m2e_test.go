package tools_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/m2e"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestM2ETool_Definition(t *testing.T) {
	tool := &m2e.M2ETool{}
	definition := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "murican_to_english", definition.Name)
	testutils.AssertNotNil(t, definition.Description)

	// Test that description contains key phrases
	desc := definition.Description
	if !testutils.Contains(desc, "American English text to standard International") {
		t.Errorf("Expected description to contain conversion information, got: %s", desc)
	}

	// Test input schema exists
	testutils.AssertNotNil(t, definition.InputSchema)
}

func TestM2ETool_Execute_InlineMode_ValidInput(t *testing.T) {
	// Enable the murican_to_english tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "murican_to_english")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &m2e.M2ETool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"text": "The color of the organization's behavior was analyzed.",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
	testutils.AssertNotNil(t, result.Content)

	// Check that the result contains converted text
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	content := result.Content[0]
	textContent, ok := mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent, got different type")
	}

	if textContent.Type != "text" {
		t.Errorf("Expected content type 'text', got: %s", textContent.Type)
	}

	// Check that conversions were made (color->colour, organization->organisation, behavior->behaviour)
	if !testutils.Contains(textContent.Text, "colour") {
		t.Error("Expected result to contain British spelling 'colour'")
	}
}

func TestM2ETool_Execute_InlineMode_EmptyText(t *testing.T) {
	// Enable the murican_to_english tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "murican_to_english")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &m2e.M2ETool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"text": "   ", // Whitespace only text
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "text parameter cannot be empty")
}

func TestM2ETool_Execute_InlineMode_TrulyEmptyText(t *testing.T) {
	// Enable the murican_to_english tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "murican_to_english")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &m2e.M2ETool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"text": "", // Empty string
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "either 'text' or 'file_path' parameter must be provided")
}

func TestM2ETool_Execute_InlineMode_ExcessivelyLongText(t *testing.T) {
	// Enable the murican_to_english tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "murican_to_english")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &m2e.M2ETool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create text that exceeds the default 40000 character limit
	baseText := "This is American text with color and organization that will be repeated many times to exceed the character limit. "
	repetitions := 400 // 110 chars * 400 = 44000 chars (exceeds 40000)
	var excessivelyLongText string
	for range repetitions {
		excessivelyLongText += baseText
	}

	args := map[string]any{
		"text": excessivelyLongText,
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "text exceeds maximum length of 40000 characters")
}

func TestM2ETool_Execute_CustomMaxLengthEnvironmentVariable(t *testing.T) {
	// Enable the murican_to_english tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "murican_to_english")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	// Save original environment variable
	originalValue := os.Getenv("M2E_MAX_LENGTH")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("M2E_MAX_LENGTH")
		} else {
			_ = os.Setenv("M2E_MAX_LENGTH", originalValue)
		}
	}()

	// Set custom max length to 100 characters
	err := os.Setenv("M2E_MAX_LENGTH", "100")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &m2e.M2ETool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test with text that exceeds the custom limit (over 100 characters)
	longText := "This is American text with color and organization that is definitely longer than one hundred characters and should trigger the validation error when testing custom limits set via environment variables."

	args := map[string]any{
		"text": longText,
	}

	_, execErr := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, execErr)
	testutils.AssertErrorContains(t, execErr, "text exceeds maximum length of 100 characters")

	// Test with text within the custom limit (should succeed)
	shortText := "This American text has color."

	args = map[string]any{
		"text": shortText,
	}

	result, execErr := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, execErr)
	testutils.AssertNotNil(t, result)
}

func TestM2ETool_Execute_FileMode_ValidFile(t *testing.T) {
	// Enable the murican_to_english tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "murican_to_english")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	// Create a temporary file with American text
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.txt")

	americanText := "The color of the organization's behavior was analyzed with great care."
	err := os.WriteFile(tempFile, []byte(americanText), 0600)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	tool := &m2e.M2ETool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"file_path": tempFile,
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Check that the file was updated
	updatedContent, err := os.ReadFile(tempFile)
	testutils.AssertNoError(t, err)

	updatedText := string(updatedContent)
	// Should contain British spellings
	if !testutils.Contains(updatedText, "colour") {
		t.Error("Expected updated file to contain British spelling 'colour'")
	}
	if !testutils.Contains(updatedText, "organisation") {
		t.Error("Expected updated file to contain British spelling 'organisation'")
	}
	if !testutils.Contains(updatedText, "behaviour") {
		t.Error("Expected updated file to contain British spelling 'behaviour'")
	}
}

func TestM2ETool_Execute_FileMode_ExcessivelyLargeFile(t *testing.T) {
	// Enable the murican_to_english tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "murican_to_english")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	// Create a temporary file with content exceeding the limit
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "large_test.txt")

	// Create content that exceeds the default 40000 character limit
	baseText := "This is American text with color and organization that will be repeated many times to exceed the character limit. "
	repetitions := 400 // 110 chars * 400 = 44000 chars (exceeds 40000)
	var largeText string
	for range repetitions {
		largeText += baseText
	}

	err := os.WriteFile(tempFile, []byte(largeText), 0600)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	tool := &m2e.M2ETool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"file_path": tempFile,
	}

	_, err = tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "file content exceeds maximum length of 40000 characters")
}

func TestM2ETool_Execute_MissingParameters(t *testing.T) {
	// Enable the murican_to_english tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "murican_to_english")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &m2e.M2ETool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "either 'text' or 'file_path' parameter must be provided")
}

func TestM2ETool_Execute_BothParameters(t *testing.T) {
	// Enable the murican_to_english tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "murican_to_english")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &m2e.M2ETool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"text":      "Some text",
		"file_path": "/some/path",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "cannot provide both 'text' and 'file_path' parameters")
}

func TestM2ETool_Execute_InvalidFilePath(t *testing.T) {
	// Enable the murican_to_english tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "murican_to_english")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &m2e.M2ETool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"file_path": "relative/path", // Not absolute
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "file_path must be a fully qualified absolute path")
}

func TestM2ETool_Execute_SmartQuotesOption(t *testing.T) {
	// Enable the murican_to_english tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "murican_to_english")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &m2e.M2ETool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"text":              "The color is beautiful.",
		"keep_smart_quotes": true,
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Check that the result contains some response
	content := result.Content[0]
	textContent, ok := mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent, got different type")
	}

	// Just verify that we get a successful response
	if textContent.Text == "" {
		t.Error("Expected non-empty response text")
	}
}

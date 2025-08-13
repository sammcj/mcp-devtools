package tools_test

import (
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/think"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestThinkTool_Definition(t *testing.T) {
	tool := &think.ThinkTool{}
	definition := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "think", definition.Name)
	testutils.AssertNotNil(t, definition.Description)

	// Test that description contains key phrases
	desc := definition.Description
	if !testutils.Contains(desc, "think about something") {
		t.Errorf("Expected description to contain 'think about something', got: %s", desc)
	}

	// Test input schema exists
	testutils.AssertNotNil(t, definition.InputSchema)
}

func TestThinkTool_Execute_ValidInput(t *testing.T) {
	tool := &think.ThinkTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"thought": "This is a test thought",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
	testutils.AssertNotNil(t, result.Content)

	// Check that the thought is returned in the result
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	// The content should be text type and contain our thought with default "hard" prefix
	content := result.Content[0]
	textContent, ok := mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent, got different type")
	}

	if textContent.Type != "text" {
		t.Errorf("Expected content type 'text', got: %s", textContent.Type)
	}

	expectedText := "I should use the think hard tool on this problem: This is a test thought"
	if textContent.Text != expectedText {
		t.Errorf("Expected result to be '%s', got: %s", expectedText, textContent.Text)
	}
}

func TestThinkTool_Execute_EmptyThought(t *testing.T) {
	tool := &think.ThinkTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"thought": "",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: thought")
}

func TestThinkTool_Execute_MissingThought(t *testing.T) {
	tool := &think.ThinkTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: thought")
}

func TestThinkTool_Execute_InvalidThoughtType(t *testing.T) {
	tool := &think.ThinkTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"thought": 123, // Invalid type
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: thought")
}

func TestThinkTool_Execute_LongThought(t *testing.T) {
	tool := &think.ThinkTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test with a thought that's within the default limit (should succeed)
	normalLongThought := "This is a very long thought that goes on and on about complex reasoning and analysis. " +
		"It includes multiple sentences and demonstrates that the think tool can handle substantial " +
		"amounts of text for complex reasoning tasks. This is important for AI agents that need to " +
		"work through complicated problems step by step."

	args := map[string]interface{}{
		"thought": normalLongThought,
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Verify the full thought is preserved
	content := result.Content[0]
	textContent, ok := mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent, got different type")
	}

	if !testutils.Contains(textContent.Text, normalLongThought) {
		t.Error("Expected result to contain full long thought")
	}
}

func TestThinkTool_Execute_ExcessivelyLongThought(t *testing.T) {
	tool := &think.ThinkTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Create a thought that exceeds the default 2000 character limit
	baseText := "This is a very long thought that will be repeated many times to exceed the character limit. "
	repetitions := 25 // 90 chars * 25 = ~2250 chars (exceeds 2000)
	var excessivelyLongThought string
	for i := 0; i < repetitions; i++ {
		excessivelyLongThought += baseText
	}

	args := map[string]interface{}{
		"thought": excessivelyLongThought,
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "thought exceeds maximum length of 2000 characters")
}

func TestThinkTool_Execute_CustomMaxLengthEnvironmentVariable(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("THINK_MAX_LENGTH")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("THINK_MAX_LENGTH")
		} else {
			_ = os.Setenv("THINK_MAX_LENGTH", originalValue)
		}
	}()

	// Set custom max length to 100 characters
	err := os.Setenv("THINK_MAX_LENGTH", "100")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &think.ThinkTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test with a thought that exceeds the custom limit (over 100 characters)
	longThought := "This is a thought that is definitely longer than one hundred characters and should trigger the validation error when testing custom limits set via environment variables."

	args := map[string]interface{}{
		"thought": longThought,
	}

	_, execErr := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, execErr)
	testutils.AssertErrorContains(t, execErr, "thought exceeds maximum length of 100 characters")

	// Test with a thought within the custom limit (should succeed)
	shortThought := "This is within the custom limit."

	args = map[string]interface{}{
		"thought": shortThought,
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestThinkTool_Execute_HowHardParameter(t *testing.T) {
	tool := &think.ThinkTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test with "harder" parameter
	args := map[string]interface{}{
		"thought":  "This is a complex problem",
		"how_hard": "harder",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	content := result.Content[0]
	textContent, ok := mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent, got different type")
	}

	expectedText := "I should use the think harder tool on this problem: This is a complex problem"
	if textContent.Text != expectedText {
		t.Errorf("Expected result to be '%s', got: %s", expectedText, textContent.Text)
	}

	// Test with "ultra" parameter
	args = map[string]interface{}{
		"thought":  "This is an extremely complex problem",
		"how_hard": "ultra",
	}

	result, err = tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	content = result.Content[0]
	textContent, ok = mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent, got different type")
	}

	expectedText = "I should use the ultrathink tool on this problem: This is an extremely complex problem"
	if textContent.Text != expectedText {
		t.Errorf("Expected result to be '%s', got: %s", expectedText, textContent.Text)
	}

	// Test with explicit "hard" parameter
	args = map[string]interface{}{
		"thought":  "This is a standard problem",
		"how_hard": "hard",
	}

	result, err = tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	content = result.Content[0]
	textContent, ok = mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent, got different type")
	}

	expectedText = "I should use the think hard tool on this problem: This is a standard problem"
	if textContent.Text != expectedText {
		t.Errorf("Expected result to be '%s', got: %s", expectedText, textContent.Text)
	}
}

func TestThinkTool_Execute_InvalidHowHardParameter(t *testing.T) {
	tool := &think.ThinkTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test with invalid string value
	args := map[string]interface{}{
		"thought":  "This is a test thought",
		"how_hard": "invalid",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "invalid how_hard parameter: must be 'hard', 'harder', or 'ultra', got 'invalid'")

	// Test with invalid type
	args = map[string]interface{}{
		"thought":  "This is a test thought",
		"how_hard": 123,
	}

	_, err = tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "invalid how_hard parameter: must be a string")
}

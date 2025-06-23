package tools_test

import (
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
	if !contains(desc, "think about something") {
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

	// The content should be text type and contain our thought
	content := result.Content[0]
	textContent, ok := mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent, got different type")
	}

	if textContent.Type != "text" {
		t.Errorf("Expected content type 'text', got: %s", textContent.Type)
	}

	if !contains(textContent.Text, "This is a test thought") {
		t.Errorf("Expected result to contain thought text, got: %s", textContent.Text)
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

	// Test with a very long thought
	longThought := "This is a very long thought that goes on and on about complex reasoning and analysis. " +
		"It includes multiple sentences and demonstrates that the think tool can handle substantial " +
		"amounts of text for complex reasoning tasks. This is important for AI agents that need to " +
		"work through complicated problems step by step."

	args := map[string]interface{}{
		"thought": longThought,
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

	if !contains(textContent.Text, longThought) {
		t.Error("Expected result to contain full long thought")
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		containsMiddle(s, substr))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

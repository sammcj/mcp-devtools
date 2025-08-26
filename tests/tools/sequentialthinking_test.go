package tools_test

import (
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/sequentialthinking"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestSequentialThinkingTool_Definition(t *testing.T) {
	tool := &sequentialthinking.SequentialThinkingTool{}
	definition := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "sequential_thinking", definition.Name)
	testutils.AssertNotNil(t, definition.Description)

	// Test that description contains key phrases
	desc := definition.Description
	if !testutils.Contains(desc, "sequential thinking") {
		t.Errorf("Expected description to contain 'sequential thinking', got: %s", desc)
	}
	if !testutils.Contains(desc, "problem-solving") {
		t.Errorf("Expected description to contain 'problem-solving', got: %s", desc)
	}

	// Test input schema exists and has required fields
	testutils.AssertNotNil(t, definition.InputSchema)

	// Test that required parameters are present in schema
	schema := definition.InputSchema
	if schema.Type != "object" {
		t.Errorf("Expected schema type 'object', got: %s", schema.Type)
	}

	if schema.Properties == nil {
		t.Error("Expected schema properties to be defined")
	} else {
		// Test new simplified parameters
		requiredParams := []string{"action", "thought", "continue"}
		for _, param := range requiredParams {
			if _, exists := schema.Properties[param]; !exists {
				t.Errorf("Expected parameter '%s' to exist in schema", param)
			}
		}

		// Test optional parameters
		optionalParams := []string{"revise", "explore"}
		for _, param := range optionalParams {
			if _, exists := schema.Properties[param]; !exists {
				t.Errorf("Expected optional parameter '%s' to exist in schema", param)
			}
		}
	}
}

func TestSequentialThinkingTool_Execute_ToolDisabled(t *testing.T) {
	// Ensure tool is disabled
	_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action":   "think",
		"thought":  "Test thought",
		"continue": false,
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	if result != nil {
		t.Error("Expected result to be nil when tool is disabled")
	}
	if !testutils.Contains(err.Error(), "not enabled") {
		t.Errorf("Expected error about tool not being enabled, got: %s", err.Error())
	}
}

func TestSequentialThinkingTool_Execute_ValidInput(t *testing.T) {
	// Enable tool for this test
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sequential-thinking")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action":   "think",
		"thought":  "This is my first thought about the problem",
		"continue": true,
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
	testutils.AssertNotNil(t, result.Content)

	// Check that the result contains expected fields
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	content := result.Content[0]
	textContent, ok := mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent, got different type")
	}

	textResult := textContent.Text
	if !testutils.Contains(textResult, "thoughtNumber") {
		t.Error("Expected result to contain thoughtNumber")
	}
	if !testutils.Contains(textResult, "totalThoughts") {
		t.Error("Expected result to contain totalThoughts")
	}
	if !testutils.Contains(textResult, "continue") {
		t.Error("Expected result to contain continue")
	}
}

func TestSequentialThinkingTool_Execute_WithRevision(t *testing.T) {
	// Enable tool for this test
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sequential-thinking")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action":   "think",
		"thought":  "I need to revise my previous thought",
		"continue": true,
		"revise":   "previous thought",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestSequentialThinkingTool_Execute_WithBranching(t *testing.T) {
	// Enable tool for this test
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sequential-thinking")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action":   "think",
		"thought":  "Let me explore an alternative approach",
		"continue": true,
		"explore":  "alternative-approach",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Check that the result mentions branches
	if len(result.Content) > 0 {
		content := result.Content[0]
		textContent, ok := mcp.AsTextContent(content)
		if ok {
			textResult := textContent.Text
			if !testutils.Contains(textResult, "branches") {
				t.Error("Expected result to contain branches information")
			}
		}
	}
}

func TestSequentialThinkingTool_Execute_MissingRequiredField(t *testing.T) {
	// Enable tool for this test
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sequential-thinking")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Missing required field "thought"
	args := map[string]interface{}{
		"action":   "think",
		"continue": true,
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	if result != nil {
		t.Error("Expected result to be nil when required field is missing")
	}
	if !testutils.Contains(err.Error(), "thought is required") {
		t.Errorf("Expected error about missing thought field, got: %s", err.Error())
	}
}

func TestSequentialThinkingTool_Execute_MissingContinueField(t *testing.T) {
	// Enable tool for this test
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sequential-thinking")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Missing required field "continue"
	args := map[string]interface{}{
		"action":  "think",
		"thought": "Test thought",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	if result != nil {
		t.Error("Expected result to be nil when required field is missing")
	}
	if !testutils.Contains(err.Error(), "continue is required") {
		t.Errorf("Expected error about missing continue field, got: %s", err.Error())
	}
}

func TestSequentialThinkingTool_Execute_AutoNumbering(t *testing.T) {
	// Enable tool for this test
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sequential-thinking")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// First thought - should be numbered 1
	args := map[string]interface{}{
		"action":   "think",
		"thought":  "First thought",
		"continue": true,
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Check that the first thought is numbered 1
	if len(result.Content) > 0 {
		content := result.Content[0]
		textContent, ok := mcp.AsTextContent(content)
		if ok {
			textResult := textContent.Text
			if !testutils.Contains(textResult, `"thoughtNumber": 1`) {
				t.Error("Expected first thought to be numbered 1")
			}
		}
	}
}

func TestSequentialThinkingTool_Execute_DisableLogging(t *testing.T) {
	// Enable tool and disable logging
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sequential-thinking")
	_ = os.Setenv("DISABLE_THOUGHT_LOGGING", "true")
	defer func() {
		_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		_ = os.Unsetenv("DISABLE_THOUGHT_LOGGING")
	}()

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action":   "think",
		"thought":  "This thought should not be logged",
		"continue": false,
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
}

func TestSequentialThinkingTool_Execute_GetUsage(t *testing.T) {
	// Enable tool for this test
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sequential-thinking")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action": "get_usage",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
	testutils.AssertNotNil(t, result.Content)

	// Check that the result contains usage information
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	content := result.Content[0]
	textContent, ok := mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent, got different type")
	}

	usageText := textContent.Text
	if !testutils.Contains(usageText, "Sequential Thinking Tool") {
		t.Error("Expected usage to contain tool name")
	}
	if !testutils.Contains(usageText, "Parameters Explained") {
		t.Error("Expected usage to contain parameter explanations")
	}
	if !testutils.Contains(usageText, "Best Practices") {
		t.Error("Expected usage to contain best practices")
	}
}

func TestSequentialThinkingTool_Execute_DefaultAction(t *testing.T) {
	// Enable tool for this test
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sequential-thinking")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test that omitting action parameter defaults to "think"
	args := map[string]interface{}{
		"thought":  "Test thought",
		"continue": false,
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
	testutils.AssertNotNil(t, result.Content)

	// Should behave like normal thinking
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	content := result.Content[0]
	textContent, ok := mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent, got different type")
	}

	textResult := textContent.Text
	if !testutils.Contains(textResult, "thoughtNumber") {
		t.Error("Expected result to contain thoughtNumber")
	}
}

func TestSequentialThinkingTool_Execute_InvalidAction(t *testing.T) {
	// Enable tool for this test
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sequential-thinking")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action": "invalid_action",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	if result != nil {
		t.Error("Expected result to be nil when action is invalid")
	}
	if !testutils.Contains(err.Error(), "invalid action") {
		t.Errorf("Expected error about invalid action, got: %s", err.Error())
	}
}

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
		requiredParams := []string{"thought", "nextThoughtNeeded", "thoughtNumber", "totalThoughts"}
		for _, param := range requiredParams {
			if _, exists := schema.Properties[param]; !exists {
				t.Errorf("Expected parameter '%s' to exist in schema", param)
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
		"thought":           "Test thought",
		"nextThoughtNeeded": false,
		"thoughtNumber":     1.0,
		"totalThoughts":     1.0,
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
		"thought":           "This is my first thought about the problem",
		"nextThoughtNeeded": true,
		"thoughtNumber":     1.0,
		"totalThoughts":     3.0,
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
	if !testutils.Contains(textResult, "nextThoughtNeeded") {
		t.Error("Expected result to contain nextThoughtNeeded")
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
		"thought":           "I need to revise my previous thought",
		"nextThoughtNeeded": true,
		"thoughtNumber":     2.0,
		"totalThoughts":     3.0,
		"isRevision":        true,
		"revisesThought":    1.0,
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
		"thought":           "Let me explore an alternative approach",
		"nextThoughtNeeded": true,
		"thoughtNumber":     3.0,
		"totalThoughts":     5.0,
		"branchFromThought": 2.0,
		"branchId":          "alternative-approach",
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
		"nextThoughtNeeded": true,
		"thoughtNumber":     1.0,
		"totalThoughts":     1.0,
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

func TestSequentialThinkingTool_Execute_InvalidThoughtNumber(t *testing.T) {
	// Enable tool for this test
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sequential-thinking")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Invalid thought number (negative)
	args := map[string]interface{}{
		"thought":           "Test thought",
		"nextThoughtNeeded": false,
		"thoughtNumber":     -1.0,
		"totalThoughts":     1.0,
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	if result != nil {
		t.Error("Expected result to be nil when thought number is invalid")
	}
	if !testutils.Contains(err.Error(), "positive integer") {
		t.Errorf("Expected error about positive integer, got: %s", err.Error())
	}
}

func TestSequentialThinkingTool_Execute_ThoughtNumberExceedsTotalThoughts(t *testing.T) {
	// Enable tool for this test
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "sequential-thinking")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &sequentialthinking.SequentialThinkingTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Thought number exceeds total thoughts (should be adjusted automatically)
	args := map[string]interface{}{
		"thought":           "This thought exceeds the original estimate",
		"nextThoughtNeeded": false,
		"thoughtNumber":     5.0,
		"totalThoughts":     3.0,
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Should still work as the tool adjusts totalThoughts automatically
	if len(result.Content) > 0 {
		content := result.Content[0]
		textContent, ok := mcp.AsTextContent(content)
		if ok {
			textResult := textContent.Text
			if !testutils.Contains(textResult, "5") {
				t.Error("Expected result to show adjusted total thoughts")
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
		"thought":           "This thought should not be logged",
		"nextThoughtNeeded": false,
		"thoughtNumber":     1.0,
		"totalThoughts":     1.0,
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
		"thought":           "Test thought",
		"nextThoughtNeeded": false,
		"thoughtNumber":     1.0,
		"totalThoughts":     1.0,
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

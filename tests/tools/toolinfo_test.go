package tools_test

import (
	"context"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sammcj/mcp-devtools/internal/tools/think"
	"github.com/sammcj/mcp-devtools/internal/tools/utilities/toolhelp"
	"github.com/sammcj/mcp-devtools/tests/testutils"
	"github.com/sirupsen/logrus"
)

// mockExtendedInfoTool is a test tool that implements ExtendedHelpProvider
type mockExtendedInfoTool struct{}

func (m *mockExtendedInfoTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"mock_extended_tool",
		mcp.WithDescription("A mock tool that provides extended info"),
		mcp.WithString("param1",
			mcp.Required(),
			mcp.Description("A required parameter"),
		),
	)
}

func (m *mockExtendedInfoTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("mock result"), nil
}

func (m *mockExtendedInfoTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Basic usage example",
				Arguments: map[string]any{
					"param1": "test_value",
				},
				ExpectedResult: "Success with test_value",
			},
		},
		CommonPatterns: []string{
			"Use param1 with descriptive values",
			"Always check the result field",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Tool fails with empty param1",
				Solution: "Ensure param1 is a non-empty string",
			},
		},
		ParameterDetails: map[string]string{
			"param1": "This parameter accepts any non-empty string value",
		},
		WhenToUse:    "Use this tool when you need to test extended information",
		WhenNotToUse: "Don't use this tool in production environments",
	}
}

func TestToolHelpTool_Definition(t *testing.T) {
	// Set up registry with test logger
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Register a mock tool with extended help so the description is populated
	mockTool := &mockExtendedInfoTool{}
	registry.Register(mockTool)

	tool := &toolhelp.ToolHelpTool{}
	definition := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "get_tool_help", definition.Name)
	testutils.AssertNotNil(t, definition.Description)

	// Test that description contains key phrases (when tools with extended help exist)
	desc := definition.Description
	if !testutils.Contains(desc, "MCP DevTools") {
		t.Errorf("Expected description to reference MCP DevTools, got: %s", desc)
	}

	if !testutils.Contains(desc, "troubleshooting") {
		t.Errorf("Expected description to mention troubleshooting, got: %s", desc)
	}

	// Test input schema exists
	testutils.AssertNotNil(t, definition.InputSchema)
}

func TestToolHelpTool_Execute_ValidToolWithoutExtendedInfo(t *testing.T) {
	// Set up registry with test logger
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Register the think tool (which doesn't implement ExtendedHelpProvider)
	registry.Register(&think.ThinkTool{})

	tool := &toolhelp.ToolHelpTool{}
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"tool_name": "think",
	}

	// This should now fail because 'think' doesn't provide extended help
	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "does not provide extended help")

	// Result should be nil when there's an error
	if result != nil {
		t.Error("Expected nil result when tool doesn't provide extended help")
	}
}

func TestToolHelpTool_Execute_ValidToolWithExtendedInfo(t *testing.T) {
	// Enable the mock tool (mock tools are not in defaultTools list)
	defer testutils.WithEnv(t, "ENABLE_ADDITIONAL_TOOLS", "mock_extended_tool")()

	// Set up registry with test logger
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Register our mock tool with extended info
	mockTool := &mockExtendedInfoTool{}
	registry.Register(mockTool)

	tool := &toolhelp.ToolHelpTool{}
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"tool_name": "mock_extended_tool",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)
	testutils.AssertNotNil(t, result.Content)

	// Check that the result is JSON content
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	content := result.Content[0]
	jsonContent, ok := mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent with JSON, got different type")
	}

	resultText := jsonContent.Text

	// Verify extended info is present
	if !testutils.Contains(resultText, "\"has_extended_info\": true") {
		t.Error("Expected has_extended_info to be true for mock tool")
	}
	if !testutils.Contains(resultText, "Basic usage example") {
		t.Error("Expected result to contain example description")
	}
	if !testutils.Contains(resultText, "when_to_use") {
		t.Error("Expected result to contain when_to_use field")
	}
	if !testutils.Contains(resultText, "troubleshooting") {
		t.Error("Expected result to contain troubleshooting field")
	}
}

func TestToolHelpTool_Execute_InvalidToolName(t *testing.T) {
	// Set up registry with test logger
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Register a tool with extended help so we have a reference
	mockTool := &mockExtendedInfoTool{}
	registry.Register(mockTool)

	tool := &toolhelp.ToolHelpTool{}
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"tool_name": "nonexistent_tool",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "tool 'nonexistent_tool' not found")
	testutils.AssertErrorContains(t, err, "Tools with extended help:")
}

func TestToolHelpTool_Execute_MissingToolName(t *testing.T) {
	// Set up registry with test logger
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	tool := &toolhelp.ToolHelpTool{}
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: tool_name")
}

func TestToolHelpTool_Execute_InvalidToolNameType(t *testing.T) {
	// Set up registry with test logger
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	tool := &toolhelp.ToolHelpTool{}
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"tool_name": 123, // Invalid type
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: tool_name")
}

func TestToolHelpTool_Execute_AlwaysIncludeExamples(t *testing.T) {
	// Enable the mock tool (mock tools are not in defaultTools list)
	defer testutils.WithEnv(t, "ENABLE_ADDITIONAL_TOOLS", "mock_extended_tool")()

	// Set up registry with test logger
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Register our mock tool with extended info
	mockTool := &mockExtendedInfoTool{}
	registry.Register(mockTool)

	tool := &toolhelp.ToolHelpTool{}
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"tool_name": "mock_extended_tool",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	content := result.Content[0]
	jsonContent, ok := mcp.AsTextContent(content)
	if !ok {
		t.Fatal("Expected TextContent with JSON, got different type")
	}

	resultText := jsonContent.Text

	// Verify examples are always included when available
	if !testutils.Contains(resultText, "Basic usage example") {
		t.Error("Expected examples to always be included when available")
	}
}

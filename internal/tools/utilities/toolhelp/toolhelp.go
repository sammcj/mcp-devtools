package toolhelp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// ToolHelpTool implements a tool that provides extended information about MCP DevTools server tools
type ToolHelpTool struct{}

// init registers the tool with the registry
func init() {
	registry.Register(&ToolHelpTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *ToolHelpTool) Definition() mcp.Tool {
	// Get only tools that provide extended help
	toolsWithExtendedHelp := registry.GetToolNamesWithExtendedHelp()
	toolList := strings.Join(toolsWithExtendedHelp, ", ")

	var description string
	if len(toolsWithExtendedHelp) > 0 {
		description = fmt.Sprintf(
			"Get detailed usage information, examples, and troubleshooting for tools provided by MCP DevTools. "+
				"Provides comprehensive guidance including parameter constraints, common patterns, and error solutions. "+
				"Use when you are encountering unexpected errors with these specific tools:"+
				"Extended help available for: [ %s ]. ",
			toolList,
		)
	} else {
		description = "No tools currently provide extended help information."
	}

	// If no tools provide extended help, still create the tool but with empty enum
	enumValues := toolsWithExtendedHelp
	if len(enumValues) == 0 {
		enumValues = []string{} // Empty enum will prevent the tool from being used
	}

	return mcp.NewTool(
		"devtools_help",
		mcp.WithDescription(description),
		mcp.WithString("tool_name",
			mcp.Required(),
			mcp.Description(fmt.Sprintf("Name of the DevTools tool that provides extended help. MUST be one of: %s", toolList)),
			mcp.Enum(enumValues...),
		),
	)
}

// Execute executes the devtools_help tool
func (t *ToolHelpTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse and validate parameters
	toolName, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Get the tool from registry
	tool, exists := registry.GetTool(toolName)
	if !exists {
		availableTools := registry.GetToolNamesWithExtendedHelp()
		return nil, fmt.Errorf("tool '%s' not found, disabled, or does not provide extended help. Tools with extended help: %s", toolName, strings.Join(availableTools, ", "))
	}

	// Check if tool implements ExtendedHelpProvider (this should always be true now due to filtering)
	extendedProvider, ok := tool.(tools.ExtendedHelpProvider)
	if !ok {
		availableTools := registry.GetToolNamesWithExtendedHelp()
		return nil, fmt.Errorf("tool '%s' does not provide extended help. Tools with extended help: %s", toolName, strings.Join(availableTools, ", "))
	}

	// Build response
	response := &ToolHelpResponse{
		ToolName:        toolName,
		BasicInfo:       t.extractBasicInfo(tool),
		HasExtendedInfo: true,
	}

	// Get extended information
	extendedInfo := extendedProvider.ProvideExtendedInfo()
	if extendedInfo != nil {
		response.ExtendedInfo = t.convertExtendedInfo(extendedInfo)
	} else {
		// This shouldn't happen if the tool properly implements the interface
		response.HasExtendedInfo = false
		response.Message = fmt.Sprintf("Tool '%s' implements ExtendedHelpProvider but returned no extended information", toolName)
	}

	return t.newToolResult(response)
}

// parseRequest parses and validates the tool arguments
func (t *ToolHelpTool) parseRequest(args map[string]interface{}) (string, error) {
	// Parse tool_name (required)
	toolName, ok := args["tool_name"].(string)
	if !ok || toolName == "" {
		return "", fmt.Errorf("missing or invalid required parameter: tool_name")
	}

	return toolName, nil
}

// extractBasicInfo extracts basic information from a tool's definition
func (t *ToolHelpTool) extractBasicInfo(tool tools.Tool) map[string]interface{} {
	definition := tool.Definition()

	basicInfo := map[string]interface{}{
		"name":        definition.Name,
		"description": definition.Description,
	}

	// Add input schema if available
	if definition.InputSchema.Type != "" {
		basicInfo["input_schema"] = definition.InputSchema
	}

	return basicInfo
}

// convertExtendedInfo converts tools.ExtendedInfo to our response format
func (t *ToolHelpTool) convertExtendedInfo(info *tools.ExtendedHelp) *ExtendedHelpData {
	result := &ExtendedHelpData{
		CommonPatterns:   info.CommonPatterns,
		ParameterDetails: info.ParameterDetails,
		WhenToUse:        info.WhenToUse,
		WhenNotToUse:     info.WhenNotToUse,
	}

	// Convert troubleshooting tips
	if len(info.Troubleshooting) > 0 {
		result.Troubleshooting = make([]TroubleshootingData, len(info.Troubleshooting))
		for i, tip := range info.Troubleshooting {
			result.Troubleshooting[i] = TroubleshootingData{
				Problem:  tip.Problem,
				Solution: tip.Solution,
			}
		}
	}

	// Convert examples (always include if available)
	if len(info.Examples) > 0 {
		result.Examples = make([]ToolExampleData, len(info.Examples))
		for i, example := range info.Examples {
			result.Examples[i] = ToolExampleData{
				Description:    example.Description,
				Arguments:      example.Arguments,
				ExpectedResult: example.ExpectedResult,
			}
		}
	}

	return result
}

// newToolResult creates a new tool result from the response
func (t *ToolHelpTool) newToolResult(response *ToolHelpResponse) (*mcp.CallToolResult, error) {
	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(responseJSON)), nil
}

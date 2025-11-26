package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

// DynamicProxyTool represents a single upstream tool exposed as a first-class tool.
type DynamicProxyTool struct {
	toolName         string // The exposed tool name (possibly prefixed)
	originalToolName string // The original upstream tool name
	upstreamName     string
	description      string
	inputSchema      any
	manager          *ProxyManager // Shared manager instance
}

// Definition returns the tool's definition for MCP registration.
func (d *DynamicProxyTool) Definition() mcp.Tool {
	// Create base tool with name and description
	opts := []mcp.ToolOption{
		mcp.WithDescription(d.description),
	}

	// Add input schema if available
	if d.inputSchema != nil {
		// The input schema from upstream should be a JSON schema object
		// We need to convert it to mcp.ToolOption format
		// For now, we'll accept any arguments as a passthrough
		opts = append(opts, mcp.WithObject("arguments",
			mcp.Description("Arguments for the upstream tool (passed through as-is)"),
		))
	}

	return mcp.NewTool(d.toolName, opts...)
}

// Execute executes the upstream tool.
func (d *DynamicProxyTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Ensure manager is initialised
	if err := d.manager.EnsureInitialised(ctx, logger); err != nil {
		return nil, fmt.Errorf("failed to initialise proxy manager: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"tool_name":          d.toolName,
		"upstream":           d.upstreamName,
		"original_tool_name": d.originalToolName,
	}).Debug("executing upstream tool")

	// Extract arguments - they might be nested in an "arguments" field
	toolArgs := args
	if nestedArgs, ok := args["arguments"].(map[string]any); ok {
		toolArgs = nestedArgs
	}

	// Execute tool via manager
	response, err := d.manager.GetManager().ExecuteTool(ctx, d.originalToolName, toolArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tool on upstream: %w", err)
	}

	// Check for error in response
	if response.Error != nil {
		return nil, fmt.Errorf("upstream error: %s (code %d)", response.Error.Message, response.Error.Code)
	}

	// Parse and return result
	var result map[string]any
	if err := json.Unmarshal(response.Result, &result); err != nil {
		// If unmarshalling fails, return raw result
		result = map[string]any{
			"raw_result": string(response.Result),
		}
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

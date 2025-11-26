package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// ProxyTool implements the tools.Tool interface for proxying to upstream MCP servers.
// This tool uses lazy initialisation to connect to upstreams on first use.
type ProxyTool struct {
	manager      *ProxyManager
	initialised  bool
	initialiseMu sync.Mutex
}

// init registers the proxy tool.
// Connections to upstreams are established lazily on first use to avoid blocking server startup.
func init() {
	registry.Register(&ProxyTool{})
}

// Definition returns the tool's definition for MCP registration.
func (t *ProxyTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"proxy",
		mcp.WithDescription("Proxy tools from upstream MCP servers with OAuth support"),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform: 'list' (list upstream tools), 'call' (call upstream tool)"),
			mcp.Enum("list", "call"),
		),
		mcp.WithString("tool_name",
			mcp.Description("Name of the tool to call (required for 'call' action)"),
		),
		mcp.WithObject("arguments",
			mcp.Description("Arguments to pass to the tool (required for 'call' action)"),
		),
	)
}

// Execute executes the tool's logic.
func (t *ProxyTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Lazy initialisation using shared manager
	if err := t.ensureInitialised(ctx, logger); err != nil {
		return nil, fmt.Errorf("failed to initialise proxy tool: %w", err)
	}

	// Parse action
	action, ok := args["action"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'action' parameter")
	}

	switch action {
	case "list":
		return t.handleList(logger)
	case "call":
		return t.handleCall(ctx, logger, args)
	default:
		return nil, fmt.Errorf("invalid action: %s (must be 'list' or 'call')", action)
	}
}

// ensureInitialised ensures the proxy manager is initialised (lazy initialisation).
func (t *ProxyTool) ensureInitialised(ctx context.Context, logger *logrus.Logger) error {
	t.initialiseMu.Lock()
	defer t.initialiseMu.Unlock()

	if t.initialised {
		return nil
	}

	// Get or create the global manager
	if t.manager == nil {
		t.manager = GetGlobalProxyManager()
	}

	// Ensure manager is initialised
	if err := t.manager.EnsureInitialised(ctx, logger); err != nil {
		return err
	}

	t.initialised = true
	return nil
}

// handleList returns the list of all upstream tools.
func (t *ProxyTool) handleList(logger *logrus.Logger) (*mcp.CallToolResult, error) {
	logger.Debug("listing upstream tools")

	aggregator := t.manager.GetAggregator()
	tools := aggregator.GetTools()

	result := map[string]any{
		"count": len(tools),
		"tools": tools,
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// handleCall calls a tool on an upstream server.
func (t *ProxyTool) handleCall(ctx context.Context, logger *logrus.Logger, args map[string]any) (*mcp.CallToolResult, error) {
	// Parse tool name
	toolName, ok := args["tool_name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'tool_name' parameter")
	}

	// Parse arguments
	toolArgs, ok := args["arguments"].(map[string]any)
	if !ok {
		toolArgs = make(map[string]any)
	}

	logger.WithField("tool_name", toolName).Debug("calling upstream tool")

	aggregator := t.manager.GetAggregator()
	upstreamMgr := t.manager.GetManager()

	// Get upstream for this tool
	upstreamName, err := aggregator.GetUpstreamForTool(toolName)
	if err != nil {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	// Get original tool name (without prefix)
	originalToolName := aggregator.GetOriginalToolName(toolName)

	logger.WithFields(logrus.Fields{
		"tool_name":          toolName,
		"upstream":           upstreamName,
		"original_tool_name": originalToolName,
	}).Debug("routing tool call to upstream")

	// Execute tool on upstream
	response, err := upstreamMgr.ExecuteTool(ctx, originalToolName, toolArgs)
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

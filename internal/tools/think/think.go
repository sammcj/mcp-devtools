package think

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// ThinkTool implements a simple thinking tool for structured reasoning
type ThinkTool struct{}

// init registers the think tool
func init() {
	registry.Register(&ThinkTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *ThinkTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"think",
		mcp.WithDescription(`Use the tool to think about something. It will not obtain new information or change the database, but just append the thought to the log. Use it when complex reasoning or some cache memory is needed.

This tool is particularly valuable for:
- Analysing tool outputs before taking action
- Breaking down complex multi-step problems
- Reasoning through policy decisions or constraints
- Planning sequential actions where mistakes are costly
- Processing and reflecting on information gathered from previous tool calls

Use this tool as a structured thinking space during complex workflows, especially when you need to pause and reason about what you've learned before proceeding.`),
		mcp.WithString("thought",
			mcp.Required(),
			mcp.Description("A thought to think about."),
		),
	)
}

// Execute executes the think tool
func (t *ThinkTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse and validate parameters
	request, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Create response with timestamp
	response := &ThinkResponse{
		Thought:   request.Thought,
		Timestamp: time.Now(),
	}

	return t.newToolResultText(response.Thought)
}

// parseRequest parses and validates the tool arguments
func (t *ThinkTool) parseRequest(args map[string]interface{}) (*ThinkRequest, error) {
	// Parse thought (required)
	thought, ok := args["thought"].(string)
	if !ok || thought == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: thought")
	}

	return &ThinkRequest{
		Thought: thought,
	}, nil
}

// newToolResultText creates a new tool result with text content
func (t *ThinkTool) newToolResultText(thought string) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText(thought), nil
}

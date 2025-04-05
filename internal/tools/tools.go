package tools

import (
	"context"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

// Tool is the interface that all MCP tool implementations must satisfy
type Tool interface {
	// Definition returns the tool's definition for MCP registration
	Definition() mcp.Tool

	// Execute executes the tool's logic using shared resources (logger, cache) and parsed arguments
	Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error)
}

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

// ExtendedHelpProvider is an optional interface that tools can implement to provide
// detailed usage information, examples, and troubleshooting help
type ExtendedHelpProvider interface {
	ProvideExtendedInfo() *ExtendedHelp
}

// ExtendedHelp contains detailed information about a tool's usage
type ExtendedHelp struct {
	Examples         []ToolExample        `json:"examples,omitempty"`
	CommonPatterns   []string             `json:"common_patterns,omitempty"`
	Troubleshooting  []TroubleshootingTip `json:"troubleshooting,omitempty"`
	ParameterDetails map[string]string    `json:"parameter_details,omitempty"`
	WhenToUse        string               `json:"when_to_use,omitempty"`
	WhenNotToUse     string               `json:"when_not_to_use,omitempty"`
}

// ToolExample represents a usage example for a tool
type ToolExample struct {
	Description    string                 `json:"description"`
	Arguments      map[string]interface{} `json:"arguments"`
	ExpectedResult string                 `json:"expected_result,omitempty"`
}

// TroubleshootingTip represents a troubleshooting tip for a tool
type TroubleshootingTip struct {
	Problem  string `json:"problem"`
	Solution string `json:"solution"`
}

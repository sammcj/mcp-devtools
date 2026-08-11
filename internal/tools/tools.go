package tools

import (
	"context"
	"sync"

	"github.com/sammcj/mcp-devtools/internal/mcpapi"
	"github.com/sirupsen/logrus"
)

// Tool is the interface that all MCP tool implementations must satisfy
type Tool interface {
	// Definition returns the tool's definition for MCP registration
	Definition() mcpapi.Tool

	// Execute executes the tool's logic using shared resources (logger, cache) and parsed arguments
	Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcpapi.CallToolResult, error)
}

// StdioOnly marks a tool that keeps state in the server process between calls
// and so only works on the stdio transport, where one client owns one process.
// MCP 2026-07-28 is stateless, and such a tool gives wrong answers as soon as a
// load balancer puts consecutive calls on different instances. Tools marked
// this way are not registered on any other transport.
type StdioOnly interface {
	StdioOnly()
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
	Description    string         `json:"description"`
	Arguments      map[string]any `json:"arguments"`
	ExpectedResult string         `json:"expected_result,omitempty"`
}

// TroubleshootingTip represents a troubleshooting tip for a tool
type TroubleshootingTip struct {
	Problem  string `json:"problem"`
	Solution string `json:"solution"`
}

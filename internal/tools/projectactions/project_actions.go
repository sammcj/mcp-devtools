package projectactions

import (
	"context"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// ProjectActionsTool provides security-aware execution of project development tasks
type ProjectActionsTool struct {
	workingDir           string
	makefileTargets      []string
	maxCommitMessageSize int
}

// init registers the tool with the registry
func init() {
	registry.Register(&ProjectActionsTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *ProjectActionsTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"project_actions",
		mcp.WithDescription("Execute project development tasks (tests, linters, formatters) and git operations through a project's Makefile. Provides security-aware access to make targets and git add/commit operations."),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
}

// Execute handles tool invocation
func (t *ProjectActionsTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	logger.Info("Executing project_actions")
	return mcp.NewToolResultText("Not yet implemented"), nil
}

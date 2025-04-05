package bedrock

import (
	"context"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// LatestBedrockModelTool handles getting the latest Claude Sonnet model
type LatestBedrockModelTool struct {
	client BedrockTool
}

// init registers the latest bedrock model tool with the registry
func init() {
	registry.Register(&LatestBedrockModelTool{
		client: BedrockTool{},
	})
}

// Definition returns the tool's definition for MCP registration
func (t *LatestBedrockModelTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"get_latest_bedrock_model",
		mcp.WithDescription("Get the latest Claude Sonnet model from Amazon Bedrock (best for coding tasks)"),
	)
}

// Execute executes the tool's logic
func (t *LatestBedrockModelTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Getting latest Claude Sonnet model")
	return t.client.getLatestClaudeSonnet()
}

package anthropic

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

// AnthropicTool handles Anthropic Claude model queries across all platforms
type AnthropicTool struct {
	parser *Parser
	cache  *sync.Map
	logger *logrus.Logger
	once   sync.Once
}

// NewAnthropicTool creates a new Anthropic tool
func NewAnthropicTool() *AnthropicTool {
	return &AnthropicTool{}
}

// Execute handles Anthropic model queries
func (t *AnthropicTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	logger.Info("Getting Anthropic model information")

	// Initialise parser thread-safely using sync.Once
	t.once.Do(func() {
		t.parser = NewParser(logger, cache)
		t.cache = cache
		t.logger = logger
	})

	// Parse action
	action := "list"
	if actionRaw, ok := args["action"].(string); ok && actionRaw != "" {
		action = actionRaw
	}

	// Handle different actions
	switch action {
	case "list":
		return t.listModels(ctx)
	case "search":
		return t.searchModels(ctx, args)
	case "get":
		return t.getModel(ctx, args)
	default:
		return nil, fmt.Errorf("invalid action: %s (valid actions: list, search, get)", action)
	}
}

// getModels retrieves all latest Anthropic models (internal helper)
func (t *AnthropicTool) getModels(ctx context.Context) ([]AnthropicModel, error) {
	models, err := t.parser.GetLatestModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Anthropic models: %w", err)
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("no Anthropic models available")
	}

	return models, nil
}

// listModels lists all available Anthropic models
func (t *AnthropicTool) listModels(ctx context.Context) (*mcp.CallToolResult, error) {
	models, err := t.getModels(ctx)
	if err != nil {
		return nil, err
	}

	// Sort models by family and version
	sort.Slice(models, func(i, j int) bool {
		if models[i].ModelFamily != models[j].ModelFamily {
			return models[i].ModelFamily < models[j].ModelFamily
		}
		return models[i].ModelVersion > models[j].ModelVersion // Newer versions first
	})

	result := AnthropicModelSearchResult{
		Models:     models,
		TotalCount: len(models),
	}

	return packageversions.NewToolResultJSON(result)
}

// searchModels searches for Anthropic models
func (t *AnthropicTool) searchModels(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	// Get all models
	models, err := t.getModels(ctx)
	if err != nil {
		return nil, err
	}

	// Parse query
	query := ""
	if queryRaw, ok := args["query"].(string); ok {
		query = strings.ToLower(strings.TrimSpace(queryRaw))
	}

	// If no query provided, return all models
	if query == "" {
		return t.listModels(ctx)
	}

	// Filter models
	var filteredModels []AnthropicModel
	for _, model := range models {
		// Check various fields for matches
		if matchesQuery(model, query) {
			filteredModels = append(filteredModels, model)
		}
	}

	// Sort results
	sort.Slice(filteredModels, func(i, j int) bool {
		if filteredModels[i].ModelFamily != filteredModels[j].ModelFamily {
			return filteredModels[i].ModelFamily < filteredModels[j].ModelFamily
		}
		return filteredModels[i].ModelVersion > filteredModels[j].ModelVersion
	})

	result := AnthropicModelSearchResult{
		Models:     filteredModels,
		TotalCount: len(filteredModels),
	}

	return packageversions.NewToolResultJSON(result)
}

// getModel gets a specific Anthropic model by ID
func (t *AnthropicTool) getModel(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	// Parse model ID (can be any of the ID formats)
	modelID, ok := args["modelId"].(string)
	if !ok || modelID == "" {
		// Try query parameter as fallback
		if queryID, ok := args["query"].(string); ok && queryID != "" {
			modelID = queryID
		} else {
			return nil, fmt.Errorf("missing required parameter: modelId or query")
		}
	}

	// Get all models
	models, err := t.getModels(ctx)
	if err != nil {
		return nil, err
	}

	// Search for model by any ID field
	modelID = strings.ToLower(strings.TrimSpace(modelID))
	for _, model := range models {
		if strings.ToLower(model.ClaudeAPIID) == modelID ||
			strings.ToLower(model.ClaudeAPIAlias) == modelID ||
			strings.ToLower(model.AWSBedrockID) == modelID ||
			strings.ToLower(model.GCPVertexAIID) == modelID ||
			strings.ToLower(model.ModelName) == modelID {
			return packageversions.NewToolResultJSON(model)
		}
	}

	return nil, fmt.Errorf("model not found: %s", modelID)
}

// matchesQuery checks if a model matches the search query
func matchesQuery(model AnthropicModel, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))

	// Check model name
	if strings.Contains(strings.ToLower(model.ModelName), query) {
		return true
	}

	// Check all ID fields
	if strings.Contains(strings.ToLower(model.ClaudeAPIID), query) ||
		strings.Contains(strings.ToLower(model.ClaudeAPIAlias), query) ||
		strings.Contains(strings.ToLower(model.AWSBedrockID), query) ||
		strings.Contains(strings.ToLower(model.GCPVertexAIID), query) {
		return true
	}

	// Check model family and version
	if strings.Contains(strings.ToLower(model.ModelFamily), query) ||
		strings.Contains(strings.ToLower(model.ModelVersion), query) {
		return true
	}

	// Enhanced matching for common aliases
	return matchesModelAlias(query, model)
}

// matchesModelAlias checks if query matches common model aliases
func matchesModelAlias(query string, model AnthropicModel) bool {
	// Normalise query - remove "claude-" or "claude " prefix if present
	normalisedQuery := query
	normalisedQuery = strings.TrimPrefix(normalisedQuery, "claude-")
	normalisedQuery = strings.TrimPrefix(normalisedQuery, "claude ")

	// Direct family match (e.g., "sonnet", "haiku", "opus")
	if normalisedQuery == model.ModelFamily {
		return true
	}

	// Family with version (e.g., "sonnet-4.5", "haiku-4")
	if strings.HasPrefix(normalisedQuery, model.ModelFamily+"-") ||
		strings.HasPrefix(normalisedQuery, model.ModelFamily+" ") {
		return true
	}

	// With claude- prefix (e.g., "claude-sonnet")
	if query == "claude-"+model.ModelFamily || query == "claude "+model.ModelFamily {
		return true
	}

	return false
}

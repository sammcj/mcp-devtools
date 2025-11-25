package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/anthropic"
	"github.com/sirupsen/logrus"
)

// BedrockTool handles AWS Bedrock model checking
type BedrockTool struct {
	anthropicTool *anthropic.AnthropicTool
	cache         *sync.Map
	logger        *logrus.Logger
	once          sync.Once
}

// Definition returns the tool's definition for MCP registration
func (t *BedrockTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"check_bedrock_models",
		mcp.WithDescription("Search, list, and get information about Amazon Bedrock models"),
		mcp.WithString("action",
			mcp.Description("Action to perform: list all models, search for models, or get a specific model"),
			mcp.Enum("list", "search", "get"),
			mcp.DefaultString("list"),
		),
		mcp.WithString("modelId",
			mcp.Description("Model ID to retrieve (used with action: \"get\")"),
		),
		mcp.WithString("provider",
			mcp.Description("Filter by provider name (used with action: \"search\")"),
		),
		mcp.WithString("query",
			mcp.Description("Search query for model name or ID (used with action: \"search\")"),
		),
		mcp.WithString("region",
			mcp.Description("Filter by AWS region (used with action: \"search\")"),
		),
	)
}

// Execute executes the tool's logic
func (t *BedrockTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	logger.Info("Getting AWS Bedrock model information")

	// Initialise Anthropic tool thread-safely using sync.Once
	t.once.Do(func() {
		t.anthropicTool = anthropic.NewAnthropicTool()
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
	case "get_latest_claude_sonnet":
		return t.getLatestClaudeSonnet(ctx)
	default:
		return nil, fmt.Errorf("invalid action: %s", action)
	}
}

// getModels gets all available AWS Bedrock models (internal helper)
func (t *BedrockTool) getModels(ctx context.Context) ([]packageversions.BedrockModel, error) {
	// Fetch latest Anthropic models using the Anthropic tool
	anthropicModels := []anthropic.AnthropicModel{}

	// Use Anthropic tool to get models
	anthropicArgs := map[string]any{"action": "list"}
	anthropicResult, err := t.anthropicTool.Execute(ctx, t.logger, t.cache, anthropicArgs)
	if err != nil {
		t.logger.WithError(err).Warn("Failed to fetch latest Anthropic models, using fallback data. Visit https://platform.claude.com/docs/en/about-claude/models/overview#latest-models-comparison for latest model IDs.")
	} else {
		// Extract models from the result
		anthropicModels = extractAnthropicModelsFromResult(anthropicResult)
	}

	// Start with statically defined non-Anthropic models
	models := []packageversions.BedrockModel{
		{
			Provider:           "amazon",
			ModelName:          "Titan Text G1 - Express",
			ModelID:            "amazon.titan-text-express-v1",
			RegionsSupported:   []string{"us-east-1", "us-west-2", "us-gov-west-1", "ap-northeast-1", "ap-south-1", "ap-southeast-2", "ca-central-1", "eu-central-1", "eu-west-1", "eu-west-2", "eu-west-3", "sa-east-1"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "amazon",
			ModelName:          "Titan Embeddings G1 - Text",
			ModelID:            "amazon.titan-embed-text-v1",
			RegionsSupported:   []string{"us-east-1", "us-west-2", "ap-northeast-1", "eu-central-1"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"embeddings"},
			StreamingSupported: false,
		},
		{
			Provider:           "amazon",
			ModelName:          "Titan Text Embeddings V2",
			ModelID:            "amazon.titan-embed-text-v2:0",
			RegionsSupported:   []string{"us-east-1", "us-east-2", "us-west-2", "us-gov-east-1", "us-gov-west-1", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3", "ap-south-1", "ap-south-2", "ap-southeast-2", "ca-central-1", "eu-central-1", "eu-central-2", "eu-north-1", "eu-south-1", "eu-south-2", "eu-west-1", "eu-west-2", "eu-west-3", "sa-east-1"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"embeddings"},
			StreamingSupported: false,
		},
		{
			Provider:           "amazon",
			ModelName:          "Rerank 1.0",
			ModelID:            "amazon.rerank-v1:0",
			RegionsSupported:   []string{"us-west-2", "ap-northeast-1", "ca-central-1", "eu-central-1"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"rankings"},
			StreamingSupported: false,
		},
		{
			Provider:           "amazon",
			ModelName:          "Nova Pro",
			ModelID:            "amazon.nova-pro-v1:0",
			RegionsSupported:   []string{"us-east-1", "us-east-2", "us-west-2", "us-gov-west-1", "ap-northeast-1", "ap-northeast-2", "ap-south-1", "ap-southeast-1", "ap-southeast-2", "eu-central-1", "eu-north-1", "eu-south-1", "eu-south-2", "eu-west-1", "eu-west-2", "eu-west-3"},
			InputModalities:    []string{"text", "image"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "amazon",
			ModelName:          "Titan Text G1 - Lite",
			ModelID:            "amazon.titan-text-lite-v1",
			RegionsSupported:   []string{"us-east-1", "us-west-2", "ap-south-1", "ap-southeast-2", "ca-central-1", "eu-central-1", "eu-west-1", "eu-west-2", "eu-west-3", "sa-east-1"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "amazon",
			ModelName:          "Titan Multimodal Embeddings G1",
			ModelID:            "amazon.titan-embed-image-v1",
			RegionsSupported:   []string{"us-east-1", "us-west-2", "ap-south-1", "ap-southeast-2", "ca-central-1", "eu-central-1", "eu-west-1", "eu-west-2", "eu-west-3", "sa-east-1"},
			InputModalities:    []string{"text", "image"},
			OutputModalities:   []string{"embeddings"},
			StreamingSupported: false,
		},
		{
			Provider:           "amazon",
			ModelName:          "Titan Image Generator G1 v2",
			ModelID:            "amazon.titan-image-generator-v2",
			RegionsSupported:   []string{"us-east-1", "us-west-2"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"image"},
			StreamingSupported: false,
		},
		{
			Provider:           "amazon",
			ModelName:          "Nova Premier",
			ModelID:            "amazon.nova-premier-v1:0",
			RegionsSupported:   []string{"us-east-1", "us-east-2", "us-west-2"},
			InputModalities:    []string{"text", "image"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "amazon",
			ModelName:          "Titan Text G1 - Premier",
			ModelID:            "amazon.titan-text-premier-v1:0",
			RegionsSupported:   []string{"us-east-1"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "cohere",
			ModelName:          "Embed English",
			ModelID:            "cohere.embed-english-v3",
			RegionsSupported:   []string{"us-east-1", "us-west-2", "ap-northeast-1", "ap-south-1", "ap-southeast-1", "ap-southeast-2", "ca-central-1", "eu-central-1", "eu-west-1", "eu-west-2", "eu-west-3", "sa-east-1"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"embeddings"},
			StreamingSupported: false,
		},
		{
			Provider:           "cohere",
			ModelName:          "Embed Multilingual",
			ModelID:            "cohere.embed-multilingual-v3",
			RegionsSupported:   []string{"us-east-1", "us-west-2", "ap-northeast-1", "ap-south-1", "ap-southeast-1", "ap-southeast-2", "ca-central-1", "eu-central-1", "eu-west-1", "eu-west-2", "eu-west-3", "sa-east-1"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"embeddings"},
			StreamingSupported: false,
		},
		{
			Provider:           "cohere",
			ModelName:          "Rerank 3.5",
			ModelID:            "cohere.rerank-v3-5:0",
			RegionsSupported:   []string{"us-west-2", "ap-northeast-1", "ca-central-1", "eu-central-1"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"rankings"},
			StreamingSupported: false,
		},
		{
			Provider:           "deepseek",
			ModelName:          "DeepSeek-R1",
			ModelID:            "deepseek.deepseek-r1-distill-qwen-32b-v1:0",
			RegionsSupported:   []string{"us-east-1", "us-east-2", "us-west-2"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "stability",
			ModelName:          "Stable Diffusion 3.5 Large",
			ModelID:            "stability.sd3-5-large-v1:0",
			RegionsSupported:   []string{"us-west-2"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"image"},
			StreamingSupported: false,
		},
		{
			Provider:           "stability",
			ModelName:          "Stable Image Core 1.0",
			ModelID:            "stability.stable-image-core-v1:0",
			RegionsSupported:   []string{"us-west-2"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"image"},
			StreamingSupported: false,
		},
		{
			Provider:           "stability",
			ModelName:          "Stable Image Ultra 1.0",
			ModelID:            "stability.stable-image-ultra-v1:0",
			RegionsSupported:   []string{"us-west-2"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"image"},
			StreamingSupported: false,
		},
	}

	// Add dynamically fetched Anthropic models
	for _, anthropicModel := range anthropicModels {
		bedrockModel := packageversions.BedrockModel{
			Provider:           "anthropic",
			ModelName:          anthropicModel.ModelName,
			ModelID:            anthropicModel.AWSBedrockID,
			RegionsSupported:   anthropicModel.RegionsSupported,
			InputModalities:    []string{"text", "image"}, // All Claude models support text and image
			OutputModalities:   []string{"text"},
			StreamingSupported: true, // All Claude models support streaming
		}

		// If no regions specified, use common regions
		if len(bedrockModel.RegionsSupported) == 0 {
			bedrockModel.RegionsSupported = []string{"us-east-1", "us-east-2", "us-west-2"}
		}

		models = append(models, bedrockModel)
	}

	// Sort models by provider and name
	sort.Slice(models, func(i, j int) bool {
		if models[i].Provider != models[j].Provider {
			return models[i].Provider < models[j].Provider
		}
		return models[i].ModelName < models[j].ModelName
	})

	// Defensive check - should never happen, but if we have no models, return error
	if len(models) == 0 {
		return nil, fmt.Errorf("no models available")
	}

	return models, nil
}

// listModels lists all available AWS Bedrock models
func (t *BedrockTool) listModels(ctx context.Context) (*mcp.CallToolResult, error) {
	models, err := t.getModels(ctx)
	if err != nil {
		return nil, err
	}

	result := packageversions.BedrockModelSearchResult{
		Models:     models,
		TotalCount: len(models),
	}

	return packageversions.NewToolResultJSON(result)
}

// searchModels searches for AWS Bedrock models
func (t *BedrockTool) searchModels(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	// Get all models directly
	models, err := t.getModels(ctx)
	if err != nil {
		return nil, err
	}

	// Parse query
	query := ""
	if queryRaw, ok := args["query"].(string); ok {
		query = strings.ToLower(queryRaw)
	}

	// Parse provider
	provider := ""
	if providerRaw, ok := args["provider"].(string); ok {
		provider = strings.ToLower(providerRaw)
	}

	// Parse region
	region := ""
	if regionRaw, ok := args["region"].(string); ok {
		region = strings.ToLower(regionRaw)
	}

	// Filter models
	var filteredModels []packageversions.BedrockModel
	for _, model := range models {
		// Apply filters
		if query != "" {
			// Standard search fields
			nameMatch := strings.Contains(strings.ToLower(model.ModelName), query)
			idMatch := strings.Contains(strings.ToLower(model.ModelID), query)
			providerMatch := strings.Contains(strings.ToLower(model.Provider), query)

			// Enhanced matching for common Claude model family aliases
			aliasMatch := false
			if model.Provider == "anthropic" {
				lowerModelName := strings.ToLower(model.ModelName)
				// Support queries like "sonnet", "claude-sonnet", "haiku", "opus"
				aliasMatch = matchesClaudeAlias(query, lowerModelName)
			}

			if !nameMatch && !idMatch && !providerMatch && !aliasMatch {
				continue
			}
		}

		if provider != "" && !strings.Contains(strings.ToLower(model.Provider), provider) {
			continue
		}

		if region != "" {
			var regionMatch bool
			for _, r := range model.RegionsSupported {
				if strings.Contains(strings.ToLower(r), region) {
					regionMatch = true
					break
				}
			}
			if !regionMatch {
				continue
			}
		}

		filteredModels = append(filteredModels, model)
	}

	// Sort models by provider and name
	sort.Slice(filteredModels, func(i, j int) bool {
		if filteredModels[i].Provider != filteredModels[j].Provider {
			return filteredModels[i].Provider < filteredModels[j].Provider
		}
		return filteredModels[i].ModelName < filteredModels[j].ModelName
	})

	searchResult := packageversions.BedrockModelSearchResult{
		Models:     filteredModels,
		TotalCount: len(filteredModels),
	}

	return packageversions.NewToolResultJSON(searchResult)
}

// getModel gets a specific AWS Bedrock model
func (t *BedrockTool) getModel(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	// Parse model ID
	modelID, ok := args["modelId"].(string)
	if !ok || modelID == "" {
		return nil, fmt.Errorf("missing required parameter: modelId")
	}

	// Get all models directly
	models, err := t.getModels(ctx)
	if err != nil {
		return nil, err
	}

	// Find model by ID
	for _, model := range models {
		if model.ModelID == modelID {
			return packageversions.NewToolResultJSON(model)
		}
	}

	return nil, fmt.Errorf("model not found: %s", modelID)
}

// getLatestClaudeSonnet gets the latest Claude Sonnet model
func (t *BedrockTool) getLatestClaudeSonnet(ctx context.Context) (*mcp.CallToolResult, error) {
	// Get all models directly
	models, err := t.getModels(ctx)
	if err != nil {
		return nil, err
	}

	// Find Claude Sonnet model
	for _, model := range models {
		if model.Provider == "anthropic" && strings.Contains(model.ModelName, "Sonnet") {
			return packageversions.NewToolResultJSON(model)
		}
	}

	return nil, fmt.Errorf("claude Sonnet model not found")
}

// matchesClaudeAlias checks if a query matches common Claude model aliases
func matchesClaudeAlias(query, modelName string) bool {
	query = strings.TrimSpace(strings.ToLower(query))

	// Normalise query - remove "claude-" or "claude " prefix if present
	normalisedQuery := query
	normalisedQuery = strings.TrimPrefix(normalisedQuery, "claude-")
	normalisedQuery = strings.TrimPrefix(normalisedQuery, "claude ")

	// Check for model family matches
	families := []string{"sonnet", "haiku", "opus"}
	for _, family := range families {
		if strings.Contains(modelName, family) {
			// Direct family match (e.g., "sonnet" or "haiku")
			if normalisedQuery == family {
				return true
			}
			// Family with version (e.g., "sonnet-4.5", "haiku-4")
			if strings.HasPrefix(normalisedQuery, family+"-") || strings.HasPrefix(normalisedQuery, family+" ") {
				return true
			}
			// Original query with claude- prefix (e.g., "claude-sonnet")
			if query == "claude-"+family || query == "claude "+family {
				return true
			}
		}
	}

	return false
}

// extractAnthropicModelsFromResult extracts Anthropic models from an MCP result
func extractAnthropicModelsFromResult(result *mcp.CallToolResult) []anthropic.AnthropicModel {
	if result == nil || len(result.Content) == 0 {
		return []anthropic.AnthropicModel{}
	}

	// The result is a JSON string in the first content item (text type)
	var searchResult anthropic.AnthropicModelSearchResult
	if textContent, ok := result.Content[0].(mcp.TextContent); ok {
		if err := json.Unmarshal([]byte(textContent.Text), &searchResult); err == nil {
			return searchResult.Models
		}
	}

	return []anthropic.AnthropicModel{}
}

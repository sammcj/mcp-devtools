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
	"github.com/sirupsen/logrus"
)

// BedrockTool handles AWS Bedrock model checking
type BedrockTool struct{}

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
func (t *BedrockTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Getting AWS Bedrock model information")

	// Parse action
	action := "list"
	if actionRaw, ok := args["action"].(string); ok && actionRaw != "" {
		action = actionRaw
	}

	// Handle different actions
	switch action {
	case "list":
		return t.listModels()
	case "search":
		return t.searchModels(args)
	case "get":
		return t.getModel(args)
	case "get_latest_claude_sonnet":
		return t.getLatestClaudeSonnet()
	default:
		return nil, fmt.Errorf("invalid action: %s", action)
	}
}

// listModels lists all available AWS Bedrock models
func (t *BedrockTool) listModels() (*mcp.CallToolResult, error) {
	// In a real implementation, this would fetch data from AWS Bedrock API
	// For now, we'll return a static list of models
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
			Provider:           "anthropic",
			ModelName:          "Claude Sonnet 4",
			ModelID:            "anthropic.claude-sonnet-4-20250514-v1:0",
			RegionsSupported:   []string{"us-east-1", "us-east-2", "us-west-2", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3", "ap-south-1", "ap-south-2", "ap-southeast-1", "ap-southeast-2", "eu-central-1", "eu-north-1", "eu-south-1", "eu-south-2", "eu-west-1", "eu-west-3"},
			InputModalities:    []string{"text", "image"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "anthropic",
			ModelName:          "Claude 3.5 Haiku",
			ModelID:            "anthropic.claude-3-5-haiku-20241022-v1:0",
			RegionsSupported:   []string{"us-east-1", "us-east-2", "us-west-2"},
			InputModalities:    []string{"text", "image"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "anthropic",
			ModelName:          "Claude Opus 4",
			ModelID:            "anthropic.claude-opus-4-20250514-v1:0",
			RegionsSupported:   []string{"us-east-1", "us-east-2", "us-west-2"},
			InputModalities:    []string{"text", "image"},
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

	// Sort models by provider and name
	sort.Slice(models, func(i, j int) bool {
		if models[i].Provider != models[j].Provider {
			return models[i].Provider < models[j].Provider
		}
		return models[i].ModelName < models[j].ModelName
	})

	result := packageversions.BedrockModelSearchResult{
		Models:     models,
		TotalCount: len(models),
	}

	return packageversions.NewToolResultJSON(result)
}

// searchModels searches for AWS Bedrock models
func (t *BedrockTool) searchModels(args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Get all models
	result, err := t.listModels()
	if err != nil {
		return nil, err
	}

	// Convert result to JSON string
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal(resultJSON, &data); err != nil {
		return nil, fmt.Errorf("failed to parse model data: %w", err)
	}

	// Get models
	modelsRaw, ok := data["models"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid model data format")
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
	for _, modelRaw := range modelsRaw {
		modelMap, ok := modelRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Convert to BedrockModel
		var model packageversions.BedrockModel
		modelJSON, err := json.Marshal(modelMap)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(modelJSON, &model); err != nil {
			continue
		}

		// Apply filters
		if query != "" {
			nameMatch := strings.Contains(strings.ToLower(model.ModelName), query)
			idMatch := strings.Contains(strings.ToLower(model.ModelID), query)
			providerMatch := strings.Contains(strings.ToLower(model.Provider), query)
			if !nameMatch && !idMatch && !providerMatch {
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
func (t *BedrockTool) getModel(args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse model ID
	modelID, ok := args["modelId"].(string)
	if !ok || modelID == "" {
		return nil, fmt.Errorf("missing required parameter: modelId")
	}

	// Get all models
	result, err := t.listModels()
	if err != nil {
		return nil, err
	}

	// Convert result to JSON string
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal(resultJSON, &data); err != nil {
		return nil, fmt.Errorf("failed to parse model data: %w", err)
	}

	// Get models
	modelsRaw, ok := data["models"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid model data format")
	}

	// Find model
	for _, modelRaw := range modelsRaw {
		modelMap, ok := modelRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Check model ID
		if id, ok := modelMap["modelId"].(string); ok && id == modelID {
			return packageversions.NewToolResultJSON(modelMap)
		}
	}

	return nil, fmt.Errorf("model not found: %s", modelID)
}

// getLatestClaudeSonnet gets the latest Claude Sonnet model
func (t *BedrockTool) getLatestClaudeSonnet() (*mcp.CallToolResult, error) {
	// Get all models
	result, err := t.listModels()
	if err != nil {
		return nil, err
	}

	// Convert result to JSON string
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	// Parse result
	var data map[string]interface{}
	if err := json.Unmarshal(resultJSON, &data); err != nil {
		return nil, fmt.Errorf("failed to parse model data: %w", err)
	}

	// Get models
	modelsRaw, ok := data["models"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid model data format")
	}

	// Find Claude Sonnet model
	for _, modelRaw := range modelsRaw {
		modelMap, ok := modelRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Convert to BedrockModel
		var model packageversions.BedrockModel
		modelJSON, err := json.Marshal(modelMap)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(modelJSON, &model); err != nil {
			continue
		}

		// Check if it's Claude Sonnet
		if model.Provider == "anthropic" && strings.Contains(model.ModelName, "Sonnet") {
			return packageversions.NewToolResultJSON(model)
		}
	}

	return nil, fmt.Errorf("claude Sonnet model not found") // Lowercased "claude"
}

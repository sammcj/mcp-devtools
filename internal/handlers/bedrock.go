package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

// BedrockHandler handles AWS Bedrock model checking
type BedrockHandler struct {
	client HTTPClient
	cache  *sync.Map
	logger *logrus.Logger
}

// NewBedrockHandler creates a new Bedrock handler
func NewBedrockHandler(logger *logrus.Logger, cache *sync.Map) *BedrockHandler {
	if cache == nil {
		cache = &sync.Map{}
	}
	return &BedrockHandler{
		client: DefaultHTTPClient,
		cache:  cache,
		logger: logger,
	}
}

// GetLatestVersion gets information about AWS Bedrock models
func (h *BedrockHandler) GetLatestVersion(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	h.logger.Info("Getting AWS Bedrock model information")

	// Parse action
	action := "list"
	if actionRaw, ok := args["action"].(string); ok && actionRaw != "" {
		action = actionRaw
	}

	// Handle different actions
	switch action {
	case "list":
		return h.listModels()
	case "search":
		return h.searchModels(args)
	case "get":
		return h.getModel(args)
	case "get_latest_claude_sonnet":
		return h.getLatestClaudeSonnet()
	default:
		return nil, fmt.Errorf("invalid action: %s", action)
	}
}

// listModels lists all available AWS Bedrock models
func (h *BedrockHandler) listModels() (*mcp.CallToolResult, error) {
	// In a real implementation, this would fetch data from AWS Bedrock API
	// For now, we'll return a static list of models
	models := []BedrockModel{
		{
			Provider:           "anthropic",
			ModelName:          "Claude 3 Opus",
			ModelID:            "anthropic.claude-3-opus-20240229-v1:0",
			RegionsSupported:   []string{"us-east-1", "us-west-2", "eu-central-1"},
			InputModalities:    []string{"text", "image"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "anthropic",
			ModelName:          "Claude 3 Sonnet",
			ModelID:            "anthropic.claude-3-sonnet-20240229-v1:0",
			RegionsSupported:   []string{"us-east-1", "us-west-2", "eu-central-1"},
			InputModalities:    []string{"text", "image"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "anthropic",
			ModelName:          "Claude 3 Haiku",
			ModelID:            "anthropic.claude-3-haiku-20240307-v1:0",
			RegionsSupported:   []string{"us-east-1", "us-west-2", "eu-central-1"},
			InputModalities:    []string{"text", "image"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "amazon",
			ModelName:          "Titan Text G1 - Express",
			ModelID:            "amazon.titan-text-express-v1",
			RegionsSupported:   []string{"us-east-1", "us-west-2"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "amazon",
			ModelName:          "Titan Image Generator G1",
			ModelID:            "amazon.titan-image-generator-v1",
			RegionsSupported:   []string{"us-east-1", "us-west-2"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"image"},
			StreamingSupported: false,
		},
		{
			Provider:           "cohere",
			ModelName:          "Command",
			ModelID:            "cohere.command-text-v14",
			RegionsSupported:   []string{"us-east-1", "us-west-2"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "meta",
			ModelName:          "Llama 2 Chat 13B",
			ModelID:            "meta.llama2-13b-chat-v1",
			RegionsSupported:   []string{"us-east-1", "us-west-2"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "meta",
			ModelName:          "Llama 2 Chat 70B",
			ModelID:            "meta.llama2-70b-chat-v1",
			RegionsSupported:   []string{"us-east-1", "us-west-2"},
			InputModalities:    []string{"text"},
			OutputModalities:   []string{"text"},
			StreamingSupported: true,
		},
		{
			Provider:           "stability",
			ModelName:          "Stable Diffusion XL 1.0",
			ModelID:            "stability.stable-diffusion-xl-v1",
			RegionsSupported:   []string{"us-east-1", "us-west-2"},
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

	result := BedrockModelSearchResult{
		Models:     models,
		TotalCount: len(models),
	}

	return NewToolResultJSON(result)
}

// searchModels searches for AWS Bedrock models
func (h *BedrockHandler) searchModels(args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Get all models
	result, err := h.listModels()
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
	var filteredModels []BedrockModel
	for _, modelRaw := range modelsRaw {
		modelMap, ok := modelRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Convert to BedrockModel
		var model BedrockModel
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

	searchResult := BedrockModelSearchResult{
		Models:     filteredModels,
		TotalCount: len(filteredModels),
	}

	return NewToolResultJSON(searchResult)
}

// getModel gets a specific AWS Bedrock model
func (h *BedrockHandler) getModel(args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse model ID
	modelID, ok := args["modelId"].(string)
	if !ok || modelID == "" {
		return nil, fmt.Errorf("missing required parameter: modelId")
	}

	// Get all models
	result, err := h.listModels()
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
			return NewToolResultJSON(modelMap)
		}
	}

	return nil, fmt.Errorf("model not found: %s", modelID)
}

// getLatestClaudeSonnet gets the latest Claude Sonnet model
func (h *BedrockHandler) getLatestClaudeSonnet() (*mcp.CallToolResult, error) {
	// Get all models
	result, err := h.listModels()
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
		var model BedrockModel
		modelJSON, err := json.Marshal(modelMap)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(modelJSON, &model); err != nil {
			continue
		}

		// Check if it's Claude Sonnet
		if model.Provider == "anthropic" && strings.Contains(model.ModelName, "Sonnet") {
			return NewToolResultJSON(model)
}
}

return nil, fmt.Errorf("claude Sonnet model not found") // Lowercased "claude"
}

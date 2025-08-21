package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// DynamicAPITool implements the tools.Tool interface for configured APIs
type DynamicAPITool struct {
	apiName  string
	apiDef   APIDefinition
	client   *HTTPClient
	toolName string
}

// CacheEntry represents a cached API response
type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

// NewDynamicAPITool creates a new dynamic API tool
func NewDynamicAPITool(apiName string, apiDef APIDefinition) *DynamicAPITool {
	return &DynamicAPITool{
		apiName:  apiName,
		apiDef:   apiDef,
		client:   NewHTTPClient(apiDef),
		toolName: apiName + "_api",
	}
}

// Definition returns the tool's definition for MCP registration
func (t *DynamicAPITool) Definition() mcp.Tool {
	// Build endpoint enum values
	endpointNames := make([]string, len(t.apiDef.Endpoints))
	for i, endpoint := range t.apiDef.Endpoints {
		endpointNames[i] = endpoint.Name
	}

	description := t.apiDef.Description
	if description == "" {
		description = fmt.Sprintf("API tool for %s", t.apiName)
	}

	// Create tool with basic parameters
	toolOptions := []mcp.ToolOption{
		mcp.WithDescription(description),
		mcp.WithString("endpoint",
			mcp.Required(),
			mcp.Enum(endpointNames...),
			mcp.Description("API endpoint to call"),
		),
	}

	// Add dynamic parameters based on all possible endpoint parameters
	allParams := t.collectAllParameters()
	for _, param := range allParams {
		switch param.Type {
		case "string":
			stringOptions := []mcp.PropertyOption{mcp.Description(param.Description)}
			if param.Required {
				stringOptions = append(stringOptions, mcp.Required())
			}
			if param.Default != nil {
				if defaultStr, ok := param.Default.(string); ok {
					stringOptions = append(stringOptions, mcp.DefaultString(defaultStr))
				}
			}
			if len(param.Enum) > 0 {
				stringOptions = append(stringOptions, mcp.Enum(param.Enum...))
			}
			toolOptions = append(toolOptions, mcp.WithString(param.Name, stringOptions...))

		case "number":
			numberOptions := []mcp.PropertyOption{mcp.Description(param.Description)}
			if param.Required {
				numberOptions = append(numberOptions, mcp.Required())
			}
			if param.Default != nil {
				if defaultNum, ok := param.Default.(float64); ok {
					numberOptions = append(numberOptions, mcp.DefaultNumber(defaultNum))
				} else if defaultInt, ok := param.Default.(int); ok {
					numberOptions = append(numberOptions, mcp.DefaultNumber(float64(defaultInt)))
				}
			}
			toolOptions = append(toolOptions, mcp.WithNumber(param.Name, numberOptions...))

		case "boolean":
			boolOptions := []mcp.PropertyOption{mcp.Description(param.Description)}
			if param.Required {
				boolOptions = append(boolOptions, mcp.Required())
			}
			if param.Default != nil {
				if defaultBool, ok := param.Default.(bool); ok {
					boolOptions = append(boolOptions, mcp.DefaultBool(defaultBool))
				}
			}
			toolOptions = append(toolOptions, mcp.WithBoolean(param.Name, boolOptions...))

		case "array":
			arrayOptions := []mcp.PropertyOption{mcp.Description(param.Description)}
			if param.Required {
				arrayOptions = append(arrayOptions, mcp.Required())
			}
			toolOptions = append(toolOptions, mcp.WithArray(param.Name, arrayOptions...))

		case "object":
			objectOptions := []mcp.PropertyOption{mcp.Description(param.Description)}
			if param.Required {
				objectOptions = append(objectOptions, mcp.Required())
			}
			toolOptions = append(toolOptions, mcp.WithObject(param.Name, objectOptions...))
		}
	}

	return mcp.NewTool(t.toolName, toolOptions...)
}

// collectAllParameters collects all unique parameters across all endpoints
// Makes all parameters optional at MCP level - validation happens at execution time
func (t *DynamicAPITool) collectAllParameters() []ParameterConfig {
	paramMap := make(map[string]ParameterConfig)

	for _, endpoint := range t.apiDef.Endpoints {
		for _, param := range endpoint.Parameters {
			// Use parameter name as key, keep the first occurrence but merge descriptions
			if existing, exists := paramMap[param.Name]; exists {
				// Keep existing but enhance description if needed
				if param.Description != existing.Description {
					paramMap[param.Name] = ParameterConfig{
						Name:        existing.Name,
						Type:        existing.Type,
						Description: fmt.Sprintf("%s (varies by endpoint)", existing.Description),
						Required:    false, // Always optional at MCP level
						Default:     existing.Default,
						Enum:        existing.Enum,
					}
				}
			} else {
				// Make a copy and set as optional at MCP level
				paramCopy := param
				paramCopy.Required = false
				paramMap[param.Name] = paramCopy
			}
		}
	}

	// Convert map to slice
	params := make([]ParameterConfig, 0, len(paramMap))
	for _, param := range paramMap {
		params = append(params, param)
	}

	return params
}

// Execute executes the tool's logic
func (t *DynamicAPITool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Check if API tool is enabled
	if !tools.IsToolEnabled("api") {
		return nil, fmt.Errorf("API tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'api'")
	}

	logger.WithField("api", t.apiName).Info("Executing API tool")

	// Parse endpoint parameter
	endpointName, ok := args["endpoint"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: endpoint")
	}

	// Find the endpoint configuration
	var endpoint *EndpointConfig
	for i := range t.apiDef.Endpoints {
		if t.apiDef.Endpoints[i].Name == endpointName {
			endpoint = &t.apiDef.Endpoints[i]
			break
		}
	}

	if endpoint == nil {
		return nil, fmt.Errorf("unknown endpoint: %s", endpointName)
	}

	// Parse parameters - now they come directly from args, not nested in "parameters"
	parameters := make(map[string]interface{})
	for _, param := range endpoint.Parameters {
		if value, exists := args[param.Name]; exists {
			parameters[param.Name] = value
		} else if param.Required {
			return nil, fmt.Errorf("missing required parameter: %s", param.Name)
		} else if param.Default != nil {
			parameters[param.Name] = param.Default
		}
	}

	// Check cache
	cacheKey := fmt.Sprintf("%s:%s:%v", t.apiName, endpointName, parameters)
	if cachedEntryRaw, ok := cache.Load(cacheKey); ok {
		if cachedEntry, ok := cachedEntryRaw.(CacheEntry); ok {
			if time.Now().Before(cachedEntry.ExpiresAt) {
				logger.WithField("cache_key", cacheKey).Debug("Using cached response")
				// Convert cached data to JSON string
				jsonBytes, err := json.MarshalIndent(cachedEntry.Data, "", "  ")
				if err != nil {
					return nil, fmt.Errorf("failed to marshal cached result: %w", err)
				}
				return mcp.NewToolResultText(string(jsonBytes)), nil
			}
			// Remove expired entry
			cache.Delete(cacheKey)
		}
	}

	// Execute the API request
	result, err := t.client.ExecuteRequest(ctx, *endpoint, parameters)
	if err != nil {
		// Check if it's a security error
		if secErr, ok := err.(*security.SecurityError); ok {
			return nil, fmt.Errorf("security block [ID: %s]: %s Check with the user if you may use security_override tool with ID %s",
				secErr.GetSecurityID(), err.Error(), secErr.GetSecurityID())
		}
		return nil, err
	}

	// Cache the result
	if t.apiDef.CacheTTL > 0 {
		cacheEntry := CacheEntry{
			Data:      result,
			ExpiresAt: time.Now().Add(time.Duration(t.apiDef.CacheTTL) * time.Second),
		}
		cache.Store(cacheKey, cacheEntry)
		logger.WithFields(logrus.Fields{
			"cache_key": cacheKey,
			"ttl":       t.apiDef.CacheTTL,
		}).Debug("Cached API response")
	}

	// Add metadata to result
	result["endpoint"] = endpointName
	result["api"] = t.apiName
	result["cached"] = false

	// Convert result to JSON string
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// ProvideExtendedInfo provides extended help information for the API tool
func (t *DynamicAPITool) ProvideExtendedInfo() *tools.ExtendedHelp {
	examples := make([]tools.ToolExample, 0, len(t.apiDef.Endpoints))
	parameterDetails := make(map[string]string)

	// Generate examples for each endpoint
	for _, endpoint := range t.apiDef.Endpoints {
		// Build example parameters
		exampleParams := make(map[string]interface{})
		for _, param := range endpoint.Parameters {
			switch param.Type {
			case "string":
				if len(param.Enum) > 0 {
					exampleParams[param.Name] = param.Enum[0]
				} else {
					exampleParams[param.Name] = fmt.Sprintf("example_%s", param.Name)
				}
			case "number":
				exampleParams[param.Name] = 42
			case "boolean":
				exampleParams[param.Name] = true
			}
		}

		// Build arguments map with endpoint and parameters merged at top level
		args := make(map[string]interface{})
		args["endpoint"] = endpoint.Name
		for k, v := range exampleParams {
			args[k] = v
		}

		examples = append(examples, tools.ToolExample{
			Description:    fmt.Sprintf("Call %s endpoint: %s", endpoint.Name, endpoint.Description),
			Arguments:      args,
			ExpectedResult: fmt.Sprintf("API response with data from %s %s", endpoint.Method, endpoint.Path),
		})

		// Add parameter details for this endpoint
		for _, param := range endpoint.Parameters {
			key := fmt.Sprintf("%s.%s", endpoint.Name, param.Name)
			details := param.Description
			if param.Required {
				details += " (required)"
			}
			if len(param.Enum) > 0 {
				details += fmt.Sprintf(" - allowed values: %v", param.Enum)
			}
			parameterDetails[key] = details
		}
	}

	commonPatterns := []string{
		"Use the 'endpoint' parameter to specify which API endpoint to call",
		"Pass endpoint-specific parameters in the 'parameters' object",
		"Required parameters must be provided for each endpoint",
		"Responses are automatically cached based on the configured TTL",
	}

	troubleshooting := []tools.TroubleshootingTip{
		{
			Problem:  "Authentication failed or missing credentials",
			Solution: "Check that the required environment variable is set with valid credentials",
		},
		{
			Problem:  "Unknown endpoint error",
			Solution: "Use one of the configured endpoint names from the enum list",
		},
		{
			Problem:  "Missing required parameter",
			Solution: "Check the endpoint configuration and provide all required parameters",
		},
	}

	return &tools.ExtendedHelp{
		Examples:         examples,
		CommonPatterns:   commonPatterns,
		Troubleshooting:  troubleshooting,
		ParameterDetails: parameterDetails,
		WhenToUse:        fmt.Sprintf("When you need to interact with the %s API", t.apiName),
		WhenNotToUse:     "When the specific endpoint is not configured or for operations requiring direct HTTP control",
	}
}

// RegisterConfiguredAPIs loads API configuration and registers tools
func RegisterConfiguredAPIs() error {
	configPath := "~/.mcp-devtools/apis.yaml"
	config, err := LoadAPIConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load API configuration: %w", err)
	}

	// Register each configured API as a tool
	for apiName, apiDef := range config.APIs {
		tool := NewDynamicAPITool(apiName, apiDef)
		registry.Register(tool)
	}

	return nil
}

// init registers configured API tools - this is called when the package is imported
func init() {
	// Only register APIs if the tool is enabled (disabled by default for security)
	if tools.IsToolEnabled("api") {
		// Register configured APIs at startup
		if err := RegisterConfiguredAPIs(); err != nil {
			// Log the error for debugging but don't fail startup
			// This allows the server to start even with missing/invalid API config
			logrus.WithError(err).Info("Failed to register configured APIs; continuing startup")
		}
	}
}

package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// ConfluenceTool implements Confluence search and content retrieval
type ConfluenceTool struct {
	client *Client
}

// init registers the confluence tool with the registry
func init() {
	// Only register if Confluence is configured
	if IsConfigured() {
		registry.Register(&ConfluenceTool{})
	}
}

// Definition returns the tool's definition for MCP registration
func (t *ConfluenceTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"confluence_search",
		mcp.WithDescription(`Search Confluence and retrieve content converted to Markdown.`),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query string to find content in Confluence"),
		),
		mcp.WithString("space_key",
			mcp.Description("Optional space key to limit search to a specific Confluence space"),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum number of results to fetch and convert (default: 3, max: 10)"),
			mcp.DefaultNumber(3),
		),
		mcp.WithArray("content_types",
			mcp.Description("Filter by content types (e.g., ['page', 'blogpost'])"),
		),
	)
}

// Execute executes the confluence search tool
func (t *ConfluenceTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Executing Confluence search tool")

	// Initialize client if not already done
	if t.client == nil {
		client, err := NewClient()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Confluence client: %w", err)
		}
		t.client = client
	}

	// Parse and validate parameters
	request, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"query":         request.Query,
		"space_key":     request.SpaceKey,
		"max_results":   request.MaxResults,
		"content_types": request.ContentTypes,
	}).Debug("Confluence search parameters")

	// Check if the query looks like a URL
	if t.isURL(request.Query) {
		logger.Info("Query appears to be a URL, fetching specific page")
		return t.fetchSpecificPage(ctx, logger, cache, request.Query)
	}

	// Check cache first for search queries
	cacheKey := t.buildCacheKey(request)
	if cachedResult, ok := cache.Load(cacheKey); ok {
		logger.Debug("Returning cached Confluence search result")
		if response, ok := cachedResult.(*SearchResponse); ok {
			return t.newToolResultJSON(response)
		}
	}

	// Perform the search
	response, err := t.client.Search(ctx, logger, request)
	if err != nil {
		// Return structured error response
		errorResponse := map[string]interface{}{
			"error":   err.Error(),
			"query":   request.Query,
			"success": false,
		}
		return t.newToolResultJSON(errorResponse)
	}

	// Cache the successful response for 5 minutes
	cache.Store(cacheKey, response)

	logger.WithFields(logrus.Fields{
		"query":           request.Query,
		"results_found":   response.TotalCount,
		"results_fetched": len(response.Results),
	}).Info("Confluence search completed successfully")

	return t.newToolResultJSON(response)
}

// parseRequest parses and validates the tool arguments
func (t *ConfluenceTool) parseRequest(args map[string]interface{}) (*SearchRequest, error) {
	// Parse query (required)
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: query")
	}

	request := &SearchRequest{
		Query:      query,
		MaxResults: 3, // Default
	}

	// Parse space_key (optional)
	if spaceKey, ok := args["space_key"].(string); ok && spaceKey != "" {
		request.SpaceKey = spaceKey
	}

	// Parse max_results (optional)
	if maxResultsRaw, ok := args["max_results"].(float64); ok {
		maxResults := int(maxResultsRaw)
		if maxResults < 1 {
			return nil, fmt.Errorf("max_results must be at least 1")
		}
		if maxResults > 10 {
			return nil, fmt.Errorf("max_results cannot exceed 10")
		}
		request.MaxResults = maxResults
	}

	// Parse content_types (optional)
	if contentTypesRaw, ok := args["content_types"].([]interface{}); ok {
		contentTypes := make([]string, 0, len(contentTypesRaw))
		for _, ct := range contentTypesRaw {
			if contentType, ok := ct.(string); ok {
				// Validate content type
				switch contentType {
				case "page", "blogpost", "comment", "attachment":
					contentTypes = append(contentTypes, contentType)
				default:
					return nil, fmt.Errorf("invalid content type: %s. Valid types: page, blogpost, comment, attachment", contentType)
				}
			}
		}
		request.ContentTypes = contentTypes
	}

	return request, nil
}

// buildCacheKey creates a cache key for the request
func (t *ConfluenceTool) buildCacheKey(request *SearchRequest) string {
	key := fmt.Sprintf("confluence_search:%s", request.Query)
	if request.SpaceKey != "" {
		key += fmt.Sprintf(":space:%s", request.SpaceKey)
	}
	key += fmt.Sprintf(":max:%d", request.MaxResults)
	if len(request.ContentTypes) > 0 {
		key += fmt.Sprintf(":types:%v", request.ContentTypes)
	}
	return key
}

// isURL checks if a string looks like a URL
func (t *ConfluenceTool) isURL(query string) bool {
	return len(query) > 10 && (query[:7] == "http://" ||
		query[:8] == "https://" ||
		(len(query) > 20 && query[:4] == "www."))
}

// fetchSpecificPage fetches content from a specific Confluence page URL
func (t *ConfluenceTool) fetchSpecificPage(ctx context.Context, logger *logrus.Logger, cache *sync.Map, pageURL string) (*mcp.CallToolResult, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("confluence_page:%s", pageURL)
	if cachedResult, ok := cache.Load(cacheKey); ok {
		logger.Debug("Returning cached Confluence page result")
		if response, ok := cachedResult.(*SearchResponse); ok {
			return t.newToolResultJSON(response)
		}
	}

	// Fetch the specific page
	response, err := t.client.FetchSpecificPage(ctx, logger, pageURL)
	if err != nil {
		// Return structured error response
		errorResponse := map[string]interface{}{
			"error":    err.Error(),
			"page_url": pageURL,
			"success":  false,
		}
		return t.newToolResultJSON(errorResponse)
	}

	// Cache the successful response
	cache.Store(cacheKey, response)

	logger.WithFields(logrus.Fields{
		"page_url":        pageURL,
		"results_fetched": len(response.Results),
	}).Info("Confluence page fetch completed successfully")

	return t.newToolResultJSON(response)
}

// newToolResultJSON creates a new tool result with JSON content
func (t *ConfluenceTool) newToolResultJSON(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

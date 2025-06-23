package brave

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

// newToolResultJSON creates a new tool result with JSON content
func newToolResultJSON(data interface{}) (*mcp.CallToolResult, error) {
	buffer := &strings.Builder{}
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false) // Don't escape HTML characters like < and >

	if err := encoder.Encode(data); err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Remove the trailing newline that Encode adds
	jsonString := strings.TrimSuffix(buffer.String(), "\n")
	return mcp.NewToolResultText(jsonString), nil
}

// BraveWebSearchTool implements web search using Brave Search API
type BraveWebSearchTool struct {
	client *BraveClient
}

// Definition returns the tool's definition for MCP registration
func (t *BraveWebSearchTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"brave_web_search",
		mcp.WithDescription("Performs a web search using the Brave Search API, ideal for general queries, and online content. Use this for broad information gathering, recent events, or when you need diverse web sources. Maximum 20 results per request"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The term to search the internet for"),
		),
		mcp.WithNumber("count",
			mcp.Description("The number of results to return, minimum 1, maximum 20"),
			mcp.DefaultNumber(10),
		),
		mcp.WithNumber("offset",
			mcp.Description("The offset for pagination, minimum 0"),
			mcp.DefaultNumber(0),
		),
		mcp.WithString("freshness",
			mcp.Description("Filters search results by when they were discovered. The following values are supported:\n- pd: Discovered within the last 24 hours.\n- pw: Discovered within the last 7 Days.\n- pm: Discovered within the last 31 Days.\n- py: Discovered within the last 365 Days.\n- YYYY-MM-DDtoYYYY-MM-DD: Custom date range (e.g., 2022-04-01to2022-07-30)"),
		),
	)
}

// Execute executes the web search tool
func (t *BraveWebSearchTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Executing Brave web search")

	// Parse required parameters
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: query")
	}

	// Parse optional parameters with defaults
	count := 10
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 20 {
			return nil, fmt.Errorf("count must be between 1 and 20, got %d", count)
		}
	}

	offset := 0
	if offsetRaw, ok := args["offset"].(float64); ok {
		offset = int(offsetRaw)
		if offset < 0 {
			return nil, fmt.Errorf("offset must be >= 0, got %d", offset)
		}
	}

	freshness := ""
	if freshnessRaw, ok := args["freshness"].(string); ok {
		freshness = freshnessRaw
	}

	logger.WithFields(logrus.Fields{
		"query":     query,
		"count":     count,
		"offset":    offset,
		"freshness": freshness,
	}).Debug("Brave web search parameters")

	// Perform the search
	response, err := t.client.WebSearch(ctx, logger, query, count, offset, freshness)
	if err != nil {
		logger.WithError(err).Error("Brave web search failed")
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	// Check if we have results
	if response.Web == nil || len(response.Web.Results) == 0 {
		logger.WithField("query", query).Info("No web search results found")
		result := SearchResponse{
			Query:       query,
			ResultCount: 0,
			Results:     []SearchResult{},
			Provider:    "Brave",
			Timestamp:   time.Now(),
		}
		return newToolResultJSON(result)
	}

	// Convert results to unified format
	results := make([]SearchResult, 0, len(response.Web.Results))
	for _, webResult := range response.Web.Results {
		metadata := make(map[string]interface{})
		if webResult.Age != "" {
			metadata["age"] = webResult.Age
		}

		results = append(results, SearchResult{
			Title:       decodeHTMLEntities(webResult.Title),
			URL:         webResult.URL,
			Description: decodeHTMLEntities(webResult.Description),
			Type:        "web",
			Metadata:    metadata,
		})
	}

	// Create response
	result := SearchResponse{
		Query:       query,
		ResultCount: len(results),
		Results:     results,
		Provider:    "Brave",
		Timestamp:   time.Now(),
	}

	logger.WithFields(logrus.Fields{
		"query":        query,
		"result_count": len(results),
	}).Info("Brave web search completed successfully")

	return newToolResultJSON(result)
}

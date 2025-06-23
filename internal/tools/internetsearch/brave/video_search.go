package brave

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

// BraveVideoSearchTool implements video search using Brave Search API
type BraveVideoSearchTool struct {
	client *BraveClient
}

// Definition returns the tool's definition for MCP registration
func (t *BraveVideoSearchTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"brave_video_search",
		mcp.WithDescription("Searches for videos using the Brave Search API. Use this for video content, tutorials, or any media-related queries. Returns a list of videos with titles, URLs, and descriptions. Maximum 20 results per request."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The term to search the internet for videos of"),
		),
		mcp.WithNumber("count",
			mcp.Description("The number of results to return, minimum 1, maximum 20"),
			mcp.DefaultNumber(10),
		),
		mcp.WithString("freshness",
			mcp.Description("Filters search results by when they were discovered. The following values are supported:\n- pd: Discovered within the last 24 hours.\n- pw: Discovered within the last 7 Days.\n- pm: Discovered within the last 31 Days.\n- py: Discovered within the last 365 Days.\n- YYYY-MM-DDtoYYYY-MM-DD: Custom date range (e.g., 2022-04-01to2022-07-30)"),
		),
	)
}

// Execute executes the video search tool
func (t *BraveVideoSearchTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Executing Brave video search")

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

	freshness := ""
	if freshnessRaw, ok := args["freshness"].(string); ok {
		freshness = freshnessRaw
	}

	logger.WithFields(logrus.Fields{
		"query":     query,
		"count":     count,
		"freshness": freshness,
	}).Debug("Brave video search parameters")

	// Perform the search
	response, err := t.client.VideoSearch(ctx, logger, query, count, freshness)
	if err != nil {
		logger.WithError(err).Error("Brave video search failed")
		return nil, fmt.Errorf("video search failed: %w", err)
	}

	// Check if we have results
	if len(response.Results) == 0 {
		logger.WithField("query", query).Info("No video search results found")
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
	results := make([]SearchResult, 0, len(response.Results))
	for _, videoResult := range response.Results {
		metadata := make(map[string]interface{})
		if videoResult.Video.Duration != "" {
			metadata["duration"] = videoResult.Video.Duration
		}
		if videoResult.Video.Views != nil {
			metadata["views"] = videoResult.Video.Views
		}
		if videoResult.Video.Creator != "" {
			metadata["creator"] = videoResult.Video.Creator
		}

		results = append(results, SearchResult{
			Title:       videoResult.Title,
			URL:         videoResult.URL,
			Description: fmt.Sprintf("Video: %s", videoResult.Title),
			Type:        "video",
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
	}).Info("Brave video search completed successfully")

	return newToolResultJSON(result)
}

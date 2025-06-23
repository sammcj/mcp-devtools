package brave

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

// BraveImageSearchTool implements image search using Brave Search API
type BraveImageSearchTool struct {
	client *BraveClient
}

// Definition returns the tool's definition for MCP registration
func (t *BraveImageSearchTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"brave_image_search",
		mcp.WithDescription("A tool for searching the web for images using the Brave Search API."),
		mcp.WithString("searchTerm",
			mcp.Required(),
			mcp.Description("The term to search the internet for images of"),
		),
		mcp.WithNumber("count",
			mcp.Description("The number of images to search for, minimum 1, maximum 3"),
			mcp.DefaultNumber(1),
		),
	)
}

// Execute executes the image search tool
func (t *BraveImageSearchTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Executing Brave image search")

	// Parse required parameters
	searchTerm, ok := args["searchTerm"].(string)
	if !ok || searchTerm == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: searchTerm")
	}

	// Parse optional parameters with defaults
	count := 1
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 3 {
			return nil, fmt.Errorf("count must be between 1 and 3, got %d", count)
		}
	}

	logger.WithFields(logrus.Fields{
		"searchTerm": searchTerm,
		"count":      count,
	}).Debug("Brave image search parameters")

	// Perform the search
	response, err := t.client.ImageSearch(ctx, logger, searchTerm, count)
	if err != nil {
		logger.WithError(err).Error("Brave image search failed")
		return nil, fmt.Errorf("image search failed: %w", err)
	}

	// Check if we have results
	if len(response.Results) == 0 {
		logger.WithField("searchTerm", searchTerm).Info("No image search results found")
		result := SearchResponse{
			Query:       searchTerm,
			ResultCount: 0,
			Results:     []SearchResult{},
			Provider:    "Brave",
			Timestamp:   time.Now(),
		}
		return newToolResultJSON(result)
	}

	// Convert results to unified format
	results := make([]SearchResult, 0, len(response.Results))
	for _, imageResult := range response.Results {
		metadata := make(map[string]interface{})
		metadata["imageURL"] = imageResult.Properties.URL
		if imageResult.Properties.Format != "" {
			metadata["format"] = imageResult.Properties.Format
		}
		if imageResult.Properties.Width > 0 {
			metadata["width"] = imageResult.Properties.Width
		}
		if imageResult.Properties.Height > 0 {
			metadata["height"] = imageResult.Properties.Height
		}

		results = append(results, SearchResult{
			Title:       imageResult.Title,
			URL:         imageResult.URL,
			Description: fmt.Sprintf("Image: %s", imageResult.Title),
			Type:        "image",
			Metadata:    metadata,
		})
	}

	// Create response
	result := SearchResponse{
		Query:       searchTerm,
		ResultCount: len(results),
		Results:     results,
		Provider:    "Brave",
		Timestamp:   time.Now(),
	}

	logger.WithFields(logrus.Fields{
		"searchTerm":   searchTerm,
		"result_count": len(results),
	}).Info("Brave image search completed successfully")

	return newToolResultJSON(result)
}

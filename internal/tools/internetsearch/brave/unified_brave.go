package brave

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// UnifiedBraveSearchTool provides a single interface for all Brave Search operations
type UnifiedBraveSearchTool struct {
	client *BraveClient
}

func init() {
	apiKey := os.Getenv("BRAVE_API_KEY")
	if apiKey == "" {
		// Tool will not be registered - this is expected behaviour
		return
	}

	registry.Register(&UnifiedBraveSearchTool{
		client: NewBraveClient(apiKey),
	})
}

// Definition returns the tool's definition for MCP registration
func (t *UnifiedBraveSearchTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"brave_search",
		mcp.WithDescription(`Brave Internet Search. Supports web, image, news, video, and local search with a single interface.

Search Types:
- web: General web search for broad information gathering
- image: Search for images (max 3 results)
- news: Search for recent news articles and events
- video: Search for video content and tutorials
- local: Search for local businesses and places (requires Pro API plan)

Examples:
- Web search: {"type": "web", "query": "golang best practices", "count": 10}
- Image search: {"type": "image", "query": "golang gopher mascot", "count": 3}
- News search: {"type": "news", "query": "AI breakthrough", "freshness": "pd"}
- Video search: {"type": "video", "query": "golang tutorial"}
- Local search: {"type": "local", "query": "pizza near Central Park"}

Freshness options (web/news/video): pd (24h), pw (7d), pm (31d), py (365d), or custom range (YYYY-MM-DDtoYYYY-MM-DD)`),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Search type: 'web', 'image', 'news', 'video', or 'local'"),
			mcp.Enum("web", "image", "news", "video", "local"),
		),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query term"),
		),
		mcp.WithNumber("count",
			mcp.Description("Number of results (1-20 for most types, 1-3 for images)"),
			mcp.DefaultNumber(10),
		),
		mcp.WithNumber("offset",
			mcp.Description("Pagination offset (web search only)"),
			mcp.DefaultNumber(0),
		),
		mcp.WithString("freshness",
			mcp.Description("Time filter for web/news/video: pd, pw, pm, py, or custom range"),
		),
	)
}

// Execute executes the unified Brave search tool
func (t *UnifiedBraveSearchTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse required parameters
	searchType, ok := args["type"].(string)
	if !ok || searchType == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: type")
	}

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: query")
	}

	logger.WithFields(logrus.Fields{
		"type":  searchType,
		"query": query,
	}).Info("Executing unified Brave search")

	switch searchType {
	case "web":
		return t.executeWebSearch(ctx, logger, cache, args)
	case "image":
		return t.executeImageSearch(ctx, logger, cache, args)
	case "news":
		return t.executeNewsSearch(ctx, logger, cache, args)
	case "video":
		return t.executeVideoSearch(ctx, logger, cache, args)
	case "local":
		return t.executeLocalSearch(ctx, logger, cache, args)
	default:
		return nil, fmt.Errorf("invalid search type: %s. Must be one of: web, image, news, video, local", searchType)
	}
}

// executeWebSearch handles web search
func (t *UnifiedBraveSearchTool) executeWebSearch(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	query := args["query"].(string)

	// Parse optional parameters
	count := 10
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 20 {
			return nil, fmt.Errorf("count must be between 1 and 20 for web search, got %d", count)
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

	response, err := t.client.WebSearch(ctx, logger, query, count, offset, freshness)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	// Convert to unified format
	if response.Web == nil || len(response.Web.Results) == 0 {
		return t.createEmptyResponse(query)
	}

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

	return t.createSuccessResponse(query, results, logger)
}

// executeImageSearch handles image search
func (t *UnifiedBraveSearchTool) executeImageSearch(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	query := args["query"].(string)

	// Parse optional parameters
	count := 1
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 3 {
			return nil, fmt.Errorf("count must be between 1 and 3 for image search, got %d", count)
		}
	}

	logger.WithFields(logrus.Fields{
		"query": query,
		"count": count,
	}).Debug("Brave image search parameters")

	response, err := t.client.ImageSearch(ctx, logger, query, count)
	if err != nil {
		return nil, fmt.Errorf("image search failed: %w", err)
	}

	// Convert to unified format
	if len(response.Results) == 0 {
		return t.createEmptyResponse(query)
	}

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
			Title:       decodeHTMLEntities(imageResult.Title),
			URL:         imageResult.URL,
			Description: fmt.Sprintf("Image: %s", decodeHTMLEntities(imageResult.Title)),
			Type:        "image",
			Metadata:    metadata,
		})
	}

	return t.createSuccessResponse(query, results, logger)
}

// executeNewsSearch handles news search
func (t *UnifiedBraveSearchTool) executeNewsSearch(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	query := args["query"].(string)

	// Parse optional parameters
	count := 10
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 20 {
			return nil, fmt.Errorf("count must be between 1 and 20 for news search, got %d", count)
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
	}).Debug("Brave news search parameters")

	response, err := t.client.NewsSearch(ctx, logger, query, count, freshness)
	if err != nil {
		return nil, fmt.Errorf("news search failed: %w", err)
	}

	// Convert to unified format
	if len(response.Results) == 0 {
		return t.createEmptyResponse(query)
	}

	results := make([]SearchResult, 0, len(response.Results))
	for _, newsResult := range response.Results {
		metadata := make(map[string]interface{})
		if newsResult.Age != "" {
			metadata["age"] = newsResult.Age
		}

		results = append(results, SearchResult{
			Title:       decodeHTMLEntities(newsResult.Title),
			URL:         newsResult.URL,
			Description: decodeHTMLEntities(newsResult.Description),
			Type:        "news",
			Metadata:    metadata,
		})
	}

	return t.createSuccessResponse(query, results, logger)
}

// executeVideoSearch handles video search
func (t *UnifiedBraveSearchTool) executeVideoSearch(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	query := args["query"].(string)

	// Parse optional parameters
	count := 10
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 20 {
			return nil, fmt.Errorf("count must be between 1 and 20 for video search, got %d", count)
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

	response, err := t.client.VideoSearch(ctx, logger, query, count, freshness)
	if err != nil {
		return nil, fmt.Errorf("video search failed: %w", err)
	}

	// Convert to unified format
	if len(response.Results) == 0 {
		return t.createEmptyResponse(query)
	}

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
			Title:       decodeHTMLEntities(videoResult.Title),
			URL:         videoResult.URL,
			Description: fmt.Sprintf("Video: %s", decodeHTMLEntities(videoResult.Title)),
			Type:        "video",
			Metadata:    metadata,
		})
	}

	return t.createSuccessResponse(query, results, logger)
}

// executeLocalSearch handles local search with web fallback
func (t *UnifiedBraveSearchTool) executeLocalSearch(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	query := args["query"].(string)

	// Parse optional parameters
	count := 10
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 20 {
			return nil, fmt.Errorf("count must be between 1 and 20 for local search, got %d", count)
		}
	}

	logger.WithFields(logrus.Fields{
		"query": query,
		"count": count,
	}).Debug("Brave local search parameters")

	response, err := t.client.LocalSearch(ctx, logger, query, count)
	if err != nil {
		return nil, fmt.Errorf("local search failed: %w", err)
	}

	// Check for local results first
	if response.Locations != nil && len(response.Locations.Results) > 0 {
		return t.processLocalResults(ctx, logger, query, count, response)
	}

	// Fallback to web search
	logger.WithField("query", query).Info("No location results found, falling back to web search")
	webResponse, err := t.client.WebSearch(ctx, logger, query, count, 0, "")
	if err != nil {
		return nil, fmt.Errorf("local search found no results and fallback web search failed: %w", err)
	}

	if webResponse.Web == nil || len(webResponse.Web.Results) == 0 {
		return t.createEmptyResponse(query)
	}

	// Convert web results with fallback indicator
	results := make([]SearchResult, 0, len(webResponse.Web.Results))
	for _, webResult := range webResponse.Web.Results {
		metadata := make(map[string]interface{})
		metadata["fallback"] = "web_search"
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

	return t.createSuccessResponse(query, results, logger)
}

// processLocalResults processes local search results with POI details
func (t *UnifiedBraveSearchTool) processLocalResults(ctx context.Context, logger *logrus.Logger, query string, count int, response *BraveLocalSearchResponse) (*mcp.CallToolResult, error) {
	// Extract location IDs for detailed information
	locationIDs := make([]string, 0, len(response.Locations.Results))
	for _, location := range response.Locations.Results {
		if location.ID != "" {
			locationIDs = append(locationIDs, location.ID)
		}
	}

	// Limit to requested count
	if len(locationIDs) > count {
		locationIDs = locationIDs[:count]
	}

	// Fetch POI details and descriptions in parallel
	var poiResponse *BraveLocalPOIResponse
	var descResponse *BraveLocalDescriptionsResponse
	var poiErr, descErr error

	done := make(chan bool, 2)

	go func() {
		poiResponse, poiErr = t.client.LocalPOISearch(ctx, logger, locationIDs)
		done <- true
	}()

	go func() {
		descResponse, descErr = t.client.LocalDescriptionsSearch(ctx, logger, locationIDs)
		done <- true
	}()

	// Wait for both requests to complete
	<-done
	<-done

	// Create maps for quick lookup
	poiMap := make(map[string]*BravePOIData)
	if poiErr == nil && poiResponse != nil {
		for i, poi := range poiResponse.Results {
			if i < len(locationIDs) {
				poi.ID = locationIDs[i]
				poiMap[locationIDs[i]] = &poi
			}
		}
	} else {
		logger.WithError(poiErr).Warn("Failed to fetch POI details")
	}

	descMap := make(map[string]string)
	if descErr == nil && descResponse != nil {
		for _, desc := range descResponse.Results {
			descMap[desc.ID] = desc.Description
		}
	} else {
		logger.WithError(descErr).Warn("Failed to fetch location descriptions")
	}

	// Convert results to unified format
	results := make([]SearchResult, 0, len(response.Locations.Results))
	for i, location := range response.Locations.Results {
		if i >= count {
			break
		}

		metadata := make(map[string]interface{})

		// Add POI data if available
		if poi, exists := poiMap[location.ID]; exists {
			if poi.Address != "" {
				metadata["address"] = poi.Address
			}
			if poi.PhoneNumber != "" {
				metadata["phone"] = poi.PhoneNumber
			}
			if poi.Rating > 0 {
				metadata["rating"] = poi.Rating
			}
			if poi.ReviewCount > 0 {
				metadata["reviewCount"] = poi.ReviewCount
			}
			if poi.Website != "" {
				metadata["website"] = poi.Website
			}
			if len(poi.Hours) > 0 {
				metadata["hours"] = poi.Hours
			}
		}

		// Add coordinates if available
		if len(location.Coordinates) >= 2 {
			metadata["coordinates"] = location.Coordinates
		}

		// Use description from descriptions API if available
		description := location.Description
		if desc, exists := descMap[location.ID]; exists && desc != "" {
			description = desc
		}

		results = append(results, SearchResult{
			Title:       decodeHTMLEntities(location.Title),
			URL:         location.URL,
			Description: decodeHTMLEntities(description),
			Type:        "local",
			Metadata:    metadata,
		})
	}

	return t.createSuccessResponse(query, results, logger)
}

// Helper functions
func (t *UnifiedBraveSearchTool) createEmptyResponse(query string) (*mcp.CallToolResult, error) {
	result := SearchResponse{
		Query:       query,
		ResultCount: 0,
		Results:     []SearchResult{},
		Provider:    "Brave",
		Timestamp:   time.Now(),
	}
	return newToolResultJSON(result)
}

func (t *UnifiedBraveSearchTool) createSuccessResponse(query string, results []SearchResult, logger *logrus.Logger) (*mcp.CallToolResult, error) {
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
	}).Info("Brave search completed successfully")

	return newToolResultJSON(result)
}

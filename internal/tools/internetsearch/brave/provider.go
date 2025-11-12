package brave

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sirupsen/logrus"
)

// BraveProvider implements the unified SearchProvider interface
type BraveProvider struct {
	client *BraveClient
}

// NewBraveProvider creates a new Brave search provider
func NewBraveProvider() *BraveProvider {
	apiKey := os.Getenv("BRAVE_API_KEY")
	if apiKey == "" {
		return nil
	}

	return &BraveProvider{
		client: NewBraveClient(apiKey),
	}
}

// GetName returns the provider name
func (p *BraveProvider) GetName() string {
	return "brave"
}

// IsAvailable checks if the provider is available
func (p *BraveProvider) IsAvailable() bool {
	return p.client != nil && os.Getenv("BRAVE_API_KEY") != ""
}

// GetSupportedTypes returns the search types this provider supports
func (p *BraveProvider) GetSupportedTypes() []string {
	return []string{"web", "image", "news", "video", "local"}
}

// Search executes a search using the Brave provider
func (p *BraveProvider) Search(ctx context.Context, logger *logrus.Logger, searchType string, args map[string]any) (*internetsearch.SearchResponse, error) {
	query := args["query"].(string)

	logger.WithFields(logrus.Fields{
		"provider": "brave",
		"type":     searchType,
		"query":    query,
	}).Debug("Brave search parameters")

	switch searchType {
	case "web":
		return p.executeInternetSearch(ctx, logger, args)
	case "image":
		return p.executeImageSearch(ctx, logger, args)
	case "news":
		return p.executeNewsSearch(ctx, logger, args)
	case "video":
		return p.executeVideoSearch(ctx, logger, args)
	case "local":
		return p.executeLocalSearch(ctx, logger, args)
	default:
		return nil, fmt.Errorf("unsupported search type for Brave: %s", searchType)
	}
}

// executeInternetSearch handles internet search for web results
func (p *BraveProvider) executeInternetSearch(ctx context.Context, logger *logrus.Logger, args map[string]any) (*internetsearch.SearchResponse, error) {
	query := args["query"].(string)

	// Parse optional parameters
	count := 10
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 20 {
			return nil, fmt.Errorf("count must be between 1 and 20 for internet search, got %d", count)
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

	response, err := p.client.InternetSearch(ctx, logger, query, count, offset, freshness)
	if err != nil {
		return nil, fmt.Errorf("internet search failed: %w", err)
	}

	// Convert to unified format
	if response.Web == nil || len(response.Web.Results) == 0 {
		return p.createEmptyResponse(query)
	}

	results := make([]internetsearch.SearchResult, 0, len(response.Web.Results))
	for _, webResult := range response.Web.Results {
		metadata := make(map[string]any)
		if webResult.Age != "" {
			metadata["age"] = webResult.Age
		}

		results = append(results, internetsearch.SearchResult{
			Title:       decodeHTMLEntities(webResult.Title),
			URL:         webResult.URL,
			Description: decodeHTMLEntities(webResult.Description),
			Type:        "web",
			Metadata:    metadata,
		})
	}

	return p.createSuccessResponse(query, results, logger)
}

// executeImageSearch handles image search
func (p *BraveProvider) executeImageSearch(ctx context.Context, logger *logrus.Logger, args map[string]any) (*internetsearch.SearchResponse, error) {
	query := args["query"].(string)

	// Parse optional parameters
	count := 1
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 3 {
			return nil, fmt.Errorf("count must be between 1 and 3 for image search, got %d", count)
		}
	}

	response, err := p.client.ImageSearch(ctx, logger, query, count)
	if err != nil {
		return nil, fmt.Errorf("image search failed: %w", err)
	}

	// Convert to unified format
	if len(response.Results) == 0 {
		return p.createEmptyResponse(query)
	}

	results := make([]internetsearch.SearchResult, 0, len(response.Results))
	for _, imageResult := range response.Results {
		metadata := make(map[string]any)
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

		results = append(results, internetsearch.SearchResult{
			Title:       decodeHTMLEntities(imageResult.Title),
			URL:         imageResult.URL,
			Description: fmt.Sprintf("Image: %s", decodeHTMLEntities(imageResult.Title)),
			Type:        "image",
			Metadata:    metadata,
		})
	}

	return p.createSuccessResponse(query, results, logger)
}

// executeNewsSearch handles news search
func (p *BraveProvider) executeNewsSearch(ctx context.Context, logger *logrus.Logger, args map[string]any) (*internetsearch.SearchResponse, error) {
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

	response, err := p.client.NewsSearch(ctx, logger, query, count, freshness)
	if err != nil {
		return nil, fmt.Errorf("news search failed: %w", err)
	}

	// Convert to unified format
	if len(response.Results) == 0 {
		return p.createEmptyResponse(query)
	}

	results := make([]internetsearch.SearchResult, 0, len(response.Results))
	for _, newsResult := range response.Results {
		metadata := make(map[string]any)
		if newsResult.Age != "" {
			metadata["age"] = newsResult.Age
		}

		results = append(results, internetsearch.SearchResult{
			Title:       decodeHTMLEntities(newsResult.Title),
			URL:         newsResult.URL,
			Description: decodeHTMLEntities(newsResult.Description),
			Type:        "news",
			Metadata:    metadata,
		})
	}

	return p.createSuccessResponse(query, results, logger)
}

// executeVideoSearch handles video search
func (p *BraveProvider) executeVideoSearch(ctx context.Context, logger *logrus.Logger, args map[string]any) (*internetsearch.SearchResponse, error) {
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

	response, err := p.client.VideoSearch(ctx, logger, query, count, freshness)
	if err != nil {
		return nil, fmt.Errorf("video search failed: %w", err)
	}

	// Convert to unified format
	if len(response.Results) == 0 {
		return p.createEmptyResponse(query)
	}

	results := make([]internetsearch.SearchResult, 0, len(response.Results))
	for _, videoResult := range response.Results {
		metadata := make(map[string]any)
		if videoResult.Video.Duration != "" {
			metadata["duration"] = videoResult.Video.Duration
		}
		if videoResult.Video.Views != nil {
			metadata["views"] = videoResult.Video.Views
		}
		if videoResult.Video.Creator != "" {
			metadata["creator"] = videoResult.Video.Creator
		}

		results = append(results, internetsearch.SearchResult{
			Title:       decodeHTMLEntities(videoResult.Title),
			URL:         videoResult.URL,
			Description: fmt.Sprintf("Video: %s", decodeHTMLEntities(videoResult.Title)),
			Type:        "video",
			Metadata:    metadata,
		})
	}

	return p.createSuccessResponse(query, results, logger)
}

// executeLocalSearch handles local search with web fallback
func (p *BraveProvider) executeLocalSearch(ctx context.Context, logger *logrus.Logger, args map[string]any) (*internetsearch.SearchResponse, error) {
	query := args["query"].(string)

	// Parse optional parameters
	count := 10
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 20 {
			return nil, fmt.Errorf("count must be between 1 and 20 for local search, got %d", count)
		}
	}

	response, err := p.client.LocalSearch(ctx, logger, query, count)
	if err != nil {
		return nil, fmt.Errorf("local search failed: %w", err)
	}

	// Check for local results first
	if response.Locations != nil && len(response.Locations.Results) > 0 {
		return p.processLocalResults(ctx, logger, query, count, response)
	}

	// Fallback to internet search
	logger.WithField("query", query).Info("No location results found, falling back to internet search")
	webResponse, err := p.client.InternetSearch(ctx, logger, query, count, 0, "")
	if err != nil {
		return nil, fmt.Errorf("local search found no results and fallback internet search failed: %w", err)
	}

	if webResponse.Web == nil || len(webResponse.Web.Results) == 0 {
		return p.createEmptyResponse(query)
	}

	// Convert web results with fallback indicator
	results := make([]internetsearch.SearchResult, 0, len(webResponse.Web.Results))
	for _, webResult := range webResponse.Web.Results {
		metadata := make(map[string]any)
		metadata["fallback"] = "internet_search_fallback"
		if webResult.Age != "" {
			metadata["age"] = webResult.Age
		}

		results = append(results, internetsearch.SearchResult{
			Title:       decodeHTMLEntities(webResult.Title),
			URL:         webResult.URL,
			Description: decodeHTMLEntities(webResult.Description),
			Type:        "web",
			Metadata:    metadata,
		})
	}

	return p.createSuccessResponse(query, results, logger)
}

// processLocalResults processes local search results with POI details
func (p *BraveProvider) processLocalResults(ctx context.Context, logger *logrus.Logger, query string, count int, response *BraveLocalSearchResponse) (*internetsearch.SearchResponse, error) {
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
		poiResponse, poiErr = p.client.LocalPOISearch(ctx, logger, locationIDs)
		done <- true
	}()

	go func() {
		descResponse, descErr = p.client.LocalDescriptionsSearch(ctx, logger, locationIDs)
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
	results := make([]internetsearch.SearchResult, 0, len(response.Locations.Results))
	for i, location := range response.Locations.Results {
		if i >= count {
			break
		}

		metadata := make(map[string]any)

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

		results = append(results, internetsearch.SearchResult{
			Title:       decodeHTMLEntities(location.Title),
			URL:         location.URL,
			Description: decodeHTMLEntities(description),
			Type:        "local",
			Metadata:    metadata,
		})
	}

	return p.createSuccessResponse(query, results, logger)
}

// Helper functions
func (p *BraveProvider) createEmptyResponse(query string) (*internetsearch.SearchResponse, error) {
	result := &internetsearch.SearchResponse{
		Query:       query,
		ResultCount: 0,
		Results:     []internetsearch.SearchResult{},
		Provider:    "brave",
		Timestamp:   time.Now(),
	}
	return result, nil
}

func (p *BraveProvider) createSuccessResponse(query string, results []internetsearch.SearchResult, logger *logrus.Logger) (*internetsearch.SearchResponse, error) {
	result := &internetsearch.SearchResponse{
		Query:       query,
		ResultCount: len(results),
		Results:     results,
		Provider:    "brave",
		Timestamp:   time.Now(),
	}

	logger.WithFields(logrus.Fields{
		"query":        query,
		"result_count": len(results),
	}).Info("Brave search completed successfully")

	return result, nil
}

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

// BraveLocalSearchTool implements local search using Brave Search API
type BraveLocalSearchTool struct {
	client *BraveClient
}

// init registers the local search tool with conditional registration
func init() {
	apiKey := os.Getenv("BRAVE_API_KEY")
	if apiKey == "" {
		// Tool will not be registered - this is expected behaviour
		return
	}

	registry.Register(&BraveLocalSearchTool{
		client: NewBraveClient(apiKey),
	})
}

// Definition returns the tool's definition for MCP registration
func (t *BraveLocalSearchTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"brave_local_search",
		mcp.WithDescription("Searches for local businesses and places using Brave's Local Search API. Best for queries related to physical locations, businesses, restaurants, services, etc. Returns detailed information including:\n- Business names and addresses\n- Ratings and review counts\n- Phone numbers and opening hours\nUse this when the query implies 'near me' or mentions specific locations. Automatically falls back to web search if no local results are found."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Local search query (e.g. 'pizza near Central Park')"),
		),
		mcp.WithNumber("count",
			mcp.Description("The number of results to return, minimum 1, maximum 20"),
			mcp.DefaultNumber(10),
		),
	)
}

// Execute executes the local search tool
func (t *BraveLocalSearchTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Executing Brave local search")

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

	logger.WithFields(logrus.Fields{
		"query": query,
		"count": count,
	}).Debug("Brave local search parameters")

	// Perform the local search
	response, err := t.client.LocalSearch(logger, query, count)
	if err != nil {
		logger.WithError(err).Error("Brave local search failed")
		return nil, fmt.Errorf("local search failed: %w", err)
	}

	// Check if we have location results
	if response.Locations == nil || len(response.Locations.Results) == 0 {
		logger.WithField("query", query).Info("No location results found, falling back to web search")

		// Fallback to web search
		webResponse, err := t.client.WebSearch(logger, query, count, 0, "")
		if err != nil {
			logger.WithError(err).Error("Fallback web search failed")
			return nil, fmt.Errorf("local search found no results and fallback web search failed: %w", err)
		}

		// Check if web search has results
		if webResponse.Web == nil || len(webResponse.Web.Results) == 0 {
			logger.WithField("query", query).Info("No results found in local or web search")
			result := SearchResponse{
				Query:       query,
				ResultCount: 0,
				Results:     []SearchResult{},
				Provider:    "Brave",
				Timestamp:   time.Now(),
			}
			return newToolResultJSON(result)
		}

		// Convert web results to unified format
		results := make([]SearchResult, 0, len(webResponse.Web.Results))
		for _, webResult := range webResponse.Web.Results {
			metadata := make(map[string]interface{})
			metadata["fallback"] = "web_search"
			if webResult.Age != "" {
				metadata["age"] = webResult.Age
			}

			results = append(results, SearchResult{
				Title:       webResult.Title,
				URL:         webResult.URL,
				Description: webResult.Description,
				Type:        "web",
				Metadata:    metadata,
			})
		}

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
		}).Info("Brave local search completed with web search fallback")

		return newToolResultJSON(result)
	}

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

	logger.WithFields(logrus.Fields{
		"query":        query,
		"location_ids": len(locationIDs),
	}).Debug("Fetching detailed location information")

	// Fetch POI details and descriptions in parallel
	var poiResponse *BraveLocalPOIResponse
	var descResponse *BraveLocalDescriptionsResponse
	var poiErr, descErr error

	// Use goroutines for parallel requests
	done := make(chan bool, 2)

	go func() {
		poiResponse, poiErr = t.client.LocalPOISearch(logger, locationIDs)
		done <- true
	}()

	go func() {
		descResponse, descErr = t.client.LocalDescriptionsSearch(logger, locationIDs)
		done <- true
	}()

	// Wait for both requests to complete
	<-done
	<-done

	// Create maps for quick lookup
	poiMap := make(map[string]*BravePOIData)
	if poiErr == nil && poiResponse != nil {
		for i, poi := range poiResponse.Results {
			// Assign ID based on order (as mentioned in reference implementation)
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

		// Use description from descriptions API if available, otherwise use location description
		description := location.Description
		if desc, exists := descMap[location.ID]; exists && desc != "" {
			description = desc
		}

		results = append(results, SearchResult{
			Title:       location.Title,
			URL:         location.URL,
			Description: description,
			Type:        "local",
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
	}).Info("Brave local search completed successfully")

	return newToolResultJSON(result)
}

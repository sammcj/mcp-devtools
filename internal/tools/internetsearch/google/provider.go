package google

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sirupsen/logrus"
)

// GoogleProvider implements the unified SearchProvider interface
type GoogleProvider struct {
	client *GoogleClient
}

// NewGoogleProvider creates a new Google Custom Search provider
func NewGoogleProvider() *GoogleProvider {
	apiKey := os.Getenv("GOOGLE_SEARCH_API_KEY")
	searchID := os.Getenv("GOOGLE_SEARCH_ID")

	if apiKey == "" || searchID == "" {
		return nil
	}

	return &GoogleProvider{
		client: NewGoogleClient(apiKey, searchID),
	}
}

// GetName returns the provider name
func (p *GoogleProvider) GetName() string {
	return "google"
}

// IsAvailable checks if the provider is available
func (p *GoogleProvider) IsAvailable() bool {
	return p.client != nil && os.Getenv("GOOGLE_SEARCH_API_KEY") != "" && os.Getenv("GOOGLE_SEARCH_ID") != ""
}

// GetSupportedTypes returns the search types this provider supports
func (p *GoogleProvider) GetSupportedTypes() []string {
	return []string{"web", "image"}
}

// extractQuery safely extracts and validates the query parameter
func (p *GoogleProvider) extractQuery(args map[string]any) (string, error) {
	queryVal, exists := args["query"]
	if !exists {
		return "", fmt.Errorf("missing required parameter: query")
	}

	query, ok := queryVal.(string)
	if !ok {
		return "", fmt.Errorf("invalid query parameter: expected string, got %T", queryVal)
	}

	if query == "" {
		return "", fmt.Errorf("query parameter cannot be empty")
	}

	return query, nil
}

// Search executes a search using the Google provider
func (p *GoogleProvider) Search(ctx context.Context, logger *logrus.Logger, searchType string, args map[string]any) (*internetsearch.SearchResponse, error) {
	query, err := p.extractQuery(args)
	if err != nil {
		return nil, err
	}

	logger.WithFields(logrus.Fields{
		"provider": "google",
		"type":     searchType,
		"query":    query,
	}).Debug("Google search parameters")

	switch searchType {
	case "web":
		return p.executeWebSearch(ctx, logger, args)
	case "image":
		return p.executeImageSearch(ctx, logger, args)
	default:
		return nil, fmt.Errorf("unsupported search type for Google: %s", searchType)
	}
}

// executeWebSearch handles web search
func (p *GoogleProvider) executeWebSearch(ctx context.Context, logger *logrus.Logger, args map[string]any) (*internetsearch.SearchResponse, error) {
	query, err := p.extractQuery(args)
	if err != nil {
		return nil, err
	}

	// Parse optional parameters
	count := 10
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 10 {
			return nil, fmt.Errorf("count must be between 1 and 10 for Google search, got %d", count)
		}
	}

	start := 0
	if startRaw, ok := args["start"].(float64); ok {
		start = int(startRaw)
		if start < 0 {
			return nil, fmt.Errorf("start must be >= 0, got %d", start)
		}
	}

	response, err := p.client.Search(ctx, logger, query, "web", count, start)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	// Convert to unified format
	if len(response.Items) == 0 {
		return p.createEmptyResponse(query)
	}

	results := make([]internetsearch.SearchResult, 0, len(response.Items))
	for _, item := range response.Items {
		metadata := make(map[string]any)
		if item.DisplayLink != "" {
			metadata["displayLink"] = item.DisplayLink
		}

		results = append(results, internetsearch.SearchResult{
			Title:       item.Title,
			URL:         item.Link,
			Description: item.Snippet,
			Type:        "web",
			Metadata:    metadata,
		})
	}

	return p.createSuccessResponse(query, results, logger)
}

// executeImageSearch handles image search
func (p *GoogleProvider) executeImageSearch(ctx context.Context, logger *logrus.Logger, args map[string]any) (*internetsearch.SearchResponse, error) {
	query, err := p.extractQuery(args)
	if err != nil {
		return nil, err
	}

	// Parse optional parameters
	count := 10
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 10 {
			return nil, fmt.Errorf("count must be between 1 and 10 for Google image search, got %d", count)
		}
	}

	start := 0
	if startRaw, ok := args["start"].(float64); ok {
		start = int(startRaw)
		if start < 0 {
			return nil, fmt.Errorf("start must be >= 0, got %d", start)
		}
	}

	response, err := p.client.Search(ctx, logger, query, "image", count, start)
	if err != nil {
		return nil, fmt.Errorf("image search failed: %w", err)
	}

	// Convert to unified format
	if len(response.Items) == 0 {
		return p.createEmptyResponse(query)
	}

	results := make([]internetsearch.SearchResult, 0, len(response.Items))
	for _, item := range response.Items {
		metadata := make(map[string]any)

		// Add image-specific metadata
		if item.Image != nil {
			if item.Image.ContextLink != "" {
				metadata["imageURL"] = item.Image.ContextLink
			}
			if item.Image.Height > 0 {
				metadata["height"] = item.Image.Height
			}
			if item.Image.Width > 0 {
				metadata["width"] = item.Image.Width
			}
			if item.Image.ThumbnailLink != "" {
				metadata["thumbnailURL"] = item.Image.ThumbnailLink
			}
		}

		// Use snippet if available, otherwise fall back to title-based description
		description := item.Snippet
		if description == "" {
			description = fmt.Sprintf("Image: %s", item.Title)
		}

		results = append(results, internetsearch.SearchResult{
			Title:       item.Title,
			URL:         item.Link,
			Description: description,
			Type:        "image",
			Metadata:    metadata,
		})
	}

	return p.createSuccessResponse(query, results, logger)
}

// Helper functions
func (p *GoogleProvider) createEmptyResponse(query string) (*internetsearch.SearchResponse, error) {
	result := &internetsearch.SearchResponse{
		Query:       query,
		ResultCount: 0,
		Results:     []internetsearch.SearchResult{},
		Provider:    "google",
		Timestamp:   time.Now(),
	}
	return result, nil
}

func (p *GoogleProvider) createSuccessResponse(query string, results []internetsearch.SearchResult, logger *logrus.Logger) (*internetsearch.SearchResponse, error) {
	result := &internetsearch.SearchResponse{
		Query:       query,
		ResultCount: len(results),
		Results:     results,
		Provider:    "google",
		Timestamp:   time.Now(),
	}

	logger.WithFields(logrus.Fields{
		"query":        query,
		"result_count": len(results),
		"provider":     "google",
	}).Info("Google search completed successfully")

	return result, nil
}

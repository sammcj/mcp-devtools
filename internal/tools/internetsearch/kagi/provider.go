package kagi

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sirupsen/logrus"
)

// KagiProvider implements the unified SearchProvider interface
type KagiProvider struct {
	client *KagiClient
}

// NewKagiProvider creates a new Kagi search provider
func NewKagiProvider() *KagiProvider {
	apiKey := os.Getenv("KAGI_API_KEY")
	if apiKey == "" {
		return nil
	}

	return &KagiProvider{
		client: NewKagiClient(apiKey),
	}
}

// GetName returns the provider name
func (p *KagiProvider) GetName() string {
	return "kagi"
}

// IsAvailable checks if the provider is available
func (p *KagiProvider) IsAvailable() bool {
	return p.client != nil && os.Getenv("KAGI_API_KEY") != ""
}

// GetSupportedTypes returns the search types this provider supports
func (p *KagiProvider) GetSupportedTypes() []string {
	// Kagi currently only supports web search
	return []string{"web"}
}

// Search executes a search using the Kagi provider
func (p *KagiProvider) Search(ctx context.Context, logger *logrus.Logger, searchType string, args map[string]any) (*internetsearch.SearchResponse, error) {
	query := args["query"].(string)

	logger.WithFields(logrus.Fields{
		"provider": "kagi",
		"type":     searchType,
		"query":    query,
	}).Debug("Kagi search parameters")

	switch searchType {
	case "web":
		return p.executeWebSearch(ctx, logger, args)
	default:
		return nil, fmt.Errorf("unsupported search type for Kagi: %s", searchType)
	}
}

// executeWebSearch handles web search for search results
func (p *KagiProvider) executeWebSearch(ctx context.Context, logger *logrus.Logger, args map[string]any) (*internetsearch.SearchResponse, error) {
	query := args["query"].(string)

	// Parse optional parameters
	limit := 10
	if countRaw, ok := args["count"].(float64); ok {
		limit = int(countRaw)
		if limit < 1 || limit > 25 {
			return nil, fmt.Errorf("count must be between 1 and 25 for Kagi search, got %d", limit)
		}
	}

	response, err := p.client.Search(ctx, logger, query, limit)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	// Convert to unified format
	if len(response.Data) == 0 {
		return p.createEmptyResponse()
	}

	results := make([]internetsearch.SearchResult, 0, len(response.Data))
	for _, kagiResult := range response.Data {
		// Only include actual search results (t=0), not related searches (t=1)
		if kagiResult.T != 0 {
			continue
		}

		metadata := make(map[string]any)
		metadata["rank"] = kagiResult.Rank

		if kagiResult.Published != "" {
			metadata["published"] = kagiResult.Published
		}

		if kagiResult.Thumbnail != nil {
			thumbnailInfo := make(map[string]any)
			thumbnailInfo["url"] = kagiResult.Thumbnail.URL
			if kagiResult.Thumbnail.Width > 0 {
				thumbnailInfo["width"] = kagiResult.Thumbnail.Width
			}
			if kagiResult.Thumbnail.Height > 0 {
				thumbnailInfo["height"] = kagiResult.Thumbnail.Height
			}
			metadata["thumbnail"] = thumbnailInfo
		}

		results = append(results, internetsearch.SearchResult{
			Title:       decodeHTMLEntities(kagiResult.Title),
			URL:         kagiResult.URL,
			Description: decodeHTMLEntities(kagiResult.Snippet),
			Metadata:    metadata,
		})
	}

	return p.createSuccessResponse(query, results, logger)
}

// Helper functions
func (p *KagiProvider) createEmptyResponse() (*internetsearch.SearchResponse, error) {
	result := &internetsearch.SearchResponse{
		Results:   []internetsearch.SearchResult{},
		Provider:  "kagi",
		Timestamp: time.Now(),
	}
	return result, nil
}

func (p *KagiProvider) createSuccessResponse(query string, results []internetsearch.SearchResult, logger *logrus.Logger) (*internetsearch.SearchResponse, error) {
	result := &internetsearch.SearchResponse{
		Results:   results,
		Provider:  "kagi",
		Timestamp: time.Now(),
	}

	logger.WithFields(logrus.Fields{
		"query":        query,
		"result_count": len(results),
	}).Info("Kagi search completed successfully")

	return result, nil
}

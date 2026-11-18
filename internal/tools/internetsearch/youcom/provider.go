// Package youcom implements the You.com internet search provider.
//
// The provider is opt-in. It is registered when YDC_API_KEY is set (keyed Web
// Search API at ydc-index.io) or when YOUCOM_SEARCH_KEYLESS=true is set (the
// free keyless tier at api.you.com, about 100 searches a day, no account).
package youcom

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sirupsen/logrus"
)

const (
	// APIKeyEnvVar holds the optional You.com API key
	APIKeyEnvVar = "YDC_API_KEY"
	// KeylessEnvVar opts in to the free keyless tier when no API key is set
	KeylessEnvVar = "YOUCOM_SEARCH_KEYLESS"
)

// YouComProvider implements the unified SearchProvider interface
type YouComProvider struct {
	client *YouComClient
}

// NewYouComProvider creates a new You.com search provider, or returns nil when
// the provider has not been enabled (no YDC_API_KEY and no keyless opt-in).
func NewYouComProvider() *YouComProvider {
	apiKey := strings.TrimSpace(os.Getenv(APIKeyEnvVar))
	if apiKey == "" && !keylessEnabled() {
		return nil
	}

	return &YouComProvider{
		client: NewYouComClient(apiKey),
	}
}

func keylessEnabled() bool {
	enabled, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(KeylessEnvVar)))
	return err == nil && enabled
}

// GetName returns the provider name
func (p *YouComProvider) GetName() string {
	return "youcom"
}

// IsAvailable checks if the provider is available
func (p *YouComProvider) IsAvailable() bool {
	return p.client != nil && (strings.TrimSpace(os.Getenv(APIKeyEnvVar)) != "" || keylessEnabled())
}

// GetSupportedTypes returns the search types this provider supports
func (p *YouComProvider) GetSupportedTypes() []string {
	// News items, when the API returns them, are merged into web results
	return []string{"web"}
}

// Search executes a search using the You.com provider
func (p *YouComProvider) Search(ctx context.Context, logger *logrus.Logger, searchType string, args map[string]any) (*internetsearch.SearchResponse, error) {
	query := args["query"].(string)

	logger.WithFields(logrus.Fields{
		"provider": "youcom",
		"type":     searchType,
		"query":    query,
		"keyed":    p.client.apiKey != "",
	}).Debug("You.com search parameters")

	switch searchType {
	case "web":
		return p.executeSearch(ctx, logger, searchType, args)
	default:
		return nil, fmt.Errorf("unsupported search type for You.com: %s", searchType)
	}
}

func (p *YouComProvider) executeSearch(ctx context.Context, logger *logrus.Logger, searchType string, args map[string]any) (*internetsearch.SearchResponse, error) {
	query := args["query"].(string)

	count := 10
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > MaxKeylessCount {
			return nil, fmt.Errorf("count must be between 1 and %d for You.com search, got %d", MaxKeylessCount, count)
		}
	}

	response, err := p.client.Search(ctx, logger, query, count)
	if err != nil {
		return nil, fmt.Errorf("%s search failed: %w", searchType, err)
	}

	items := append(response.Results.Web, response.Results.News...)

	results := make([]internetsearch.SearchResult, 0, len(items))
	for _, item := range items {
		if item.URL == "" {
			continue
		}
		description := item.Description
		if description == "" && len(item.Snippets) > 0 {
			description = strings.Join(item.Snippets, " ")
		}
		metadata := map[string]any{}
		if item.PageAge != "" {
			metadata["published"] = item.PageAge
		}
		if len(item.Snippets) > 0 {
			metadata["snippets"] = item.Snippets
		}
		results = append(results, internetsearch.SearchResult{
			Title:       item.Title,
			URL:         item.URL,
			Description: description,
			Metadata:    metadata,
		})
		if len(results) == count {
			break
		}
	}

	logger.WithFields(logrus.Fields{
		"query":        query,
		"result_count": len(results),
	}).Info("You.com search completed successfully")

	return &internetsearch.SearchResponse{
		Results:   results,
		Provider:  "youcom",
		Timestamp: time.Now(),
	}, nil
}

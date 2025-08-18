package searxng

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sirupsen/logrus"
)

// SearXNGProvider implements the unified SearchProvider interface
type SearXNGProvider struct {
	baseURL  string
	username string
	password string
	client   internetsearch.HTTPClientInterface
}

// SearXNGResponse represents the response from SearXNG API
type SearXNGResponse struct {
	Results []SearXNGResult `json:"results"`
}

// SearXNGResult represents a single search result from SearXNG
type SearXNGResult struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	URL     string `json:"url"`
}

// NewSearXNGProvider creates a new SearXNG search provider
func NewSearXNGProvider() *SearXNGProvider {
	baseURL := os.Getenv("SEARXNG_BASE_URL")
	if baseURL == "" {
		return nil
	}

	// Validate URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil
	}

	return &SearXNGProvider{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		username: os.Getenv("SEARXNG_USERNAME"),
		password: os.Getenv("SEARXNG_PASSWORD"),
		client:   internetsearch.NewRateLimitedHTTPClient(),
	}
}

// GetName returns the provider name
func (p *SearXNGProvider) GetName() string {
	return "searxng"
}

// IsAvailable checks if the provider is available
func (p *SearXNGProvider) IsAvailable() bool {
	return p.baseURL != ""
}

// GetSupportedTypes returns the search types this provider supports
func (p *SearXNGProvider) GetSupportedTypes() []string {
	// SearXNG primarily supports web search, but we'll map all types to web search
	return []string{"web", "image", "news", "video"}
}

// Search executes a search using the SearXNG provider
func (p *SearXNGProvider) Search(ctx context.Context, logger *logrus.Logger, searchType string, args map[string]interface{}) (*internetsearch.SearchResponse, error) {
	query := args["query"].(string)

	logger.WithFields(logrus.Fields{
		"provider": "searxng",
		"type":     searchType,
		"query":    query,
		"baseURL":  p.baseURL,
	}).Debug("SearXNG search parameters")

	// For SearXNG, all search types are handled as web search with different categories
	return p.executeSearch(ctx, logger, searchType, args)
}

// executeSearch handles the actual search execution
func (p *SearXNGProvider) executeSearch(ctx context.Context, logger *logrus.Logger, searchType string, args map[string]interface{}) (*internetsearch.SearchResponse, error) {
	query := args["query"].(string)

	// Parse SearXNG-specific parameters
	pageno := 1
	if pagenoRaw, ok := args["pageno"].(float64); ok {
		pageno = int(pagenoRaw)
		if pageno < 1 {
			pageno = 1
		}
	}

	timeRange := ""
	if timeRangeRaw, ok := args["time_range"].(string); ok {
		if timeRangeRaw == "day" || timeRangeRaw == "month" || timeRangeRaw == "year" {
			timeRange = timeRangeRaw
		}
	}

	language := "all"
	if languageRaw, ok := args["language"].(string); ok && languageRaw != "" {
		language = languageRaw
	}

	safesearch := "0"
	if safesearchRaw, ok := args["safesearch"].(string); ok {
		if safesearchRaw == "0" || safesearchRaw == "1" || safesearchRaw == "2" {
			safesearch = safesearchRaw
		}
	}

	// Build search URL
	searchURL, err := url.Parse(p.baseURL + "/search")
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Add query parameters
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("pageno", fmt.Sprintf("%d", pageno))

	// Add category based on search type
	switch searchType {
	case "image":
		params.Set("categories", "images")
	case "news":
		params.Set("categories", "news")
	case "video":
		params.Set("categories", "videos")
	default:
		params.Set("categories", "general")
	}

	if timeRange != "" {
		params.Set("time_range", timeRange)
	}

	if language != "all" {
		params.Set("language", language)
	}

	params.Set("safesearch", safesearch)

	searchURL.RawQuery = params.Encode()

	// Check domain access security before making request
	if err := security.CheckDomainAccess(searchURL.Hostname()); err != nil {
		if secErr, ok := err.(*security.SecurityError); ok {
			return nil, fmt.Errorf("security block [ID: %s]: %s Check with the user if you may use security_override tool with ID %s",
				secErr.GetSecurityID(), secErr.Error(), secErr.GetSecurityID())
		}
		return nil, err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add basic authentication if credentials are provided
	if p.username != "" && p.password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(p.username + ":" + p.password))
		req.Header.Set("Authorization", "Basic "+auth)
	}

	// Add user agent
	req.Header.Set("User-Agent", "MCP-DevTools/1.0")

	// Execute request using rate-limited client
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.WithError(err).Warn("Failed to close response body")
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Security analysis on content
	if security.IsEnabled() {
		sourceCtx := security.SourceContext{
			URL:         searchURL.String(),
			Domain:      searchURL.Hostname(),
			ContentType: resp.Header.Get("Content-Type"),
			Tool:        "internetsearch",
		}

		if secResult, err := security.AnalyseContent(string(body), sourceCtx); err == nil {
			switch secResult.Action {
			case security.ActionBlock:
				return nil, fmt.Errorf("security block [ID: %s]: %s Check with the user if you may use security_override tool with ID %s",
					secResult.ID, secResult.Message, secResult.ID)
			case security.ActionWarn:
				logger.WithField("security_id", secResult.ID).Warn(secResult.Message)
			}
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SearXNG API error: %d %s", resp.StatusCode, resp.Status)
	}

	// Parse response
	var searxngResp SearXNGResponse
	if err := json.Unmarshal(body, &searxngResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to unified format
	if len(searxngResp.Results) == 0 {
		return p.createEmptyResponse(query)
	}

	results := make([]internetsearch.SearchResult, 0, len(searxngResp.Results))
	for _, searxngResult := range searxngResp.Results {
		metadata := make(map[string]interface{})
		metadata["category"] = searchType
		if language != "all" {
			metadata["language"] = language
		}
		if timeRange != "" {
			metadata["time_range"] = timeRange
		}

		results = append(results, internetsearch.SearchResult{
			Title:       searxngResult.Title,
			URL:         searxngResult.URL,
			Description: searxngResult.Content,
			Type:        searchType,
			Metadata:    metadata,
		})
	}

	return p.createSuccessResponse(query, results, logger)
}

// Helper functions
func (p *SearXNGProvider) createEmptyResponse(query string) (*internetsearch.SearchResponse, error) {
	result := &internetsearch.SearchResponse{
		Query:       query,
		ResultCount: 0,
		Results:     []internetsearch.SearchResult{},
		Provider:    "searxng",
		Timestamp:   time.Now(),
	}
	return result, nil
}

func (p *SearXNGProvider) createSuccessResponse(query string, results []internetsearch.SearchResult, logger *logrus.Logger) (*internetsearch.SearchResponse, error) {
	result := &internetsearch.SearchResponse{
		Query:       query,
		ResultCount: len(results),
		Results:     results,
		Provider:    "searxng",
		Timestamp:   time.Now(),
	}

	logger.WithFields(logrus.Fields{
		"query":        query,
		"result_count": len(results),
		"provider":     "searxng",
	}).Info("SearXNG search completed successfully")

	return result, nil
}

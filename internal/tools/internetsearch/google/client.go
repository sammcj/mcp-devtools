package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sirupsen/logrus"
)

const (
	googleSearchAPIURL = "https://www.googleapis.com/customsearch/v1"
	UserAgent          = "MCP-DevTools/1.0"
)

// GoogleClient handles communication with Google Custom Search API
type GoogleClient struct {
	apiKey string
	cx     string // Custom Search Engine ID
	client internetsearch.HTTPClientInterface
}

// NewGoogleClient creates a new Google Custom Search API client
func NewGoogleClient(apiKey, cx string) *GoogleClient {
	return &GoogleClient{
		apiKey: apiKey,
		cx:     cx,
		client: internetsearch.NewRateLimitedHTTPClient(),
	}
}

// Search performs a search query
func (c *GoogleClient) Search(ctx context.Context, logger *logrus.Logger, query string, searchType string, count int, start int) (*GoogleSearchResponse, error) {
	// Build query parameters
	params := url.Values{}
	params.Set("key", c.apiKey)
	params.Set("cx", c.cx)
	params.Set("q", query)
	params.Set("num", fmt.Sprintf("%d", count))

	if start > 0 {
		params.Set("start", fmt.Sprintf("%d", start))
	}

	// Set search type
	if searchType == "image" {
		params.Set("searchType", "image")
	}

	requestURL := fmt.Sprintf("%s?%s", googleSearchAPIURL, params.Encode())

	// Security check: verify domain access
	if err := security.CheckDomainAccess("www.googleapis.com"); err != nil {
		return nil, err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")

	// Execute request with rate limiting
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Security analysis: check response content for threats
	if security.IsEnabled() {
		source := security.SourceContext{
			Tool:        "internet_search",
			Domain:      "www.googleapis.com",
			ContentType: "application/json",
			URL:         requestURL,
		}
		if secResult, err := security.AnalyseContent(string(body), source); err == nil {
			switch secResult.Action {
			case security.ActionBlock:
				return nil, security.FormatSecurityBlockError(&security.SecurityError{
					ID:      secResult.ID,
					Message: secResult.Message,
					Action:  security.ActionBlock,
				})
			case security.ActionWarn:
				logger.Warnf("Security warning [ID: %s]: %s", secResult.ID, secResult.Message)
			}
		}
	}

	// Parse JSON response
	var result GoogleSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"query":        query,
		"search_type":  searchType,
		"result_count": len(result.Items),
	}).Debug("Google search completed successfully")

	return &result, nil
}

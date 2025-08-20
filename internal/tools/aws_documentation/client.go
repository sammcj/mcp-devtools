package aws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sirupsen/logrus"
)

const (
	searchAPIURL          = "https://proxy.search.docs.aws.amazon.com/search"
	recommendationsAPIURL = "https://contentrecs-api.docs.aws.amazon.com/v1/recommendations"
	userAgent             = "Mozilla/5.0 (compatible; MCP-DevTools/1.0; AWS Documentation Client)"
	requestTimeout        = 30 * time.Second
)

// Client handles HTTP operations for AWS documentation API
type Client struct {
	sessionUUID string
	httpClient  *http.Client
	logger      *logrus.Logger
	ops         *security.Operations
}

// NewClient creates a new AWS documentation API client
func NewClient(logger *logrus.Logger) *Client {
	return &Client{
		sessionUUID: uuid.New().String(),
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
		logger: logger,
		ops:    security.NewOperations("aws"),
	}
}

// FetchDocumentation fetches AWS documentation content from a URL
func (c *Client) FetchDocumentation(url string) (string, error) {
	c.logger.WithField("url", url).Debug("Fetching AWS documentation")

	// Add session parameter to URL
	urlWithSession := fmt.Sprintf("%s?session=%s", url, c.sessionUUID)

	// Use security helper for HTTP request
	resp, err := c.ops.SafeHTTPGet(urlWithSession)
	if err != nil {
		if secErr, ok := err.(*security.SecurityError); ok {
			return "", fmt.Errorf("security block [ID: %s]: %s Check with the user if you may use security_override tool with ID %s",
				secErr.GetSecurityID(), secErr.Error(), secErr.GetSecurityID())
		}
		return "", fmt.Errorf("failed to fetch documentation: %w", err)
	}

	// Handle security warnings
	if resp.SecurityResult != nil && resp.SecurityResult.Action == security.ActionWarn {
		c.logger.WithField("security_id", resp.SecurityResult.ID).Warn(resp.SecurityResult.Message)
	}

	return string(resp.Content), nil
}

// SearchDocumentation searches AWS documentation using the search API
func (c *Client) SearchDocumentation(query string, limit int) ([]SearchResult, error) {
	c.logger.WithFields(logrus.Fields{
		"query": query,
		"limit": limit,
	}).Debug("Searching AWS documentation")

	// Prepare request body
	reqBody := SearchAPIRequest{
		TextQuery: struct {
			Input string `json:"input"`
		}{
			Input: query,
		},
		ContextAttributes: []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}{
			{Key: "domain", Value: "docs.aws.amazon.com"},
		},
		AcceptSuggestionBody: "RawText",
		Locales:              []string{"en_us"},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Add session parameter to URL
	searchURL := fmt.Sprintf("%s?session=%s", searchAPIURL, c.sessionUUID)

	// Check domain access
	if err := security.CheckDomainAccess("proxy.search.docs.aws.amazon.com"); err != nil {
		return nil, err
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", searchURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-MCP-Session-Id", c.sessionUUID)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error as we're already handling other errors
	}()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("search API returned error: status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Analyse content for security
	source := security.SourceContext{
		Tool:   "aws",
		Domain: "proxy.search.docs.aws.amazon.com",
	}

	if result, err := security.AnalyseContent(string(body), source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, fmt.Errorf("search response blocked by security policy: %s", result.Message)
		case security.ActionWarn:
			c.logger.WithField("security_id", result.ID).Warn(result.Message)
		}
	}

	// Parse response
	var apiResp SearchAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	// Convert to SearchResult format
	results := make([]SearchResult, 0, len(apiResp.Suggestions))
	for i, suggestion := range apiResp.Suggestions {
		if i >= limit {
			break
		}

		textSuggestion := suggestion.TextExcerptSuggestion
		var context *string

		// Add context if available
		if textSuggestion.Summary != "" {
			context = &textSuggestion.Summary
		} else if textSuggestion.SuggestionBody != "" {
			context = &textSuggestion.SuggestionBody
		}

		results = append(results, SearchResult{
			RankOrder: i + 1,
			URL:       textSuggestion.Link,
			Title:     textSuggestion.Title,
			Context:   context,
		})
	}

	c.logger.WithField("results_count", len(results)).Debug("Search completed successfully")
	return results, nil
}

// GetRecommendations gets content recommendations for an AWS documentation URL
func (c *Client) GetRecommendations(url string) ([]RecommendationResult, error) {
	c.logger.WithField("url", url).Debug("Getting recommendations for AWS documentation")

	// Build recommendation URL
	recommendURL := fmt.Sprintf("%s?path=%s&session=%s", recommendationsAPIURL, url, c.sessionUUID)

	// Check domain access
	if err := security.CheckDomainAccess("contentrecs-api.docs.aws.amazon.com"); err != nil {
		return nil, err
	}

	// Create HTTP request
	req, err := http.NewRequest("GET", recommendURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("recommendations request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close() // Ignore close error as we're already handling other errors
	}()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("recommendations API returned error: status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Analyse content for security
	source := security.SourceContext{
		Tool:   "aws",
		Domain: "contentrecs-api.docs.aws.amazon.com",
	}

	if result, err := security.AnalyseContent(string(body), source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, fmt.Errorf("recommendations response blocked by security policy: %s", result.Message)
		case security.ActionWarn:
			c.logger.WithField("security_id", result.ID).Warn(result.Message)
		}
	}

	// Parse response
	var apiResp RecommendationAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse recommendations response: %w", err)
	}

	// Convert to RecommendationResult format
	var results []RecommendationResult

	// Process highly rated recommendations
	for _, item := range apiResp.HighlyRated.Items {
		var context *string
		if item.Abstract != "" {
			context = &item.Abstract
		}

		results = append(results, RecommendationResult{
			URL:     item.URL,
			Title:   item.AssetTitle,
			Context: context,
		})
	}

	// Process journey recommendations
	for _, intentGroup := range apiResp.Journey.Items {
		intent := intentGroup.Intent
		for _, urlItem := range intentGroup.URLs {
			var context *string
			if intent != "" {
				contextStr := fmt.Sprintf("Intent: %s", intent)
				context = &contextStr
			}

			results = append(results, RecommendationResult{
				URL:     urlItem.URL,
				Title:   urlItem.AssetTitle,
				Context: context,
			})
		}
	}

	// Process new content recommendations
	for _, item := range apiResp.New.Items {
		var context *string
		if item.DateCreated != "" {
			contextStr := fmt.Sprintf("New content added on %s", item.DateCreated)
			context = &contextStr
		} else {
			contextStr := "New content"
			context = &contextStr
		}

		results = append(results, RecommendationResult{
			URL:     item.URL,
			Title:   item.AssetTitle,
			Context: context,
		})
	}

	// Process similar recommendations
	for _, item := range apiResp.Similar.Items {
		var context *string
		if item.Abstract != "" {
			context = &item.Abstract
		} else {
			contextStr := "Similar content"
			context = &contextStr
		}

		results = append(results, RecommendationResult{
			URL:     item.URL,
			Title:   item.AssetTitle,
			Context: context,
		})
	}

	c.logger.WithField("results_count", len(results)).Debug("Recommendations retrieved successfully")
	return results, nil
}

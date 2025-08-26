package brave

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sirupsen/logrus"
)

const (
	// BraveAPIBaseURL is the base URL for Brave Search API
	BraveAPIBaseURL = "https://api.search.brave.com/res/v1"

	// UserAgent for API requests
	UserAgent = "mcp-devtools/1.0"

	// DefaultTimeout for HTTP requests
	DefaultTimeout = 30 * time.Second
)

// BraveClient handles HTTP requests to the Brave Search API
type BraveClient struct {
	apiKey     string
	httpClient internetsearch.HTTPClientInterface
	baseURL    string
}

// NewBraveClient creates a new Brave API client with rate limiting
func NewBraveClient(apiKey string) *BraveClient {
	return &BraveClient{
		apiKey:     apiKey,
		baseURL:    BraveAPIBaseURL,
		httpClient: internetsearch.NewRateLimitedHTTPClient(),
	}
}

// makeRequest performs an HTTP request to the Brave API with security checking
func (c *BraveClient) makeRequest(ctx context.Context, logger *logrus.Logger, endpoint string, params map[string]string) ([]byte, error) {
	// Build URL with parameters
	reqURL, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Check domain access security for API endpoint using security helper
	if err := security.CheckDomainAccess(reqURL.Host); err != nil {
		if secErr, ok := err.(*security.SecurityError); ok {
			return nil, fmt.Errorf("security block [ID: %s]: %s Check with the user if you may use security_override tool with ID %s",
				secErr.GetSecurityID(), secErr.Error(), secErr.GetSecurityID())
		}
		return nil, err
	}

	// Add query parameters
	query := reqURL.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	reqURL.RawQuery = query.Encode()

	logger.WithFields(logrus.Fields{
		"url":      reqURL.String(),
		"endpoint": endpoint,
	}).Debug("Making Brave API request")

	// Retry logic for resilient requests
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	for attempt := range maxRetries {
		// Check context cancellation before each attempt
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("request canceled: %w", ctx.Err())
		default:
		}

		// Create request with context
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("X-Subscription-Token", c.apiKey)
		req.Header.Set("User-Agent", UserAgent)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cache-Control", "no-cache")

		// Make request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Check if error is due to context cancellation
			if ctx.Err() != nil {
				return nil, fmt.Errorf("request canceled: %w", ctx.Err())
			}

			// Log the error and determine if we should retry
			logger.WithFields(logrus.Fields{
				"attempt": attempt + 1,
				"error":   err.Error(),
			}).Warn("HTTP request failed")

			// Retry on network errors, but not on the last attempt
			if attempt < maxRetries-1 {
				delay := time.Duration(attempt+1) * baseDelay
				logger.WithField("delay", delay).Debug("Retrying request after delay")

				select {
				case <-ctx.Done():
					return nil, fmt.Errorf("request canceled during retry: %w", ctx.Err())
				case <-time.After(delay):
				}
				continue
			}

			return nil, fmt.Errorf("failed to make request after %d attempts: %w", maxRetries, err)
		}

		// Process successful response with security analysis
		return c.processResponseWithSecurity(ctx, logger, resp, attempt+1, reqURL.String())
	}

	return nil, fmt.Errorf("unexpected end of retry loop")
}

// processResponseWithSecurity handles the HTTP response processing with security analysis
func (c *BraveClient) processResponseWithSecurity(ctx context.Context, logger *logrus.Logger, resp *http.Response, attempt int, requestURL string) ([]byte, error) {
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	// Handle gzip decompression if needed
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer func() {
			if closeErr := gzipReader.Close(); closeErr != nil {
				logger.WithError(closeErr).Warn("Failed to close gzip reader")
			}
		}()
		reader = gzipReader
	}

	// Read response body
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Security analysis on content
	if security.IsEnabled() {
		parsedURL, _ := url.Parse(requestURL)
		sourceCtx := security.SourceContext{
			URL:         requestURL,
			Domain:      parsedURL.Hostname(),
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

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		logger.WithFields(logrus.Fields{
			"status_code": resp.StatusCode,
			"status":      resp.Status,
			"body":        string(body),
		}).Error("Brave API request failed")

		// Try to parse error response
		var errorResp BraveErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Message != "" {
			return nil, fmt.Errorf("brave API error (%d): %s", resp.StatusCode, errorResp.Message)
		}

		// Provide specific error messages for common status codes
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("authentication failed: invalid API key")
		case http.StatusForbidden:
			return nil, fmt.Errorf("access forbidden: check your API key and subscription plan")
		case http.StatusTooManyRequests:
			return nil, fmt.Errorf("rate limit exceeded: please wait before making more requests")
		case http.StatusInternalServerError:
			return nil, fmt.Errorf("brave API internal server error: please try again later")
		default:
			return nil, fmt.Errorf("brave API request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	logger.WithFields(logrus.Fields{
		"status_code":   resp.StatusCode,
		"response_size": len(body),
	}).Debug("Brave API request successful")

	return body, nil
}

// WebSearch performs a web search using the Brave API
func (c *BraveClient) WebSearch(ctx context.Context, logger *logrus.Logger, query string, count int, offset int, freshness string) (*BraveWebSearchResponse, error) {
	params := map[string]string{
		"q":      query,
		"count":  fmt.Sprintf("%d", count),
		"offset": fmt.Sprintf("%d", offset),
	}

	if freshness != "" {
		params["freshness"] = freshness
	}

	body, err := c.makeRequest(ctx, logger, "/web/search", params)
	if err != nil {
		return nil, err
	}

	var response BraveWebSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse web search response: %w", err)
	}

	return &response, nil
}

// ImageSearch performs an image search using the Brave API
func (c *BraveClient) ImageSearch(ctx context.Context, logger *logrus.Logger, query string, count int) (*BraveImageSearchResponse, error) {
	params := map[string]string{
		"q":     query,
		"count": fmt.Sprintf("%d", count),
	}

	body, err := c.makeRequest(ctx, logger, "/images/search", params)
	if err != nil {
		return nil, err
	}

	var response BraveImageSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse image search response: %w", err)
	}

	return &response, nil
}

// NewsSearch performs a news search using the Brave API
func (c *BraveClient) NewsSearch(ctx context.Context, logger *logrus.Logger, query string, count int, freshness string) (*BraveNewsSearchResponse, error) {
	params := map[string]string{
		"q":     query,
		"count": fmt.Sprintf("%d", count),
	}

	if freshness != "" {
		params["freshness"] = freshness
	}

	body, err := c.makeRequest(ctx, logger, "/news/search", params)
	if err != nil {
		return nil, err
	}

	var response BraveNewsSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse news search response: %w", err)
	}

	return &response, nil
}

// VideoSearch performs a video search using the Brave API
func (c *BraveClient) VideoSearch(ctx context.Context, logger *logrus.Logger, query string, count int, freshness string) (*BraveVideoSearchResponse, error) {
	params := map[string]string{
		"q":     query,
		"count": fmt.Sprintf("%d", count),
	}

	if freshness != "" {
		params["freshness"] = freshness
	}

	body, err := c.makeRequest(ctx, logger, "/videos/search", params)
	if err != nil {
		return nil, err
	}

	var response BraveVideoSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse video search response: %w", err)
	}

	return &response, nil
}

// LocalSearch performs a local search using the Brave API
func (c *BraveClient) LocalSearch(ctx context.Context, logger *logrus.Logger, query string, count int) (*BraveLocalSearchResponse, error) {
	params := map[string]string{
		"q":             query,
		"count":         fmt.Sprintf("%d", count),
		"result_filter": "locations",
	}

	body, err := c.makeRequest(ctx, logger, "/web/search", params)
	if err != nil {
		return nil, err
	}

	var response BraveLocalSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse local search response: %w", err)
	}

	return &response, nil
}

// LocalPOISearch fetches POI details for given location IDs
func (c *BraveClient) LocalPOISearch(ctx context.Context, logger *logrus.Logger, ids []string) (*BraveLocalPOIResponse, error) {
	params := make(map[string]string)
	for i, id := range ids {
		params[fmt.Sprintf("ids[%d]", i)] = id
	}

	body, err := c.makeRequest(ctx, logger, "/local/pois", params)
	if err != nil {
		return nil, err
	}

	var response BraveLocalPOIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse local POI response: %w", err)
	}

	return &response, nil
}

// LocalDescriptionsSearch fetches descriptions for given location IDs
func (c *BraveClient) LocalDescriptionsSearch(ctx context.Context, logger *logrus.Logger, ids []string) (*BraveLocalDescriptionsResponse, error) {
	params := make(map[string]string)
	for i, id := range ids {
		params[fmt.Sprintf("ids[%d]", i)] = id
	}

	body, err := c.makeRequest(ctx, logger, "/local/descriptions", params)
	if err != nil {
		return nil, err
	}

	var response BraveLocalDescriptionsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse local descriptions response: %w", err)
	}

	return &response, nil
}

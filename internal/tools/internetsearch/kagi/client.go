package kagi

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
	// KagiAPIBaseURL is the base URL for Kagi Search API
	KagiAPIBaseURL = "https://kagi.com/api/v0"

	// UserAgent for API requests
	UserAgent = "mcp-devtools/1.0"

	// DefaultTimeout for HTTP requests
	DefaultTimeout = 30 * time.Second
)

// KagiClient handles HTTP requests to the Kagi Search API
type KagiClient struct {
	apiKey     string
	httpClient internetsearch.HTTPClientInterface
	baseURL    string
}

// NewKagiClient creates a new Kagi API client with rate limiting
func NewKagiClient(apiKey string) *KagiClient {
	return &KagiClient{
		apiKey:     apiKey,
		baseURL:    KagiAPIBaseURL,
		httpClient: internetsearch.NewRateLimitedHTTPClient(),
	}
}

// makeRequest performs an HTTP request to the Kagi API with security checking
func (c *KagiClient) makeRequest(ctx context.Context, logger *logrus.Logger, endpoint string, params map[string]string) ([]byte, error) {
	// Build URL with parameters
	reqURL, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Check domain access security for API endpoint using security helper
	if err := security.CheckDomainAccess(reqURL.Host); err != nil {
		if secErr, ok := err.(*security.SecurityError); ok {
			return nil, security.FormatSecurityBlockError(secErr)
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
	}).Debug("Making Kagi API request")

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
		req.Header.Set("Authorization", fmt.Sprintf("Bot %s", c.apiKey))
		req.Header.Set("User-Agent", UserAgent)

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
		return c.processResponseWithSecurity(logger, resp, reqURL.String())
	}

	return nil, fmt.Errorf("unexpected end of retry loop")
}

// processResponseWithSecurity handles the HTTP response processing with security analysis
func (c *KagiClient) processResponseWithSecurity(logger *logrus.Logger, resp *http.Response, requestURL string) ([]byte, error) {
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
				return nil, security.FormatSecurityBlockErrorFromResult(secResult)
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
		}).Error("Kagi API request failed")

		// Try to parse error response
		var errorResp KagiSearchResponse
		if err := json.Unmarshal(body, &errorResp); err == nil && len(errorResp.Error) > 0 {
			errMsg := errorResp.Error[0].Msg
			return nil, fmt.Errorf("kagi API error (%d): %s", resp.StatusCode, errMsg)
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
			return nil, fmt.Errorf("kagi API internal server error: please try again later")
		default:
			return nil, fmt.Errorf("kagi API request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	logger.WithFields(logrus.Fields{
		"status_code":   resp.StatusCode,
		"response_size": len(body),
	}).Debug("Kagi API request successful")

	return body, nil
}

// Search performs a web search using the Kagi API
func (c *KagiClient) Search(ctx context.Context, logger *logrus.Logger, query string, limit int) (*KagiSearchResponse, error) {
	params := map[string]string{
		"q":     query,
		"limit": fmt.Sprintf("%d", limit),
	}

	body, err := c.makeRequest(ctx, logger, "/search", params)
	if err != nil {
		return nil, err
	}

	var response KagiSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	// Check for API errors in response
	if len(response.Error) > 0 {
		return nil, fmt.Errorf("kagi API error: %s", response.Error[0].Msg)
	}

	return &response, nil
}

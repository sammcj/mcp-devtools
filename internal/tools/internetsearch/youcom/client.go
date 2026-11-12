package youcom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sirupsen/logrus"
)

const (
	// KeylessSearchURL is the free keyless You.com search tier
	KeylessSearchURL = "https://api.you.com/v1/agents/search"
	// KeyedSearchURL is the keyed You.com Web Search API
	KeyedSearchURL = "https://ydc-index.io/v1/search"
	// UserAgent is sent only on requests to You.com hosts
	UserAgent = "mcp-devtools/1.0 youdotcom-integration/sammcj-mcp-devtools"
	// MaxKeylessCount is the largest count the keyless tier accepts
	MaxKeylessCount = 50
)

// YouComClient handles HTTP requests to the You.com search APIs
type YouComClient struct {
	apiKey     string
	httpClient internetsearch.HTTPClientInterface
	keylessURL string
	keyedURL   string
}

// NewYouComClient creates a new You.com client. An empty apiKey uses the keyless tier.
func NewYouComClient(apiKey string) *YouComClient {
	return &YouComClient{
		apiKey:     apiKey,
		httpClient: internetsearch.NewRateLimitedHTTPClient(),
		keylessURL: KeylessSearchURL,
		keyedURL:   KeyedSearchURL,
	}
}

// newRequest builds the keyless GET or the keyed POST request
func (c *YouComClient) newRequest(ctx context.Context, query string, count int) (*http.Request, error) {
	var req *http.Request
	var err error
	if c.apiKey != "" {
		body, merr := json.Marshal(map[string]any{"query": query, "count": count})
		if merr != nil {
			return nil, fmt.Errorf("failed to encode request: %w", merr)
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, c.keyedURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", c.apiKey)
	} else {
		reqURL, perr := url.Parse(c.keylessURL)
		if perr != nil {
			return nil, fmt.Errorf("failed to parse URL: %w", perr)
		}
		params := reqURL.Query()
		params.Set("query", query)
		params.Set("count", strconv.Itoa(count))
		reqURL.RawQuery = params.Encode()
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	return req, nil
}

// Search performs a search against You.com
func (c *YouComClient) Search(ctx context.Context, logger *logrus.Logger, query string, count int) (*YouComSearchResponse, error) {
	req, err := c.newRequest(ctx, query, count)
	if err != nil {
		return nil, err
	}

	// Check domain access security for the API endpoint
	if err := security.CheckDomainAccess(req.URL.Host); err != nil {
		if secErr, ok := err.(*security.SecurityError); ok {
			return nil, security.FormatSecurityBlockError(secErr)
		}
		return nil, err
	}

	logger.WithFields(logrus.Fields{
		"host":  req.URL.Host,
		"keyed": c.apiKey != "",
	}).Debug("Making You.com API request")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("request canceled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Security analysis on content
	if security.IsEnabled() {
		sourceCtx := security.SourceContext{
			URL:         req.URL.String(),
			Domain:      req.URL.Hostname(),
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

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, fmt.Errorf("authentication failed (%d): check YDC_API_KEY", resp.StatusCode)
		case http.StatusTooManyRequests:
			if c.apiKey == "" {
				return nil, fmt.Errorf("rate limit exceeded: keyless daily limit reached, set YDC_API_KEY for higher limits")
			}
			return nil, fmt.Errorf("rate limit exceeded: please wait before making more requests")
		default:
			return nil, fmt.Errorf("you.com API request failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	var response YouComSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}
	return &response, nil
}

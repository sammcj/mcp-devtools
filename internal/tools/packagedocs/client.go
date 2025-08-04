package packagedocs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	context7BaseURL      = "https://context7.com/api"
	defaultMinimumTokens = 10000
	cacheExpiry          = 120 * time.Minute
)

// Client handles communication with the Context7 API
type Client struct {
	httpClient *http.Client
	logger     *logrus.Logger
	cache      map[string]cacheEntry
}

type cacheEntry struct {
	data      interface{}
	timestamp time.Time
}

// NewClient creates a new Context7 API client
func NewClient(logger *logrus.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
		cache:  make(map[string]cacheEntry),
	}
}

// SearchLibrariesResponse represents the response from the search API
type SearchLibrariesResponse struct {
	Results []*SearchResult `json:"results"`
}

// SearchResult represents a library search result
type SearchResult struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	LastUpdate    time.Time `json:"lastUpdateDate"`
	TotalTokens   int       `json:"totalTokens"`
	TotalSnippets int       `json:"totalSnippets"`
	Stars         int       `json:"stars"`
	TrustScore    float64   `json:"trustScore,omitempty"`
}

// GetResourceURI returns the Context7 resource URI for this search result
func (s *SearchResult) GetResourceURI() string {
	return "context7://libraries/" + strings.TrimLeft(s.ID, "/")
}

// SearchLibraries searches for libraries matching the given query
func (c *Client) SearchLibraries(ctx context.Context, query string) ([]*SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	// Check cache first
	cacheKey := "search:" + query
	if entry, ok := c.cache[cacheKey]; ok {
		if time.Since(entry.timestamp) < cacheExpiry {
			c.logger.Debug("Returning cached search results")
			return entry.data.([]*SearchResult), nil
		}
		delete(c.cache, cacheKey)
	}

	c.logger.WithField("query", query).Info("Searching libraries")

	params := map[string]string{"query": query}
	var response SearchLibrariesResponse

	err := c.makeRequest(ctx, "GET", "/v1/search", params, nil, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to search libraries: %w", err)
	}

	// Cache the results
	c.cache[cacheKey] = cacheEntry{
		data:      response.Results,
		timestamp: time.Now(),
	}

	c.logger.WithFields(logrus.Fields{
		"query":        query,
		"result_count": len(response.Results),
	}).Info("Library search completed")

	return response.Results, nil
}

// SearchLibraryDocsParams represents parameters for searching library documentation
type SearchLibraryDocsParams struct {
	Topic   string   `json:"topic,omitempty"`
	Tokens  int      `json:"tokens,omitempty"`
	Folders []string `json:"folders,omitempty"`
}

// GetLibraryDocs retrieves documentation for a specific library
func (c *Client) GetLibraryDocs(ctx context.Context, libraryID string, params *SearchLibraryDocsParams) (string, error) {
	if libraryID == "" {
		return "", fmt.Errorf("library ID cannot be empty")
	}

	// Handle full Context7 URI or just the path part
	var apiPath string
	if strings.HasPrefix(libraryID, "context7://libraries/") {
		// Parse the Context7 URI and extract the path part only
		parsedURL, err := url.Parse(libraryID)
		if err != nil {
			return "", fmt.Errorf("invalid Context7 URI format: %w", err)
		}
		if parsedURL.Scheme != "context7" || parsedURL.Host != "libraries" {
			return "", fmt.Errorf("invalid Context7 URI scheme or host")
		}
		apiPath = parsedURL.Path // This will be something like "/vercel/next.js"
	} else if strings.HasPrefix(libraryID, "/") {
		// Already a path, use as-is
		apiPath = libraryID
	} else {
		// Just the library name, prepend with /
		apiPath = "/" + libraryID
	}

	if params == nil {
		params = &SearchLibraryDocsParams{}
	}
	if params.Tokens == 0 {
		params.Tokens = defaultMinimumTokens
	}

	// Build cache key
	cacheKey := fmt.Sprintf("docs:%s:%s:%d:%s",
		apiPath,
		params.Topic,
		params.Tokens,
		strings.Join(params.Folders, ","))

	// Check cache
	if entry, ok := c.cache[cacheKey]; ok {
		if time.Since(entry.timestamp) < cacheExpiry {
			c.logger.Debug("Returning cached documentation")
			return entry.data.(string), nil
		}
		delete(c.cache, cacheKey)
	}

	c.logger.WithFields(logrus.Fields{
		"library_id": libraryID,
		"api_path":   apiPath,
		"topic":      params.Topic,
		"tokens":     params.Tokens,
		"folders":    params.Folders,
	}).Info("Fetching library documentation")

	// Build query parameters
	queryParams := map[string]string{
		"type":   "txt",
		"tokens": strconv.Itoa(params.Tokens),
	}
	if params.Topic != "" {
		queryParams["topic"] = params.Topic
	}
	if len(params.Folders) > 0 {
		queryParams["folders"] = strings.Join(params.Folders, ",")
	}

	var result string
	err := c.makeRequest(ctx, "GET", "/v1"+apiPath, queryParams, nil, &result)
	if err != nil {
		return "", fmt.Errorf("failed to get library documentation: %w", err)
	}

	// Cache the result
	c.cache[cacheKey] = cacheEntry{
		data:      result,
		timestamp: time.Now(),
	}

	c.logger.WithFields(logrus.Fields{
		"library_id":     libraryID,
		"content_length": len(result),
	}).Info("Library documentation retrieved")

	return result, nil
}

// makeRequest makes an HTTP request to the Context7 API
func (c *Client) makeRequest(ctx context.Context, method, path string, params map[string]string, body io.Reader, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, context7BaseURL+path, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to match the official client
	req.Header.Set("User-Agent", "mcp-devtools")
	req.Header.Set("X-Context7-Source", "mcp-server")

	// Add query parameters
	if params != nil {
		query := req.URL.Query()
		for k, v := range params {
			query.Set(k, v)
		}
		req.URL.RawQuery = query.Encode()
	}

	c.logger.WithFields(logrus.Fields{
		"method": method,
		"url":    req.URL.String(),
	}).Debug("Making Context7 API request")

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.WithError(err).Warn("Failed to close response body")
		}
	}()

	duration := time.Since(start)
	c.logger.WithFields(logrus.Fields{
		"status":   resp.Status,
		"duration": duration.Round(time.Millisecond),
	}).Debug("Context7 API request completed")

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Handle string response type
	if _, ok := result.(*string); ok {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		*(result.(*string)) = string(bodyBytes)
		return nil
	}

	// Handle JSON response
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode JSON response: %w", err)
	}

	return nil
}

// ValidateLibraryID validates a Context7 library ID format
func ValidateLibraryID(libraryID string) error {
	if libraryID == "" {
		return fmt.Errorf("library ID cannot be empty")
	}

	// Parse as URL to validate format
	resourceURL := "context7://libraries" + libraryID
	_, err := url.Parse(resourceURL)
	if err != nil {
		return fmt.Errorf("invalid library ID format: %w", err)
	}

	return nil
}

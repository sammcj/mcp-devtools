package packagedocs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sirupsen/logrus"
)

const (
	context7BaseURL      = "https://context7.com/api"
	defaultMinimumTokens = 10000
	cacheExpiry          = 120 * time.Minute

	// DefaultPackageDocsRateLimit is the default maximum requests per second for package docs
	DefaultPackageDocsRateLimit = 10
	// PackageDocsRateLimitEnvVar is the environment variable for configuring rate limit
	PackageDocsRateLimitEnvVar = "PACKAGE_DOCS_RATE_LIMIT"
)

// RateLimitedHTTPClient implements a rate-limited HTTP client
type RateLimitedHTTPClient struct {
	client  *http.Client
	limiter *rate.Limiter
	mu      sync.Mutex
}

// Do implements the HTTP client interface with rate limiting
func (c *RateLimitedHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Wait for rate limiter to allow the request
	err := c.limiter.Wait(context.Background())
	if err != nil {
		return nil, err
	}

	return c.client.Do(req)
}

// getPackageDocsRateLimit returns the configured rate limit for package docs requests
func getPackageDocsRateLimit() float64 {
	if envValue := os.Getenv(PackageDocsRateLimitEnvVar); envValue != "" {
		if value, err := strconv.ParseFloat(envValue, 64); err == nil && value > 0 {
			return value
		}
	}
	return DefaultPackageDocsRateLimit
}

// newRateLimitedHTTPClient creates a new rate-limited HTTP client
func newRateLimitedHTTPClient() *RateLimitedHTTPClient {
	rateLimit := getPackageDocsRateLimit()
	return &RateLimitedHTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		limiter: rate.NewLimiter(rate.Limit(rateLimit), 1), // Allow burst of 1
	}
}

// HTTPClientInterface defines the interface for HTTP clients
type HTTPClientInterface interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client handles communication with the Context7 API
type Client struct {
	httpClient HTTPClientInterface
	logger     *logrus.Logger
	cache      map[string]cacheEntry
}

type cacheEntry struct {
	data      interface{}
	timestamp time.Time
}

// NewClient creates a new Context7 API client with rate limiting
func NewClient(logger *logrus.Logger) *Client {
	return &Client{
		httpClient: newRateLimitedHTTPClient(),
		logger:     logger,
		cache:      make(map[string]cacheEntry),
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
	// Build full URL
	fullURL := context7BaseURL + path

	// Add query parameters
	if params != nil {
		parsedURL, err := url.Parse(fullURL)
		if err != nil {
			return fmt.Errorf("failed to parse URL: %w", err)
		}
		query := parsedURL.Query()
		for k, v := range params {
			query.Set(k, v)
		}
		parsedURL.RawQuery = query.Encode()
		fullURL = parsedURL.String()
	}

	c.logger.WithFields(logrus.Fields{
		"method": method,
		"url":    fullURL,
	}).Debug("Making Context7 API request")

	start := time.Now()

	// Use security helper for HTTP operations
	ops := security.NewOperations("packagedocs")

	var safeResp *security.SafeHTTPResponse
	var err error

	switch method {
	case "GET":
		safeResp, err = ops.SafeHTTPGet(fullURL)
	case "POST":
		safeResp, err = ops.SafeHTTPPost(fullURL, body)
	default:
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}

	if err != nil {
		if secErr, ok := err.(*security.SecurityError); ok {
			return fmt.Errorf("security block [ID: %s]: %s. Check with the user if you may use security_override tool with ID %s",
				secErr.GetSecurityID(), secErr.Error(), secErr.GetSecurityID())
		}
		return fmt.Errorf("request failed: %w", err)
	}

	duration := time.Since(start)
	c.logger.WithFields(logrus.Fields{
		"status":   safeResp.StatusCode,
		"duration": duration.Round(time.Millisecond),
	}).Debug("Context7 API request completed")

	// Handle warnings
	if safeResp.SecurityResult != nil && safeResp.SecurityResult.Action == security.ActionWarn {
		c.logger.Warnf("Security warning [ID: %s]: %s", safeResp.SecurityResult.ID, safeResp.SecurityResult.Message)
	}

	if safeResp.StatusCode >= 400 {
		// Limit error response content for security
		content := safeResp.Content
		if len(content) > 1024*1024 { // 1MB limit for error responses
			content = content[:1024*1024]
		}
		return fmt.Errorf("API request failed with status %d: %s", safeResp.StatusCode, string(content))
	}

	// Handle string response type
	if _, ok := result.(*string); ok {
		// Use exact content from security helper (already validated)
		*(result.(*string)) = string(safeResp.Content)
		return nil
	}

	// Handle JSON response
	if err := json.NewDecoder(strings.NewReader(string(safeResp.Content))).Decode(result); err != nil {
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

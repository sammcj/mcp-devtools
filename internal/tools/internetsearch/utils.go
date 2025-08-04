package internetsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	// DefaultInternetSearchRateLimit is the default maximum requests per second for internet search
	DefaultInternetSearchRateLimit = 1
	// InternetSearchRateLimitEnvVar is the environment variable for configuring rate limit
	InternetSearchRateLimitEnvVar = "INTERNET_SEARCH_RATE_LIMIT"
)

// NewToolResultJSON creates a new tool result with JSON content
func NewToolResultJSON(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// HTTPClientInterface defines the interface for HTTP clients
type HTTPClientInterface interface {
	Do(req *http.Request) (*http.Response, error)
}

// RateLimitedHTTPClient implements HTTPClientInterface with rate limiting
type RateLimitedHTTPClient struct {
	client  *http.Client
	limiter *rate.Limiter
	mu      sync.Mutex
}

// getInternetSearchRateLimit returns the configured rate limit for internet search requests
func getInternetSearchRateLimit() float64 {
	if envValue := os.Getenv(InternetSearchRateLimitEnvVar); envValue != "" {
		if value, err := strconv.ParseFloat(envValue, 64); err == nil && value > 0 {
			return value
		}
	}
	return DefaultInternetSearchRateLimit
}

// NewRateLimitedHTTPClient creates a new rate-limited HTTP client for internet search
func NewRateLimitedHTTPClient() *RateLimitedHTTPClient {
	rateLimit := getInternetSearchRateLimit()
	return &RateLimitedHTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		limiter: rate.NewLimiter(rate.Limit(rateLimit), 1), // Allow burst of 1
	}
}

// Do implements the HTTPClientInterface interface with rate limiting
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

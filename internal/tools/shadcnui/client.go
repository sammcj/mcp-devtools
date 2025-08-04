package shadcnui

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	ShadcnDocsURL        = "https://ui.shadcn.com"
	ShadcnDocsComponents = ShadcnDocsURL + "/docs/components"
	ShadcnGitHubURL      = "https://github.com/shadcn-ui/ui"
	ShadcnRawGitHubURL   = "https://raw.githubusercontent.com/shadcn-ui/ui/main"

	// DefaultShadcnRateLimit is the default maximum requests per second
	DefaultShadcnRateLimit = 5
	// ShadcnRateLimitEnvVar is the environment variable for configuring rate limit
	ShadcnRateLimitEnvVar = "SHADCN_RATE_LIMIT"
)

// HTTPClient defines the interface for an HTTP client.
type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

// RateLimitedHTTPClient implements HTTPClient with rate limiting
type RateLimitedHTTPClient struct {
	client  *http.Client
	limiter *rate.Limiter
	mu      sync.Mutex
}

// getShadcnRateLimit returns the configured rate limit for ShadCN requests
func getShadcnRateLimit() float64 {
	if envValue := os.Getenv(ShadcnRateLimitEnvVar); envValue != "" {
		if value, err := strconv.ParseFloat(envValue, 64); err == nil && value > 0 {
			return value
		}
	}
	return DefaultShadcnRateLimit
}

// NewRateLimitedHTTPClient creates a new rate-limited HTTP client
func NewRateLimitedHTTPClient() *RateLimitedHTTPClient {
	rateLimit := getShadcnRateLimit()
	return &RateLimitedHTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		limiter: rate.NewLimiter(rate.Limit(rateLimit), 1), // Allow burst of 1
	}
}

// Get implements the HTTPClient interface with rate limiting
func (c *RateLimitedHTTPClient) Get(url string) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Wait for rate limiter to allow the request
	err := c.limiter.Wait(context.Background())
	if err != nil {
		return nil, err
	}

	return c.client.Get(url)
}

// DefaultHTTPClient is the default HTTP client implementation with rate limiting.
var DefaultHTTPClient HTTPClient = NewRateLimitedHTTPClient()

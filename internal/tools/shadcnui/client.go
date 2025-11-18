package shadcnui

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/utils/httpclient"
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
// This interface is maintained for compatibility but tools should migrate to security.Operations
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

// NewRateLimitedHTTPClient creates a new rate-limited HTTP client with proxy support
func NewRateLimitedHTTPClient() *RateLimitedHTTPClient {
	rateLimit := getShadcnRateLimit()

	// Use shared HTTP client factory with proxy support
	client := httpclient.NewHTTPClientWithProxy(30 * time.Second)

	return &RateLimitedHTTPClient{
		client:  client,
		limiter: rate.NewLimiter(rate.Limit(rateLimit), 1), // Allow burst of 1
	}
}

// Get implements the HTTPClient interface with rate limiting
// Note: This client is deprecated in favour of security.Operations.SafeHTTPGet
func (c *RateLimitedHTTPClient) Get(reqURL string) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx := context.Background()

	// Wait for rate limiter to allow the request
	err := c.limiter.Wait(ctx)
	if err != nil {
		return nil, err
	}

	// Use security helper for consistent security handling
	ops := security.NewOperations("shadcnui")
	safeResp, err := ops.SafeHTTPGet(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	// Convert back to http.Response for interface compatibility
	resp := &http.Response{
		StatusCode: safeResp.StatusCode,
		Header:     safeResp.Headers,
		Body:       &responseBodyWrapper{content: safeResp.Content},
	}

	return resp, nil
}

// responseBodyWrapper wraps content as an io.ReadCloser for http.Response compatibility
type responseBodyWrapper struct {
	content []byte
	pos     int
}

func (r *responseBodyWrapper) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.content) {
		return 0, nil // EOF
	}
	n = copy(p, r.content[r.pos:])
	r.pos += n
	return n, nil
}

func (r *responseBodyWrapper) Close() error {
	return nil // No-op for byte slice wrapper
}

// DefaultHTTPClient is the default HTTP client implementation with rate limiting.
var DefaultHTTPClient HTTPClient = NewRateLimitedHTTPClient()

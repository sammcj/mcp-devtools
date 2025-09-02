package packageversions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sirupsen/logrus"
)

// HTTPClient is an interface for making HTTP requests
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

const (
	// DefaultPackagesRateLimit is the default maximum requests per second
	DefaultPackagesRateLimit = 10
	// PackagesRateLimitEnvVar is the environment variable for configuring rate limit
	PackagesRateLimitEnvVar = "PACKAGES_RATE_LIMIT"
)

// RateLimitedHTTPClient implements HTTPClient with rate limiting
type RateLimitedHTTPClient struct {
	client  *http.Client
	limiter *rate.Limiter
	mu      sync.Mutex
}

// getPackagesRateLimit returns the configured rate limit for package requests
func getPackagesRateLimit() float64 {
	if envValue := os.Getenv(PackagesRateLimitEnvVar); envValue != "" {
		if value, err := strconv.ParseFloat(envValue, 64); err == nil && value > 0 {
			return value
		}
	}
	return DefaultPackagesRateLimit
}

// NewRateLimitedHTTPClient creates a new rate-limited HTTP client
func NewRateLimitedHTTPClient() *RateLimitedHTTPClient {
	rateLimit := getPackagesRateLimit()
	return &RateLimitedHTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		limiter: rate.NewLimiter(rate.Limit(rateLimit), 1), // Allow burst of 1
	}
}

// Do implements the HTTPClient interface with rate limiting
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

var (
	// DefaultHTTPClient is the default HTTP client with rate limiting
	DefaultHTTPClient HTTPClient = NewRateLimitedHTTPClient()
)

// MakeRequest makes an HTTP request and returns the response body
func MakeRequest(client HTTPClient, method, url string, headers map[string]string) ([]byte, error) {
	return MakeRequestWithLogger(client, nil, method, url, headers)
}

// MakeRequestWithLogger makes an HTTP request with logging and returns the response body
func MakeRequestWithLogger(client HTTPClient, logger *logrus.Logger, method, reqURL string, headers map[string]string) ([]byte, error) {
	if logger != nil {
		logger.WithFields(logrus.Fields{
			"method": method,
			"url":    reqURL,
		}).Debug("Making HTTP request")
	}

	// Parse URL for security checks
	parsedURL, err := url.Parse(reqURL)
	if err != nil {
		return nil, err
	}

	// Check domain access control via security system
	if err := security.CheckDomainAccess(parsedURL.Hostname()); err != nil {
		if secErr, ok := err.(*security.SecurityError); ok {
			return nil, security.FormatSecurityBlockError(secErr)
		}
		return nil, err
	}

	req, err := http.NewRequest(method, reqURL, nil)
	if err != nil {
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"method": method,
				"url":    reqURL,
				"error":  err.Error(),
			}).Error("Failed to create request")
		}
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Set default headers if not provided
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "MCP-DevTools/1.0.0")
	}

	// Send request with rate-limited client
	resp, err := client.Do(req)
	if err != nil {
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"method": method,
				"url":    reqURL,
				"error":  err.Error(),
			}).Error("Failed to send request")
		}
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			if logger != nil {
				logger.WithError(err).Error("Failed to close response body")
			}
			// Note: Silently ignore if logger is nil to avoid stdio pollution
		}
	}()

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"method":     method,
				"url":        reqURL,
				"statusCode": resp.StatusCode,
			}).Error("Unexpected status code")
		}
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body with size limit to prevent memory exhaustion
	limitedReader := io.LimitReader(resp.Body, 10*1024*1024) // 10MB limit for package API responses
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"method": method,
				"url":    reqURL,
				"error":  err.Error(),
			}).Error("Failed to read response body")
		}
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Analyse content for security threats using security helper
	sourceContext := security.SourceContext{
		URL:         reqURL,
		Domain:      parsedURL.Hostname(),
		ContentType: resp.Header.Get("Content-Type"),
		Tool:        "packageversions",
	}
	if secResult, err := security.AnalyseContent(string(body), sourceContext); err == nil {
		switch secResult.Action {
		case security.ActionBlock:
			return nil, security.FormatSecurityBlockErrorFromResult(secResult)
		case security.ActionWarn:
			if logger != nil {
				logger.Warnf("Security warning [ID: %s]: %s", secResult.ID, secResult.Message)
			}
		}
	}

	if logger != nil {
		logger.WithFields(logrus.Fields{
			"method":     method,
			"url":        reqURL,
			"statusCode": resp.StatusCode,
		}).Debug("HTTP request completed successfully")
	}

	return body, nil
}

// NewToolResultJSON creates a new tool result with JSON content
func NewToolResultJSON(data any) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// ParseVersion parses a version string into major, minor, and patch components
func ParseVersion(version string) (major, minor, patch int, err error) {
	// Remove any leading 'v' or other prefixes
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")

	// Remove any build metadata or pre-release identifiers
	if idx := strings.IndexAny(version, "-+"); idx != -1 {
		version = version[:idx]
	}

	// Split the version string
	parts := strings.Split(version, ".")
	if len(parts) < 1 {
		return 0, 0, 0, fmt.Errorf("invalid version format: %s", version)
	}

	// Parse major version
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version: %s", parts[0])
	}

	// Parse minor version if available
	if len(parts) > 1 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return major, 0, 0, fmt.Errorf("invalid minor version: %s", parts[1])
		}
	}

	// Parse patch version if available
	if len(parts) > 2 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return major, minor, 0, fmt.Errorf("invalid patch version: %s", parts[2])
		}
	}

	return major, minor, patch, nil
}

// CompareVersions compares two version strings
// Returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	 1 if v1 > v2
func CompareVersions(v1, v2 string) (int, error) {
	major1, minor1, patch1, err := ParseVersion(v1)
	if err != nil {
		return 0, fmt.Errorf("failed to parse version 1: %w", err)
	}

	major2, minor2, patch2, err := ParseVersion(v2)
	if err != nil {
		return 0, fmt.Errorf("failed to parse version 2: %w", err)
	}

	// Compare major version
	if major1 < major2 {
		return -1, nil
	}
	if major1 > major2 {
		return 1, nil
	}

	// Compare minor version
	if minor1 < minor2 {
		return -1, nil
	}
	if minor1 > minor2 {
		return 1, nil
	}

	// Compare patch version
	if patch1 < patch2 {
		return -1, nil
	}
	if patch1 > patch2 {
		return 1, nil
	}

	// Versions are equal
	return 0, nil
}

// CleanVersion removes any leading version prefix (^, ~, >, =, <, etc.) from a version string
func CleanVersion(version string) string {
	re := regexp.MustCompile(`^[\^~>=<]+`)
	return re.ReplaceAllString(version, "")
}

// StringPtr returns a pointer to the given string
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns a pointer to the given int
func IntPtr(i int) *int {
	return &i
}

// Int64Ptr returns a pointer to the given int64
func Int64Ptr(i int64) *int64 {
	return &i
}

// ExtractMajorVersion extracts the major version from a version string
func ExtractMajorVersion(version string) (int, error) {
	major, _, _, err := ParseVersion(version)
	return major, err
}

// FuzzyMatch performs a simple fuzzy match between a string and a query
func FuzzyMatch(str, query string) bool {
	if query == "" {
		return true
	}
	if str == "" {
		return false
	}

	// Direct substring match
	if strings.Contains(str, query) {
		return true
	}

	// Check for character-by-character fuzzy match
	strIndex := 0
	queryIndex := 0

	for strIndex < len(str) && queryIndex < len(query) {
		if str[strIndex] == query[queryIndex] {
			queryIndex++
		}
		strIndex++
	}

	return queryIndex == len(query)
}

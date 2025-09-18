package httpclient

import (
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// ProxyEnvironmentVariables defines the order of preference for proxy environment variables
// Following standard conventions used by curl, wget, and other tools
var ProxyEnvironmentVariables = []string{
	"HTTPS_PROXY",
	"https_proxy",
	"HTTP_PROXY",
	"http_proxy",
}

// NewHTTPClientWithProxy creates an HTTP client with optional proxy support
// Only configures proxy if environment variables are set
// Uses standard proxy environment variables in order of preference
func NewHTTPClientWithProxy(timeout time.Duration) *http.Client {
	client := &http.Client{
		Timeout: timeout,
	}

	// Only configure proxy if environment variables are set
	if proxyURL := getProxyURL(); proxyURL != "" {
		if parsedProxy, err := url.Parse(proxyURL); err == nil {
			// Use default transport as base to preserve important settings
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.Proxy = http.ProxyURL(parsedProxy)
			client.Transport = transport
		}
	}

	return client
}

// NewHTTPClientWithProxyAndLogger creates an HTTP client with optional proxy support and logging
func NewHTTPClientWithProxyAndLogger(timeout time.Duration, logger *logrus.Logger) *http.Client {
	client := &http.Client{
		Timeout: timeout,
	}

	// Only configure proxy if environment variables are set
	if proxyURL := getProxyURL(); proxyURL != "" {
		if parsedProxy, err := url.Parse(proxyURL); err == nil {
			// Use default transport as base to preserve important settings
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.Proxy = http.ProxyURL(parsedProxy)
			client.Transport = transport
			if logger != nil {
				logger.WithField("proxy_url", redactProxyCredentials(proxyURL)).Debug("HTTP client configured with proxy")
			}
		} else {
			if logger != nil {
				logger.WithError(err).WithField("proxy_url", redactProxyCredentials(proxyURL)).Warn("Failed to parse proxy URL, using direct connection")
			}
		}
	}

	return client
}

// getProxyURL returns the first valid proxy URL from environment variables
// Returns empty string if no proxy is configured
func getProxyURL() string {
	for _, envVar := range ProxyEnvironmentVariables {
		if proxyURL := os.Getenv(envVar); proxyURL != "" {
			// Skip placeholder values that some tools use
			if proxyURL != "$HTTPS_PROXY" && proxyURL != "$HTTP_PROXY" {
				return proxyURL
			}
		}
	}
	return ""
}

// redactProxyCredentials removes credentials from proxy URL for safe logging
func redactProxyCredentials(proxyURL string) string {
	if parsed, err := url.Parse(proxyURL); err == nil {
		if parsed.User != nil {
			parsed.User = url.UserPassword("***", "***")
		}
		return parsed.String()
	}
	return "[invalid-url]"
}

// IsProxyConfigured returns true if any proxy environment variable is set
func IsProxyConfigured() bool {
	return getProxyURL() != ""
}

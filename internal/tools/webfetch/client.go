package webfetch

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sirupsen/logrus"
)

const (
	// DefaultTimeout for HTTP requests
	DefaultTimeout = 15 * time.Second

	// UserAgent for web requests
	UserAgent = "mcp-devtools-fetch/1.0 (AI Assistant Tool)"

	// MaxContentSize to prevent memory issues (20MB)
	MaxContentSize = 20 * 1024 * 1024
)

// WebClient handles HTTP requests for fetching web content
type WebClient struct {
	httpClient *http.Client
	userAgent  string
}

// NewWebClient creates a new web client with proper timeouts and context support
func NewWebClient() *WebClient {
	return &WebClient{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Allow up to 10 redirects
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				// Preserve User-Agent on redirects
				req.Header.Set("User-Agent", UserAgent)
				return nil
			},
		},
		userAgent: UserAgent,
	}
}

// decompressGzip decompresses gzip-compressed data
func decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			// Log error but don't fail the operation as data is already read
			// This is best practice for cleanup operations where we can't propagate the error
			_ = closeErr
		}
	}()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress gzip data: %w", err)
	}

	return decompressed, nil
}

// FetchContent fetches content from a URL with context support
func (c *WebClient) FetchContent(ctx context.Context, logger *logrus.Logger, targetURL string) (*FetchURLResponse, error) {
	// Validate URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Ensure URL has a scheme
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "https"
		targetURL = parsedURL.String()
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s (only http and https are supported)", parsedURL.Scheme)
	}

	logger.WithField("url", targetURL).Debug("Fetching URL content")

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for polite web scraping
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.7")
	req.Header.Set("Accept-Language", "en-GB,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	logger.WithFields(logrus.Fields{
		"url":               targetURL,
		"status_code":       resp.StatusCode,
		"content_type":      resp.Header.Get("Content-Type"),
		"content_length":    resp.Header.Get("Content-Length"),
		"content_encoding":  resp.Header.Get("Content-Encoding"),
		"transfer_encoding": resp.Header.Get("Transfer-Encoding"),
		"all_headers":       resp.Header,
	}).Debug("Received HTTP response")

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return &FetchURLResponse{
			URL:              targetURL,
			ContentType:      resp.Header.Get("Content-Type"),
			StatusCode:       resp.StatusCode,
			Content:          "",
			Truncated:        false,
			StartIndex:       0,
			EndIndex:         0,
			TotalLength:      0,
			TotalLines:       0,
			StartLine:        0,
			EndLine:          0,
			NextChunkPreview: "",
			RemainingLines:   0,
			Message:          fmt.Sprintf("HTTP error %d: %s", resp.StatusCode, resp.Status),
		}, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	// Limit response size to prevent memory issues
	// Note: Go's HTTP client automatically handles gzip decompression
	limitedReader := io.LimitReader(resp.Body, MaxContentSize)

	// Read the response body
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle gzip decompression manually if needed
	// Go's HTTP client should do this automatically, but sometimes it doesn't work properly
	contentEncoding := resp.Header.Get("Content-Encoding")
	if contentEncoding == "gzip" {
		// Check if content looks like it's still compressed (binary)
		if len(body) > 0 && !utf8.Valid(body) {
			logger.Info("Manually decompressing gzip content")
			decompressedBody, err := decompressGzip(body)
			if err != nil {
				logger.WithError(err).Warn("Failed to decompress gzip content, using raw body")
			} else {
				body = decompressedBody
				logger.WithField("original_size", len(body)).WithField("decompressed_size", len(decompressedBody)).Info("Successfully decompressed gzip content")
			}
		}
	}

	// Ensure content is valid UTF-8, replace invalid sequences
	if !utf8.Valid(body) {
		logger.Debug("Content contains invalid UTF-8, cleaning up")
		body = []byte(strings.ToValidUTF8(string(body), "�"))
	}

	logger.WithFields(logrus.Fields{
		"url":         targetURL,
		"body_size":   len(body),
		"status_code": resp.StatusCode,
	}).Debug("Successfully fetched content")

	response := &FetchURLResponse{
		URL:              targetURL,
		Content:          string(body),
		Truncated:        false, // Will be set later during pagination
		StartIndex:       0,
		EndIndex:         len(body),
		TotalLength:      len(body),
		TotalLines:       len(strings.Split(string(body), "\n")),
		StartLine:        1,
		EndLine:          len(strings.Split(string(body), "\n")),
		NextChunkPreview: "",
		RemainingLines:   0,
		Message:          "",
	}

	// Only include ContentType and StatusCode if they're not defaults
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		response.ContentType = contentType
	}
	if resp.StatusCode != 200 {
		response.StatusCode = resp.StatusCode
	}

	return response, nil
}

// DetectContentType analyses the content type and determines how to process it
func DetectContentType(contentType, content string) ContentTypeInfo {
	ct := strings.ToLower(strings.TrimSpace(contentType))

	// Split content type to remove charset and other parameters
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = strings.TrimSpace(ct[:idx])
	}

	info := ContentTypeInfo{
		MIME: ct,
	}

	// Check for HTML content
	if strings.Contains(ct, "text/html") ||
		strings.Contains(ct, "application/xhtml") ||
		strings.Contains(content[:min(500, len(content))], "<html") ||
		strings.Contains(content[:min(500, len(content))], "<!DOCTYPE html") {
		info.IsHTML = true
		info.IsText = true
		return info
	}

	// Check for text content
	if strings.HasPrefix(ct, "text/") ||
		strings.Contains(ct, "application/json") ||
		strings.Contains(ct, "application/xml") ||
		strings.Contains(ct, "application/javascript") {
		info.IsText = true
		return info
	}

	// If no content type specified, try to detect from content
	if ct == "" {
		// Simple heuristic: if it contains common HTML tags, treat as HTML
		lowerContent := strings.ToLower(content[:min(1000, len(content))])
		if strings.Contains(lowerContent, "<html") ||
			strings.Contains(lowerContent, "<head") ||
			strings.Contains(lowerContent, "<body") ||
			strings.Contains(lowerContent, "<!doctype") {
			info.IsHTML = true
			info.IsText = true
			return info
		}

		// If it looks like text (printable ASCII), treat as text
		isPrintable := true
		for i, r := range content[:min(1000, len(content))] {
			if r < 32 && r != 9 && r != 10 && r != 13 { // Allow tab, newline, carriage return
				isPrintable = false
				break
			}
			if i > 1000 {
				break
			}
		}

		if isPrintable {
			info.IsText = true
			return info
		}
	}

	// Default to binary
	info.IsBinary = true
	return info
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

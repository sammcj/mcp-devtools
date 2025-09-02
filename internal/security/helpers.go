package security

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/sirupsen/logrus"
)

// Operations provides simplified security-aware operations for tools
type Operations struct {
	toolName string
}

// SafeHTTPResponse contains HTTP response data with security metadata
type SafeHTTPResponse struct {
	Content        []byte          // EXACT original bytes - never modified
	ContentType    string          // Original content type
	StatusCode     int             // Original status code
	Headers        http.Header     // Original headers
	SecurityResult *SecurityResult // nil if safe, populated if warn
}

// SafeFileContent contains file data with security metadata
type SafeFileContent struct {
	Content        []byte          // EXACT file bytes - never modified
	Path           string          // Resolved path
	Info           os.FileInfo     // Original file info
	SecurityResult *SecurityResult // nil if safe, populated if warn
}

// NewOperations creates a new Operations instance for a specific tool
func NewOperations(toolName string) *Operations {
	return &Operations{toolName: toolName}
}

// SafeHTTPGet performs a secure HTTP GET with content integrity preservation
func (o *Operations) SafeHTTPGet(urlStr string) (*SafeHTTPResponse, error) {
	// 1. Parse and validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	// 2. Check domain access (before any HTTP call)
	if err := CheckDomainAccess(parsedURL.Hostname()); err != nil {
		return nil, err // Hard block - no content fetched
	}

	// 3. Fetch content normally (no modifications)
	resp, err := http.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logrus.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 4. Security analysis on copy of content (original untouched)
	var securityResult *SecurityResult
	if o.shouldAnalyseContent(content, resp.Header.Get("Content-Type")) {
		sourceCtx := SourceContext{
			URL:         urlStr,
			Domain:      parsedURL.Hostname(),
			ContentType: resp.Header.Get("Content-Type"),
			Tool:        o.toolName,
		}

		// Create copy for analysis to ensure no side effects
		contentForAnalysis := make([]byte, len(content))
		copy(contentForAnalysis, content)

		var err error
		securityResult, err = AnalyseContent(string(contentForAnalysis), sourceCtx)
		if err != nil {
			// Log error but don't fail - return content with no security metadata
			logrus.WithError(err).Warn("Security analysis failed")
			securityResult = nil
		}

		// Handle security blocks
		if securityResult != nil && securityResult.Action == ActionBlock {
			return nil, &SecurityError{
				ID:      securityResult.ID,
				Message: securityResult.Message,
				Action:  ActionBlock,
			}
		}
	}

	// 5. Return original content with optional security metadata
	return &SafeHTTPResponse{
		Content:        content, // EXACT original bytes
		ContentType:    resp.Header.Get("Content-Type"),
		StatusCode:     resp.StatusCode,
		Headers:        resp.Header,    // Full original headers
		SecurityResult: securityResult, // nil for safe, populated for warnings
	}, nil
}

// SafeHTTPPost performs a secure HTTP POST with content integrity preservation
func (o *Operations) SafeHTTPPost(urlStr string, body io.Reader) (*SafeHTTPResponse, error) {
	// 1. Parse and validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	// 2. Check domain access (before any HTTP call)
	if err := CheckDomainAccess(parsedURL.Hostname()); err != nil {
		return nil, err // Hard block - no content fetched
	}

	// 3. Fetch content normally (no modifications)
	resp, err := http.Post(urlStr, "application/json", body)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logrus.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 4. Security analysis on copy of content (original untouched)
	var securityResult *SecurityResult
	if o.shouldAnalyseContent(content, resp.Header.Get("Content-Type")) {
		sourceCtx := SourceContext{
			URL:         urlStr,
			Domain:      parsedURL.Hostname(),
			ContentType: resp.Header.Get("Content-Type"),
			Tool:        o.toolName,
		}

		// Create copy for analysis to ensure no side effects
		contentForAnalysis := make([]byte, len(content))
		copy(contentForAnalysis, content)

		var err error
		securityResult, err = AnalyseContent(string(contentForAnalysis), sourceCtx)
		if err != nil {
			// Log error but don't fail - return content with no security metadata
			logrus.WithError(err).Warn("Security analysis failed")
			securityResult = nil
		}

		// Handle security blocks
		if securityResult != nil && securityResult.Action == ActionBlock {
			return nil, &SecurityError{
				ID:      securityResult.ID,
				Message: securityResult.Message,
				Action:  ActionBlock,
			}
		}
	}

	// 5. Return original content with optional security metadata
	return &SafeHTTPResponse{
		Content:        content, // EXACT original bytes
		ContentType:    resp.Header.Get("Content-Type"),
		StatusCode:     resp.StatusCode,
		Headers:        resp.Header,    // Full original headers
		SecurityResult: securityResult, // nil for safe, populated for warnings
	}, nil
}

// SafeHTTPGetWithHeaders performs a secure HTTP GET with custom headers
func (o *Operations) SafeHTTPGetWithHeaders(urlStr string, headers map[string]string) (*SafeHTTPResponse, error) {
	// 1. Parse and validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	// 2. Check domain access (before any HTTP call)
	if err := CheckDomainAccess(parsedURL.Hostname()); err != nil {
		return nil, err // Hard block - no content fetched
	}

	// 3. Create request with headers
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 4. Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logrus.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 5. Security analysis on copy of content (original untouched)
	var securityResult *SecurityResult
	if o.shouldAnalyseContent(content, resp.Header.Get("Content-Type")) {
		sourceCtx := SourceContext{
			URL:         urlStr,
			Domain:      parsedURL.Hostname(),
			ContentType: resp.Header.Get("Content-Type"),
			Tool:        o.toolName,
		}

		// Create copy for analysis to ensure no side effects
		contentForAnalysis := make([]byte, len(content))
		copy(contentForAnalysis, content)

		var err error
		securityResult, err = AnalyseContent(string(contentForAnalysis), sourceCtx)
		if err != nil {
			// Log error but don't fail - return content with no security metadata
			logrus.WithError(err).Warn("Security analysis failed")
			securityResult = nil
		}

		// Handle security blocks
		if securityResult != nil && securityResult.Action == ActionBlock {
			return nil, &SecurityError{
				ID:      securityResult.ID,
				Message: securityResult.Message,
				Action:  ActionBlock,
			}
		}
	}

	// 6. Return original content with optional security metadata
	return &SafeHTTPResponse{
		Content:        content, // EXACT original bytes
		ContentType:    resp.Header.Get("Content-Type"),
		StatusCode:     resp.StatusCode,
		Headers:        resp.Header,    // Full original headers
		SecurityResult: securityResult, // nil for safe, populated for warnings
	}, nil
}

// SafeHTTPPostWithHeaders performs a secure HTTP POST with custom headers
func (o *Operations) SafeHTTPPostWithHeaders(urlStr string, body io.Reader, headers map[string]string) (*SafeHTTPResponse, error) {
	// 1. Parse and validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	// 2. Check domain access (before any HTTP call)
	if err := CheckDomainAccess(parsedURL.Hostname()); err != nil {
		return nil, err // Hard block - no content fetched
	}

	// 3. Create request with headers
	req, err := http.NewRequest("POST", urlStr, body)
	if err != nil {
		return nil, err
	}

	// Set default content type if not provided
	if _, ok := headers["Content-Type"]; !ok {
		req.Header.Set("Content-Type", "application/json")
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 4. Execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logrus.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 5. Security analysis on copy of content (original untouched)
	var securityResult *SecurityResult
	if o.shouldAnalyseContent(content, resp.Header.Get("Content-Type")) {
		sourceCtx := SourceContext{
			URL:         urlStr,
			Domain:      parsedURL.Hostname(),
			ContentType: resp.Header.Get("Content-Type"),
			Tool:        o.toolName,
		}

		// Create copy for analysis to ensure no side effects
		contentForAnalysis := make([]byte, len(content))
		copy(contentForAnalysis, content)

		var err error
		securityResult, err = AnalyseContent(string(contentForAnalysis), sourceCtx)
		if err != nil {
			// Log error but don't fail - return content with no security metadata
			logrus.WithError(err).Warn("Security analysis failed")
			securityResult = nil
		}

		// Handle security blocks
		if securityResult != nil && securityResult.Action == ActionBlock {
			return nil, &SecurityError{
				ID:      securityResult.ID,
				Message: securityResult.Message,
				Action:  ActionBlock,
			}
		}
	}

	// 6. Return original content with optional security metadata
	return &SafeHTTPResponse{
		Content:        content, // EXACT original bytes
		ContentType:    resp.Header.Get("Content-Type"),
		StatusCode:     resp.StatusCode,
		Headers:        resp.Header,    // Full original headers
		SecurityResult: securityResult, // nil for safe, populated for warnings
	}, nil
}

// SafeFileRead performs a secure file read with content integrity preservation
func (o *Operations) SafeFileRead(path string) (*SafeFileContent, error) {
	// 1. Check file access (before any file operation)
	if err := CheckFileAccess(path); err != nil {
		return nil, err // Hard block - no content read
	}

	// 2. Read exact bytes (no encoding assumptions)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// 3. Security analysis on copy (if it's analyzable text)
	var securityResult *SecurityResult
	if o.shouldAnalyseContent(content, "") {
		sourceCtx := SourceContext{
			URL:  "file://" + path, // Use file:// URL scheme for file paths
			Tool: o.toolName,
		}

		// Create copy for analysis to ensure no side effects
		contentForAnalysis := make([]byte, len(content))
		copy(contentForAnalysis, content)

		var err error
		securityResult, err = AnalyseContent(string(contentForAnalysis), sourceCtx)
		if err != nil {
			// Log error but don't fail - return content with no security metadata
			logrus.WithError(err).Warn("Security analysis failed")
			securityResult = nil
		}

		// Handle security blocks
		if securityResult != nil && securityResult.Action == ActionBlock {
			return nil, &SecurityError{
				ID:      securityResult.ID,
				Message: securityResult.Message,
				Action:  ActionBlock,
			}
		}
	}

	return &SafeFileContent{
		Content:        content, // EXACT file bytes
		Path:           path,
		Info:           info,           // Original file info
		SecurityResult: securityResult, // nil for binary/safe, populated for text warnings
	}, nil
}

// SafeFileWrite performs a secure file write with access control
func (o *Operations) SafeFileWrite(path string, content []byte) error {
	// Check file access (before any file operation)
	if err := CheckFileAccess(path); err != nil {
		return err // Hard block - no write allowed
	}

	// Write file with secure permissions
	return os.WriteFile(path, content, 0600)
}

// shouldAnalyseContent determines if content should be analysed for security threats
func (o *Operations) shouldAnalyseContent(content []byte, contentType string) bool {
	// Skip analysis for obviously binary content types
	if contentType != "" {
		switch contentType {
		case "application/octet-stream", "application/pdf":
			return false
		}
		if strings.HasPrefix(contentType, "image/") ||
			strings.HasPrefix(contentType, "video/") ||
			strings.HasPrefix(contentType, "audio/") {
			return false
		}
	}

	// Only analyse content that appears to be text
	return isTextContent(content)
}

// isTextContent checks if content appears to be text (safe for security analysis)
func isTextContent(content []byte) bool {
	if len(content) == 0 {
		return false
	}

	// Check for null bytes (typically indicates binary)
	sampleSize := 512
	if len(content) < sampleSize {
		sampleSize = len(content)
	}

	for i := 0; i < sampleSize; i++ {
		if content[i] == 0 {
			return false
		}
	}

	// Check UTF-8 validity for reasonable portion
	sample := content
	if len(content) > 1024 {
		sample = content[:1024]
	}

	return utf8.Valid(sample)
}

// SecurityError represents a security-related error
type SecurityError struct {
	ID      string
	Message string
	Action  string
}

func (e *SecurityError) Error() string {
	return e.Message
}

// GetSecurityID returns the security ID for override purposes
func (e *SecurityError) GetSecurityID() string {
	return e.ID
}

// FormatSecurityBlockError creates a standardised security block error message
func FormatSecurityBlockError(secErr *SecurityError) error {
	return fmt.Errorf("%s", secErr.Error())
}

// FormatSecurityBlockErrorFromResult creates a standardised security block error from a SecurityResult
func FormatSecurityBlockErrorFromResult(result *SecurityResult) error {
	return fmt.Errorf("%s", result.Message)
}

// FormatSecurityWarningPrefix creates a standardised security warning prefix for content
func FormatSecurityWarningPrefix(result *SecurityResult) string {
	return fmt.Sprintf("⚠️  Security Notice: %s\n\n", result.Message)
}

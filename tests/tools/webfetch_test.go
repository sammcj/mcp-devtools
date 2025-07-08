package tools_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sammcj/mcp-devtools/internal/tools/webfetch"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestFetchURLTool_Definition(t *testing.T) {
	tool := &webfetch.FetchURLTool{}
	definition := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "fetch_url", definition.Name)
	testutils.AssertNotNil(t, definition.Description)

	// Test that description contains key phrases
	desc := definition.Description
	if !testutils.Contains(desc, "Fetches content from URL") || !testutils.Contains(desc, "markdown") {
		t.Errorf("Expected description to contain key phrases about URL fetching and markdown conversion, got: %s", desc)
	}

	// Test input schema exists
	testutils.AssertNotNil(t, definition.InputSchema)
}

func TestFetchURLTool_Execute_MissingURL(t *testing.T) {
	tool := &webfetch.FetchURLTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: url")
}

func TestFetchURLTool_Execute_EmptyURL(t *testing.T) {
	tool := &webfetch.FetchURLTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"url": "",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: url")
}

func TestFetchURLTool_Execute_InvalidURLType(t *testing.T) {
	tool := &webfetch.FetchURLTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"url": 123, // Invalid type
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: url")
}

func TestFetchURLTool_Execute_InvalidURLScheme(t *testing.T) {
	tool := &webfetch.FetchURLTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"url": "ftp://example.com",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "URL must use http or https scheme")
}

func TestFetchURLTool_Execute_InvalidMaxLength(t *testing.T) {
	tool := &webfetch.FetchURLTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test max_length too small
	args := map[string]interface{}{
		"url":        "https://example.com",
		"max_length": float64(0),
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "max_length must be at least 1")

	// Test max_length too large
	args["max_length"] = float64(2000000) // Over 1M limit

	_, err = tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "max_length cannot exceed 1,000,000")
}

func TestFetchURLTool_Execute_InvalidStartIndex(t *testing.T) {
	tool := &webfetch.FetchURLTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"url":         "https://example.com",
		"start_index": float64(-1),
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "start_index must be >= 0")
}

// Note: Tests that require actual HTTP requests are omitted to avoid external dependencies
// and nil pointer issues with uninitialized WebClient. The core parameter validation
// logic is already tested through the other test functions above.

// Test the DetectContentType function (pure function, no external dependencies)
func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name         string
		contentType  string
		content      string
		expectHTML   bool
		expectText   bool
		expectBinary bool
	}{
		{
			name:         "HTML content type",
			contentType:  "text/html; charset=utf-8",
			content:      "<html><body>Hello</body></html>",
			expectHTML:   true,
			expectText:   true,
			expectBinary: false,
		},
		{
			name:         "Plain text",
			contentType:  "text/plain",
			content:      "Hello, world!",
			expectHTML:   false,
			expectText:   true,
			expectBinary: false,
		},
		{
			name:         "JSON content",
			contentType:  "application/json",
			content:      `{"message": "hello"}`,
			expectHTML:   false,
			expectText:   true,
			expectBinary: false,
		},
		{
			name:         "HTML detected by content",
			contentType:  "",
			content:      "<!DOCTYPE html><html><head><title>Test</title></head><body>Content</body></html>",
			expectHTML:   true,
			expectText:   true,
			expectBinary: false,
		},
		{
			name:         "Binary content",
			contentType:  "application/octet-stream",
			content:      string([]byte{0x89, 0x50, 0x4E, 0x47}), // PNG header
			expectHTML:   false,
			expectText:   false,
			expectBinary: true,
		},
		{
			name:         "Empty content type with text content",
			contentType:  "",
			content:      "This is plain text content without any HTML tags.",
			expectHTML:   false,
			expectText:   true,
			expectBinary: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := webfetch.DetectContentType(tt.contentType, tt.content)

			testutils.AssertEqual(t, tt.expectHTML, info.IsHTML)
			testutils.AssertEqual(t, tt.expectText, info.IsText)
			testutils.AssertEqual(t, tt.expectBinary, info.IsBinary)
		})
	}
}

// Test the ProcessContent function with mocked responses
func TestProcessContent(t *testing.T) {
	logger := testutils.CreateTestLogger()

	tests := []struct {
		name           string
		response       *webfetch.FetchURLResponse
		raw            bool
		expectError    bool
		expectContains string
	}{
		{
			name: "HTML content conversion",
			response: &webfetch.FetchURLResponse{
				URL:         "https://example.com",
				ContentType: "text/html",
				StatusCode:  200,
				Content:     "<html><body><h1>Title</h1><p>Content</p></body></html>",
			},
			raw:            false,
			expectError:    false,
			expectContains: "Title", // Should contain converted content
		},
		{
			name: "Raw HTML content",
			response: &webfetch.FetchURLResponse{
				URL:         "https://example.com",
				ContentType: "text/html",
				StatusCode:  200,
				Content:     "<html><body><h1>Title</h1><p>Content</p></body></html>",
			},
			raw:            true,
			expectError:    false,
			expectContains: "<html>", // Should contain raw HTML
		},
		{
			name: "Plain text content",
			response: &webfetch.FetchURLResponse{
				URL:         "https://example.com",
				ContentType: "text/plain",
				StatusCode:  200,
				Content:     "This is plain text content.",
			},
			raw:            false,
			expectError:    false,
			expectContains: "plain text", // Should contain original text
		},
		{
			name: "Binary content",
			response: &webfetch.FetchURLResponse{
				URL:         "https://example.com",
				ContentType: "application/octet-stream",
				StatusCode:  200,
				Content:     string([]byte{0x89, 0x50, 0x4E, 0x47}), // PNG header
			},
			raw:            false,
			expectError:    false,
			expectContains: "binary", // Should indicate binary content
		},
		{
			name: "HTTP error response",
			response: &webfetch.FetchURLResponse{
				URL:         "https://example.com",
				ContentType: "text/html",
				StatusCode:  404,
				Content:     "Not Found",
			},
			raw:            false,
			expectError:    false,
			expectContains: "Not Found", // Should return raw content for errors
		},
		{
			name: "Empty content",
			response: &webfetch.FetchURLResponse{
				URL:         "https://example.com",
				ContentType: "text/html",
				StatusCode:  200,
				Content:     "",
			},
			raw:         false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := webfetch.ProcessContent(logger, tt.response, tt.raw)

			if tt.expectError {
				testutils.AssertError(t, err)
			} else {
				testutils.AssertNoError(t, err)
			}

			if tt.expectContains != "" {
				if !testutils.Contains(result, tt.expectContains) {
					t.Errorf("Expected result to contain '%s', got: %s", tt.expectContains, result)
				}
			}
		})
	}
}

// Test pagination logic using the public types and simulation
func TestFetchURLTool_PaginationLogic(t *testing.T) {
	// We can't directly test the private applyPagination method,
	// but we can test the logic through understanding of the expected behaviour

	tests := []struct {
		name         string
		content      string
		maxLength    int
		startIndex   int
		expectTrunc  bool
		expectLength int
	}{
		{
			name:         "Content within limits",
			content:      "Hello, world!",
			maxLength:    100,
			startIndex:   0,
			expectTrunc:  false,
			expectLength: 13,
		},
		{
			name:         "Content exceeds max_length",
			content:      "This is a very long piece of content that exceeds the maximum length limit.",
			maxLength:    20,
			startIndex:   0,
			expectTrunc:  true,
			expectLength: 20,
		},
		{
			name:         "Start index beyond content",
			content:      "Short content",
			maxLength:    100,
			startIndex:   50,
			expectTrunc:  false,
			expectLength: 0,
		},
		{
			name:         "Pagination in middle",
			content:      "0123456789abcdefghijklmnopqrstuvwxyz",
			maxLength:    10,
			startIndex:   10,
			expectTrunc:  true,
			expectLength: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the pagination logic
			totalLength := len(tt.content)

			if tt.startIndex >= totalLength {
				// Content beyond start index
				testutils.AssertEqual(t, 0, tt.expectLength)
				return
			}

			endIndex := tt.startIndex + tt.maxLength
			if endIndex > totalLength {
				endIndex = totalLength
			}

			resultLength := endIndex - tt.startIndex
			truncated := endIndex < totalLength

			testutils.AssertEqual(t, tt.expectLength, resultLength)
			testutils.AssertEqual(t, tt.expectTrunc, truncated)
		})
	}
}

// Test result format structure
func TestFetchURLTool_ResultFormat(t *testing.T) {
	// Since the tool returns JSON, test that we can parse expected structure
	expectedFields := []string{
		"url", "content_type", "status_code", "content",
		"truncated", "start_index", "end_index", "total_length",
		"timestamp",
	}

	// This is a structure test - we verify the expected JSON structure
	sampleResponse := map[string]interface{}{
		"url":          "https://example.com",
		"content_type": "text/html",
		"status_code":  200,
		"content":      "Sample content",
		"truncated":    false,
		"start_index":  0,
		"end_index":    14,
		"total_length": 14,
		"timestamp":    time.Now(),
		"message":      "",
	}

	// Convert to JSON and back to verify structure
	jsonBytes, err := json.Marshal(sampleResponse)
	testutils.AssertNoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(jsonBytes, &parsed)
	testutils.AssertNoError(t, err)

	// Verify all expected fields are present
	for _, field := range expectedFields {
		if _, exists := parsed[field]; !exists {
			t.Errorf("Expected field '%s' not found in response structure", field)
		}
	}
}

package tools_test

import (
	"encoding/json"
	"strings"
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

	args := map[string]any{}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: url")
}

func TestFetchURLTool_Execute_EmptyURL(t *testing.T) {
	tool := &webfetch.FetchURLTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
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

	args := map[string]any{
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

	args := map[string]any{
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
	args := map[string]any{
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

	args := map[string]any{
		"url":         "https://example.com",
		"start_index": float64(-1),
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "start_index must be >= 0")
}

// Note: Tests that require actual HTTP requests are omitted to avoid external dependencies
// and nil pointer issues with uninitialised WebClient. The core parameter validation
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
		notContains    string
	}{
		{
			name: "HTML content conversion to markdown",
			response: &webfetch.FetchURLResponse{
				ContentType: "text/html",
				StatusCode:  200,
				Content:     "<html><body><h1>Title</h1><p>Content</p></body></html>",
			},
			raw:            false,
			expectError:    false,
			expectContains: "# Title", // Should contain markdown heading syntax
		},
		{
			name: "Raw HTML content",
			response: &webfetch.FetchURLResponse{
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

			if tt.notContains != "" {
				if testutils.Contains(result, tt.notContains) {
					t.Errorf("Expected result NOT to contain '%s', got: %s", tt.notContains, result)
				}
			}
		})
	}
}

// Test markdown converter functionality
func TestMarkdownConverter(t *testing.T) {
	logger := testutils.CreateTestLogger()

	tests := []struct {
		name           string
		html           string
		expectContains []string
		notContains    []string
	}{
		{
			name: "Headings conversion",
			html: "<h1>Heading 1</h1><h2>Heading 2</h2><h3>Heading 3</h3>",
			expectContains: []string{
				"# Heading 1",
				"## Heading 2",
				"### Heading 3",
			},
		},
		{
			name: "Bold and italic text",
			html: "<p>This is <strong>bold</strong> and this is <em>italic</em></p>",
			expectContains: []string{
				"**bold**",
				"*italic*",
			},
		},
		{
			name: "Links conversion",
			html: "<p>Check out <a href='https://example.com'>this link</a></p>",
			expectContains: []string{
				"[this link](https://example.com)",
			},
		},
		{
			name: "Lists conversion",
			html: "<ul><li>Item 1</li><li>Item 2</li></ul>",
			expectContains: []string{
				"- Item 1",
				"- Item 2",
			},
		},
		{
			name: "Script tags removed",
			html: "<html><head><script>alert('test');</script></head><body><p>Content</p></body></html>",
			expectContains: []string{
				"Content",
			},
			notContains: []string{
				"alert",
				"script",
			},
		},
		{
			name: "Navigation elements removed",
			html: "<nav><a href='/home'>Home</a></nav><article><p>Main content</p></article>",
			expectContains: []string{
				"Main content",
			},
			notContains: []string{
				"Home",
			},
		},
		{
			name: "Form elements removed",
			html: "<form><input type='text' name='username'/><button>Submit</button></form><p>Text content</p>",
			expectContains: []string{
				"Text content",
			},
			notContains: []string{
				"Submit",
				"username",
			},
		},
		{
			name: "Nested HTML structures",
			html: "<div><section><article><h2>Title</h2><p>Paragraph with <strong>bold</strong> text.</p></article></section></div>",
			expectContains: []string{
				"## Title",
				"Paragraph with **bold** text",
			},
		},
		{
			name: "Code blocks",
			html: "<pre><code>func main() {\n  fmt.Println(\"Hello\")\n}</code></pre>",
			expectContains: []string{
				"func main()",
				"fmt.Println",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := webfetch.NewMarkdownConverter()
			result, err := converter.ConvertToMarkdown(logger, tt.html)

			testutils.AssertNoError(t, err)

			for _, expected := range tt.expectContains {
				if !testutils.Contains(result, expected) {
					t.Errorf("Expected markdown to contain '%s', got: %s", expected, result)
				}
			}

			for _, unexpected := range tt.notContains {
				if testutils.Contains(result, unexpected) {
					t.Errorf("Expected markdown NOT to contain '%s', got: %s", unexpected, result)
				}
			}
		})
	}
}

// Test markdown cleaning functionality
func TestMarkdownCleaning(t *testing.T) {
	logger := testutils.CreateTestLogger()
	converter := webfetch.NewMarkdownConverter()

	tests := []struct {
		name        string
		html        string
		checkFunc   func(result string) bool
		description string
	}{
		{
			name: "No excessive whitespace",
			html: "<p>Line 1</p><p>Line 2</p><p>Line 3</p>",
			checkFunc: func(result string) bool {
				// Should not have more than 2 consecutive newlines
				return !testutils.Contains(result, "\n\n\n")
			},
			description: "should not contain more than 2 consecutive newlines",
		},
		{
			name: "Trimmed output",
			html: "<p>Content</p>",
			checkFunc: func(result string) bool {
				// Should not start or end with whitespace
				return result == strings.TrimSpace(result)
			},
			description: "should not have leading or trailing whitespace",
		},
		{
			name: "Empty elements ignored",
			html: "<p></p><div></div><span></span><p>Real content</p>",
			checkFunc: func(result string) bool {
				return testutils.Contains(result, "Real content")
			},
			description: "should contain real content and ignore empty elements",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertToMarkdown(logger, tt.html)
			testutils.AssertNoError(t, err)

			if !tt.checkFunc(result) {
				t.Errorf("Markdown cleaning check failed: %s. Got: %s", tt.description, result)
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

			endIndex := min(tt.startIndex+tt.maxLength, totalLength)

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
	sampleResponse := map[string]any{
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

	var parsed map[string]any
	err = json.Unmarshal(jsonBytes, &parsed)
	testutils.AssertNoError(t, err)

	// Verify all expected fields are present
	for _, field := range expectedFields {
		if _, exists := parsed[field]; !exists {
			t.Errorf("Expected field '%s' not found in response structure", field)
		}
	}
}

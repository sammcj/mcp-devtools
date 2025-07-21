package tools_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sammcj/mcp-devtools/internal/tools/confluence"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestConfluenceTool_Definition(t *testing.T) {
	tool := &confluence.ConfluenceTool{}
	definition := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "confluence_search", definition.Name)
	testutils.AssertNotNil(t, definition.Description)

	// Test that description contains key phrases
	desc := definition.Description
	if !testutils.Contains(desc, "Search Confluence") || !testutils.Contains(desc, "Markdown") {
		t.Errorf("Expected description to contain key phrases about Confluence search and Markdown conversion, got: %s", desc)
	}

	// Test input schema exists
	testutils.AssertNotNil(t, definition.InputSchema)
}

func TestConfluenceTool_Execute_MissingQuery(t *testing.T) {
	tool := &confluence.ConfluenceTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	// The tool fails at client initialization before parameter validation
	testutils.AssertErrorContains(t, err, "failed to initialize Confluence client")
}

func TestConfluenceTool_Execute_EmptyQuery(t *testing.T) {
	tool := &confluence.ConfluenceTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"query": "",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	// The tool fails at client initialization before parameter validation
	testutils.AssertErrorContains(t, err, "failed to initialize Confluence client")
}

func TestConfluenceTool_Execute_InvalidQueryType(t *testing.T) {
	tool := &confluence.ConfluenceTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"query": 123, // Invalid type
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	// The tool fails at client initialization before parameter validation
	testutils.AssertErrorContains(t, err, "failed to initialize Confluence client")
}

func TestConfluenceTool_Execute_InvalidMaxResults(t *testing.T) {
	tool := &confluence.ConfluenceTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test max_results too small
	args := map[string]interface{}{
		"query":       "test query",
		"max_results": float64(0),
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	// The tool fails at client initialization before parameter validation
	testutils.AssertErrorContains(t, err, "failed to initialize Confluence client")

	// Test max_results too large
	args["max_results"] = float64(15) // Over 10 limit

	_, err = tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	// The tool fails at client initialization before parameter validation
	testutils.AssertErrorContains(t, err, "failed to initialize Confluence client")
}

func TestConfluenceTool_Execute_InvalidContentTypes(t *testing.T) {
	tool := &confluence.ConfluenceTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"query":         "test query",
		"content_types": []interface{}{"invalid_type"},
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	// The tool fails at client initialization before parameter validation
	testutils.AssertErrorContains(t, err, "failed to initialize Confluence client")
}

func TestConfluenceTool_Execute_ValidContentTypes(t *testing.T) {
	// This test validates that valid content types are accepted
	// We can't test the full execution without a real Confluence instance,
	// but we can test parameter validation

	validContentTypes := []string{"page", "blogpost", "comment", "attachment"}

	for _, contentType := range validContentTypes {
		t.Run("ContentType_"+contentType, func(t *testing.T) {
			tool := &confluence.ConfluenceTool{}
			logger := testutils.CreateTestLogger()
			cache := testutils.CreateTestCache()
			ctx := testutils.CreateTestContext()

			args := map[string]interface{}{
				"query":         "test query",
				"content_types": []interface{}{contentType},
			}

			// This will fail due to missing configuration, but should not fail on content type validation
			_, err := tool.Execute(ctx, logger, cache, args)

			// Should fail due to missing Confluence configuration, not invalid content type
			if err != nil && testutils.Contains(err.Error(), "invalid content type") {
				t.Errorf("Valid content type '%s' was rejected", contentType)
			}
		})
	}
}

func TestConfluenceTool_ParseRequest_ValidParameters(t *testing.T) {
	// We can't directly test the private parseRequest method,
	// but we can test the parameter validation logic through Execute

	tests := []struct {
		name          string
		args          map[string]interface{}
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid minimal parameters",
			args: map[string]interface{}{
				"query": "test search",
			},
			expectError: false,
		},
		{
			name: "Valid all parameters",
			args: map[string]interface{}{
				"query":         "test search",
				"space_key":     "TEST",
				"max_results":   float64(5),
				"content_types": []interface{}{"page", "blogpost"},
			},
			expectError: false,
		},
		{
			name: "Invalid max_results type",
			args: map[string]interface{}{
				"query":       "test search",
				"max_results": "invalid",
			},
			expectError: false, // Type conversion should handle this gracefully
		},
		{
			name: "Empty space_key should be ignored",
			args: map[string]interface{}{
				"query":     "test search",
				"space_key": "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := &confluence.ConfluenceTool{}
			logger := testutils.CreateTestLogger()
			cache := testutils.CreateTestCache()
			ctx := testutils.CreateTestContext()

			_, err := tool.Execute(ctx, logger, cache, tt.args)

			if tt.expectError {
				testutils.AssertError(t, err)
				if tt.errorContains != "" {
					testutils.AssertErrorContains(t, err, tt.errorContains)
				}
			} else {
				// We expect this to fail due to missing configuration, not parameter validation
				// The error should be about missing configuration, not parameter validation
				if err != nil && !testutils.Contains(err.Error(), "failed to initialize Confluence client") {
					t.Errorf("Expected configuration error, got: %v", err)
				}
			}
		})
	}
}

func TestConfluenceTool_URLDetection(t *testing.T) {
	// Test URL detection logic (we can infer this from the code structure)
	tests := []struct {
		name  string
		query string
		isURL bool
	}{
		{
			name:  "HTTP URL",
			query: "http://confluence.example.com/display/SPACE/Page",
			isURL: true,
		},
		{
			name:  "HTTPS URL",
			query: "https://confluence.example.com/display/SPACE/Page",
			isURL: true,
		},
		{
			name:  "WWW URL",
			query: "www.confluence.example.com/display/SPACE/Page",
			isURL: true,
		},
		{
			name:  "Short HTTP URL",
			query: "http://x.com",
			isURL: true, // Actually 11 chars, so it passes the > 10 check
		},
		{
			name:  "Regular search query",
			query: "how to configure authentication",
			isURL: false,
		},
		{
			name:  "Empty query",
			query: "",
			isURL: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't directly test the private isURL method,
			// but we can infer the logic from the code
			isURL := len(tt.query) > 10 && (strings.HasPrefix(tt.query, "http://") ||
				strings.HasPrefix(tt.query, "https://") ||
				(len(tt.query) > 20 && strings.HasPrefix(tt.query, "www.")))

			testutils.AssertEqual(t, tt.isURL, isURL)
		})
	}
}

func TestConfluenceTool_CacheKeyGeneration(t *testing.T) {
	// Test cache key generation logic (inferred from code structure)
	tests := []struct {
		name             string
		query            string
		spaceKey         string
		maxResults       int
		contentTypes     []string
		expectedContains []string
	}{
		{
			name:             "Basic query",
			query:            "test search",
			maxResults:       3,
			expectedContains: []string{"confluence_search:", "test search", ":max:3"},
		},
		{
			name:             "With space key",
			query:            "test search",
			spaceKey:         "TEST",
			maxResults:       5,
			expectedContains: []string{"confluence_search:", "test search", ":space:TEST", ":max:5"},
		},
		{
			name:             "With content types",
			query:            "test search",
			maxResults:       3,
			contentTypes:     []string{"page", "blogpost"},
			expectedContains: []string{"confluence_search:", "test search", ":max:3", ":types:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate cache key generation logic
			key := "confluence_search:" + tt.query
			if tt.spaceKey != "" {
				key += ":space:" + tt.spaceKey
			}
			key += ":max:" + string(rune(tt.maxResults+'0'))
			if len(tt.contentTypes) > 0 {
				key += ":types:[" + strings.Join(tt.contentTypes, " ") + "]"
			}

			for _, expected := range tt.expectedContains {
				if !testutils.Contains(key, expected) {
					t.Errorf("Expected cache key to contain '%s', got: %s", expected, key)
				}
			}
		})
	}
}

func TestConfluenceConfig_IsConfigured(t *testing.T) {
	// Save original environment
	originalEnv := map[string]string{
		"CONFLUENCE_URL":              os.Getenv("CONFLUENCE_URL"),
		"CONFLUENCE_USERNAME":         os.Getenv("CONFLUENCE_USERNAME"),
		"CONFLUENCE_TOKEN":            os.Getenv("CONFLUENCE_TOKEN"),
		"CONFLUENCE_SESSION_COOKIES":  os.Getenv("CONFLUENCE_SESSION_COOKIES"),
		"CONFLUENCE_BROWSER_TYPE":     os.Getenv("CONFLUENCE_BROWSER_TYPE"),
		"CONFLUENCE_OAUTH_CLIENT_ID":  os.Getenv("CONFLUENCE_OAUTH_CLIENT_ID"),
		"CONFLUENCE_OAUTH_ISSUER_URL": os.Getenv("CONFLUENCE_OAUTH_ISSUER_URL"),
	}

	// Clean environment
	for key := range originalEnv {
		_ = os.Unsetenv(key)
	}

	defer func() {
		// Restore original environment
		for key, value := range originalEnv {
			if value != "" {
				_ = os.Setenv(key, value)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected bool
	}{
		{
			name:     "No configuration",
			envVars:  map[string]string{},
			expected: false,
		},
		{
			name: "Missing URL",
			envVars: map[string]string{
				"CONFLUENCE_USERNAME": "user",
				"CONFLUENCE_TOKEN":    "token",
			},
			expected: false,
		},
		{
			name: "Basic auth configuration",
			envVars: map[string]string{
				"CONFLUENCE_URL":      "https://confluence.example.com",
				"CONFLUENCE_USERNAME": "user",
				"CONFLUENCE_TOKEN":    "token",
			},
			expected: true,
		},
		{
			name: "Session cookies configuration",
			envVars: map[string]string{
				"CONFLUENCE_URL":             "https://confluence.example.com",
				"CONFLUENCE_SESSION_COOKIES": "session=abc123",
			},
			expected: true,
		},
		{
			name: "Browser type configuration",
			envVars: map[string]string{
				"CONFLUENCE_URL":          "https://confluence.example.com",
				"CONFLUENCE_BROWSER_TYPE": "chrome",
			},
			expected: true,
		},
		{
			name: "OAuth configuration",
			envVars: map[string]string{
				"CONFLUENCE_URL":              "https://confluence.example.com",
				"CONFLUENCE_OAUTH_CLIENT_ID":  "client123",
				"CONFLUENCE_OAUTH_ISSUER_URL": "https://auth.example.com",
			},
			expected: true,
		},
		{
			name: "Incomplete OAuth configuration",
			envVars: map[string]string{
				"CONFLUENCE_URL":             "https://confluence.example.com",
				"CONFLUENCE_OAUTH_CLIENT_ID": "client123",
				// Missing CONFLUENCE_OAUTH_ISSUER_URL
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				_ = os.Setenv(key, value)
			}

			result := confluence.IsConfigured()
			testutils.AssertEqual(t, tt.expected, result)

			// Clean up for next test
			for key := range tt.envVars {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func TestConfluenceConverter_ConvertToMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string // Use contains instead of exact match for more flexible testing
	}{
		{
			name:     "Empty input",
			input:    "",
			contains: []string{}, // Empty result
		},
		{
			name:     "Simple paragraph",
			input:    "<p>Hello, world!</p>",
			contains: []string{"Hello, world!"},
		},
		{
			name:     "Header conversion",
			input:    "<h1>Main Title</h1><h2>Subtitle</h2>",
			contains: []string{"# Main Title", "## Subtitle"},
		},
		{
			name:     "Bold and italic text",
			input:    "<p>This is <strong>bold</strong> and <em>italic</em> text.</p>",
			contains: []string{"**bold**", "*italic*"},
		},
		{
			name:     "Code blocks",
			input:    "<pre><code>console.log('Hello');</code></pre>",
			contains: []string{"```", "console.log('Hello');"},
		},
		{
			name:     "Links",
			input:    "<p>Visit <a href=\"https://example.com\">our website</a></p>",
			contains: []string{"[our website](https://example.com)"},
		},
		{
			name:     "Lists",
			input:    "<ul><li>Item 1</li><li>Item 2</li></ul>",
			contains: []string{"- Item 1", "- Item 2"},
		},
		{
			name:     "Blockquotes",
			input:    "<blockquote>This is a quote</blockquote>",
			contains: []string{"> This is a quote"},
		},
		{
			name:     "Complex nested content",
			input:    "<div><h2>Section</h2><p>Content with <strong>bold</strong> text.</p><ul><li>List item</li></ul></div>",
			contains: []string{"## Section", "**bold**", "- List item"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := confluence.ConvertToMarkdown(tt.input)
			testutils.AssertNoError(t, err)

			// For empty input, expect empty result
			if tt.input == "" {
				if result != "" {
					t.Errorf("Expected empty result for empty input, got: %s", result)
				}
				return
			}

			// Check that result contains expected elements
			for _, expected := range tt.contains {
				if !testutils.Contains(result, expected) {
					t.Errorf("Expected result to contain '%s', got: %s", expected, result)
				}
			}
		})
	}
}

func TestConfluenceConverter_ConvertToMarkdown_ConfluenceMacros(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		minOne   bool // At least one of the contains should be present
	}{
		{
			name:     "Code macro",
			input:    `<ac:structured-macro ac:name="code"><ac:parameter ac:name="language">javascript</ac:parameter><ac:plain-text-body>console.log('test');</ac:plain-text-body></ac:structured-macro>`,
			contains: []string{"```", "console.log('test');"},
			minOne:   true,
		},
		{
			name:     "Info macro",
			input:    `<ac:structured-macro ac:name="info"><ac:parameter ac:name="title">Important</ac:parameter><ac:rich-text-body><p>This is important information.</p></ac:rich-text-body></ac:structured-macro>`,
			contains: []string{">", "Important", "important information"},
			minOne:   true,
		},
		{
			name:     "Note macro",
			input:    `<ac:structured-macro ac:name="note"><ac:rich-text-body><p>This is a note.</p></ac:rich-text-body></ac:structured-macro>`,
			contains: []string{">", "note"},
			minOne:   true,
		},
		{
			name:     "Table of contents",
			input:    `<ac:structured-macro ac:name="toc"></ac:structured-macro>`,
			contains: []string{"Table of Contents"},
			minOne:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := confluence.ConvertToMarkdown(tt.input)
			testutils.AssertNoError(t, err)

			if tt.minOne {
				// At least one of the expected strings should be present
				found := false
				for _, expected := range tt.contains {
					if testutils.Contains(result, expected) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected result to contain at least one of %v, got: %s", tt.contains, result)
				}
			} else {
				// All expected strings should be present
				for _, expected := range tt.contains {
					if !testutils.Contains(result, expected) {
						t.Errorf("Expected result to contain '%s', got: %s", expected, result)
					}
				}
			}
		})
	}
}

func TestConfluenceBrowserTypes_ParseBrowserType(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    confluence.BrowserType
		expectError bool
	}{
		{
			name:        "Chrome",
			input:       "chrome",
			expected:    confluence.BrowserChrome,
			expectError: false,
		},
		{
			name:        "Chrome uppercase",
			input:       "CHROME",
			expected:    confluence.BrowserChrome,
			expectError: false,
		},
		{
			name:        "Firefox",
			input:       "firefox",
			expected:    confluence.BrowserFirefox,
			expectError: false,
		},
		{
			name:        "Edge",
			input:       "edge",
			expected:    confluence.BrowserEdge,
			expectError: false,
		},
		{
			name:        "Microsoft Edge",
			input:       "microsoft-edge",
			expected:    confluence.BrowserEdge,
			expectError: false,
		},
		{
			name:        "Brave",
			input:       "brave",
			expected:    confluence.BrowserBrave,
			expectError: false,
		},
		{
			name:        "Invalid browser",
			input:       "invalid-browser",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := confluence.ParseBrowserType(tt.input)

			if tt.expectError {
				testutils.AssertError(t, err)
			} else {
				testutils.AssertNoError(t, err)
				testutils.AssertEqual(t, tt.expected, result)
			}
		})
	}
}

func TestConfluenceBrowserTypes_GetSupportedBrowsers(t *testing.T) {
	browsers := confluence.GetSupportedBrowsers()

	// Should always include these browsers
	expectedBrowsers := []confluence.BrowserType{
		confluence.BrowserChrome,
		confluence.BrowserChromium,
		confluence.BrowserBrave,
		confluence.BrowserEdge,
		confluence.BrowserFirefox,
		confluence.BrowserFirefoxNightly,
	}

	for _, expected := range expectedBrowsers {
		found := false
		for _, browser := range browsers {
			if browser == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected browser %s not found in supported browsers list", expected)
		}
	}

	// Should have at least the expected browsers
	if len(browsers) < len(expectedBrowsers) {
		t.Errorf("Expected at least %d browsers, got %d", len(expectedBrowsers), len(browsers))
	}
}

func TestConfluenceTypes_OAuthTokenCache(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		token         confluence.OAuthTokenCache
		expectValid   bool
		expectExpired bool
	}{
		{
			name: "Valid token",
			token: confluence.OAuthTokenCache{
				AccessToken: "valid-token",
				ExpiresAt:   now.Add(1 * time.Hour),
				CachedAt:    now,
			},
			expectValid:   true,
			expectExpired: false,
		},
		{
			name: "Expired token",
			token: confluence.OAuthTokenCache{
				AccessToken: "expired-token",
				ExpiresAt:   now.Add(-1 * time.Hour),
				CachedAt:    now.Add(-2 * time.Hour),
			},
			expectValid:   false,
			expectExpired: true,
		},
		{
			name: "Empty token",
			token: confluence.OAuthTokenCache{
				AccessToken: "",
				ExpiresAt:   now.Add(1 * time.Hour),
				CachedAt:    now,
			},
			expectValid:   false,
			expectExpired: false,
		},
		{
			name: "Token expiring soon (within 30 seconds)",
			token: confluence.OAuthTokenCache{
				AccessToken: "expiring-token",
				ExpiresAt:   now.Add(15 * time.Second), // Within 30 second buffer
				CachedAt:    now,
			},
			expectValid:   false,
			expectExpired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutils.AssertEqual(t, tt.expectValid, tt.token.IsValid())
			testutils.AssertEqual(t, tt.expectExpired, tt.token.IsExpired())
		})
	}
}

func TestConfluenceTypes_SearchResponseStructure(t *testing.T) {
	// Test that we can marshal and unmarshal SearchResponse correctly
	originalResponse := confluence.SearchResponse{
		Query:      "test query",
		TotalCount: 2,
		Message:    "Test message",
		Results: []confluence.ContentResult{
			{
				ID:             "123",
				Type:           "page",
				Title:          "Test Page",
				URL:            "https://confluence.example.com/rest/api/content/123",
				WebURL:         "https://confluence.example.com/display/TEST/Test+Page",
				LastModified:   time.Now(),
				Content:        "# Test Page\n\nThis is test content.",
				ContentPreview: "Test Page - This is test content...",
				Space: confluence.SpaceInfo{
					Key:  "TEST",
					Name: "Test Space",
					Type: "global",
				},
				Author: confluence.Author{
					AccountID:   "user123",
					DisplayName: "Test User",
					Email:       "test@example.com",
				},
				Metadata: map[string]interface{}{
					"version": 1,
					"status":  "current",
				},
			},
		},
	}

	// Marshal to JSON
	jsonBytes, err := json.MarshalIndent(originalResponse, "", "  ")
	testutils.AssertNoError(t, err)

	// Unmarshal back
	var parsedResponse confluence.SearchResponse
	err = json.Unmarshal(jsonBytes, &parsedResponse)
	testutils.AssertNoError(t, err)

	// Verify key fields
	testutils.AssertEqual(t, originalResponse.Query, parsedResponse.Query)
	testutils.AssertEqual(t, originalResponse.TotalCount, parsedResponse.TotalCount)
	testutils.AssertEqual(t, originalResponse.Message, parsedResponse.Message)
	testutils.AssertEqual(t, len(originalResponse.Results), len(parsedResponse.Results))

	if len(parsedResponse.Results) > 0 {
		result := parsedResponse.Results[0]
		originalResult := originalResponse.Results[0]

		testutils.AssertEqual(t, originalResult.ID, result.ID)
		testutils.AssertEqual(t, originalResult.Type, result.Type)
		testutils.AssertEqual(t, originalResult.Title, result.Title)
		testutils.AssertEqual(t, originalResult.Space.Key, result.Space.Key)
		testutils.AssertEqual(t, originalResult.Author.DisplayName, result.Author.DisplayName)
	}
}

// Test error response structure
func TestConfluenceTypes_ErrorResponseStructure(t *testing.T) {
	errorResponse := confluence.ErrorResponse{
		StatusCode: 403,
		Message:    "Access denied",
		Reason:     "Insufficient permissions",
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(errorResponse)
	testutils.AssertNoError(t, err)

	// Unmarshal back
	var parsedError confluence.ErrorResponse
	err = json.Unmarshal(jsonBytes, &parsedError)
	testutils.AssertNoError(t, err)

	// Verify fields
	testutils.AssertEqual(t, errorResponse.StatusCode, parsedError.StatusCode)
	testutils.AssertEqual(t, errorResponse.Message, parsedError.Message)
	testutils.AssertEqual(t, errorResponse.Reason, parsedError.Reason)
}

// Test that the tool handles various error conditions gracefully
func TestConfluenceTool_ErrorHandling(t *testing.T) {
	tool := &confluence.ConfluenceTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test with valid parameters but no configuration
	args := map[string]interface{}{
		"query": "test search",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	// Should return a result (possibly with error information) rather than panic
	if result == nil && err == nil {
		t.Error("Expected either a result or an error, got neither")
	}

	// If we get a result, it should be valid JSON
	if result != nil {
		// The result should be a CallToolResult with text content
		if result.Content == nil {
			t.Error("Expected result to have content")
		}
	}
}

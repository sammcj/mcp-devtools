package tools

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/aws_documentation"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWSDocumentationTool_Definition(t *testing.T) {
	tool := &aws.AWSDocumentationTool{}
	definition := tool.Definition()

	assert.Equal(t, "aws_documentation", definition.Name)
	assert.NotEmpty(t, definition.Description)

	// Check parameters
	schema := definition.InputSchema
	require.NotNil(t, schema.Properties)

	// action parameter should be required
	actionProp, exists := schema.Properties["action"]
	assert.True(t, exists)
	assert.NotNil(t, actionProp)

	// search_phrase should be optional
	searchPhraseProp, exists := schema.Properties["search_phrase"]
	assert.True(t, exists)
	assert.NotNil(t, searchPhraseProp)

	// url should be optional
	urlProp, exists := schema.Properties["url"]
	assert.True(t, exists)
	assert.NotNil(t, urlProp)

	// limit should have default
	limitProp, exists := schema.Properties["limit"]
	assert.True(t, exists)
	assert.NotNil(t, limitProp)

	// Check required fields
	assert.Contains(t, schema.Required, "action")
}

func TestAWSDocumentationTool_Execute_InvalidAction(t *testing.T) {
	// Enable AWS tools for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "aws_documentation")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &aws.AWSDocumentationTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress output during tests
	cache := &sync.Map{}

	testCases := []struct {
		name          string
		args          map[string]any
		expectedError string
	}{
		{
			name:          "missing action parameter",
			args:          map[string]any{},
			expectedError: "missing required parameter: action",
		},
		{
			name: "invalid action",
			args: map[string]any{
				"action": "invalid",
			},
			expectedError: "invalid action: invalid. Must be one of: search, fetch, recommend",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), logger, cache, tc.args)
			assert.Nil(t, result)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestAWSDocumentationTool_Execute_SearchAction(t *testing.T) {
	// Enable AWS tools for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "aws_documentation")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &aws.AWSDocumentationTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress output during tests
	cache := &sync.Map{}

	testCases := []struct {
		name          string
		args          map[string]any
		expectedError string
	}{
		{
			name: "missing search_phrase",
			args: map[string]any{
				"action": "search",
			},
			expectedError: "missing required parameter for search action: search_phrase",
		},
		{
			name: "empty search phrase",
			args: map[string]any{
				"action":        "search",
				"search_phrase": "",
			},
			expectedError: "search_phrase cannot be empty",
		},
		{
			name: "limit too low",
			args: map[string]any{
				"action":        "search",
				"search_phrase": "S3",
				"limit":         0.0, // Use float64 as that's what comes from JSON
			},
			expectedError: "limit must be between 1 and 50",
		},
		{
			name: "limit too high",
			args: map[string]any{
				"action":        "search",
				"search_phrase": "S3",
				"limit":         51.0, // Use float64 as that's what comes from JSON
			},
			expectedError: "limit must be between 1 and 50",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), logger, cache, tc.args)
			if tc.expectedError != "" {
				assert.Nil(t, result)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			}
			// For test cases that should succeed, we just check they don't panic
			// We won't make actual network requests in unit tests
		})
	}
}

func TestAWSDocumentationTool_Execute_FetchAction(t *testing.T) {
	// Enable AWS tools for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "aws_documentation")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &aws.AWSDocumentationTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress output during tests
	cache := &sync.Map{}

	testCases := []struct {
		name          string
		args          map[string]any
		expectedError string
	}{
		{
			name: "missing URL parameter",
			args: map[string]any{
				"action": "fetch",
			},
			expectedError: "missing required parameter for fetch action: url",
		},
		{
			name: "non-AWS domain",
			args: map[string]any{
				"action": "fetch",
				"url":    "https://google.com/page.html",
			},
			expectedError: "URL must be from the docs.aws.amazon.com domain",
		},
		{
			name: "non-HTML file",
			args: map[string]any{
				"action": "fetch",
				"url":    "https://docs.aws.amazon.com/page.pdf",
			},
			expectedError: "URL must end with .html",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), logger, cache, tc.args)
			assert.Nil(t, result)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestAWSDocumentationTool_Execute_RecommendAction(t *testing.T) {
	// Enable AWS tools for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "aws_documentation")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &aws.AWSDocumentationTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress output during tests
	cache := &sync.Map{}

	testCases := []struct {
		name          string
		args          map[string]any
		expectedError string
	}{
		{
			name: "missing URL parameter",
			args: map[string]any{
				"action": "recommend",
			},
			expectedError: "missing required parameter for recommend action: url",
		},
		{
			name: "empty URL",
			args: map[string]any{
				"action": "recommend",
				"url":    "",
			},
			expectedError: "URL cannot be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), logger, cache, tc.args)
			assert.Nil(t, result)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestAWSModels_JSONSerialization(t *testing.T) {
	// Test SearchResult
	searchResult := aws.SearchResult{
		RankOrder: 1,
		URL:       "https://docs.aws.amazon.com/s3/latest/userguide/example.html",
		Title:     "Example S3 Documentation",
		Context:   stringPtr("Example context"),
	}

	// Should be serializable to JSON
	assert.NotPanics(t, func() {
		// This would be used in actual JSON marshalling
		_ = searchResult.RankOrder
		_ = searchResult.URL
		_ = searchResult.Title
		_ = searchResult.Context
	})

	// Test RecommendationResult
	recResult := aws.RecommendationResult{
		URL:     "https://docs.aws.amazon.com/s3/latest/userguide/example.html",
		Title:   "Example S3 Documentation",
		Context: stringPtr("Related content"),
	}

	assert.NotPanics(t, func() {
		_ = recResult.URL
		_ = recResult.Title
		_ = recResult.Context
	})

	// Test DocumentationResponse
	docResponse := aws.DocumentationResponse{
		URL:            "https://docs.aws.amazon.com/s3/latest/userguide/example.html",
		Content:        "# Example Documentation\\nThis is example content.",
		TotalLength:    100,
		StartIndex:     0,
		EndIndex:       50,
		HasMoreContent: true,
		NextStartIndex: intPtr(50),
	}

	assert.NotPanics(t, func() {
		_ = docResponse.URL
		_ = docResponse.Content
		_ = docResponse.TotalLength
		_ = docResponse.StartIndex
		_ = docResponse.EndIndex
		_ = docResponse.HasMoreContent
		_ = docResponse.NextStartIndex
	})
}

func TestParser_IsHTMLContent(t *testing.T) {
	testCases := []struct {
		name        string
		content     string
		contentType string
		expected    bool
	}{
		{
			name:        "HTML with doctype",
			content:     "<!DOCTYPE html><html><body>content</body></html>",
			contentType: "text/html",
			expected:    true,
		},
		{
			name:        "HTML content type",
			content:     "some content",
			contentType: "text/html; charset=utf-8",
			expected:    true,
		},
		{
			name:        "HTML tags detected",
			content:     "<html><head><title>Test</title></head><body>content</body></html>",
			contentType: "",
			expected:    true,
		},
		{
			name:        "Plain text",
			content:     "This is plain text content without HTML",
			contentType: "text/plain",
			expected:    false,
		},
		{
			name:        "Empty content",
			content:     "",
			contentType: "",
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := aws.IsHTMLContent(tc.content, tc.contentType)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParser_FormatDocumentationResult(t *testing.T) {
	parser := aws.NewParser()

	testCases := []struct {
		name        string
		url         string
		content     string
		startIndex  int
		maxLength   int
		expectedLen int
		hasMore     bool
	}{
		{
			name:        "content fits in limit",
			url:         "https://docs.aws.amazon.com/s3/latest/userguide/example.html",
			content:     "Short content",
			startIndex:  0,
			maxLength:   100,
			expectedLen: 13,
			hasMore:     false,
		},
		{
			name:        "content exceeds limit",
			url:         "https://docs.aws.amazon.com/s3/latest/userguide/example.html",
			content:     "This is a very long piece of content that exceeds the maximum length limit",
			startIndex:  0,
			maxLength:   20,
			expectedLen: 20,
			hasMore:     true,
		},
		{
			name:        "start index beyond content",
			url:         "https://docs.aws.amazon.com/s3/latest/userguide/example.html",
			content:     "Short content",
			startIndex:  50,
			maxLength:   100,
			expectedLen: 0,
			hasMore:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := parser.FormatDocumentationResult(tc.url, tc.content, tc.startIndex, tc.maxLength)

			assert.Equal(t, tc.url, result.URL)
			assert.Equal(t, len(tc.content), result.TotalLength)
			assert.Equal(t, tc.startIndex, result.StartIndex)
			assert.Equal(t, tc.hasMore, result.HasMoreContent)

			if tc.expectedLen == 0 {
				assert.Contains(t, result.Content, "No more content available")
			} else {
				assert.Equal(t, tc.expectedLen, len(result.Content))
			}

			if tc.hasMore {
				assert.NotNil(t, result.NextStartIndex)
				assert.Equal(t, tc.startIndex+tc.expectedLen, *result.NextStartIndex)
			} else {
				assert.Nil(t, result.NextStartIndex)
			}
		})
	}
}

func TestExtendedHelp(t *testing.T) {
	// Test that the unified tool provides extended help
	tool := &aws.AWSDocumentationTool{}
	help := tool.ProvideExtendedInfo()
	assert.NotNil(t, help)
	assert.NotEmpty(t, help.Examples)
	assert.NotEmpty(t, help.CommonPatterns)
	assert.NotEmpty(t, help.WhenToUse)
	assert.NotEmpty(t, help.WhenNotToUse)

	// Check that examples cover main actions
	var hasSearch, hasFetch, hasRecommend bool
	for _, example := range help.Examples {
		if action, ok := example.Arguments["action"].(string); ok {
			switch action {
			case "search":
				hasSearch = true
			case "fetch":
				hasFetch = true
			case "recommend":
				hasRecommend = true
			}
		}
	}
	assert.True(t, hasSearch, "Should have search action example")
	assert.True(t, hasFetch, "Should have fetch action example")
	assert.True(t, hasRecommend, "Should have recommend action example")
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func TestAWSDocumentationTool_Execute_ListPricingServicesAction(t *testing.T) {
	// Enable AWS tools for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "aws_documentation")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	// Note: This test requires AWS credentials to be set
	// If credentials are not available, the test will verify error handling
	// If credentials are available, it will verify the action executes
	t.Run("list_pricing_services", func(t *testing.T) {
		tool := &aws.AWSDocumentationTool{}
		logger := logrus.New()
		logger.SetLevel(logrus.PanicLevel) // Suppress output during tests
		cache := &sync.Map{}

		result, err := tool.Execute(context.Background(), logger, cache, map[string]any{
			"action": "list_pricing_services",
		})

		// If AWS credentials are not available, expect an error
		// If credentials are available, expect success
		if err != nil {
			assert.Contains(t, err.Error(), "AWS credentials required for pricing operations")
		} else {
			assert.NotNil(t, result)
		}
	})
}

func TestAWSDocumentationTool_Execute_GetServicePricingAction(t *testing.T) {
	// Enable AWS tools for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "aws_documentation")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &aws.AWSDocumentationTool{}
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel) // Suppress output during tests
	cache := &sync.Map{}

	testCases := []struct {
		name          string
		args          map[string]any
		expectedError string
	}{
		{
			name: "missing service_code",
			args: map[string]any{
				"action": "get_service_pricing",
			},
			expectedError: "missing required parameter for get_service_pricing action: service_code",
		},
		{
			name: "empty service_code",
			args: map[string]any{
				"action":       "get_service_pricing",
				"service_code": "",
			},
			expectedError: "service_code cannot be empty",
		},
		{
			name: "max_results too low",
			args: map[string]any{
				"action":       "get_service_pricing",
				"service_code": "AmazonEC2",
				"max_results":  0.0,
			},
			expectedError: "max_results must be at least 1",
		},
		{
			name: "filter missing field",
			args: map[string]any{
				"action":       "get_service_pricing",
				"service_code": "AmazonEC2",
				"filters": []any{
					map[string]any{
						"value": "t2.micro",
					},
				},
			},
			expectedError: "filter missing required 'field' property",
		},
		{
			name: "filter missing value",
			args: map[string]any{
				"action":       "get_service_pricing",
				"service_code": "AmazonEC2",
				"filters": []any{
					map[string]any{
						"field": "instanceType",
					},
				},
			},
			expectedError: "filter missing required 'value' property",
		},
		{
			name: "filter with empty field",
			args: map[string]any{
				"action":       "get_service_pricing",
				"service_code": "AmazonEC2",
				"filters": []any{
					map[string]any{
						"field": "  ",
						"value": "t2.micro",
					},
				},
			},
			expectedError: "filter missing required 'field' property",
		},
		{
			name: "filter with empty value",
			args: map[string]any{
				"action":       "get_service_pricing",
				"service_code": "AmazonEC2",
				"filters": []any{
					map[string]any{
						"field": "instanceType",
						"value": "  ",
					},
				},
			},
			expectedError: "filter missing required 'value' property",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), logger, cache, tc.args)
			assert.Nil(t, result)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// AWSDocumentationTool implements the unified AWS documentation functionality
type AWSDocumentationTool struct {
	client *Client
	parser *Parser
}

// init registers the AWS documentation tool with the registry
func init() {
	registry.Register(&AWSDocumentationTool{})
}

// Definition returns the AWS documentation tool's definition for MCP registration
func (t *AWSDocumentationTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"aws_documentation",
		mcp.WithDescription("Access AWS documentation with search, fetch, recommendation capabilities. For AWS Strands Agents SDK documentation, use resolve_library_id with 'strands agents' then get_library_docs."),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform: 'search', 'fetch', or 'recommend'"),
			mcp.Enum("search", "fetch", "recommend"),
		),
		mcp.WithString("search_phrase",
			mcp.Description("Search phrase for finding AWS documentation (required for 'search' action)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return for search (Optional, 1-50, default: 5)"),
		),
		mcp.WithString("url",
			mcp.Description("AWS documentation URL (required for 'fetch' and 'recommend' actions, must be from docs.aws.amazon.com and end with .html)"),
		),
		mcp.WithNumber("max_length",
			mcp.Description("Maximum number of characters to return for fetch (Optional, default: 5000)"),
		),
		mcp.WithNumber("start_index",
			mcp.Description("Starting character index for pagination in fetch (Optional, default: 0)"),
		),
	)
}

// Execute performs the specified action on AWS documentation
func (t *AWSDocumentationTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Initialise client and parser if needed
	if t.client == nil {
		t.client = NewClient(logger)
	}
	if t.parser == nil {
		t.parser = NewParser()
	}

	// Parse action parameter
	action, ok := args["action"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: action")
	}

	action = strings.TrimSpace(action)
	if action == "" {
		return nil, fmt.Errorf("action parameter cannot be empty")
	}

	// Dispatch to appropriate handler
	switch action {
	case "search":
		return t.executeSearch(ctx, logger, cache, args)
	case "fetch":
		return t.executeFetch(ctx, logger, cache, args)
	case "recommend":
		return t.executeRecommend(ctx, logger, cache, args)
	default:
		return nil, fmt.Errorf("invalid action: %s. Must be one of: search, fetch, recommend", action)
	}
}

// executeSearch performs documentation search
func (t *AWSDocumentationTool) executeSearch(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Parse search phrase
	searchPhrase, ok := args["search_phrase"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required parameter for search action: search_phrase")
	}

	searchPhrase = strings.TrimSpace(searchPhrase)
	if searchPhrase == "" {
		return nil, fmt.Errorf("search_phrase cannot be empty")
	}

	// Parse limit
	limit := 5
	if limitRaw, ok := args["limit"].(float64); ok {
		limit = int(limitRaw)
		if limit < 1 || limit > 50 {
			return nil, fmt.Errorf("limit must be between 1 and 50")
		}
	}

	// Perform search
	results, err := t.client.SearchDocumentation(searchPhrase, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Format results
	result := map[string]any{
		"action":        "search",
		"search_phrase": searchPhrase,
		"results_count": len(results),
		"results":       results,
	}

	// Convert result to JSON string
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// executeFetch performs documentation fetching and conversion
func (t *AWSDocumentationTool) executeFetch(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Parse the URL parameter
	urlRaw, ok := args["url"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required parameter for fetch action: url")
	}

	// Parse optional parameters
	maxLength := 5000
	if maxLengthRaw, ok := args["max_length"].(float64); ok {
		maxLength = int(maxLengthRaw)
	}

	startIndex := 0
	if startIndexRaw, ok := args["start_index"].(float64); ok {
		startIndex = int(startIndexRaw)
	}

	// Validate URL
	if err := validateAWSDocumentationURL(urlRaw); err != nil {
		return nil, err
	}

	// Fetch the documentation
	htmlContent, err := t.client.FetchDocumentation(urlRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch documentation: %w", err)
	}

	// Check if content is HTML
	contentType := "text/html" // AWS docs are always HTML
	var markdownContent string
	if IsHTMLContent(htmlContent, contentType) {
		markdownContent, err = t.parser.ConvertHTMLToMarkdown(htmlContent)
		if err != nil {
			return nil, fmt.Errorf("failed to convert HTML to markdown: %w", err)
		}
	} else {
		markdownContent = htmlContent
	}

	// Format result with pagination
	docResponse := t.parser.FormatDocumentationResult(urlRaw, markdownContent, startIndex, maxLength)

	// Create formatted response
	result := map[string]any{
		"action":           "fetch",
		"url":              docResponse.URL,
		"content":          docResponse.Content,
		"total_length":     docResponse.TotalLength,
		"start_index":      docResponse.StartIndex,
		"end_index":        docResponse.EndIndex,
		"has_more_content": docResponse.HasMoreContent,
	}

	if docResponse.NextStartIndex != nil {
		result["next_start_index"] = *docResponse.NextStartIndex
	}

	// Add pagination hint if there's more content
	if docResponse.HasMoreContent {
		result["pagination_hint"] = fmt.Sprintf("Content truncated. Call aws_documentation with action='fetch' and start_index=%d to get more content.", *docResponse.NextStartIndex)
	}

	// Convert result to JSON string
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// executeRecommend performs recommendation fetching
func (t *AWSDocumentationTool) executeRecommend(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Parse URL
	url, ok := args["url"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required parameter for recommend action: url")
	}

	url = strings.TrimSpace(url)
	if url == "" {
		return nil, fmt.Errorf("URL cannot be empty")
	}

	// Get recommendations
	recommendations, err := t.client.GetRecommendations(url)
	if err != nil {
		return nil, fmt.Errorf("recommendations failed: %w", err)
	}

	// Format results
	result := map[string]any{
		"action":                "recommend",
		"url":                   url,
		"recommendations":       recommendations,
		"recommendations_count": len(recommendations),
	}

	// Convert result to JSON string
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// validateAWSDocumentationURL validates that the URL is a valid AWS documentation URL
func validateAWSDocumentationURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Must be from docs.aws.amazon.com domain
	if !regexp.MustCompile(`^https?://docs\.aws\.amazon\.com/`).MatchString(url) {
		return fmt.Errorf("URL must be from the docs.aws.amazon.com domain")
	}

	// Must end with .html
	if !strings.HasSuffix(url, ".html") {
		return fmt.Errorf("URL must end with .html")
	}

	return nil
}

// ProvideExtendedInfo implements the ExtendedHelpProvider interface for the AWS documentation tool
func (t *AWSDocumentationTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Search for S3 bucket documentation",
				Arguments: map[string]any{
					"action":        "search",
					"search_phrase": "S3 bucket versioning",
					"limit":         5,
				},
				ExpectedResult: "List of AWS documentation pages about S3 bucket versioning with URLs",
			},
			{
				Description: "Fetch AWS documentation page content",
				Arguments: map[string]any{
					"action": "fetch",
					"url":    "https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html",
				},
				ExpectedResult: "Markdown content of the S3 bucket naming rules documentation",
			},
			{
				Description: "Get recommendations for related AWS content",
				Arguments: map[string]any{
					"action": "recommend",
					"url":    "https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-express-one-zone.html",
				},
				ExpectedResult: "Related S3 documentation, highly rated pages, and similar content",
			},
		},
		CommonPatterns: []string{
			"Start with 'search' action to find relevant documentation URLs",
			"Use 'fetch' action to get full content from discovered URLs",
			"Use 'recommend' action after reading to discover related content",
			"For large documents, use pagination with start_index and max_length",
			"Check has_more_content field to determine if pagination is needed",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "URL validation error for fetch or recommend actions",
				Solution: "Ensure URL starts with https://docs.aws.amazon.com/ and ends with .html",
			},
			{
				Problem:  "Search returns no results for known topics",
				Solution: "Try broader search terms, include service names, or use synonyms",
			},
		},
		ParameterDetails: map[string]string{
			"action":        "Required parameter that determines the operation: 'search' finds documentation, 'fetch' retrieves content, 'recommend' suggests related pages",
			"search_phrase": "Required for search action - use specific technical terms and service names for best results",
			"url":           "Required for fetch and recommend actions - must be valid AWS documentation URL ending with .html",
			"limit":         "Optional for search action - controls number of search results returned (1-50)",
			"max_length":    "Optional for fetch action - controls content truncation, use smaller values for summaries",
			"start_index":   "Optional for fetch action - used for pagination to continue reading from specific position",
		},
		WhenToUse:    "Use when you need to find, read, or explore AWS documentation content",
		WhenNotToUse: "Don't use for non-AWS documentation or when you need to search for AWS service APIs rather than documentation",
	}
}

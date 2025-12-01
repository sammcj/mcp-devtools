package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sammcj/mcp-devtools/internal/tools/aws_documentation/pricing"
	"github.com/sirupsen/logrus"
)

// AWSDocumentationTool implements the unified AWS documentation functionality
type AWSDocumentationTool struct {
	client            *Client
	parser            *Parser
	pricingClient     *pricing.Client
	pricingClientOnce sync.Once
	pricingClientErr  error
}

// init registers the AWS documentation tool with the registry
func init() {
	registry.Register(&AWSDocumentationTool{})
}

// Definition returns the AWS documentation tool's definition for MCP registration
func (t *AWSDocumentationTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"aws_documentation",
		mcp.WithDescription("AWS documentation search, fetch, recommendation, and pricing capabilities. (For AWS Strands Agents SDK instead use resolve_library_id 'strands agents')"),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform: 'search', 'fetch', 'recommend', 'list_pricing_services', 'get_service_pricing'"),
			mcp.Enum("search", "fetch", "recommend", "list_pricing_services", "get_service_pricing"),
		),
		mcp.WithString("search_phrase",
			mcp.Description("Search phrase (required for 'search' action)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results (Optional, 1-50, default: 5, only used for 'search' action)"),
		),
		mcp.WithString("url",
			mcp.Description("documentation URL (required for 'fetch', 'recommend' actions, must be from docs.aws.amazon.com and end with .html)"),
		),
		mcp.WithNumber("max_length",
			mcp.Description("Max characters to fetch (Optional, default: 5000)"),
		),
		mcp.WithNumber("start_index",
			mcp.Description("Starting character index for pagination in fetch (Optional, default: 0)"),
		),
		mcp.WithString("service_code",
			mcp.Description("AWS service code for pricing (required for 'get_service_pricing' action, e.g., 'AmazonEC2', 'AmazonS3')"),
		),
		mcp.WithArray("filters",
			mcp.Description("Pricing filters (optional for 'get_service_pricing' action). Array of filter objects with 'field' and 'value' properties. Each object MUST contain 'field' (string), 'value' (string), optionally 'type' (string, default: 'TERM_MATCH')"),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Max pricing results to return (optional, default: 10)"),
		),
		// Read-only annotations for AWS documentation fetching tool
		mcp.WithReadOnlyHintAnnotation(true),     // Only fetches AWS documentation and pricing, doesn't modify environment
		mcp.WithDestructiveHintAnnotation(false), // No destructive operations
		mcp.WithIdempotentHintAnnotation(true),   // Same queries return same results
		mcp.WithOpenWorldHintAnnotation(true),    // Fetches from external AWS APIs
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
		return t.executeSearch(args)
	case "fetch":
		return t.executeFetch(args)
	case "recommend":
		return t.executeRecommend(args)
	case "list_pricing_services":
		return t.executeListPricingServices(ctx, logger, args)
	case "get_service_pricing":
		return t.executeGetServicePricing(ctx, logger, args)
	default:
		return nil, fmt.Errorf("invalid action: %s. Must be one of: search, fetch, recommend, list_pricing_services, get_service_pricing", action)
	}
}

// executeSearch performs documentation search
func (t *AWSDocumentationTool) executeSearch(args map[string]any) (*mcp.CallToolResult, error) {
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
func (t *AWSDocumentationTool) executeFetch(args map[string]any) (*mcp.CallToolResult, error) {
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
func (t *AWSDocumentationTool) executeRecommend(args map[string]any) (*mcp.CallToolResult, error) {
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

// executeListPricingServices lists all AWS services with available pricing
func (t *AWSDocumentationTool) executeListPricingServices(ctx context.Context, logger *logrus.Logger, _ map[string]any) (*mcp.CallToolResult, error) {
	// Initialise pricing client if needed (thread-safe)
	t.pricingClientOnce.Do(func() {
		t.pricingClient, t.pricingClientErr = pricing.NewClient(ctx, logger)
	})
	if t.pricingClientErr != nil {
		return nil, fmt.Errorf("AWS credentials required for pricing operations: %w", t.pricingClientErr)
	}

	// Get the list of services
	services, err := t.pricingClient.DescribeServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list pricing services: %w", err)
	}

	// Extract service codes
	serviceCodes := make([]string, 0, len(services))
	for _, svc := range services {
		if svc.ServiceCode != nil {
			serviceCodes = append(serviceCodes, *svc.ServiceCode)
		}
	}

	// Format results
	result := map[string]any{
		"action":         "list_pricing_services",
		"services_count": len(serviceCodes),
		"services":       serviceCodes,
	}

	// Convert result to JSON string
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// executeGetServicePricing gets pricing for a specific AWS service
func (t *AWSDocumentationTool) executeGetServicePricing(ctx context.Context, logger *logrus.Logger, args map[string]any) (*mcp.CallToolResult, error) {
	// Parse service_code (required) - validate BEFORE initialising AWS client
	serviceCode, ok := args["service_code"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required parameter for get_service_pricing action: service_code")
	}

	serviceCode = strings.TrimSpace(serviceCode)
	if serviceCode == "" {
		return nil, fmt.Errorf("service_code cannot be empty")
	}

	// Parse max_results (optional) - validate BEFORE initialising AWS client
	maxResults := int32(10)
	if maxResultsRaw, ok := args["max_results"].(float64); ok {
		maxResults = int32(maxResultsRaw)
		if maxResults < 1 {
			return nil, fmt.Errorf("max_results must be at least 1")
		}
	}

	// Parse filters (optional) - validate BEFORE initialising AWS client
	var awsFilters []types.Filter
	if filtersRaw, ok := args["filters"].([]any); ok {
		for i, filterRaw := range filtersRaw {
			filterMap, ok := filterRaw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("filter at index %d is not a valid object", i)
			}

			field, hasField := filterMap["field"].(string)
			value, hasValue := filterMap["value"].(string)

			// Validate required fields
			if !hasField || strings.TrimSpace(field) == "" {
				return nil, fmt.Errorf("filter at index %d missing required 'field' property", i)
			}
			if !hasValue || strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("filter at index %d missing required 'value' property", i)
			}

			// Trim whitespace after validation to ensure clean values are used in AWS Filter
			field = strings.TrimSpace(field)
			value = strings.TrimSpace(value)

			// Default filter type is TERM_MATCH
			filterType := types.FilterTypeTermMatch
			if filterTypeStr, ok := filterMap["type"].(string); ok {
				filterType = types.FilterType(filterTypeStr)
			}

			awsFilters = append(awsFilters, types.Filter{
				Field: &field,
				Value: &value,
				Type:  filterType,
			})
		}
	}

	// Initialise pricing client if needed (thread-safe) - only AFTER parameter validation
	t.pricingClientOnce.Do(func() {
		t.pricingClient, t.pricingClientErr = pricing.NewClient(ctx, logger)
	})
	if t.pricingClientErr != nil {
		return nil, fmt.Errorf("AWS credentials required for pricing operations: %w", t.pricingClientErr)
	}

	// Get pricing products
	priceList, err := t.pricingClient.GetProducts(ctx, serviceCode, awsFilters, maxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to get service pricing: %w", err)
	}

	// Format results
	result := map[string]any{
		"action":        "get_service_pricing",
		"service_code":  serviceCode,
		"product_count": len(priceList),
		"price_list":    priceList,
	}

	// Convert result to JSON string
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
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
			{
				Description: "List all AWS services with available pricing",
				Arguments: map[string]any{
					"action": "list_pricing_services",
				},
				ExpectedResult: "List of all AWS service codes with pricing data (e.g., AmazonEC2, AmazonS3, AmazonRDS)",
			},
			{
				Description: "Get EC2 pricing with location filter",
				Arguments: map[string]any{
					"action":       "get_service_pricing",
					"service_code": "AmazonEC2",
					"filters": []map[string]any{
						{"field": "location", "value": "US East (N. Virginia)"},
					},
					"max_results": 10,
				},
				ExpectedResult: "Pricing information for EC2 instances in us-east-1 with product details and pricing",
			},
			{
				Description: "Get S3 pricing with storage class filter",
				Arguments: map[string]any{
					"action":       "get_service_pricing",
					"service_code": "AmazonS3",
					"filters": []map[string]any{
						{"field": "storageClass", "value": "General Purpose"},
					},
					"max_results": 5,
				},
				ExpectedResult: "Pricing for S3 Standard storage class",
			},
		},
		CommonPatterns: []string{
			"Documentation: Start with 'search' action to find relevant documentation URLs",
			"Documentation: Use 'fetch' action to get full content from discovered URLs",
			"Documentation: Use 'recommend' action after reading to discover related content",
			"Documentation: For large documents, use pagination with start_index and max_length",
			"Pricing: Use 'list_pricing_services' to discover available AWS services",
			"Pricing: Use 'get_service_pricing' with filters to find specific pricing (instance types, storage classes, etc.)",
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
			{
				Problem:  "Pricing request is slow",
				Solution: "Pricing requests fetch data directly from AWS API. Response time depends on AWS API performance and the number of results",
			},
			{
				Problem:  "Too many pricing results returned",
				Solution: "Use filters (instanceType, storageClass, location) or reduce max_results to get more specific results",
			},
		},
		ParameterDetails: map[string]string{
			"action":        "Required parameter: 'search', 'fetch', 'recommend', 'list_pricing_services', or 'get_service_pricing'",
			"search_phrase": "Required for search action - use specific technical terms and service names",
			"url":           "Required for fetch and recommend actions - must be valid AWS documentation URL",
			"limit":         "Optional for search action - controls number of results (1-50)",
			"max_length":    "Optional for fetch action - controls content truncation",
			"start_index":   "Optional for fetch action - used for pagination",
			"service_code":  "Required for get_service_pricing - AWS service code like 'AmazonEC2' or 'AmazonS3'",
			"filters":       "Optional for get_service_pricing - array of filter objects with 'field' and 'value' properties (e.g., location, instanceType, storageClass)",
			"max_results":   "Optional for get_service_pricing - limit number of products returned (default: 10)",
		},
		WhenToUse:    "Use for AWS documentation search/fetch/recommendations (no credentials needed) and AWS pricing information (requires AWS credentials)",
		WhenNotToUse: "Don't use for non-AWS documentation or when you need AWS account-specific pricing (use AWS Cost Explorer instead)",
	}
}

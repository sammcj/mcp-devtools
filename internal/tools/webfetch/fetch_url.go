package webfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// FetchURLTool implements URL fetching with HTML-to-markdown conversion
type FetchURLTool struct{}

// init registers the fetch-url tool
func init() {
	registry.Register(&FetchURLTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *FetchURLTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"fetch_url",
		mcp.WithDescription(`Fetches content from URL and returns it in a readable markdown format.

This tool enables fetching web content for analysis and processing with enhanced pagination support.

Response includes detailed pagination information:
- total_lines: Total number of lines in the content
- start_line/end_line: Line numbers for the returned chunk
- remaining_lines: Number of lines remaining after current chunk
- next_chunk_preview: Preview of what comes next

This tool is useful for fetching web content - for example to get documentation, information from blog posts, changelogs, implementation guidelines and content from search results.
`),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL to fetch (must be http or https)"),
		),
		mcp.WithNumber("max_length",
			mcp.Description("Maximum number of characters to return (default: 6000, max: 1000000)"),
			mcp.DefaultNumber(6000),
		),
		mcp.WithNumber("start_index",
			mcp.Description("Starting character index for pagination (default: 0)"),
			mcp.DefaultNumber(0),
		),
		mcp.WithBoolean("raw",
			mcp.Description("Return raw HTML content without markdown conversion (default: false)"),
		),
	)
}

// Execute executes the fetch-url tool
func (t *FetchURLTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Executing fetch-url tool")

	// Parse and validate parameters
	request, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"url":         request.URL,
		"max_length":  request.MaxLength,
		"start_index": request.StartIndex,
		"raw":         request.Raw,
	}).Debug("Fetch URL parameters")

	// Use security helper for safe HTTP GET
	ops := security.NewOperations("webfetch")
	safeResp, err := ops.SafeHTTPGet(request.URL)
	if err != nil {
		// Handle security errors properly
		if secErr, ok := err.(*security.SecurityError); ok {
			return nil, fmt.Errorf("security block [ID: %s]: %s Check with the user if you may use security_override tool with ID %s",
				secErr.GetSecurityID(), secErr.Error(), secErr.GetSecurityID())
		}
		// Return error information in a structured way
		errorResponse := map[string]interface{}{
			"url":       request.URL,
			"error":     err.Error(),
			"timestamp": time.Now(),
		}
		return t.newToolResultJSON(errorResponse)
	}

	// Convert SafeHTTPResponse to FetchURLResponse format
	response := &FetchURLResponse{
		URL:         request.URL,
		Content:     string(safeResp.Content),
		ContentType: safeResp.ContentType,
		StatusCode:  safeResp.StatusCode,
		TotalLength: len(safeResp.Content),
		TotalLines:  len(strings.Split(string(safeResp.Content), "\n")),
		StartLine:   1,
		EndLine:     len(strings.Split(string(safeResp.Content), "\n")),
	}

	// Process the content (convert HTML to markdown, handle different content types)
	processedContent, err := ProcessContent(logger, response, request.Raw)
	if err != nil {
		logger.WithError(err).Warn("Failed to process content, returning raw content")
		processedContent = response.Content
	}

	// Handle security warnings from the helper
	var securityNotice string
	if safeResp.SecurityResult != nil && safeResp.SecurityResult.Action == security.ActionWarn {
		securityNotice = fmt.Sprintf("Security Warning [ID: %s]: %s Use security_override tool with ID %s if this is intentional.",
			safeResp.SecurityResult.ID, safeResp.SecurityResult.Message, safeResp.SecurityResult.ID)
	}

	// Apply pagination
	paginatedResponse := t.applyPagination(response, processedContent, request)

	// Add security notice to response if needed
	if securityNotice != "" {
		// Convert response to map for adding security field
		responseMap := map[string]interface{}{
			"url":             paginatedResponse.URL,
			"content":         paginatedResponse.Content,
			"truncated":       paginatedResponse.Truncated,
			"start_index":     paginatedResponse.StartIndex,
			"end_index":       paginatedResponse.EndIndex,
			"total_length":    paginatedResponse.TotalLength,
			"total_lines":     paginatedResponse.TotalLines,
			"start_line":      paginatedResponse.StartLine,
			"end_line":        paginatedResponse.EndLine,
			"remaining_lines": paginatedResponse.RemainingLines,
			"security_notice": securityNotice,
		}

		if paginatedResponse.NextChunkPreview != "" {
			responseMap["next_chunk_preview"] = paginatedResponse.NextChunkPreview
		}
		if paginatedResponse.Message != "" {
			responseMap["message"] = paginatedResponse.Message
		}
		if paginatedResponse.ContentType != "" {
			responseMap["content_type"] = paginatedResponse.ContentType
		}
		if paginatedResponse.StatusCode != 200 {
			responseMap["status_code"] = paginatedResponse.StatusCode
		}

		logger.WithFields(logrus.Fields{
			"url":              request.URL,
			"content_type":     safeResp.ContentType,
			"status_code":      safeResp.StatusCode,
			"total_length":     paginatedResponse.TotalLength,
			"returned":         len(paginatedResponse.Content),
			"truncated":        paginatedResponse.Truncated,
			"security_warning": true,
		}).Info("Fetch URL completed with security warning")

		return t.newToolResultJSON(responseMap)
	}

	logger.WithFields(logrus.Fields{
		"url":          request.URL,
		"content_type": safeResp.ContentType,
		"status_code":  safeResp.StatusCode,
		"total_length": paginatedResponse.TotalLength,
		"returned":     len(paginatedResponse.Content),
		"truncated":    paginatedResponse.Truncated,
	}).Info("Fetch URL completed successfully")

	return t.newToolResultJSON(paginatedResponse)
}

// parseRequest parses and validates the tool arguments
func (t *FetchURLTool) parseRequest(args map[string]interface{}) (*FetchURLRequest, error) {
	// Parse URL (required)
	url, ok := args["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: url")
	}

	// Basic URL validation
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// Try to add https:// if no scheme is provided
		if !strings.Contains(url, "://") {
			url = "https://" + url
		} else {
			return nil, fmt.Errorf("URL must use http or https scheme")
		}
	}

	request := &FetchURLRequest{
		URL:        url,
		MaxLength:  6000,  // Default
		StartIndex: 0,     // Default
		Raw:        false, // Default
	}

	// Parse max_length (optional)
	if maxLengthRaw, ok := args["max_length"].(float64); ok {
		maxLength := int(maxLengthRaw)
		if maxLength < 1 {
			return nil, fmt.Errorf("max_length must be at least 1")
		}
		if maxLength > 1000000 {
			return nil, fmt.Errorf("max_length cannot exceed 1,000,000")
		}
		request.MaxLength = maxLength
	}

	// Parse start_index (optional)
	if startIndexRaw, ok := args["start_index"].(float64); ok {
		startIndex := int(startIndexRaw)
		if startIndex < 0 {
			return nil, fmt.Errorf("start_index must be >= 0")
		}
		request.StartIndex = startIndex
	}

	// Parse raw (optional)
	if rawRaw, ok := args["raw"].(bool); ok {
		request.Raw = rawRaw
	}

	return request, nil
}

// applyPagination applies enhanced pagination logic to the content
func (t *FetchURLTool) applyPagination(originalResponse *FetchURLResponse, processedContent string, request *FetchURLRequest) *FetchURLResponse {
	totalLength := len(processedContent)

	// Split content into lines for better analysis
	lines := strings.Split(processedContent, "\n")
	totalLines := len(lines)

	// Check if start_index is beyond content length
	if request.StartIndex >= totalLength {
		return &FetchURLResponse{
			URL:            originalResponse.URL,
			Content:        "",
			Truncated:      false,
			StartIndex:     request.StartIndex,
			EndIndex:       request.StartIndex,
			TotalLength:    totalLength,
			TotalLines:     totalLines,
			StartLine:      0,
			EndLine:        0,
			RemainingLines: 0,
			Message:        "start_index is beyond content length",
		}
	}

	// Calculate end index with smart truncation
	endIndex := request.StartIndex + request.MaxLength
	if endIndex > totalLength {
		endIndex = totalLength
	}

	// Try to truncate at natural boundaries (end of lines, paragraphs)
	content := processedContent[request.StartIndex:endIndex]
	truncated := endIndex < totalLength

	// If we're truncating and not at the end, try to find a better break point
	if truncated && endIndex < totalLength {
		// Look for natural break points within the last 200 characters
		lookbackStart := max(0, endIndex-200)
		lookbackSection := processedContent[lookbackStart:endIndex]

		// Try to find paragraph breaks first
		if lastParaBreak := strings.LastIndex(lookbackSection, "\n\n"); lastParaBreak != -1 {
			endIndex = lookbackStart + lastParaBreak + 2
			content = processedContent[request.StartIndex:endIndex]
		} else if lastLineBreak := strings.LastIndex(lookbackSection, "\n"); lastLineBreak != -1 {
			// Fall back to line breaks
			endIndex = lookbackStart + lastLineBreak + 1
			content = processedContent[request.StartIndex:endIndex]
		}
	}

	// Calculate line numbers
	startLine := t.calculateLineNumber(processedContent, request.StartIndex)
	endLine := t.calculateLineNumber(processedContent, endIndex)

	// Generate next chunk preview if truncated
	var nextChunkPreview string
	remainingLines := 0
	if truncated {
		remainingLines = totalLines - endLine
		// Get a preview of the next chunk (first 200 chars)
		previewStart := endIndex
		previewEnd := min(previewStart+200, totalLength)
		preview := processedContent[previewStart:previewEnd]

		// Clean up the preview
		if idx := strings.Index(preview, "\n"); idx != -1 && idx < 100 {
			preview = preview[:idx] + "..."
		}
		nextChunkPreview = strings.TrimSpace(preview)
		if len(nextChunkPreview) > 150 {
			nextChunkPreview = nextChunkPreview[:150] + "..."
		}
	}

	// Create enhanced response
	response := &FetchURLResponse{
		URL:              originalResponse.URL,
		Content:          content,
		Truncated:        truncated,
		StartIndex:       request.StartIndex,
		EndIndex:         endIndex,
		TotalLength:      totalLength,
		TotalLines:       totalLines,
		StartLine:        startLine,
		EndLine:          endLine,
		NextChunkPreview: nextChunkPreview,
		RemainingLines:   remainingLines,
		Message:          "",
	}

	// Only include ContentType and StatusCode if they're not defaults
	if originalResponse.ContentType != "" {
		response.ContentType = originalResponse.ContentType
	}
	if originalResponse.StatusCode != 200 {
		response.StatusCode = originalResponse.StatusCode
	}

	// Add helpful pagination message if content is truncated
	if truncated {
		nextStartIndex := endIndex
		response.Message = fmt.Sprintf("Content truncated at line %d of %d total lines. %d lines remaining. To fetch more content, call with start_index=%d you can also optionally specify max_length to fetch more content at once.",
			endLine, totalLines, remainingLines, nextStartIndex)
	}

	return response
}

// calculateLineNumber calculates the line number for a given character index
func (t *FetchURLTool) calculateLineNumber(content string, charIndex int) int {
	if charIndex <= 0 {
		return 1
	}
	if charIndex >= len(content) {
		return len(strings.Split(content, "\n"))
	}
	return strings.Count(content[:charIndex], "\n") + 1
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// newToolResultJSON creates a new tool result with JSON content
func (t *FetchURLTool) newToolResultJSON(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// ProvideExtendedInfo provides detailed usage information for the fetch_url tool
func (t *FetchURLTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Fetch a webpage and convert to markdown",
				Arguments: map[string]interface{}{
					"url": "https://docs.example.com/getting-started",
				},
				ExpectedResult: "Returns webpage content converted to clean markdown format, useful for analysis and processing",
			},
			{
				Description: "Fetch raw HTML without markdown conversion",
				Arguments: map[string]interface{}{
					"url": "https://api.example.com/status",
					"raw": true,
				},
				ExpectedResult: "Returns raw HTML content without conversion, useful for parsing structured HTML or APIs returning HTML",
			},
			{
				Description: "Fetch large content with pagination",
				Arguments: map[string]interface{}{
					"url":        "https://longdocument.example.com/guide",
					"max_length": 15000,
				},
				ExpectedResult: "Returns first 15,000 characters with pagination info including total length, line numbers, and next chunk preview",
			},
			{
				Description: "Continue reading from a specific point",
				Arguments: map[string]interface{}{
					"url":         "https://longdocument.example.com/guide",
					"start_index": 15000,
					"max_length":  10000,
				},
				ExpectedResult: "Returns content starting from character 15,000 for the next 10,000 characters, enabling sequential reading of long documents",
			},
			{
				Description: "Fetch API documentation with custom length",
				Arguments: map[string]interface{}{
					"url":        "https://api-docs.example.com/v2/reference",
					"max_length": 25000,
				},
				ExpectedResult: "Returns API documentation content up to 25,000 characters, converted to markdown for easy reading and analysis",
			},
		},
		CommonPatterns: []string{
			"Start with default settings first to get a preview of content structure",
			"For long documents: use pagination (start with default, then continue with start_index)",
			"Use raw=true for HTML parsing or when markdown conversion breaks the structure",
			"Increase max_length for comprehensive content, decrease for quick previews",
			"Combine with internet search results to fetch full content from interesting URLs",
			"Use with memory tool to store important content for later reference",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "SSL certificate errors or connection timeouts",
				Solution: "The website may have security restrictions or be temporarily unavailable. Try again later or check if the URL is correct and publicly accessible.",
			},
			{
				Problem:  "Content appears garbled or poorly formatted",
				Solution: "Try setting 'raw: true' to get unprocessed content, or the website may use complex JavaScript rendering that requires a browser to display properly.",
			},
			{
				Problem:  "Pagination returns empty content with start_index",
				Solution: "The start_index may be beyond the content length. Check the total_length from a previous fetch and ensure start_index is less than that value.",
			},
			{
				Problem:  "Authentication required or access denied errors",
				Solution: "The content requires login or API keys. This tool only fetches publicly accessible content - private or authenticated content cannot be accessed.",
			},
			{
				Problem:  "Content is truncated unexpectedly",
				Solution: "The content hit the max_length limit. Use pagination with start_index to fetch more content, or increase max_length parameter (up to 1,000,000 characters).",
			},
		},
		ParameterDetails: map[string]string{
			"url":         "Must be a complete HTTP/HTTPS URL. Tool will attempt to add 'https://' if no protocol is specified. Does not support FTP, file://, or other protocols.",
			"max_length":  "Controls how much content to return (1 to 1,000,000 characters). Default is 6,000. Use larger values for comprehensive content, smaller for previews.",
			"start_index": "Character position to start reading from (0-based). Use for pagination when content is longer than max_length. Default is 0 (start of content).",
			"raw":         "When true, returns raw HTML without markdown conversion. When false (default), converts HTML to clean markdown format for easier reading and analysis.",
		},
		WhenToUse:    "Use to fetch and process web content for analysis, extract information from documentation, get full text from search results, or read blog posts and articles. Ideal for content that needs to be analysed or processed by AI.",
		WhenNotToUse: "Don't use for downloading files, accessing authenticated content, scraping data that requires JavaScript execution, or fetching binary content like images or PDFs.",
	}
}

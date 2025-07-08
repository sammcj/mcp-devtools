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
	"github.com/sirupsen/logrus"
)

// FetchURLTool implements URL fetching with HTML-to-markdown conversion
type FetchURLTool struct {
	client *WebClient
}

// init registers the fetch-url tool
func init() {
	registry.Register(&FetchURLTool{
		client: NewWebClient(),
	})
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

	// Fetch the content
	response, err := t.client.FetchContent(ctx, logger, request.URL)
	if err != nil {
		// Return error information in a structured way
		errorResponse := map[string]interface{}{
			"url":       request.URL,
			"error":     err.Error(),
			"timestamp": time.Now(),
		}
		return t.newToolResultJSON(errorResponse)
	}

	// Process the content (convert HTML to markdown, handle different content types)
	processedContent, err := ProcessContent(logger, response, request.Raw)
	if err != nil {
		logger.WithError(err).Warn("Failed to process content, returning raw content")
		processedContent = response.Content
	}

	// Apply pagination
	paginatedResponse := t.applyPagination(response, processedContent, request)

	logger.WithFields(logrus.Fields{
		"url":          request.URL,
		"content_type": response.ContentType,
		"status_code":  response.StatusCode,
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

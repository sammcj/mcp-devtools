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
		mcp.WithDescription(`Fetches a URL from the internet and converts HTML content to readable markdown format.

This tool enables fetching web content for AI analysis and processing. It automatically converts HTML to clean markdown, filters out navigation elements, ads, and other non-content elements, and supports pagination for large content.

Features:
- Automatic HTML-to-markdown conversion for better readability
- Content type detection (HTML, plain text, binary)
- Pagination support for large content
- Context-aware HTTP requests with proper error handling
- Raw content option for debugging

Use this when you need to fetch and analyse web content, documentation, articles, or any web-based information.`),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL to fetch (must be http or https)"),
		),
		mcp.WithNumber("max_length",
			mcp.Description("Maximum number of characters to return (default: 5000, max: 1000000)"),
			mcp.DefaultNumber(5000),
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
		MaxLength:  5000,  // Default
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

// applyPagination applies pagination logic to the content
func (t *FetchURLTool) applyPagination(originalResponse *FetchURLResponse, processedContent string, request *FetchURLRequest) *FetchURLResponse {
	totalLength := len(processedContent)

	// Check if start_index is beyond content length
	if request.StartIndex >= totalLength {
		return &FetchURLResponse{
			URL:         originalResponse.URL,
			ContentType: originalResponse.ContentType,
			StatusCode:  originalResponse.StatusCode,
			Content:     "",
			Truncated:   false,
			StartIndex:  request.StartIndex,
			EndIndex:    request.StartIndex,
			TotalLength: totalLength,
			Timestamp:   originalResponse.Timestamp,
			Message:     "start_index is beyond content length",
		}
	}

	// Calculate end index
	endIndex := request.StartIndex + request.MaxLength
	if endIndex > totalLength {
		endIndex = totalLength
	}

	// Extract the requested slice
	content := processedContent[request.StartIndex:endIndex]

	// Determine if content is truncated
	truncated := endIndex < totalLength

	// Create response
	response := &FetchURLResponse{
		URL:         originalResponse.URL,
		ContentType: originalResponse.ContentType,
		StatusCode:  originalResponse.StatusCode,
		Content:     content,
		Truncated:   truncated,
		StartIndex:  request.StartIndex,
		EndIndex:    endIndex,
		TotalLength: totalLength,
		Timestamp:   originalResponse.Timestamp,
		Message:     "",
	}

	// Add helpful pagination message if content is truncated
	if truncated {
		nextStartIndex := endIndex
		response.Message = fmt.Sprintf("Content truncated. To fetch more content, call with start_index=%d", nextStartIndex)
	}

	return response
}

// newToolResultJSON creates a new tool result with JSON content
func (t *FetchURLTool) newToolResultJSON(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

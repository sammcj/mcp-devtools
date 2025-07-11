package packagedocs

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// GetLibraryDocsTool fetches documentation for a specific library
type GetLibraryDocsTool struct {
	client *Client
}

// init registers the get_library_docs tool
func init() {
	registry.Register(&GetLibraryDocsTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *GetLibraryDocsTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"get_library_docs",
		mcp.WithDescription(`Fetches up-to-date documentation for a library / package. You must call 'resolve_library_id' first to obtain the exact Context7-compatible library ID required to use this tool, UNLESS the user explicitly provides a library ID in the format '/org/project' or '/org/project/version' in their query.`),
		mcp.WithString("context7CompatibleLibraryID",
			mcp.Required(),
			mcp.Description("Exact Context7-compatible library ID (e.g., '/mongodb/docs', '/vercel/next.js', '/supabase/supabase', '/vercel/next.js/v14.3.0-canary.87') retrieved from 'resolve_library_id' or directly from user query in the format '/org/project' or '/org/project/version'."),
		),
		mcp.WithString("topic",
			mcp.Description("Topic to focus documentation on (e.g., 'hooks', 'routing')."),
		),
		mcp.WithNumber("tokens",
			mcp.Description("Maximum number of tokens of documentation to retrieve (default: 10000). Higher values provide more context but consume more tokens."),
			mcp.DefaultNumber(10000),
		),
	)
}

// Execute executes the get_library_docs tool
func (t *GetLibraryDocsTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Lazy initialise client
	if t.client == nil {
		t.client = NewClient(logger)
	}

	logger.Info("Executing get_library_docs tool")

	// Parse required parameters
	libraryID, ok := args["context7CompatibleLibraryID"].(string)
	if !ok || strings.TrimSpace(libraryID) == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: context7CompatibleLibraryID")
	}

	libraryID = strings.TrimSpace(libraryID)

	// Validate library ID format
	if err := ValidateLibraryID(libraryID); err != nil {
		return nil, fmt.Errorf("invalid library ID format: %w", err)
	}

	// Parse optional parameters
	var topic string
	if topicRaw, ok := args["topic"].(string); ok {
		topic = strings.TrimSpace(topicRaw)
	}

	tokens := 10000 // Default
	if tokensRaw, ok := args["tokens"].(float64); ok {
		tokens = int(tokensRaw)
		if tokens < 1000 {
			return nil, fmt.Errorf("tokens must be at least 1000")
		}
		if tokens > 100000 {
			return nil, fmt.Errorf("tokens cannot exceed 100,000")
		}
	}

	logger.WithFields(logrus.Fields{
		"library_id": libraryID,
		"topic":      topic,
		"tokens":     tokens,
	}).Debug("Fetching library documentation")

	// Prepare parameters
	params := &SearchLibraryDocsParams{
		Topic:  topic,
		Tokens: tokens,
	}

	// Fetch documentation
	docs, err := t.client.GetLibraryDocs(ctx, libraryID, params)
	if err != nil {
		logger.WithError(err).Error("Failed to fetch library documentation")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch documentation: %v", err)), nil
	}

	if strings.TrimSpace(docs) == "" {
		logger.WithField("library_id", libraryID).Warn("No documentation content returned")
		return mcp.NewToolResultError("No documentation content found for the specified library."), nil
	}

	// Format the response with metadata
	response := t.formatResponse(libraryID, topic, tokens, docs)

	logger.WithFields(logrus.Fields{
		"library_id":     libraryID,
		"topic":          topic,
		"tokens":         tokens,
		"content_length": len(docs),
	}).Info("Library documentation retrieved successfully")

	return mcp.NewToolResultText(response), nil
}

// formatResponse formats the documentation with helpful metadata
func (t *GetLibraryDocsTool) formatResponse(libraryID, topic string, tokens int, docs string) string {
	var builder strings.Builder

	// Add header with metadata
	builder.WriteString(fmt.Sprintf("# Documentation for %s\n\n", libraryID))

	if topic != "" {
		builder.WriteString(fmt.Sprintf("**Topic Focus:** %s\n", topic))
	}
	builder.WriteString(fmt.Sprintf("**Token Limit:** %d\n", tokens))
	builder.WriteString(fmt.Sprintf("**Content Length:** %d characters\n\n", len(docs)))

	builder.WriteString("---\n\n")

	// Add the actual documentation
	builder.WriteString(docs)

	// Add footer note
	builder.WriteString("\n\n---\n\n")
	builder.WriteString("*Documentation retrieved from Context7. This content is optimised for AI consumption and may be summarised or filtered based on the specified topic and token limits.*")

	return builder.String()
}

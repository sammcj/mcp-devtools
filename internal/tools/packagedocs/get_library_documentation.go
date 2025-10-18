package packagedocs

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// GetLibraryDocsTool fetches documentation for a specific library
type GetLibraryDocsTool struct {
	client *Client
}

// init registers the get_library_documentation tool
func init() {
	registry.Register(&GetLibraryDocsTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *GetLibraryDocsTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"get_library_documentation",
		mcp.WithDescription(`Fetches up to date documentation for a library / package. You must call 'resolve_library_id' first to obtain the exact Context7-compatible library ID required to use this tool, UNLESS the user explicitly provides a library ID in the format '/org/project' or '/org/project/version' in their query.

		You should use this tool in combination with resolve_library_id when looking up software documentation and examples to understand how to implement a specific library or package in your code.
		`),
		mcp.WithString("context7CompatibleLibraryID",
			mcp.Required(),
			mcp.Description("Exact Context7-compatible library ID (e.g., '/strands-agents/sdk-python', '/strands-agents/tools', '/strands-agents/samples', '/mongodb/docs', '/vercel/next.js', '/mark3labs/mcp-go', '/vercel/next.js/v14.3.0-canary.87') retrieved from 'resolve_library_id' or directly from user query in the format '/org/project' or '/org/project/version'."),
		),
		mcp.WithString("topic",
			mcp.Description("Topic to focus documentation on (e.g., 'hooks', 'routing')."),
		),
		mcp.WithNumber("tokens",
			mcp.Description("Maximum number of tokens of documentation to retrieve (default: 10000). Higher values provide more context but consume more tokens."),
			mcp.DefaultNumber(10000),
		),
		// Read-only annotations for library documentation fetching tool
		mcp.WithReadOnlyHintAnnotation(true),     // Only fetches documentation, doesn't modify environment
		mcp.WithDestructiveHintAnnotation(false), // No destructive operations
		mcp.WithIdempotentHintAnnotation(true),   // Same library ID returns same documentation
		mcp.WithOpenWorldHintAnnotation(true),    // Fetches from external documentation APIs
	)
}

// Execute executes the get_library_documentation tool
func (t *GetLibraryDocsTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Lazy initialise client
	if t.client == nil {
		t.client = NewClient(logger)
	}

	logger.Info("Executing get_library_documentation tool")

	// Parse required parameters
	libraryID, ok := args["context7CompatibleLibraryID"].(string)
	if !ok || strings.TrimSpace(libraryID) == "" {
		return nil, fmt.Errorf("missing required parameter 'context7CompatibleLibraryID'. First call 'resolve_library_id' to get the correct library ID (e.g., '/vercel/next.js'), or if the user provided a library ID directly, ensure it starts with '/' and follows the format '/org/project' or '/org/project/version'")
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
			return nil, fmt.Errorf("'tokens' must be at least 1000 (you provided %d). Use 1000-5000 for quick overviews, 10000-25000 for comprehensive documentation", tokens)
		}
		if tokens > 100000 {
			return nil, fmt.Errorf("'tokens' cannot exceed 100,000 (you provided %d). For large libraries, make multiple calls with different 'topic' values instead", tokens)
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

// ProvideExtendedInfo provides detailed usage information for the get_library_documentation tool
func (t *GetLibraryDocsTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Get Next.js documentation without a specific topic focus",
				Arguments: map[string]any{
					"context7CompatibleLibraryID": "/vercel/next.js",
				},
				ExpectedResult: "Returns general Next.js documentation up to 10,000 tokens covering core concepts, setup, and API reference",
			},
			{
				Description: "Get focused React hooks documentation with higher token limit",
				Arguments: map[string]any{
					"context7CompatibleLibraryID": "/facebook/react",
					"topic":                       "hooks",
					"tokens":                      20000,
				},
				ExpectedResult: "Returns React documentation focused specifically on hooks (useState, useEffect, custom hooks, etc.) with up to 20,000 tokens",
			},
			{
				Description: "Get MongoDB driver documentation for a specific version",
				Arguments: map[string]any{
					"context7CompatibleLibraryID": "/mongodb/node-mongodb-native/v4.17.1",
					"topic":                       "aggregation",
				},
				ExpectedResult: "Returns MongoDB Node.js driver v4.17.1 documentation focused on aggregation pipelines and operations",
			},
			{
				Description: "Get concise AWS SDK documentation for Lambda functions",
				Arguments: map[string]any{
					"context7CompatibleLibraryID": "/aws/aws-sdk-js-v3",
					"topic":                       "lambda",
					"tokens":                      5000,
				},
				ExpectedResult: "Returns AWS SDK v3 documentation focused on Lambda client and operations, limited to 5,000 tokens for conciseness",
			},
		},
		CommonPatterns: []string{
			"Always call 'resolve_library_id' first to get the correct Context7-compatible library ID",
			"Use specific topic focus to get targeted documentation (e.g., 'authentication', 'routing', 'database')",
			"Start with lower token counts (5000-10000) for quick overviews, increase (15000-25000) for comprehensive docs",
			"Common workflow: resolve_library_id → get_library_documentation → implement based on documentation",
			"Combine with package version tools to ensure you're using compatible API versions",
			"Use topic parameter to avoid overwhelming responses for large libraries",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Invalid library ID format error",
				Solution: "Ensure the library ID starts with '/' and follows the pattern '/org/project' or '/org/project/version'. Use 'resolve_library_id' tool first to get the correct format.",
			},
			{
				Problem:  "No documentation content found for the specified library",
				Solution: "The library might not be available in Context7's database. Try using 'resolve_library_id' to search for alternative names or check if the library exists in the Context7 catalogue.",
			},
			{
				Problem:  "Documentation is too broad or unfocused",
				Solution: "Use the 'topic' parameter to focus on specific areas (e.g., 'getting-started', 'api-reference', 'examples'). Also consider reducing the token count for more targeted results.",
			},
			{
				Problem:  "Token limit errors or truncated responses",
				Solution: "Reduce the 'tokens' parameter (minimum 1000, maximum 100,000). For large libraries, use multiple calls with different topics rather than one large request.",
			},
			{
				Problem:  "Outdated or incorrect documentation",
				Solution: "Specify a version in the library ID (e.g., '/vercel/next.js/v13.0.0') or use 'resolve_library_id' to find the most recent available version.",
			},
		},
		ParameterDetails: map[string]string{
			"context7CompatibleLibraryID": "Must be in the format '/org/project' or '/org/project/version'. Examples: '/vercel/next.js', '/facebook/react/v18.2.0'. Always use 'resolve_library_id' first unless the user provides this exact format.",
			"topic":                       "Optional focus area for the documentation. Examples: 'getting-started', 'api-reference', 'hooks', 'routing', 'authentication', 'examples'. Helps filter large documentation sets to relevant sections.",
			"tokens":                      "Controls the amount of documentation returned (1000-100000). Lower values (5000) for quick reference, higher values (20000+) for comprehensive guides. Default is 10000 tokens.",
		},
		WhenToUse:    "Use when you need current, comprehensive documentation for implementing a specific library or package. Ideal for understanding APIs, getting code examples, learning best practices, or troubleshooting integration issues.",
		WhenNotToUse: "Don't use for general programming concepts, language syntax, or libraries not available in Context7. Use 'resolve_library_id' first if you're unsure about library availability or naming.",
	}
}

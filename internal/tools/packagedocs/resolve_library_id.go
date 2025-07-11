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

// ResolveLibraryIDTool resolves a library name to a Context7-compatible library ID
type ResolveLibraryIDTool struct {
	client *Client
}

// init registers the resolve_library_id tool
func init() {
	registry.Register(&ResolveLibraryIDTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *ResolveLibraryIDTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"resolve_library_id",
		mcp.WithDescription(`Resolves a package/product name to a Context7-compatible library ID and returns a list of matching libraries.

You MUST call this function before 'get_library_docs' to obtain a valid Context7-compatible library ID UNLESS the user explicitly provides a library ID in the format '/org/project' or '/org/project/version' in their query.

Selection Process:
1. Analyse the query to understand what library/package the user is looking for
2. Return the most relevant match based on:
- Name similarity to the query (exact matches prioritised)
- Description relevance to the query's intent
- Documentation coverage (prioritise libraries with higher Code Snippet counts)
- Trust score (consider libraries with scores of 7-10 more authoritative)

Response Format:
- Return the selected library ID in a clearly marked section
- Provide a brief explanation for why this library was chosen
- If multiple good matches exist, acknowledge this but proceed with the most relevant one
- If no good matches exist, clearly state this and suggest query refinements

For ambiguous queries, request clarification before proceeding with a best-guess match.`),
		mcp.WithString("libraryName",
			mcp.Required(),
			mcp.Description("Library name to search for and retrieve a Context7-compatible library ID."),
		),
	)
}

// Execute executes the resolve_library_id tool
func (t *ResolveLibraryIDTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Lazy initialise client
	if t.client == nil {
		t.client = NewClient(logger)
	}

	logger.Info("Executing resolve_library_id tool")

	// Parse library name
	libraryName, ok := args["libraryName"].(string)
	if !ok || strings.TrimSpace(libraryName) == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: libraryName")
	}

	libraryName = strings.TrimSpace(libraryName)

	logger.WithField("library_name", libraryName).Debug("Resolving library ID")

	// Search for libraries
	results, err := t.client.SearchLibraries(ctx, libraryName)
	if err != nil {
		logger.WithError(err).Error("Failed to search libraries")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search for libraries: %v", err)), nil
	}

	if len(results) == 0 {
		logger.WithField("library_name", libraryName).Warn("No libraries found")
		return mcp.NewToolResultError(fmt.Sprintf("No libraries found matching '%s'. Try a more specific or different search term.", libraryName)), nil
	}

	// Format the response
	response := t.formatResponse(libraryName, results)

	logger.WithFields(logrus.Fields{
		"library_name": libraryName,
		"result_count": len(results),
		"selected_id":  results[0].GetResourceURI(),
	}).Info("Library ID resolution completed")

	return mcp.NewToolResultText(response), nil
}

// formatResponse formats the search results into a user-friendly response
func (t *ResolveLibraryIDTool) formatResponse(libraryName string, results []*SearchResult) string {
	var builder strings.Builder

	// Select the best match (first result is typically most relevant)
	bestMatch := results[0]

	builder.WriteString("## Selected Library ID\n\n")
	builder.WriteString(fmt.Sprintf("**Library ID:** `%s`\n\n", bestMatch.GetResourceURI()))
	builder.WriteString(fmt.Sprintf("**Chosen Library:** %s\n", bestMatch.Title))
	if bestMatch.Description != "" {
		builder.WriteString(fmt.Sprintf("**Description:** %s\n", bestMatch.Description))
	}

	// Add selection rationale
	builder.WriteString("\n**Selection Rationale:**\n")
	builder.WriteString(fmt.Sprintf("- This library was selected as the most relevant match for '%s'\n", libraryName))

	if bestMatch.TrustScore > 0 {
		builder.WriteString(fmt.Sprintf("- Trust Score: %.1f/10 (higher scores indicate more authoritative sources)\n", bestMatch.TrustScore))
	}
	if bestMatch.Stars > 0 {
		builder.WriteString(fmt.Sprintf("- GitHub Stars: %d\n", bestMatch.Stars))
	}
	if bestMatch.TotalSnippets > 0 {
		builder.WriteString(fmt.Sprintf("- Documentation Coverage: %d code snippets, %d tokens\n", bestMatch.TotalSnippets, bestMatch.TotalTokens))
	}

	// Show alternative matches if there are more results
	if len(results) > 1 {
		builder.WriteString("\n## Alternative Matches Found\n\n")
		builder.WriteString(fmt.Sprintf("Found %d total matches. Other options include:\n\n", len(results)))

		for i, result := range results[1:] {
			if i >= 4 { // Limit to top 5 alternatives
				builder.WriteString(fmt.Sprintf("... and %d more matches\n", len(results)-5))
				break
			}

			builder.WriteString(fmt.Sprintf("%d. **%s** (`%s`)\n", i+2, result.Title, result.GetResourceURI()))
			if result.Description != "" {
				// Truncate long descriptions
				desc := result.Description
				if len(desc) > 100 {
					desc = desc[:100] + "..."
				}
				builder.WriteString(fmt.Sprintf("   %s\n", desc))
			}
			if result.TrustScore > 0 || result.Stars > 0 {
				builder.WriteString(fmt.Sprintf("   Trust Score: %.1f, Stars: %d\n", result.TrustScore, result.Stars))
			}
			builder.WriteString("\n")
		}

		builder.WriteString("If you need documentation for a different library, please call this function again with a more specific library name.\n")
	}

	return builder.String()
}

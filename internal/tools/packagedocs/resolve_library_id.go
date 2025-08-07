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

// ProvideExtendedInfo provides detailed usage information for the resolve_library_id tool
func (t *ResolveLibraryIDTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Find the Context7 library ID for React",
				Arguments: map[string]interface{}{
					"libraryName": "React",
				},
				ExpectedResult: "Returns the Context7-compatible library ID for React (e.g., '/facebook/react') along with trust score, stars, and documentation coverage details",
			},
			{
				Description: "Resolve Next.js library ID",
				Arguments: map[string]interface{}{
					"libraryName": "Next.js",
				},
				ExpectedResult: "Identifies '/vercel/next.js' as the library ID with explanation of selection criteria and alternative matches if available",
			},
			{
				Description: "Find MongoDB Node.js driver",
				Arguments: map[string]interface{}{
					"libraryName": "mongodb nodejs driver",
				},
				ExpectedResult: "Locates the official MongoDB Node.js driver library ID with version information and documentation metrics",
			},
			{
				Description: "Search for AWS SDK",
				Arguments: map[string]interface{}{
					"libraryName": "aws-sdk",
				},
				ExpectedResult: "Returns the most relevant AWS SDK library ID (likely JavaScript version) with alternatives for different language SDKs",
			},
			{
				Description: "Resolve specific version of a library",
				Arguments: map[string]interface{}{
					"libraryName": "vue 3",
				},
				ExpectedResult: "Finds Vue.js version 3 specific documentation or the main Vue library with version-specific information",
			},
		},
		CommonPatterns: []string{
			"Always use this tool BEFORE calling get_library_docs to find the correct library ID format",
			"Use specific library names rather than generic terms (e.g., 'React' not 'frontend framework')",
			"Include version or variant info in search (e.g., 'vue 3', 'aws-sdk-js')",
			"Check alternative matches if the selected library doesn't match your needs",
			"Use the exact library ID returned in subsequent get_library_docs calls",
			"For ambiguous results, try more specific search terms or library variants",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "No libraries found for a search term",
				Solution: "Try alternative names, abbreviations, or more generic terms. For example, try 'mongoose' instead of 'mongoose orm', or 'express' instead of 'express.js'.",
			},
			{
				Problem:  "Wrong library selected as best match",
				Solution: "Check the alternative matches section in the response. You can use a more specific libraryName or manually select from the alternatives provided.",
			},
			{
				Problem:  "Multiple similar libraries returned",
				Solution: "Look at trust scores, GitHub stars, and documentation coverage to choose the most appropriate option. Higher trust scores (7-10) indicate more authoritative sources.",
			},
			{
				Problem:  "Library ID format doesn't work with get_library_docs",
				Solution: "Ensure you're using the exact library ID returned (e.g., '/facebook/react'), not the library title. The library ID always starts with '/' and follows '/org/project' format.",
			},
			{
				Problem:  "Outdated or deprecated library versions",
				Solution: "Check the alternative matches for newer versions, or search with version-specific terms like 'react 18' or 'vue 3' to find current versions.",
			},
		},
		ParameterDetails: map[string]string{
			"libraryName": "Library or package name to search for. Can include version numbers, variants, or descriptive terms. Examples: 'React', 'Next.js', 'mongodb driver', 'aws-sdk-js', 'vue 3'. More specific terms usually yield better results.",
		},
		WhenToUse:    "Use as the first step before getting library documentation. Essential when you know the library name but need the Context7-compatible format, or when discovering available libraries for a technology stack.",
		WhenNotToUse: "Don't use when you already have the exact Context7 library ID in '/org/project' format, for general technology searches (use web search instead), or when looking for tutorials rather than official documentation.",
	}
}

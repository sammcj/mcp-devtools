package aceternityui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// AceternityUITool provides access to Aceternity UI component library
type AceternityUITool struct{}

func init() {
	registry.Register(&AceternityUITool{})
}

// Definition returns the tool's definition for MCP registration
func (t *AceternityUITool) Definition() mcp.Tool {
	return mcp.NewTool(
		"aceternity_ui",
		mcp.WithDescription(`List, search, get details for Aceternity UI frontend components.

Actions:
- list: Get all components
- search: Search component by keyword in name, description, or tags
- details: Get information about a component
- categories: List categories

Examples:
- List categories: {"action": "categories"}
- List all components: {"action": "list"}
- Search for grid components: {"action": "search", "query": "grid"}
- Get bento-grid details: {"action": "details", "componentName": "bento-grid"}`),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action: 'list', 'search', 'details', or 'categories'"),
			mcp.Enum("list", "search", "details", "categories"),
		),
		mcp.WithString("query",
			mcp.Description("Search query (required for 'search' action)"),
		),
		mcp.WithString("componentName",
			mcp.Description("Component name (required for 'details' action)"),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false), // Uses hardcoded data, no external calls
	)
}

// Execute executes the Aceternity UI tool
func (t *AceternityUITool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: action")
	}

	logger.WithField("action", action).Info("Executing Aceternity UI tool")

	switch action {
	case "list":
		return t.executeList(logger)
	case "search":
		query, ok := args["query"].(string)
		if !ok || query == "" {
			return nil, fmt.Errorf("query parameter is required for search action")
		}
		return t.executeSearch(logger, query)
	case "details":
		componentName, ok := args["componentName"].(string)
		if !ok || componentName == "" {
			return nil, fmt.Errorf("componentName parameter is required for details action")
		}
		return t.executeDetails(logger, componentName)
	case "categories":
		return t.executeCategories(logger)
	default:
		return nil, fmt.Errorf("invalid action: %s. Must be one of: list, search, details, categories", action)
	}
}

// executeList handles the list action
func (t *AceternityUITool) executeList(logger *logrus.Logger) (*mcp.CallToolResult, error) {
	logger.Info("Listing Aceternity UI components")

	resultJSON, err := json.MarshalIndent(AceternityComponents, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal components: %w", err)
	}

	logger.WithField("count", len(AceternityComponents)).Info("Successfully retrieved Aceternity UI components")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// executeSearch handles the search action
func (t *AceternityUITool) executeSearch(logger *logrus.Logger, query string) (*mcp.CallToolResult, error) {
	logger.Infof("Searching Aceternity UI components with query: %s", query)

	var searchResults []ComponentInfo
	lowerQuery := strings.ToLower(query)

	for _, component := range AceternityComponents {
		// Search in name, description, and tags
		if strings.Contains(strings.ToLower(component.Name), lowerQuery) ||
			strings.Contains(strings.ToLower(component.Description), lowerQuery) ||
			containsTag(component.Tags, lowerQuery) {
			searchResults = append(searchResults, component)
		}
	}

	logger.Infof("Found %d components matching query: %s", len(searchResults), query)

	resultJSON, err := json.MarshalIndent(searchResults, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search results: %w", err)
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// executeDetails handles the details action
func (t *AceternityUITool) executeDetails(logger *logrus.Logger, componentName string) (*mcp.CallToolResult, error) {
	logger.Infof("Getting details for Aceternity UI component: %s", componentName)

	// Find the component
	var foundComponent *ComponentInfo
	for _, component := range AceternityComponents {
		if component.Name == componentName {
			foundComponent = &component
			break
		}
	}

	if foundComponent == nil {
		return nil, fmt.Errorf("component not found: %s", componentName)
	}

	logger.Infof("Successfully retrieved details for component: %s", componentName)

	resultJSON, err := json.MarshalIndent(foundComponent, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal component: %w", err)
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// executeCategories handles the categories action
func (t *AceternityUITool) executeCategories(logger *logrus.Logger) (*mcp.CallToolResult, error) {
	logger.Info("Listing Aceternity UI component categories")

	resultJSON, err := json.MarshalIndent(ComponentCategories, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal categories: %w", err)
	}

	logger.WithField("count", len(ComponentCategories)).Info("Successfully retrieved component categories")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// containsTag checks if a tag list contains a query string
func containsTag(tags []string, query string) bool {
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}

// ProvideExtendedInfo provides detailed usage information for the Aceternity UI tool
func (t *AceternityUITool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "List all available Aceternity UI components",
				Arguments: map[string]any{
					"action": "list",
				},
				ExpectedResult: "Returns all 24+ animated Aceternity UI components with names, descriptions, and installation commands",
			},
			{
				Description: "Search for background components",
				Arguments: map[string]any{
					"action": "search",
					"query":  "background",
				},
				ExpectedResult: "Returns components related to backgrounds (background-beams, hero-highlight, etc.)",
			},
			{
				Description: "Get details about the bento-grid component",
				Arguments: map[string]any{
					"action":        "details",
					"componentName": "bento-grid",
				},
				ExpectedResult: "Returns detailed info about bento-grid including dependencies, tags, and installation command",
			},
			{
				Description: "List all component categories",
				Arguments: map[string]any{
					"action": "categories",
				},
				ExpectedResult: "Returns all categories with their descriptions and component lists",
			},
		},
		CommonPatterns: []string{
			"Start with 'categories' to see component organisation",
			"Use 'list' to see all available animated components",
			"Use 'search' to find components by keyword (e.g., 'text', 'card', 'animation')",
			"Get component 'details' to see installation commands and dependencies",
			"All components use Framer Motion for animations",
			"Installation: npx shadcn@latest add [registry-url]",
			"Common dependencies: motion, tailwindcss, clsx, tailwind-merge",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Component not found error",
				Solution: "Use 'list' or 'search' first to get exact component names. Names use lowercase with hyphens (e.g., 'bento-grid', not 'BentoGrid').",
			},
			{
				Problem:  "Search returns too many results",
				Solution: "Use more specific keywords like 'text-generate' instead of 'text', or search by category name like 'navigation' or 'form'.",
			},
			{
				Problem:  "Need to find components by category",
				Solution: "Use the 'categories' action first to see all categories and their components, then get details on specific components.",
			},
		},
		ParameterDetails: map[string]string{
			"action":        "Operation to perform. 'list' shows all components, 'search' finds by keyword in name/description/tags, 'details' gets full component info, 'categories' lists all categories.",
			"query":         "Search term for finding components. Searches across component names, descriptions, and tags. Use keywords like 'text', 'card', 'background', 'animation'.",
			"componentName": "Exact component name from the list (lowercase with hyphens). Get correct names from 'list', 'search', or 'categories' actions first.",
		},
		WhenToUse:    "Use when building React/Next.js applications with Aceternity UI's animated components. Ideal for discovering the 24+ free animated components, finding effects and animations, and understanding installation and dependencies. Perfect for modern, visually appealing web applications.",
		WhenNotToUse: "Don't use for non-React frameworks (Vue, Angular, Svelte), static sites without JavaScript, or when animations aren't needed. Aceternity UI components require Framer Motion and are optimised for React.",
	}
}

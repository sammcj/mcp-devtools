package magicui

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

// MagicUITool provides access to Magic UI component library
type MagicUITool struct{}

func init() {
	registry.Register(&MagicUITool{})
}

// Definition returns the tool's definition for MCP registration
func (t *MagicUITool) Definition() mcp.Tool {
	return mcp.NewTool(
		"magic_ui",
		mcp.WithDescription(`List, search, get details for Magic UI frontend components.

Actions:
- list: Get all components
- search: Search by keyword in name or description
- details: Get information about a component

Examples:
- List all components: {"action": "list"}
- Search for text components: {"action": "search", "query": "text"}
- Get marquee details: {"action": "details", "componentName": "marquee"}`),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform: 'list', 'search', or 'details'"),
			mcp.Enum("list", "search", "details"),
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
		mcp.WithOpenWorldHintAnnotation(true),
	)
}

// Execute executes the Magic UI tool
func (t *MagicUITool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: action")
	}

	logger.WithField("action", action).Info("Executing Magic UI tool")

	switch action {
	case "list":
		return t.executeList(ctx, logger, cache)
	case "search":
		query, ok := args["query"].(string)
		if !ok || query == "" {
			return nil, fmt.Errorf("query parameter is required for search action")
		}
		return t.executeSearch(ctx, logger, cache, query)
	case "details":
		componentName, ok := args["componentName"].(string)
		if !ok || componentName == "" {
			return nil, fmt.Errorf("componentName parameter is required for details action")
		}
		return t.executeDetails(ctx, logger, cache, componentName)
	default:
		return nil, fmt.Errorf("invalid action: %s. Must be one of: list, search, details", action)
	}
}

// executeList handles the list action
func (t *MagicUITool) executeList(_ context.Context, logger *logrus.Logger, cache *sync.Map) (*mcp.CallToolResult, error) {
	logger.Info("Listing Magic UI components")

	components, err := t.fetchComponents(logger, cache)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch components: %w", err)
	}

	logger.WithField("count", len(components)).Info("Successfully fetched Magic UI components")

	resultJSON, err := json.MarshalIndent(components, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal components: %w", err)
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// executeSearch handles the search action
func (t *MagicUITool) executeSearch(_ context.Context, logger *logrus.Logger, cache *sync.Map, query string) (*mcp.CallToolResult, error) {
	logger.Infof("Searching Magic UI components with query: %s", query)

	allComponents, err := t.fetchComponents(logger, cache)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch components for search: %w", err)
	}

	var searchResults []ComponentInfo
	lowerQuery := strings.ToLower(query)

	for _, component := range allComponents {
		if strings.Contains(strings.ToLower(component.Name), lowerQuery) ||
			strings.Contains(strings.ToLower(component.Title), lowerQuery) ||
			strings.Contains(strings.ToLower(component.Description), lowerQuery) {
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
func (t *MagicUITool) executeDetails(_ context.Context, logger *logrus.Logger, cache *sync.Map, componentName string) (*mcp.CallToolResult, error) {
	logger.Infof("Getting details for Magic UI component: %s", componentName)

	cacheKey := componentDetailsCachePrefix + componentName

	// Check cache
	if cachedData, ok := cache.Load(cacheKey); ok {
		if entry, ok := cachedData.(CacheEntry); ok && time.Since(entry.Timestamp) < componentDetailsCacheTTL {
			logger.Debugf("Returning cached details for component: %s", componentName)
			resultJSON, err := json.MarshalIndent(entry.Data, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal cached component: %w", err)
			}
			return mcp.NewToolResultText(string(resultJSON)), nil
		}
	}

	allComponents, err := t.fetchComponents(logger, cache)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch components: %w", err)
	}

	// Find the component
	var foundComponent *ComponentInfo
	for _, component := range allComponents {
		if component.Name == componentName {
			foundComponent = &component
			break
		}
	}

	if foundComponent == nil {
		return nil, fmt.Errorf("component not found: %s", componentName)
	}

	// Add documentation URL
	foundComponent.Files = append(foundComponent.Files, File{
		Path: fmt.Sprintf("%s/docs/components/%s", MagicUIDocsURL, componentName),
		Type: "documentation",
	})

	// Store in cache
	cache.Store(cacheKey, CacheEntry{
		Data:      foundComponent,
		Timestamp: time.Now(),
	})

	logger.Infof("Successfully fetched details for component: %s", componentName)

	resultJSON, err := json.MarshalIndent(foundComponent, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal component: %w", err)
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// fetchComponents fetches and caches the component list from the registry
func (t *MagicUITool) fetchComponents(logger *logrus.Logger, cache *sync.Map) ([]ComponentInfo, error) {
	// Check cache
	if cachedData, ok := cache.Load(registryCacheKey); ok {
		if entry, ok := cachedData.(CacheEntry); ok && time.Since(entry.Timestamp) < registryCacheTTL {
			logger.Debug("Returning cached Magic UI registry")
			return entry.Data.([]ComponentInfo), nil
		}
	}

	// Fetch from GitHub
	ops := security.NewOperations("magicui")
	safeResp, err := ops.SafeHTTPGet(MagicUIRegistryURL)
	if err != nil {
		if secErr, ok := err.(*security.SecurityError); ok {
			return nil, fmt.Errorf("security block [ID: %s]: %s", secErr.GetSecurityID(), secErr.Error())
		}
		return nil, fmt.Errorf("failed to fetch Magic UI registry: %w", err)
	}

	if safeResp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch Magic UI registry: status %d", safeResp.StatusCode)
	}

	// Handle security warnings
	if safeResp.SecurityResult != nil && safeResp.SecurityResult.Action == security.ActionWarn {
		logger.Warnf("Security warning [ID: %s]: %s", safeResp.SecurityResult.ID, safeResp.SecurityResult.Message)
	}

	// Parse registry
	var regData Registry
	if err := json.Unmarshal(safeResp.Content, &regData); err != nil {
		return nil, fmt.Errorf("failed to parse Magic UI registry: %w", err)
	}

	// Filter to only UI components (type: "registry:ui")
	var components []ComponentInfo
	for _, item := range regData.Items {
		if item.Type == "registry:ui" {
			components = append(components, ComponentInfo{
				Name:         item.Name,
				Title:        item.Title,
				Description:  item.Description,
				Dependencies: item.Dependencies,
				Files:        item.Files,
			})
		}
	}

	// Store in cache
	cache.Store(registryCacheKey, CacheEntry{
		Data:      components,
		Timestamp: time.Now(),
	})

	logger.Infof("Successfully fetched %d Magic UI components from registry", len(components))
	return components, nil
}

// ProvideExtendedInfo provides detailed usage information for the Magic UI tool
func (t *MagicUITool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "List all available Magic UI components",
				Arguments: map[string]any{
					"action": "list",
				},
				ExpectedResult: "Returns all 74+ animated Magic UI components with names, titles, and descriptions",
			},
			{
				Description: "Search for text animation components",
				Arguments: map[string]any{
					"action": "search",
					"query":  "text",
				},
				ExpectedResult: "Returns components related to text animations (aurora-text, morphing-text, etc.)",
			},
			{
				Description: "Get details about the marquee component",
				Arguments: map[string]any{
					"action":        "details",
					"componentName": "marquee",
				},
				ExpectedResult: "Returns detailed info about marquee including dependencies, file paths, and documentation URL",
			},
			{
				Description: "Search for particle effects",
				Arguments: map[string]any{
					"action": "search",
					"query":  "particle",
				},
				ExpectedResult: "Returns particle-related components (particles, meteors, etc.)",
			},
		},
		CommonPatterns: []string{
			"Start with 'list' action to see all available animated components",
			"Use 'search' to find components by keyword (e.g., 'text', 'button', 'card', 'background')",
			"Get component 'details' to see dependencies and file information",
			"All components use Framer Motion for animations",
			"Common dependencies: motion (framer-motion), tw-animate-css",
			"Installation: npx magicui-cli add [component-name]",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Component not found error",
				Solution: "Use 'list' or 'search' first to get exact component names. Names use lowercase with hyphens (e.g., 'magic-card', not 'MagicCard').",
			},
			{
				Problem:  "Missing dependencies information",
				Solution: "Some components have no external dependencies beyond the base Magic UI setup. Check the 'dependencies' field in component details.",
			},
			{
				Problem:  "Search returns too many results",
				Solution: "Use more specific keywords like 'aurora-text' instead of just 'text', or 'neon-gradient' instead of 'gradient'.",
			},
		},
		ParameterDetails: map[string]string{
			"action":        "Operation to perform. 'list' shows all components, 'search' finds by keyword in name/title/description, 'details' gets full component info.",
			"query":         "Search term for finding components. Searches across component names, titles, and descriptions. Use keywords like 'text', 'card', 'background', 'button'.",
			"componentName": "Exact component name from the list (lowercase with hyphens). Get correct names from 'list' or 'search' actions first.",
		},
		WhenToUse:    "Use when building React/Next.js applications with animated UI components. Ideal for discovering Magic UI's 74+ free animated components, finding micro-interactions, and understanding component dependencies. Perfect for marketing sites, portfolios, and modern web apps.",
		WhenNotToUse: "Don't use for non-React frameworks (Vue, Angular, Svelte), static sites without JavaScript, or when animations aren't needed. Magic UI components require Framer Motion and are optimised for React.",
	}
}

package unified

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch/brave"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch/searxng"
	"github.com/sirupsen/logrus"
)

// InternetSearchTool provides a single interface for multiple search providers
type InternetSearchTool struct {
	providers map[string]SearchProvider
}

// SearchProvider defines the interface all search providers must implement
type SearchProvider interface {
	Search(ctx context.Context, logger *logrus.Logger, searchType string, args map[string]interface{}) (*internetsearch.SearchResponse, error)
	GetName() string
	IsAvailable() bool
	GetSupportedTypes() []string
}

func init() {
	tool := &InternetSearchTool{
		providers: make(map[string]SearchProvider),
	}

	// Register available providers
	if braveProvider := brave.NewBraveProvider(); braveProvider != nil && braveProvider.IsAvailable() {
		tool.providers["brave"] = braveProvider
	}

	if searxngProvider := searxng.NewSearXNGProvider(); searxngProvider != nil && searxngProvider.IsAvailable() {
		tool.providers["searxng"] = searxngProvider
	}

	// Only register if we have at least one provider
	if len(tool.providers) > 0 {
		registry.Register(tool)
	}
}

// Definition returns the tool's definition for MCP registration
func (t *InternetSearchTool) Definition() mcp.Tool {
	// Get available providers for description
	availableProviders := make([]string, 0, len(t.providers))
	for name := range t.providers {
		availableProviders = append(availableProviders, name)
	}

	// Get all supported search types
	supportedTypes := make(map[string]bool)
	for _, provider := range t.providers {
		for _, searchType := range provider.GetSupportedTypes() {
			supportedTypes[searchType] = true
		}
	}

	typesList := make([]string, 0, len(supportedTypes))
	for searchType := range supportedTypes {
		typesList = append(typesList, searchType)
	}

	// Default provider (prefer brave if available, otherwise first available)
	defaultProvider := "brave"
	if _, exists := t.providers[defaultProvider]; !exists {
		for name := range t.providers {
			defaultProvider = name
			break
		}
	}

	// Check which providers are available
	_, hasBrave := t.providers["brave"]
	_, hasSearXNG := t.providers["searxng"]

	// Build provider-specific parameter description
	var providerSpecificParams []string
	if hasBrave {
		providerSpecificParams = append(providerSpecificParams, "- Brave: freshness (pd/pw/pm/py), offset (web search only)")
	}
	if hasSearXNG {
		providerSpecificParams = append(providerSpecificParams, "- SearXNG: pageno, time_range (day/month/year), language, safesearch")
	}

	description := fmt.Sprintf(`Search the internet for information and links.

Available Providers: [%s]
Default Provider: %s

Search Types: %v

Examples:
- Web search: {"query": "golang best practices", "count": 10}
- Image search: {"type": "image", "query": "golang gopher mascot", "count": 3}
- News search: {"type": "news", "query": "AI breakthrough", "time_range": "day"}
- Video search: {"type": "video", "query": "golang tutorial"}
- Local search: {"type": "local", "query": "pizza near Central Park"}

Provider-specific optional parameters:
%s`,
		strings.Join(availableProviders, ", "), defaultProvider, typesList, strings.Join(providerSpecificParams, "\n"))

	enumValues := make([]string, 0, len(typesList))
	enumValues = append(enumValues, typesList...)

	providerEnumValues := make([]string, 0, len(availableProviders))
	providerEnumValues = append(providerEnumValues, availableProviders...)

	// Start building the tool definition with common parameters
	toolOptions := []mcp.ToolOption{
		mcp.WithDescription(description),
		mcp.WithString("type",
			mcp.Description("Search type"),
			mcp.DefaultString("web"),
			mcp.Enum(enumValues...),
		),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query term"),
		),
		mcp.WithString("provider",
			mcp.Description(fmt.Sprintf("Search provider to use (default: %s)", defaultProvider)),
			mcp.DefaultString(defaultProvider),
			mcp.Enum(providerEnumValues...),
		),
		mcp.WithNumber("count",
			mcp.Description("Number of results (limits vary by provider and type)"),
			mcp.DefaultNumber(5),
		),
	}

	// Add provider-specific parameters only if the provider is available
	if hasBrave {
		toolOptions = append(toolOptions,
			mcp.WithNumber("offset",
				mcp.Description("Pagination offset (Brave web search only)"),
				mcp.DefaultNumber(0),
			),
			mcp.WithString("freshness",
				mcp.Description("Time filter for Brave (pd/pw/pm/py or custom range)"),
			),
		)
	}

	if hasSearXNG {
		toolOptions = append(toolOptions,
			mcp.WithNumber("pageno",
				mcp.Description("Page number for SearXNG (starts at 1)"),
				mcp.DefaultNumber(1),
			),
			mcp.WithString("time_range",
				mcp.Description("Time range for SearXNG (day/month/year)"),
				mcp.Enum("day", "month", "year"),
			),
			mcp.WithString("language",
				mcp.Description("Language code for SearXNG (e.g., 'all', 'en', 'fr', 'de')"),
				mcp.DefaultString("en"),
			),
			mcp.WithString("safesearch",
				mcp.Description("Safe search filter for SearXNG (0: None, 1: Moderate, 2: Strict)"),
				mcp.Enum("0", "1", "2"),
				mcp.DefaultString("1"),
			),
		)
	}

	return mcp.NewTool("internet_search", toolOptions...)
}

// Execute executes the unified search tool
func (t *InternetSearchTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse parameters (with default for type)
	searchType, ok := args["type"].(string)
	if !ok || searchType == "" {
		searchType = "web" // Default to web search
	}

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: query")
	}

	// Get provider (default to brave if available, otherwise first available)
	providerName := "brave"
	if _, exists := t.providers[providerName]; !exists {
		for name := range t.providers {
			providerName = name
			break
		}
	}

	if providerRaw, ok := args["provider"].(string); ok && providerRaw != "" {
		providerName = providerRaw
	}

	provider, exists := t.providers[providerName]
	if !exists {
		return nil, fmt.Errorf("provider not available: %s. Available providers: %v", providerName, t.getAvailableProviders())
	}

	// Check if provider supports the search type
	if !t.providerSupportsType(provider, searchType) {
		return nil, fmt.Errorf("provider %s does not support search type: %s. Supported types: %v",
			providerName, searchType, provider.GetSupportedTypes())
	}

	logger.WithFields(logrus.Fields{
		"provider": providerName,
		"type":     searchType,
		"query":    query,
	}).Info("Executing internet search")

	// Execute search with the selected provider
	response, err := provider.Search(ctx, logger, searchType, args)
	if err != nil {
		return nil, fmt.Errorf("search failed with provider %s: %w", providerName, err)
	}

	return internetsearch.NewToolResultJSON(response)
}

// Helper methods
func (t *InternetSearchTool) getAvailableProviders() []string {
	providers := make([]string, 0, len(t.providers))
	for name := range t.providers {
		providers = append(providers, name)
	}
	return providers
}

func (t *InternetSearchTool) providerSupportsType(provider SearchProvider, searchType string) bool {
	for _, supportedType := range provider.GetSupportedTypes() {
		if supportedType == searchType {
			return true
		}
	}
	return false
}

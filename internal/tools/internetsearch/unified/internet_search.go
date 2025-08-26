package unified

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch/brave"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch/duckduckgo"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch/searxng"
	"github.com/sirupsen/logrus"
)

// InternetSearchTool provides a single interface for multiple search providers
type InternetSearchTool struct {
	providers map[string]SearchProvider
}

// SearchProvider defines the interface all search providers must implement
type SearchProvider interface {
	Search(ctx context.Context, logger *logrus.Logger, searchType string, args map[string]any) (*internetsearch.SearchResponse, error)
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

	// DuckDuckGo is always available since it doesn't require an API key
	if duckduckgoProvider := duckduckgo.NewDuckDuckGoProvider(); duckduckgoProvider != nil && duckduckgoProvider.IsAvailable() {
		tool.providers["duckduckgo"] = duckduckgoProvider
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

	// Default provider priority: brave > searxng > duckduckgo
	var defaultProvider string
	if _, exists := t.providers["brave"]; exists {
		defaultProvider = "brave"
	} else if _, exists := t.providers["searxng"]; exists {
		defaultProvider = "searxng"
	} else if _, exists := t.providers["duckduckgo"]; exists {
		defaultProvider = "duckduckgo"
	} else {
		// Fallback to first available provider
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

Provider-specific optional parameters:
%s

After you have received the results you can fetch the url if you want to read the full content.
`,
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
			mcp.Description("Number of results (limits vary by provider & type)"),
			mcp.DefaultNumber(4),
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
func (t *InternetSearchTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Parse parameters (with default for type)
	searchType, ok := args["type"].(string)
	if !ok || searchType == "" {
		searchType = "web" // Default to web search
	}

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: query")
	}

	// Get provider using the same priority logic as in Definition
	var providerName string
	if _, exists := t.providers["brave"]; exists {
		providerName = "brave"
	} else if _, exists := t.providers["searxng"]; exists {
		providerName = "searxng"
	} else if _, exists := t.providers["duckduckgo"]; exists {
		providerName = "duckduckgo"
	} else {
		// Fallback to first available provider
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

	// Analyse search results for security threats
	if security.IsEnabled() && response != nil {
		for i, result := range response.Results {
			source := security.SourceContext{
				Tool:        "internet_search",
				Domain:      providerName,
				ContentType: "search_results",
			}
			// Analyse the search result content
			content := result.Title + " " + result.Description
			if secResult, err := security.AnalyseContent(content, source); err == nil {
				switch secResult.Action {
				case security.ActionBlock:
					return nil, fmt.Errorf("search result blocked by security policy: %s", secResult.Message)
				case security.ActionWarn:
					// Add security notice to result metadata
					if result.Metadata == nil {
						result.Metadata = make(map[string]any)
					}
					result.Metadata["security_warning"] = secResult.Message
					result.Metadata["security_id"] = secResult.ID
					logger.WithField("security_id", secResult.ID).Warn(secResult.Message)
				}
				// Update the result in the response
				response.Results[i] = result
			}
		}
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
	return slices.Contains(provider.GetSupportedTypes(), searchType)
}

// ProvideExtendedInfo provides detailed usage information for the internet search tool
func (t *InternetSearchTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	examples := []tools.ToolExample{
		{
			Description: "Basic web search with default provider",
			Arguments: map[string]any{
				"query": "golang best practices",
				"count": 5,
			},
			ExpectedResult: "Returns 5 web search results about Go programming best practices",
		},
		{
			Description: "News search with time filtering",
			Arguments: map[string]any{
				"type":  "news",
				"query": "artificial intelligence breakthrough",
				"count": 3,
			},
			ExpectedResult: "Returns 3 recent news articles about AI breakthroughs",
		},
		{
			Description: "Image search with specific provider",
			Arguments: map[string]any{
				"type":     "image",
				"query":    "golang gopher mascot",
				"provider": "brave",
				"count":    10,
			},
			ExpectedResult: "Returns 10 images of the Go programming language mascot using Brave search",
		},
	}

	// Add provider-specific examples if available
	if t.hasProvider("brave") {
		examples = append(examples, tools.ToolExample{
			Description: "Brave search with time filtering and pagination",
			Arguments: map[string]any{
				"query":     "machine learning tutorials",
				"provider":  "brave",
				"freshness": "pw", // Past week
				"offset":    10,   // Skip first 10 results
				"count":     5,
			},
			ExpectedResult: "Returns 5 ML tutorial results from the past week, starting from result 11",
		})
	}

	if t.hasProvider("searxng") {
		examples = append(examples, tools.ToolExample{
			Description: "SearXNG search with language and safe search settings",
			Arguments: map[string]any{
				"query":      "programming tutorials",
				"provider":   "searxng",
				"language":   "en",
				"safesearch": "1",
				"pageno":     2,
			},
			ExpectedResult: "Returns programming tutorials in English with moderate safe search, page 2",
		})
	}

	commonPatterns := []string{
		"Use count parameter to control result volume (more results = more context but higher latency)",
		"Combine with web_fetch tool to get full content from interesting search results",
		"For research workflows: search → analyse results → fetch detailed content → store in memory",
	}

	// Add provider-specific patterns only for available providers
	if t.hasProvider("brave") && t.hasProvider("duckduckgo") {
		commonPatterns = append(commonPatterns, "Use provider parameter to choose between Brave (with API features) and DuckDuckGo (always available)")
	} else if t.hasProvider("searxng") && t.hasProvider("duckduckgo") {
		commonPatterns = append(commonPatterns, "Use provider parameter to choose between SearXNG (with language options) and DuckDuckGo (always available)")
	}

	// Add search type guidance based on available providers
	supportedTypes := make(map[string]bool)
	for _, provider := range t.providers {
		for _, searchType := range provider.GetSupportedTypes() {
			supportedTypes[searchType] = true
		}
	}

	if len(supportedTypes) > 1 {
		var types []string
		for searchType := range supportedTypes {
			types = append(types, searchType)
		}
		commonPatterns = append(commonPatterns, fmt.Sprintf("Available search types with current providers: %s", strings.Join(types, ", ")))
	}

	troubleshooting := []tools.TroubleshootingTip{
		{
			Problem:  "No search results returned",
			Solution: "Try different search terms, reduce specificity, or check for typos in your query.",
		},
	}

	// Add provider-specific troubleshooting only for available providers
	var apiRequirements []string
	if t.hasProvider("brave") {
		apiRequirements = append(apiRequirements, "BRAVE_API_KEY for Brave")
	}
	if t.hasProvider("searxng") {
		apiRequirements = append(apiRequirements, "SEARXNG_BASE_URL for SearXNG")
	}

	if len(apiRequirements) > 0 {
		troubleshooting = append(troubleshooting, tools.TroubleshootingTip{
			Problem:  "Search returns 'provider not available' error",
			Solution: fmt.Sprintf("Check that required API keys/URLs are set: %s. DuckDuckGo requires no setup.", strings.Join(apiRequirements, ", ")),
		})
	}

	// Add search type troubleshooting if multiple providers with different capabilities
	if len(t.providers) > 1 {
		troubleshooting = append(troubleshooting, tools.TroubleshootingTip{
			Problem:  "Search type not supported by chosen provider",
			Solution: "Different providers support different search types. Use default provider selection or check supported types in error message.",
		})
	}

	// Add rate limiting troubleshooting if there are multiple providers to switch between
	if len(t.providers) > 1 {
		troubleshooting = append(troubleshooting, tools.TroubleshootingTip{
			Problem:  "Rate limit errors",
			Solution: "Wait before retrying, or switch to a different provider using the 'provider' parameter. Each provider has different rate limits.",
		})
	} else {
		troubleshooting = append(troubleshooting, tools.TroubleshootingTip{
			Problem:  "Rate limit errors",
			Solution: "Wait before retrying. Rate limits vary by provider and search type.",
		})
	}

	parameterDetails := map[string]string{
		"query": "The search query should be descriptive but not too long. Use natural language rather than keyword stuffing.",
		"type":  "Web search is default and most versatile. Use 'news' for current events, 'image' for visual content, 'video' for tutorials.",
		"count": "More results provide broader coverage but increase latency. Typical range: 3-10 results for focused searches, 10-20 for research.",
	}

	// Build provider description based on available providers
	var providerDescriptions []string
	if t.hasProvider("brave") {
		providerDescriptions = append(providerDescriptions, "Brave (requires API key) offers freshness filtering")
	}
	if t.hasProvider("searxng") {
		providerDescriptions = append(providerDescriptions, "SearXNG (requires instance URL) offers language options")
	}
	if t.hasProvider("duckduckgo") {
		providerDescriptions = append(providerDescriptions, "DuckDuckGo (always available)")
	}

	if len(providerDescriptions) > 0 {
		parameterDetails["provider"] = strings.Join(providerDescriptions, ". ")
	}

	// Add provider-specific parameter details only for available providers
	if t.hasProvider("brave") {
		parameterDetails["freshness"] = "Brave only: 'pd' (past day), 'pw' (past week), 'pm' (past month), 'py' (past year). Useful for current events."
		parameterDetails["offset"] = "Brave only: Skip first N results for pagination. Useful for getting more diverse results."
	}

	if t.hasProvider("searxng") {
		parameterDetails["language"] = "SearXNG only: Use language codes like 'en', 'fr', 'de', or 'all'. Affects both query processing and result filtering."
		parameterDetails["safesearch"] = "SearXNG only: Safe search filter (0: None, 1: Moderate, 2: Strict). Default is moderate."
		parameterDetails["pageno"] = "SearXNG only: Page number starting from 1. Use for pagination through results."
		parameterDetails["time_range"] = "SearXNG only: Filter by time (day/month/year). Useful for recent content."
	}

	whenToUse := "Use internet search to find current information, research topics, discover resources, or gather multiple perspectives on a subject. Ideal for tasks requiring up-to-date information that may not be in training data."

	whenNotToUse := "Avoid for: well-established facts available in training data, private/internal information, real-time data requiring live APIs, or when you already have specific URLs to fetch content from."

	return &tools.ExtendedHelp{
		Examples:         examples,
		CommonPatterns:   commonPatterns,
		Troubleshooting:  troubleshooting,
		ParameterDetails: parameterDetails,
		WhenToUse:        whenToUse,
		WhenNotToUse:     whenNotToUse,
	}
}

// hasProvider checks if a specific provider is available
func (t *InternetSearchTool) hasProvider(providerName string) bool {
	_, exists := t.providers[providerName]
	return exists
}

package unified

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch/brave"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch/duckduckgo"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch/google"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch/kagi"
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

const (
	// fallbackBaseDelay is the base delay between fallback attempts (multiplied by attempt number)
	fallbackBaseDelay = 1 * time.Second
	// defaultMaxParallelSearches is the default number of concurrent search queries
	defaultMaxParallelSearches = 3
)

// providerPriorityOrder defines the order providers are tried during fallback
var providerPriorityOrder = []string{"brave", "google", "kagi", "searxng", "duckduckgo"}

// maxParallelSearches controls how many queries can execute concurrently
var maxParallelSearches = defaultMaxParallelSearches

// queryWork represents a single query to be processed by a worker
type queryWork struct {
	index int
	query string
}

func init() {
	// Configure max parallel searches from environment
	if v := os.Getenv("INTERNET_SEARCH_MAX_PARALLEL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxParallelSearches = n
		}
	}

	tool := &InternetSearchTool{
		providers: make(map[string]SearchProvider),
	}

	// Register available providers
	if braveProvider := brave.NewBraveProvider(); braveProvider != nil && braveProvider.IsAvailable() {
		tool.providers["brave"] = braveProvider
	}

	if googleProvider := google.NewGoogleProvider(); googleProvider != nil && googleProvider.IsAvailable() {
		tool.providers["google"] = googleProvider
	}

	if kagiProvider := kagi.NewKagiProvider(); kagiProvider != nil && kagiProvider.IsAvailable() {
		tool.providers["kagi"] = kagiProvider
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

	// Default provider based on priority order
	var defaultProvider string
	for _, providerName := range providerPriorityOrder {
		if _, exists := t.providers[providerName]; exists {
			defaultProvider = providerName
			break
		}
	}

	// If no provider from priority list, use first available
	if defaultProvider == "" {
		for name := range t.providers {
			defaultProvider = name
			break
		}
	}

	// Check which providers are available
	_, hasBrave := t.providers["brave"]
	_, hasGoogle := t.providers["google"]
	_, hasSearXNG := t.providers["searxng"]

	description := fmt.Sprintf(`Search the internet for information and links. Supports multiple queries executed in parallel.

Available Providers: [%s]
Default Provider: %s
Search Types: %v`,
		strings.Join(availableProviders, ", "), defaultProvider, typesList)

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
		mcp.WithArray("query",
			mcp.Required(),
			mcp.Description("One or more search queries to execute. Multiple queries run in parallel."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithString("provider",
			mcp.Description(fmt.Sprintf("Search provider to use (default: %s)", defaultProvider)),
			mcp.DefaultString(defaultProvider),
			mcp.Enum(providerEnumValues...),
		),
		mcp.WithNumber("count",
			mcp.Description("Number of results per query (limits vary by provider & type)"),
			mcp.DefaultNumber(5),
		),
	}

	// Add provider-specific parameters only if the provider is available
	if hasBrave {
		toolOptions = append(toolOptions,
			mcp.WithNumber("offset",
				mcp.Description("Pagination offset (Brave internet search only)"),
				mcp.DefaultNumber(0),
			),
			mcp.WithString("freshness",
				mcp.Description("Time filter for Brave (pd/pw/pm/py or custom range)"),
			),
		)
	}

	if hasGoogle {
		toolOptions = append(toolOptions,
			mcp.WithNumber("start",
				mcp.Description("Start index for Google search pagination (default: 0)"),
				mcp.DefaultNumber(0),
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

	// Add read-only annotations for internet search tool
	toolOptions = append(toolOptions,
		mcp.WithReadOnlyHintAnnotation(true),     // Only queries external APIs, doesn't modify environment
		mcp.WithDestructiveHintAnnotation(false), // No destructive operations
		mcp.WithIdempotentHintAnnotation(true),   // Same query returns similar results
		mcp.WithOpenWorldHintAnnotation(true),    // Interacts with external internet APIs
	)

	return mcp.NewTool("internet_search", toolOptions...)
}

// Execute executes the unified search tool with parallel query support
func (t *InternetSearchTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Parse search type (with default)
	searchType, ok := args["type"].(string)
	if !ok || searchType == "" {
		searchType = "web"
	}

	// Parse queries array
	queries, err := t.parseQueries(args)
	if err != nil {
		return nil, err
	}

	// Determine if user explicitly requested a specific provider
	userRequestedProvider := ""
	if providerRaw, ok := args["provider"].(string); ok && providerRaw != "" {
		userRequestedProvider = providerRaw
	}

	// Get ordered list of providers to try (with fallback support)
	providersToTry := t.getOrderedProviders(searchType, userRequestedProvider)
	if len(providersToTry) == 0 {
		return nil, fmt.Errorf("no available providers support search type: %s", searchType)
	}

	// Execute queries in parallel using worker pool
	numWorkers := min(maxParallelSearches, len(queries))
	queryChan := make(chan queryWork, len(queries))
	results := make([]internetsearch.QueryResult, len(queries))
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Start workers
	for range numWorkers {
		wg.Go(func() {
			for work := range queryChan {
				result := t.executeSingleSearch(ctx, logger, work.query, searchType, providersToTry, userRequestedProvider, args)
				mu.Lock()
				results[work.index] = result
				mu.Unlock()
			}
		})
	}

	// Queue all queries
	for i, q := range queries {
		queryChan <- queryWork{index: i, query: q}
	}
	close(queryChan)

	// Wait for all queries to complete
	wg.Wait()

	// Aggregate results
	return t.aggregateResults(results)
}

// parseQueries extracts the queries array from args
func (t *InternetSearchTool) parseQueries(args map[string]any) ([]string, error) {
	queryRaw, ok := args["query"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter 'query'. Provide search terms as an array (e.g., {\"query\": [\"golang best practices\"]})")
	}

	// Handle array of queries
	switch v := queryRaw.(type) {
	case []any:
		if len(v) == 0 {
			return nil, fmt.Errorf("'query' array cannot be empty")
		}
		queries := make([]string, 0, len(v))
		for i, item := range v {
			if s, ok := item.(string); ok && s != "" {
				queries = append(queries, s)
			} else {
				return nil, fmt.Errorf("query at index %d must be a non-empty string", i)
			}
		}
		return queries, nil
	case []string:
		if len(v) == 0 {
			return nil, fmt.Errorf("'query' array cannot be empty")
		}
		queries := make([]string, 0, len(v))
		for _, s := range v {
			if s != "" {
				queries = append(queries, s)
			}
		}
		if len(queries) == 0 {
			return nil, fmt.Errorf("'query' array cannot contain only empty strings")
		}
		return queries, nil
	default:
		return nil, fmt.Errorf("'query' must be an array of strings (e.g., {\"query\": [\"search term\"]})")
	}
}

// executeSingleSearch performs a search for a single query with provider fallback
func (t *InternetSearchTool) executeSingleSearch(ctx context.Context, logger *logrus.Logger, query, searchType string, providersToTry []string, userRequestedProvider string, args map[string]any) internetsearch.QueryResult {
	result := internetsearch.QueryResult{
		Query:   query,
		Results: []internetsearch.SearchResult{},
	}

	// Create args copy with this specific query for the provider
	searchArgs := make(map[string]any)
	maps.Copy(searchArgs, args)
	searchArgs["query"] = query

	var allErrors []string

	for i, providerName := range providersToTry {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			result.Error = fmt.Sprintf("search cancelled: %v", err)
			return result
		}

		// Add delay between fallback attempts
		if i > 0 {
			delay := time.Duration(i) * fallbackBaseDelay
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				result.Error = fmt.Sprintf("search cancelled during fallback: %v", ctx.Err())
				return result
			}
		}

		provider, exists := t.providers[providerName]
		if !exists || !t.providerSupportsType(provider, searchType) {
			continue
		}

		logger.WithFields(logrus.Fields{
			"provider": providerName,
			"type":     searchType,
			"query":    query,
		}).Debug("Executing search query")

		response, err := provider.Search(ctx, logger, searchType, searchArgs)
		if err != nil {
			errorMsg := fmt.Sprintf("%s: %v", providerName, err)
			allErrors = append(allErrors, errorMsg)

			if userRequestedProvider != "" || i == len(providersToTry)-1 {
				if len(allErrors) > 1 {
					result.Error = fmt.Sprintf("all providers failed: %s", strings.Join(allErrors, "; "))
				} else {
					result.Error = fmt.Sprintf("search failed: %s", errorMsg)
				}
				return result
			}
			continue
		}

		// Apply security analysis
		if security.IsEnabled() && response != nil {
			for resultIdx, searchResult := range response.Results {
				source := security.SourceContext{
					Tool:        "internet_search",
					Domain:      providerName,
					ContentType: "search_results",
				}
				content := searchResult.Title + " " + searchResult.Description
				if secResult, secErr := security.AnalyseContent(content, source); secErr == nil {
					switch secResult.Action {
					case security.ActionBlock:
						result.Error = fmt.Sprintf("search result blocked by security policy: %s", secResult.Message)
						return result
					case security.ActionWarn:
						if searchResult.Metadata == nil {
							searchResult.Metadata = make(map[string]any)
						}
						searchResult.Metadata["security_warning"] = secResult.Message
						response.Results[resultIdx] = searchResult
					}
				}
			}
		}

		// Success - populate result
		result.Results = response.Results
		result.Provider = providerName
		return result
	}

	result.Error = "no providers could complete the search"
	return result
}

// aggregateResults combines individual query results into a MultiSearchResponse
func (t *InternetSearchTool) aggregateResults(results []internetsearch.QueryResult) (*mcp.CallToolResult, error) {
	successful := 0
	failed := 0
	var errors []string

	for _, r := range results {
		if r.Error == "" {
			successful++
		} else {
			failed++
			errors = append(errors, fmt.Sprintf("%s: %s", r.Query, r.Error))
		}
	}

	// Return error if all queries failed
	if successful == 0 && failed > 0 {
		return nil, fmt.Errorf("all queries failed: %s", strings.Join(errors, "; "))
	}

	response := internetsearch.MultiSearchResponse{
		Searches: results,
		Summary: internetsearch.SearchSummary{
			Total:      len(results),
			Successful: successful,
			Failed:     failed,
		},
	}

	return internetsearch.NewToolResultJSON(response)
}

// Helper methods
func (t *InternetSearchTool) providerSupportsType(provider SearchProvider, searchType string) bool {
	return slices.Contains(provider.GetSupportedTypes(), searchType)
}

// getOrderedProviders returns an ordered list of providers to try for the given search type
// If userRequestedProvider is set, only that provider is returned
// Otherwise, returns all providers supporting the search type in priority order
func (t *InternetSearchTool) getOrderedProviders(searchType, userRequestedProvider string) []string {
	// If user explicitly requested a provider, only use that one (no fallback)
	if userRequestedProvider != "" {
		if provider, exists := t.providers[userRequestedProvider]; exists {
			if t.providerSupportsType(provider, searchType) {
				return []string{userRequestedProvider}
			}
		}
		return []string{}
	}

	// Build ordered list of providers that support the search type
	var orderedProviders []string
	for _, providerName := range providerPriorityOrder {
		if provider, exists := t.providers[providerName]; exists {
			if t.providerSupportsType(provider, searchType) {
				orderedProviders = append(orderedProviders, providerName)
			}
		}
	}

	// Add any remaining providers not in priority order (for future extensibility)
	for providerName, provider := range t.providers {
		if !slices.Contains(orderedProviders, providerName) && t.providerSupportsType(provider, searchType) {
			orderedProviders = append(orderedProviders, providerName)
		}
	}

	return orderedProviders
}

// ProvideExtendedInfo provides detailed usage information for the internet search tool
func (t *InternetSearchTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	examples := []tools.ToolExample{
		{
			Description: "Basic internet search with default provider",
			Arguments: map[string]any{
				"query": []string{"golang best practices"},
				"count": 5,
			},
			ExpectedResult: "Returns 5 internet search results about Go programming best practices",
		},
		{
			Description: "Multiple queries in parallel",
			Arguments: map[string]any{
				"query": []string{"golang best practices", "rust vs go performance", "python async patterns"},
				"count": 3,
			},
			ExpectedResult: "Returns results for all 3 queries, executed in parallel",
		},
		{
			Description: "News search with time filtering",
			Arguments: map[string]any{
				"type":  "news",
				"query": []string{"artificial intelligence breakthrough"},
				"count": 3,
			},
			ExpectedResult: "Returns 3 recent news articles about AI breakthroughs",
		},
		{
			Description: "Image search with specific provider",
			Arguments: map[string]any{
				"type":     "image",
				"query":    []string{"golang gopher mascot"},
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
				"query":     []string{"machine learning tutorials"},
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
				"query":      []string{"programming tutorials"},
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
		"Combine with fetch_url tool to get full content from interesting search results",
		"For research workflows: search → analyse results → fetch detailed content → store in memory",
		"Automatic fallback: If the default provider fails, the tool automatically tries other available providers",
	}

	// Add provider-specific patterns only for available providers
	if t.hasProvider("brave") && t.hasProvider("duckduckgo") {
		commonPatterns = append(commonPatterns, "Use provider parameter to choose between Brave (with API features) and DuckDuckGo (always available)")
	} else if t.hasProvider("searxng") && t.hasProvider("duckduckgo") {
		commonPatterns = append(commonPatterns, "Use provider parameter to choose between SearXNG (with language options) and DuckDuckGo (always available)")
	}

	// Add fallback information if multiple providers exist
	if len(t.providers) > 1 {
		commonPatterns = append(commonPatterns, "Fallback is automatic when no specific provider is requested; specify a provider to disable fallback")
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
			Solution: "The tool automatically falls back to alternative providers when rate limits are hit. If all providers fail, wait before retrying. You can also explicitly specify a provider to bypass automatic fallback.",
		})
	} else {
		troubleshooting = append(troubleshooting, tools.TroubleshootingTip{
			Problem:  "Rate limit errors",
			Solution: "Wait before retrying. Rate limits vary by provider and search type.",
		})
	}

	// Add fallback-specific troubleshooting
	if len(t.providers) > 1 {
		troubleshooting = append(troubleshooting, tools.TroubleshootingTip{
			Problem:  "Want to see which provider was used",
			Solution: "Check the 'provider' field in the search response. If fallback occurred, results will include 'fallback_used' and 'original_provider_errors' in metadata.",
		})
	}

	parameterDetails := map[string]string{
		"query": "The search query should be descriptive but not too long. Use natural language rather than keyword stuffing.",
		"type":  "Internet search is default and most versatile. Use 'news' for current events, 'image' for visual content, 'video' for tutorials.",
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

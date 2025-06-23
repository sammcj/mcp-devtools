package shadcnui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/PuerkitoBio/goquery"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

// UnifiedShadcnTool provides a single interface for all shadcn/ui operations
type UnifiedShadcnTool struct {
	client HTTPClient
}

func init() {
	registry.Register(&UnifiedShadcnTool{
		client: DefaultHTTPClient,
	})
}

// Definition returns the tool's definition for MCP registration
func (t *UnifiedShadcnTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"shadcn",
		mcp.WithDescription(`shadcn/ui components. Supports listing, searching, getting details, and examples for shadcn/ui components.

Actions:
- list: Get all available shadcn/ui components
- search: Search components by keyword in name or description
- details: Get detailed information about a specific component
- examples: Get usage examples for a specific component

Examples:
- List all components: {"action": "list"}
- Search for button components: {"action": "search", "query": "button"}
- Get button details: {"action": "details", "componentName": "button"}
- Get button examples: {"action": "examples", "componentName": "button"}`),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform: 'list', 'search', 'details', or 'examples'"),
			mcp.Enum("list", "search", "details", "examples"),
		),
		mcp.WithString("query",
			mcp.Description("Search query (required for 'search' action)"),
		),
		mcp.WithString("componentName",
			mcp.Description("Component name (required for 'details' and 'examples' actions)"),
		),
	)
}

// Execute executes the unified shadcn tool
func (t *UnifiedShadcnTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse action (required)
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: action")
	}

	logger.WithField("action", action).Info("Executing unified shadcn tool")

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
	case "examples":
		componentName, ok := args["componentName"].(string)
		if !ok || componentName == "" {
			return nil, fmt.Errorf("componentName parameter is required for examples action")
		}
		return t.executeExamples(ctx, logger, cache, componentName)
	default:
		return nil, fmt.Errorf("invalid action: %s. Must be one of: list, search, details, examples", action)
	}
}

// executeList handles the list action
func (t *UnifiedShadcnTool) executeList(ctx context.Context, logger *logrus.Logger, cache *sync.Map) (*mcp.CallToolResult, error) {
	logger.Info("Listing shadcn/ui components")

	// Check cache
	if cachedData, ok := cache.Load(listComponentsCacheKey); ok {
		if entry, ok := cachedData.(CacheEntry); ok && time.Since(entry.Timestamp) < listComponentsCacheTTL {
			logger.Debug("Returning cached list of shadcn/ui components")
			return packageversions.NewToolResultJSON(entry.Data)
		}
	}

	components, err := t.fetchComponentsList(logger, cache)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch components list: %w", err)
	}

	logger.WithField("count", len(components)).Info("Successfully fetched and parsed shadcn/ui components list")
	return packageversions.NewToolResultJSON(components)
}

// executeSearch handles the search action
func (t *UnifiedShadcnTool) executeSearch(ctx context.Context, logger *logrus.Logger, cache *sync.Map, query string) (*mcp.CallToolResult, error) {
	logger.Infof("Searching shadcn/ui components with query: %s", query)

	// Get component list (from cache or fetch)
	var allComponents []ComponentInfo
	if cachedData, found := cache.Load(listComponentsCacheKey); found {
		if entry, valid := cachedData.(CacheEntry); valid && time.Since(entry.Timestamp) < listComponentsCacheTTL {
			logger.Debug("Using cached list of shadcn/ui components for search")
			allComponents = entry.Data.([]ComponentInfo)
		}
	}

	if allComponents == nil {
		logger.Info("Component list not in cache or expired, fetching for search...")
		fetchedComponents, err := t.fetchComponentsList(logger, cache)
		if err != nil {
			return nil, fmt.Errorf("failed to get component list for search: %w", err)
		}
		allComponents = fetchedComponents
	}

	// Perform search
	var searchResults []ComponentInfo
	lowerQuery := strings.ToLower(query)

	for _, component := range allComponents {
		if strings.Contains(strings.ToLower(component.Name), lowerQuery) {
			searchResults = append(searchResults, component)
		}
	}

	logger.Infof("Found %d components matching query: %s", len(searchResults), query)
	return packageversions.NewToolResultJSON(searchResults)
}

// executeDetails handles the details action
func (t *UnifiedShadcnTool) executeDetails(ctx context.Context, logger *logrus.Logger, cache *sync.Map, componentName string) (*mcp.CallToolResult, error) {
	logger.Infof("Getting details for shadcn/ui component: %s", componentName)

	cacheKey := getComponentDetailsCachePrefix + componentName
	// Check cache
	if cachedData, ok := cache.Load(cacheKey); ok {
		if entry, ok := cachedData.(CacheEntry); ok && time.Since(entry.Timestamp) < getComponentDetailsCacheTTL {
			logger.Debugf("Returning cached details for component: %s", componentName)
			return packageversions.NewToolResultJSON(entry.Data)
		}
	}

	componentURL := fmt.Sprintf("%s/%s", ShadcnDocsComponents, componentName)
	resp, err := t.client.Get(componentURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch component page %s: %w", componentURL, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.WithError(err).Errorf("Failed to close response body for %s", componentURL)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch component page %s: status %d", componentURL, resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse component page %s: %w", componentURL, err)
	}

	info := ComponentInfo{
		Name: componentName,
		URL:  componentURL,
	}

	// Scrape Description
	info.Description = strings.TrimSpace(doc.Find("h1").First().NextFiltered("p").Text())

	// Scrape Installation command
	doc.Find("pre code").Each(func(i int, s *goquery.Selection) {
		codeText := s.Text()
		if strings.Contains(codeText, "npx shadcn-ui@latest add") {
			info.Installation = strings.TrimSpace(codeText)
		}
	})

	// Scrape Usage examples
	doc.Find("h2").Each(func(i int, h2 *goquery.Selection) {
		if strings.TrimSpace(h2.Text()) == "Usage" {
			h2.NextFilteredUntil("pre", "h2,h3").Find("code").Each(func(j int, code *goquery.Selection) {
				if info.Usage == "" {
					info.Usage = strings.TrimSpace(code.Text())
				}
			})
		}
	})

	// Initialise Props map
	info.Props = make(map[string]ComponentProp)

	// Construct Source URL
	info.SourceURL = fmt.Sprintf("%s/tree/main/apps/www/content/docs/components/%s.mdx", ShadcnGitHubURL, componentName)

	// Store in cache
	cache.Store(cacheKey, CacheEntry{
		Data:      info,
		Timestamp: time.Now(),
	})

	logger.Infof("Successfully fetched and parsed details for component: %s", componentName)
	return packageversions.NewToolResultJSON(info)
}

// executeExamples handles the examples action
func (t *UnifiedShadcnTool) executeExamples(ctx context.Context, logger *logrus.Logger, cache *sync.Map, componentName string) (*mcp.CallToolResult, error) {
	logger.Infof("Getting examples for shadcn/ui component: %s", componentName)

	cacheKey := getComponentExamplesCachePrefix + componentName
	// Check cache
	if cachedData, ok := cache.Load(cacheKey); ok {
		if entry, ok := cachedData.(CacheEntry); ok && time.Since(entry.Timestamp) < getComponentExamplesCacheTTL {
			logger.Debugf("Returning cached examples for component: %s", componentName)
			return packageversions.NewToolResultJSON(entry.Data)
		}
	}

	var examples []ComponentExample

	// 1. Scrape from component's doc page
	componentURL := fmt.Sprintf("%s/%s", ShadcnDocsComponents, componentName)
	resp, err := t.client.Get(componentURL)
	if err != nil {
		logger.Warnf("Failed to fetch component page %s for examples: %v", componentURL, err)
	} else {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				logger.WithError(err).Errorf("Failed to close response body for component page %s", componentURL)
			}
		}()
		if resp.StatusCode == http.StatusOK {
			doc, docErr := goquery.NewDocumentFromReader(resp.Body)
			if docErr != nil {
				logger.Warnf("Failed to parse component page %s for examples: %v", componentURL, docErr)
			} else {
				doc.Find("h2, h3").Each(func(i int, heading *goquery.Selection) {
					headingText := strings.TrimSpace(heading.Text())
					if strings.Contains(strings.ToLower(headingText), "example") || strings.Contains(strings.ToLower(headingText), "usage") {
						heading.NextUntil("h2, h3").Find("pre code").Each(func(j int, codeBlock *goquery.Selection) {
							example := ComponentExample{
								Title: headingText + fmt.Sprintf(" Example %d", j+1),
								Code:  strings.TrimSpace(codeBlock.Text()),
							}
							examples = append(examples, example)
						})
					}
				})
			}
		} else {
			logger.Warnf("Failed to fetch component page %s: status %d", componentURL, resp.StatusCode)
		}
	}

	// 2. Attempt to fetch the demo file from GitHub
	demoURL := fmt.Sprintf("%s/apps/www/registry/default/example/%s-demo.tsx", ShadcnRawGitHubURL, componentName)
	respDemo, errDemo := t.client.Get(demoURL)

	if errDemo != nil {
		logger.Warnf("Failed to fetch demo file %s: %v. Proceeding without it.", demoURL, errDemo)
	} else if respDemo != nil {
		defer func() {
			if err := respDemo.Body.Close(); err != nil {
				logger.WithError(err).Errorf("Failed to close response body for demo file %s", demoURL)
			}
		}()
		if respDemo.StatusCode == http.StatusOK {
			bodyBytes, readErr := io.ReadAll(respDemo.Body)
			if readErr != nil {
				logger.Warnf("Failed to read demo file %s: %v", demoURL, readErr)
			} else {
				titleCaser := cases.Title(language.AmericanEnglish, cases.NoLower)
				examples = append(examples, ComponentExample{
					Title:       fmt.Sprintf("%s Demo from GitHub", titleCaser.String(componentName)),
					Code:        string(bodyBytes),
					Description: "Example .tsx demo file from the official shadcn/ui GitHub repository.",
				})
			}
		} else {
			logger.Warnf("Failed to fetch demo file %s: status %d", demoURL, respDemo.StatusCode)
		}
	}

	if len(examples) == 0 {
		logger.Warnf("No examples found for component: %s", componentName)
	}

	// Store in cache
	cache.Store(cacheKey, CacheEntry{
		Data:      examples,
		Timestamp: time.Now(),
	})

	logger.Infof("Successfully processed examples for component: %s, found %d", componentName, len(examples))
	return packageversions.NewToolResultJSON(examples)
}

// fetchComponentsList fetches and caches the component list
func (t *UnifiedShadcnTool) fetchComponentsList(logger *logrus.Logger, cache *sync.Map) ([]ComponentInfo, error) {
	resp, err := t.client.Get(ShadcnDocsComponents)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch shadcn components page: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.WithError(closeErr).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch shadcn components page: status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse shadcn components page: %w", err)
	}

	var components []ComponentInfo
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && strings.HasPrefix(href, "/docs/components/") {
			componentName := strings.TrimPrefix(href, "/docs/components/")
			components = append(components, ComponentInfo{
				Name: componentName,
				URL:  ShadcnDocsURL + href,
			})
		}
	})

	// Remove duplicates
	components = removeDuplicateComponents(components)

	// Store in cache
	cache.Store(listComponentsCacheKey, CacheEntry{
		Data:      components,
		Timestamp: time.Now(),
	})

	return components, nil
}

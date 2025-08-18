package shadcnui

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions" // Added import
	"github.com/sirupsen/logrus"
)

// SearchShadcnComponentsTool defines the tool for searching shadcn ui components.
type SearchShadcnComponentsTool struct {
	// No direct dependency on ListShadcnComponentsTool, but uses its cache key and similar logic.
}

// Definition returns the tool's definition.
func (t *SearchShadcnComponentsTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"shadcn_search_components",
		mcp.WithDescription("Search for shadcn ui components by keyword in name or description."),
		mcp.WithString("query", mcp.Description("The keyword to search for."), mcp.Required()),
	)
}

// fetchAndCacheComponentList is a helper function similar to ListShadcnComponentsTool.Execute
// It fetches, parses, and caches the component list.
// This is duplicated logic but avoids direct tool-to-tool calls or complex dependencies.
func (t *SearchShadcnComponentsTool) fetchAndCacheComponentList(logger *logrus.Logger, cache *sync.Map) ([]ComponentInfo, error) {
	// Use security helper for consistent security handling
	ops := security.NewOperations("shadcnui")
	safeResp, err := ops.SafeHTTPGet(ShadcnDocsComponents)
	if err != nil {
		if secErr, ok := err.(*security.SecurityError); ok {
			return nil, fmt.Errorf("security block [ID: %s]: %s", secErr.GetSecurityID(), secErr.Error())
		}
		return nil, fmt.Errorf("failed to fetch shadcn components page: %w", err)
	}

	if safeResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch shadcn components page: status %d", safeResp.StatusCode)
	}

	// Handle security warnings
	if safeResp.SecurityResult != nil && safeResp.SecurityResult.Action == security.ActionWarn {
		logger.Warnf("Security warning [ID: %s]: %s", safeResp.SecurityResult.ID, safeResp.SecurityResult.Message)
	}

	bodyBytes := safeResp.Content

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes)))
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
	seen := make(map[string]bool)
	result := []ComponentInfo{}
	for _, component := range components {
		if _, ok := seen[component.Name]; !ok {
			seen[component.Name] = true
			result = append(result, component)
		}
	}
	components = result

	// Store in cache (using the shared key from utils.go)
	cache.Store(listComponentsCacheKey, CacheEntry{
		Data:      components,
		Timestamp: time.Now(),
	})
	logger.WithField("count", len(components)).Info("Fetched and cached shadcn ui components list for search.")
	return components, nil
}

// Execute performs the tool's action.
func (t *SearchShadcnComponentsTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query is a required argument and must be a non-empty string")
	}
	logger.Infof("Searching shadcn ui components with query: %s", query)

	var allComponents []ComponentInfo

	// Try to load component list from cache using shared constants from utils.go
	if cachedData, found := cache.Load(listComponentsCacheKey); found {
		if entry, valid := cachedData.(CacheEntry); valid && time.Since(entry.Timestamp) < listComponentsCacheTTL {
			logger.Debug("Using cached list of shadcn ui components for search")
			allComponents = entry.Data.([]ComponentInfo) // Type assertion
		}
	}

	// If not in cache or expired, fetch it
	if allComponents == nil {
		logger.Info("Component list not in cache or expired, fetching for search...")
		fetchedComponents, err := t.fetchAndCacheComponentList(logger, cache)
		if err != nil {
			return nil, fmt.Errorf("failed to get component list for search: %w", err)
		}
		allComponents = fetchedComponents
	}

	var searchResults []ComponentInfo
	lowerQuery := strings.ToLower(query)

	for _, component := range allComponents {
		if strings.Contains(strings.ToLower(component.Name), lowerQuery) {
			searchResults = append(searchResults, component)
			continue
		}
		// Future: search in description if descriptions are fetched with the list.
	}

	logger.Infof("Found %d components matching query: %s", len(searchResults), query)
	return packageversions.NewToolResultJSON(searchResults) // Use packageversions helper
}

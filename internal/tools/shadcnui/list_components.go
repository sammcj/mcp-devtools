package shadcnui

import (
	"context" // Re-add for marshalling
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

// ListShadcnComponentsTool defines the tool for listing shadcn ui components.
// listComponentsCacheKey and listComponentsCacheTTL are now in utils.go
type ListShadcnComponentsTool struct{}

// Definition returns the tool's definition.
func (t *ListShadcnComponentsTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"shadcn_list_components",
		mcp.WithDescription("Get a list of all available shadcn ui components."),
	// No input schema needed as it's an empty object.
	)
}

// Execute performs the tool's action.
func (t *ListShadcnComponentsTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Listing shadcn ui components")

	// Check cache
	if cachedData, ok := cache.Load(listComponentsCacheKey); ok {
		if entry, ok := cachedData.(CacheEntry); ok && time.Since(entry.Timestamp) < listComponentsCacheTTL {
			logger.Debug("Returning cached list of shadcn ui components")
			return packageversions.NewToolResultJSON(entry.Data) // Use packageversions helper
		}
	}

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
			// Further parsing might be needed for description if available on this page
			// For now, just name and URL
			components = append(components, ComponentInfo{
				Name: componentName,
				URL:  ShadcnDocsURL + href,
				// Description can be scraped from the individual component page later or if a summary is here
			})
		}
	})

	// Remove duplicates if any (though unlikely with this selector)
	components = removeDuplicateComponents(components)

	// Store in cache
	cache.Store(listComponentsCacheKey, CacheEntry{
		Data:      components,
		Timestamp: time.Now(),
	})

	logger.WithField("count", len(components)).Info("Successfully fetched and parsed shadcn ui components list")

	return packageversions.NewToolResultJSON(components) // Use packageversions helper
}

func removeDuplicateComponents(components []ComponentInfo) []ComponentInfo {
	seen := make(map[string]bool)
	result := []ComponentInfo{}
	for _, component := range components {
		if _, ok := seen[component.Name]; !ok {
			seen[component.Name] = true
			result = append(result, component)
		}
	}
	return result
}

package shadcnui

import (
	"context" // Re-add for marshalling
	"fmt"
	"io"
	"net/http"
	"net/url"
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
type ListShadcnComponentsTool struct {
	client HTTPClient
}

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

	// Read response body for security analysis with size limit
	limitedReader := io.LimitReader(resp.Body, 5*1024*1024) // 5MB limit for component lists
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Analyse content for security threats
	if parsedURL, err := url.Parse(ShadcnDocsComponents); err == nil {
		sourceContext := security.SourceContext{
			URL:         ShadcnDocsComponents,
			Domain:      parsedURL.Hostname(),
			ContentType: "html",
			Tool:        "shadcnui",
		}
		if secResult, err := security.AnalyseContent(string(bodyBytes), sourceContext); err == nil {
			switch secResult.Action {
			case security.ActionBlock:
				return nil, fmt.Errorf("content blocked by security policy [ID: %s]: %s", secResult.ID, secResult.Message)
			case security.ActionWarn:
				logger.WithField("security_id", secResult.ID).Warn(secResult.Message)
			}
		}
	}

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

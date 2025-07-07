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
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions" // Added import
	"github.com/sirupsen/logrus"
)

// GetComponentDetailsTool defines the tool for getting shadcn ui component details.
type GetComponentDetailsTool struct {
	client HTTPClient
}

// Definition returns the tool's definition.
func (t *GetComponentDetailsTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"shadcn_get_component_details",
		mcp.WithDescription("Get detailed information about a specific shadcn ui component."),
		mcp.WithString("componentName", mcp.Description("The name of the component (e.g., 'button', 'accordion')."), mcp.Required()),
	)
}

// Execute performs the tool's action.
func (t *GetComponentDetailsTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	componentName, ok := args["componentName"].(string)
	if !ok || componentName == "" {
		return nil, fmt.Errorf("componentName is a required argument and must be a non-empty string")
	}
	logger.Infof("Getting details for shadcn ui component: %s", componentName)

	cacheKey := getComponentDetailsCachePrefix + componentName
	// Check cache
	if cachedData, ok := cache.Load(cacheKey); ok {
		if entry, ok := cachedData.(CacheEntry); ok && time.Since(entry.Timestamp) < getComponentDetailsCacheTTL {
			logger.Debugf("Returning cached details for component: %s", componentName)
			return packageversions.NewToolResultJSON(entry.Data) // Use packageversions helper
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

	// Scrape Title (already have componentName, but good to confirm)
	// doc.Find("h1").Each(func(i int, s *goquery.Selection) {
	//  info.Name = strings.TrimSpace(s.Text()) // Usually the first h1
	// })

	// Scrape Description (typically the paragraph after the main H1 title)
	info.Description = strings.TrimSpace(doc.Find("h1").First().NextFiltered("p").Text())

	// Scrape Installation command
	doc.Find("pre code").Each(func(i int, s *goquery.Selection) {
		codeText := s.Text()
		if strings.Contains(codeText, "npx shadcn-ui@latest add") {
			info.Installation = strings.TrimSpace(codeText)
		}
	})

	// Scrape Usage examples (general usage often under an H2 "Usage")
	// This is a basic attempt; more specific selectors might be needed.
	doc.Find("h2").Each(func(i int, h2 *goquery.Selection) {
		if strings.TrimSpace(h2.Text()) == "Usage" {
			// Find the first <pre><code> block after this H2
			h2.NextFilteredUntil("pre", "h2,h3").Find("code").Each(func(j int, code *goquery.Selection) {
				if info.Usage == "" { // Take the first one for now
					info.Usage = strings.TrimSpace(code.Text())
				}
			})
		}
	})

	// Scrape Props/Variants (highly dependent on page structure)
	// This is a placeholder and needs refinement based on actual page structure.
	// Example: Look for tables within sections titled "Props", "API Reference", or "Examples"
	info.Props = make(map[string]ComponentProp) // Initialise Props map

	// Scrape Source URL (construct)
	info.SourceURL = fmt.Sprintf("%s/tree/main/apps/www/content/docs/components/%s.mdx", ShadcnGitHubURL, componentName) // Assuming .mdx files

	// Store in cache
	cache.Store(cacheKey, CacheEntry{
		Data:      info,
		Timestamp: time.Now(),
	})

	logger.Infof("Successfully fetched and parsed details for component: %s", componentName)
	return packageversions.NewToolResultJSON(info) // Use packageversions helper
}

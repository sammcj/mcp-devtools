package shadcnui

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/cases"    // For Title casing
	"golang.org/x/text/language" // For Title casing

	"github.com/PuerkitoBio/goquery"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions" // Added import
	"github.com/sirupsen/logrus"
)

// GetComponentExamplesTool defines the tool for getting shadcn ui component examples.
type GetComponentExamplesTool struct{}

// Definition returns the tool's definition.
func (t *GetComponentExamplesTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"shadcn_get_component_examples",
		mcp.WithDescription("Get usage examples for a specific shadcn ui component."),
		mcp.WithString("componentName", mcp.Description("The name of the component (e.g., 'button', 'accordion')."), mcp.Required()),
	)
}

// Execute performs the tool's action.
func (t *GetComponentExamplesTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	componentName, ok := args["componentName"].(string)
	if !ok || componentName == "" {
		return nil, fmt.Errorf("componentName is a required argument and must be a non-empty string")
	}
	logger.Infof("Getting examples for shadcn ui component: %s", componentName)

	cacheKey := getComponentExamplesCachePrefix + componentName
	// Check cache
	if cachedData, ok := cache.Load(cacheKey); ok {
		if entry, ok := cachedData.(CacheEntry); ok && time.Since(entry.Timestamp) < getComponentExamplesCacheTTL {
			logger.Debugf("Returning cached examples for component: %s", componentName)
			return packageversions.NewToolResultJSON(entry.Data) // Use packageversions helper
		}
	}

	var examples []ComponentExample

	// 1. Scrape from component's doc page
	componentURL := fmt.Sprintf("%s/%s", ShadcnDocsComponents, componentName)

	// Use security helper for consistent security handling
	ops := security.NewOperations("shadcnui")
	safeResp, err := ops.SafeHTTPGet(componentURL)
	if err != nil {
		logger.Warnf("Failed to fetch component page %s for examples: %v", componentURL, err)
		// Continue to try fetching from GitHub demo file
	} else {
		if safeResp.StatusCode == http.StatusOK {
			// Handle security warnings
			if safeResp.SecurityResult != nil && safeResp.SecurityResult.Action == security.ActionWarn {
				logger.Warnf("Security warning [ID: %s]: %s", safeResp.SecurityResult.ID, safeResp.SecurityResult.Message)
			}

			bodyBytes := safeResp.Content
			doc, docErr := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes)))
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
						heading.NextFiltered("pre").Find("code").Each(func(j int, codeBlock *goquery.Selection) {
							example := ComponentExample{
								Title: headingText + fmt.Sprintf(" Sibling Example %d", j+1),
								Code:  strings.TrimSpace(codeBlock.Text()),
							}
							examples = append(examples, example)
						})
					}
				})
			}
		} else {
			logger.Warnf("Failed to fetch component page %s: status %d", componentURL, safeResp.StatusCode)
		}
	}

	// 2. Attempt to fetch the demo file from GitHub
	demoURL := fmt.Sprintf("%s/apps/www/registry/default/example/%s-demo.tsx", ShadcnRawGitHubURL, componentName)
	safeDemoResp, errDemo := ops.SafeHTTPGet(demoURL)

	if errDemo != nil {
		logger.Warnf("Failed to fetch demo file %s: %v. Proceeding without it.", demoURL, errDemo)
	} else if safeDemoResp.StatusCode == http.StatusOK {
		// Handle security warnings for demo file
		if safeDemoResp.SecurityResult != nil && safeDemoResp.SecurityResult.Action == security.ActionWarn {
			logger.Warnf("Security warning for demo file [ID: %s]: %s", safeDemoResp.SecurityResult.ID, safeDemoResp.SecurityResult.Message)
		}

		titleCaser := cases.Title(language.AmericanEnglish, cases.NoLower)
		examples = append(examples, ComponentExample{
			Title:       fmt.Sprintf("%s Demo from GitHub", titleCaser.String(componentName)),
			Code:        string(safeDemoResp.Content),
			Description: "Example .tsx demo file from the official shadcn ui GitHub repository.",
		})
	} else if safeDemoResp != nil {
		logger.Warnf("Failed to fetch demo file %s: status %d", demoURL, safeDemoResp.StatusCode)
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
	return packageversions.NewToolResultJSON(examples) // Use packageversions helper
}

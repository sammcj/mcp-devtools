package duckduckgo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sirupsen/logrus"
)

// DuckDuckGoProvider implements the unified SearchProvider interface
type DuckDuckGoProvider struct {
	client internetsearch.HTTPClientInterface
}

// NewDuckDuckGoProvider creates a new DuckDuckGo search provider with rate limiting
// DuckDuckGo doesn't require an API key, so it's always available
func NewDuckDuckGoProvider() *DuckDuckGoProvider {
	return &DuckDuckGoProvider{
		client: internetsearch.NewRateLimitedHTTPClient(),
	}
}

// GetName returns the provider name
func (p *DuckDuckGoProvider) GetName() string {
	return "duckduckgo"
}

// IsAvailable checks if the provider is available
// DuckDuckGo is always available since it doesn't require an API key
func (p *DuckDuckGoProvider) IsAvailable() bool {
	return true
}

// GetSupportedTypes returns the search types this provider supports
func (p *DuckDuckGoProvider) GetSupportedTypes() []string {
	// DuckDuckGo HTML interface primarily supports web search
	// We'll map all types to web search for simplicity
	return []string{"web"}
}

// Search executes a search using the DuckDuckGo provider
func (p *DuckDuckGoProvider) Search(ctx context.Context, logger *logrus.Logger, searchType string, args map[string]any) (*internetsearch.SearchResponse, error) {
	query := args["query"].(string)

	logger.WithFields(logrus.Fields{
		"provider": "duckduckgo",
		"type":     searchType,
		"query":    query,
	}).Debug("DuckDuckGo search parameters")

	// For DuckDuckGo, all search types are handled as web search
	return p.executeWebSearch(ctx, logger, args)
}

// executeWebSearch handles web search execution
func (p *DuckDuckGoProvider) executeWebSearch(ctx context.Context, logger *logrus.Logger, args map[string]any) (*internetsearch.SearchResponse, error) {
	query := args["query"].(string)

	// Parse optional parameters
	count := 10
	if countRaw, ok := args["count"].(float64); ok {
		count = int(countRaw)
		if count < 1 || count > 50 {
			return nil, fmt.Errorf("count must be between 1 and 50 for DuckDuckGo search, got %d", count)
		}
	}

	// Create form data for POST request
	formData := url.Values{}
	formData.Set("q", query)
	formData.Set("b", "")
	formData.Set("kl", "")

	// Use security helper for safe HTTP POST
	ops := security.NewOperations("internetsearch")
	safeResp, err := ops.SafeHTTPPost("https://html.duckduckgo.com/html", strings.NewReader(formData.Encode()))
	if err != nil {
		if secErr, ok := err.(*security.SecurityError); ok {
			return nil, fmt.Errorf("security block [ID: %s]: %s Check with the user if you may use security_override tool with ID %s",
				secErr.GetSecurityID(), secErr.Error(), secErr.GetSecurityID())
		}
		return nil, fmt.Errorf("search request failed: %w", err)
	}

	// Handle security warnings
	if safeResp.SecurityResult != nil && safeResp.SecurityResult.Action == security.ActionWarn {
		logger.Warnf("Security warning [ID: %s]: %s", safeResp.SecurityResult.ID, safeResp.SecurityResult.Message)
	}

	if safeResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DuckDuckGo search error: %d", safeResp.StatusCode)
	}

	// Parse HTML response using goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(safeResp.Content)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML response: %w", err)
	}

	// Extract search results
	var results []internetsearch.SearchResult
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if len(results) >= count {
			return
		}

		// Extract title and link
		titleElem := s.Find(".result__title a").First()
		if titleElem.Length() == 0 {
			return
		}

		title := strings.TrimSpace(titleElem.Text())
		link, exists := titleElem.Attr("href")
		if !exists || title == "" {
			return
		}

		// Skip ad results
		if strings.Contains(link, "y.js") {
			return
		}

		// Clean up DuckDuckGo redirect URLs
		if strings.HasPrefix(link, "//duckduckgo.com/l/?uddg=") {
			parts := strings.Split(link, "uddg=")
			if len(parts) > 1 {
				urlPart := strings.Split(parts[1], "&")[0]
				if decodedURL, err := url.QueryUnescape(urlPart); err == nil {
					link = decodedURL
				}
			}
		}

		// Extract snippet
		snippet := ""
		snippetElem := s.Find(".result__snippet").First()
		if snippetElem.Length() > 0 {
			snippet = strings.TrimSpace(snippetElem.Text())
		}

		metadata := make(map[string]any)
		metadata["provider"] = "duckduckgo"
		metadata["position"] = len(results) + 1

		results = append(results, internetsearch.SearchResult{
			Title:       p.cleanText(title),
			URL:         link,
			Description: p.cleanText(snippet),
			Type:        "web",
			Metadata:    metadata,
		})
	})

	if len(results) == 0 {
		return p.createEmptyResponse(query)
	}

	return p.createSuccessResponse(query, results, logger)
}

// cleanText removes extra whitespace and cleans up text
func (p *DuckDuckGoProvider) cleanText(text string) string {
	// Replace multiple whitespace with single space
	re := regexp.MustCompile(`\s+`)
	cleaned := re.ReplaceAllString(text, " ")
	return strings.TrimSpace(cleaned)
}

// Helper functions
func (p *DuckDuckGoProvider) createEmptyResponse(query string) (*internetsearch.SearchResponse, error) {
	result := &internetsearch.SearchResponse{
		Query:       query,
		ResultCount: 0,
		Results:     []internetsearch.SearchResult{},
		Provider:    "duckduckgo",
		Timestamp:   time.Now(),
	}
	return result, nil
}

func (p *DuckDuckGoProvider) createSuccessResponse(query string, results []internetsearch.SearchResult, logger *logrus.Logger) (*internetsearch.SearchResponse, error) {
	result := &internetsearch.SearchResponse{
		Query:       query,
		ResultCount: len(results),
		Results:     results,
		Provider:    "duckduckgo",
		Timestamp:   time.Now(),
	}

	logger.WithFields(logrus.Fields{
		"query":        query,
		"result_count": len(results),
		"provider":     "duckduckgo",
	}).Info("DuckDuckGo search completed successfully")

	return result, nil
}

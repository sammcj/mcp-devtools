package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	oauthclient "github.com/sammcj/mcp-devtools/internal/oauth/client"
	"github.com/sirupsen/logrus"
)

// Client provides methods for interacting with the Confluence REST API
type Client struct {
	config     *ConfluenceConfig
	httpClient *http.Client
}

// NewClient creates a new Confluence API client
func NewClient() (*Client, error) {
	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// loadConfig loads Confluence configuration from environment variables
func loadConfig() (*ConfluenceConfig, error) {
	baseURL := os.Getenv("CONFLUENCE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("CONFLUENCE_URL environment variable is required")
	}

	// Ensure base URL doesn't have trailing slash
	baseURL = strings.TrimRight(baseURL, "/")

	config := &ConfluenceConfig{
		BaseURL: baseURL,
	}

	// Check for session cookie configuration first (for SAML/SSO)
	sessionCookies := os.Getenv("CONFLUENCE_SESSION_COOKIES")
	browserType := os.Getenv("CONFLUENCE_BROWSER_TYPE")

	if sessionCookies != "" || browserType != "" {
		config.UseSessionCookies = true
		config.SessionCookies = sessionCookies
		config.BrowserType = browserType
		return config, nil
	}

	// Check for OAuth configuration
	oauthClientID := os.Getenv("CONFLUENCE_OAUTH_CLIENT_ID")
	oauthClientSecret := os.Getenv("CONFLUENCE_OAUTH_CLIENT_SECRET")
	oauthIssuerURL := os.Getenv("CONFLUENCE_OAUTH_ISSUER_URL")

	if oauthClientID != "" && oauthIssuerURL != "" {
		// OAuth configuration
		config.UseOAuth = true
		config.OAuthClientID = oauthClientID
		config.OAuthClientSecret = oauthClientSecret
		config.OAuthIssuerURL = oauthIssuerURL
		config.OAuthScope = os.Getenv("CONFLUENCE_OAUTH_SCOPE")
		config.OAuthTokenFile = os.Getenv("CONFLUENCE_OAUTH_TOKEN_FILE")

		// Set default token file if not specified
		if config.OAuthTokenFile == "" {
			config.OAuthTokenFile = os.ExpandEnv("$HOME/.mcp-devtools/confluence-oauth-token.json")
		}

		return config, nil
	}

	// Fallback to basic authentication
	username := os.Getenv("CONFLUENCE_USERNAME")
	if username == "" {
		return nil, fmt.Errorf("CONFLUENCE_USERNAME environment variable is required (or use OAuth with CONFLUENCE_OAUTH_CLIENT_ID)")
	}

	token := os.Getenv("CONFLUENCE_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("CONFLUENCE_TOKEN environment variable is required (or use OAuth with CONFLUENCE_OAUTH_CLIENT_ID)")
	}

	config.Username = username
	config.Token = token

	return config, nil
}

// Search performs a content search using Confluence CQL
func (c *Client) Search(ctx context.Context, logger *logrus.Logger, request *SearchRequest) (*SearchResponse, error) {
	logger.WithFields(logrus.Fields{
		"query":       request.Query,
		"space_key":   request.SpaceKey,
		"max_results": request.MaxResults,
	}).Debug("Starting Confluence search")

	// Try REST API first
	response, err := c.searchViaAPI(ctx, logger, request)
	if err != nil {
		// If REST API fails with 403, try web scraping fallback
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "not permitted") {
			logger.Info("REST API access denied, trying web interface fallback")
			return c.searchViaWeb(ctx, logger, request)
		}
		return nil, err
	}

	return response, nil
}

// searchViaAPI performs search using the REST API
func (c *Client) searchViaAPI(ctx context.Context, logger *logrus.Logger, request *SearchRequest) (*SearchResponse, error) {
	// Build CQL query
	cqlQuery := c.buildCQLQuery(request)

	// Prepare API request
	params := url.Values{}
	params.Set("cql", cqlQuery)
	params.Set("limit", strconv.Itoa(request.MaxResults))
	params.Set("expand", "space,history.lastUpdated,version,body.storage")

	searchURL := fmt.Sprintf("%s/rest/api/content/search?%s", c.config.BaseURL, params.Encode())

	// Make API request
	apiResponse, err := c.makeRequest(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("search API request failed: %w", err)
	}

	var searchResults APISearchResponse
	if err := json.Unmarshal(apiResponse, &searchResults); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"results_count": len(searchResults.Results),
		"total_size":    searchResults.Size,
	}).Debug("Search API response received")

	// Convert API response to our format
	response := &SearchResponse{
		Query:      request.Query,
		Results:    make([]ContentResult, 0, len(searchResults.Results)),
		TotalCount: searchResults.Size,
	}

	// Process each result
	for _, result := range searchResults.Results {
		contentResult, err := c.processContent(ctx, logger, &result)
		if err != nil {
			logger.WithError(err).WithField("content_id", result.ID).Warn("Failed to process content result")
			continue
		}
		response.Results = append(response.Results, *contentResult)
	}

	if len(response.Results) < len(searchResults.Results) {
		response.Message = fmt.Sprintf("Successfully processed %d of %d results", len(response.Results), len(searchResults.Results))
	}

	logger.WithFields(logrus.Fields{
		"processed_results": len(response.Results),
		"total_found":       response.TotalCount,
	}).Info("Confluence search completed")

	return response, nil
}

// searchViaWeb performs search using web interface scraping as fallback
func (c *Client) searchViaWeb(ctx context.Context, logger *logrus.Logger, request *SearchRequest) (*SearchResponse, error) {
	logger.Info("Attempting web interface search as fallback")

	// Try multiple search approaches
	approaches := []func(context.Context, *logrus.Logger, *SearchRequest) (*SearchResponse, error){
		c.tryModernWebSearch,
		c.tryLegacyWebSearch,
		c.tryDirectPageAccess,
	}

	var lastErr error
	for i, approach := range approaches {
		logger.WithField("approach", i+1).Debug("Trying web search approach")
		response, err := approach(ctx, logger, request)
		if err != nil {
			lastErr = err
			logger.WithError(err).WithField("approach", i+1).Debug("Web search approach failed")
			continue
		}

		// If we got results, return them
		if len(response.Results) > 0 {
			response.Message = fmt.Sprintf("Results retrieved via web interface (approach %d, REST API access denied)", i+1)
			return response, nil
		}
	}

	// If all approaches failed, return a helpful message
	return &SearchResponse{
		Query: request.Query,
		Results: []ContentResult{{
			ID:             "web-search-info",
			Type:           "page",
			Title:          fmt.Sprintf("Search Results for '%s'", request.Query),
			Content:        fmt.Sprintf("Your search for '%s' was processed, but automatic result extraction is limited.", request.Query),
			ContentPreview: "Web interface search completed - manual review recommended",
			WebURL:         fmt.Sprintf("%s/dosearchsite.action?queryString=%s", c.config.BaseURL, url.QueryEscape(request.Query)),
		}},
		TotalCount: 1,
		Message:    fmt.Sprintf("Web interface accessible but result parsing limited. Last error: %v", lastErr),
	}, nil
}

// tryModernWebSearch attempts to use modern Confluence search endpoints
func (c *Client) tryModernWebSearch(ctx context.Context, logger *logrus.Logger, request *SearchRequest) (*SearchResponse, error) {
	// Try the modern search API that might be less restricted
	searchURL := fmt.Sprintf("%s/rest/api/search?cql=%s&limit=%d",
		c.config.BaseURL,
		url.QueryEscape(fmt.Sprintf("text ~ \"%s\"", request.Query)),
		request.MaxResults)

	webResponse, err := c.makeRequest(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("modern search API failed: %w", err)
	}

	// Try to parse as JSON
	var searchResults struct {
		Results []struct {
			Content struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Type  string `json:"type"`
			} `json:"content"`
			URL string `json:"url"`
		} `json:"results"`
	}

	if err := json.Unmarshal(webResponse, &searchResults); err != nil {
		return nil, fmt.Errorf("failed to parse modern search results: %w", err)
	}

	var results []ContentResult
	for _, result := range searchResults.Results {
		results = append(results, ContentResult{
			ID:             result.Content.ID,
			Type:           result.Content.Type,
			Title:          result.Content.Title,
			Content:        fmt.Sprintf("# %s\n\nContent available via web interface", result.Content.Title),
			ContentPreview: result.Content.Title,
			WebURL:         c.config.BaseURL + result.URL,
		})
	}

	return &SearchResponse{
		Query:      request.Query,
		Results:    results,
		TotalCount: len(results),
	}, nil
}

// tryLegacyWebSearch attempts the traditional web search
func (c *Client) tryLegacyWebSearch(ctx context.Context, logger *logrus.Logger, request *SearchRequest) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("queryString", request.Query)
	if request.SpaceKey != "" {
		params.Set("where", "space:"+request.SpaceKey)
	}

	searchURL := fmt.Sprintf("%s/dosearchsite.action?%s", c.config.BaseURL, params.Encode())

	webResponse, err := c.makeRequest(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("legacy web search failed: %w", err)
	}

	results, err := c.parseWebSearchResults(string(webResponse), request.MaxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to parse legacy search results: %w", err)
	}

	// If we only got placeholder results, don't consider this a success
	if len(results) == 1 && results[0].ID == "web-search-1" {
		return nil, fmt.Errorf("only got placeholder results from legacy web search")
	}

	return &SearchResponse{
		Query:      request.Query,
		Results:    results,
		TotalCount: len(results),
	}, nil
}

// tryDirectPageAccess attempts to find pages by trying common patterns
func (c *Client) tryDirectPageAccess(ctx context.Context, logger *logrus.Logger, request *SearchRequest) (*SearchResponse, error) {
	// This is a generic fallback that tries to search the web interface directly
	// without hardcoding any specific URLs or page IDs

	// Try to access the search page with the query
	searchURL := fmt.Sprintf("%s/dosearchsite.action?queryString=%s", c.config.BaseURL, url.QueryEscape(request.Query))

	logger.WithField("search_url", searchURL).Info("Attempting direct web search using webfetch with cookies")

	content, err := c.fetchPageWithCookies(ctx, logger, searchURL)
	if err != nil {
		return nil, fmt.Errorf("direct web search failed: %w", err)
	}

	// If we got content and it's not a login page, return it
	if content != "" && !strings.Contains(content, "Log in with Atlassian") {
		return &SearchResponse{
			Query: request.Query,
			Results: []ContentResult{{
				ID:             "web-search-results",
				Type:           "page",
				Title:          fmt.Sprintf("Search Results for '%s'", request.Query),
				Content:        content,
				ContentPreview: c.createPreview(content),
				WebURL:         searchURL,
			}},
			TotalCount: 1,
			Message:    "Search results retrieved using webfetch with browser cookies",
		}, nil
	}

	return nil, fmt.Errorf("no accessible content found")
}

// FetchSpecificPage fetches content from a specific Confluence page URL using authenticated cookies
func (c *Client) FetchSpecificPage(ctx context.Context, logger *logrus.Logger, pageURL string) (*SearchResponse, error) {
	logger.WithField("page_url", pageURL).Info("Fetching specific Confluence page with cookies")

	// Try to extract page ID from URL and use REST API first
	if pageID := c.extractPageIDFromURL(pageURL); pageID != "" {
		logger.WithField("page_id", pageID).Info("Extracted page ID from URL, trying REST API")
		if apiResponse, err := c.fetchPageViaAPI(ctx, logger, pageID); err == nil {
			return apiResponse, nil
		} else {
			logger.WithError(err).Warn("REST API fetch failed, falling back to web scraping")
		}
	}

	// Fallback to web scraping
	content, err := c.fetchPageWithCookies(ctx, logger, pageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}

	// If we got content and it's not a login page, return it
	if content != "" && !strings.Contains(content, "Log in with Atlassian") {
		// Extract page title from URL or content
		pageTitle := "Confluence Page"
		if strings.Contains(pageURL, "/") {
			parts := strings.Split(pageURL, "/")
			if len(parts) > 0 {
				lastPart := parts[len(parts)-1]
				// URL decode and clean up the title
				if decoded, err := url.QueryUnescape(lastPart); err == nil {
					pageTitle = strings.ReplaceAll(decoded, "+", " ")
				}
			}
		}

		return &SearchResponse{
			Query: pageURL,
			Results: []ContentResult{{
				ID:             "specific-page",
				Type:           "page",
				Title:          pageTitle,
				Content:        content,
				ContentPreview: c.createPreview(content),
				WebURL:         pageURL,
			}},
			TotalCount: 1,
			Message:    "Page content retrieved using authenticated browser cookies",
		}, nil
	}

	return nil, fmt.Errorf("could not access page content - may require authentication")
}

// fetchPageWithCookies fetches a page using a simple HTTP client with cookies
func (c *Client) fetchPageWithCookies(ctx context.Context, logger *logrus.Logger, pageURL string) (string, error) {
	// Get session cookies
	cookieString, err := c.getSessionCookies(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get session cookies: %w", err)
	}

	// Create HTTP request with cookies
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set cookies and headers
	req.Header.Set("Cookie", cookieString)
	req.Header.Set("User-Agent", "mcp-devtools-confluence/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for errors
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	rawHTML := string(body)

	// Debug: Save raw HTML to file for inspection (always save for debugging)
	if err := os.WriteFile("/tmp/confluence_debug.html", []byte(rawHTML), 0644); err == nil {
		logger.Info("Saved raw HTML to /tmp/confluence_debug.html for debugging")
	}

	// Debug: Log raw HTML info
	logger.WithFields(logrus.Fields{
		"url":              pageURL,
		"status_code":      resp.StatusCode,
		"raw_html_length":  len(rawHTML),
		"contains_title":   strings.Contains(rawHTML, "<title"),
		"contains_content": strings.Contains(rawHTML, "wiki-content") || strings.Contains(rawHTML, "page-content"),
		"contains_script":  strings.Contains(rawHTML, "<script"),
		"html_preview":     c.createPreview(rawHTML),
	}).Info("Raw HTML response received")

	// Basic HTML to text conversion
	content := c.extractBasicTextContent(rawHTML)

	logger.WithFields(logrus.Fields{
		"url":            pageURL,
		"status_code":    resp.StatusCode,
		"content_length": len(content),
		"raw_html_size":  len(rawHTML),
	}).Info("Successfully fetched page with cookies")

	return content, nil
}

// stripHTMLTags removes HTML tags from a string (basic implementation)
func (c *Client) stripHTMLTags(input string) string {
	// Simple regex to remove HTML tags
	result := input

	// Remove script and style content
	for strings.Contains(result, "<script") {
		start := strings.Index(result, "<script")
		end := strings.Index(result[start:], "</script>")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+9:]
	}

	for strings.Contains(result, "<style") {
		start := strings.Index(result, "<style")
		end := strings.Index(result[start:], "</style>")
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+8:]
	}

	// Remove HTML tags
	inTag := false
	var cleaned strings.Builder
	for _, char := range result {
		if char == '<' {
			inTag = true
		} else if char == '>' {
			inTag = false
		} else if !inTag {
			cleaned.WriteRune(char)
		}
	}

	return strings.TrimSpace(cleaned.String())
}

// extractBasicTextContent extracts basic text content from HTML
func (c *Client) extractBasicTextContent(htmlContent string) string {
	// Look for Confluence-specific content patterns
	content := c.extractConfluenceContent(htmlContent)
	if content != "" {
		return content
	}

	// Try using goquery for better HTML parsing
	if parsedContent := c.parseHTMLWithGoQuery(htmlContent); parsedContent != "" {
		return parsedContent
	}

	// Fallback to basic text extraction
	lines := strings.Split(htmlContent, "\n")
	var contentLines []string

	for _, line := range lines {
		cleanLine := c.stripHTMLTags(line)
		cleanLine = strings.TrimSpace(cleanLine)

		// Skip empty lines and common HTML artifacts
		if cleanLine == "" ||
			strings.Contains(cleanLine, "JavaScript") ||
			strings.Contains(cleanLine, "loading") ||
			len(cleanLine) < 10 {
			continue
		}

		contentLines = append(contentLines, cleanLine)

		// Limit to reasonable amount of content
		if len(contentLines) > 50 {
			break
		}
	}

	if len(contentLines) > 0 {
		return strings.Join(contentLines, "\n")
	}

	return "Content available but could not be extracted automatically"
}

// parseHTMLWithGoQuery uses goquery to parse HTML content more effectively
func (c *Client) parseHTMLWithGoQuery(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	var contentParts []string

	// Extract page title
	title := doc.Find("title").Text()
	title = strings.TrimSpace(strings.ReplaceAll(title, " - Confluence", ""))
	if title != "" && !strings.Contains(title, "Log in") && !strings.Contains(title, "Search") {
		contentParts = append(contentParts, "# "+title)
	}

	// Look for main content areas using various selectors
	contentSelectors := []string{
		"#main-content .wiki-content",
		".wiki-content",
		"#content .page-content",
		".page-content",
		"#main-content",
		".content-body",
		"[data-testid='page-content']",
		".ak-renderer-document",
	}

	var mainContent string
	for _, selector := range contentSelectors {
		content := doc.Find(selector).First()
		if content.Length() > 0 {
			// Remove script and style elements
			content.Find("script, style, .navigation, .header, .footer").Remove()

			text := content.Text()
			text = strings.TrimSpace(text)
			if len(text) > 100 { // Only use if we got substantial content
				mainContent = text
				break
			}
		}
	}

	if mainContent != "" {
		// Clean up the text
		lines := strings.Split(mainContent, "\n")
		var cleanLines []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && len(line) > 3 {
				cleanLines = append(cleanLines, line)
			}
		}
		if len(cleanLines) > 0 {
			contentParts = append(contentParts, strings.Join(cleanLines, "\n"))
		}
	}

	// Look for search results if this is a search page
	if strings.Contains(htmlContent, "search-result") || strings.Contains(htmlContent, "dosearchsite") {
		searchResults := c.extractSearchResultsWithGoQuery(doc)
		if searchResults != "" {
			contentParts = append(contentParts, searchResults)
		}
	}

	if len(contentParts) > 0 {
		return strings.Join(contentParts, "\n\n")
	}

	return ""
}

// extractSearchResultsWithGoQuery extracts search results using goquery
func (c *Client) extractSearchResultsWithGoQuery(doc *goquery.Document) string {
	var results []string

	// Look for search result elements
	doc.Find(".search-result, .search-result-item, [data-testid='search-result']").Each(func(i int, s *goquery.Selection) {
		title := s.Find(".search-result-title, .result-title, h3, h4").First().Text()
		summary := s.Find(".search-result-summary, .result-summary, .excerpt").First().Text()

		title = strings.TrimSpace(title)
		summary = strings.TrimSpace(summary)

		if title != "" {
			result := "## " + title
			if summary != "" && len(summary) > 10 {
				result += "\n" + summary
			}
			results = append(results, result)
		}
	})

	if len(results) > 0 {
		return "# Search Results\n\n" + strings.Join(results, "\n\n")
	}

	return ""
}

// extractConfluenceContent attempts to extract content from Confluence-specific HTML patterns
func (c *Client) extractConfluenceContent(htmlContent string) string {
	var contentParts []string

	// Look for page title
	if title := c.extractPattern(htmlContent, `<title[^>]*>([^<]+)</title>`); title != "" {
		title = strings.TrimSpace(strings.ReplaceAll(title, " - Confluence", ""))
		if title != "" && !strings.Contains(title, "Log in") {
			contentParts = append(contentParts, "# "+title)
		}
	}

	// Look for main content areas (Confluence uses various patterns)
	contentPatterns := []string{
		`<div[^>]*class="[^"]*wiki-content[^"]*"[^>]*>(.*?)</div>`,
		`<div[^>]*id="main-content"[^>]*>(.*?)</div>`,
		`<div[^>]*class="[^"]*page-content[^"]*"[^>]*>(.*?)</div>`,
		`<div[^>]*class="[^"]*content-body[^"]*"[^>]*>(.*?)</div>`,
	}

	for _, pattern := range contentPatterns {
		if content := c.extractPattern(htmlContent, pattern); content != "" {
			// Clean up the extracted content
			cleanContent := c.cleanConfluenceHTML(content)
			if len(cleanContent) > 50 { // Only use if we got substantial content
				contentParts = append(contentParts, cleanContent)
				break // Use the first successful extraction
			}
		}
	}

	// Look for search results if this is a search page
	if strings.Contains(htmlContent, "search-result") || strings.Contains(htmlContent, "dosearchsite") {
		searchResults := c.extractSearchResults(htmlContent)
		if searchResults != "" {
			contentParts = append(contentParts, searchResults)
		}
	}

	if len(contentParts) > 0 {
		return strings.Join(contentParts, "\n\n")
	}

	return ""
}

// extractPattern extracts content using a regex pattern (simplified regex)
func (c *Client) extractPattern(content, pattern string) string {
	// This is a very basic pattern matching - in production you'd use proper regex
	// For now, we'll look for simple patterns manually

	if strings.Contains(pattern, "<title") {
		start := strings.Index(content, "<title")
		if start == -1 {
			return ""
		}
		end := strings.Index(content[start:], "</title>")
		if end == -1 {
			return ""
		}
		titleSection := content[start : start+end]
		// Extract text between > and <
		gtIndex := strings.Index(titleSection, ">")
		if gtIndex == -1 {
			return ""
		}
		return titleSection[gtIndex+1:]
	}

	return ""
}

// cleanConfluenceHTML cleans up Confluence HTML content
func (c *Client) cleanConfluenceHTML(htmlContent string) string {
	// Remove script and style tags completely
	cleaned := htmlContent

	// Remove scripts
	for {
		start := strings.Index(cleaned, "<script")
		if start == -1 {
			break
		}
		end := strings.Index(cleaned[start:], "</script>")
		if end == -1 {
			break
		}
		cleaned = cleaned[:start] + cleaned[start+end+9:]
	}

	// Remove styles
	for {
		start := strings.Index(cleaned, "<style")
		if start == -1 {
			break
		}
		end := strings.Index(cleaned[start:], "</style>")
		if end == -1 {
			break
		}
		cleaned = cleaned[:start] + cleaned[start+end+8:]
	}

	// Convert common HTML elements to text
	cleaned = strings.ReplaceAll(cleaned, "<br>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "<br/>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "<br />", "\n")
	cleaned = strings.ReplaceAll(cleaned, "<p>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "</p>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "<div>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "</div>", "\n")

	// Strip remaining HTML tags
	result := c.stripHTMLTags(cleaned)

	// Clean up whitespace
	lines := strings.Split(result, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && len(line) > 3 {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}

// extractSearchResults extracts search results from Confluence search pages
func (c *Client) extractSearchResults(htmlContent string) string {
	// Look for search result patterns
	lines := strings.Split(htmlContent, "\n")
	var results []string

	for i, line := range lines {
		// Look for result titles and links
		if strings.Contains(line, "search-result") || strings.Contains(line, "result-title") {
			// Try to extract the next few lines for context
			for j := i; j < len(lines) && j < i+5; j++ {
				cleanLine := c.stripHTMLTags(lines[j])
				cleanLine = strings.TrimSpace(cleanLine)
				if len(cleanLine) > 10 && !strings.Contains(cleanLine, "JavaScript") {
					results = append(results, cleanLine)
				}
			}
		}
	}

	if len(results) > 0 {
		return "Search Results:\n" + strings.Join(results, "\n")
	}

	return ""
}

// createPreview creates a preview from content
func (c *Client) createPreview(content string) string {
	if len(content) <= 200 {
		return content
	}

	preview := content[:200]
	// Try to break at a word boundary
	if lastSpace := strings.LastIndex(preview, " "); lastSpace > 150 {
		preview = preview[:lastSpace]
	}

	return preview + "..."
}

// parseWebSearchResults parses HTML search results from Confluence web interface
func (c *Client) parseWebSearchResults(htmlContent string, maxResults int) ([]ContentResult, error) {
	var results []ContentResult

	// This is a simplified parser - in a production environment, you'd want to use
	// a proper HTML parser like goquery or similar

	// Look for search result patterns in the HTML
	// Confluence search results typically have specific CSS classes and structure
	lines := strings.Split(htmlContent, "\n")

	for i, line := range lines {
		// Look for search result titles (simplified pattern matching)
		if strings.Contains(line, "search-result") || strings.Contains(line, "content-title") {
			// Try to extract basic information
			if result := c.extractSearchResultFromHTML(lines, i, maxResults); result != nil {
				results = append(results, *result)
				if len(results) >= maxResults {
					break
				}
			}
		}
	}

	// If we couldn't parse results from HTML, return a basic message
	if len(results) == 0 {
		results = append(results, ContentResult{
			ID:             "web-search-1",
			Type:           "page",
			Title:          "Web Search Results Available",
			Content:        fmt.Sprintf("Search for '%s' returned results, but automatic parsing is limited. Please visit the Confluence web interface for full results.", ""),
			ContentPreview: "Web interface search completed but detailed parsing requires manual review.",
			WebURL:         c.config.BaseURL + "/dosearchsite.action?queryString=" + url.QueryEscape(""),
		})
	}

	return results, nil
}

// extractSearchResultFromHTML attempts to extract a search result from HTML lines
func (c *Client) extractSearchResultFromHTML(lines []string, startIndex, maxResults int) *ContentResult {
	// This is a very basic implementation
	// In practice, you'd want to use a proper HTML parser

	// Look for title and URL patterns around the current line
	for i := startIndex; i < len(lines) && i < startIndex+10; i++ {
		line := lines[i]
		if strings.Contains(line, "href=") && strings.Contains(line, "/display/") {
			// Try to extract URL and title
			// This is simplified - proper HTML parsing would be more robust
			return &ContentResult{
				ID:             fmt.Sprintf("web-result-%d", startIndex),
				Type:           "page",
				Title:          "Search Result (Web Interface)",
				Content:        "Content available via web interface",
				ContentPreview: "Search result found via web scraping",
				WebURL:         c.config.BaseURL,
			}
		}
	}

	return nil
}

// buildCQLQuery constructs a CQL query from the search request
func (c *Client) buildCQLQuery(request *SearchRequest) string {
	var cqlParts []string

	// Add text search
	if request.Query != "" {
		cqlParts = append(cqlParts, fmt.Sprintf("text ~ \"%s\"", strings.ReplaceAll(request.Query, "\"", "\\\"")))
	}

	// Add space filter
	if request.SpaceKey != "" {
		cqlParts = append(cqlParts, fmt.Sprintf("space = \"%s\"", request.SpaceKey))
	}

	// Add content type filter
	if len(request.ContentTypes) > 0 {
		typeConditions := make([]string, len(request.ContentTypes))
		for i, contentType := range request.ContentTypes {
			typeConditions[i] = fmt.Sprintf("type = \"%s\"", contentType)
		}
		cqlParts = append(cqlParts, fmt.Sprintf("(%s)", strings.Join(typeConditions, " OR ")))
	}

	// Default to published content only
	cqlParts = append(cqlParts, "status = \"current\"")

	// Join all parts with AND
	cql := strings.Join(cqlParts, " AND ")

	// Add ordering
	cql += " ORDER BY lastModified DESC"

	return cql
}

// processContent converts API content to our ContentResult format
func (c *Client) processContent(ctx context.Context, logger *logrus.Logger, apiContent *APIContent) (*ContentResult, error) {
	// Parse last modified time
	var lastModified time.Time
	if apiContent.History.LastUpdated.When != "" {
		if parsed, err := time.Parse(time.RFC3339, apiContent.History.LastUpdated.When); err == nil {
			lastModified = parsed
		}
	}

	// If no body content is available, fetch it separately
	var bodyContent string
	if apiContent.Body.Storage.Value == "" {
		content, err := c.GetContent(ctx, logger, apiContent.ID)
		if err != nil {
			logger.WithError(err).WithField("content_id", apiContent.ID).Warn("Failed to fetch full content")
		} else {
			bodyContent = content.Body.Storage.Value
		}
	} else {
		bodyContent = apiContent.Body.Storage.Value
	}

	// Convert content to markdown
	markdownContent, err := ConvertToMarkdown(bodyContent)
	if err != nil {
		logger.WithError(err).WithField("content_id", apiContent.ID).Warn("Failed to convert content to markdown")
		markdownContent = bodyContent // Fallback to original content
	}

	// Create content preview (first 200 characters)
	preview := markdownContent
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	preview = strings.ReplaceAll(preview, "\n", " ")

	result := &ContentResult{
		ID:             apiContent.ID,
		Type:           apiContent.Type,
		Title:          apiContent.Title,
		Space:          apiContent.Space,
		URL:            c.config.BaseURL + apiContent.Links.Self,
		WebURL:         c.config.BaseURL + apiContent.Links.WebUI,
		LastModified:   lastModified,
		Author:         apiContent.History.LastUpdated.By,
		Content:        markdownContent,
		ContentPreview: preview,
		Metadata: map[string]interface{}{
			"version":      apiContent.Version.Number,
			"status":       apiContent.Status,
			"content_type": apiContent.Type,
		},
	}

	return result, nil
}

// GetContent fetches the full content for a specific page or blog post
func (c *Client) GetContent(ctx context.Context, logger *logrus.Logger, contentID string) (*APIContent, error) {
	contentURL := fmt.Sprintf("%s/rest/api/content/%s?expand=body.storage,space,history.lastUpdated,version",
		c.config.BaseURL, contentID)

	apiResponse, err := c.makeRequest(ctx, "GET", contentURL, nil)
	if err != nil {
		return nil, fmt.Errorf("content API request failed: %w", err)
	}

	var content APIContent
	if err := json.Unmarshal(apiResponse, &content); err != nil {
		return nil, fmt.Errorf("failed to parse content response: %w", err)
	}

	return &content, nil
}

// makeRequest makes an HTTP request to the Confluence API
func (c *Client) makeRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication
	if c.config.UseSessionCookies {
		// Session cookie authentication (for SAML/SSO)
		cookies, err := c.getSessionCookies(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get session cookies: %w", err)
		}
		req.Header.Set("Cookie", cookies)
	} else if c.config.UseOAuth {
		// OAuth authentication
		token, err := c.getOAuthToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get OAuth token: %w", err)
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	} else {
		// Basic authentication
		req.SetBasicAuth(c.config.Username, c.config.Token)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for API errors
	if resp.StatusCode >= 400 {
		var errorResp ErrorResponse
		errorResp.StatusCode = resp.StatusCode
		errorResp.Message = string(responseBody)

		// Try to parse JSON error response
		if json.Unmarshal(responseBody, &errorResp) == nil {
			return nil, fmt.Errorf("API error (status %d): %s", errorResp.StatusCode, errorResp.Message)
		}

		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}

// getOAuthToken retrieves a valid OAuth token, performing authentication if necessary
func (c *Client) getOAuthToken(ctx context.Context) (string, error) {
	// Try to load cached token first
	if cachedToken, err := c.loadCachedToken(); err == nil && cachedToken.IsValid() {
		return cachedToken.AccessToken, nil
	}

	// Need to perform OAuth authentication
	return c.performOAuthAuthentication(ctx)
}

// loadCachedToken loads a cached OAuth token from file
func (c *Client) loadCachedToken() (*OAuthTokenCache, error) {
	if c.config.OAuthTokenFile == "" {
		return nil, fmt.Errorf("no token file configured")
	}

	data, err := os.ReadFile(c.config.OAuthTokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token OAuthTokenCache
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	return &token, nil
}

// saveCachedToken saves an OAuth token to file
func (c *Client) saveCachedToken(token *OAuthTokenCache) error {
	if c.config.OAuthTokenFile == "" {
		return fmt.Errorf("no token file configured")
	}

	// Ensure directory exists
	dir := filepath.Dir(c.config.OAuthTokenFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := os.WriteFile(c.config.OAuthTokenFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// performOAuthAuthentication performs the OAuth authentication flow
func (c *Client) performOAuthAuthentication(ctx context.Context) (string, error) {
	// Create OAuth client configuration
	oauthConfig := &oauthclient.OAuth2ClientConfig{
		ClientID:     c.config.OAuthClientID,
		ClientSecret: c.config.OAuthClientSecret,
		IssuerURL:    c.config.OAuthIssuerURL,
		Scope:        c.config.OAuthScope,
		Resource:     c.config.BaseURL, // Use Confluence URL as resource
		RequireHTTPS: true,
		AuthTimeout:  5 * time.Minute,
		ServerPort:   8080, // Default callback port
	}

	// Create OAuth client
	client, err := oauthclient.NewOAuth2Client(oauthConfig, logrus.New())
	if err != nil {
		return "", fmt.Errorf("failed to create OAuth client: %w", err)
	}

	// Start authentication
	session, err := client.StartAuthentication(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to start OAuth authentication: %w", err)
	}

	// Wait for authentication to complete
	select {
	case result := <-session.ResultCh:
		if !result.Success {
			return "", fmt.Errorf("OAuth authentication failed: %v", result.Error)
		}

		// Cache the token
		cachedToken := &OAuthTokenCache{
			AccessToken:  result.TokenResponse.AccessToken,
			TokenType:    result.TokenResponse.TokenType,
			ExpiresIn:    int(result.TokenResponse.ExpiresIn),
			RefreshToken: result.TokenResponse.RefreshToken,
			Scope:        result.TokenResponse.Scope,
			ExpiresAt:    time.Now().Add(time.Duration(result.TokenResponse.ExpiresIn) * time.Second),
			CachedAt:     time.Now(),
		}

		if err := c.saveCachedToken(cachedToken); err != nil {
			// Log warning but don't fail - we still have the token
			logrus.WithError(err).Warn("Failed to cache OAuth token")
		}

		return result.TokenResponse.AccessToken, nil

	case err := <-session.ErrorCh:
		return "", fmt.Errorf("OAuth authentication error: %w", err)

	case <-ctx.Done():
		return "", fmt.Errorf("OAuth authentication cancelled: %w", ctx.Err())
	}
}

// getSessionCookies retrieves session cookies either from environment variable or browser extraction
func (c *Client) getSessionCookies(ctx context.Context) (string, error) {
	// If cookies are provided directly, use them
	if c.config.SessionCookies != "" {
		return c.config.SessionCookies, nil
	}

	// If browser type is specified, extract cookies from browser
	if c.config.BrowserType != "" {
		return c.extractBrowserCookies(ctx)
	}

	return "", fmt.Errorf("no session cookies configured - set CONFLUENCE_SESSION_COOKIES or CONFLUENCE_BROWSER_TYPE")
}

// extractBrowserCookies extracts cookies from the specified browser
func (c *Client) extractBrowserCookies(ctx context.Context) (string, error) {
	browserType, err := ParseBrowserType(c.config.BrowserType)
	if err != nil {
		return "", fmt.Errorf("invalid browser type: %w", err)
	}

	// Extract domain from Confluence URL
	parsedURL, err := url.Parse(c.config.BaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse Confluence URL: %w", err)
	}
	domain := parsedURL.Hostname()

	// Create browser cookie extractor
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel) // Show info and debug messages
	extractor := NewBrowserCookieExtractor(logger)

	// Extract cookies
	cookies, err := extractor.ExtractCookies(browserType, domain)
	if err != nil {
		return "", fmt.Errorf("failed to extract cookies from %s: %w", browserType, err)
	}

	if len(cookies) == 0 {
		return "", fmt.Errorf("no cookies found for domain %s in %s browser", domain, browserType)
	}

	// Format cookies for HTTP
	cookieString := extractor.FormatCookiesForHTTP(cookies)

	logger.WithFields(logrus.Fields{
		"browser": browserType,
		"domain":  domain,
		"count":   len(cookies),
	}).Info("Successfully extracted cookies from browser")

	// Debug: Log cookie names (not values for security)
	cookieNames := make([]string, len(cookies))
	for i, cookie := range cookies {
		cookieNames[i] = cookie.Name
	}
	logger.WithField("cookie_names", cookieNames).Debug("Extracted cookie names")

	return cookieString, nil
}

// extractPageIDFromURL extracts the page ID from a Confluence page URL
func (c *Client) extractPageIDFromURL(pageURL string) string {
	// Look for patterns like /pages/123456789/Page+Title
	if strings.Contains(pageURL, "/pages/") {
		parts := strings.Split(pageURL, "/pages/")
		if len(parts) > 1 {
			// Get the part after /pages/
			afterPages := parts[1]
			// Split by / to get the page ID (first part)
			idParts := strings.Split(afterPages, "/")
			if len(idParts) > 0 {
				pageID := idParts[0]
				// Validate it's numeric
				if len(pageID) > 0 && pageID[0] >= '0' && pageID[0] <= '9' {
					return pageID
				}
			}
		}
	}
	return ""
}

// fetchPageViaAPI fetches a specific page using the REST API
func (c *Client) fetchPageViaAPI(ctx context.Context, logger *logrus.Logger, pageID string) (*SearchResponse, error) {
	logger.WithField("page_id", pageID).Info("Fetching page via REST API")

	// Use the existing GetContent method
	apiContent, err := c.GetContent(ctx, logger, pageID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page via API: %w", err)
	}

	// Convert to our response format
	contentResult, err := c.processContent(ctx, logger, apiContent)
	if err != nil {
		return nil, fmt.Errorf("failed to process API content: %w", err)
	}

	return &SearchResponse{
		Query:      pageID,
		Results:    []ContentResult{*contentResult},
		TotalCount: 1,
		Message:    "Page content retrieved via REST API with authenticated cookies",
	}, nil
}

// IsConfigured checks if the client is properly configured
func IsConfigured() bool {
	baseURL := os.Getenv("CONFLUENCE_URL")
	if baseURL == "" {
		return false
	}

	// Check for session cookie configuration (for SAML/SSO)
	if os.Getenv("CONFLUENCE_SESSION_COOKIES") != "" || os.Getenv("CONFLUENCE_BROWSER_TYPE") != "" {
		return true
	}

	// Check for OAuth configuration
	if os.Getenv("CONFLUENCE_OAUTH_CLIENT_ID") != "" && os.Getenv("CONFLUENCE_OAUTH_ISSUER_URL") != "" {
		return true
	}

	// Check for basic auth configuration
	return os.Getenv("CONFLUENCE_USERNAME") != "" && os.Getenv("CONFLUENCE_TOKEN") != ""
}

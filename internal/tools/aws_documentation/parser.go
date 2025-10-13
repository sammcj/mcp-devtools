package aws

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/PuerkitoBio/goquery"
)

// Parser handles HTML parsing and markdown conversion for AWS documentation
type Parser struct {
	converter *converter.Converter
}

// NewParser creates a new HTML parser for AWS documentation
func NewParser() *Parser {
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
		),
	)

	return &Parser{
		converter: conv,
	}
}

// contentSelectors defines the CSS selectors to find main content in AWS documentation
var contentSelectors = []string{
	"main",
	"article",
	"#main-content",
	".main-content",
	"#content",
	".content",
	"div[role='main']",
	"#awsdocs-content",
	".awsui-article",
}

// elementsToRemove defines selectors for elements that should be removed
var elementsToRemove = []string{
	"noscript",
	"script",
	"style",
	"nav",
	"aside",
	"header",
	"footer",
	".prev-next",
	"#main-col-footer",
	".awsdocs-page-utilities",
	"#quick-feedback-yes",
	"#quick-feedback-no",
	".page-loading-indicator",
	"#tools-panel",
	".doc-cookie-banner",
	".awsdocs-copyright",
	".awsdocs-thumb-feedback",
	".awsdocs-cookie-consent-container",
	".awsdocs-feedback-container",
	".awsdocs-page-header",
	".awsdocs-page-header-container",
	".awsdocs-filter-selector",
	".awsdocs-breadcrumb-container",
	".awsdocs-page-footer",
	".awsdocs-page-footer-container",
	".awsdocs-footer",
	".awsdocs-cookie-banner",
	"js-show-more-buttons",
	"js-show-more-text",
	"feedback-container",
	"feedback-section",
	"doc-feedback-container",
	"doc-feedback-section",
	"warning-container",
	"warning-section",
	"cookie-banner",
	"cookie-notice",
	"copyright-section",
	"legal-section",
	"terms-section",
}

// ConvertHTMLToMarkdown converts AWS documentation HTML to clean markdown
func (p *Parser) ConvertHTMLToMarkdown(htmlContent string) (string, error) {
	if htmlContent == "" {
		return "", fmt.Errorf("empty HTML content")
	}

	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract main content
	mainContent := p.extractMainContent(doc)

	// Clean up the content
	p.cleanContent(mainContent)

	// Get the HTML string from the cleaned content
	cleanedHTML, err := mainContent.Html()
	if err != nil {
		return "", fmt.Errorf("failed to get cleaned HTML: %w", err)
	}

	// Convert to markdown
	markdown, err := p.converter.ConvertString(cleanedHTML)
	if err != nil {
		return "", fmt.Errorf("failed to convert to markdown: %w", err)
	}

	// Post-process markdown
	processedMarkdown := p.postProcessMarkdown(markdown)

	return processedMarkdown, nil
}

// extractMainContent finds and extracts the main content area from AWS documentation
func (p *Parser) extractMainContent(doc *goquery.Document) *goquery.Selection {
	var mainContent *goquery.Selection

	// Try to find main content using content selectors
	for _, selector := range contentSelectors {
		content := doc.Find(selector).First()
		if content.Length() > 0 {
			mainContent = content
			break
		}
	}

	// If no main content found, use the body
	if mainContent == nil || mainContent.Length() == 0 {
		mainContent = doc.Find("body")
		if mainContent.Length() == 0 {
			// Fallback to the entire document
			mainContent = doc.Selection
		}
	}

	return mainContent
}

// cleanContent removes unwanted elements and cleans up the HTML
func (p *Parser) cleanContent(content *goquery.Selection) {
	// Remove unwanted elements
	for _, selector := range elementsToRemove {
		content.Find(selector).Remove()
	}

	// Remove empty paragraphs and divs
	content.Find("p, div").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			s.Remove()
		}
	})

	// Clean up AWS-specific elements
	p.cleanAWSSpecificElements(content)
}

// cleanAWSSpecificElements handles AWS-specific HTML patterns
func (p *Parser) cleanAWSSpecificElements(content *goquery.Selection) {
	// Remove elements with AWS-specific classes that contain no useful content
	awsCleanupSelectors := []string{
		"[class*='awsdocs-']",
		"[id*='awsdocs-']",
		".feedback",
		".rating",
		".thumbs",
		".copyright",
	}

	for _, selector := range awsCleanupSelectors {
		content.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			// Only remove if it's clearly navigation/feedback content
			if text == "" ||
				strings.Contains(strings.ToLower(text), "feedback") ||
				strings.Contains(strings.ToLower(text), "thumbs") ||
				strings.Contains(strings.ToLower(text), "helpful") ||
				strings.Contains(strings.ToLower(text), "copyright") ||
				len(text) < 10 {
				s.Remove()
			}
		})
	}
}

// postProcessMarkdown cleans up and improves the generated markdown
func (p *Parser) postProcessMarkdown(markdown string) string {
	// Remove excessive whitespace
	markdown = regexp.MustCompile(`\n{3,}`).ReplaceAllString(markdown, "\n\n")

	// Clean up malformed links
	markdown = regexp.MustCompile(`\[([^\]]*)\]\(\s*\)`).ReplaceAllString(markdown, "$1")

	// Fix code blocks that might be malformed
	markdown = regexp.MustCompile("```([^`]+)```").ReplaceAllStringFunc(markdown, func(match string) string {
		// Ensure code blocks have proper spacing
		return "\n" + match + "\n"
	})

	// Remove leading/trailing whitespace
	markdown = strings.TrimSpace(markdown)

	// Ensure we have some content
	if markdown == "" {
		return "Failed to extract meaningful content from the page."
	}

	return markdown
}

// FormatDocumentationResult formats the documentation content with pagination information
func (p *Parser) FormatDocumentationResult(url, content string, startIndex, maxLength int) DocumentationResponse {
	originalLength := len(content)

	if startIndex >= originalLength {
		return DocumentationResponse{
			URL:            url,
			Content:        "No more content available.",
			TotalLength:    originalLength,
			StartIndex:     startIndex,
			EndIndex:       startIndex,
			HasMoreContent: false,
			NextStartIndex: nil,
		}
	}

	// Calculate end index
	endIndex := min(startIndex+maxLength, originalLength)

	// Extract the content slice
	extractedContent := content[startIndex:endIndex]
	hasMoreContent := endIndex < originalLength

	response := DocumentationResponse{
		URL:            url,
		Content:        extractedContent,
		TotalLength:    originalLength,
		StartIndex:     startIndex,
		EndIndex:       endIndex,
		HasMoreContent: hasMoreContent,
	}

	// Set next start index if there's more content
	if hasMoreContent {
		nextStart := endIndex
		response.NextStartIndex = &nextStart
	}

	return response
}

// IsHTMLContent determines if the content is HTML based on content type and content
func IsHTMLContent(content, contentType string) bool {
	contentLower := strings.ToLower(content)
	contentTypeLower := strings.ToLower(contentType)

	// Check content type header
	if strings.Contains(contentTypeLower, "text/html") {
		return true
	}

	// Check if content starts with HTML
	if len(content) > 100 {
		return strings.Contains(contentLower[:100], "<html") ||
			strings.Contains(contentLower[:100], "<!doctype html")
	}

	// Fallback check for HTML tags
	return strings.Contains(contentLower, "<html") ||
		strings.Contains(contentLower, "<!doctype html") ||
		(strings.Contains(contentLower, "<body") && strings.Contains(contentLower, "</body>"))
}

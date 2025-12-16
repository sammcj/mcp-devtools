package webfetch

import (
	"fmt"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

// MarkdownConverter handles HTML to markdown conversion with custom rules
type MarkdownConverter struct {
	converter *converter.Converter
}

// NewMarkdownConverter creates a new converter with AI-friendly settings
func NewMarkdownConverter() *MarkdownConverter {
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
		),
	)

	// Remove unnecessary elements that don't add value for AI consumption
	// Note: script, style, noscript, and iframe are already removed by base plugin
	tagsToRemove := []string{
		"embed", "object", "nav", "header", "footer", "aside",
		"form", "button", "select", "canvas", "svg", "video", "audio",
	}
	for _, tag := range tagsToRemove {
		conv.Register.TagType(tag, converter.TagTypeRemove, converter.PriorityStandard)
	}

	return &MarkdownConverter{
		converter: conv,
	}
}

// ConvertToMarkdown converts HTML content to clean markdown
func (c *MarkdownConverter) ConvertToMarkdown(logger *logrus.Logger, htmlContent string) (string, error) {
	if htmlContent == "" {
		return "", nil
	}

	logger.Debug("Converting HTML to markdown")

	// Convert HTML to markdown
	markdown, err := c.converter.ConvertString(htmlContent)
	if err != nil {
		return "", fmt.Errorf("failed to convert HTML to markdown: %w", err)
	}

	// Clean up the markdown output
	cleaned := c.cleanMarkdown(markdown)

	logger.WithFields(logrus.Fields{
		"original_length": len(htmlContent),
		"markdown_length": len(cleaned),
	}).Debug("HTML to markdown conversion completed")

	return cleaned, nil
}

// cleanMarkdown performs post-processing cleanup on the markdown
func (c *MarkdownConverter) cleanMarkdown(markdown string) string {
	// Split into lines for processing
	lines := strings.Split(markdown, "\n")
	var cleanedLines []string

	var inCodeBlock bool
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track code blocks to preserve their content
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			cleanedLines = append(cleanedLines, line)
			continue
		}

		// Skip processing inside code blocks
		if inCodeBlock {
			cleanedLines = append(cleanedLines, line)
			continue
		}

		// Skip empty lines that create too much whitespace
		if trimmed == "" {
			// Only add empty line if the previous line wasn't also empty
			if len(cleanedLines) > 0 && strings.TrimSpace(cleanedLines[len(cleanedLines)-1]) != "" {
				cleanedLines = append(cleanedLines, "")
			}
			continue
		}

		// Clean up excessive whitespace within lines
		cleaned := strings.Join(strings.Fields(trimmed), " ")

		// Skip lines that are just markup artifacts
		if c.isMarkupArtifact(cleaned) {
			continue
		}

		cleanedLines = append(cleanedLines, cleaned)
	}

	// Join back and clean up excessive newlines
	result := strings.Join(cleanedLines, "\n")

	// Replace double newlines with single newlines to save tokens
	result = strings.ReplaceAll(result, "\n\n", "\n")

	// Trim leading and trailing whitespace
	result = strings.TrimSpace(result)

	return result
}

// isMarkupArtifact checks if a line is likely just markup artifacts
func (c *MarkdownConverter) isMarkupArtifact(line string) bool {
	// Skip lines that are just punctuation or symbols
	if len(line) < 3 {
		return true
	}

	// Skip lines that are likely navigation artifacts
	artifacts := []string{
		"skip to content",
		"skip to main",
		"menu",
		"search",
		"toggle navigation",
		"home",
		"about",
		"contact",
		"privacy policy",
		"terms of service",
		"cookie policy",
		"©", // Copyright symbols
		"all rights reserved",
	}

	lowerLine := strings.ToLower(line)
	for _, artifact := range artifacts {
		if strings.Contains(lowerLine, artifact) {
			return true
		}
	}

	// Skip lines that are mostly punctuation or special characters
	alphaCount := 0
	for _, r := range line {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			alphaCount++
		}
	}

	// If less than 30% alphanumeric, likely an artifact
	if float64(alphaCount)/float64(len(line)) < 0.3 {
		return true
	}

	return false
}

// FilterHTMLByFragment filters HTML content to only include the section identified by the fragment ID
// and its subsections. For heading elements, this includes all following content until the next heading
// of the same or higher level. For container elements (like section, div, article), this includes all
// child content. Returns the filtered HTML or the original content if the fragment is not found.
func FilterHTMLByFragment(logger *logrus.Logger, htmlContent string, fragment string) (string, error) {
	if fragment == "" {
		return htmlContent, nil
	}

	logger.WithField("fragment", fragment).Debug("Filtering HTML by fragment ID")

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Find the element with the matching ID
	targetElement := doc.Find("#" + fragment)
	if targetElement.Length() == 0 {
		logger.WithField("fragment", fragment).Warn("Fragment ID not found in HTML, returning full content")
		return htmlContent, nil
	}

	// Get the tag name to determine how to extract content
	tagName := goquery.NodeName(targetElement)

	// Check if this is a heading element (h1-h6)
	isHeading := tagName == "h1" || tagName == "h2" || tagName == "h3" || 
		tagName == "h4" || tagName == "h5" || tagName == "h6"

	var filteredHTML string

	if isHeading {
		// For headings, we need to include the heading and all following siblings
		// until we encounter another heading of the same or higher level
		headingLevel := int(tagName[1] - '0') // Extract level from h1, h2, etc.

		// Start with the heading itself
		outerHTML, err := goquery.OuterHtml(targetElement)
		if err != nil {
			logger.WithError(err).Warn("Failed to extract heading HTML, returning full content")
			return htmlContent, nil
		}

		var contentParts []string
		contentParts = append(contentParts, outerHTML)

		// Iterate through following siblings
		targetElement.NextAll().Each(func(i int, s *goquery.Selection) {
			siblingTag := goquery.NodeName(s)
			
			// Check if this is a heading of same or higher level
			if siblingTag == "h1" || siblingTag == "h2" || siblingTag == "h3" ||
				siblingTag == "h4" || siblingTag == "h5" || siblingTag == "h6" {
				siblingLevel := int(siblingTag[1] - '0')
				if siblingLevel <= headingLevel {
					// Stop here - we've reached the next section
					return
				}
			}

			// Include this sibling in the filtered content
			siblingHTML, _ := goquery.OuterHtml(s)
			contentParts = append(contentParts, siblingHTML)
		})

		filteredHTML = strings.Join(contentParts, "\n")
	} else {
		// For container elements, just get the HTML of the element and all its children
		filteredHTML, err = goquery.OuterHtml(targetElement)
		if err != nil {
			logger.WithError(err).Warn("Failed to extract filtered HTML, returning full content")
			return htmlContent, nil
		}
	}

	// Wrap the filtered content in a proper HTML structure to ensure it can be converted to markdown
	wrappedHTML := fmt.Sprintf("<!DOCTYPE html><html><body>%s</body></html>", filteredHTML)

	logger.WithFields(logrus.Fields{
		"fragment":      fragment,
		"original_size": len(htmlContent),
		"filtered_size": len(wrappedHTML),
	}).Debug("Successfully filtered HTML by fragment")

	return wrappedHTML, nil
}

// ProcessContent determines how to process content based on its type
func ProcessContent(logger *logrus.Logger, response *FetchURLResponse, raw bool, fragment string) (string, error) {
	if response.Content == "" {
		return "", nil
	}

	// If raw is requested or there was an HTTP error, return content as-is
	if raw || response.StatusCode >= 400 {
		return response.Content, nil
	}

	// Detect content type
	contentInfo := DetectContentType(response.ContentType, response.Content)

	logger.WithFields(logrus.Fields{
		"content_type": response.ContentType,
		"is_html":      contentInfo.IsHTML,
		"is_text":      contentInfo.IsText,
		"is_binary":    contentInfo.IsBinary,
		"fragment":     fragment,
	}).Debug("Processing content based on detected type")

	// Handle different content types
	if contentInfo.IsBinary {
		return fmt.Sprintf("Content type %s is binary and cannot be displayed as text. Content length: %d bytes.",
			response.ContentType, len(response.Content)), nil
	}

	if contentInfo.IsHTML {
		// Apply fragment filtering if specified
		contentToConvert := response.Content
		if fragment != "" {
			filteredContent, err := FilterHTMLByFragment(logger, response.Content, fragment)
			if err != nil {
				logger.WithError(err).Warn("Failed to filter HTML by fragment, using full content")
			} else {
				contentToConvert = filteredContent
			}
		}

		// Convert HTML to markdown
		converter := NewMarkdownConverter()
		markdown, err := converter.ConvertToMarkdown(logger, contentToConvert)
		if err != nil {
			logger.WithError(err).Warn("Failed to convert HTML to markdown, returning raw content")
			return response.Content, nil
		}
		return markdown, nil
	}

	// For plain text and other text-based content, return as-is
	return response.Content, nil
}

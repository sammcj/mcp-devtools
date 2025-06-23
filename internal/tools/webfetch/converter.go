package webfetch

import (
	"fmt"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/sirupsen/logrus"
)

// MarkdownConverter handles HTML to markdown conversion with custom rules
type MarkdownConverter struct {
	converter *md.Converter
}

// NewMarkdownConverter creates a new converter with AI-friendly settings
func NewMarkdownConverter() *MarkdownConverter {
	conv := md.NewConverter("", true, nil)

	// Remove unnecessary elements that don't add value for AI consumption
	conv = conv.Remove("script", "style", "noscript", "iframe", "embed", "object")
	conv = conv.Remove("nav", "header", "footer", "aside")              // Remove navigation elements
	conv = conv.Remove("form", "input", "button", "select", "textarea") // Remove form elements
	conv = conv.Remove("canvas", "svg", "video", "audio")               // Remove media elements

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

// ProcessContent determines how to process content based on its type
func ProcessContent(logger *logrus.Logger, response *FetchURLResponse, raw bool) (string, error) {
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
	}).Debug("Processing content based on detected type")

	// Handle different content types
	if contentInfo.IsBinary {
		return fmt.Sprintf("Content type %s is binary and cannot be displayed as text. Content length: %d bytes.",
			response.ContentType, len(response.Content)), nil
	}

	if contentInfo.IsHTML {
		// Convert HTML to markdown
		converter := NewMarkdownConverter()
		markdown, err := converter.ConvertToMarkdown(logger, response.Content)
		if err != nil {
			logger.WithError(err).Warn("Failed to convert HTML to markdown, returning raw content")
			return response.Content, nil
		}
		return markdown, nil
	}

	// For plain text and other text-based content, return as-is
	return response.Content, nil
}

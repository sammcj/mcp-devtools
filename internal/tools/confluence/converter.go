package confluence

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ConvertToMarkdown converts Confluence storage format (XHTML) to Markdown
func ConvertToMarkdown(xhtml string) (string, error) {
	if xhtml == "" {
		return "", nil
	}

	// Parse the XHTML content
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(xhtml))
	if err != nil {
		return "", err
	}

	// Start conversion process
	markdown := convertElement(doc.Selection)

	// Clean up the markdown
	markdown = cleanupMarkdown(markdown)

	return markdown, nil
}

// convertElement recursively converts HTML elements to Markdown
func convertElement(s *goquery.Selection) string {
	var result strings.Builder

	s.Contents().Each(func(i int, content *goquery.Selection) {
		if goquery.NodeName(content) == "#text" {
			// Handle text nodes
			text := content.Text()
			result.WriteString(text)
		} else {
			// Handle element nodes
			tagName := goquery.NodeName(content)
			converted := convertTag(content, tagName)
			result.WriteString(converted)
		}
	})

	return result.String()
}

// convertTag converts specific HTML tags to Markdown
func convertTag(s *goquery.Selection, tagName string) string {
	innerContent := convertElement(s)

	switch tagName {
	case "h1":
		return "\n# " + strings.TrimSpace(innerContent) + "\n\n"
	case "h2":
		return "\n## " + strings.TrimSpace(innerContent) + "\n\n"
	case "h3":
		return "\n### " + strings.TrimSpace(innerContent) + "\n\n"
	case "h4":
		return "\n#### " + strings.TrimSpace(innerContent) + "\n\n"
	case "h5":
		return "\n##### " + strings.TrimSpace(innerContent) + "\n\n"
	case "h6":
		return "\n###### " + strings.TrimSpace(innerContent) + "\n\n"

	case "p":
		content := strings.TrimSpace(innerContent)
		if content == "" {
			return "\n"
		}
		return "\n" + content + "\n\n"

	case "br":
		return "\n"

	case "strong", "b":
		return "**" + innerContent + "**"

	case "em", "i":
		return "*" + innerContent + "*"

	case "code":
		return "`" + innerContent + "`"

	case "pre":
		// Handle code blocks
		language := ""
		if class, exists := s.Attr("class"); exists {
			if strings.Contains(class, "language-") {
				parts := strings.Split(class, "language-")
				if len(parts) > 1 {
					language = strings.Split(parts[1], " ")[0]
				}
			}
		}
		return "\n```" + language + "\n" + innerContent + "\n```\n\n"

	case "blockquote":
		lines := strings.Split(strings.TrimSpace(innerContent), "\n")
		var quotedLines []string
		for _, line := range lines {
			quotedLines = append(quotedLines, "> "+line)
		}
		return "\n" + strings.Join(quotedLines, "\n") + "\n\n"

	case "ul":
		return "\n" + convertList(s, "*") + "\n"

	case "ol":
		return "\n" + convertList(s, "1.") + "\n"

	case "li":
		return "- " + strings.TrimSpace(innerContent) + "\n"

	case "a":
		href, exists := s.Attr("href")
		if exists && href != "" {
			title := strings.TrimSpace(innerContent)
			if title == "" {
				title = href
			}
			return "[" + title + "](" + href + ")"
		}
		return innerContent

	case "img":
		src, _ := s.Attr("src")
		alt, _ := s.Attr("alt")
		title, _ := s.Attr("title")

		if alt == "" {
			alt = "image"
		}

		if title != "" {
			return "![" + alt + "](" + src + " \"" + title + "\")"
		}
		return "![" + alt + "](" + src + ")"

	case "table":
		return convertTable(s)

	case "hr":
		return "\n---\n\n"

	case "div", "span":
		// Handle Confluence-specific div classes
		if class, exists := s.Attr("class"); exists {
			if strings.Contains(class, "code") {
				return "\n```\n" + innerContent + "\n```\n\n"
			}
			if strings.Contains(class, "panel") {
				return "\n> " + strings.ReplaceAll(strings.TrimSpace(innerContent), "\n", "\n> ") + "\n\n"
			}
		}
		return innerContent

	// Confluence-specific elements
	case "ac:structured-macro":
		return convertConfluenceMacro(s)

	case "ac:parameter":
		// Skip macro parameters in conversion
		return ""

	case "ac:rich-text-body", "ac:plain-text-body":
		return innerContent

	default:
		// For unknown tags, just return the inner content
		return innerContent
	}
}

// convertList handles ordered and unordered lists
func convertList(s *goquery.Selection, marker string) string {
	var result strings.Builder
	listItems := s.Find("li")

	listItems.Each(func(i int, li *goquery.Selection) {
		content := convertElement(li)
		content = strings.TrimSpace(content)

		if marker == "1." {
			result.WriteString(strings.Repeat("  ", getListDepth(li)) + "1. " + content + "\n")
		} else {
			result.WriteString(strings.Repeat("  ", getListDepth(li)) + "- " + content + "\n")
		}
	})

	return result.String()
}

// getListDepth calculates the nesting depth of a list item
func getListDepth(s *goquery.Selection) int {
	depth := 0
	s.ParentsFiltered("ul, ol").Each(func(i int, parent *goquery.Selection) {
		depth++
	})
	return depth
}

// convertTable converts HTML tables to Markdown tables
func convertTable(s *goquery.Selection) string {
	var result strings.Builder
	result.WriteString("\n")

	rows := s.Find("tr")
	headerProcessed := false

	rows.Each(func(i int, row *goquery.Selection) {
		cells := row.Find("th, td")
		if cells.Length() == 0 {
			return
		}

		result.WriteString("| ")
		cells.Each(func(j int, cell *goquery.Selection) {
			content := strings.TrimSpace(convertElement(cell))
			content = strings.ReplaceAll(content, "|", "\\|")
			content = strings.ReplaceAll(content, "\n", " ")
			result.WriteString(content + " | ")
		})
		result.WriteString("\n")

		// Add header separator after first row
		if !headerProcessed {
			result.WriteString("|")
			for k := 0; k < cells.Length(); k++ {
				result.WriteString(" --- |")
			}
			result.WriteString("\n")
			headerProcessed = true
		}
	})

	result.WriteString("\n")
	return result.String()
}

// convertConfluenceMacro handles Confluence-specific macros
func convertConfluenceMacro(s *goquery.Selection) string {
	macroName, exists := s.Attr("ac:name")
	if !exists {
		return convertElement(s)
	}

	switch macroName {
	case "code":
		language := ""
		s.Find("ac:parameter").Each(func(i int, param *goquery.Selection) {
			if name, exists := param.Attr("ac:name"); exists && name == "language" {
				language = param.Text()
			}
		})

		body := s.Find("ac:plain-text-body").Text()
		if body == "" {
			body = s.Find("ac:rich-text-body").Text()
		}

		return "\n```" + language + "\n" + body + "\n```\n\n"

	case "info", "note", "warning", "tip":
		title := ""
		s.Find("ac:parameter").Each(func(i int, param *goquery.Selection) {
			if name, exists := param.Attr("ac:name"); exists && name == "title" {
				title = param.Text()
			}
		})

		body := convertElement(s.Find("ac:rich-text-body"))

		prefix := "> "
		if title != "" {
			prefix = "> **" + title + "**: "
		}

		lines := strings.Split(strings.TrimSpace(body), "\n")
		var quotedLines []string
		for i, line := range lines {
			if i == 0 {
				quotedLines = append(quotedLines, prefix+line)
			} else {
				quotedLines = append(quotedLines, "> "+line)
			}
		}

		return "\n" + strings.Join(quotedLines, "\n") + "\n\n"

	case "quote":
		body := convertElement(s.Find("ac:rich-text-body"))
		lines := strings.Split(strings.TrimSpace(body), "\n")
		var quotedLines []string
		for _, line := range lines {
			quotedLines = append(quotedLines, "> "+line)
		}
		return "\n" + strings.Join(quotedLines, "\n") + "\n\n"

	case "toc":
		return "\n**Table of Contents**\n\n"

	default:
		// For unknown macros, try to extract the body content
		body := convertElement(s.Find("ac:rich-text-body"))
		if body == "" {
			body = convertElement(s.Find("ac:plain-text-body"))
		}
		if body == "" {
			body = convertElement(s)
		}
		return body
	}
}

// cleanupMarkdown performs final cleanup on the converted Markdown
func cleanupMarkdown(markdown string) string {
	// Remove excessive blank lines
	markdown = regexp.MustCompile(`\n{3,}`).ReplaceAllString(markdown, "\n\n")

	// Clean up whitespace around headers
	markdown = regexp.MustCompile(`\n+#`).ReplaceAllString(markdown, "\n\n#")

	// Remove trailing whitespace from lines
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	markdown = strings.Join(lines, "\n")

	// Trim leading and trailing whitespace
	markdown = strings.TrimSpace(markdown)

	// Ensure document ends with a single newline
	if markdown != "" && !strings.HasSuffix(markdown, "\n") {
		markdown += "\n"
	}

	return markdown
}

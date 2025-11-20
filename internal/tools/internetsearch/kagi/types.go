package kagi

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

// decodeHTMLEntities cleans HTML entities, returning clean plain text
func decodeHTMLEntities(s string) string {
	// First handle standard HTML entity decoding
	decoded := html.UnescapeString(s)

	// Strip any HTML tags
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	decoded = htmlTagRegex.ReplaceAllString(decoded, "")

	// Clean up and normalise the text
	return normaliseText(decoded)
}

// normaliseText removes problematic Unicode and normalises whitespace
func normaliseText(s string) string {
	// Remove or replace problematic Unicode characters
	var cleaned strings.Builder
	for _, r := range s {
		// Keep printable ASCII and common Unicode characters
		if unicode.IsPrint(r) && (r < 127 || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || unicode.IsPunct(r)) {
			cleaned.WriteRune(r)
		} else if unicode.IsSpace(r) {
			cleaned.WriteRune(' ')
		}
	}

	result := cleaned.String()

	// Normalise whitespace
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	result = strings.TrimSpace(result)

	return result
}

// KagiSearchResponse represents the response from Kagi Search API
type KagiSearchResponse struct {
	Meta  KagiMeta     `json:"meta"`
	Data  []KagiResult `json:"data"`
	Error []KagiError  `json:"error,omitempty"`
}

// KagiMeta contains metadata about the search request
type KagiMeta struct {
	ID   string  `json:"id"`
	Node string  `json:"node"`
	MS   float64 `json:"ms"`
}

// KagiResult represents a single search result from Kagi
type KagiResult struct {
	T         int            `json:"t"`                   // Result type (0=search result, 1=related search)
	Rank      int            `json:"rank"`                // Result ranking position
	URL       string         `json:"url"`                 // Result URL
	Title     string         `json:"title"`               // Result title
	Snippet   string         `json:"snippet"`             // Result snippet/description
	Published string         `json:"published,omitempty"` // Publication date if available
	Thumbnail *KagiThumbnail `json:"thumbnail,omitempty"` // Thumbnail if available
	List      []string       `json:"list,omitempty"`      // Related searches
}

// KagiThumbnail contains thumbnail information for a result
type KagiThumbnail struct {
	URL    string `json:"url"`
	Height int    `json:"height,omitempty"`
	Width  int    `json:"width,omitempty"`
}

// KagiError represents an error response from the API
type KagiError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Ref  string `json:"ref,omitempty"`
}

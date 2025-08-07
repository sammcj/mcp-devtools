package brave

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

// decodeHTMLEntities cleans HTML entities and malformed tags, returning clean plain text
func decodeHTMLEntities(s string) string {
	// First handle standard HTML entity decoding
	decoded := html.UnescapeString(s)

	// Remove malformed HTML patterns completely - just strip them out
	replacements := map[string]string{
		// Handle escaped patterns that Brave API returns by removing them
		`\strong\`: ``, `\/strong\`: ``,
		`\em\`: ``, `\/em\`: ``,
		`\b\`: ``, `\/b\`: ``,
		`\i\`: ``, `\/i\`: ``,
		`\u\`: ``, `\/u\`: ``,
		// Handle double-escaped patterns
		`\\strong\\`: ``, `\\/strong\\`: ``,
		`\\em\\`: ``, `\\/em\\`: ``,
		`\\b\\`: ``, `\\/b\\`: ``,
		`\\i\\`: ``, `\\/i\\`: ``,
		`\\u\\`: ``, `\\/u\\`: ``,
	}

	// Apply string replacements to remove escaped patterns
	for malformed, replacement := range replacements {
		decoded = strings.ReplaceAll(decoded, malformed, replacement)
	}

	// Handle non-escaped malformed tags using regex (e.g., strongText/strong -> Text)
	for _, tag := range []string{"strong", "em", "b", "i", "u"} {
		pattern := regexp.MustCompile(`\b` + tag + `([^/]*?)/` + tag + `\b`)
		decoded = pattern.ReplaceAllString(decoded, `$1`)
	}

	// Strip any remaining HTML tags
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	decoded = htmlTagRegex.ReplaceAllString(decoded, "")

	// Clean up and Normalise the text
	return NormaliseText(decoded)
}

// NormaliseText removes problematic Unicode and Normalises whitespace
func NormaliseText(s string) string {
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

// BraveWebSearchResponse represents the response from Brave web search API
type BraveWebSearchResponse struct {
	Type  string                `json:"type"`
	Query BraveQuery            `json:"query"`
	Web   *BraveWebSearchResult `json:"web,omitempty"`
}

// BraveWebSearchResult contains web search results
type BraveWebSearchResult struct {
	Type    string           `json:"type"`
	Results []BraveWebResult `json:"results"`
}

// BraveWebResult represents a single web search result
type BraveWebResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Age         string `json:"age,omitempty"`
}

// BraveImageSearchResponse represents the response from Brave image search API
type BraveImageSearchResponse struct {
	Type    string             `json:"type"`
	Query   BraveQuery         `json:"query"`
	Results []BraveImageResult `json:"results"`
}

// BraveImageResult represents a single image search result
type BraveImageResult struct {
	Type       string               `json:"type"`
	Title      string               `json:"title"`
	URL        string               `json:"url"`
	Properties BraveImageProperties `json:"properties"`
}

// BraveImageProperties contains image metadata
type BraveImageProperties struct {
	URL    string `json:"url"`
	Format string `json:"format,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// BraveNewsSearchResponse represents the response from Brave news search API
type BraveNewsSearchResponse struct {
	Type    string            `json:"type"`
	Query   BraveQuery        `json:"query"`
	Results []BraveNewsResult `json:"results"`
}

// BraveNewsResult represents a single news search result
type BraveNewsResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Age         string `json:"age"`
}

// BraveVideoSearchResponse represents the response from Brave video search API
type BraveVideoSearchResponse struct {
	Type    string             `json:"type"`
	Query   BraveQuery         `json:"query"`
	Results []BraveVideoResult `json:"results"`
}

// BraveVideoResult represents a single video search result
type BraveVideoResult struct {
	Type  string         `json:"type"`
	Title string         `json:"title"`
	URL   string         `json:"url"`
	Video BraveVideoData `json:"video"`
}

// BraveVideoData contains video metadata
type BraveVideoData struct {
	Duration string      `json:"duration,omitempty"`
	Views    interface{} `json:"views,omitempty"` // Can be string or number
	Creator  string      `json:"creator,omitempty"`
}

// BraveLocalSearchResponse represents the response from Brave local search API
type BraveLocalSearchResponse struct {
	Type      string                `json:"type"`
	Query     BraveQuery            `json:"query"`
	Locations *BraveLocationResult  `json:"locations,omitempty"`
	Web       *BraveWebSearchResult `json:"web,omitempty"`
}

// BraveLocationResult contains local search results
type BraveLocationResult struct {
	Type    string              `json:"type"`
	Results []BraveLocationItem `json:"results"`
}

// BraveLocationItem represents a single location result
type BraveLocationItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Description string    `json:"description"`
	Coordinates []float64 `json:"coordinates,omitempty"`
}

// BraveLocalPOIResponse represents the response from local POI API
type BraveLocalPOIResponse struct {
	Type    string         `json:"type"`
	Results []BravePOIData `json:"results"`
}

// BravePOIData contains point of interest data
type BravePOIData struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name"`
	Address     string                 `json:"address,omitempty"`
	PhoneNumber string                 `json:"phone_number,omitempty"`
	Rating      float64                `json:"rating,omitempty"`
	ReviewCount int                    `json:"review_count,omitempty"`
	Hours       map[string]interface{} `json:"hours,omitempty"`
	Website     string                 `json:"website,omitempty"`
}

// BraveLocalDescriptionsResponse represents the response from local descriptions API
type BraveLocalDescriptionsResponse struct {
	Type    string                     `json:"type"`
	Results []BraveLocationDescription `json:"results"`
}

// BraveLocationDescription contains location description data
type BraveLocationDescription struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// BraveQuery represents the query information in API responses
type BraveQuery struct {
	Original string `json:"original"`
	Show     string `json:"show"`
}

// BraveErrorResponse represents an error response from the API
type BraveErrorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

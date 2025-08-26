package internetsearch

import (
	"time"
)

// SearchResult represents a unified search result
type SearchResult struct {
	Title       string         `json:"title"`
	URL         string         `json:"url"`
	Description string         `json:"description"`
	Type        string         `json:"type"` // web, image, news, video, local
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// SearchResponse represents a unified response structure
type SearchResponse struct {
	Query       string         `json:"query"`
	ResultCount int            `json:"resultCount"`
	Results     []SearchResult `json:"results"`
	Provider    string         `json:"provider"`
	Timestamp   time.Time      `json:"timestamp"`
}

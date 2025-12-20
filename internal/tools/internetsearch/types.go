package internetsearch

import (
	"time"
)

// SearchResult represents a unified search result
type SearchResult struct {
	Title       string         `json:"title"`
	URL         string         `json:"url"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// SearchResponse represents a unified response structure
type SearchResponse struct {
	Results   []SearchResult `json:"results"`
	Provider  string         `json:"provider"`
	Timestamp time.Time      `json:"timestamp"`
}

// QueryResult represents the result of a single query in a multi-query search
type QueryResult struct {
	Query    string         `json:"query"`
	Results  []SearchResult `json:"results"`
	Provider string         `json:"provider,omitempty"`
	Error    string         `json:"error,omitempty"`
}

// MultiSearchResponse represents the response for multi-query searches
type MultiSearchResponse struct {
	Searches []QueryResult `json:"searches"`
	Summary  SearchSummary `json:"summary"`
}

// SearchSummary provides aggregate statistics for multi-query searches
type SearchSummary struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
}

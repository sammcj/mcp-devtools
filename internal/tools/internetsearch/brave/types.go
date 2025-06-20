package brave

import "time"

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

// SearchResult represents a unified search result for consistent output
type SearchResult struct {
	Title       string                 `json:"title"`
	URL         string                 `json:"url"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"` // web, image, news, video, local
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// SearchResponse represents a unified response structure
type SearchResponse struct {
	Query       string         `json:"query"`
	ResultCount int            `json:"resultCount"`
	Results     []SearchResult `json:"results"`
	Provider    string         `json:"provider"`
	Timestamp   time.Time      `json:"timestamp"`
}

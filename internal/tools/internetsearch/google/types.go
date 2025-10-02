package google

// GoogleSearchResponse represents the response from Google Custom Search API
type GoogleSearchResponse struct {
	Items             []GoogleSearchResult `json:"items"`
	Queries           GoogleQueries        `json:"queries"`
	SearchInformation GoogleSearchInfo     `json:"searchInformation"`
}

// GoogleSearchResult represents a single search result
type GoogleSearchResult struct {
	Title       string           `json:"title"`
	HTMLTitle   string           `json:"htmlTitle,omitempty"`
	Link        string           `json:"link"`
	DisplayLink string           `json:"displayLink,omitempty"`
	Snippet     string           `json:"snippet"`
	HTMLSnippet string           `json:"htmlSnippet,omitempty"`
	Image       *GoogleImageInfo `json:"image,omitempty"`
	PageMap     *GooglePageMap   `json:"pagemap,omitempty"`
}

// GoogleImageInfo contains image-specific metadata
type GoogleImageInfo struct {
	ContextLink     string `json:"contextLink,omitempty"`
	Height          int    `json:"height,omitempty"`
	Width           int    `json:"width,omitempty"`
	ByteSize        int    `json:"byteSize,omitempty"`
	ThumbnailLink   string `json:"thumbnailLink,omitempty"`
	ThumbnailHeight int    `json:"thumbnailHeight,omitempty"`
	ThumbnailWidth  int    `json:"thumbnailWidth,omitempty"`
}

// GooglePageMap contains structured data from the page
type GooglePageMap struct {
	CSEThumbnail []GoogleThumbnail `json:"cse_thumbnail,omitempty"`
	CSEImage     []GoogleImage     `json:"cse_image,omitempty"`
}

// GoogleThumbnail represents thumbnail information
type GoogleThumbnail struct {
	Src    string `json:"src,omitempty"`
	Width  string `json:"width,omitempty"`
	Height string `json:"height,omitempty"`
}

// GoogleImage represents image information from pagemap
type GoogleImage struct {
	Src string `json:"src,omitempty"`
}

// GoogleQueries contains query metadata
type GoogleQueries struct {
	Request      []GoogleQueryMetadata `json:"request,omitempty"`
	NextPage     []GoogleQueryMetadata `json:"nextPage,omitempty"`
	PreviousPage []GoogleQueryMetadata `json:"previousPage,omitempty"`
}

// GoogleQueryMetadata contains metadata about a query
type GoogleQueryMetadata struct {
	Title          string `json:"title,omitempty"`
	TotalResults   string `json:"totalResults,omitempty"`
	SearchTerms    string `json:"searchTerms,omitempty"`
	Count          int    `json:"count,omitempty"`
	StartIndex     int    `json:"startIndex,omitempty"`
	InputEncoding  string `json:"inputEncoding,omitempty"`
	OutputEncoding string `json:"outputEncoding,omitempty"`
}

// GoogleSearchInfo contains information about the search
type GoogleSearchInfo struct {
	SearchTime            float64 `json:"searchTime,omitempty"`
	FormattedSearchTime   string  `json:"formattedSearchTime,omitempty"`
	TotalResults          string  `json:"totalResults,omitempty"`
	FormattedTotalResults string  `json:"formattedTotalResults,omitempty"`
}

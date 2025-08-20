package aws

// SearchResult represents a search result from AWS documentation search API
type SearchResult struct {
	RankOrder int     `json:"rank_order"`
	URL       string  `json:"url"`
	Title     string  `json:"title"`
	Context   *string `json:"context,omitempty"`
}

// RecommendationResult represents a recommendation result from AWS documentation
type RecommendationResult struct {
	URL     string  `json:"url"`
	Title   string  `json:"title"`
	Context *string `json:"context,omitempty"`
}

// DocumentationResponse represents a documentation reading response with pagination info
type DocumentationResponse struct {
	URL            string `json:"url"`
	Content        string `json:"content"`
	TotalLength    int    `json:"total_length"`
	StartIndex     int    `json:"start_index"`
	EndIndex       int    `json:"end_index"`
	HasMoreContent bool   `json:"has_more_content"`
	NextStartIndex *int   `json:"next_start_index,omitempty"`
}

// SearchAPIRequest represents the request structure for AWS documentation search API
type SearchAPIRequest struct {
	TextQuery struct {
		Input string `json:"input"`
	} `json:"textQuery"`
	ContextAttributes []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"contextAttributes"`
	AcceptSuggestionBody string   `json:"acceptSuggestionBody"`
	Locales              []string `json:"locales"`
}

// SearchAPIResponse represents the response structure from AWS documentation search API
type SearchAPIResponse struct {
	Suggestions []struct {
		TextExcerptSuggestion struct {
			Link           string `json:"link"`
			Title          string `json:"title"`
			Summary        string `json:"summary"`
			SuggestionBody string `json:"suggestionBody"`
		} `json:"textExcerptSuggestion"`
	} `json:"suggestions"`
}

// RecommendationAPIResponse represents the response structure from AWS recommendations API
type RecommendationAPIResponse struct {
	HighlyRated struct {
		Items []struct {
			URL        string `json:"url"`
			AssetTitle string `json:"assetTitle"`
			Abstract   string `json:"abstract"`
		} `json:"items"`
	} `json:"highlyRated"`
	Journey struct {
		Items []struct {
			Intent string `json:"intent"`
			URLs   []struct {
				URL        string `json:"url"`
				AssetTitle string `json:"assetTitle"`
			} `json:"urls"`
		} `json:"items"`
	} `json:"journey"`
	New struct {
		Items []struct {
			URL         string `json:"url"`
			AssetTitle  string `json:"assetTitle"`
			DateCreated string `json:"dateCreated"`
		} `json:"items"`
	} `json:"new"`
	Similar struct {
		Items []struct {
			URL        string `json:"url"`
			AssetTitle string `json:"assetTitle"`
			Abstract   string `json:"abstract"`
		} `json:"items"`
	} `json:"similar"`
}

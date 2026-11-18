package youcom

// YouComSearchResponse is the response shared by the keyless and keyed search endpoints
type YouComSearchResponse struct {
	Results struct {
		Web  []YouComResult `json:"web"`
		News []YouComResult `json:"news"`
	} `json:"results"`
}

// YouComResult is a single web or news result
type YouComResult struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Snippets    []string `json:"snippets"`
	PageAge     string   `json:"page_age"`
}

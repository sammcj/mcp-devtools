package webfetch

// FetchURLRequest represents the parameters for the fetch-url tool
type FetchURLRequest struct {
	URL        string `json:"url"`
	MaxLength  int    `json:"max_length,omitempty"`
	StartIndex int    `json:"start_index,omitempty"`
	Raw        bool   `json:"raw,omitempty"`
}

// FetchURLResponse represents the response from the fetch-url tool
type FetchURLResponse struct {
	ContentType      string `json:"content_type,omitempty"`
	StatusCode       int    `json:"status_code,omitempty"`
	Content          string `json:"content"`
	Truncated        bool   `json:"truncated"`
	StartIndex       int    `json:"start_index"`
	EndIndex         int    `json:"end_index"`
	TotalLength      int    `json:"total_length"`
	TotalLines       int    `json:"total_lines"`
	StartLine        int    `json:"start_line"`
	EndLine          int    `json:"end_line"`
	NextChunkPreview string `json:"next_chunk_preview,omitempty"`
	RemainingLines   int    `json:"remaining_lines"`
	Message          string `json:"message,omitempty"`
}

// ContentTypeInfo represents information about detected content type
type ContentTypeInfo struct {
	MIME     string
	IsHTML   bool
	IsText   bool
	IsBinary bool
}

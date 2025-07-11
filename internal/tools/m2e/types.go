package m2e

// ConvertRequest represents the request parameters for text conversion
type ConvertRequest struct {
	Text            string `json:"text,omitempty"`              // For inline mode
	FilePath        string `json:"file_path,omitempty"`         // For update_file mode (default)
	KeepSmartQuotes bool   `json:"keep_smart_quotes,omitempty"` // Whether to keep smart quotes (default: false, i.e., normalise them)
}

// ConvertResponse represents the response from text conversion
type ConvertResponse struct {
	ConvertedText string `json:"converted_text"`
	OriginalText  string `json:"original_text"`
	ChangesCount  int    `json:"changes_count"`
}

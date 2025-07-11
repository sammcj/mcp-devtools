package pdf

// PDFRequest represents a request to process a PDF file
type PDFRequest struct {
	// FilePath is the absolute path to the PDF file to process
	FilePath string `json:"file_path"`

	// OutputDir is the directory where markdown and images will be saved
	OutputDir string `json:"output_dir"`

	// ExtractImages indicates whether to extract images from the PDF
	ExtractImages bool `json:"extract_images"`

	// Pages specifies which pages to process (e.g., "1-5", "1,3,5", "all")
	Pages string `json:"pages"`
}

// PDFResponse represents the result of PDF processing
type PDFResponse struct {
	// FilePath is the original PDF file that was processed
	FilePath string `json:"file_path"`

	// MarkdownFile is the path to the generated markdown file
	MarkdownFile string `json:"markdown_file"`

	// ExtractedImages is a list of extracted image file paths
	ExtractedImages []string `json:"extracted_images"`

	// PagesProcessed is the number of pages that were processed
	PagesProcessed int `json:"pages_processed"`

	// TotalPages is the total number of pages in the PDF
	TotalPages int `json:"total_pages"`

	// OutputDir is the directory where files were saved
	OutputDir string `json:"output_dir"`
}

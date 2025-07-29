package filelength

import "time"

// FindLongFilesRequest represents the request to find long files
type FindLongFilesRequest struct {
	Path                  string   `json:"path"`
	LineThreshold         int      `json:"line_threshold"`
	AdditionalExcludes    []string `json:"additional_excludes"`
	SortByDirectoryTotals bool     `json:"sort_by_directory_totals"`
}

// FindLongFilesResponse represents the response with long files found
type FindLongFilesResponse struct {
	Checklist         string    `json:"checklist"`
	LastChecked       time.Time `json:"last_checked"`
	CalculationTime   string    `json:"calculation_time"`
	TotalFilesScanned int       `json:"total_files_scanned"`
	TotalFilesFound   int       `json:"total_files_found"`
	Message           string    `json:"message"`
}

// FileInfo represents information about a file that exceeds the line threshold
type FileInfo struct {
	Path      string `json:"path"`
	LineCount int    `json:"line_count"`
	Directory string `json:"directory"`
	SizeBytes int64  `json:"size_bytes"`
}

// DirectoryInfo represents aggregated information about a directory
type DirectoryInfo struct {
	Path       string     `json:"path"`
	Files      []FileInfo `json:"files"`
	TotalLines int        `json:"total_lines"`
	FileCount  int        `json:"file_count"`
}

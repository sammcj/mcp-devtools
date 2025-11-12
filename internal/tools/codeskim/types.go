package codeskim

// Language represents supported programming languages
type Language string

const (
	LanguagePython     Language = "python"
	LanguageGo         Language = "go"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
	LanguageRust       Language = "rust"
	LanguageC          Language = "c"
	LanguageCPP        Language = "cpp"
	LanguageBash       Language = "bash"
	LanguageHTML       Language = "html"
	LanguageCSS        Language = "css"
	LanguageSwift      Language = "swift"
	LanguageJava       Language = "java"
	LanguageYAML       Language = "yaml"
	LanguageHCL        Language = "hcl"
)

// SkimRequest represents a request to transform code
type SkimRequest struct {
	Source       string `json:"source"` // File path, directory path, or glob pattern
	ClearCache   bool   `json:"clear_cache,omitempty"`
	StartingLine int    `json:"starting_line,omitempty"` // Line number to start from (1-based)
}

// FileResult represents the result for a single file
type FileResult struct {
	Path             string   `json:"path"`
	Transformed      string   `json:"transformed"`
	Language         Language `json:"language"`
	FromCache        bool     `json:"from_cache"`
	Truncated        bool     `json:"truncated,omitempty"`
	TotalLines       *int     `json:"total_lines,omitempty"`
	ReturnedLines    *int     `json:"returned_lines,omitempty"`
	NextStartingLine *int     `json:"next_starting_line,omitempty"`
	Error            string   `json:"error,omitempty"`
}

// SkimResponse represents the response from a code transformation
type SkimResponse struct {
	Files            []FileResult `json:"files"`
	TotalFiles       int          `json:"total_files"`
	ProcessedFiles   int          `json:"processed_files"`
	FailedFiles      int          `json:"failed_files"`
	ProcessingTimeMs *int64       `json:"processing_time_ms,omitempty"`
}

// NodeTypes represents language-specific AST node type names
type NodeTypes struct {
	Function string
	Method   string
	Class    string
}

//go:build cgo && (darwin || (linux && amd64))

package codesearch

// ActionType represents the type of action to perform
type ActionType string

const (
	ActionSearch ActionType = "search"
	ActionIndex  ActionType = "index"
	ActionStatus ActionType = "status"
	ActionClear  ActionType = "clear"
)

// SearchRequest represents a request to the code_search tool
type SearchRequest struct {
	Action    ActionType `json:"action"`
	Source    []string   `json:"source,omitempty"`    // Paths for index/search/status/clear
	Query     string     `json:"query,omitempty"`     // Natural language query for search
	Limit     int        `json:"limit,omitempty"`     // Max results (default: 10)
	Threshold float64    `json:"threshold,omitempty"` // Similarity threshold (default: 0.5)
}

// SearchResult represents a single search result
type SearchResult struct {
	Path       string  `json:"path"`
	Name       string  `json:"name"`
	Type       string  `json:"type"` // "function", "method", "class"
	Signature  string  `json:"signature"`
	Similarity float64 `json:"similarity"`
	Line       int     `json:"line,omitempty"`
}

// SearchResponse represents the response from a search action
type SearchResponse struct {
	Results      []SearchResult `json:"results,omitempty"`
	TotalMatches int            `json:"total_matches,omitempty"` // Total matches before limit applied
	LimitApplied int            `json:"limit_applied,omitempty"` // Limit that was applied (only if truncated)
	LastIndexed  string         `json:"last_indexed,omitempty"`  // When index was last updated, e.g. "2025-10-23 06:30"
}

// IndexResponse represents the response from an index action
type IndexResponse struct {
	IndexedFiles int `json:"indexed_files"`
	IndexedItems int `json:"indexed_items"`
	SkippedFiles int `json:"skipped_files,omitempty"`
}

// StatusResponse represents the response from a status action
type StatusResponse struct {
	Indexed        bool   `json:"indexed"`
	TotalFiles     int    `json:"total_files"`
	TotalItems     int    `json:"total_items"`
	ModelLoaded    bool   `json:"model_loaded"`
	RuntimeLoaded  bool   `json:"runtime_loaded"`
	RuntimeVersion string `json:"runtime_version,omitempty"`
}

// ClearResponse represents the response from a clear action
type ClearResponse struct {
	Cleared      bool `json:"cleared"`
	FilesCleared int  `json:"files_cleared"`
	ItemsCleared int  `json:"items_cleared"`
}

// IndexedItem represents an item stored in the vector database
type IndexedItem struct {
	ID        string    `json:"id"`        // Unique identifier
	Path      string    `json:"path"`      // File path
	Name      string    `json:"name"`      // Function/class name
	Type      string    `json:"type"`      // "function", "method", "class"
	Signature string    `json:"signature"` // Full signature
	Line      int       `json:"line"`      // Line number
	Embedding []float32 `json:"embedding"` // Vector embedding
}

// EmbeddingConfig holds configuration for the embedding engine
type EmbeddingConfig struct {
	ModelPath    string // Path to model files
	RuntimePath  string // Path to ONNX runtime library
	BatchSize    int    // Batch size for embedding generation
	MaxSeqLength int    // Maximum sequence length
}

// DefaultEmbeddingConfig returns the default embedding configuration
func DefaultEmbeddingConfig() EmbeddingConfig {
	return EmbeddingConfig{
		ModelPath:    "", // Will be set from environment or default
		RuntimePath:  "", // Will be auto-detected
		BatchSize:    32,
		MaxSeqLength: 512,
	}
}

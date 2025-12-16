//go:build cgo && (darwin || (linux && amd64))

package vectorstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	chromem "github.com/philippgille/chromem-go"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultStorePath is the default path for vector storage
	DefaultStorePath = ".mcp-devtools/embeddings"
	// CollectionName is the name of the chromem collection
	CollectionName = "code-search"
)

// Item represents an indexed item with its embedding
type Item struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Signature string    `json:"signature"`
	Line      int       `json:"line"`
	Embedding []float32 `json:"embedding"`
}

// SearchResult represents a search result with similarity score
type SearchResult struct {
	Path       string  `json:"path"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Signature  string  `json:"signature"`
	Similarity float64 `json:"similarity"`
	Line       int     `json:"line,omitempty"`
}

// StatusResult represents the store status
type StatusResult struct {
	Indexed        bool     `json:"indexed"`
	TotalFiles     int      `json:"total_files"`
	TotalItems     int      `json:"total_items"`
	IndexedPaths   []string `json:"indexed_paths,omitempty"`
	ModelLoaded    bool     `json:"model_loaded"`
	ModelPath      string   `json:"model_path,omitempty"`
	RuntimeLoaded  bool     `json:"runtime_loaded"`
	RuntimeVersion string   `json:"runtime_version,omitempty"`
}

// ClearResult represents the result of a clear operation
type ClearResult struct {
	Cleared      bool `json:"cleared"`
	FilesCleared int  `json:"files_cleared"`
	ItemsCleared int  `json:"items_cleared"`
}

// Store provides vector storage and similarity search using chromem-go
type Store struct {
	storePath  string
	logger     *logrus.Logger
	db         *chromem.DB
	collection *chromem.Collection
	files      map[string]bool // file path -> indexed (for tracking)
	mu         sync.RWMutex
}

// NewStore creates a new vector store with chromem-go persistence
func NewStore(logger *logrus.Logger) (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	storePath := filepath.Join(homeDir, DefaultStorePath)
	if err := os.MkdirAll(storePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}

	// Create chromem DB with persistence
	dbPath := filepath.Join(storePath, "chromem.gob")
	db, err := chromem.NewPersistentDB(dbPath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create chromem DB: %w", err)
	}

	// Get or create collection
	// We use a no-op embedding function since we provide embeddings ourselves
	collection, err := db.GetOrCreateCollection(CollectionName, nil, noOpEmbeddingFunc())
	if err != nil {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}

	store := &Store{
		storePath:  storePath,
		logger:     logger,
		db:         db,
		collection: collection,
		files:      make(map[string]bool),
	}

	// Load file tracking info from existing documents
	store.loadFileTracking()

	return store, nil
}

// noOpEmbeddingFunc returns an embedding function that does nothing
// since we provide our own embeddings
func noOpEmbeddingFunc() chromem.EmbeddingFunc {
	return func(_ context.Context, _ string) ([]float32, error) {
		// This shouldn't be called since we provide embeddings
		return nil, fmt.Errorf("embedding function called but embeddings should be pre-computed")
	}
}

// loadFileTracking loads file paths from existing documents
func (s *Store) loadFileTracking() {
	// chromem-go doesn't expose a way to list all documents
	// so we start fresh with file tracking
	// This is acceptable since we can re-index if needed
	s.logger.Debug("File tracking initialised")
}

// Add adds an item to the store
func (s *Store) Add(item *Item) error {
	return s.AddBatch([]*Item{item})
}

// AddBatch adds multiple items to the store
func (s *Store) AddBatch(items []*Item) error {
	if len(items) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := context.Background()

	// Convert items to chromem documents
	docs := make([]chromem.Document, len(items))
	for i, item := range items {
		// Create metadata for filtering
		metadata := map[string]string{
			"path":      item.Path,
			"name":      item.Name,
			"type":      item.Type,
			"line":      fmt.Sprintf("%d", item.Line),
			"signature": item.Signature,
		}

		docs[i] = chromem.Document{
			ID:        item.ID,
			Content:   item.Signature, // Use signature as searchable content
			Metadata:  metadata,
			Embedding: item.Embedding,
		}

		// Track file
		s.files[item.Path] = true
	}

	// Add documents to collection
	if err := s.collection.AddDocuments(ctx, docs, 1); err != nil {
		return fmt.Errorf("failed to add documents to chromem: %w", err)
	}

	return nil
}

// Search performs similarity search. Returns results, total matches (before limit), and error.
func (s *Store) Search(ctx context.Context, queryEmbedding []float32, limit int, threshold float64, filterPaths []string) ([]SearchResult, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.collection == nil {
		return nil, 0, fmt.Errorf("collection not initialised")
	}

	// Build where filter for paths
	var whereFilter map[string]string
	if len(filterPaths) > 0 && len(filterPaths) == 1 {
		// chromem only supports exact match, so we can only filter by single path prefix
		// For multiple paths, we filter post-query
		whereFilter = map[string]string{"path": filterPaths[0]}
	}

	// Query collection using pre-computed embedding
	// Request more results to account for threshold filtering
	results, err := s.collection.QueryEmbedding(ctx, queryEmbedding, limit*3, whereFilter, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query chromem: %w", err)
	}

	// Convert results and apply threshold/path filtering
	searchResults := make([]SearchResult, 0, len(results))
	for _, r := range results {
		// Apply similarity threshold
		if float64(r.Similarity) < threshold {
			continue
		}

		// Apply path filter (for multiple paths)
		if len(filterPaths) > 1 {
			matched := false
			if path, ok := r.Metadata["path"]; ok {
				for _, filterPath := range filterPaths {
					if strings.HasPrefix(path, filterPath) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		// Parse line number
		line := 0
		if lineStr, ok := r.Metadata["line"]; ok {
			_, _ = fmt.Sscanf(lineStr, "%d", &line)
		}

		searchResults = append(searchResults, SearchResult{
			Path:       r.Metadata["path"],
			Name:       r.Metadata["name"],
			Type:       r.Metadata["type"],
			Signature:  r.Metadata["signature"],
			Similarity: math.Round(float64(r.Similarity)*100) / 100, // Round to 2 decimal places
			Line:       line,
		})
	}

	// Track total matches before limiting
	totalMatches := len(searchResults)

	// Limit results
	if len(searchResults) > limit {
		searchResults = searchResults[:limit]
	}

	return searchResults, totalMatches, nil
}

// Count returns the number of indexed items
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.collection == nil {
		return 0
	}

	return s.collection.Count()
}

// Status returns the current store status
func (s *Store) Status() *StatusResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	paths := make([]string, 0, len(s.files))
	for path := range s.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	count := 0
	if s.collection != nil {
		count = s.collection.Count()
	}

	return &StatusResult{
		Indexed:      count > 0,
		TotalFiles:   len(s.files),
		TotalItems:   count,
		IndexedPaths: paths,
	}
}

// Clear removes items from the store
func (s *Store) Clear(filterPaths []string) (*ClearResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prevCount := 0
	if s.collection != nil {
		prevCount = s.collection.Count()
	}
	prevFiles := len(s.files)

	if len(filterPaths) == 0 {
		// Clear everything by recreating collection
		if err := s.db.DeleteCollection(CollectionName); err != nil {
			s.logger.WithError(err).Warn("Failed to delete collection")
		}

		// Recreate collection
		collection, err := s.db.GetOrCreateCollection(CollectionName, nil, noOpEmbeddingFunc())
		if err != nil {
			return nil, fmt.Errorf("failed to recreate collection: %w", err)
		}
		s.collection = collection
		s.files = make(map[string]bool)

		return &ClearResult{
			Cleared:      true,
			FilesCleared: prevFiles,
			ItemsCleared: prevCount,
		}, nil
	}

	// For path-specific clearing, chromem-go doesn't support deletion yet
	// We would need to rebuild the collection without matching items
	// For now, log a warning and suggest full clear
	s.logger.Warn("Path-specific clearing not fully supported, performing full clear")

	if err := s.db.DeleteCollection(CollectionName); err != nil {
		s.logger.WithError(err).Warn("Failed to delete collection")
	}

	collection, err := s.db.GetOrCreateCollection(CollectionName, nil, noOpEmbeddingFunc())
	if err != nil {
		return nil, fmt.Errorf("failed to recreate collection: %w", err)
	}
	s.collection = collection
	s.files = make(map[string]bool)

	return &ClearResult{
		Cleared:      true,
		FilesCleared: prevFiles,
		ItemsCleared: prevCount,
	}, nil
}

// IsFileIndexed checks if a file is already indexed
func (s *Store) IsFileIndexed(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.files[path]
}

// Save persists the store to disk
func (s *Store) Save() error {
	// chromem-go with PersistentDB auto-persists
	// No additional action needed
	return nil
}

// StorePath returns the path to the store directory
func (s *Store) StorePath() string {
	return s.storePath
}

// GenerateItemID generates a unique ID for an item
func GenerateItemID(path, name string, line int) string {
	data := fmt.Sprintf("%s:%s:%d", path, name, line)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

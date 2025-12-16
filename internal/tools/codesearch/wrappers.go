//go:build cgo && (darwin || (linux && amd64))

package codesearch

import (
	"context"

	"github.com/sammcj/mcp-devtools/internal/tools/codesearch/embedding"
	"github.com/sammcj/mcp-devtools/internal/tools/codesearch/filetracker"
	"github.com/sammcj/mcp-devtools/internal/tools/codesearch/index"
	"github.com/sammcj/mcp-devtools/internal/tools/codesearch/vectorstore"
	"github.com/sirupsen/logrus"
)

// EmbeddingEngine wraps the embedding package
type EmbeddingEngine struct {
	*embedding.Engine
}

// NewEmbeddingEngine creates a new embedding engine
func NewEmbeddingEngine(config EmbeddingConfig, logger *logrus.Logger) (*EmbeddingEngine, error) {
	embConfig := embedding.Config{
		ModelPath:    config.ModelPath,
		RuntimePath:  config.RuntimePath,
		BatchSize:    config.BatchSize,
		MaxSeqLength: config.MaxSeqLength,
	}

	engine, err := embedding.NewEngine(embConfig, logger)
	if err != nil {
		return nil, err
	}

	return &EmbeddingEngine{Engine: engine}, nil
}

// Embed generates an embedding for text
func (e *EmbeddingEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	return e.Engine.Embed(ctx, text)
}

// VectorStore wraps the vectorstore package
type VectorStore struct {
	*vectorstore.Store
}

// NewVectorStore creates a new vector store
func NewVectorStore(logger *logrus.Logger) (*VectorStore, error) {
	store, err := vectorstore.NewStore(logger)
	if err != nil {
		return nil, err
	}

	return &VectorStore{Store: store}, nil
}

// Search performs similarity search and returns SearchResult slice
func (v *VectorStore) Search(ctx context.Context, queryEmbedding []float32, limit int, threshold float64, filterPaths []string) ([]SearchResult, error) {
	results, err := v.Store.Search(ctx, queryEmbedding, limit, threshold, filterPaths)
	if err != nil {
		return nil, err
	}

	// Convert vectorstore.SearchResult to codesearch.SearchResult
	searchResults := make([]SearchResult, len(results))
	for i, r := range results {
		searchResults[i] = SearchResult{
			Path:       r.Path,
			Name:       r.Name,
			Type:       r.Type,
			Signature:  r.Signature,
			Similarity: r.Similarity,
			Line:       r.Line,
		}
	}

	return searchResults, nil
}

// Status returns the store status
func (v *VectorStore) Status() *StatusResponse {
	status := v.Store.Status()
	return &StatusResponse{
		Indexed:    status.Indexed,
		TotalFiles: status.TotalFiles,
		TotalItems: status.TotalItems,
	}
}

// Clear clears the store
func (v *VectorStore) Clear(filterPaths []string) (*ClearResponse, error) {
	result, err := v.Store.Clear(filterPaths)
	if err != nil {
		return nil, err
	}

	return &ClearResponse{
		Cleared:      result.Cleared,
		FilesCleared: result.FilesCleared,
		ItemsCleared: result.ItemsCleared,
	}, nil
}

// StorePath returns the path to the store directory
func (v *VectorStore) StorePath() string {
	return v.Store.StorePath()
}

// FileTracker wraps the filetracker package
type FileTracker = filetracker.Tracker

// NewFileTracker creates a new file tracker
func NewFileTracker(storePath string, logger *logrus.Logger) (*FileTracker, error) {
	return filetracker.NewTracker(storePath, logger)
}

// Indexer wraps the index package
type Indexer struct {
	*index.Indexer
}

// NewIndexer creates a new indexer
func NewIndexer(embedder *EmbeddingEngine, store *VectorStore, tracker *FileTracker, logger *logrus.Logger) *Indexer {
	indexer := index.NewIndexer(embedder.Engine, store.Store, tracker, logger)
	return &Indexer{Indexer: indexer}
}

// Index indexes the specified paths
func (i *Indexer) Index(ctx context.Context, paths []string) (*IndexResponse, error) {
	result, err := i.Indexer.Index(ctx, paths)
	if err != nil {
		return nil, err
	}

	return &IndexResponse{
		IndexedFiles: result.IndexedFiles,
		IndexedItems: result.IndexedItems,
		SkippedFiles: result.SkippedFiles,
	}, nil
}

// IndexFiles indexes specific files (used for reindexing stale files)
func (i *Indexer) IndexFiles(ctx context.Context, files []string) (*IndexResponse, error) {
	result, err := i.Indexer.IndexFiles(ctx, files)
	if err != nil {
		return nil, err
	}

	return &IndexResponse{
		IndexedFiles: result.IndexedFiles,
		IndexedItems: result.IndexedItems,
		SkippedFiles: result.SkippedFiles,
	}, nil
}

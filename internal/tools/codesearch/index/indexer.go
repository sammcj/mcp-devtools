//go:build cgo && (darwin || (linux && amd64))

package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sammcj/mcp-devtools/internal/tools/codesearch/embedding"
	"github.com/sammcj/mcp-devtools/internal/tools/codesearch/filetracker"
	"github.com/sammcj/mcp-devtools/internal/tools/codesearch/vectorstore"
	"github.com/sammcj/mcp-devtools/internal/tools/codeskim"
	"github.com/sirupsen/logrus"
)

// IndexResult contains the results of an indexing operation
type IndexResult struct {
	IndexedFiles    int   `json:"indexed_files"`
	IndexedItems    int   `json:"indexed_items"`
	SkippedFiles    int   `json:"skipped_files"`
	IndexTimeMs     int64 `json:"index_time_ms"`
	EmbeddingTimeMs int64 `json:"embedding_time_ms"`
}

// Indexer indexes code for semantic search
type Indexer struct {
	embedder *embedding.Engine
	store    *vectorstore.Store
	tracker  *filetracker.Tracker
	logger   *logrus.Logger
}

// NewIndexer creates a new indexer
func NewIndexer(embedder *embedding.Engine, store *vectorstore.Store, tracker *filetracker.Tracker, logger *logrus.Logger) *Indexer {
	return &Indexer{
		embedder: embedder,
		store:    store,
		tracker:  tracker,
		logger:   logger,
	}
}

// Index indexes the specified paths
func (i *Indexer) Index(ctx context.Context, paths []string) (*IndexResult, error) {
	startTime := time.Now()
	result := &IndexResult{}

	// Ensure embedder is ready
	if err := i.embedder.EnsureReady(ctx); err != nil {
		return nil, fmt.Errorf("failed to prepare embedder: %w", err)
	}

	// Resolve all files from paths
	var allFiles []string
	for _, path := range paths {
		files, err := resolveFiles(path)
		if err != nil {
			i.logger.WithError(err).WithField("path", path).Warn("Failed to resolve path")
			continue
		}
		allFiles = append(allFiles, files...)
	}

	if len(allFiles) == 0 {
		return nil, fmt.Errorf("no supported files found in specified paths")
	}

	// Process each file
	var itemsToEmbed []itemToEmbed
	var indexedFilePaths []string
	var embedStartTime time.Time

	for _, filePath := range allFiles {
		// Skip already indexed files (incremental indexing)
		if i.tracker.IsIndexed(filePath) {
			result.SkippedFiles++
			continue
		}

		// Extract code items using code_skim
		items, err := i.extractItems(ctx, filePath)
		if err != nil {
			i.logger.WithError(err).WithField("file", filePath).Warn("Failed to extract items")
			result.SkippedFiles++
			continue
		}

		result.IndexedFiles++
		itemsToEmbed = append(itemsToEmbed, items...)
		indexedFilePaths = append(indexedFilePaths, filePath)
	}

	if len(itemsToEmbed) == 0 {
		return result, nil
	}

	// Generate embeddings in batches
	embedStartTime = time.Now()
	batchSize := 32
	var storeItems []*vectorstore.Item

	for start := 0; start < len(itemsToEmbed); start += batchSize {
		end := min(start+batchSize, len(itemsToEmbed))
		batch := itemsToEmbed[start:end]

		// Extract texts for embedding
		texts := make([]string, len(batch))
		for j, item := range batch {
			texts[j] = item.signature
		}

		// Generate embeddings
		embeddings, err := i.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			i.logger.WithError(err).Warn("Failed to generate embeddings for batch")
			continue
		}

		// Create store items
		for j, item := range batch {
			storeItem := &vectorstore.Item{
				ID:        vectorstore.GenerateItemID(item.path, item.name, item.line),
				Path:      item.path,
				Name:      item.name,
				Type:      item.itemType,
				Signature: item.signature,
				Line:      item.line,
				Embedding: embeddings[j],
			}
			storeItems = append(storeItems, storeItem)
		}
	}

	result.EmbeddingTimeMs = time.Since(embedStartTime).Milliseconds()

	// Add to store
	if err := i.store.AddBatch(storeItems); err != nil {
		return nil, fmt.Errorf("failed to add items to store: %w", err)
	}

	result.IndexedItems = len(storeItems)
	result.IndexTimeMs = time.Since(startTime).Milliseconds()

	// Track indexed files with their mtimes
	if err := i.tracker.MarkIndexedBatch(indexedFilePaths); err != nil {
		i.logger.WithError(err).Warn("Failed to track indexed files")
	}
	if err := i.tracker.Save(); err != nil {
		i.logger.WithError(err).Warn("Failed to persist file tracker")
	}

	// Persist store
	if err := i.store.Save(); err != nil {
		i.logger.WithError(err).Warn("Failed to persist store")
	}

	return result, nil
}

// IndexFiles indexes specific files without checking if already indexed
// Used for reindexing stale files
func (i *Indexer) IndexFiles(ctx context.Context, files []string) (*IndexResult, error) {
	startTime := time.Now()
	result := &IndexResult{}

	// Ensure embedder is ready
	if err := i.embedder.EnsureReady(ctx); err != nil {
		return nil, fmt.Errorf("failed to prepare embedder: %w", err)
	}

	if len(files) == 0 {
		return result, nil
	}

	// Process each file (no skip check - always reindex)
	var itemsToEmbed []itemToEmbed
	var indexedFilePaths []string
	var embedStartTime time.Time

	for _, filePath := range files {
		// Check file exists and is supported
		if !isSupportedFile(filePath) {
			result.SkippedFiles++
			continue
		}

		// Extract code items using code_skim
		items, err := i.extractItems(ctx, filePath)
		if err != nil {
			i.logger.WithError(err).WithField("file", filePath).Warn("Failed to extract items")
			result.SkippedFiles++
			continue
		}

		result.IndexedFiles++
		itemsToEmbed = append(itemsToEmbed, items...)
		indexedFilePaths = append(indexedFilePaths, filePath)
	}

	if len(itemsToEmbed) == 0 {
		return result, nil
	}

	// Generate embeddings in batches
	embedStartTime = time.Now()
	batchSize := 32
	var storeItems []*vectorstore.Item

	for start := 0; start < len(itemsToEmbed); start += batchSize {
		end := min(start+batchSize, len(itemsToEmbed))
		batch := itemsToEmbed[start:end]

		// Extract texts for embedding
		texts := make([]string, len(batch))
		for j, item := range batch {
			texts[j] = item.signature
		}

		// Generate embeddings
		embeddings, err := i.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			i.logger.WithError(err).Warn("Failed to generate embeddings for batch")
			continue
		}

		// Create store items
		for j, item := range batch {
			storeItem := &vectorstore.Item{
				ID:        vectorstore.GenerateItemID(item.path, item.name, item.line),
				Path:      item.path,
				Name:      item.name,
				Type:      item.itemType,
				Signature: item.signature,
				Line:      item.line,
				Embedding: embeddings[j],
			}
			storeItems = append(storeItems, storeItem)
		}
	}

	result.EmbeddingTimeMs = time.Since(embedStartTime).Milliseconds()

	// Add to store
	if err := i.store.AddBatch(storeItems); err != nil {
		return nil, fmt.Errorf("failed to add items to store: %w", err)
	}

	result.IndexedItems = len(storeItems)
	result.IndexTimeMs = time.Since(startTime).Milliseconds()

	// Track indexed files with their mtimes
	if err := i.tracker.MarkIndexedBatch(indexedFilePaths); err != nil {
		i.logger.WithError(err).Warn("Failed to track indexed files")
	}
	if err := i.tracker.Save(); err != nil {
		i.logger.WithError(err).Warn("Failed to persist file tracker")
	}

	// Persist store
	if err := i.store.Save(); err != nil {
		i.logger.WithError(err).Warn("Failed to persist store")
	}

	return result, nil
}

// itemToEmbed represents an item to be embedded
type itemToEmbed struct {
	path      string
	name      string
	itemType  string
	signature string
	line      int
}

// extractItems extracts indexable items from a file using code_skim
func (i *Indexer) extractItems(ctx context.Context, filePath string) ([]itemToEmbed, error) {
	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Detect language
	lang, err := codeskim.DetectLanguage(filePath)
	if err != nil {
		return nil, fmt.Errorf("unsupported language: %w", err)
	}

	// Transform with graph extraction to get function/class info
	isTSX := strings.HasSuffix(filePath, ".tsx")
	result, err := codeskim.TransformWithOptions(ctx, string(content), lang, isTSX, nil, true)
	if err != nil {
		return nil, fmt.Errorf("failed to transform: %w", err)
	}

	var items []itemToEmbed

	// Extract from graph if available
	if result.Graph != nil {
		// Add functions
		for _, fn := range result.Graph.Functions {
			sig := fn.Signature
			if sig == "" {
				// Fallback if signature extraction failed
				sig = "func " + fn.Name
			}
			items = append(items, itemToEmbed{
				path:      filePath,
				name:      fn.Name,
				itemType:  "function",
				signature: sig,
				line:      fn.Line,
			})
		}

		// Add classes
		for _, class := range result.Graph.Classes {
			items = append(items, itemToEmbed{
				path:      filePath,
				name:      class.Name,
				itemType:  "class",
				signature: formatClassSignature(class),
				line:      0, // Classes don't have line numbers yet
			})
		}
	}

	return items, nil
}

// formatClassSignature formats a class signature for embedding
func formatClassSignature(class codeskim.ClassInfo) string {
	var parts []string
	parts = append(parts, "class "+class.Name)
	if class.Extends != "" {
		parts = append(parts, "extends "+class.Extends)
	}
	if len(class.Implements) > 0 {
		parts = append(parts, "implements "+strings.Join(class.Implements, ", "))
	}
	return strings.Join(parts, " ")
}

// resolveFiles resolves a path to a list of files
func resolveFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		// Single file
		if isSupportedFile(path) {
			return []string{path}, nil
		}
		return nil, nil
	}

	// Directory - walk and find supported files
	var files []string
	err = filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && filePath != path {
			return filepath.SkipDir
		}

		// Skip hidden files
		if !d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		if !d.IsDir() && isSupportedFile(filePath) {
			files = append(files, filePath)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// isSupportedFile checks if a file is supported for indexing
func isSupportedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	supportedExts := map[string]bool{
		".py":    true,
		".go":    true,
		".js":    true,
		".jsx":   true,
		".ts":    true,
		".tsx":   true,
		".rs":    true,
		".java":  true,
		".swift": true,
		".c":     true,
		".cpp":   true,
		".h":     true,
		".hpp":   true,
	}
	return supportedExts[ext]
}

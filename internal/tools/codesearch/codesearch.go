//go:build cgo && (darwin || (linux && amd64))

package codesearch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sammcj/mcp-devtools/internal/tools/codesearch/filetracker"
	"github.com/sirupsen/logrus"
)

// CodeSearchTool implements the tools.Tool interface for semantic code search
type CodeSearchTool struct {
	embedder       *EmbeddingEngine
	vectorStore    *VectorStore
	indexer        *Indexer
	fileTracker    *filetracker.Tracker
	staleThreshold time.Duration // 0 = disabled
	initOnce       sync.Once
	initErr        error
}

const (
	toolName = "code_search"
)

// init registers the tool with the registry
func init() {
	registry.Register(&CodeSearchTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *CodeSearchTool) Definition() mcp.Tool {
	return mcp.NewTool(
		toolName,
		mcp.WithDescription("Finds code by natural language description using local embeddings. Index a codebase first, then use search."),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("'index' to index paths, 'search' to query, 'status' to check, 'clear' to reset"),
			mcp.Enum("search", "index", "status", "clear"),
		),
		mcp.WithArray("source",
			mcp.Description("Fully qualified path to index or filter results by"),
			mcp.WithStringItems(),
		),
		mcp.WithString("query",
			mcp.Description("Short natural language query. Describe what the code does if you don't know what it's called (e.g., 'function that handles authentication')"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results (10, Optional)"),
		),
		mcp.WithNumber("threshold",
			mcp.Description("Min similarity 0-1 (0.3, Optional)"),
		),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
}

// Execute executes the code search tool
func (t *CodeSearchTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	logger.Info("Executing code_search")

	// Parse request
	req, err := t.parseRequest(args)
	if err != nil {
		return nil, err
	}

	// Initialise components on first use
	t.initOnce.Do(func() {
		t.initErr = t.initialise(logger)
	})
	if t.initErr != nil {
		return nil, fmt.Errorf("failed to initialise code_search: %w", t.initErr)
	}

	// Execute action
	switch req.Action {
	case ActionSearch:
		return t.executeSearch(ctx, req, logger)
	case ActionIndex:
		return t.executeIndex(ctx, req, logger)
	case ActionStatus:
		return t.executeStatus(ctx, req, logger)
	case ActionClear:
		return t.executeClear(ctx, req, logger)
	default:
		return nil, fmt.Errorf("unknown action: %s", req.Action)
	}
}

// initialise sets up the embedding engine, vector store, and indexer
func (t *CodeSearchTool) initialise(logger *logrus.Logger) error {
	logger.Info("Initialising code_search components")

	// Create embedding engine
	config := DefaultEmbeddingConfig()
	embedder, err := NewEmbeddingEngine(config, logger)
	if err != nil {
		return fmt.Errorf("failed to create embedding engine: %w", err)
	}
	t.embedder = embedder

	// Create vector store
	vectorStore, err := NewVectorStore(logger)
	if err != nil {
		return fmt.Errorf("failed to create vector store: %w", err)
	}
	t.vectorStore = vectorStore

	// Create file tracker for stale file detection
	tracker, err := NewFileTracker(vectorStore.StorePath(), logger)
	if err != nil {
		return fmt.Errorf("failed to create file tracker: %w", err)
	}
	t.fileTracker = tracker

	// Create indexer with file tracker
	t.indexer = NewIndexer(t.embedder, t.vectorStore, t.fileTracker, logger)

	// Parse stale threshold from environment (disabled by default)
	if thresholdStr := os.Getenv("CODE_SEARCH_STALE_THRESHOLD"); thresholdStr != "" {
		threshold, err := time.ParseDuration(thresholdStr)
		if err != nil {
			logger.WithError(err).Warn("Invalid CODE_SEARCH_STALE_THRESHOLD, disabling stale detection")
		} else {
			t.staleThreshold = threshold
			logger.WithField("threshold", threshold).Info("Stale file detection enabled")
		}
	}

	logger.Info("code_search initialised successfully")
	return nil
}

// executeSearch performs a semantic search
func (t *CodeSearchTool) executeSearch(ctx context.Context, req *SearchRequest, logger *logrus.Logger) (*mcp.CallToolResult, error) {
	if req.Query == "" {
		return nil, fmt.Errorf("query is required for search action")
	}

	// Check for stale files if threshold is configured
	if t.staleThreshold > 0 {
		staleFiles := t.fileTracker.GetStaleFiles(t.staleThreshold)
		if len(staleFiles) > 0 {
			logger.WithField("stale_files", len(staleFiles)).Debug("Reindexing stale files before search")

			// Remove stale entries from tracker before reindexing
			t.fileTracker.RemoveFiles(staleFiles)

			// Reindex stale files
			if _, err := t.indexer.IndexFiles(ctx, staleFiles); err != nil {
				logger.WithError(err).Warn("Failed to reindex some stale files")
			}
		}
	}

	// Get limit and threshold with defaults
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 0.3
	}

	// Embed the query
	queryEmbedding, err := t.embedder.Embed(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Search the vector store
	results, err := t.vectorStore.Search(ctx, queryEmbedding, limit, threshold, req.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	response := &SearchResponse{
		Results:      results,
		TotalIndexed: t.vectorStore.Count(),
	}

	logger.WithFields(logrus.Fields{
		"query":   req.Query,
		"results": len(results),
	}).Debug("Search completed")

	return t.newToolResultJSON(response)
}

// executeIndex indexes the specified paths
func (t *CodeSearchTool) executeIndex(ctx context.Context, req *SearchRequest, logger *logrus.Logger) (*mcp.CallToolResult, error) {
	if len(req.Source) == 0 {
		return nil, fmt.Errorf("source paths are required for index action")
	}

	result, err := t.indexer.Index(ctx, req.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to index: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"indexed_files": result.IndexedFiles,
		"indexed_items": result.IndexedItems,
	}).Info("Indexing completed")

	return t.newToolResultJSON(result)
}

// executeStatus returns the current index status
func (t *CodeSearchTool) executeStatus(_ context.Context, _ *SearchRequest, _ *logrus.Logger) (*mcp.CallToolResult, error) {
	status := t.vectorStore.Status()
	status.ModelLoaded = t.embedder.IsLoaded()
	status.RuntimeLoaded = t.embedder.IsRuntimeLoaded()

	if t.embedder.IsRuntimeLoaded() {
		status.RuntimeVersion = t.embedder.RuntimeVersion()
	}

	return t.newToolResultJSON(status)
}

// executeClear clears the index
func (t *CodeSearchTool) executeClear(_ context.Context, req *SearchRequest, logger *logrus.Logger) (*mcp.CallToolResult, error) {
	result, err := t.vectorStore.Clear(req.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to clear index: %w", err)
	}

	// Clear file tracker as well
	if len(req.Source) == 0 {
		t.fileTracker.Clear()
	} else {
		t.fileTracker.RemoveFiles(req.Source)
	}
	if err := t.fileTracker.Save(); err != nil {
		logger.WithError(err).Warn("Failed to save file tracker after clear")
	}

	logger.WithFields(logrus.Fields{
		"files_cleared": result.FilesCleared,
		"items_cleared": result.ItemsCleared,
	}).Info("Index cleared")

	return t.newToolResultJSON(result)
}

// parseRequest parses and validates the tool arguments
func (t *CodeSearchTool) parseRequest(args map[string]any) (*SearchRequest, error) {
	req := &SearchRequest{}

	// Parse action (required)
	actionRaw, ok := args["action"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter 'action'")
	}
	actionStr, ok := actionRaw.(string)
	if !ok {
		return nil, fmt.Errorf("action must be a string")
	}
	req.Action = ActionType(actionStr)

	// Parse source (optional array)
	if sourceRaw, ok := args["source"]; ok {
		sourceArray, ok := sourceRaw.([]any)
		if !ok {
			return nil, fmt.Errorf("source must be an array of strings")
		}
		for i, item := range sourceArray {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("source array item %d must be a string", i)
			}
			if str != "" {
				req.Source = append(req.Source, str)
			}
		}
	}

	// Parse query (optional)
	if queryRaw, ok := args["query"]; ok {
		if queryStr, ok := queryRaw.(string); ok {
			req.Query = queryStr
		}
	}

	// Parse limit (optional)
	if limitRaw, ok := args["limit"]; ok {
		switch v := limitRaw.(type) {
		case float64:
			req.Limit = int(v)
		case int:
			req.Limit = v
		}
	}

	// Parse threshold (optional)
	if thresholdRaw, ok := args["threshold"]; ok {
		if thresholdFloat, ok := thresholdRaw.(float64); ok {
			req.Threshold = thresholdFloat
		}
	}

	return req, nil
}

// newToolResultJSON creates a new tool result with JSON content
func (t *CodeSearchTool) newToolResultJSON(data any) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// ProvideExtendedInfo implements the ExtendedHelpProvider interface
func (t *CodeSearchTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		WhenToUse:    "Use when you need to find code by meaning rather than exact name. Ideal for discovering relevant functions when you know what you want but not the name.",
		WhenNotToUse: "Don't use for exact name lookups (use grep/glob instead) or when the codebase hasn't been indexed yet.",
		CommonPatterns: []string{
			"Index codebase: {\"action\": \"index\", \"source\": [\"/path/to/project\"]}",
			"Search: {\"action\": \"search\", \"query\": \"function that handles authentication\"}",
			"Check status: {\"action\": \"status\"}",
			"Clear index: {\"action\": \"clear\"}",
		},
		ParameterDetails: map[string]string{
			"action":    "Required. One of: 'search', 'index', 'status', 'clear'",
			"source":    "Paths to index or filter. Required for 'index' action.",
			"query":     "Natural language query. Required for 'search' action.",
			"limit":     "Maximum results to return (default: 10).",
			"threshold": "Minimum similarity score 0-1 (default: 0.3).",
		},
		Examples: []tools.ToolExample{
			{
				Description: "Index a project",
				Arguments: map[string]any{
					"action": "index",
					"source": []string{"/path/to/project"},
				},
				ExpectedResult: "Returns count of indexed files and items",
			},
			{
				Description: "Search for authentication code",
				Arguments: map[string]any{
					"action": "search",
					"query":  "function that handles user authentication",
					"limit":  5,
				},
				ExpectedResult: "Returns matching functions with similarity scores",
			},
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Search returns no results",
				Solution: "Ensure the codebase has been indexed first with 'index' action",
			},
			{
				Problem:  "Model download fails",
				Solution: "Inform user that the model download failed and to check network connectivity.",
			},
		},
	}
}

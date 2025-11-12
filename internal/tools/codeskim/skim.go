package codeskim

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// CodeSkimTool implements the tools.Tool interface for code transformation
type CodeSkimTool struct{}

const (
	toolName        = "code_skim"
	defaultMaxLines = 10000 // Default: 10,000 lines
	envVarMaxLines  = "CODE_SKIM_MAX_LINES"
	maxWorkers      = 10                     // Maximum number of parallel workers
	maxMemoryBytes  = 4 * 1024 * 1024 * 1024 // 4GB maximum memory usage
	maxFileSize     = 500 * 1024             // 500KB maximum individual file size
)

// init registers the tool with the registry
func init() {
	registry.Register(&CodeSkimTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *CodeSkimTool) Definition() mcp.Tool {
	return mcp.NewTool(
		toolName,
		mcp.WithDescription("Returns information about source code in an efficient way by stripping function/method bodies whilst preserving signatures, types, and structure. Useful to understand large files or codebases without implementation details to save token usage."),
		mcp.WithArray("source",
			mcp.Required(),
			mcp.Description("Array of absolute paths to files, directories (recursively processes all supported files - use sparingly!), or glob patterns (e.g., [\"/path/file.py\", \"/dir\", \"**/*.go\"])."),
			mcp.WithStringItems(),
		),
		mcp.WithBoolean("clear_cache",
			mcp.Description("Force clear cache entries before processing"),
			mcp.DefaultBool(false),
		),
		mcp.WithNumber("starting_line",
			mcp.Description("Line number to start from (1-based) for pagination of large results. Use when previous response was truncated"),
		),
		mcp.WithArray("filter",
			mcp.Description("Optional array of glob patterns to filter by function/method/class name (e.g., [\"handle_*\", \"process_*\"]). Optionally prefix pattern with ! for exclusions. Returns matched_items, total_items, filtered_items counts."),
			mcp.WithStringItems(),
		),
		// Read-only annotations - reads files but doesn't modify them
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
}

// Execute executes the code skim tool
func (t *CodeSkimTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	logger.Info("Executing code_skim")
	startTime := time.Now()

	// Parse request
	req, err := t.parseRequest(args)
	if err != nil {
		return nil, err
	}

	// Resolve source to list of files
	files, err := ResolveFiles(req.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source: %w", err)
	}

	logger.WithField("files_count", len(files)).Debug("Resolved files to process")

	// Process files in parallel with worker pool
	results := t.processFilesParallel(ctx, files, req, cache, logger)

	// Count successes and failures
	processedCount := 0
	failedCount := 0
	for _, result := range results {
		if result.Error == "" {
			processedCount++
		} else {
			failedCount++
		}
	}

	// Build response
	processingTime := time.Since(startTime).Milliseconds()
	response := &SkimResponse{
		Files:            results,
		TotalFiles:       len(files),
		ProcessedFiles:   processedCount,
		FailedFiles:      failedCount,
		ProcessingTimeMs: &processingTime,
	}

	return t.newToolResultJSON(response)
}

// processFilesParallel processes multiple files in parallel using a worker pool
func (t *CodeSkimTool) processFilesParallel(ctx context.Context, files []string, req *SkimRequest, cache *sync.Map, logger *logrus.Logger) []FileResult {
	// If only one file, process directly without goroutines
	if len(files) == 1 {
		filePath := files[0]
		if err := security.CheckFileAccess(filePath); err != nil {
			return []FileResult{{
				Path:  filePath,
				Error: fmt.Sprintf("access denied: %v", err),
			}}
		}

		// Check file size
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return []FileResult{{
				Path:  filePath,
				Error: fmt.Sprintf("failed to stat file: %v", err),
			}}
		}

		if fileInfo.Size() > maxFileSize {
			return []FileResult{{
				Path:  filePath,
				Error: fmt.Sprintf("file too large: %d bytes (max: %d bytes / 500KB)", fileInfo.Size(), maxFileSize),
			}}
		}

		return []FileResult{t.processFile(ctx, filePath, req, cache, logger)}
	}

	// Determine worker count (min of files count and max workers)
	workerCount := min(len(files), maxWorkers)

	// Memory tracking with mutex for atomic check-and-allocate
	var memoryUsed sync.Map // map[int]int64 - track memory per file
	var memoryMutex sync.Mutex

	// Channels for work distribution
	type job struct {
		index int
		path  string
	}
	jobs := make(chan job, len(files))
	results := make([]FileResult, len(files))
	var wg sync.WaitGroup

	// Start workers (Go 1.25+ WaitGroup.Go pattern)
	for range workerCount {
		wg.Go(func() {
			for j := range jobs {
				// Security check for each file
				if err := security.CheckFileAccess(j.path); err != nil {
					results[j.index] = FileResult{
						Path:  j.path,
						Error: fmt.Sprintf("access denied: %v", err),
					}
					logger.WithFields(logrus.Fields{
						"file":  j.path,
						"error": err,
					}).Warn("Skipping file due to access denial")
					continue
				}

				// Check file size before processing
				fileInfo, err := os.Stat(j.path)
				if err != nil {
					results[j.index] = FileResult{
						Path:  j.path,
						Error: fmt.Sprintf("failed to stat file: %v", err),
					}
					continue
				}

				fileSize := fileInfo.Size()
				if fileSize > maxFileSize {
					results[j.index] = FileResult{
						Path:  j.path,
						Error: fmt.Sprintf("file too large: %d bytes (max: %d bytes / 500KB)", fileSize, maxFileSize),
					}
					logger.WithFields(logrus.Fields{
						"file": j.path,
						"size": fileSize,
					}).Warn("Skipping file - exceeds size limit")
					continue
				}

				// Check total memory budget (approximate: file size * 3 for source + transformed + overhead)
				estimatedMemory := fileSize * 3

				// Atomic check-and-allocate with mutex
				memoryMutex.Lock()
				var currentTotal int64
				memoryUsed.Range(func(key, value any) bool {
					if mem, ok := value.(int64); ok {
						currentTotal += mem
					}
					return true
				})

				// Check if adding this file would exceed limit
				if currentTotal+estimatedMemory > maxMemoryBytes {
					memoryMutex.Unlock()
					results[j.index] = FileResult{
						Path:  j.path,
						Error: fmt.Sprintf("memory limit exceeded: would use ~%d bytes (limit: %d bytes / 4GB)", currentTotal+estimatedMemory, maxMemoryBytes),
					}
					logger.WithFields(logrus.Fields{
						"file":          j.path,
						"current_total": currentTotal,
						"estimated":     estimatedMemory,
					}).Warn("Skipping file - would exceed memory limit")
					continue
				}

				// Store memory allocation
				memoryUsed.Store(j.index, estimatedMemory)
				memoryMutex.Unlock()

				// Process file
				results[j.index] = t.processFile(ctx, j.path, req, cache, logger)

				// Clean up memory tracking
				memoryUsed.Delete(j.index)
			}
		})
	}

	// Send jobs
	for i, filePath := range files {
		jobs <- job{index: i, path: filePath}
	}
	close(jobs)

	// Wait for completion
	wg.Wait()

	return results
}

// processFile processes a single file
func (t *CodeSkimTool) processFile(ctx context.Context, filePath string, req *SkimRequest, cache *sync.Map, logger *logrus.Logger) FileResult {
	result := FileResult{
		Path:      filePath,
		FromCache: false,
	}

	// Detect language
	detectedLang, err := DetectLanguage(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("language detection failed: %v", err)
		return result
	}
	lang := detectedLang
	isTSX := strings.HasSuffix(filePath, ".tsx")
	result.Language = lang

	// Get file modification time for preliminary cache check
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to stat file: %v", err)
		return result
	}
	mtime := fileInfo.ModTime().Unix()

	// Generate preliminary cache key (file path + mtime + language + filter)
	preliminaryCacheKey := t.generatePreliminaryCacheKey(filePath, mtime, lang, req.Filter)

	// Clear cache if requested
	if req.ClearCache && preliminaryCacheKey != "" {
		cache.Delete(preliminaryCacheKey)
		logger.WithField("cache_key", preliminaryCacheKey).Debug("Cleared cache entry")
	}

	// Check cache with preliminary key (avoids reading/hashing file)
	var transformResult *TransformResult
	if preliminaryCacheKey != "" {
		if cached, ok := cache.Load(preliminaryCacheKey); ok {
			if cachedResult, ok := cached.(*TransformResult); ok {
				transformResult = cachedResult
				result.FromCache = true
				logger.WithField("cache_key", preliminaryCacheKey).Debug("Using cached result")
			}
		}
	}

	// Read file only if not cached
	var source string
	if !result.FromCache {
		sourceBytes, err := os.ReadFile(filePath)
		if err != nil {
			result.Error = fmt.Sprintf("failed to read file: %v", err)
			return result
		}
		source = string(sourceBytes)
	}

	// Transform if not from cache
	if !result.FromCache {
		// req.Filter is []string or nil
		var filterPatterns []string
		if req.Filter != nil {
			if patterns, ok := req.Filter.([]string); ok {
				filterPatterns = patterns
			}
		}

		var err error
		transformResult, err = TransformWithFilter(ctx, source, lang, isTSX, filterPatterns)
		if err != nil {
			result.Error = fmt.Sprintf("transformation failed: %v", err)
			return result
		}

		// Store in cache using preliminary key
		if preliminaryCacheKey != "" {
			cache.Store(preliminaryCacheKey, transformResult)
			logger.WithField("cache_key", preliminaryCacheKey).Debug("Stored result in cache")
		}
	}

	// Extract transformed string
	transformed := transformResult.Transformed

	// Calculate reduction percentage
	// For cached results, we need to get the original size from file info
	var originalSize int
	if result.FromCache {
		originalSize = int(fileInfo.Size())
	} else {
		originalSize = len(source)
	}
	transformedSize := len(transformed)
	if originalSize > 0 {
		reduction := int(float64(originalSize-transformedSize) / float64(originalSize) * 100)
		// Clamp reduction percentage to [0, 100] range
		if reduction < 0 {
			reduction = 0
		} else if reduction > 100 {
			reduction = 100
		}
		result.ReductionPercentage = &reduction
	}

	// Add filter counts if filtering was applied
	if req.Filter != nil {
		result.MatchedItems = &transformResult.MatchedItems
		result.TotalItems = &transformResult.TotalItems
		result.FilteredItems = &transformResult.FilteredItems
	}

	// Apply line limiting and pagination
	lines := strings.Split(transformed, "\n")
	totalLines := len(lines)
	totalLinesPtr := &totalLines

	// Get starting line (1-based, defaults to 1)
	startLine := 1
	if req.StartingLine > 0 {
		startLine = req.StartingLine
	}

	// Get max lines from environment or use default
	maxLines := t.getMaxLines()

	// Apply pagination
	if startLine > totalLines {
		result.Error = fmt.Sprintf("starting_line %d exceeds total lines %d", startLine, totalLines)
		return result
	}

	startIdx := startLine - 1 // Convert to 0-based
	endIdx := startIdx + maxLines
	truncated := false

	if endIdx < totalLines {
		truncated = true
		lines = lines[startIdx:endIdx]
	} else {
		lines = lines[startIdx:]
	}

	result.Transformed = strings.Join(lines, "\n")
	result.TotalLines = totalLinesPtr
	returnedLines := len(lines)
	result.ReturnedLines = &returnedLines

	if truncated {
		result.Truncated = true
		nextLine := startLine + returnedLines
		result.NextStartingLine = &nextLine
	}

	return result
}

// parseRequest parses and validates the tool arguments
func (t *CodeSkimTool) parseRequest(args map[string]any) (*SkimRequest, error) {
	req := &SkimRequest{}

	// Parse source (required) - array of strings
	sourceRaw, ok := args["source"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter 'source': provide an array of file paths, directory paths, or glob patterns")
	}

	// Must be an array
	sourceArray, ok := sourceRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("source must be an array of strings (e.g., [\"/path/to/file.py\"])")
	}

	if len(sourceArray) == 0 {
		return nil, fmt.Errorf("source array cannot be empty")
	}

	// Parse array items
	var sources []string
	for i, item := range sourceArray {
		str, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("source array item %d must be a string", i)
		}
		if str == "" {
			return nil, fmt.Errorf("source array item %d cannot be empty", i)
		}
		sources = append(sources, str)
	}

	// Convert all sources to absolute paths
	for i, source := range sources {
		if !filepath.IsAbs(source) {
			absPath, err := filepath.Abs(source)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve absolute path for source %d: %w", i, err)
			}
			sources[i] = absPath
		}
	}

	// Always store as array (converted to []any for interface{})
	sourceInterfaces := make([]any, len(sources))
	for i, s := range sources {
		sourceInterfaces[i] = s
	}
	req.Source = sourceInterfaces

	// Parse clear_cache (optional)
	if clearRaw, ok := args["clear_cache"]; ok {
		if clearBool, ok := clearRaw.(bool); ok {
			req.ClearCache = clearBool
		}
	}

	// Parse starting_line (optional)
	if startRaw, ok := args["starting_line"]; ok {
		switch v := startRaw.(type) {
		case float64:
			req.StartingLine = int(v)
		case int:
			req.StartingLine = v
		}
	}

	// Parse filter (optional) - array of strings
	if filterRaw, ok := args["filter"]; ok {
		filterArray, ok := filterRaw.([]any)
		if !ok {
			return nil, fmt.Errorf("filter must be an array of strings (e.g., [\"handle_*\", \"!temp_*\"])")
		}

		if len(filterArray) > 0 {
			var filters []string
			for i, item := range filterArray {
				str, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("filter array item %d must be a string", i)
				}
				if str != "" {
					filters = append(filters, str)
				}
			}
			if len(filters) > 0 {
				// Store as []string
				req.Filter = filters
			}
		}
	}

	return req, nil
}

// getMaxLines returns the maximum number of lines to return, from env or default
func (t *CodeSkimTool) getMaxLines() int {
	if maxLinesStr := os.Getenv(envVarMaxLines); maxLinesStr != "" {
		if maxLines, err := strconv.Atoi(maxLinesStr); err == nil && maxLines > 0 {
			return maxLines
		}
	}
	return defaultMaxLines
}

// generatePreliminaryCacheKey generates a lightweight cache key using mtime instead of hashing
// This avoids reading and hashing the entire file content on cache hits
func (t *CodeSkimTool) generatePreliminaryCacheKey(filePath string, mtime int64, lang Language, filter any) string {
	// Use file path + mtime + language + filter
	// mtime changes when file is modified, invalidating the cache

	// Generate filter string for cache key (filter is []string or nil)
	var filterStr string
	if filter != nil {
		if filterSlice, ok := filter.([]string); ok && len(filterSlice) > 0 {
			filterStr = strings.Join(filterSlice, ",")
		}
	}

	if filterStr != "" {
		return fmt.Sprintf("codeskim:%s:%d:%s:%s", filePath, mtime, lang, filterStr)
	}
	return fmt.Sprintf("codeskim:%s:%d:%s", filePath, mtime, lang)
}

// newToolResultJSON creates a new tool result with JSON content
func (t *CodeSkimTool) newToolResultJSON(data any) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// ProvideExtendedInfo implements the ExtendedHelpProvider interface
func (t *CodeSkimTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		WhenToUse:    "Use when you need to analyse code structure without implementation details, when fitting large codebases into limited AI context windows, when providing architectural overviews, or when examining API surfaces and function signatures. Ideal for understanding 'what' code does without the 'how' details. Subject to memory limits (4GB total, 500KB per file).",
		WhenNotToUse: "Don't use when you need to debug implementation logic, when examining algorithm details matters, when reviewing line-by-line code quality, when the actual implementation is required for the task, or when files exceed 500KB.",
		CommonPatterns: []string{
			"Process single file: {\"source\": [\"/path/to/file.py\"]}",
			"Process directory: {\"source\": [\"/path/to/project\"]}",
			"Use glob pattern: {\"source\": [\"/path/to/project/**/*.ts\"]}",
			"Multiple sources: {\"source\": [\"/file1.py\", \"/file2.go\", \"/dir\"]}",
			"With filter: {\"source\": [\"/api.py\"], \"filter\": [\"handle_*\"]}",
			"Clear cache: {\"source\": [\"/file.py\"], \"clear_cache\": true}",
			"Paginate: {\"source\": [\"/large.py\"], \"starting_line\": 10001}",
		},
		ParameterDetails: map[string]string{
			"source":        "Array of absolute paths to files, directories, or glob patterns. Directories are processed recursively. Globs support ** for recursive matching. Multiple sources are deduplicated.",
			"filter":        "Optional array of glob patterns to filter by function/method/class name. Prefix with ! for exclusion. Returns matched_items, total_items, filtered_items counts.",
			"clear_cache":   "When true, clears cache entries before processing.",
			"starting_line": "Line number to start from (1-based) for pagination. Use when previous response was truncated.",
		},
		Examples: []tools.ToolExample{
			{
				Description: "Process a single Python file",
				Arguments: map[string]any{
					"source": []string{"/Users/samm/project/app.py"},
				},
				ExpectedResult: "Returns array with one file result containing transformed code",
			},
			{
				Description: "Process all TypeScript files in a directory",
				Arguments: map[string]any{
					"source": []string{"/Users/samm/project/src"},
				},
				ExpectedResult: "Returns array of results for all .ts and .tsx files found recursively",
			},
			{
				Description: "Process files matching a glob pattern",
				Arguments: map[string]any{
					"source": []string{"/Users/samm/project/**/*.go"},
				},
				ExpectedResult: "Returns array of results for all .go files matching the pattern",
			},
			{
				Description: "Filter to show only handle functions",
				Arguments: map[string]any{
					"source": []string{"/Users/samm/project/api.py"},
					"filter": []string{"handle_*"},
				},
				ExpectedResult: "Returns transformed code with only functions matching 'handle_*', includes matched_items count",
			},
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Source not found error",
				Solution: "Ensure the path is absolute, exists, and is accessible. Use absolute paths, not relative.",
			},
			{
				Problem:  "No supported files found",
				Solution: "Check that the directory or glob pattern contains files with supported extensions (.py, .go, .js, .jsx, .ts, .tsx)",
			},
			{
				Problem:  "Access denied error",
				Solution: "Verify file permissions and that the security system allows access to the requested paths",
			},
			{
				Problem:  "Some files show errors in results",
				Solution: "Check the 'error' field in individual file results. Files with errors are counted separately in failed_files",
			},
			{
				Problem:  "File too large error",
				Solution: "Individual files cannot exceed 500KB. Consider splitting large files or processing smaller subsets",
			},
			{
				Problem:  "Memory limit exceeded error",
				Solution: "Total memory usage limited to 4GB. Process fewer files at once or use glob patterns to target specific subsets",
			},
		},
	}
}

package codeskim

import (
	"context"
	"crypto/sha256"
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
)

// init registers the tool with the registry
func init() {
	registry.Register(&CodeSkimTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *CodeSkimTool) Definition() mcp.Tool {
	return mcp.NewTool(
		toolName,
		mcp.WithDescription("Returns information about source code in an efficient way by stripping function/method bodies whilst preserving signatures, types, and structure. Use when you need to analyse or summarise large files or codebases before reading an entire file."),
		mcp.WithString("source",
			mcp.Required(),
			mcp.Description("Absolute path to: a file, a directory (recursively processes all supported files), or a glob pattern (e.g., '**/*.py')"),
		),
		mcp.WithBoolean("clear_cache",
			mcp.Description("Clear cache entries before processing"),
			mcp.DefaultBool(false),
		),
		mcp.WithNumber("starting_line",
			mcp.Description("Line number to start from (1-based) for pagination of large results. Use when previous response was truncated."),
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

	// Security check: verify access to all files
	for _, filePath := range files {
		if err := security.CheckFileAccess(filePath); err != nil {
			return nil, fmt.Errorf("access denied to %s: %w", filePath, err)
		}
	}

	// Process files
	var results []FileResult
	processedCount := 0
	failedCount := 0

	for _, filePath := range files {
		result := t.processFile(ctx, filePath, req, cache, logger)
		results = append(results, result)

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

	// Read file
	sourceBytes, err := os.ReadFile(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read file: %v", err)
		return result
	}
	source := string(sourceBytes)

	// Generate cache key
	cacheKey := t.generateCacheKey(filePath, source, lang)

	// Clear cache if requested
	if req.ClearCache && cacheKey != "" {
		cache.Delete(cacheKey)
		logger.WithField("cache_key", cacheKey).Debug("Cleared cache entry")
	}

	// Check cache
	var transformed string
	if cacheKey != "" {
		if cached, ok := cache.Load(cacheKey); ok {
			if cachedResult, ok := cached.(string); ok {
				transformed = cachedResult
				result.FromCache = true
				logger.WithField("cache_key", cacheKey).Debug("Using cached result")
			}
		}
	}

	// Transform if not from cache
	if !result.FromCache {
		transformed, err = Transform(ctx, source, lang, isTSX)
		if err != nil {
			result.Error = fmt.Sprintf("transformation failed: %v", err)
			return result
		}

		// Store in cache
		if cacheKey != "" {
			cache.Store(cacheKey, transformed)
			logger.WithField("cache_key", cacheKey).Debug("Stored result in cache")
		}
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

	// Parse source (required)
	source, ok := args["source"].(string)
	if !ok || source == "" {
		return nil, fmt.Errorf("missing required parameter 'source': provide a file path, directory path, or glob pattern")
	}

	// Convert to absolute path if relative
	if !filepath.IsAbs(source) {
		absPath, err := filepath.Abs(source)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		source = absPath
	}

	req.Source = source

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

// generateCacheKey generates a cache key for a file
func (t *CodeSkimTool) generateCacheKey(filePath string, source string, lang Language) string {
	// Use file path + language + source hash
	hash := sha256.Sum256([]byte(source))
	return fmt.Sprintf("codeskim:%s:%s:%x", filePath, lang, hash[:8])
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
		WhenToUse:    "Use when you need to analyse code structure without implementation details, when fitting large codebases into limited AI context windows, when providing architectural overviews, or when examining API surfaces and function signatures. Ideal for understanding 'what' code does without the 'how' details.",
		WhenNotToUse: "Don't use when you need to debug implementation logic, when examining algorithm details matters, when reviewing line-by-line code quality, or when the actual implementation is required for the task.",
		CommonPatterns: []string{
			"Process single file: {\"source\": \"/path/to/file.py\"}",
			"Process directory: {\"source\": \"/path/to/project\"}",
			"Use glob pattern: {\"source\": \"/path/to/project/**/*.ts\"}",
			"Clear cache: {\"source\": \"file.py\", \"clear_cache\": true}",
			"Paginate large file: {\"source\": \"large.py\", \"starting_line\": 10001}",
		},
		ParameterDetails: map[string]string{
			"source":        "Absolute path to a file, directory, or glob pattern. Directories are processed recursively. Globs support ** for recursive matching.",
			"clear_cache":   "When true, clears cache entries before processing.",
			"starting_line": "Line number to start from (1-based) for pagination. Use when previous response was truncated.",
		},
		Examples: []tools.ToolExample{
			{
				Description: "Process a single Python file",
				Arguments: map[string]any{
					"source": "/Users/samm/project/app.py",
				},
				ExpectedResult: "Returns array with one file result containing transformed code",
			},
			{
				Description: "Process all TypeScript files in a directory",
				Arguments: map[string]any{
					"source": "/Users/samm/project/src",
				},
				ExpectedResult: "Returns array of results for all .ts and .tsx files found recursively",
			},
			{
				Description: "Process files matching a glob pattern",
				Arguments: map[string]any{
					"source": "/Users/samm/project/**/*.go",
				},
				ExpectedResult: "Returns array of results for all .go files matching the pattern",
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
		},
	}
}

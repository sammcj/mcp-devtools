package docprocessing

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// DocumentProcessorTool implements document processing using Docling via Python subprocess
type DocumentProcessorTool struct {
	config       *Config
	cacheManager *CacheManager
}

// init registers the document processor tool only if docling is available
func init() {
	config := LoadConfig()

	// Only register the tool if docling is available
	if config.PythonPath != "" && config.isDoclingAvailable() {
		// Rotate debug logs on startup (keeps logs from past 48 hours)
		rotateDebugLogs()

		cacheManager := NewCacheManager(config)
		registry.Register(&DocumentProcessorTool{
			config:       config,
			cacheManager: cacheManager,
		})
	}
	// Note: We don't log here as this runs during init and could interfere with MCP protocol
	// The tool will simply not be available if docling is not found
}

// Definition returns the MCP tool definition
func (t *DocumentProcessorTool) Definition() mcp.Tool {
	// Build profile description dynamically based on available features
	profileDesc := "Processing profile: 'text-and-image' (text and images, default), 'basic' (text only), 'scanned' (OCR), 'llm-smoldocling' (SmolDocling vision)"

	// Only include llm-external if LLM environment variables are configured
	if IsLLMConfigured() {
		profileDesc += ", 'llm-external' (external LLM for chart / diagram conversion to mermaid)"
	}

	return mcp.NewTool(
		"process_document",
		mcp.WithDescription("Process documents (PDF, DOCX, XLSX, PPTX, HTML, CSV, PNG, JPG) and convert them to structured Markdown with optional OCR, image extraction, and table processing. Supports hardware acceleration, intelligent caching, and batch processing."),
		mcp.WithString("source",
			mcp.Description("Document source: MUST be a fully qualified absolute file path (e.g., /Users/user/documents/file.pdf), complete URL (e.g., https://example.com/doc.pdf). Relative paths are NOT supported - always provide the complete absolute path to the file. For batch processing, use 'sources' instead."),
		),
		mcp.WithArray("sources",
			mcp.Description("Multiple document sources for batch processing: Array of fully qualified absolute file paths or URLs. When provided, 'source' parameter is ignored."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithString("profile",
			mcp.Description(profileDesc),
		),
		mcp.WithBoolean("return_inline_only",
			mcp.Description("Optionally return content inline only. When false (default), the tool will save the processed content to a file in the same directory as the source file which is usually desired."),
		),
		mcp.WithString("save_to",
			mcp.Description("Override the file path for saved content (default: same directory as source file). MUST be a fully qualified absolute path"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Processing timeout in seconds (overrides default)"),
		),
		mcp.WithBoolean("clear_file_cache",
			mcp.Description("Force clear all cache entries the source file before processing"),
		),
		mcp.WithBoolean("debug",
			mcp.Description("Return debug information including environment variables (secrets masked)"),
		),
	)
}

// Execute processes the document using the Python wrapper
func (t *DocumentProcessorTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Note: No logging to stdout/stderr in stdio mode to avoid breaking MCP protocol

	// Perform cache maintenance in background (6 weeks default, configurable)
	go func() {
		maxAge := 6 * 7 * 24 * time.Hour // 6 weeks default
		if maxAgeEnv := os.Getenv("DOCLING_CACHE_MAX_AGE_HOURS"); maxAgeEnv != "" {
			if hours, err := strconv.Atoi(maxAgeEnv); err == nil && hours > 0 {
				maxAge = time.Duration(hours) * time.Hour
			}
		}

		// Silently perform maintenance - no logging to avoid MCP protocol interference
		_ = t.cacheManager.PerformMaintenance(maxAge)
	}()

	// Check for batch processing (sources array)
	if sources, ok := args["sources"].([]interface{}); ok && len(sources) > 0 {
		return t.executeBatch(ctx, args, sources)
	}

	// Parse and validate arguments for single document
	req, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Handle debug mode - return debug information without processing
	if req.Debug {
		debugInfo := t.getDebugInfo()
		return t.newToolResultJSON(debugInfo)
	}

	// Validate configuration
	if err := t.config.Validate(); err != nil {
		errorResult := map[string]interface{}{
			"error":       fmt.Sprintf("Configuration error: %s", err.Error()),
			"system_info": t.config.GetSystemInfo(),
		}
		return t.newToolResultJSON(errorResult)
	}

	// Clear file cache if requested
	if req.ClearFileCache {
		// Clear cache for this source file, ignore errors to avoid failing the request
		_ = t.cacheManager.ClearFileCache(req.Source)
	}

	// Check cache first
	cacheEnabled := t.shouldUseCache(req)
	var cacheKey string
	if cacheEnabled {
		cacheKey = t.cacheManager.GenerateCacheKey(req)
		if cached, found := t.cacheManager.Get(cacheKey); found {
			// Handle file saving for cached results
			if t.shouldSaveToFile(req) && cached.Error == "" {
				return t.handleSaveToFile(req.SaveTo, cached)
			}
			return t.newToolResultJSON(t.formatResponse(cached))
		}
	}

	// Process document
	response, err := t.processDocument(req)
	if err != nil {
		errorResult := map[string]interface{}{
			"error": err.Error(),
		}
		return t.newToolResultJSON(errorResult)
	}

	// Cache result if successful
	if cacheEnabled && response.Error == "" {
		// Cache the result but don't include cache key in response
		_ = t.cacheManager.Set(cacheKey, response)
	}

	// Handle file saving if specified
	if t.shouldSaveToFile(req) && response.Error == "" {
		return t.handleSaveToFile(req.SaveTo, response)
	}

	return t.newToolResultJSON(t.formatResponse(response))
}

// shouldUseCache determines if caching should be used for this request
func (t *DocumentProcessorTool) shouldUseCache(req *DocumentProcessingRequest) bool {
	// Always use cache since we removed the cache_enabled parameter
	return t.config.CacheEnabled
}

package docprocessing

import (
	"context"
	"fmt"
	"os"
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

	tool := mcp.NewTool(
		"process_document",
		mcp.WithDescription("Process documents (PDF, DOCX, DOC, XLSX, XLS, PPTX, PPT, TXT, MD, RTF, HTML, CSV, PNG, JPG, JPEG, GIF, BMP, TIFF) and convert them to structured Markdown with optional OCR, image extraction, and table processing. Supports hardware acceleration, intelligent caching, and batch processing."),
		mcp.WithString("source",
			mcp.Description("Document source: MUST be a fully qualified absolute file path (e.g., /Users/user/documents/file.pdf), complete URL (e.g., https://example.com/doc.pdf). Relative paths are NOT supported - always provide the complete absolute path to the file. For batch processing, use 'sources' instead."),
		),
		mcp.WithArray("sources",
			mcp.Description("Multiple document sources for batch processing: Array of fully qualified absolute file paths or URLs. When provided, 'source' parameter is ignored."),
			mcp.WithStringItems(),
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

		// Non-destructive writing annotations
		mcp.WithReadOnlyHintAnnotation(false),    // Converts documents to new formats
		mcp.WithDestructiveHintAnnotation(false), // Doesn't destroy source documents
		mcp.WithIdempotentHintAnnotation(false),  // Creates new content each run
		mcp.WithOpenWorldHintAnnotation(true),    // May fetch from external URLs
	)
	return tool
}

// Execute processes the document using the Python wrapper
func (t *DocumentProcessorTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Check if process_document tool is enabled (disabled by default)
	if !tools.IsToolEnabled("process_document") {
		return nil, fmt.Errorf("process_document tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'process_document'")
	}

	// Note: No logging to stdout/stderr in stdio mode to avoid breaking MCP protocol

	// Perform cache maintenance and temporary file cleanup in background
	go func() {
		maxAge := 6 * 7 * 24 * time.Hour // 6 weeks default
		if maxAgeEnv := os.Getenv("DOCLING_CACHE_MAX_AGE_HOURS"); maxAgeEnv != "" {
			if hours, err := strconv.Atoi(maxAgeEnv); err == nil && hours > 0 {
				maxAge = time.Duration(hours) * time.Hour
			}
		}

		// Silently perform maintenance - no logging to avoid MCP protocol interference
		_ = t.cacheManager.PerformMaintenance(maxAge)

		// Perform temporary file cleanup
		_ = t.config.CleanupTemporaryFiles()
	}()

	// Check for batch processing (sources array)
	if sources, ok := args["sources"].([]any); ok && len(sources) > 0 {
		return t.executeBatch(ctx, args, sources)
	}

	// Parse and validate arguments for single document
	req, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Security: Check file access control for local file paths
	if !strings.HasPrefix(req.Source, "http://") && !strings.HasPrefix(req.Source, "https://") {
		// Check security system file access control first
		if err := security.CheckFileAccess(req.Source); err != nil {
			return nil, err
		}

		// Validate file type
		if err := t.config.ValidateFileType(req.Source); err != nil {
			return nil, fmt.Errorf("file type validation failed: %w", err)
		}

		// Validate file size if file exists
		if fileInfo, err := os.Stat(req.Source); err == nil {
			if err := t.config.ValidateFileSize(fileInfo.Size()); err != nil {
				return nil, fmt.Errorf("file size validation failed: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("file access error: %w", err)
		}
		// Note: If file doesn't exist, let the processing handle it (it might be a URL or other valid source)
	}

	// Handle debug mode - return debug information without processing
	if req.Debug {
		debugInfo := t.getDebugInfo()
		return t.newToolResultJSON(debugInfo)
	}

	// Validate configuration
	if err := t.config.Validate(); err != nil {
		errorResult := map[string]any{
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
				return t.handleSaveToFile(req.SaveTo, cached, "")
			}
			return t.newToolResultJSON(t.formatResponse(cached))
		}
	}

	// Process document
	response, err := t.processDocument(req)
	if err != nil {
		errorResult := map[string]any{
			"error": err.Error(),
		}
		return t.newToolResultJSON(errorResult)
	}

	// Cache result if successful
	if cacheEnabled && response.Error == "" {
		// Cache the result but don't include cache key in response
		_ = t.cacheManager.Set(cacheKey, response)
	}

	// Security: Analyse processed content
	var securityNotice string
	if security.IsEnabled() && response.Error == "" {
		sourceContext := security.SourceContext{
			Tool: "document_processing",
			URL:  req.Source,
		}

		result, err := security.AnalyseContent(response.Content, sourceContext)
		if err != nil {
			logrus.WithError(err).Warn("Security analysis failed")
		} else {
			switch result.Action {
			case security.ActionBlock:
				return nil, fmt.Errorf("content blocked by security policy: %s", result.Message)
			case security.ActionWarn:
				securityNotice = fmt.Sprintf("Security Warning [ID: %s]: %s", result.ID, result.Message)
				logrus.WithField("security_id", result.ID).Warn(result.Message)
			}
		}
	}

	// Handle file saving if specified
	if t.shouldSaveToFile(req) && response.Error == "" {
		return t.handleSaveToFile(req.SaveTo, response, securityNotice)
	}

	// Add security notice to response if needed
	formattedResponse := t.formatResponse(response)
	if securityNotice != "" {
		formattedResponse["security_notice"] = securityNotice
	}

	return t.newToolResultJSON(formattedResponse)
}

// shouldUseCache determines if caching should be used for this request
func (t *DocumentProcessorTool) shouldUseCache(req *DocumentProcessingRequest) bool {
	// Always use cache since we removed the cache_enabled parameter
	return t.config.CacheEnabled
}

// ProvideExtendedInfo provides detailed usage information for the document processing tool
func (t *DocumentProcessorTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Process a PDF with text and images",
				Arguments: map[string]any{
					"source":  "/Users/username/documents/report.pdf",
					"profile": "text-and-image",
				},
				ExpectedResult: "Extracts text and images, saves markdown file to same directory, returns structured content with metadata and processing statistics",
			},
			{
				Description: "Batch process multiple documents",
				Arguments: map[string]any{
					"sources": []string{
						"/Users/username/docs/file1.pdf",
						"/Users/username/docs/file2.docx",
						"/Users/username/docs/file3.xlsx",
					},
					"profile": "basic",
				},
				ExpectedResult: "Processes all documents in batch, returns array of results with individual success/failure status and content for each file",
			},
			{
				Description: "Process scanned document with OCR",
				Arguments: map[string]any{
					"source":  "/Users/username/scanned/invoice.pdf",
					"profile": "scanned",
				},
				ExpectedResult: "Applies OCR to extract text from scanned/image-based PDF, may take longer but handles non-text documents",
			},
			{
				Description: "Return content inline without saving file",
				Arguments: map[string]any{
					"source":             "/Users/username/docs/contract.docx",
					"profile":            "text-and-image",
					"return_inline_only": true,
				},
				ExpectedResult: "Processes document and returns content in response only, does not save to file system",
			},
			{
				Description: "Force cache refresh and save to custom location",
				Arguments: map[string]any{
					"source":           "/Users/username/docs/presentation.pptx",
					"profile":          "text-and-image",
					"clear_file_cache": true,
					"save_to":          "/Users/username/output/presentation.md",
				},
				ExpectedResult: "Clears any cached results, reprocesses document, and saves output to specified location",
			},
		},
		CommonPatterns: []string{
			"Use 'text-and-image' profile (default) for comprehensive document processing including visual elements",
			"Use 'basic' profile for faster text-only extraction when images are not needed",
			"Use 'scanned' profile specifically for PDFs that contain scanned images or poor-quality text",
			"Use batch processing with 'sources' array for multiple files to improve efficiency",
			"Set 'clear_file_cache: true' when document content has changed but filename is the same",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Relative paths are NOT supported error",
				Solution: "Always use fully qualified absolute paths starting with / (Unix) or drive letter (Windows). Convert relative paths like './file.pdf' to absolute paths like '/Users/username/project/file.pdf'.",
			},
			{
				Problem:  "Configuration error: docling not available",
				Solution: "The document processing tool requires docling Python package. Install with: pip install docling. Ensure Python is in your PATH and docling is importable.",
			},
			{
				Problem:  "File size exceeds maximum allowed size",
				Solution: "Large files are rejected for security. Increase limits with environment variables DOC_PROCESSOR_MAX_FILE_SIZE or split large documents into smaller files.",
			},
			{
				Problem:  "Unsupported file type error",
				Solution: "Check that file extension is supported: PDF, DOCX, DOC, XLSX, XLS, PPTX, PPT, TXT, MD, RTF, HTML, CSV, PNG, JPG, JPEG, GIF, BMP, TIFF. Verify file is not corrupted.",
			},
			{
				Problem:  "Processing timeout or hanging",
				Solution: "Complex documents may take time. Increase timeout parameter or use 'basic' profile for faster processing. Check that docling environment is properly configured.",
			},
			{
				Problem:  "Poor OCR results or missing text",
				Solution: "For scanned documents, ensure you use 'scanned' profile. Some documents may have complex layouts that are difficult to process. Try different profiles or preprocessing the document.",
			},
		},
		ParameterDetails: map[string]string{
			"source":             "Single document source (required unless using sources). MUST be absolute path (e.g., /Users/user/file.pdf) or complete URL. Supports PDF, Office documents, images, and text files.",
			"sources":            "Array of document sources for batch processing. When provided, 'source' is ignored. Each item must be an absolute path or URL. Batch processing is more efficient for multiple files.",
			"profile":            "Processing profile affects quality vs speed: 'text-and-image' (comprehensive, default), 'basic' (text-only, fast), 'scanned' (OCR for images), 'llm-smoldocling' (vision model), 'llm-external' (if configured).",
			"return_inline_only": "When true, returns content only in response without saving to file. When false (default), saves processed markdown to file system and returns file path.",
			"save_to":            "Override output file location (absolute path required). By default, saves to same directory as source with .md extension. Useful for organising output or preventing overwrites.",
			"clear_file_cache":   "Forces reprocessing by clearing cached results for the source file. Use when document content changed but filename is the same, or when troubleshooting cache issues.",
			"timeout":            "Processing timeout in seconds. Override default timeouts for complex documents. Larger documents or OCR processing may need longer timeouts.",
			"debug":              "Returns environment and configuration information without processing. Useful for troubleshooting setup issues or verifying tool configuration.",
		},
		WhenToUse:    "Use for extracting structured content from documents, converting documents to markdown, batch processing document collections, OCR on scanned documents, or preparing documents for analysis workflows.",
		WhenNotToUse: "Don't use for simple text files that don't need processing, password-protected documents, or when you need to preserve exact formatting. Not suitable for real-time processing due to potential processing overhead.",
	}
}

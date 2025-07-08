package docprocessing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	return mcp.NewTool(
		"process_document",
		mcp.WithDescription("Process documents (PDF, DOCX, XLSX, PPTX) and convert them to structured Markdown with optional OCR, image extraction, and table processing. Supports hardware acceleration and intelligent caching."),
		mcp.WithString("source",
			mcp.Required(),
			mcp.Description("Document source: MUST be a fully qualified absolute file path (e.g., /Users/user/documents/file.pdf), complete URL (e.g., https://example.com/doc.pdf), or base64-encoded content. Relative paths are NOT supported - always provide the complete absolute path to the file."),
		),
		mcp.WithString("processing_mode",
			mcp.Description("Processing mode: basic (fast), advanced (vision model), ocr (scanned docs), tables (table focus), images (image focus)"),
			mcp.DefaultString("basic"),
		),
		mcp.WithString("output_format",
			mcp.Description("Output format: markdown, json (metadata only), or both"),
			mcp.DefaultString("markdown"),
		),
		mcp.WithBoolean("enable_ocr",
			mcp.Description("Enable OCR processing with a recognition model for scanned documents"),
		),
		mcp.WithBoolean("preserve_images",
			mcp.Description("Extract and preserve images from the document"),
		),
		mcp.WithBoolean("cache_enabled",
			mcp.Description("Override global cache setting for this request"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Processing timeout in seconds (overrides default)"),
		),
		mcp.WithNumber("max_file_size",
			mcp.Description("Maximum file size in MB (overrides default)"),
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

	// Parse and validate arguments
	req, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Validate configuration
	if err := t.config.Validate(); err != nil {
		errorResult := map[string]interface{}{
			"error":       fmt.Sprintf("Configuration error: %s", err.Error()),
			"system_info": t.config.GetSystemInfo(),
		}
		return t.newToolResultJSON(errorResult)
	}

	// Check cache first
	cacheEnabled := t.shouldUseCache(req)
	var cacheKey string
	if cacheEnabled {
		cacheKey = t.cacheManager.GenerateCacheKey(req)
		if cached, found := t.cacheManager.Get(cacheKey); found {
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
		if err := t.cacheManager.Set(cacheKey, response); err != nil {
			// Log cache error but don't fail the request
			response.ProcessingInfo.CacheKey = fmt.Sprintf("cache_error: %s", err.Error())
		} else {
			response.ProcessingInfo.CacheKey = cacheKey
		}
	}

	return t.newToolResultJSON(t.formatResponse(response))
}

// parseRequest parses and validates the request arguments
func (t *DocumentProcessorTool) parseRequest(args map[string]interface{}) (*DocumentProcessingRequest, error) {
	req := &DocumentProcessingRequest{}

	// Required: source
	if source, ok := args["source"].(string); ok {
		req.Source = strings.TrimSpace(source)
		if req.Source == "" {
			return nil, fmt.Errorf("source cannot be empty")
		}
	} else {
		return nil, fmt.Errorf("source is required")
	}

	// Optional: processing_mode
	if mode, ok := args["processing_mode"].(string); ok {
		req.ProcessingMode = ProcessingMode(mode)
	} else {
		req.ProcessingMode = ProcessingModeBasic
	}

	// Optional: output_format
	if format, ok := args["output_format"].(string); ok {
		req.OutputFormat = OutputFormat(format)
	} else {
		req.OutputFormat = OutputFormatMarkdown
	}

	// Optional: enable_ocr
	if ocr, ok := args["enable_ocr"].(bool); ok {
		req.EnableOCR = ocr
	}

	// Optional: ocr_languages
	if langs, ok := args["ocr_languages"].([]interface{}); ok {
		for _, lang := range langs {
			if langStr, ok := lang.(string); ok {
				req.OCRLanguages = append(req.OCRLanguages, langStr)
			}
		}
	}
	if len(req.OCRLanguages) == 0 {
		req.OCRLanguages = []string{"en"}
	}

	// Optional: preserve_images
	if images, ok := args["preserve_images"].(bool); ok {
		req.PreserveImages = images
	}

	// Optional: cache_enabled
	if cache, ok := args["cache_enabled"].(bool); ok {
		req.CacheEnabled = &cache
	}

	// Optional: timeout
	if timeout, ok := args["timeout"].(float64); ok {
		timeoutInt := int(timeout)
		req.Timeout = &timeoutInt
	}

	// Optional: max_file_size
	if maxSize, ok := args["max_file_size"].(float64); ok {
		maxSizeInt := int(maxSize)
		req.MaxFileSize = &maxSizeInt
	}

	return req, nil
}

// shouldUseCache determines if caching should be used for this request
func (t *DocumentProcessorTool) shouldUseCache(req *DocumentProcessingRequest) bool {
	if req.CacheEnabled != nil {
		return *req.CacheEnabled
	}
	return t.config.CacheEnabled
}

// processDocument processes the document using the Python wrapper
func (t *DocumentProcessorTool) processDocument(req *DocumentProcessingRequest) (*DocumentProcessingResponse, error) {
	// Resolve source path to absolute path
	sourcePath, err := t.resolveSourcePath(req.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source path: %w", err)
	}

	// Get and validate script path
	scriptPath := t.config.GetScriptPath()
	if _, err := os.Stat(scriptPath); err != nil {
		return nil, fmt.Errorf("python script not found at %s: %w", scriptPath, err)
	}

	// Build command arguments
	args := []string{
		scriptPath,
		"process",
		sourcePath,
		"--processing-mode", string(req.ProcessingMode),
		"--output-format", string(req.OutputFormat),
	}

	if req.EnableOCR {
		args = append(args, "--enable-ocr")
		if len(req.OCRLanguages) > 0 {
			args = append(args, "--ocr-languages")
			args = append(args, req.OCRLanguages...)
		}
	}

	if req.PreserveImages {
		args = append(args, "--preserve-images")
	}

	// Determine timeout
	timeout := t.config.Timeout
	if req.Timeout != nil {
		timeout = *req.Timeout
	}

	// Execute Python script
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.config.PythonPath, args...)

	// Set working directory to the project root so relative paths work
	if cwd, err := os.Getwd(); err == nil {
		cmd.Dir = cwd
	}

	// Only capture stdout to avoid mixing with stderr logs
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("processing timeout after %d seconds", timeout)
		}
		// Get stderr for better error reporting
		if exitError, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("python script failed: %w, stderr: %s", err, string(exitError.Stderr))
		}
		return nil, fmt.Errorf("python script failed: %w", err)
	}

	// Parse Python script output
	var pythonResult map[string]interface{}
	if err := json.Unmarshal(output, &pythonResult); err != nil {
		return nil, fmt.Errorf("failed to parse python output: %w, raw output: %s", err, string(output))
	}

	// Check if processing was successful
	success, ok := pythonResult["success"].(bool)
	if !ok || !success {
		errorMsg := "unknown error"
		if errStr, ok := pythonResult["error"].(string); ok {
			errorMsg = errStr
		}
		return &DocumentProcessingResponse{
			Source: req.Source,
			Error:  errorMsg,
		}, nil
	}

	// Build response
	response := &DocumentProcessingResponse{
		Source:   req.Source,
		CacheHit: false,
	}

	// Extract content
	if content, ok := pythonResult["content"].(string); ok {
		response.Content = content
	}

	// Extract metadata
	if metaData, ok := pythonResult["metadata"].(map[string]interface{}); ok {
		response.Metadata = t.parseMetadata(metaData)
	}

	// Extract processing info
	if procInfo, ok := pythonResult["processing_info"].(map[string]interface{}); ok {
		response.ProcessingInfo = t.parseProcessingInfo(procInfo)
	}

	return response, nil
}

// parseMetadata converts the Python metadata to Go struct
func (t *DocumentProcessorTool) parseMetadata(data map[string]interface{}) *DocumentMetadata {
	metadata := &DocumentMetadata{}

	if title, ok := data["title"].(string); ok {
		metadata.Title = title
	}
	if author, ok := data["author"].(string); ok {
		metadata.Author = author
	}
	if subject, ok := data["subject"].(string); ok {
		metadata.Subject = subject
	}
	if pageCount, ok := data["page_count"].(float64); ok {
		metadata.PageCount = int(pageCount)
	}
	if wordCount, ok := data["word_count"].(float64); ok {
		metadata.WordCount = int(wordCount)
	}

	return metadata
}

// parseProcessingInfo converts the Python processing info to Go struct
func (t *DocumentProcessorTool) parseProcessingInfo(data map[string]interface{}) ProcessingInfo {
	info := ProcessingInfo{}

	if mode, ok := data["processing_mode"].(string); ok {
		info.ProcessingMode = ProcessingMode(mode)
	}
	if hwAccel, ok := data["hardware_acceleration"].(string); ok {
		info.HardwareAcceleration = HardwareAcceleration(hwAccel)
	}
	if ocrEnabled, ok := data["ocr_enabled"].(bool); ok {
		info.OCREnabled = ocrEnabled
	}
	if procTime, ok := data["processing_time"].(float64); ok {
		info.ProcessingTime = time.Duration(procTime * float64(time.Second))
	}
	if timestamp, ok := data["timestamp"].(float64); ok {
		info.Timestamp = time.Unix(int64(timestamp), 0)
	}

	return info
}

// formatResponse formats the response for MCP output
func (t *DocumentProcessorTool) formatResponse(response *DocumentProcessingResponse) map[string]interface{} {
	result := map[string]interface{}{
		"source":          response.Source,
		"content":         response.Content,
		"cache_hit":       response.CacheHit,
		"processing_info": response.ProcessingInfo,
	}

	if response.Error != "" {
		result["error"] = response.Error
	}

	if response.Metadata != nil {
		result["metadata"] = response.Metadata
	}

	if len(response.Images) > 0 {
		result["images"] = response.Images
	}

	if len(response.Tables) > 0 {
		result["tables"] = response.Tables
	}

	return result
}

// resolveSourcePath resolves the source path to an absolute path
func (t *DocumentProcessorTool) resolveSourcePath(source string) (string, error) {
	// Check if it's a URL
	if parsedURL, err := url.Parse(source); err == nil && parsedURL.Scheme != "" {
		// It's a URL, return as-is
		return source, nil
	}

	// Check if it's already an absolute path
	if filepath.IsAbs(source) {
		// Verify the file exists
		if _, err := os.Stat(source); err != nil {
			return "", fmt.Errorf("file not found: %s", source)
		}
		return source, nil
	}

	// It's a relative path, make it absolute
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	absolutePath := filepath.Join(cwd, source)

	// Verify the file exists
	if _, err := os.Stat(absolutePath); err != nil {
		return "", fmt.Errorf("file not found: %s (resolved to %s)", source, absolutePath)
	}

	return absolutePath, nil
}

// newToolResultJSON creates a new tool result with JSON content
func (t *DocumentProcessorTool) newToolResultJSON(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

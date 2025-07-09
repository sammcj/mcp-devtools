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
	// Build profile description dynamically based on available features
	profileDesc := "Processing profile: 'text-and-image' (text and images, default), 'basic' (text only), 'scanned' (OCR), 'llm-smoldocling' (SmolDocling vision)"

	// Only include llm-external if LLM environment variables are configured
	if IsLLMConfigured() {
		profileDesc += ", 'llm-external' (external LLM for chart / diagram conversion to mermaid)"
	}

	return mcp.NewTool(
		"process_document",
		mcp.WithDescription("Process documents (PDF, DOCX, XLSX, PPTX, HTML, CSV, PNG, JPG) and convert them to structured Markdown with optional OCR, image extraction, and table processing. Supports hardware acceleration and intelligent caching."),
		mcp.WithString("source",
			mcp.Required(),
			mcp.Description("Document source: MUST be a fully qualified absolute file path (e.g., /Users/user/documents/file.pdf), complete URL (e.g., https://example.com/doc.pdf). Relative paths are NOT supported - always provide the complete absolute path to the file."),
		),
		mcp.WithString("profile",
			mcp.Description(profileDesc),
		),
		mcp.WithBoolean("inline",
			mcp.Description("Return content inline in response instead of to files (default: false)."),
		),
		mcp.WithString("save_to",
			mcp.Description("Override the file path for saved content (default: same directory as source file). MUST be a fully qualified absolute path"),
		),
		mcp.WithBoolean("cache_enabled",
			mcp.Description("Override global cache setting for this request (default: true)"),
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

	// Parse and validate arguments
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

	// Optional: inline (default: false)
	if inline, ok := args["inline"].(bool); ok {
		req.Inline = &inline
	}

	// Optional: save_to
	if saveTo, ok := args["save_to"].(string); ok {
		req.SaveTo = strings.TrimSpace(saveTo)
	}

	// Optional: profile (default: text-and-image)
	if profile, ok := args["profile"].(string); ok {
		req.Profile = ProcessingProfile(profile)
	} else {
		req.Profile = ProfileTextAndImage
	}

	// Optional: table_former_mode
	if tableMode, ok := args["table_former_mode"].(string); ok {
		req.TableFormerMode = TableFormerMode(tableMode)
	} else {
		req.TableFormerMode = TableFormerModeAccurate
	}

	// Optional: cell_matching
	if cellMatching, ok := args["cell_matching"].(bool); ok {
		req.CellMatching = &cellMatching
	}

	// Optional: vision_mode
	if visionMode, ok := args["vision_mode"].(string); ok {
		req.VisionMode = VisionProcessingMode(visionMode)
	} else {
		req.VisionMode = VisionModeStandard
	}

	// Optional: diagram_description
	if diagramDesc, ok := args["diagram_description"].(bool); ok {
		req.DiagramDescription = diagramDesc
	}

	// Optional: chart_data_extraction
	if chartExtraction, ok := args["chart_data_extraction"].(bool); ok {
		req.ChartDataExtraction = chartExtraction
	}

	// Optional: enable_remote_services
	if remoteServices, ok := args["enable_remote_services"].(bool); ok {
		req.EnableRemoteServices = remoteServices
	}

	// Optional: convert_diagrams_to_mermaid
	if convertMermaid, ok := args["convert_diagrams_to_mermaid"].(bool); ok {
		req.ConvertDiagramsToMermaid = convertMermaid
	}

	// Optional: generate_diagrams
	if generateDiagrams, ok := args["generate_diagrams"].(bool); ok {
		req.GenerateDiagrams = generateDiagrams
	}

	// Optional: clear_file_cache
	if clearCache, ok := args["clear_file_cache"].(bool); ok {
		req.ClearFileCache = clearCache
	}

	// Optional: extract_images
	if extractImages, ok := args["extract_images"].(bool); ok {
		req.ExtractImages = extractImages
	}

	// Optional: debug
	if debug, ok := args["debug"].(bool); ok {
		req.Debug = debug
	}

	// Apply profile settings if specified
	if req.Profile != "" {
		t.applyProfile(req)
	}

	return req, nil
}

// applyProfile applies the specified processing profile to the request
func (t *DocumentProcessorTool) applyProfile(req *DocumentProcessingRequest) {
	switch req.Profile {
	case ProfileBasic:
		// Text extraction only (fast processing)
		req.ProcessingMode = ProcessingModeBasic
		req.VisionMode = VisionModeStandard
		req.DiagramDescription = false
		req.ChartDataExtraction = false
		req.EnableRemoteServices = false
		req.GenerateDiagrams = false

	case ProfileTextAndImage:
		// Text and image extraction with tables
		req.ProcessingMode = ProcessingModeAdvanced
		req.VisionMode = VisionModeStandard
		req.PreserveImages = true
		req.DiagramDescription = false
		req.ChartDataExtraction = false
		req.EnableRemoteServices = false
		req.GenerateDiagrams = false

	case ProfileScanned:
		// OCR-focused processing for scanned documents
		req.ProcessingMode = ProcessingModeOCR
		req.EnableOCR = true
		req.VisionMode = VisionModeStandard
		req.DiagramDescription = false
		req.ChartDataExtraction = false
		req.EnableRemoteServices = false
		req.GenerateDiagrams = false

	case ProfileLLMSmolDocling:
		// Text and image extraction enhanced with SmolDocling vision model
		req.ProcessingMode = ProcessingModeAdvanced
		req.VisionMode = VisionModeSmolDocling
		req.PreserveImages = true
		req.DiagramDescription = true
		req.ChartDataExtraction = true
		req.EnableRemoteServices = true
		req.GenerateDiagrams = false // SmolDocling only, no external LLM

	case ProfileLLMExternal:
		// Text and image extraction enhanced with external vision LLM for diagram conversion to Mermaid
		// Only available when DOCLING_LLM_* environment variables are configured
		if IsLLMConfigured() {
			req.ProcessingMode = ProcessingModeAdvanced
			req.VisionMode = VisionModeSmolDocling
			req.PreserveImages = true
			req.DiagramDescription = true
			req.ChartDataExtraction = true
			req.EnableRemoteServices = true
			req.GenerateDiagrams = true // Enable external LLM enhancement
		} else {
			// Fall back to SmolDocling profile if LLM not configured
			req.ProcessingMode = ProcessingModeAdvanced
			req.VisionMode = VisionModeSmolDocling
			req.PreserveImages = true
			req.DiagramDescription = true
			req.ChartDataExtraction = true
			req.EnableRemoteServices = true
			req.GenerateDiagrams = false
		}
	}
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

	// Add new parameters
	if req.TableFormerMode != "" {
		args = append(args, "--table-former-mode", string(req.TableFormerMode))
	}

	if req.CellMatching != nil {
		if *req.CellMatching {
			args = append(args, "--cell-matching")
		} else {
			args = append(args, "--no-cell-matching")
		}
	}

	if req.VisionMode != "" && req.VisionMode != VisionModeStandard {
		args = append(args, "--vision-mode", string(req.VisionMode))
	}

	if req.DiagramDescription {
		args = append(args, "--diagram-description")
	}

	if req.ChartDataExtraction {
		args = append(args, "--chart-data-extraction")
	}

	if req.EnableRemoteServices {
		args = append(args, "--enable-remote-services")
	}

	if req.ConvertDiagramsToMermaid {
		args = append(args, "--convert-diagrams-to-mermaid")
	}

	// Auto-enable image extraction when saving to file or extract_images is true
	if t.shouldSaveToFile(req) || req.ExtractImages {
		args = append(args, "--extract-images")
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

	// Set up environment with certificate configuration
	cmd.Env = os.Environ() // Start with current environment
	certEnv := t.config.GetCertificateEnvironment()
	cmd.Env = append(cmd.Env, certEnv...)

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

	// Extract diagrams if available
	if diagramsData, ok := pythonResult["diagrams"].([]interface{}); ok {
		response.Diagrams = t.parseDiagrams(diagramsData)
	}

	// Extract images if available
	if imagesData, ok := pythonResult["images"].([]interface{}); ok {
		response.Images = t.parseImages(imagesData)
	}

	// Enhance diagrams with LLM if requested and configured
	if req.GenerateDiagrams && len(response.Diagrams) > 0 {
		enhancedDiagrams, err := t.enhanceDiagramsWithLLM(response.Diagrams)
		if err != nil {
			// Log the error but don't fail the entire processing - fall back gracefully
			// Add a note to the processing info that LLM enhancement was attempted but failed
			if response.ProcessingInfo.ProcessingMethod != "" {
				response.ProcessingInfo.ProcessingMethod += "+llm:failed"
			}
			// Continue with original diagrams instead of failing
		} else {
			// LLM enhancement succeeded
			response.Diagrams = enhancedDiagrams
			if response.ProcessingInfo.ProcessingMethod != "" {
				response.ProcessingInfo.ProcessingMethod += "+llm:enhanced"
			}

			// Update the markdown content to include the enhanced diagrams with Mermaid code
			if req.OutputFormat == OutputFormatMarkdown || req.OutputFormat == OutputFormatBoth {
				response.Content = t.insertMermaidDiagramsIntoMarkdown(response.Content, enhancedDiagrams)
			}
		}
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
	if method, ok := data["processing_method"].(string); ok {
		info.ProcessingMethod = method
	}
	if hwAccel, ok := data["hardware_acceleration"].(string); ok {
		info.HardwareAcceleration = HardwareAcceleration(hwAccel)
	}
	if ocrEnabled, ok := data["ocr_enabled"].(bool); ok {
		info.OCREnabled = ocrEnabled
	}
	if ocrLangs, ok := data["ocr_languages"].([]interface{}); ok {
		for _, lang := range ocrLangs {
			if langStr, ok := lang.(string); ok {
				info.OCRLanguages = append(info.OCRLanguages, langStr)
			}
		}
	}
	if procTime, ok := data["processing_time"].(float64); ok {
		// Processing time is already in seconds from Python
		info.ProcessingTime = procTime
	}
	if timestamp, ok := data["timestamp"].(float64); ok {
		info.Timestamp = time.Unix(int64(timestamp), 0)
	}

	return info
}

// parseDiagrams converts the Python diagrams data to Go structs
func (t *DocumentProcessorTool) parseDiagrams(data []interface{}) []ExtractedDiagram {
	var diagrams []ExtractedDiagram

	for _, item := range data {
		if diagramData, ok := item.(map[string]interface{}); ok {
			diagram := ExtractedDiagram{}

			if id, ok := diagramData["id"].(string); ok {
				diagram.ID = id
			}
			if diagramType, ok := diagramData["type"].(string); ok {
				diagram.Type = diagramType
			}
			if caption, ok := diagramData["caption"].(string); ok {
				diagram.Caption = caption
			}
			if description, ok := diagramData["description"].(string); ok {
				diagram.Description = description
			}
			if diagType, ok := diagramData["diagram_type"].(string); ok {
				diagram.DiagramType = diagType
			}
			if mermaidCode, ok := diagramData["mermaid_code"].(string); ok {
				diagram.MermaidCode = mermaidCode
			}
			if base64Data, ok := diagramData["base64_data"].(string); ok {
				diagram.Base64Data = base64Data
			}
			if pageNum, ok := diagramData["page_number"].(float64); ok {
				diagram.PageNumber = int(pageNum)
			}
			if confidence, ok := diagramData["confidence"].(float64); ok {
				diagram.Confidence = confidence
			}

			// Parse bounding box
			if bboxData, ok := diagramData["bounding_box"].(map[string]interface{}); ok {
				bbox := &BoundingBox{}
				if x, ok := bboxData["x"].(float64); ok {
					bbox.X = x
				}
				if y, ok := bboxData["y"].(float64); ok {
					bbox.Y = y
				}
				if width, ok := bboxData["width"].(float64); ok {
					bbox.Width = width
				}
				if height, ok := bboxData["height"].(float64); ok {
					bbox.Height = height
				}
				diagram.BoundingBox = bbox
			}

			// Parse elements
			if elementsData, ok := diagramData["elements"].([]interface{}); ok {
				for _, elemItem := range elementsData {
					if elemData, ok := elemItem.(map[string]interface{}); ok {
						element := DiagramElement{}
						if elemType, ok := elemData["type"].(string); ok {
							element.Type = elemType
						}
						if content, ok := elemData["content"].(string); ok {
							element.Content = content
						}
						if position, ok := elemData["position"].(string); ok {
							element.Position = position
						}

						// Parse element bounding box
						if elemBboxData, ok := elemData["bounding_box"].(map[string]interface{}); ok {
							elemBbox := &BoundingBox{}
							if x, ok := elemBboxData["x"].(float64); ok {
								elemBbox.X = x
							}
							if y, ok := elemBboxData["y"].(float64); ok {
								elemBbox.Y = y
							}
							if width, ok := elemBboxData["width"].(float64); ok {
								elemBbox.Width = width
							}
							if height, ok := elemBboxData["height"].(float64); ok {
								elemBbox.Height = height
							}
							element.BoundingBox = elemBbox
						}

						diagram.Elements = append(diagram.Elements, element)
					}
				}
			}

			// Parse properties
			if props, ok := diagramData["properties"].(map[string]interface{}); ok {
				diagram.Properties = props
			}

			diagrams = append(diagrams, diagram)
		}
	}

	return diagrams
}

// parseImages converts the Python images data to Go structs
func (t *DocumentProcessorTool) parseImages(data []interface{}) []ExtractedImage {
	var images []ExtractedImage

	for _, item := range data {
		if imageData, ok := item.(map[string]interface{}); ok {
			image := ExtractedImage{}

			if id, ok := imageData["id"].(string); ok {
				image.ID = id
			}
			if imageType, ok := imageData["type"].(string); ok {
				image.Type = imageType
			}
			if caption, ok := imageData["caption"].(string); ok {
				image.Caption = caption
			}
			if altText, ok := imageData["alt_text"].(string); ok {
				image.AltText = altText
			}
			if format, ok := imageData["format"].(string); ok {
				image.Format = format
			}
			if width, ok := imageData["width"].(float64); ok {
				image.Width = int(width)
			}
			if height, ok := imageData["height"].(float64); ok {
				image.Height = int(height)
			}
			if size, ok := imageData["size"].(float64); ok {
				image.Size = int64(size)
			}
			if filePath, ok := imageData["file_path"].(string); ok {
				image.FilePath = filePath
			}
			if pageNum, ok := imageData["page_number"].(float64); ok {
				image.PageNumber = int(pageNum)
			}

			// Parse bounding box
			if bboxData, ok := imageData["bounding_box"].(map[string]interface{}); ok {
				bbox := &BoundingBox{}
				if x, ok := bboxData["x"].(float64); ok {
					bbox.X = x
				}
				if y, ok := bboxData["y"].(float64); ok {
					bbox.Y = y
				}
				if width, ok := bboxData["width"].(float64); ok {
					bbox.Width = width
				}
				if height, ok := bboxData["height"].(float64); ok {
					bbox.Height = height
				}
				image.BoundingBox = bbox
			}

			// Parse extracted text
			if extractedTextData, ok := imageData["extracted_text"].([]interface{}); ok {
				for _, textItem := range extractedTextData {
					if textStr, ok := textItem.(string); ok {
						image.ExtractedText = append(image.ExtractedText, textStr)
					}
				}
			}

			images = append(images, image)
		}
	}

	return images
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

	if len(response.Diagrams) > 0 {
		result["diagrams"] = response.Diagrams
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

// enhanceDiagramsWithLLM enhances diagrams using external LLM analysis
func (t *DocumentProcessorTool) enhanceDiagramsWithLLM(diagrams []ExtractedDiagram) ([]ExtractedDiagram, error) {
	// Check if LLM is configured
	if !IsLLMConfigured() {
		return diagrams, fmt.Errorf("LLM not configured: required environment variables %s, %s, %s not set",
			EnvOpenAIAPIBase, EnvOpenAIModel, EnvOpenAIAPIKey)
	}

	// Create LLM client
	llmClient, err := NewDiagramLLMClient()
	if err != nil {
		return diagrams, fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Process each diagram
	enhancedDiagrams := make([]ExtractedDiagram, len(diagrams))
	for i, diagram := range diagrams {
		// Start with the original diagram
		enhancedDiagrams[i] = diagram

		// Enhance with LLM analysis
		analysis, err := llmClient.AnalyseDiagram(&diagram)
		if err != nil {
			// Return error instead of silently continuing - this was the bug!
			return diagrams, fmt.Errorf("LLM analysis failed for diagram %s: %w", diagram.ID, err)
		}

		// Update diagram with LLM analysis
		if analysis.Description != "" {
			enhancedDiagrams[i].Description = analysis.Description
		}
		if analysis.DiagramType != "" {
			enhancedDiagrams[i].DiagramType = analysis.DiagramType
		}
		if analysis.MermaidCode != "" {
			enhancedDiagrams[i].MermaidCode = analysis.MermaidCode
		}
		if len(analysis.Elements) > 0 {
			enhancedDiagrams[i].Elements = analysis.Elements
		}
		if analysis.Confidence > 0 {
			enhancedDiagrams[i].Confidence = analysis.Confidence
		}

		// Merge properties
		if enhancedDiagrams[i].Properties == nil {
			enhancedDiagrams[i].Properties = make(map[string]interface{})
		}
		if analysis.Properties != nil {
			for key, value := range analysis.Properties {
				enhancedDiagrams[i].Properties[key] = value
			}
		}

		// Add LLM processing metadata
		enhancedDiagrams[i].Properties["llm_enhanced"] = true
		enhancedDiagrams[i].Properties["llm_processing_time"] = analysis.ProcessingTime.Seconds()
	}

	return enhancedDiagrams, nil
}

// getDebugInfo returns debug information including environment variables (with secrets masked)
func (t *DocumentProcessorTool) getDebugInfo() map[string]interface{} {
	debugInfo := map[string]interface{}{
		"debug_mode":  true,
		"timestamp":   time.Now().Format(time.RFC3339),
		"system_info": t.config.GetSystemInfo(),
	}

	// Environment variables related to document processing
	envVars := map[string]interface{}{
		// LLM Configuration
		"DOCLING_LLM_OPENAI_API_BASE": maskSecret(os.Getenv("DOCLING_LLM_OPENAI_API_BASE")),
		"DOCLING_LLM_MODEL_NAME":      os.Getenv("DOCLING_LLM_MODEL_NAME"),
		"DOCLING_LLM_OPENAI_API_KEY":  maskSecret(os.Getenv("DOCLING_LLM_OPENAI_API_KEY")),
		"DOCLING_LLM_MAX_TOKENS":      os.Getenv("DOCLING_LLM_MAX_TOKENS"),
		"DOCLING_LLM_TEMPERATURE":     os.Getenv("DOCLING_LLM_TEMPERATURE"),
		"DOCLING_LLM_TIMEOUT":         os.Getenv("DOCLING_LLM_TIMEOUT"),

		// Cache Configuration
		"DOCLING_CACHE_MAX_AGE_HOURS": os.Getenv("DOCLING_CACHE_MAX_AGE_HOURS"),
		"DOCLING_CACHE_ENABLED":       os.Getenv("DOCLING_CACHE_ENABLED"),

		// Processing Configuration
		"DOCLING_TIMEOUT":     os.Getenv("DOCLING_TIMEOUT"),
		"DOCLING_MAX_FILE_MB": os.Getenv("DOCLING_MAX_FILE_MB"),

		// Certificate Configuration
		"SSL_CERT_FILE": maskSecret(os.Getenv("SSL_CERT_FILE")),
		"SSL_CERT_DIR":  os.Getenv("SSL_CERT_DIR"),
	}

	debugInfo["environment_variables"] = envVars

	// LLM Configuration Status
	llmStatus := map[string]interface{}{
		"configured":   IsLLMConfigured(),
		"api_base_set": os.Getenv("DOCLING_LLM_OPENAI_API_BASE") != "",
		"model_set":    os.Getenv("DOCLING_LLM_MODEL_NAME") != "",
		"api_key_set":  os.Getenv("DOCLING_LLM_OPENAI_API_KEY") != "",
	}

	if IsLLMConfigured() {
		// Test LLM client creation (but don't make API calls)
		if _, err := NewDiagramLLMClient(); err != nil {
			llmStatus["client_creation_error"] = err.Error()
		} else {
			llmStatus["client_creation"] = "success"
		}
	}

	debugInfo["llm_status"] = llmStatus

	// Configuration details
	configInfo := map[string]interface{}{
		"python_path":       t.config.PythonPath,
		"script_path":       t.config.GetScriptPath(),
		"cache_enabled":     t.config.CacheEnabled,
		"cache_directory":   "cache", // Default cache directory
		"timeout":           t.config.Timeout,
		"max_file_size":     t.config.MaxFileSize,
		"docling_available": t.config.isDoclingAvailable(),
	}

	debugInfo["configuration"] = configInfo

	return debugInfo
}

// insertMermaidDiagramsIntoMarkdown inserts Mermaid code blocks into the markdown content
func (t *DocumentProcessorTool) insertMermaidDiagramsIntoMarkdown(content string, diagrams []ExtractedDiagram) string {
	// For each diagram with Mermaid code, find a suitable place to insert it
	updatedContent := content

	for _, diagram := range diagrams {
		if diagram.MermaidCode == "" {
			continue // Skip diagrams without Mermaid code
		}

		// Create the Mermaid code block
		mermaidBlock := fmt.Sprintf("\n\n```mermaid\n%s\n```\n\n*Enhanced diagram: %s*\n",
			diagram.MermaidCode, diagram.Description)

		// Try to find a good insertion point based on diagram caption or description
		insertionPoint := ""
		if diagram.Caption != "" {
			insertionPoint = diagram.Caption
		} else if diagram.Description != "" {
			// Use first few words of description
			words := strings.Fields(diagram.Description)
			if len(words) > 3 {
				insertionPoint = strings.Join(words[:3], " ")
			} else {
				insertionPoint = diagram.Description
			}
		}

		if insertionPoint != "" {
			// Look for the insertion point in the content
			if strings.Contains(updatedContent, insertionPoint) {
				// Insert the Mermaid block after the first occurrence
				insertIndex := strings.Index(updatedContent, insertionPoint) + len(insertionPoint)
				updatedContent = updatedContent[:insertIndex] + mermaidBlock + updatedContent[insertIndex:]
			} else {
				// Fallback: append at the end
				updatedContent += mermaidBlock
			}
		} else {
			// No good insertion point found, append at the end
			updatedContent += mermaidBlock
		}
	}

	return updatedContent
}

// shouldSaveToFile determines if content should be saved to a file
func (t *DocumentProcessorTool) shouldSaveToFile(req *DocumentProcessingRequest) bool {
	// If inline is explicitly set to true, return content inline
	if req.Inline != nil && *req.Inline {
		return false
	}
	// Default behaviour: save to file (inline=false by default)
	return true
}

// handleSaveToFile saves the converted content to the specified file and returns a success message
func (t *DocumentProcessorTool) handleSaveToFile(savePath string, response *DocumentProcessingResponse) (*mcp.CallToolResult, error) {
	// Auto-generate save path if not provided
	if savePath == "" {
		generatedPath, err := t.generateSavePath(response.Source)
		if err != nil {
			return nil, fmt.Errorf("failed to generate save path: %w", err)
		}
		savePath = generatedPath
	}

	// Validate save path is absolute
	if !filepath.IsAbs(savePath) {
		return nil, fmt.Errorf("save_to must be a fully qualified absolute path, got: %s", savePath)
	}

	// Create directory if it doesn't exist
	saveDir := filepath.Dir(savePath)
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create save directory %s: %w", saveDir, err)
	}

	// Write content to file
	if err := os.WriteFile(savePath, []byte(response.Content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write content to %s: %w", savePath, err)
	}

	// Create success response
	result := map[string]interface{}{
		"success":   true,
		"message":   "Content successfully exported to file",
		"save_path": savePath,
		"source":    response.Source,
		"cache_hit": response.CacheHit,
		"metadata": map[string]interface{}{
			"file_size": len(response.Content),
		},
		"processing_info": response.ProcessingInfo,
	}

	// Include document metadata if available
	if response.Metadata != nil {
		if metadata, ok := result["metadata"].(map[string]interface{}); ok {
			metadata["document_title"] = response.Metadata.Title
			metadata["document_author"] = response.Metadata.Author
			metadata["page_count"] = response.Metadata.PageCount
			metadata["word_count"] = response.Metadata.WordCount
		}
	}

	return t.newToolResultJSON(result)
}

// generateSavePath generates a save path in the same directory as the source file with .md extension
func (t *DocumentProcessorTool) generateSavePath(source string) (string, error) {
	// Check if it's a URL
	if parsedURL, err := url.Parse(source); err == nil && parsedURL.Scheme != "" {
		// For URLs, use the filename from the path or a default name
		urlPath := parsedURL.Path
		if urlPath == "" || urlPath == "/" {
			return "", fmt.Errorf("cannot generate save path for URL without filename: %s", source)
		}

		// Extract filename from URL path
		filename := filepath.Base(urlPath)
		if filename == "." || filename == "/" {
			filename = "document"
		}

		// Remove extension and add .md
		nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
		return filepath.Join(".", nameWithoutExt+".md"), nil
	}

	// For file paths, generate save path in the same directory
	if !filepath.IsAbs(source) {
		// Make relative path absolute first
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
		source = filepath.Join(cwd, source)
	}

	// Get directory and filename
	sourceDir := filepath.Dir(source)
	sourceFilename := filepath.Base(source)

	// Remove extension and add .md
	nameWithoutExt := strings.TrimSuffix(sourceFilename, filepath.Ext(sourceFilename))
	savePath := filepath.Join(sourceDir, nameWithoutExt+".md")

	return savePath, nil
}

// maskSecret masks sensitive information in environment variables
func maskSecret(value string) string {
	if value == "" {
		return "(not set)"
	}
	if len(value) <= 8 {
		return "***"
	}
	// Show first 4 and last 4 characters, mask the middle
	return value[:4] + "..." + value[len(value)-4:]
}

// newToolResultJSON creates a new tool result with JSON content
func (t *DocumentProcessorTool) newToolResultJSON(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

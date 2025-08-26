package docprocessing

import (
	"fmt"
	"strings"
)

// parseRequest parses and validates the request arguments
func (t *DocumentProcessorTool) parseRequest(args map[string]any) (*DocumentProcessingRequest, error) {
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
	if langs, ok := args["ocr_languages"].([]any); ok {
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

	// Optional: return_inline_only (default: false)
	if returnInline, ok := args["return_inline_only"].(bool); ok {
		req.ReturnInlineOnly = &returnInline
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

	// Apply profile settings first, then allow individual arguments to override
	if req.Profile != "" {
		t.applyProfile(req)
	}

	// Re-apply individual arguments to override profile settings
	// This ensures explicit arguments take precedence over profile defaults

	// Re-apply vision_mode if explicitly provided
	if visionMode, ok := args["vision_mode"].(string); ok {
		req.VisionMode = VisionProcessingMode(visionMode)
	}

	// Re-apply diagram_description if explicitly provided
	if diagramDesc, ok := args["diagram_description"].(bool); ok {
		req.DiagramDescription = diagramDesc
	}

	// Re-apply chart_data_extraction if explicitly provided
	if chartExtraction, ok := args["chart_data_extraction"].(bool); ok {
		req.ChartDataExtraction = chartExtraction
	}

	// Re-apply enable_remote_services if explicitly provided
	if remoteServices, ok := args["enable_remote_services"].(bool); ok {
		req.EnableRemoteServices = remoteServices
	}

	// Re-apply convert_diagrams_to_mermaid if explicitly provided
	if convertMermaid, ok := args["convert_diagrams_to_mermaid"].(bool); ok {
		req.ConvertDiagramsToMermaid = convertMermaid
	}

	// Re-apply generate_diagrams if explicitly provided
	if generateDiagrams, ok := args["generate_diagrams"].(bool); ok {
		req.GenerateDiagrams = generateDiagrams
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
		// Only available when DOCLING_VLM_* environment variables are configured
		if IsLLMConfigured() {
			req.ProcessingMode = ProcessingModeAdvanced
			req.VisionMode = VisionModeAdvanced // Use advanced mode to trigger external API
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

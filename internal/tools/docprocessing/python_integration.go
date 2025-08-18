package docprocessing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sammcj/mcp-devtools/internal/security"
)

// processDocument processes the document using the Python wrapper
func (t *DocumentProcessorTool) processDocument(req *DocumentProcessingRequest) (*DocumentProcessingResponse, error) {
	// Resolve source path to absolute path
	sourcePath, err := t.resolveSourcePath(req.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source path: %w", err)
	}

	// Get and validate script path
	scriptPath := t.config.GetScriptPath()
	// Security: Check file access for script path
	if err := security.CheckFileAccess(scriptPath); err != nil {
		return nil, fmt.Errorf("script access denied: %w", err)
	}
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

	// Set up environment with certificate configuration and VLM variables
	cmd.Env = os.Environ() // Start with current environment
	certEnv := t.config.GetCertificateEnvironment()
	cmd.Env = append(cmd.Env, certEnv...)

	// Add VLM Pipeline environment variables
	vlmEnv := t.getVLMEnvironmentVariables()
	cmd.Env = append(cmd.Env, vlmEnv...)

	// Capture both stdout and stderr for better debugging
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Log the command being executed for debugging
	cmdStr := fmt.Sprintf("%s %s", t.config.PythonPath, strings.Join(args, " "))

	// Execute the command
	err = cmd.Run()

	// Get the outputs
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	// Log outputs for debugging (but not to stdout/stderr to avoid MCP protocol issues)
	// Write to a debug log file instead
	if debugFile, debugErr := os.OpenFile(filepath.Join(os.Getenv("HOME"), ".mcp-devtools", "debug.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600); debugErr == nil {
		defer func() { _ = debugFile.Close() }()
		_, _ = fmt.Fprintf(debugFile, "[%s] Command: %s\n", time.Now().Format("2006-01-02 15:04:05"), cmdStr)
		_, _ = fmt.Fprintf(debugFile, "[%s] Exit Code: %v\n", time.Now().Format("2006-01-02 15:04:05"), err)
		_, _ = fmt.Fprintf(debugFile, "[%s] Stdout Length: %d\n", time.Now().Format("2006-01-02 15:04:05"), len(stdoutStr))
		_, _ = fmt.Fprintf(debugFile, "[%s] Stderr Length: %d\n", time.Now().Format("2006-01-02 15:04:05"), len(stderrStr))
		if len(stdoutStr) > 0 {
			_, _ = fmt.Fprintf(debugFile, "[%s] Stdout: %s\n", time.Now().Format("2006-01-02 15:04:05"), stdoutStr)
		}
		if len(stderrStr) > 0 {
			_, _ = fmt.Fprintf(debugFile, "[%s] Stderr: %s\n", time.Now().Format("2006-01-02 15:04:05"), stderrStr)
		}
		_, _ = fmt.Fprintf(debugFile, "[%s] Environment Variables:\n", time.Now().Format("2006-01-02 15:04:05"))
		for _, env := range cmd.Env {
			if strings.HasPrefix(env, "DOCLING_") {
				_, _ = fmt.Fprintf(debugFile, "  %s\n", env)
			}
		}
		_, _ = fmt.Fprintf(debugFile, "---\n")
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("processing timeout after %d seconds", timeout)
		}
		return nil, fmt.Errorf("python script failed: %w, stderr: %s", err, stderrStr)
	}

	// Use stdout as the output
	output := []byte(stdoutStr)

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

	// Process each diagram and aggregate token usage
	enhancedDiagrams := make([]ExtractedDiagram, len(diagrams))
	var totalTokenUsage *TokenUsage

	for i, diagram := range diagrams {
		// Start with the original diagram
		enhancedDiagrams[i] = diagram

		// Enhance with LLM analysis
		analysis, err := llmClient.AnalyseDiagram(&diagram)
		if err != nil {
			// Return error instead of silently continuing - this was the bug!
			return diagrams, fmt.Errorf("LLM analysis failed for diagram %s: %w", diagram.ID, err)
		}

		// Aggregate token usage
		if analysis.TokenUsage != nil {
			if totalTokenUsage == nil {
				totalTokenUsage = &TokenUsage{}
			}
			totalTokenUsage.PromptTokens += analysis.TokenUsage.PromptTokens
			totalTokenUsage.CompletionTokens += analysis.TokenUsage.CompletionTokens
			totalTokenUsage.TotalTokens += analysis.TokenUsage.TotalTokens
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

		// Add token usage to diagram properties if available
		if analysis.TokenUsage != nil {
			enhancedDiagrams[i].Properties["llm_token_usage"] = analysis.TokenUsage
		}
	}

	return enhancedDiagrams, nil
}

// getVLMEnvironmentVariables returns VLM Pipeline environment variables for the subprocess
func (t *DocumentProcessorTool) getVLMEnvironmentVariables() []string {
	var envVars []string

	// VLM Pipeline configuration variables
	if apiURL := os.Getenv("DOCLING_VLM_API_URL"); apiURL != "" {
		envVars = append(envVars, fmt.Sprintf("DOCLING_VLM_API_URL=%s", apiURL))
	}

	if model := os.Getenv("DOCLING_VLM_MODEL"); model != "" {
		envVars = append(envVars, fmt.Sprintf("DOCLING_VLM_MODEL=%s", model))
	}

	if apiKey := os.Getenv("DOCLING_VLM_API_KEY"); apiKey != "" {
		envVars = append(envVars, fmt.Sprintf("DOCLING_VLM_API_KEY=%s", apiKey))
	}

	if timeout := os.Getenv("DOCLING_VLM_TIMEOUT"); timeout != "" {
		envVars = append(envVars, fmt.Sprintf("DOCLING_VLM_TIMEOUT=%s", timeout))
	}

	if fallbackLocal := os.Getenv("DOCLING_VLM_FALLBACK_LOCAL"); fallbackLocal != "" {
		envVars = append(envVars, fmt.Sprintf("DOCLING_VLM_FALLBACK_LOCAL=%s", fallbackLocal))
	}

	if imageScale := os.Getenv("DOCLING_IMAGE_SCALE"); imageScale != "" {
		envVars = append(envVars, fmt.Sprintf("DOCLING_IMAGE_SCALE=%s", imageScale))
	}

	return envVars
}

// insertMermaidDiagramsIntoMarkdown inserts Mermaid code blocks into the markdown content
func (t *DocumentProcessorTool) insertMermaidDiagramsIntoMarkdown(content string, diagrams []ExtractedDiagram) string {
	// For each diagram with Mermaid code, find a suitable place to insert it
	updatedContent := content

	for _, diagram := range diagrams {
		if diagram.MermaidCode == "" {
			continue // Skip diagrams without Mermaid code
		}

		// Create the Mermaid code block with reference to original image
		// Only include the description if it's meaningful and different from a generic placeholder
		descriptionText := ""
		if diagram.Description != "" &&
			diagram.Description != "Diagram analysis completed" &&
			!strings.Contains(diagram.Description, "graph TD") {
			descriptionText = fmt.Sprintf("\n\n*%s*", diagram.Description)
		}

		mermaidBlock := fmt.Sprintf("\n\n**Mermaid Diagram (converted from %s):**\n\n```mermaid\n%s\n```%s\n",
			diagram.ID, diagram.MermaidCode, descriptionText)

		// Try to find the original image reference to insert the Mermaid diagram nearby
		imagePattern := fmt.Sprintf("![%s]", diagram.ID)
		if strings.Contains(updatedContent, imagePattern) {
			// Find the end of the image details section for this diagram
			imageIndex := strings.Index(updatedContent, imagePattern)

			// Look for the end of the details section (</details>)
			detailsEndPattern := "</details>"
			searchStart := imageIndex
			detailsEndIndex := strings.Index(updatedContent[searchStart:], detailsEndPattern)

			if detailsEndIndex != -1 {
				// Insert after the details section
				insertIndex := searchStart + detailsEndIndex + len(detailsEndPattern)
				updatedContent = updatedContent[:insertIndex] + mermaidBlock + updatedContent[insertIndex:]
			} else {
				// Fallback: insert after the image line
				nextLineIndex := strings.Index(updatedContent[imageIndex:], "\n")
				if nextLineIndex != -1 {
					insertIndex := imageIndex + nextLineIndex
					updatedContent = updatedContent[:insertIndex] + mermaidBlock + updatedContent[insertIndex:]
				} else {
					// Last resort: append at the end
					updatedContent += mermaidBlock
				}
			}
		} else {
			// Try to find a good insertion point based on diagram caption or description
			insertionPoint := ""
			if diagram.Caption != "" {
				insertionPoint = diagram.Caption
			} else if diagram.Description != "" && diagram.Description != "Diagram analysis completed" {
				// Use first few words of description
				words := strings.Fields(diagram.Description)
				if len(words) > 3 {
					insertionPoint = strings.Join(words[:3], " ")
				} else {
					insertionPoint = diagram.Description
				}
			}

			if insertionPoint != "" && strings.Contains(updatedContent, insertionPoint) {
				// Insert the Mermaid block after the first occurrence
				insertIndex := strings.Index(updatedContent, insertionPoint) + len(insertionPoint)
				updatedContent = updatedContent[:insertIndex] + mermaidBlock + updatedContent[insertIndex:]
			} else {
				// Fallback: append at the end
				updatedContent += mermaidBlock
			}
		}
	}

	return updatedContent
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
		// Security: Check file access control
		if err := security.CheckFileAccess(source); err != nil {
			return "", err
		}
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

	// Security: Check file access control for resolved path
	if err := security.CheckFileAccess(absolutePath); err != nil {
		return "", err
	}
	// Verify the file exists
	if _, err := os.Stat(absolutePath); err != nil {
		return "", fmt.Errorf("file not found: %s (resolved to %s)", source, absolutePath)
	}

	return absolutePath, nil
}

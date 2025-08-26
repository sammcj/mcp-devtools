package pdf

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

const (
	// PDF security limits
	DefaultMaxFileSize      = int64(200 * 1024 * 1024)      // 200MB default file size limit
	DefaultMaxMemoryLimit   = int64(5 * 1024 * 1024 * 1024) // 5GB default memory limit
	PDFMaxFileSizeEnvVar    = "PDF_MAX_FILE_SIZE"
	PDFMaxMemoryLimitEnvVar = "PDF_MAX_MEMORY_LIMIT"
)

// PDFTool implements PDF processing with pdfcpu
type PDFTool struct{}

// init registers the PDF tool
func init() {
	registry.Register(&PDFTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *PDFTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"pdf",
		mcp.WithDescription(`Extract text, tables & images from PDFs. The text extraction quality depends on the PDF structure. This PDF extraction tool is simpler & faster than the document processing tool, in general try this tool for PDFs first`),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("Absolute file path to the PDF document to process"),
		),
		mcp.WithString("output_dir",
			mcp.Description("Output directory for markdown & images (defaults to same directory as PDF)"),
		),
		mcp.WithBoolean("extract_images",
			mcp.Description("Extract images from the PDF (default: false)"),
			mcp.DefaultBool(false),
		),
		mcp.WithString("pages",
			mcp.Description("Page range to process (e.g., '1-5', '1,3,5', or 'all' for all pages, default: all)"),
			mcp.DefaultString("all"),
		),
	)
}

// Execute processes the PDF file
func (t *PDFTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Check if pdf tool is enabled (disabled by default)
	if !tools.IsToolEnabled("pdf") {
		return nil, fmt.Errorf("pdf tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'pdf'")
	}

	logger.Debug("Executing PDF processing tool")

	// Parse and validate parameters
	request, err := t.ParseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"file_path":      request.FilePath,
		"output_dir":     request.OutputDir,
		"extract_images": request.ExtractImages,
		"pages":          request.Pages,
	}).Debug("PDF processing parameters")

	// Security check for input file access
	if err := security.CheckFileAccess(request.FilePath); err != nil {
		return nil, err
	}

	// Security check for output directory access
	if err := security.CheckFileAccess(request.OutputDir); err != nil {
		return nil, err
	}

	// Validate input file exists and check file size
	fileInfo, err := os.Stat(request.FilePath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("PDF file does not exist: %s", request.FilePath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat PDF file: %w", err)
	}

	// Apply file size limits
	if err := t.ValidateFileSize(fileInfo.Size()); err != nil {
		return nil, fmt.Errorf("file size validation failed: %w", err)
	}

	// Create configuration with memory limits
	conf := model.NewDefaultConfiguration()
	t.applyMemoryLimits(conf)

	// Process the PDF
	result, err := t.processPDF(ctx, logger, request, conf)
	if err != nil {
		return t.newToolResultJSON(map[string]any{
			"error":     err.Error(),
			"file_path": request.FilePath,
		})
	}

	logger.WithFields(logrus.Fields{
		"file_path":        request.FilePath,
		"markdown_file":    result.MarkdownFile,
		"images_extracted": len(result.ExtractedImages),
		"pages_processed":  result.PagesProcessed,
	}).Debug("PDF processing completed successfully")

	return t.newToolResultJSON(result)
}

// ParseRequest parses and validates the tool arguments
func (t *PDFTool) ParseRequest(args map[string]any) (*PDFRequest, error) {
	// Parse file_path (required)
	filePath, ok := args["file_path"].(string)
	if !ok || filePath == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: file_path")
	}

	// Validate file path is absolute
	if !filepath.IsAbs(filePath) {
		return nil, fmt.Errorf("file_path must be an absolute path")
	}

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(filePath), ".pdf") {
		return nil, fmt.Errorf("file_path must be a PDF file (.pdf extension)")
	}

	request := &PDFRequest{
		FilePath:      filePath,
		ExtractImages: true,  // Default
		Pages:         "all", // Default
	}

	// Parse output_dir (optional)
	if outputDir, ok := args["output_dir"].(string); ok && outputDir != "" {
		if !filepath.IsAbs(outputDir) {
			return nil, fmt.Errorf("output_dir must be an absolute path")
		}
		request.OutputDir = outputDir
	} else {
		// Default to same directory as PDF
		request.OutputDir = filepath.Dir(filePath)
	}

	// Parse extract_images (optional)
	if extractImages, ok := args["extract_images"].(bool); ok {
		request.ExtractImages = extractImages
	}

	// Parse pages (optional)
	if pages, ok := args["pages"].(string); ok && pages != "" {
		request.Pages = pages
	}

	return request, nil
}

// processPDF handles the main PDF processing logic
func (t *PDFTool) processPDF(ctx context.Context, logger *logrus.Logger, request *PDFRequest, conf *model.Configuration) (*PDFResponse, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(request.OutputDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Get base filename without extension
	baseName := strings.TrimSuffix(filepath.Base(request.FilePath), ".pdf")
	markdownFile := filepath.Join(request.OutputDir, baseName+".md")

	var extractedImages []string
	var markdownContent strings.Builder

	// Add header to markdown
	markdownContent.WriteString(fmt.Sprintf("# %s\n\n", baseName))
	markdownContent.WriteString(fmt.Sprintf("*Extracted from: %s*\n\n", filepath.Base(request.FilePath)))

	// Get page count
	pageCount, err := api.PageCountFile(request.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}

	logger.WithField("page_count", pageCount).Debug("PDF page count")

	// Parse page selection
	selectedPages, err := t.ParsePageSelection(request.Pages, pageCount)
	if err != nil {
		return nil, fmt.Errorf("invalid page selection: %w", err)
	}

	// Convert selected pages to string slice for pdfcpu API
	selectedPageStrings := make([]string, len(selectedPages))
	for i, pageNum := range selectedPages {
		selectedPageStrings[i] = strconv.Itoa(pageNum)
	}

	// Extract images if requested
	if request.ExtractImages {
		logger.Debug("Extracting images from PDF")
		imageDir := filepath.Join(request.OutputDir, baseName+"_images")
		if err := os.MkdirAll(imageDir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create image directory: %w", err)
		}

		// Extract images using pdfcpu
		err := api.ExtractImagesFile(request.FilePath, imageDir, selectedPageStrings, conf)
		if err != nil {
			logger.WithError(err).Warn("Failed to extract images, continuing without images")
		} else {
			// Get list of extracted images
			extractedImages, err = t.getExtractedImageFiles(imageDir)
			if err != nil {
				logger.WithError(err).Warn("Failed to list extracted images")
			} else {
				logger.WithField("image_count", len(extractedImages)).Debug("Images extracted successfully")
			}
		}
	}

	// Extract content from each page
	logger.Debug("Extracting content from PDF pages")
	for _, pageNum := range selectedPages {
		pageContent, err := t.extractPageContent(request.FilePath, pageNum, conf, logger)
		if err != nil {
			logger.WithError(err).WithField("page", pageNum).Error("Failed to extract content from page")
			markdownContent.WriteString(fmt.Sprintf("## Page %d\n\n*Content extraction failed: %v*\n\n", pageNum, err))
			continue
		}

		// Add page header
		markdownContent.WriteString(fmt.Sprintf("## Page %d\n\n", pageNum))

		// Process and add content
		processedContent := t.processPageContent(pageContent)
		markdownContent.WriteString(processedContent)
		markdownContent.WriteString("\n\n")

		// Add any images for this page
		pageImages := t.getImagesForPage(extractedImages, pageNum)
		for _, imagePath := range pageImages {
			relativeImagePath, _ := filepath.Rel(request.OutputDir, imagePath)
			markdownContent.WriteString(fmt.Sprintf("![Image from page %d](%s)\n\n", pageNum, relativeImagePath))
		}
	}

	// Security content analysis for extracted text
	markdownContentStr := markdownContent.String()
	source := security.SourceContext{
		Tool:        "pdf",
		URL:         request.FilePath,
		ContentType: "extracted_text",
	}
	if result, err := security.AnalyseContent(markdownContentStr, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, fmt.Errorf("content blocked by security policy: %s", result.Message)
		case security.ActionWarn:
			// Add security warning to logs
			logger.WithField("security_id", result.ID).Warn(result.Message)
		}
	}

	// Write markdown file
	err = os.WriteFile(markdownFile, []byte(markdownContentStr), 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to write markdown file: %w", err)
	}

	// Prepare response
	response := &PDFResponse{
		FilePath:        request.FilePath,
		MarkdownFile:    markdownFile,
		ExtractedImages: extractedImages,
		PagesProcessed:  len(selectedPages),
		TotalPages:      pageCount,
		OutputDir:       request.OutputDir,
	}

	return response, nil
}

// ParsePageSelection parses page selection string into a slice of page numbers
func (t *PDFTool) ParsePageSelection(pages string, maxPage int) ([]int, error) {
	if pages == "" || pages == "all" {
		result := make([]int, maxPage)
		for i := range maxPage {
			result[i] = i + 1
		}
		return result, nil
	}

	var result []int
	parts := strings.SplitSeq(pages, ",")

	for part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			// Range: "1-5"
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range format: %s", part)
			}

			start := 0
			end := 0
			if _, err := fmt.Sscanf(rangeParts[0], "%d", &start); err != nil {
				return nil, fmt.Errorf("invalid start page: %s", rangeParts[0])
			}
			if _, err := fmt.Sscanf(rangeParts[1], "%d", &end); err != nil {
				return nil, fmt.Errorf("invalid end page: %s", rangeParts[1])
			}

			if start < 1 || end > maxPage || start > end {
				return nil, fmt.Errorf("invalid page range: %d-%d (max page: %d)", start, end, maxPage)
			}

			for i := start; i <= end; i++ {
				result = append(result, i)
			}
		} else {
			// Single page: "3"
			page := 0
			if _, err := fmt.Sscanf(part, "%d", &page); err != nil {
				return nil, fmt.Errorf("invalid page number: %s", part)
			}

			if page < 1 || page > maxPage {
				return nil, fmt.Errorf("page number out of range: %d (max page: %d)", page, maxPage)
			}

			result = append(result, page)
		}
	}

	// Remove duplicates and sort
	pageSet := make(map[int]bool)
	for _, page := range result {
		pageSet[page] = true
	}

	result = make([]int, 0, len(pageSet))
	for page := range pageSet {
		result = append(result, page)
	}
	sort.Ints(result)

	return result, nil
}

// extractPageContent extracts content from a specific page
func (t *PDFTool) extractPageContent(filePath string, pageNum int, conf *model.Configuration, logger *logrus.Logger) (string, error) {
	logger.WithField("page", pageNum).Debug("Starting text extraction for page")

	// Create temporary directory for text extraction
	tempDir, err := os.MkdirTemp("", "pdfcpu_text_*")
	if err != nil {
		logger.WithError(err).Error("Failed to create temp directory for text extraction")
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			logger.WithError(err).Warn("Failed to clean up temp directory")
		}
	}()

	// Extract content for this page using ExtractContentFile
	pageSelection := []string{strconv.Itoa(pageNum)}
	logger.WithFields(logrus.Fields{
		"page":      pageNum,
		"temp_dir":  tempDir,
		"file_path": filePath,
	}).Debug("Calling pdfcpu ExtractContentFile")

	err = api.ExtractContentFile(filePath, tempDir, pageSelection, conf)
	if err != nil {
		logger.WithError(err).WithField("page", pageNum).Error("Failed to extract content from page")
		return "", fmt.Errorf("failed to extract content: %w", err)
	}

	// List all files in temp directory to see what was actually created
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		logger.WithError(err).Error("Failed to read temp directory")
	} else {
		var fileNames []string
		for _, entry := range entries {
			fileNames = append(fileNames, entry.Name())
		}
		logger.WithFields(logrus.Fields{
			"temp_dir": tempDir,
			"files":    fileNames,
		}).Debug("Files created in temp directory")
	}

	// Read the extracted content file
	baseName := strings.TrimSuffix(filepath.Base(filePath), ".pdf")
	contentFile := filepath.Join(tempDir, fmt.Sprintf("%s_Content_page_%d.txt", baseName, pageNum))

	logger.WithField("content_file", contentFile).Debug("Reading extracted content file")

	contentBytes, err := os.ReadFile(contentFile)
	if err != nil {
		logger.WithError(err).WithField("content_file", contentFile).Error("Failed to read extracted content file")
		return "", fmt.Errorf("failed to read content file: %w", err)
	}

	content := string(contentBytes)
	logger.WithFields(logrus.Fields{
		"page":         pageNum,
		"content_size": len(content),
		"has_content":  len(strings.TrimSpace(content)) > 0,
	}).Debug("Text extraction completed for page")

	return content, nil
}

// processPageContent processes raw PDF content into more readable markdown
func (t *PDFTool) processPageContent(content string) string {
	if content == "" {
		return "*No text content found on this page*"
	}

	// Extract all text from PDF text show operations
	extractedTexts := t.extractAllTextFromPDFContent(content)

	if len(extractedTexts) == 0 {
		// If no text operations found, try to extract any readable text
		readableText := t.extractReadableText(content)
		if readableText != "" {
			return readableText
		}

		// As a last resort, show the raw content for debugging
		return "*Raw PDF content could not be processed into readable text*\n\n```\n" + content + "\n```"
	}

	// Join extracted texts with spaces and clean up
	result := strings.Join(extractedTexts, " ")
	result = t.cleanupExtractedText(result)

	return result
}

// extractAllTextFromPDFContent extracts all text strings from PDF content operations
func (t *PDFTool) extractAllTextFromPDFContent(content string) []string {
	var texts []string
	lines := strings.SplitSeq(content, "\n")

	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for text show operations: Tj, TJ, ', "
		if strings.Contains(line, " Tj") || strings.Contains(line, " TJ") ||
			strings.Contains(line, "' ") || strings.Contains(line, "\" ") {

			// Extract text from this line
			lineTexts := t.ExtractTextFromPDFOperation(line)
			for _, text := range lineTexts {
				if text != "" {
					texts = append(texts, text)
				}
			}
		}
	}

	return texts
}

// ExtractTextFromPDFOperation extracts all text strings from a PDF operation line
func (t *PDFTool) ExtractTextFromPDFOperation(operation string) []string {
	var texts []string
	inText := false
	start := -1

	for i, char := range operation {
		if char == '(' && (i == 0 || operation[i-1] != '\\') {
			inText = true
			start = i + 1
		} else if char == ')' && inText && (i == 0 || operation[i-1] != '\\') {
			if start != -1 && start < i {
				text := operation[start:i]
				// Basic cleanup of PDF escape sequences
				text = strings.ReplaceAll(text, "\\(", "(")
				text = strings.ReplaceAll(text, "\\)", ")")
				text = strings.ReplaceAll(text, "\\\\", "\\")
				text = strings.ReplaceAll(text, "\\n", "\n")
				text = strings.ReplaceAll(text, "\\r", "\r")
				text = strings.ReplaceAll(text, "\\t", "\t")

				if strings.TrimSpace(text) != "" {
					texts = append(texts, text)
				}
			}
			inText = false
			start = -1
		}
	}

	return texts
}

// extractReadableText attempts to extract any readable text from content
func (t *PDFTool) extractReadableText(content string) string {
	lines := strings.Split(content, "\n")
	var readableLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip obvious PDF commands and operators
		if t.isPDFCommand(line) {
			continue
		}

		// If line contains mostly readable characters, include it
		if t.isReadableText(line) {
			readableLines = append(readableLines, line)
		}
	}

	if len(readableLines) > 0 {
		return strings.Join(readableLines, " ")
	}

	return ""
}

// isPDFCommand checks if a line is a PDF command/operator
func (t *PDFTool) isPDFCommand(line string) bool {
	// Common PDF operators and commands
	pdfCommands := []string{
		"BT", "ET", "Tf", "Td", "TD", "Tm", "T*", "Tj", "TJ", "'", "\"",
		"q", "Q", "cm", "w", "J", "j", "M", "d", "ri", "i", "gs",
		"CS", "cs", "SC", "SCN", "sc", "scn", "G", "g", "RG", "rg", "K", "k",
		"m", "l", "c", "v", "y", "h", "re", "S", "s", "f", "F", "f*", "B", "B*", "b", "b*", "n",
		"W", "W*", "BX", "EX", "MP", "DP", "BMC", "BDC", "EMC",
	}

	words := strings.Fields(line)
	if len(words) == 0 {
		return false
	}

	// Check if line ends with a PDF command
	lastWord := words[len(words)-1]
	if slices.Contains(pdfCommands, lastWord) {
		return true
	}

	// Check if line is mostly numbers and operators (coordinates, etc.)
	nonNumericCount := 0
	for _, word := range words {
		if _, err := strconv.ParseFloat(word, 64); err != nil {
			nonNumericCount++
		}
	}

	// If most words are numeric, it's likely a PDF command
	return float64(nonNumericCount)/float64(len(words)) < 0.3
}

// isReadableText checks if a line contains readable text
func (t *PDFTool) isReadableText(line string) bool {
	if len(line) < 2 {
		return false
	}

	// Count alphabetic characters
	alphaCount := 0
	for _, char := range line {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			alphaCount++
		}
	}

	// Must have at least 30% alphabetic characters
	return float64(alphaCount)/float64(len(line)) >= 0.3
}

// cleanupExtractedText cleans up and formats extracted text
func (t *PDFTool) cleanupExtractedText(text string) string {
	// Remove excessive whitespace
	text = strings.TrimSpace(text)

	// Handle octal escape sequences (like \037, \260)
	text = t.processOctalEscapes(text)

	// Remove or replace binary/control characters
	text = t.removeBinaryCharacters(text)

	// Replace multiple spaces with single space
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	// Basic sentence formatting
	text = strings.ReplaceAll(text, " .", ".")
	text = strings.ReplaceAll(text, " ,", ",")
	text = strings.ReplaceAll(text, " !", "!")
	text = strings.ReplaceAll(text, " ?", "?")

	return text
}

// processOctalEscapes converts octal escape sequences to readable characters or removes them
func (t *PDFTool) processOctalEscapes(text string) string {
	// Common octal escape sequences in PDFs
	replacements := map[string]string{
		"\\037": "",   // Unit separator (remove)
		"\\260": "°",  // Degree symbol
		"\\256": "®",  // Registered trademark
		"\\251": "©",  // Copyright symbol
		"\\231": "'",  // Right single quotation mark
		"\\221": "'",  // Left single quotation mark
		"\\223": "\"", // Left double quotation mark
		"\\224": "\"", // Right double quotation mark
		"\\226": "–",  // En dash
		"\\227": "—",  // Em dash
		"\\240": " ",  // Non-breaking space
		"\\012": "\n", // Line feed
		"\\015": "\r", // Carriage return
		"\\011": "\t", // Tab
	}

	for octal, replacement := range replacements {
		text = strings.ReplaceAll(text, octal, replacement)
	}

	// Handle any remaining octal sequences (3-digit octal like \123)
	result := strings.Builder{}
	i := 0
	for i < len(text) {
		if i < len(text)-3 && text[i] == '\\' &&
			text[i+1] >= '0' && text[i+1] <= '7' &&
			text[i+2] >= '0' && text[i+2] <= '7' &&
			text[i+3] >= '0' && text[i+3] <= '7' {
			// Skip octal escape sequences we don't recognize
			i += 4
		} else {
			result.WriteByte(text[i])
			i++
		}
	}

	return result.String()
}

// removeBinaryCharacters removes or replaces binary/control characters
func (t *PDFTool) removeBinaryCharacters(text string) string {
	result := strings.Builder{}

	for _, char := range text {
		// Keep printable ASCII characters, common Unicode characters, and whitespace
		if (char >= 32 && char <= 126) || // Printable ASCII
			char == '\n' || char == '\r' || char == '\t' || // Whitespace
			(char >= 160 && char <= 255) || // Extended ASCII
			(char >= 0x2000 && char <= 0x206F) || // General punctuation
			(char >= 0x00A0 && char <= 0x00FF) { // Latin-1 supplement
			result.WriteRune(char)
		} else if char < 32 {
			// Replace other control characters with space
			result.WriteRune(' ')
		}
		// Skip other binary characters entirely
	}

	return result.String()
}

// getExtractedImageFiles returns a list of image files in the given directory
func (t *PDFTool) getExtractedImageFiles(imageDir string) ([]string, error) {
	entries, err := os.ReadDir(imageDir)
	if err != nil {
		return nil, err
	}

	var images []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".bmp" {
			images = append(images, filepath.Join(imageDir, name))
		}
	}

	sort.Strings(images)
	return images, nil
}

// getImagesForPage returns images that belong to a specific page
func (t *PDFTool) getImagesForPage(allImages []string, pageNum int) []string {
	var pageImages []string
	pagePrefix := fmt.Sprintf("_page_%d_", pageNum)

	for _, imagePath := range allImages {
		if strings.Contains(filepath.Base(imagePath), pagePrefix) {
			pageImages = append(pageImages, imagePath)
		}
	}

	return pageImages
}

// newToolResultJSON creates a new tool result with JSON content
func (t *PDFTool) newToolResultJSON(data any) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// GetMaxFileSize returns the configured maximum file size in bytes
func (t *PDFTool) GetMaxFileSize() int64 {
	if sizeStr := os.Getenv(PDFMaxFileSizeEnvVar); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil && size > 0 {
			return size
		}
	}
	return DefaultMaxFileSize
}

// GetMaxMemoryLimit returns the configured maximum memory limit in bytes
func (t *PDFTool) GetMaxMemoryLimit() int64 {
	if limitStr := os.Getenv(PDFMaxMemoryLimitEnvVar); limitStr != "" {
		if limit, err := strconv.ParseInt(limitStr, 10, 64); err == nil && limit > 0 {
			return limit
		}
	}
	return DefaultMaxMemoryLimit
}

// ValidateFileSize validates that the file size is within limits
func (t *PDFTool) ValidateFileSize(fileSize int64) error {
	maxSize := t.GetMaxFileSize()
	if fileSize > maxSize {
		sizeMB := float64(fileSize) / (1024 * 1024)
		maxSizeMB := float64(maxSize) / (1024 * 1024)
		return fmt.Errorf("PDF file size %.1fMB exceeds maximum allowed size of %.1fMB (use %s environment variable to adjust limit)", sizeMB, maxSizeMB, PDFMaxFileSizeEnvVar)
	}
	return nil
}

// applyMemoryLimits applies memory limits to the PDF configuration
func (t *PDFTool) applyMemoryLimits(conf *model.Configuration) {
	// Note: pdfcpu doesn't have direct memory limit configuration
	// This function is a placeholder for future memory limiting implementation
	// For now, we rely on the file size limits to prevent excessive memory usage
	// since PDF processing memory usage is generally proportional to file size

	// Set stricter validation to prevent malformed PDFs from consuming excessive memory
	conf.ValidationMode = model.ValidationStrict

	// Note: Configuration struct doesn't expose OptimizationMode field
	// Memory limits are enforced through file size validation
}

// ProvideExtendedInfo provides detailed usage information for the PDF tool
func (t *PDFTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Extract text and images from entire PDF",
				Arguments: map[string]any{
					"file_path":      "/Users/username/documents/report.pdf",
					"extract_images": true,
				},
				ExpectedResult: "Creates a markdown file with extracted text and saves images to a subfolder, returns paths to generated files and processing statistics",
			},
			{
				Description: "Extract only text from specific pages",
				Arguments: map[string]any{
					"file_path":      "/Users/username/documents/manual.pdf",
					"pages":          "1-5,10,15-20",
					"extract_images": false,
				},
				ExpectedResult: "Extracts text only from pages 1-5, 10, and 15-20, creating a markdown file without image extraction",
			},
			{
				Description: "Extract to custom output directory",
				Arguments: map[string]any{
					"file_path":      "/Users/username/pdfs/research.pdf",
					"output_dir":     "/Users/username/extracted/research",
					"extract_images": true,
				},
				ExpectedResult: "Extracts text and images to the specified output directory instead of the default PDF location",
			},
			{
				Description: "Extract single page with images",
				Arguments: map[string]any{
					"file_path":      "/Users/username/docs/presentation.pdf",
					"pages":          "5",
					"extract_images": true,
				},
				ExpectedResult: "Extracts only page 5 with any images on that page, useful for extracting specific slides or sections",
			},
		},
		CommonPatterns: []string{
			"Start with text-only extraction (extract_images: false) to quickly preview content before full processing",
			"Use page ranges to extract specific sections rather than processing entire large documents",
			"Specify custom output_dir when working with multiple PDFs to keep extractions organised",
			"Use 'all' pages parameter (default) for comprehensive document processing",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "PDF file size exceeds maximum allowed size error",
				Solution: "The tool has size limits (default 200MB) for security. Use PDF_MAX_FILE_SIZE environment variable to increase the limit, or split large PDFs into smaller files.",
			},
			{
				Problem:  "Text extraction returns poor quality or garbled text",
				Solution: "Some PDFs (especially scanned documents) may have poor text extraction. The tool works best with text-based PDFs. For scanned documents, consider OCR preprocessing.",
			},
			{
				Problem:  "Invalid page range error",
				Solution: "Ensure page numbers are valid (1-based) and don't exceed the PDF's page count. Use format '1-5' for ranges, '1,3,5' for specific pages, or 'all' for entire document.",
			},
			{
				Problem:  "No images extracted despite extract_images: true",
				Solution: "The PDF may not contain extractable images, or images may be embedded in a format not supported. Check the generated markdown file for any extracted content.",
			},
			{
				Problem:  "Output directory permission errors",
				Solution: "Ensure the output directory exists and has write permissions. The tool will create subdirectories but needs write access to the parent directory.",
			},
		},
		ParameterDetails: map[string]string{
			"file_path":      "Absolute path to PDF file (required). Must end with .pdf extension. File size limits apply for security (configurable via PDF_MAX_FILE_SIZE).",
			"output_dir":     "Directory for extracted files (optional). Defaults to same directory as PDF. Tool creates markdown file and optional image subdirectory here.",
			"extract_images": "Whether to extract and save images (optional, default: false). Images saved to subfolder with references in markdown file.",
			"pages":          "Page selection (optional, default: 'all'). Supports ranges ('1-5'), lists ('1,3,5'), or 'all'. Pages are 1-based indexed.",
		},
		WhenToUse:    "Use for extracting text and images from text-based PDFs, converting PDFs to markdown format, extracting specific pages or sections, or processing documents for further analysis or conversion workflows.",
		WhenNotToUse: "Don't use for scanned PDFs that need OCR, password-protected PDFs, or extremely large files that exceed memory constraints. Not suitable for preserving complex formatting or interactive PDF features.",
	}
}

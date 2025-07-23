package pdf

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
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
func (t *PDFTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
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

	// Validate input file exists
	if _, err := os.Stat(request.FilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("PDF file does not exist: %s", request.FilePath)
	}

	// Create configuration
	conf := model.NewDefaultConfiguration()

	// Process the PDF
	result, err := t.processPDF(ctx, logger, request, conf)
	if err != nil {
		return t.newToolResultJSON(map[string]interface{}{
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
func (t *PDFTool) ParseRequest(args map[string]interface{}) (*PDFRequest, error) {
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
	if err := os.MkdirAll(request.OutputDir, 0755); err != nil {
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
		if err := os.MkdirAll(imageDir, 0755); err != nil {
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

	// Write markdown file
	err = os.WriteFile(markdownFile, []byte(markdownContent.String()), 0644)
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
		for i := 0; i < maxPage; i++ {
			result[i] = i + 1
		}
		return result, nil
	}

	var result []int
	parts := strings.Split(pages, ",")

	for _, part := range parts {
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
	lines := strings.Split(content, "\n")

	for _, line := range lines {
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
	for _, cmd := range pdfCommands {
		if lastWord == cmd {
			return true
		}
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
func (t *PDFTool) newToolResultJSON(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

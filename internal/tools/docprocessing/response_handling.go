package docprocessing

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/security"
)

// formatResponse formats the response for MCP output
func (t *DocumentProcessorTool) formatResponse(response *DocumentProcessingResponse) map[string]any {
	result := map[string]any{
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

// shouldSaveToFile determines if content should be saved to a file
func (t *DocumentProcessorTool) shouldSaveToFile(req *DocumentProcessingRequest) bool {
	// If return_inline_only is explicitly set to true, do not save to file
	if req.ReturnInlineOnly != nil && *req.ReturnInlineOnly {
		return false
	}
	// Default behaviour: save to file (return_inline_only=false by default)
	return true
}

// handleSaveToFile saves the converted content to the specified file and returns a success message
func (t *DocumentProcessorTool) handleSaveToFile(savePath string, response *DocumentProcessingResponse, securityNotice string) (*mcp.CallToolResult, error) {
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

	// Security: Check file access for save path
	if err := security.CheckFileAccess(savePath); err != nil {
		return nil, fmt.Errorf("save file access denied: %w", err)
	}

	// Create directory if it doesn't exist
	saveDir := filepath.Dir(savePath)
	if err := os.MkdirAll(saveDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create save directory %s: %w", saveDir, err)
	}

	// Write content to file
	if err := os.WriteFile(savePath, []byte(response.Content), 0600); err != nil {
		return nil, fmt.Errorf("failed to write content to %s: %w", savePath, err)
	}

	// Create success response
	result := map[string]any{
		"success":   true,
		"message":   "Content successfully exported to file",
		"save_path": savePath,
		"source":    response.Source,
		"cache_hit": response.CacheHit,
		"metadata": map[string]any{
			"file_size": len(response.Content),
		},
		"processing_info": response.ProcessingInfo,
	}

	// Include document metadata if available
	if response.Metadata != nil {
		if metadata, ok := result["metadata"].(map[string]any); ok {
			metadata["document_title"] = response.Metadata.Title
			metadata["document_author"] = response.Metadata.Author
			metadata["page_count"] = response.Metadata.PageCount
			metadata["word_count"] = response.Metadata.WordCount
		}
	}

	// Add security notice if present
	if securityNotice != "" {
		result["security_notice"] = securityNotice
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

	// Security: Check file access for source when generating save path
	if err := security.CheckFileAccess(source); err != nil {
		return "", fmt.Errorf("source file access denied: %w", err)
	}

	// Get directory and filename
	sourceDir := filepath.Dir(source)
	sourceFilename := filepath.Base(source)

	// Remove extension and add .md
	nameWithoutExt := strings.TrimSuffix(sourceFilename, filepath.Ext(sourceFilename))
	savePath := filepath.Join(sourceDir, nameWithoutExt+".md")

	return savePath, nil
}

// newToolResultJSON creates a new tool result with JSON content
func (t *DocumentProcessorTool) newToolResultJSON(data any) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

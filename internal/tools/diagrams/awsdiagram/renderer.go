package awsdiagram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-graphviz"
	"github.com/sirupsen/logrus"
)

// renderDiagram renders the DOT content to specified formats
func (t *AWSDiagramTool) renderDiagram(ctx context.Context, logger *logrus.Logger, dotContent string, diagram *DiagramSpec, outputFormats []string, filename, workspaceDir string) (map[string]interface{}, error) {
	// Validate output formats
	validFormats := map[string]graphviz.Format{
		"png": graphviz.PNG,
		"svg": graphviz.SVG,
		"pdf": graphviz.PNG,  // Use PNG instead of PDF for now
		"dot": graphviz.XDOT, // Use XDOT for better compatibility
	}

	for _, format := range outputFormats {
		if _, ok := validFormats[format]; !ok {
			return nil, fmt.Errorf("unsupported output format: %s", format)
		}
	}

	// Determine output directory
	outputDir, err := t.getOutputDirectory(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to determine output directory: %w", err)
	}

	// Generate filename if not provided
	if filename == "" {
		filename = t.generateFilename(diagram.Name)
	}

	// Create Graphviz instance
	g, err := graphviz.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Graphviz instance: %w", err)
	}
	defer func() {
		if err := g.Close(); err != nil {
			logger.WithError(err).Warn("Failed to close Graphviz instance")
		}
	}()

	// Parse DOT content
	graph, err := graphviz.ParseBytes([]byte(dotContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse DOT content: %w", err)
	}
	defer func() {
		if err := graph.Close(); err != nil {
			logger.WithError(err).Warn("Failed to close graph")
		}
	}()

	// Render to each requested format
	outputFiles := make(map[string]string)
	var primaryImageFile string

	for _, formatStr := range outputFormats {
		format := validFormats[formatStr]

		var outputPath string
		if formatStr == "dot" {
			outputPath = filepath.Join(outputDir, filename+".dot")
		} else {
			outputPath = filepath.Join(outputDir, filename+"."+formatStr)
		}

		logger.WithFields(logrus.Fields{
			"format":      formatStr,
			"output_path": outputPath,
		}).Info("Rendering diagram")

		// Set timeout for rendering
		renderCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()

		// Special handling for DOT format (just write the content)
		if formatStr == "dot" {
			if err := t.writeDOTFile(outputPath, dotContent); err != nil {
				return nil, fmt.Errorf("failed to write DOT file: %w", err)
			}
		} else {
			// Render using graphviz
			if err := g.RenderFilename(renderCtx, graph, format, outputPath); err != nil {
				return nil, fmt.Errorf("failed to render %s format: %w", formatStr, err)
			}
		}

		// Check if file was created successfully
		if _, err := os.Stat(outputPath); err != nil {
			return nil, fmt.Errorf("output file was not created: %s", outputPath)
		}

		outputFiles[formatStr] = outputPath

		// Track primary image file for thumbnail
		if primaryImageFile == "" && (formatStr == "png" || formatStr == "svg") {
			primaryImageFile = outputPath
		}

		logger.WithField("output_path", outputPath).Info("Successfully rendered diagram")
	}

	// Get file sizes for report
	fileSizes := make(map[string]int64)
	for format, path := range outputFiles {
		if stat, err := os.Stat(path); err == nil {
			fileSizes[format] = stat.Size()
		}
	}

	// Prepare result
	result := map[string]interface{}{
		"action":            "generate",
		"status":            "success",
		"diagram_name":      diagram.Name,
		"output_directory":  outputDir,
		"output_files":      outputFiles,
		"file_sizes":        fileSizes,
		"formats_generated": outputFormats,
		"dot_content":       dotContent,
		"generation_time":   time.Now().Format(time.RFC3339),
		"message":           fmt.Sprintf("Successfully generated diagram '%s' in %d format(s)", diagram.Name, len(outputFormats)),
	}

	// Add primary image path if available
	if primaryImageFile != "" {
		result["primary_image"] = primaryImageFile
	}

	return result, nil
}

// getOutputDirectory determines and creates the output directory
func (t *AWSDiagramTool) getOutputDirectory(workspaceDir string) (string, error) {
	var baseDir string

	// Use provided workspace directory or current working directory
	if workspaceDir != "" {
		baseDir = workspaceDir
	} else {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current working directory: %w", err)
		}
	}

	// Create "generated-diagrams" subdirectory
	outputDir := filepath.Join(baseDir, "generated-diagrams")

	// Check if directory exists and is writable
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		// Create directory with proper permissions
		if err := os.MkdirAll(outputDir, 0700); err != nil {
			return "", fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Verify we can write to the directory
	testFile := filepath.Join(outputDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		return "", fmt.Errorf("cannot write to output directory %s: %w", outputDir, err)
	}
	_ = os.Remove(testFile) // Clean up test file

	return outputDir, nil
}

// generateFilename creates a filename from the diagram name
func (t *AWSDiagramTool) generateFilename(diagramName string) string {
	// Clean up diagram name for filename
	filename := strings.ToLower(diagramName)

	// Replace spaces and special characters with underscores
	filename = strings.ReplaceAll(filename, " ", "_")
	filename = strings.ReplaceAll(filename, "-", "_")
	filename = strings.ReplaceAll(filename, ".", "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	// Remove any non-alphanumeric characters except underscores
	var cleaned strings.Builder
	for _, r := range filename {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			cleaned.WriteRune(r)
		}
	}

	result := cleaned.String()

	// Ensure filename is not empty
	if result == "" {
		result = "diagram"
	}

	// Ensure filename doesn't start with underscore
	result = strings.TrimLeft(result, "_")
	if result == "" {
		result = "diagram"
	}

	// Add timestamp to avoid conflicts
	timestamp := time.Now().Format("20060102_150405")
	result = fmt.Sprintf("%s_%s", result, timestamp)

	return result
}

// writeDOTFile writes DOT content to a file
func (t *AWSDiagramTool) writeDOTFile(outputPath, dotContent string) error {
	return os.WriteFile(outputPath, []byte(dotContent), 0600)
}

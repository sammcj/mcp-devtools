//go:build sbom_vuln_tools
// +build sbom_vuln_tools

package sbom

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gologger "github.com/anchore/go-logger"
	"github.com/anchore/go-logger/adapter/discard"
	logrusadapter "github.com/anchore/go-logger/adapter/logrus"
	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/format"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// SBOMTool implements SBOM generation using Anchore Syft
type SBOMTool struct{}

// Ensure we implement the interfaces
var _ tools.Tool = (*SBOMTool)(nil)
var _ tools.ExtendedHelpProvider = (*SBOMTool)(nil)

// init registers the SBOM tool
func init() {
	registry.Register(&SBOMTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *SBOMTool) Definition() mcp.Tool {
	tool := mcp.NewTool(
		"sbom",
		mcp.WithDescription("Generate Software Bill of Materials (SBOM) from source code projects using Syft. Analyses current project dependencies and components. Always saves to specified file and returns summary."),

		// Required parameters
		mcp.WithString("source",
			mcp.Required(),
			mcp.Description("Source to scan: absolute directory path (/path/to/project). Supports any directory containing source code with package managers (npm, pip, go.mod, etc.). Must be absolute path."),
		),

		// Optional parameters with sensible defaults
		mcp.WithString("output_format",
			mcp.Description("SBOM output format: 'syft-json' (Syft native format), 'cyclonedx-json' (CycloneDX standard), 'spdx-json' (SPDX standard), 'syft-table' (human readable)"),
			mcp.Enum("syft-json", "cyclonedx-json", "spdx-json", "syft-table"),
			mcp.DefaultString("syft-json"),
		),
		mcp.WithBoolean("include_dev_dependencies",
			mcp.Description("Include development dependencies in the SBOM (test frameworks, build tools, etc.)"),
			mcp.DefaultBool(false),
		),
		mcp.WithString("output_file",
			mcp.Required(),
			mcp.Description("Absolute file path to save SBOM output. Creates directories as needed."),
		),

		// Non-destructive writing annotations
		mcp.WithReadOnlyHintAnnotation(false),    // Creates new SBOM files
		mcp.WithDestructiveHintAnnotation(false), // Doesn't modify source code
		mcp.WithIdempotentHintAnnotation(false),  // May vary based on dependency versions
		mcp.WithOpenWorldHintAnnotation(true),    // May scan external package registries
	)
	return tool
}

// Execute executes the SBOM tool
func (t *SBOMTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Check if SBOM tool is enabled (disabled by default)
	if !tools.IsToolEnabled("sbom") {
		return nil, fmt.Errorf("SBOM tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'sbom'")
	}

	// Configure Syft logging to prevent stdout/stderr interference in stdio mode
	// Check if we're likely in stdio mode by checking log level (ErrorLevel = stdio mode)
	if logger.Level == logrus.ErrorLevel {
		// In stdio mode, disable Syft logging completely to prevent MCP protocol interference
		syft.SetLogger(discard.New())
	} else {
		// In non-stdio mode, allow minimal Syft logging to stderr
		stderrLogger, err := logrusadapter.New(logrusadapter.Config{
			EnableConsole: true,
			Level:         gologger.WarnLevel,
		})
		if err != nil {
			// Fallback to discard if logger creation fails
			syft.SetLogger(discard.New())
		} else {
			// Type assert to Controller to access SetOutput method
			if ctrl, ok := stderrLogger.(gologger.Controller); ok {
				ctrl.SetOutput(os.Stderr)
			}
			syft.SetLogger(stderrLogger)
		}
	}

	// Parse and validate parameters
	request, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"source":                   request.Source,
		"output_format":            request.OutputFormat,
		"include_dev_dependencies": request.IncludeDevDependencies,
		"output_file":              request.OutputFile,
	}).Debug("SBOM generation parameters")

	// Create context with reasonable timeout (3 minutes)
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	// Execute SBOM generation
	response, err := t.executeSyft(cmdCtx, request, logger)
	if err != nil {
		return nil, fmt.Errorf("SBOM generation failed: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"source":        request.Source,
		"output_format": request.OutputFormat,
		"package_count": response.PackageCount,
		"output_file":   response.OutputFile,
	}).Info("SBOM generation completed successfully")

	summary := fmt.Sprintf("SBOM generation completed successfully!\n\nDetails:\n- Source: %s\n- Format: %s\n- Packages found: %d\n- Output saved to: %s\n\nThe SBOM has been saved to the specified file and is ready for vulnerability scanning or compliance review.",
		response.Source, response.Format, response.PackageCount, response.OutputFile)

	return mcp.NewToolResultText(summary), nil
}

// SBOMRequest represents the parsed request parameters
type SBOMRequest struct {
	Source                 string `json:"source"`
	OutputFormat           string `json:"output_format"`
	IncludeDevDependencies bool   `json:"include_dev_dependencies"`
	OutputFile             string `json:"output_file"`
}

// SBOMResponse represents the SBOM generation response
type SBOMResponse struct {
	Content      string `json:"content"`
	Format       string `json:"format"`
	PackageCount int    `json:"package_count"`
	Source       string `json:"source"`
	OutputFile   string `json:"output_file,omitempty"`
}

// parseRequest parses and validates the tool arguments
func (t *SBOMTool) parseRequest(args map[string]interface{}) (*SBOMRequest, error) {
	// Parse source (required)
	source, ok := args["source"].(string)
	if !ok || source == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: source")
	}

	request := &SBOMRequest{
		Source:                 strings.TrimSpace(source),
		OutputFormat:           "syft-json", // Default
		IncludeDevDependencies: false,       // Default
	}

	// Parse output_format (optional)
	if outputFormatRaw, ok := args["output_format"].(string); ok {
		request.OutputFormat = outputFormatRaw
	}

	// Parse include_dev_dependencies (optional)
	if includeDevRaw, ok := args["include_dev_dependencies"].(bool); ok {
		request.IncludeDevDependencies = includeDevRaw
	}

	// Parse output_file (required)
	outputFile, ok := args["output_file"].(string)
	if !ok || outputFile == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: output_file")
	}
	request.OutputFile = strings.TrimSpace(outputFile)

	// Validate paths are absolute
	if err := t.validateAbsolutePaths(request); err != nil {
		return nil, err
	}

	// Security checks for file access
	if err := security.CheckFileAccess(request.Source); err != nil {
		return nil, err
	}
	if err := security.CheckFileAccess(request.OutputFile); err != nil {
		return nil, err
	}

	return request, nil
}

// validateAbsolutePaths validates that source and output_file paths are absolute
func (t *SBOMTool) validateAbsolutePaths(request *SBOMRequest) error {
	// Validate source path is absolute
	if !filepath.IsAbs(request.Source) {
		return fmt.Errorf("source path must be absolute: %s", request.Source)
	}

	// Validate output file path is absolute
	if !filepath.IsAbs(request.OutputFile) {
		return fmt.Errorf("output_file path must be absolute: %s", request.OutputFile)
	}

	return nil
}

// executeSyft executes Syft to generate the SBOM
func (t *SBOMTool) executeSyft(ctx context.Context, request *SBOMRequest, logger *logrus.Logger) (*SBOMResponse, error) {
	// Validate source path exists and is directory
	sourcePath := request.Source
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("source path does not exist: %s", sourcePath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path must be a directory: %s", sourcePath)
	}

	logger.WithField("source_path", sourcePath).Debug("Validated source path")

	// Get source using Syft's helper exactly like the working example
	logger.WithField("syft_input", sourcePath).Info("FIXED VERSION: About to call syft.GetSource without dir: prefix")
	src, err := syft.GetSource(ctx, sourcePath, nil)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
			"input": sourcePath,
		}).Error("syft.GetSource failed")
		return nil, fmt.Errorf("failed to create source from directory: %w", err)
	}
	logger.WithField("source_type", fmt.Sprintf("%T", src)).Info("Successfully created source")

	// Generate SBOM using Syft with nil config (like the simple example)
	logger.Debug("Starting SBOM generation with Syft")
	sbomResult, err := syft.CreateSBOM(ctx, src, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SBOM: %w", err)
	}

	logger.WithField("package_count", len(sbomResult.Artifacts.Packages.Sorted())).Debug("SBOM generation completed")

	// Get the appropriate encoder
	encoder := format.NewEncoderCollection(format.Encoders()...).GetByString(request.OutputFormat)
	if encoder == nil {
		return nil, fmt.Errorf("failed to get encoder for format: %s", request.OutputFormat)
	}

	// Format the SBOM
	content, err := format.Encode(*sbomResult, encoder)
	if err != nil {
		return nil, fmt.Errorf("failed to format SBOM: %w", err)
	}

	// Security content analysis for generated SBOM
	contentStr := string(content)
	source := security.SourceContext{
		Tool:        "sbom",
		URL:         request.Source,
		ContentType: "generated_sbom",
	}
	if result, err := security.AnalyseContent(contentStr, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, fmt.Errorf("content blocked by security policy: %s", result.Message)
		case security.ActionWarn:
			// Add security warning to logs
			logger.WithField("security_id", result.ID).Warn(result.Message)
		}
	}

	if err := t.writeToFile(request.OutputFile, contentStr); err != nil {
		return nil, fmt.Errorf("failed to write to file: %w", err)
	}

	response := &SBOMResponse{
		Content:      "", // Don't include full content in response
		Format:       request.OutputFormat,
		PackageCount: len(sbomResult.Artifacts.Packages.Sorted()),
		Source:       request.Source,
		OutputFile:   request.OutputFile,
	}

	return response, nil
}

// writeToFile writes content to the specified file path
func (t *SBOMTool) writeToFile(filePath, content string) error {
	// Validate output file path for security - must be absolute
	if !filepath.IsAbs(filePath) {
		return fmt.Errorf("output file path must be absolute: %s", filePath)
	}

	// Prevent path traversal in output file
	cleanPath := filepath.Clean(filePath)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid output path: contains path traversal elements")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write content to file
	if err := os.WriteFile(cleanPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write file %s: %w", cleanPath, err)
	}

	return nil
}

// ProvideExtendedInfo provides detailed usage information for the SBOM tool
func (t *SBOMTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Analyse project dependencies",
				Arguments: map[string]interface{}{
					"source": "/Users/user/my-project",
				},
				ExpectedResult: "Complete SBOM showing all production dependencies in the project directory",
			},
			{
				Description: "Generate SBOM for specific project directory",
				Arguments: map[string]interface{}{
					"source":      "/path/to/project",
					"output_file": "/Users/user/reports/project-sbom.json",
				},
				ExpectedResult: "SBOM saved to file, ready for vulnerability scanning or compliance review",
			},
			{
				Description: "Include development dependencies for comprehensive analysis",
				Arguments: map[string]interface{}{
					"source":                   "/Users/user/my-app",
					"include_dev_dependencies": true,
					"output_format":            "cyclonedx-json",
				},
				ExpectedResult: "CycloneDX SBOM including test frameworks, build tools, and linters for complete project view",
			},
			{
				Description: "Generate production-ready SBOM for deployment",
				Arguments: map[string]interface{}{
					"source":        "/Users/user/production-app",
					"output_format": "spdx-json",
					"output_file":   "/Users/user/compliance/production-sbom.spdx.json",
				},
				ExpectedResult: "SPDX-compliant SBOM with only production dependencies, suitable for compliance and security scanning",
			},
		},
		CommonPatterns: []string{
			"Use absolute paths for all file operations to ensure consistent behaviour",
			"Use syft-json format for subsequent vulnerability scanning workflows",
			"Include dev dependencies when you need complete project understanding",
			"Use cyclonedx-json or spdx-json for compliance and security toolchain integration",
			"Save to file when preparing for vulnerability scanning: sbom → vulnerability_scan workflow",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Path must be absolute error",
				Solution: "Use absolute paths for source and output_file parameters. Relative paths like './src' are not supported.",
			},
			{
				Problem:  "No packages found in SBOM output",
				Solution: "Ensure the source directory contains package manager files (package.json, go.mod, requirements.txt, pom.xml, Cargo.toml, etc.). The tool detects dependencies through these files.",
			},
			{
				Problem:  "SBOM generation takes too long",
				Solution: "Large projects may need more time. Exclude development dependencies for faster generation or ensure the directory contains relevant package files.",
			},
		},
		ParameterDetails: map[string]string{
			"source":                   "Absolute path to source code directory. Must contain package manager files for dependency detection.",
			"output_format":            "SBOM format: 'syft-json' (Syft native format), 'cyclonedx-json' (CycloneDX standard), 'spdx-json' (SPDX standard), 'syft-table' (human readable).",
			"include_dev_dependencies": "When true, includes test frameworks, build tools, linters, and other development-only dependencies in the SBOM.",
			"output_file":              "Absolute file path to save SBOM output. Creates directories as needed. Use for integration with vulnerability scanning workflows.",
		},
		WhenToUse:    "Use when you need to understand project dependencies, prepare for vulnerability scanning, generate compliance artifacts, or analyse software composition during development.",
		WhenNotToUse: "Don't use for container image analysis (use container-specific tools), binary analysis without source code, or when package manager files are unavailable.",
	}
}

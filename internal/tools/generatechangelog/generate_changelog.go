package generatechangelog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// GenerateChangelogTool implements changelog generation using Anchore Chronicle
type GenerateChangelogTool struct{}

// init registers the generate_changelog tool
func init() {
	registry.Register(&GenerateChangelogTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *GenerateChangelogTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"generate_changelog",
		mcp.WithDescription("Generate changelogs from GitHub PRs and issues. Analyses git repository and creates structured changelogs based on GitHub labels and semantic versioning."),

		// Required parameters
		mcp.WithString("repository_path",
			mcp.Required(),
			mcp.Description("Path to local Git repository (e.g., /path/to/repo, ., or ./my-project)"),
		),

		// Optional parameters with sensible defaults
		mcp.WithString("since_tag",
			mcp.Description("Starting git tag/commit for changelog range (auto-detects last release if not provided)"),
		),
		mcp.WithString("until_tag",
			mcp.Description("Ending git tag/commit for changelog range (defaults to HEAD)"),
		),
		mcp.WithString("output_format",
			mcp.Description("Changelog output format"),
			mcp.Enum("markdown", "json"),
			mcp.DefaultString("markdown"),
		),
		mcp.WithBoolean("speculate_next_version",
			mcp.Description("Predict the next semantic version based on change types"),
			mcp.DefaultBool(false),
		),
		mcp.WithString("title",
			mcp.Description("Title for the changelog document"),
			mcp.DefaultString("Changelog"),
		),
		mcp.WithString("output_file",
			mcp.Description("Optional file path to save changelog output (relative to current directory)"),
		),
		mcp.WithNumber("timeout_minutes",
			mcp.Description("Maximum execution time in minutes"),
			mcp.DefaultNumber(5),
		),
	)
}

// Execute executes the generate_changelog tool
func (t *GenerateChangelogTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Executing generate_changelog tool")

	// Parse and validate parameters
	request, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"repository_path":        request.RepositoryPath,
		"since_tag":              request.SinceTag,
		"until_tag":              request.UntilTag,
		"output_format":          request.OutputFormat,
		"speculate_next_version": request.SpeculateNextVersion,
		"timeout_minutes":        request.TimeoutMinutes,
	}).Debug("Generate changelog parameters")

	// Create context with timeout
	timeout := time.Duration(request.TimeoutMinutes) * time.Minute
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Validate repository
	repoPath, err := t.validateRepository(request.RepositoryPath)
	if err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}

	// Generate changelog
	response, err := t.executeChronicle(cmdCtx, logger, request, repoPath)
	if err != nil {
		// Return error information in a structured way
		errorResponse := map[string]interface{}{
			"repository_path": request.RepositoryPath,
			"error":           err.Error(),
			"timestamp":       time.Now(),
		}
		return t.newToolResultJSON(errorResponse)
	}

	logger.WithFields(logrus.Fields{
		"repository_path": request.RepositoryPath,
		"output_format":   response.Format,
		"change_count":    response.ChangeCount,
		"current_version": response.CurrentVersion,
		"next_version":    response.NextVersion,
		"content_length":  len(response.Content),
		"output_file":     response.OutputFile,
	}).Info("Generate changelog completed successfully")

	return t.newToolResultJSON(response)
}

// parseRequest parses and validates the tool arguments
func (t *GenerateChangelogTool) parseRequest(args map[string]interface{}) (*GenerateChangelogRequest, error) {
	// Parse repository_path (required)
	repoPath, ok := args["repository_path"].(string)
	if !ok || repoPath == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: repository_path")
	}

	request := &GenerateChangelogRequest{
		RepositoryPath:       strings.TrimSpace(repoPath),
		SinceTag:             "",
		UntilTag:             "",
		OutputFormat:         "markdown",
		SpeculateNextVersion: false,
		Title:                "Changelog",
		OutputFile:           "",
		TimeoutMinutes:       5,
	}

	// Parse optional parameters
	if sinceTag, ok := args["since_tag"].(string); ok && sinceTag != "" {
		request.SinceTag = strings.TrimSpace(sinceTag)
	}

	if untilTag, ok := args["until_tag"].(string); ok && untilTag != "" {
		request.UntilTag = strings.TrimSpace(untilTag)
	}

	if outputFormat, ok := args["output_format"].(string); ok {
		format := strings.ToLower(strings.TrimSpace(outputFormat))
		if format != "markdown" && format != "json" {
			return nil, fmt.Errorf("output_format must be 'markdown' or 'json'")
		}
		request.OutputFormat = format
	}

	if speculate, ok := args["speculate_next_version"].(bool); ok {
		request.SpeculateNextVersion = speculate
	}

	if title, ok := args["title"].(string); ok && title != "" {
		request.Title = strings.TrimSpace(title)
	}

	if outputFile, ok := args["output_file"].(string); ok && outputFile != "" {
		request.OutputFile = strings.TrimSpace(outputFile)
	}

	if timeoutRaw, ok := args["timeout_minutes"].(float64); ok {
		timeout := int(timeoutRaw)
		if timeout < 1 {
			return nil, fmt.Errorf("timeout_minutes must be at least 1")
		}
		if timeout > 60 {
			return nil, fmt.Errorf("timeout_minutes cannot exceed 60")
		}
		request.TimeoutMinutes = timeout
	}

	return request, nil
}

// validateRepository validates that the path is a git repository
func (t *GenerateChangelogTool) validateRepository(repoPath string) (string, error) {
	// Handle relative paths
	if !filepath.IsAbs(repoPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		repoPath = filepath.Join(cwd, repoPath)
	}

	// Clean the path
	repoPath = filepath.Clean(repoPath)

	// Check if directory exists
	info, err := os.Stat(repoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("repository path does not exist: %s", repoPath)
		}
		return "", fmt.Errorf("failed to access repository path: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("repository path is not a directory: %s", repoPath)
	}

	// Check if it's a git repository
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return "", fmt.Errorf("path is not a git repository (no .git directory found): %s", repoPath)
	}

	return repoPath, nil
}

// writeToFile writes content to a file
func (t *GenerateChangelogTool) writeToFile(filename, content string) error {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// newToolResultJSON creates a new tool result with JSON content
func (t *GenerateChangelogTool) newToolResultJSON(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// ProvideExtendedInfo provides detailed usage information for the generate_changelog tool
func (t *GenerateChangelogTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Generate changelog for current repository",
				Arguments: map[string]interface{}{
					"repository_path": ".",
				},
				ExpectedResult: "Generates changelog from last release to HEAD in markdown format",
			},
			{
				Description: "Generate changelog between specific tags",
				Arguments: map[string]interface{}{
					"repository_path": "./my-project",
					"since_tag":       "v1.0.0",
					"until_tag":       "v1.1.0",
				},
				ExpectedResult: "Generates changelog for changes between v1.0.0 and v1.1.0",
			},
			{
				Description: "Generate JSON changelog with version speculation",
				Arguments: map[string]interface{}{
					"repository_path":        ".",
					"output_format":          "json",
					"speculate_next_version": true,
				},
				ExpectedResult: "Returns structured JSON changelog with predicted next version based on change types",
			},
			{
				Description: "Save changelog to file",
				Arguments: map[string]interface{}{
					"repository_path": ".",
					"output_file":     "CHANGELOG.md",
					"title":           "Release Notes",
				},
				ExpectedResult: "Generates changelog and saves it to CHANGELOG.md with custom title",
			},
		},
		CommonPatterns: []string{
			"Use '.' for current directory when running from within a git repository",
			"Combine with version speculation for automated release planning",
			"Save to standard files like CHANGELOG.md or RELEASE_NOTES.md",
			"Use JSON format for programmatic processing in CI/CD pipelines",
			"Set timeout_minutes for repositories with extensive history",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Repository path is not a git repository error",
				Solution: "Ensure the path points to a directory with a .git folder. Run 'git init' if needed or check the path is correct.",
			},
			{
				Problem:  "GitHub token authentication errors",
				Solution: "Set GITHUB_TOKEN or CHRONICLE_GITHUB_TOKEN environment variable with a valid GitHub personal access token.",
			},
			{
				Problem:  "No changes found between tags",
				Solution: "Check that the specified tags exist and that there are actually commits between them. Use 'git log --oneline tag1..tag2' to verify.",
			},
			{
				Problem:  "Timeout errors with large repositories",
				Solution: "Increase timeout_minutes parameter or focus on a smaller commit range using since_tag and until_tag.",
			},
		},
		ParameterDetails: map[string]string{
			"repository_path":        "Path to git repository. Supports absolute paths, relative paths, and '.' for current directory. Must contain a .git directory.",
			"since_tag":              "Starting point for changelog generation. Can be a tag, branch, or commit hash. Auto-detects last release tag if not specified.",
			"until_tag":              "Ending point for changelog generation. Defaults to HEAD. Can be a tag, branch, or commit hash.",
			"output_format":          "Format for generated changelog. 'markdown' creates human-readable documentation, 'json' provides structured data.",
			"speculate_next_version": "When enabled, analyses change types (breaking, feature, bugfix) to predict the next semantic version number.",
			"output_file":            "Optional file path to save changelog. Creates directories as needed. Path is relative to current working directory.",
			"timeout_minutes":        "Maximum time to spend generating changelog. Increase for repositories with extensive history or slow GitHub API responses.",
		},
		WhenToUse:    "Use for automated changelog generation in release workflows, documentation updates, stakeholder communications, or when preparing release notes from GitHub PRs and issues.",
		WhenNotToUse: "Don't use for repositories without GitHub integration, private repositories without proper tokens, or when you need changelogs from commit messages rather than PRs/issues.",
	}
}

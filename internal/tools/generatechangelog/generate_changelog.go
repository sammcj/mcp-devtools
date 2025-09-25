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
	"github.com/sammcj/mcp-devtools/internal/security"
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
	tool := mcp.NewTool(
		"generate_changelog",
		mcp.WithDescription("Generate changelogs from git repositories using commit history. Requires absolute paths to local repositories (not URLs). Analyses git repository and creates structured changelogs based on commit patterns and semantic versioning. Supports optional GitHub integration for enhanced metadata and URLs."),

		// Required parameters
		mcp.WithString("repository_path",
			mcp.Required(),
			mcp.Description("Absolute path to local Git repository or subdirectory within one. Must be local filesystem path, not URL (e.g., /Users/username/project)"),
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
			mcp.Description("Optional file absolute path to save changelog output"),
		),
		mcp.WithBoolean("enable_github_integration",
			mcp.Description("Enable GitHub integration for enhanced changelog generation with PR/issue data"),
			mcp.DefaultBool(false),
		),

		// Non-destructive writing annotations
		mcp.WithReadOnlyHintAnnotation(false),    // Creates new changelog files
		mcp.WithDestructiveHintAnnotation(false), // Doesn't destroy existing data
		mcp.WithIdempotentHintAnnotation(false),  // Creates new content each run
		mcp.WithOpenWorldHintAnnotation(false),   // Works with local git repositories
	)
	return tool
}

// Execute executes the generate_changelog tool
func (t *GenerateChangelogTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	logger.Info("Executing generate_changelog tool")

	// Check if generate_changelog tool is enabled (disabled by default)
	if !tools.IsToolEnabled("generate_changelog") {
		return nil, fmt.Errorf("generate_changelog tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'generate_changelog'")
	}

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
		errorResponse := map[string]any{
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
func (t *GenerateChangelogTool) parseRequest(args map[string]any) (*GenerateChangelogRequest, error) {
	// Parse repository_path (required)
	repoPath, ok := args["repository_path"].(string)
	if !ok || repoPath == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: repository_path")
	}

	request := &GenerateChangelogRequest{
		RepositoryPath:          strings.TrimSpace(repoPath),
		SinceTag:                "",
		UntilTag:                "",
		OutputFormat:            "markdown",
		SpeculateNextVersion:    false,
		EnableGitHubIntegration: false,
		Title:                   "Changelog",
		OutputFile:              "",
		TimeoutMinutes:          2,
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

	if enableGitHub, ok := args["enable_github_integration"].(bool); ok {
		request.EnableGitHubIntegration = enableGitHub
	}

	if title, ok := args["title"].(string); ok && title != "" {
		request.Title = strings.TrimSpace(title)
	}

	if outputFile, ok := args["output_file"].(string); ok && outputFile != "" {
		request.OutputFile = strings.TrimSpace(outputFile)
		// Security check for output file access
		if err := security.CheckFileAccess(request.OutputFile); err != nil {
			return nil, err
		}
	}

	return request, nil
}

// validateRepository validates that the path is a git repository
func (t *GenerateChangelogTool) validateRepository(repoPath string) (string, error) {
	// Reject GitHub URLs and other non-local paths
	if strings.HasPrefix(repoPath, "http://") || strings.HasPrefix(repoPath, "https://") || strings.HasPrefix(repoPath, "git@") {
		return "", fmt.Errorf("repository_path must be a local file system path, not a URL. Use the github tool to clone repositories first, then provide the local path")
	}

	// Require absolute paths for MCP server context
	if !filepath.IsAbs(repoPath) {
		return "", fmt.Errorf("repository_path must be an absolute path (e.g., /Users/username/project), got: %s", repoPath)
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

	// Walk up directory tree to find .git directory (like git does)
	gitRepoPath, err := t.findGitRepository(repoPath)
	if err != nil {
		return "", fmt.Errorf("path is not within a git repository: %s (%w)", repoPath, err)
	}

	return gitRepoPath, nil
}

// findGitRepository walks up the directory tree to find the git repository root
func (t *GenerateChangelogTool) findGitRepository(startPath string) (string, error) {
	currentPath := startPath

	for {
		gitDir := filepath.Join(currentPath, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return currentPath, nil
		}

		// Move up one directory
		parentPath := filepath.Dir(currentPath)

		// Check if we've reached the root
		if parentPath == currentPath {
			return "", fmt.Errorf("no .git directory found")
		}

		currentPath = parentPath
	}
}

// writeToFile writes content to a file
func (t *GenerateChangelogTool) writeToFile(filename, content string) error {
	// Convert to absolute path if relative
	absPath := filename
	if !filepath.IsAbs(filename) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot resolve relative path %s: failed to get working directory: %w", filename, err)
		}
		absPath = filepath.Join(cwd, filename)
	}

	// Clean the path
	absPath = filepath.Clean(absPath)

	// Security check for file access
	if err := security.CheckFileAccess(absPath); err != nil {
		return err
	}

	// Security content analysis for generated changelog
	source := security.SourceContext{
		Tool:        "generate_changelog",
		URL:         absPath,
		ContentType: "generated_changelog",
	}
	if result, err := security.AnalyseContent(content, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return fmt.Errorf("content blocked by security policy: %s", result.Message)
		case security.ActionWarn:
			// Add security warning to logs
			logrus.WithField("security_id", result.ID).Warn(result.Message)
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s (consider using absolute path): %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(absPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write file %s: %w", absPath, err)
	}

	return nil
}

// newToolResultJSON creates a new tool result with JSON content
func (t *GenerateChangelogTool) newToolResultJSON(data any) (*mcp.CallToolResult, error) {
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
				Arguments: map[string]any{
					"repository_path": "/Users/username/my-project",
				},
				ExpectedResult: "Generates changelog from last release to HEAD in markdown format",
			},
			{
				Description: "Generate changelog between specific tags",
				Arguments: map[string]any{
					"repository_path": "/Users/username/my-project",
					"since_tag":       "v1.0.0",
					"until_tag":       "v1.1.0",
				},
				ExpectedResult: "Generates changelog for changes between v1.0.0 and v1.1.0",
			},
			{
				Description: "Generate JSON changelog with version speculation",
				Arguments: map[string]any{
					"repository_path":        "/Users/username/my-project",
					"output_format":          "json",
					"speculate_next_version": true,
				},
				ExpectedResult: "Returns structured JSON changelog with predicted next version based on change types",
			},
			{
				Description: "Save changelog to file",
				Arguments: map[string]any{
					"repository_path": "/Users/username/my-project",
					"output_file":     "/Users/username/my-project/CHANGELOG.md",
					"title":           "Release Notes",
				},
				ExpectedResult: "Generates changelog and saves it to CHANGELOG.md with custom title",
			},
		},
		CommonPatterns: []string{
			"Use absolute paths to local repositories",
			"Works from any subdirectory within a git repository",
			"Combine with version speculation for automated release planning",
			"Save to standard files like CHANGELOG.md",
			"Use JSON format for programmatic processing in CI/CD pipelines",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Repository path is not a git repository error",
				Solution: "Ensure the path points to a directory with a .git folder. Run 'git init' if needed or check the path is correct.",
			},
			{
				Problem:  "No changes found between tags",
				Solution: "Check that the specified tags exist and that there are actually commits between them. Use 'git log --oneline tag1..tag2' to verify.",
			},
			{
				Problem:  "Timeout errors with large repositories",
				Solution: "Focus on a smaller commit range using since_tag and until_tag parameters.",
			},
		},
		ParameterDetails: map[string]string{
			"repository_path":           "Absolute path to git repository. Must contain a .git directory (e.g., /Users/username/project).",
			"since_tag":                 "Starting point for changelog generation. Can be a tag, branch, or commit hash. Auto-detects last release tag if not specified.",
			"until_tag":                 "Ending point for changelog generation. Defaults to HEAD. Can be a tag, branch, or commit hash.",
			"output_format":             "Format for generated changelog. 'markdown' creates human-readable documentation, 'json' provides structured data.",
			"speculate_next_version":    "When enabled, analyses change types (breaking, feature, bugfix) to predict the next semantic version number.",
			"enable_github_integration": "Enable GitHub integration for enhanced changelog generation with PR/issue data. Requires GITHUB_TOKEN environment variable.",
			"output_file":               "Optional file absolute path to save changelog. Creates directories as needed.",
		},
		WhenToUse:    "Use for automated changelog generation from local git repositories, release workflows, documentation updates, or when preparing release notes from commit history.",
		WhenNotToUse: "Don't use with URLs - only works with local filesystem paths. Avoid for repositories without commit history or when you need custom formatting beyond markdown/JSON.",
	}
}

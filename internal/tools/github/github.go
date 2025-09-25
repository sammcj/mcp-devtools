package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// GitHubTool implements GitHub functionality for MCP
type GitHubTool struct{}

// init registers the GitHub tool
func init() {
	registry.Register(&GitHubTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *GitHubTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"github",
		mcp.WithDescription(`Access GitHub repositories, issues, PRs and workflows.

Functions and their required parameters:

• search_repositories: options.query (r), repository (o), options.limit (o)
• search_issues: repository (r), options.query (o), options.limit (o)
• search_pull_requests: repository (r), options.query (o), options.limit (o)
• get_issue: repository (r), options.number (required unless repository contains full issue URL), options.include_comments (o)
• get_pull_request: repository (r), options.number (required unless repository contains full PR URL), options.include_comments (o)
• get_file_contents: repository (r), options.paths (r), options.ref (o) - Returns partial results even if some files fail
• list_directory: repository (r), options.path (optional, defaults to root), options.ref (o) - Lists directory contents to explore repository structure
• clone_repository: repository (r), options.local_path (o)
• get_workflow_run: repository (r), options.run_id (required unless repository contains full workflow URL), options.include_logs (o)

(o) = optional
(r) = required

Repository parameter accepts: owner/repo, GitHub URLs, or full issue/PR/workflow URLs.`),
		mcp.WithString("function",
			mcp.Required(),
			mcp.Description("Function to execute"),
			mcp.Enum("search_repositories", "search_issues", "search_pull_requests", "get_issue", "get_pull_request", "get_file_contents", "list_directory", "clone_repository", "get_workflow_run"),
		),
		mcp.WithString("repository",
			mcp.Description("Repository identifier: owner/repo, GitHub URL, or full URL for specific issue/PR/workflow"),
		),
		mcp.WithObject("options",
			mcp.Description("Function-specific options - see function description for parameters"),
			mcp.Properties(map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query string (for search_repositories, search_issues, search_pull_requests)",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of results (for search functions, default: 30)",
					"default":     30,
				},
				"number": map[string]any{
					"type":        "number",
					"description": "Issue or PR number (required for get_issue/get_pull_request unless using full URL)",
				},
				"run_id": map[string]any{
					"type":        "number",
					"description": "Workflow run ID (required for get_workflow_run unless using full URL)",
				},
				"include_comments": map[string]any{
					"type":        "boolean",
					"description": "Include comments for issues/PRs (default: false)",
					"default":     false,
				},
				"include_logs": map[string]any{
					"type":        "boolean",
					"description": "Include workflow run logs (default: false)",
					"default":     false,
				},
				"include_closed": map[string]any{
					"type":        "boolean",
					"description": "Include closed issues/PRs in search results (default: false, only open)",
					"default":     false,
				},
				"paths": map[string]any{
					"type":        "array",
					"description": "Array of file paths to retrieve (required for get_file_contents)",
					"items": map[string]any{
						"type": "string",
					},
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path to list (optional for list_directory, defaults to root)",
				},
				"ref": map[string]any{
					"type":        "string",
					"description": "Git reference - branch, tag, or commit SHA (optional for get_file_contents)",
				},
				"local_path": map[string]any{
					"type":        "string",
					"description": "Local directory path for cloning (optional for clone_repository)",
				},
			}),
		),
		// Destructive tool annotations
		mcp.WithReadOnlyHintAnnotation(false),   // Can modify GitHub repositories via cloning and API calls
		mcp.WithDestructiveHintAnnotation(true), // Can clone repositories and potentially modify GitHub resources
		mcp.WithIdempotentHintAnnotation(false), // GitHub operations are generally not idempotent
		mcp.WithOpenWorldHintAnnotation(true),   // Makes external GitHub API calls
	)
}

// Execute executes the GitHub tool
func (t *GitHubTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Parse and validate parameters
	request, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Create GitHub client
	client, err := NewGitHubClientWrapper(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Execute the requested function
	switch request.Function {
	case "search_repositories":
		return t.handleSearchRepositories(ctx, client, request)
	case "search_issues":
		return t.handleSearchIssues(ctx, client, request)
	case "search_pull_requests":
		return t.handleSearchPullRequests(ctx, client, request)
	case "get_issue":
		return t.handleGetIssue(ctx, client, request)
	case "get_pull_request":
		return t.handleGetPullRequest(ctx, client, request)
	case "get_file_contents":
		return t.handleGetFileContents(ctx, client, request)
	case "list_directory":
		return t.handleListDirectory(ctx, client, request)
	case "clone_repository":
		return t.handleCloneRepository(ctx, client, request)
	case "get_workflow_run":
		return t.handleGetWorkflowRun(ctx, client, request)
	default:
		return nil, fmt.Errorf("unsupported function: %s", request.Function)
	}
}

// parseRequest parses and validates the request parameters
func (t *GitHubTool) parseRequest(args map[string]any) (*GitHubRequest, error) {
	function, ok := args["function"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: function")
	}

	repository := ""
	if repo, ok := args["repository"].(string); ok {
		repository = repo
	}

	options := make(map[string]any)
	if opts, ok := args["options"].(map[string]any); ok {
		options = opts
	}

	return &GitHubRequest{
		Function:   function,
		Repository: repository,
		Options:    options,
	}, nil
}

// handleSearchRepositories handles repository search
func (t *GitHubTool) handleSearchRepositories(ctx context.Context, client *GitHubClient, request *GitHubRequest) (*mcp.CallToolResult, error) {
	query, ok := request.Options["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query parameter is required for search_repositories")
	}

	limit := 30
	if l, ok := request.Options["limit"].(float64); ok {
		limit = int(l)
	}
	if limit > 100 {
		limit = 100 // GitHub API limit
	}

	result, err := client.SearchRepositories(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search repositories: %w", err)
	}

	response := map[string]any{
		"function": "search_repositories",
		"query":    query,
		"result":   result,
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
	}

	source := security.SourceContext{
		Tool:        "github",
		Domain:      "github.com",
		ContentType: "repository_search",
	}
	if result, err := security.AnalyseContent(jsonString, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, security.FormatSecurityBlockErrorFromResult(result)
		case security.ActionWarn:
			jsonString = security.FormatSecurityWarningPrefix(result) + jsonString
		}
	}

	return mcp.NewToolResultText(jsonString), nil
}

// handleSearchIssues handles issue search
func (t *GitHubTool) handleSearchIssues(ctx context.Context, client *GitHubClient, request *GitHubRequest) (*mcp.CallToolResult, error) {
	if request.Repository == "" {
		return nil, fmt.Errorf("repository parameter is required for search_issues")
	}

	owner, repo, err := ValidateRepository(request.Repository)
	if err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}

	query := ""
	if q, ok := request.Options["query"].(string); ok {
		query = q
	}

	limit := 30
	if l, ok := request.Options["limit"].(float64); ok {
		limit = int(l)
	}
	if limit > 100 {
		limit = 100
	}

	includeClosed := false
	if ic, ok := request.Options["include_closed"].(bool); ok {
		includeClosed = ic
	}

	result, err := client.SearchIssues(ctx, owner, repo, query, limit, includeClosed)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	response := map[string]any{
		"function":   "search_issues",
		"repository": fmt.Sprintf("%s/%s", owner, repo),
		"query":      query,
		"result":     result,
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
	}

	source := security.SourceContext{
		Tool:        "github",
		Domain:      "github.com",
		ContentType: "issue_search",
	}
	if result, err := security.AnalyseContent(jsonString, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, security.FormatSecurityBlockErrorFromResult(result)
		case security.ActionWarn:
			jsonString = security.FormatSecurityWarningPrefix(result) + jsonString
		}
	}

	return mcp.NewToolResultText(jsonString), nil
}

// handleSearchPullRequests handles pull request search
func (t *GitHubTool) handleSearchPullRequests(ctx context.Context, client *GitHubClient, request *GitHubRequest) (*mcp.CallToolResult, error) {
	if request.Repository == "" {
		return nil, fmt.Errorf("repository parameter is required for search_pull_requests")
	}

	owner, repo, err := ValidateRepository(request.Repository)
	if err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}

	query := ""
	if q, ok := request.Options["query"].(string); ok {
		query = q
	}

	limit := 30
	if l, ok := request.Options["limit"].(float64); ok {
		limit = int(l)
	}
	if limit > 100 {
		limit = 100
	}

	includeClosed := false
	if ic, ok := request.Options["include_closed"].(bool); ok {
		includeClosed = ic
	}

	result, err := client.SearchPullRequests(ctx, owner, repo, query, limit, includeClosed)
	if err != nil {
		return nil, fmt.Errorf("failed to search pull requests: %w", err)
	}

	response := map[string]any{
		"function":   "search_pull_requests",
		"repository": fmt.Sprintf("%s/%s", owner, repo),
		"query":      query,
		"result":     result,
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
	}

	source := security.SourceContext{
		Tool:        "github",
		Domain:      "github.com",
		ContentType: "pull_request_search",
	}
	if result, err := security.AnalyseContent(jsonString, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, security.FormatSecurityBlockErrorFromResult(result)
		case security.ActionWarn:
			jsonString = security.FormatSecurityWarningPrefix(result) + jsonString
		}
	}

	return mcp.NewToolResultText(jsonString), nil
}

// handleGetIssue handles getting a specific issue
func (t *GitHubTool) handleGetIssue(ctx context.Context, client *GitHubClient, request *GitHubRequest) (*mcp.CallToolResult, error) {
	if request.Repository == "" {
		return nil, fmt.Errorf("repository parameter is required for get_issue")
	}

	var owner, repo string
	var issueNumber int
	var err error

	// Check if repository is a full issue URL
	if strings.Contains(request.Repository, "/issues/") {
		owner, repo, err = ValidateRepository(request.Repository)
		if err != nil {
			return nil, fmt.Errorf("invalid repository URL: %w", err)
		}
		issueNumber, err = ExtractIssueNumber(request.Repository)
		if err != nil {
			return nil, fmt.Errorf("failed to extract issue number: %w", err)
		}
	} else {
		owner, repo, err = ValidateRepository(request.Repository)
		if err != nil {
			return nil, fmt.Errorf("invalid repository: %w", err)
		}
		// Issue number should be provided in options
		if num, ok := request.Options["number"].(float64); ok {
			issueNumber = int(num)
		} else {
			return nil, fmt.Errorf("issue number is required (either in URL or options.number)")
		}
	}

	includeComments := false
	if ic, ok := request.Options["include_comments"].(bool); ok {
		includeComments = ic
	}

	issue, comments, err := client.GetIssue(ctx, owner, repo, issueNumber, includeComments)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	response := map[string]any{
		"function":   "get_issue",
		"repository": fmt.Sprintf("%s/%s", owner, repo),
		"issue":      issue,
	}

	if includeComments && len(comments) > 0 {
		response["comments"] = comments
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
	}

	source := security.SourceContext{
		Tool:        "github",
		Domain:      "github.com",
		ContentType: "issue_details",
	}
	if result, err := security.AnalyseContent(jsonString, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, security.FormatSecurityBlockErrorFromResult(result)
		case security.ActionWarn:
			jsonString = security.FormatSecurityWarningPrefix(result) + jsonString
		}
	}

	return mcp.NewToolResultText(jsonString), nil
}

// handleGetPullRequest handles getting a specific pull request
func (t *GitHubTool) handleGetPullRequest(ctx context.Context, client *GitHubClient, request *GitHubRequest) (*mcp.CallToolResult, error) {
	if request.Repository == "" {
		return nil, fmt.Errorf("repository parameter is required for get_pull_request")
	}

	var owner, repo string
	var prNumber int
	var err error

	// Check if repository is a full PR URL
	if strings.Contains(request.Repository, "/pull/") {
		owner, repo, err = ValidateRepository(request.Repository)
		if err != nil {
			return nil, fmt.Errorf("invalid repository URL: %w", err)
		}
		prNumber, err = ExtractPullRequestNumber(request.Repository)
		if err != nil {
			return nil, fmt.Errorf("failed to extract pull request number: %w", err)
		}
	} else {
		owner, repo, err = ValidateRepository(request.Repository)
		if err != nil {
			return nil, fmt.Errorf("invalid repository: %w", err)
		}
		// PR number should be provided in options
		if num, ok := request.Options["number"].(float64); ok {
			prNumber = int(num)
		} else {
			return nil, fmt.Errorf("pull request number is required (either in URL or options.number)")
		}
	}

	includeComments := false
	if ic, ok := request.Options["include_comments"].(bool); ok {
		includeComments = ic
	}

	pullRequest, comments, err := client.GetPullRequest(ctx, owner, repo, prNumber, includeComments)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request: %w", err)
	}

	response := map[string]any{
		"function":     "get_pull_request",
		"repository":   fmt.Sprintf("%s/%s", owner, repo),
		"pull_request": pullRequest,
	}

	if includeComments && len(comments) > 0 {
		response["comments"] = comments
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
	}

	source := security.SourceContext{
		Tool:        "github",
		Domain:      "github.com",
		ContentType: "pull_request_details",
	}
	if result, err := security.AnalyseContent(jsonString, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, security.FormatSecurityBlockErrorFromResult(result)
		case security.ActionWarn:
			jsonString = security.FormatSecurityWarningPrefix(result) + jsonString
		}
	}

	return mcp.NewToolResultText(jsonString), nil
}

// handleGetFileContents handles getting file contents with graceful error handling
func (t *GitHubTool) handleGetFileContents(ctx context.Context, client *GitHubClient, request *GitHubRequest) (*mcp.CallToolResult, error) {
	if request.Repository == "" {
		return nil, fmt.Errorf("repository parameter is required for get_file_contents")
	}

	owner, repo, err := ValidateRepository(request.Repository)
	if err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}

	// Get paths from options
	var paths []string
	if pathsRaw, ok := request.Options["paths"].([]any); ok {
		for _, path := range pathsRaw {
			if pathStr, ok := path.(string); ok {
				paths = append(paths, pathStr)
			}
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one file path is required in options.paths")
	}

	ref := ""
	if r, ok := request.Options["ref"].(string); ok {
		ref = r
	}

	// Get file results with graceful error handling
	fileResults, err := client.GetFileContents(ctx, owner, repo, paths, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get file contents: %w", err)
	}

	// Check if any files succeeded
	successCount := 0
	for _, result := range fileResults {
		if result.Success {
			successCount++
		}
	}

	response := map[string]any{
		"function":      "get_file_contents",
		"repository":    fmt.Sprintf("%s/%s", owner, repo),
		"ref":           ref,
		"files":         fileResults,
		"success_count": successCount,
		"total_count":   len(fileResults),
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
	}

	source := security.SourceContext{
		Tool:        "github",
		Domain:      "github.com",
		ContentType: "file_contents",
	}
	if result, err := security.AnalyseContent(jsonString, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, security.FormatSecurityBlockErrorFromResult(result)
		case security.ActionWarn:
			jsonString = security.FormatSecurityWarningPrefix(result) + jsonString
		}
	}

	return mcp.NewToolResultText(jsonString), nil
}

// handleListDirectory handles listing directory contents
func (t *GitHubTool) handleListDirectory(ctx context.Context, client *GitHubClient, request *GitHubRequest) (*mcp.CallToolResult, error) {
	if request.Repository == "" {
		return nil, fmt.Errorf("repository parameter is required for list_directory")
	}

	owner, repo, err := ValidateRepository(request.Repository)
	if err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}

	path := ""
	if p, ok := request.Options["path"].(string); ok {
		path = p
	}

	ref := ""
	if r, ok := request.Options["ref"].(string); ok {
		ref = r
	}

	listing, err := client.ListDirectory(ctx, owner, repo, path, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	response := map[string]any{
		"function":   "list_directory",
		"repository": fmt.Sprintf("%s/%s", owner, repo),
		"ref":        ref,
		"listing":    listing,
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
	}

	source := security.SourceContext{
		Tool:        "github",
		Domain:      "github.com",
		ContentType: "directory_listing",
	}
	if result, err := security.AnalyseContent(jsonString, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, security.FormatSecurityBlockErrorFromResult(result)
		case security.ActionWarn:
			jsonString = security.FormatSecurityWarningPrefix(result) + jsonString
		}
	}

	return mcp.NewToolResultText(jsonString), nil
}

// handleCloneRepository handles repository cloning
func (t *GitHubTool) handleCloneRepository(ctx context.Context, client *GitHubClient, request *GitHubRequest) (*mcp.CallToolResult, error) {
	if request.Repository == "" {
		return nil, fmt.Errorf("repository parameter is required for clone_repository")
	}

	owner, repo, err := ValidateRepository(request.Repository)
	if err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}

	localPath := ""
	if lp, ok := request.Options["local_path"].(string); ok {
		localPath = lp
	}

	// File access security is now handled by the clone operation itself
	// The helper functions will handle file access checks automatically

	result, err := client.CloneRepository(ctx, owner, repo, localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	response := map[string]any{
		"function": "clone_repository",
		"result":   result,
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
	}

	source := security.SourceContext{
		Tool:        "github",
		Domain:      "github.com",
		ContentType: "clone_result",
	}
	if result, err := security.AnalyseContent(jsonString, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, security.FormatSecurityBlockErrorFromResult(result)
		case security.ActionWarn:
			jsonString = security.FormatSecurityWarningPrefix(result) + jsonString
		}
	}

	return mcp.NewToolResultText(jsonString), nil
}

// handleGetWorkflowRun handles getting workflow run information
func (t *GitHubTool) handleGetWorkflowRun(ctx context.Context, client *GitHubClient, request *GitHubRequest) (*mcp.CallToolResult, error) {
	if request.Repository == "" {
		return nil, fmt.Errorf("repository parameter is required for get_workflow_run")
	}

	var owner, repo string
	var runID int64
	var err error

	// Check if repository is a full workflow run URL
	if strings.Contains(request.Repository, "/actions/runs/") {
		owner, repo, err = ValidateRepository(request.Repository)
		if err != nil {
			return nil, fmt.Errorf("invalid repository URL: %w", err)
		}
		runID, err = ExtractWorkflowRunID(request.Repository)
		if err != nil {
			return nil, fmt.Errorf("failed to extract workflow run ID: %w", err)
		}
	} else {
		owner, repo, err = ValidateRepository(request.Repository)
		if err != nil {
			return nil, fmt.Errorf("invalid repository: %w", err)
		}
		// Run ID should be provided in options
		if id, ok := request.Options["run_id"].(float64); ok {
			runID = int64(id)
		} else {
			return nil, fmt.Errorf("workflow run ID is required (either in URL or options.run_id)")
		}
	}

	includeLogs := false
	if il, ok := request.Options["include_logs"].(bool); ok {
		includeLogs = il
	}

	workflowRun, logs, err := client.GetWorkflowRun(ctx, owner, repo, runID, includeLogs)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow run: %w", err)
	}

	response := map[string]any{
		"function":     "get_workflow_run",
		"repository":   fmt.Sprintf("%s/%s", owner, repo),
		"workflow_run": workflowRun,
	}

	if includeLogs && logs != "" {
		response["logs"] = logs
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
	}

	source := security.SourceContext{
		Tool:        "github",
		Domain:      "github.com",
		ContentType: "workflow_run",
	}
	if result, err := security.AnalyseContent(jsonString, source); err == nil {
		switch result.Action {
		case security.ActionBlock:
			return nil, security.FormatSecurityBlockErrorFromResult(result)
		case security.ActionWarn:
			jsonString = security.FormatSecurityWarningPrefix(result) + jsonString
		}
	}

	return mcp.NewToolResultText(jsonString), nil
}

// convertToJSON converts the response to JSON string for better formatting
func (t *GitHubTool) convertToJSON(response any) (string, error) {
	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal response to JSON: %w", err)
	}
	return string(jsonBytes), nil
}

// ProvideExtendedInfo provides detailed usage information for the github tool
func (t *GitHubTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Search for repositories related to machine learning",
				Arguments: map[string]any{
					"function": "search_repositories",
					"options": map[string]any{
						"query": "machine learning python",
						"limit": 10,
					},
				},
				ExpectedResult: "Returns top 10 GitHub repositories matching 'machine learning python' with repository details, stars, and descriptions",
			},
			{
				Description: "List root directory contents to explore repository structure",
				Arguments: map[string]any{
					"function":   "list_directory",
					"repository": "anchore/chronicle",
				},
				ExpectedResult: "Returns list of files and directories in the repository root, helping you understand the project structure before requesting specific files",
			},
			{
				Description: "List contents of a specific directory",
				Arguments: map[string]any{
					"function":   "list_directory",
					"repository": "anchore/chronicle",
					"options": map[string]any{
						"path": "internal",
						"ref":  "main",
					},
				},
				ExpectedResult: "Returns list of files and directories inside the 'internal' directory on the main branch",
			},
			{
				Description: "Get file contents with graceful error handling",
				Arguments: map[string]any{
					"function":   "get_file_contents",
					"repository": "anchore/chronicle",
					"options": map[string]any{
						"paths": []string{"README.md", "nonexistent-file.go", "go.mod"},
					},
				},
				ExpectedResult: "Returns partial results showing success for README.md and go.mod, with detailed error message for nonexistent-file.go explaining how to verify file paths",
			},
			{
				Description: "Get details of a specific issue with comments",
				Arguments: map[string]any{
					"function":   "get_issue",
					"repository": "microsoft/vscode",
					"options": map[string]any{
						"number":           12345,
						"include_comments": true,
					},
				},
				ExpectedResult: "Returns issue #12345 from microsoft/vscode including full description, labels, assignees, and all comments",
			},
			{
				Description: "Get workflow run details with logs",
				Arguments: map[string]any{
					"function":   "get_workflow_run",
					"repository": "https://github.com/owner/repo/actions/runs/123456789",
					"options": map[string]any{
						"include_logs": true,
					},
				},
				ExpectedResult: "Returns workflow run details and complete execution logs for run ID 123456789",
			},
		},
		CommonPatterns: []string{
			"ALWAYS start with list_directory to explore repository structure before requesting specific files",
			"Use get_file_contents for multiple files - it handles partial failures gracefully",
			"Include specific refs (branches/tags) when analyzing version-specific code",
			"Start with root directory listing, then navigate to subdirectories as needed",
			"Use search functions to discover repositories, issues, and PRs before getting specific details",
			"Search with targeted queries to reduce result noise (e.g., 'is:open label:bug')",
			"When file requests fail, check the error suggestions and use list_directory to verify paths",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "File not found errors (404)",
				Solution: "Use list_directory to explore the repository structure first. Check the detailed error message for suggestions including browsing the repo on GitHub, verifying branch/ref, and checking for typos.",
			},
			{
				Problem:  "Some files succeed but others fail in get_file_contents",
				Solution: "This is expected behaviour - the tool returns partial results. Check the 'success' field for each file and read error messages for failed files. Use list_directory to verify paths for failed files.",
			},
			{
				Problem:  "Directory listing returns empty or unexpected results",
				Solution: "Verify the directory path exists and you're using the correct branch/ref. Try listing the root directory first (no path specified) then navigate to subdirectories.",
			},
			{
				Problem:  "Authentication errors or rate limits",
				Solution: "Ensure GITHUB_TOKEN environment variable is set with appropriate permissions. Public repositories require fewer permissions than private ones.",
			},
			{
				Problem:  "Repository not found errors",
				Solution: "Check repository name format (owner/repo), ensure repository exists and is public (or you have access), and verify spelling of repository name.",
			},
			{
				Problem:  "Workflow run access denied",
				Solution: "Workflow runs may require higher permissions than basic repository access. Ensure your GitHub token has 'actions:read' scope.",
			},
		},
		ParameterDetails: map[string]string{
			"function":   "The GitHub operation to perform. Use list_directory first to explore, then get_file_contents for specific files. Each function has different requirements - see examples.",
			"repository": "Repository identifier in 'owner/repo' format, full GitHub URLs, or URLs with issue/PR/workflow IDs. The tool automatically extracts relevant information.",
			"options":    "Function-specific parameters. For list_directory: path (optional), ref (optional). For get_file_contents: paths (required array), ref (optional). Always check examples for each function.",
		},
		WhenToUse:    "Use for GitHub repository exploration, file content examination with graceful error handling, repository structure analysis, issue tracking, PR review, and CI/CD debugging. Start with list_directory to understand repository layout.",
		WhenNotToUse: "Don't use for non-GitHub repositories, private repositories without proper authentication, or when you need to modify GitHub data (this tool is read-only). Use Git commands for actual repository cloning and manipulation.",
	}
}

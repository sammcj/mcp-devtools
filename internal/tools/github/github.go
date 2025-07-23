package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
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

• search_repositories: repository (o), options.query, options.limit (o)
• search_issues: repository (r), options.query (o), options.limit (o)
• search_pull_requests: repository (r), options.query (o), options.limit (o)
• get_issue: repository (r), options.number (required unless repository contains full issue URL), options.include_comments (o)
• get_pull_request: repository (r), options.number (required unless repository contains full PR URL), options.include_comments (o)
• get_file_contents: repository (r), options.paths (r), options.ref (o)
• clone_repository: repository (r), options.local_path (o)
• get_workflow_run: repository (r), options.run_id (required unless repository contains full workflow URL), options.include_logs (o)

(o) = optional
(r) = required

Repository parameter accepts: owner/repo, GitHub URLs, or full issue/PR/workflow URLs.`),
		mcp.WithString("function",
			mcp.Required(),
			mcp.Description("Function to execute"),
			mcp.Enum("search_repositories", "search_issues", "search_pull_requests", "get_issue", "get_pull_request", "get_file_contents", "clone_repository", "get_workflow_run"),
		),
		mcp.WithString("repository",
			mcp.Description("Repository identifier: owner/repo, GitHub URL, or full URL for specific issue/PR/workflow"),
		),
		mcp.WithObject("options",
			mcp.Description("Function-specific options - see function description for parameters"),
			mcp.Properties(map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query string (for search_repositories, search_issues, search_pull_requests)",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of results (for search functions, default: 30)",
					"default":     30,
				},
				"number": map[string]interface{}{
					"type":        "number",
					"description": "Issue or PR number (required for get_issue/get_pull_request unless using full URL)",
				},
				"run_id": map[string]interface{}{
					"type":        "number",
					"description": "Workflow run ID (required for get_workflow_run unless using full URL)",
				},
				"include_comments": map[string]interface{}{
					"type":        "boolean",
					"description": "Include comments for issues/PRs (default: false)",
					"default":     false,
				},
				"include_logs": map[string]interface{}{
					"type":        "boolean",
					"description": "Include workflow run logs (default: false)",
					"default":     false,
				},
				"include_closed": map[string]interface{}{
					"type":        "boolean",
					"description": "Include closed issues/PRs in search results (default: false, only open)",
					"default":     false,
				},
				"paths": map[string]interface{}{
					"type":        "array",
					"description": "Array of file paths to retrieve (required for get_file_contents)",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"ref": map[string]interface{}{
					"type":        "string",
					"description": "Git reference - branch, tag, or commit SHA (optional for get_file_contents)",
				},
				"local_path": map[string]interface{}{
					"type":        "string",
					"description": "Local directory path for cloning (optional for clone_repository)",
				},
			}),
		),
	)
}

// Execute executes the GitHub tool
func (t *GitHubTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
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
	case "clone_repository":
		return t.handleCloneRepository(ctx, client, request)
	case "get_workflow_run":
		return t.handleGetWorkflowRun(ctx, client, request)
	default:
		return nil, fmt.Errorf("unsupported function: %s", request.Function)
	}
}

// parseRequest parses and validates the request parameters
func (t *GitHubTool) parseRequest(args map[string]interface{}) (*GitHubRequest, error) {
	function, ok := args["function"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: function")
	}

	repository := ""
	if repo, ok := args["repository"].(string); ok {
		repository = repo
	}

	options := make(map[string]interface{})
	if opts, ok := args["options"].(map[string]interface{}); ok {
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

	response := map[string]interface{}{
		"function": "search_repositories",
		"query":    query,
		"result":   result,
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
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

	response := map[string]interface{}{
		"function":   "search_issues",
		"repository": fmt.Sprintf("%s/%s", owner, repo),
		"query":      query,
		"result":     result,
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
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

	response := map[string]interface{}{
		"function":   "search_pull_requests",
		"repository": fmt.Sprintf("%s/%s", owner, repo),
		"query":      query,
		"result":     result,
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
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

	response := map[string]interface{}{
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

	response := map[string]interface{}{
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
	return mcp.NewToolResultText(jsonString), nil
}

// handleGetFileContents handles getting file contents
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
	if pathsRaw, ok := request.Options["paths"].([]interface{}); ok {
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

	files, err := client.GetFileContents(ctx, owner, repo, paths, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get file contents: %w", err)
	}

	response := map[string]interface{}{
		"function":   "get_file_contents",
		"repository": fmt.Sprintf("%s/%s", owner, repo),
		"ref":        ref,
		"files":      files,
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
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

	result, err := client.CloneRepository(ctx, owner, repo, localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	response := map[string]interface{}{
		"function": "clone_repository",
		"result":   result,
	}

	jsonString, err := t.convertToJSON(response)
	if err != nil {
		return nil, err
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

	response := map[string]interface{}{
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
	return mcp.NewToolResultText(jsonString), nil
}

// convertToJSON converts the response to JSON string for better formatting
func (t *GitHubTool) convertToJSON(response interface{}) (string, error) {
	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal response to JSON: %w", err)
	}
	return string(jsonBytes), nil
}

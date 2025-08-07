package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
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

// ProvideExtendedInfo provides detailed usage information for the github tool
func (t *GitHubTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Search for repositories related to machine learning",
				Arguments: map[string]interface{}{
					"function": "search_repositories",
					"options": map[string]interface{}{
						"query": "machine learning python",
						"limit": 10,
					},
				},
				ExpectedResult: "Returns top 10 GitHub repositories matching 'machine learning python' with repository details, stars, and descriptions",
			},
			{
				Description: "Get details of a specific issue with comments",
				Arguments: map[string]interface{}{
					"function":   "get_issue",
					"repository": "microsoft/vscode",
					"options": map[string]interface{}{
						"number":           12345,
						"include_comments": true,
					},
				},
				ExpectedResult: "Returns issue #12345 from microsoft/vscode including full description, labels, assignees, and all comments",
			},
			{
				Description: "Search for open pull requests in a specific repository",
				Arguments: map[string]interface{}{
					"function":   "search_pull_requests",
					"repository": "facebook/react",
					"options": map[string]interface{}{
						"query": "bug fix",
						"limit": 5,
					},
				},
				ExpectedResult: "Returns 5 open pull requests from facebook/react that match 'bug fix' in title or description",
			},
			{
				Description: "Get file contents from a repository",
				Arguments: map[string]interface{}{
					"function":   "get_file_contents",
					"repository": "torvalds/linux",
					"options": map[string]interface{}{
						"paths": []string{"README", "MAINTAINERS"},
						"ref":   "v6.1",
					},
				},
				ExpectedResult: "Returns contents of README and MAINTAINERS files from Linux kernel v6.1 tag",
			},
			{
				Description: "Get workflow run details with logs",
				Arguments: map[string]interface{}{
					"function":   "get_workflow_run",
					"repository": "https://github.com/owner/repo/actions/runs/123456789",
					"options": map[string]interface{}{
						"include_logs": true,
					},
				},
				ExpectedResult: "Returns workflow run details and complete execution logs for run ID 123456789",
			},
		},
		CommonPatterns: []string{
			"Use search functions to discover repositories, issues, and PRs before getting specific details",
			"Include comments when investigating issues or PRs for full context",
			"Use specific repository format: 'owner/repo' or full GitHub URLs",
			"Combine get_file_contents with specific refs (branches/tags) for version-specific code analysis",
			"Use workflow runs to debug CI/CD issues and understand build failures",
			"Search with targeted queries to reduce result noise (e.g., 'is:open label:bug')",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Authentication errors or rate limits",
				Solution: "Ensure GITHUB_TOKEN environment variable is set with appropriate permissions. Public repositories require fewer permissions than private ones.",
			},
			{
				Problem:  "Repository not found errors",
				Solution: "Check repository name format (owner/repo), ensure repository exists and is public (or you have access), and verify spelling of repository name.",
			},
			{
				Problem:  "Issue or PR number not found",
				Solution: "Verify the issue/PR number exists in the specified repository. You can use search functions first to find the correct numbers.",
			},
			{
				Problem:  "File not found when getting file contents",
				Solution: "Check file paths are correct and exist in the specified branch/tag. Use repository browsing on GitHub first to verify paths and refs.",
			},
			{
				Problem:  "Workflow run access denied",
				Solution: "Workflow runs may require higher permissions than basic repository access. Ensure your GitHub token has 'actions:read' scope.",
			},
			{
				Problem:  "Search returns too many irrelevant results",
				Solution: "Use GitHub search qualifiers: 'language:python', 'stars:>100', 'is:open', 'label:bug', 'author:username' to refine searches.",
			},
		},
		ParameterDetails: map[string]string{
			"function":   "The GitHub operation to perform. Each function has different requirements - see examples for patterns and required options.",
			"repository": "Repository identifier in 'owner/repo' format, full GitHub URLs, or URLs with issue/PR/workflow IDs. The tool automatically extracts relevant information.",
			"options":    "Function-specific parameters. Query for searches, number for specific items, paths for files, include_* flags for additional data. See examples for each function type.",
		},
		WhenToUse:    "Use for GitHub repository analysis, issue tracking, pull request review, file content examination, CI/CD debugging, and repository discovery. Essential for code analysis and project management workflows.",
		WhenNotToUse: "Don't use for non-GitHub repositories, private repositories without proper authentication, or when you need to modify GitHub data (this tool is read-only). Use Git commands for actual repository cloning and manipulation.",
	}
}

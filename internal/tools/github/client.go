package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/google/go-github/v76/github"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

const (
	// GitHub API rate limits based on GitHub best practices
	DefaultCoreAPIRateLimit        = 80 // requests per minute (4800/hour, under 5000/hour limit)
	DefaultSearchAPIRateLimit      = 25 // requests per minute (under 30/minute limit)
	GitHubCoreAPIRateLimitEnvVar   = "GITHUB_CORE_API_RATE_LIMIT"
	GitHubSearchAPIRateLimitEnvVar = "GITHUB_SEARCH_API_RATE_LIMIT"
)

// GitHubClient wraps the GitHub API client with additional functionality
type GitHubClient struct {
	client           *github.Client
	authConfig       *AuthConfig
	logger           *logrus.Logger
	coreAPILimiter   *rate.Limiter
	searchAPILimiter *rate.Limiter
	mu               sync.Mutex
}

// NewGitHubClientWrapper creates a new GitHub client wrapper
func NewGitHubClientWrapper(ctx context.Context, logger *logrus.Logger) (*GitHubClient, error) {
	authConfig, err := GetAuthConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth config: %w", err)
	}

	client, err := NewGitHubClient(ctx, authConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	return &GitHubClient{
		client:           client,
		authConfig:       authConfig,
		logger:           logger,
		coreAPILimiter:   newCoreAPIRateLimiter(),
		searchAPILimiter: newSearchAPIRateLimiter(),
	}, nil
}

// SearchRepositories searches for repositories
func (gc *GitHubClient) SearchRepositories(ctx context.Context, query string, limit int) (*SearchResult, error) {
	// Apply search API rate limiting
	if err := gc.waitForSearchAPIRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("search API rate limit wait failed: %w", err)
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: limit,
		},
	}

	result, _, err := gc.client.Search.Repositories(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search repositories: %w", err)
	}

	repositories := make([]FilteredRepository, len(result.Repositories))
	for i, repo := range result.Repositories {
		repositories[i] = FilteredRepository{
			ID:          repo.GetID(),
			FullName:    repo.GetFullName(),
			Description: repo.GetDescription(),
			CreatedAt:   repo.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
			UpdatedAt:   repo.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
		}
	}

	return &SearchResult{
		TotalCount:        result.GetTotal(),
		IncompleteResults: result.GetIncompleteResults(),
		Items:             repositories,
	}, nil
}

// SearchIssues searches for issues in a repository
func (gc *GitHubClient) SearchIssues(ctx context.Context, owner, repo, query string, limit int, includeClosed bool) (*SearchResult, error) {
	// Apply search API rate limiting
	if err := gc.waitForSearchAPIRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("search API rate limit wait failed: %w", err)
	}

	// GitHub API requires 'type:issue' or 'is:issue' qualifier in search queries
	// to differentiate between issues and pull requests (which are both returned
	// by the /search/issues endpoint). Without this qualifier, GitHub returns a
	// 422 error: "Query must include 'is:issue' or 'is:pull-request'"
	searchQuery := fmt.Sprintf("repo:%s/%s type:issue %s", owner, repo, query)
	if !includeClosed {
		searchQuery += " state:open"
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: limit,
		},
	}

	result, _, err := gc.client.Search.Issues(ctx, searchQuery, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}

	issues := make([]FilteredIssue, 0)
	for _, issue := range result.Issues {
		// Skip pull requests (GitHub API includes PRs in issue search)
		if issue.IsPullRequest() {
			continue
		}

		issues = append(issues, FilteredIssue{
			ID:        issue.GetID(),
			Number:    issue.GetNumber(),
			Title:     issue.GetTitle(),
			Body:      issue.GetBody(),
			State:     issue.GetState(),
			Login:     issue.User.GetLogin(),
			CreatedAt: issue.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
			UpdatedAt: issue.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
		})
	}

	return &SearchResult{
		TotalCount:        result.GetTotal(),
		IncompleteResults: result.GetIncompleteResults(),
		Items:             issues,
	}, nil
}

// SearchPullRequests searches for pull requests in a repository
func (gc *GitHubClient) SearchPullRequests(ctx context.Context, owner, repo, query string, limit int, includeClosed bool) (*SearchResult, error) {
	// Apply search API rate limiting
	if err := gc.waitForSearchAPIRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("search API rate limit wait failed: %w", err)
	}

	searchQuery := fmt.Sprintf("repo:%s/%s type:pr %s", owner, repo, query)
	if !includeClosed {
		searchQuery += " state:open"
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: limit,
		},
	}

	result, _, err := gc.client.Search.Issues(ctx, searchQuery, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search pull requests: %w", err)
	}

	pullRequests := make([]FilteredPullRequest, 0)
	for _, issue := range result.Issues {
		// Only include pull requests
		if !issue.IsPullRequest() {
			continue
		}

		pullRequests = append(pullRequests, FilteredPullRequest{
			ID:        issue.GetID(),
			Number:    issue.GetNumber(),
			Title:     issue.GetTitle(),
			Body:      issue.GetBody(),
			State:     issue.GetState(),
			Login:     issue.User.GetLogin(),
			CreatedAt: issue.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
			UpdatedAt: issue.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
		})
	}

	return &SearchResult{
		TotalCount:        result.GetTotal(),
		IncompleteResults: result.GetIncompleteResults(),
		Items:             pullRequests,
	}, nil
}

// GetIssue gets a specific issue with optional comments
func (gc *GitHubClient) GetIssue(ctx context.Context, owner, repo string, number int, includeComments bool) (*FilteredIssueDetails, []Comment, error) {
	// Apply core API rate limiting
	if err := gc.waitForCoreAPIRateLimit(ctx); err != nil {
		return nil, nil, fmt.Errorf("core API rate limit wait failed: %w", err)
	}

	issue, _, err := gc.client.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get issue: %w", err)
	}

	issueResult := &FilteredIssueDetails{
		ID:         issue.GetID(),
		Body:       issue.GetBody(),
		Login:      issue.User.GetLogin(),
		Repository: fmt.Sprintf("%s/%s", owner, repo),
	}

	var comments []Comment
	if includeComments && issue.GetComments() > 0 {
		// Apply core API rate limiting for comments
		if err := gc.waitForCoreAPIRateLimit(ctx); err != nil {
			return nil, nil, fmt.Errorf("core API rate limit wait failed for comments: %w", err)
		}
		commentList, _, err := gc.client.Issues.ListComments(ctx, owner, repo, number, nil)
		if err != nil {
			gc.logger.Warnf("Failed to get comments for issue #%d: %v", number, err)
		} else {
			comments = make([]Comment, len(commentList))
			for i, comment := range commentList {
				comments[i] = Comment{
					ID:   comment.GetID(),
					Body: comment.GetBody(),
					User: User{
						ID:        comment.User.GetID(),
						Login:     comment.User.GetLogin(),
						Name:      comment.User.GetName(),
						Email:     comment.User.GetEmail(),
						AvatarURL: comment.User.GetAvatarURL(),
						HTMLURL:   comment.User.GetHTMLURL(),
						Type:      comment.User.GetType(),
					},
					HTMLURL:   comment.GetHTMLURL(),
					CreatedAt: comment.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
					UpdatedAt: comment.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
				}
			}
		}
	}

	return issueResult, comments, nil
}

// GetPullRequest gets a specific pull request with optional comments
func (gc *GitHubClient) GetPullRequest(ctx context.Context, owner, repo string, number int, includeComments bool) (*FilteredPullRequestDetails, []Comment, error) {
	// Apply core API rate limiting
	if err := gc.waitForCoreAPIRateLimit(ctx); err != nil {
		return nil, nil, fmt.Errorf("core API rate limit wait failed: %w", err)
	}

	pr, _, err := gc.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get pull request: %w", err)
	}

	pullRequest := &FilteredPullRequestDetails{
		ID:        pr.GetID(),
		Number:    pr.GetNumber(),
		Title:     pr.GetTitle(),
		Body:      pr.GetBody(),
		State:     pr.GetState(),
		Login:     pr.User.GetLogin(),
		HeadLabel: pr.Head.GetLabel(),
		BaseLabel: pr.Base.GetLabel(),
		Comments:  pr.GetComments(),
		CreatedAt: pr.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
		UpdatedAt: pr.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
	}

	var comments []Comment
	if includeComments && pr.GetComments() > 0 {
		// Apply core API rate limiting for comments
		if err := gc.waitForCoreAPIRateLimit(ctx); err != nil {
			return nil, nil, fmt.Errorf("core API rate limit wait failed for comments: %w", err)
		}
		commentList, _, err := gc.client.PullRequests.ListComments(ctx, owner, repo, number, nil)
		if err != nil {
			gc.logger.Warnf("Failed to get comments for PR #%d: %v", number, err)
		} else {
			comments = make([]Comment, len(commentList))
			for i, comment := range commentList {
				comments[i] = Comment{
					ID:   comment.GetID(),
					Body: comment.GetBody(),
					User: User{
						ID:        comment.User.GetID(),
						Login:     comment.User.GetLogin(),
						Name:      comment.User.GetName(),
						Email:     comment.User.GetEmail(),
						AvatarURL: comment.User.GetAvatarURL(),
						HTMLURL:   comment.User.GetHTMLURL(),
						Type:      comment.User.GetType(),
					},
					HTMLURL:   comment.GetHTMLURL(),
					CreatedAt: comment.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
					UpdatedAt: comment.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
				}
			}
		}
	}

	return pullRequest, comments, nil
}

// GetFileContents gets the contents of one or more files from a repository with graceful error handling
func (gc *GitHubClient) GetFileContents(ctx context.Context, owner, repo string, paths []string, ref string) ([]FileResult, error) {
	var results []FileResult

	for _, originalPath := range paths {
		// Clean and validate the path
		path := CleanPath(originalPath)

		// Apply core API rate limiting for each file request
		if err := gc.waitForCoreAPIRateLimit(ctx); err != nil {
			results = append(results, FileResult{
				Path:    originalPath,
				Success: false,
				Error:   fmt.Sprintf("rate limit wait failed: %v", err),
			})
			continue
		}

		opts := &github.RepositoryContentGetOptions{}
		if ref != "" {
			opts.Ref = ref
		}

		fileContent, _, _, err := gc.client.Repositories.GetContents(ctx, owner, repo, path, opts)
		if err != nil {
			// Create helpful error message for 404s
			errorMsg := err.Error()
			if strings.Contains(err.Error(), "404") {
				errorMsg = CreateFileNotFoundError(owner, repo, path, ref).Error()
			}

			results = append(results, FileResult{
				Path:    originalPath,
				Success: false,
				Error:   errorMsg,
			})
			continue
		}

		if fileContent == nil {
			results = append(results, FileResult{
				Path:    originalPath,
				Success: false,
				Error:   "file content is nil",
			})
			continue
		}

		content := ""

		// Only decode content for text files (not too large)
		if fileContent.GetSize() <= 1024*1024 { // 1MB limit
			contentStr, err := fileContent.GetContent()
			if err == nil && contentStr != "" {

				// Try to decode as base64 first (standard GitHub API behavior)
				if contentBytes, err := base64.StdEncoding.DecodeString(contentStr); err == nil {
					// Check if content is text (no null bytes in first 512 bytes)
					checkBytes := contentBytes
					if len(checkBytes) > 512 {
						checkBytes = checkBytes[:512]
					}

					isText := true
					if slices.Contains(checkBytes, 0) {
						isText = false
					}

					if isText {
						content = string(contentBytes)
					}
				} else {
					// If base64 decoding fails, treat as plain text content
					// This can happen with some GitHub API responses
					checkBytes := []byte(contentStr)
					if len(checkBytes) > 512 {
						checkBytes = checkBytes[:512]
					}

					isText := true
					if slices.Contains(checkBytes, 0) {
						isText = false
					}

					if isText {
						content = contentStr
					}
				}
			}
		}

		results = append(results, FileResult{
			Path:    fileContent.GetPath(),
			Size:    fileContent.GetSize(),
			Content: content,
			Success: true,
		})
	}

	return results, nil
}

// ListDirectory lists the contents of a directory in a repository
func (gc *GitHubClient) ListDirectory(ctx context.Context, owner, repo, path, ref string) (*DirectoryListing, error) {
	// Apply core API rate limiting
	if err := gc.waitForCoreAPIRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("core API rate limit wait failed: %w", err)
	}

	// Clean the path
	cleanPath := CleanPath(path)

	opts := &github.RepositoryContentGetOptions{}
	if ref != "" {
		opts.Ref = ref
	}

	_, directoryContents, _, err := gc.client.Repositories.GetContents(ctx, owner, repo, cleanPath, opts)
	if err != nil {
		// Create helpful error message for 404s
		if strings.Contains(err.Error(), "404") {
			return nil, CreateFileNotFoundError(owner, repo, cleanPath, ref)
		}
		return nil, fmt.Errorf("failed to list directory contents: %w", err)
	}

	items := make([]DirectoryItem, len(directoryContents))
	for i, item := range directoryContents {
		items[i] = DirectoryItem{
			Name: item.GetName(),
			Path: item.GetPath(),
			Type: item.GetType(),
			Size: item.GetSize(),
			SHA:  item.GetSHA(),
		}
	}

	return &DirectoryListing{
		Path:  cleanPath,
		Items: items,
	}, nil
}

// CloneRepository clones a repository to a local directory
func (gc *GitHubClient) CloneRepository(ctx context.Context, owner, repo, localPath string) (*CloneResult, error) {
	// Domain access and file access are handled by existing security infrastructure

	if localPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		localPath = filepath.Join(homeDir, "github-repos", repo)
	}

	// Check file access security using helper function
	// Note: SafeFileWrite will handle file access checks internally
	if err := security.CheckFileAccess(localPath); err != nil {
		return nil, err
	}

	// Ensure the parent directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Choose clone URL based on authentication method
	var cloneURL string
	if gc.authConfig.Method == "ssh" {
		cloneURL = fmt.Sprintf("git@github.com:%s/%s.git", owner, repo)
	} else {
		cloneURL = fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
	}

	// Set up git command
	cmd := exec.CommandContext(ctx, "git", "clone", cloneURL, localPath)

	// Set up environment for authentication
	env := os.Environ()
	if gc.authConfig.Method == "token" && gc.authConfig.Token != "" {
		// For HTTPS with token, we need to configure git credentials
		env = append(env, "GIT_ASKPASS=echo")
		env = append(env, "GIT_USERNAME="+gc.authConfig.Token)
		env = append(env, "GIT_PASSWORD=")

		// Use token in URL
		cloneURL = fmt.Sprintf("https://%s@github.com/%s/%s.git", gc.authConfig.Token, owner, repo)
		cmd = exec.CommandContext(ctx, "git", "clone", cloneURL, localPath)
	} else if gc.authConfig.Method == "ssh" {
		// For SSH, ensure SSH agent is available and key is configured
		env = append(env, "GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=no")
	}

	cmd.Env = env

	// Execute the clone command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &CloneResult{
			Repository: fmt.Sprintf("%s/%s", owner, repo),
			LocalPath:  localPath,
			CloneURL:   cloneURL,
			Success:    false,
			Message:    fmt.Sprintf("Clone failed: %s", string(output)),
		}, nil
	}

	return &CloneResult{
		Repository: fmt.Sprintf("%s/%s", owner, repo),
		LocalPath:  localPath,
		CloneURL:   cloneURL,
		Success:    true,
		Message:    "Repository cloned successfully",
	}, nil
}

// GetWorkflowRun gets GitHub Actions workflow run status and optionally logs
func (gc *GitHubClient) GetWorkflowRun(ctx context.Context, owner, repo string, runID int64, includeLogs bool) (*WorkflowRun, string, error) {
	// Create security operations instance for logs download
	ops := security.NewOperations("github")

	// Apply core API rate limiting
	if err := gc.waitForCoreAPIRateLimit(ctx); err != nil {
		return nil, "", fmt.Errorf("core API rate limit wait failed: %w", err)
	}

	run, _, err := gc.client.Actions.GetWorkflowRunByID(ctx, owner, repo, runID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get workflow run: %w", err)
	}

	workflowRun := &WorkflowRun{
		ID:           run.GetID(),
		Name:         run.GetName(),
		Status:       run.GetStatus(),
		Conclusion:   run.GetConclusion(),
		URL:          run.GetURL(),
		HTMLURL:      run.GetHTMLURL(),
		CreatedAt:    run.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    run.GetUpdatedAt().Format("2006-01-02T15:04:05Z"),
		RunStartedAt: formatTimestamp(run.RunStartedAt),
	}

	var logs string
	if includeLogs {
		// Apply core API rate limiting for logs
		if err := gc.waitForCoreAPIRateLimit(ctx); err != nil {
			return nil, "", fmt.Errorf("core API rate limit wait failed for logs: %w", err)
		}
		// Get logs URL
		logsURL, _, err := gc.client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, 1)
		if err != nil {
			gc.logger.Warnf("Failed to get workflow run logs: %v", err)
		} else if logsURL != nil {
			// Download and read logs using security helper
			safeResp, err := ops.SafeHTTPGet(logsURL.String())
			if err != nil {
				if secErr, ok := err.(*security.SecurityError); ok {
					gc.logger.Warnf("Security block [ID: %s]: %s", secErr.GetSecurityID(), secErr.Error())
				} else {
					gc.logger.Warnf("Failed to download workflow run logs: %v", err)
				}
			} else {
				// Handle security warnings
				if safeResp.SecurityResult != nil && safeResp.SecurityResult.Action == security.ActionWarn {
					gc.logger.Warnf("Security warning [ID: %s]: %s", safeResp.SecurityResult.ID, safeResp.SecurityResult.Message)
				}

				// Content is exact bytes from SafeHTTPGet
				logs = string(safeResp.Content)
				// Limit log size to prevent overwhelming the response
				if len(logs) > 50000 { // 50KB limit
					logs = logs[:50000] + "\n... (logs truncated)"
				}
			}
		}
	}

	return workflowRun, logs, nil
}

// ExtractWorkflowRunID extracts workflow run ID from GitHub Actions URL
func ExtractWorkflowRunID(url string) (int64, error) {
	if len(url) > 19 && url[:19] == "https://github.com/" {
		path := url[19:]
		parts := splitPath(path)

		// Format: owner/repo/actions/runs/123456789
		if len(parts) >= 5 && parts[2] == "actions" && parts[3] == "runs" {
			runID, err := strconv.ParseInt(parts[4], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid workflow run ID: %s", parts[4])
			}
			return runID, nil
		}
	}

	return 0, fmt.Errorf("invalid GitHub Actions workflow run URL: %s", url)
}

// formatTimestamp formats a GitHub timestamp pointer to string
func formatTimestamp(ts *github.Timestamp) string {
	if ts == nil {
		return ""
	}
	return ts.Format("2006-01-02T15:04:05Z")
}

// newCoreAPIRateLimiter creates a rate limiter for GitHub Core API calls
func newCoreAPIRateLimiter() *rate.Limiter {
	rateLimit := GetEnvRateLimit(GitHubCoreAPIRateLimitEnvVar, DefaultCoreAPIRateLimit)
	return rate.NewLimiter(rate.Limit(rateLimit)/60, 1) // Convert per-minute to per-second
}

// newSearchAPIRateLimiter creates a rate limiter for GitHub Search API calls
func newSearchAPIRateLimiter() *rate.Limiter {
	rateLimit := GetEnvRateLimit(GitHubSearchAPIRateLimitEnvVar, DefaultSearchAPIRateLimit)
	return rate.NewLimiter(rate.Limit(rateLimit)/60, 1) // Convert per-minute to per-second
}

// GetEnvRateLimit gets rate limit from environment variable with fallback to default
func GetEnvRateLimit(envVar string, defaultValue int) int {
	if limitStr := os.Getenv(envVar); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			return limit
		}
	}
	return defaultValue
}

// waitForCoreAPIRateLimit waits for core API rate limit before making a request
func (gc *GitHubClient) waitForCoreAPIRateLimit(ctx context.Context) error {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	return gc.coreAPILimiter.Wait(ctx)
}

// waitForSearchAPIRateLimit waits for search API rate limit before making a request
func (gc *GitHubClient) waitForSearchAPIRateLimit(ctx context.Context) error {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	return gc.searchAPILimiter.Wait(ctx)
}

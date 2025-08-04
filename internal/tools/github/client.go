package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/google/go-github/v73/github"
	"github.com/sirupsen/logrus"
)

// GitHubClient wraps the GitHub API client with additional functionality
type GitHubClient struct {
	client     *github.Client
	authConfig *AuthConfig
	logger     *logrus.Logger
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
		client:     client,
		authConfig: authConfig,
		logger:     logger,
	}, nil
}

// SearchRepositories searches for repositories
func (gc *GitHubClient) SearchRepositories(ctx context.Context, query string, limit int) (*SearchResult, error) {
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
	searchQuery := fmt.Sprintf("repo:%s/%s %s", owner, repo, query)
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

// GetFileContents gets the contents of one or more files from a repository
func (gc *GitHubClient) GetFileContents(ctx context.Context, owner, repo string, paths []string, ref string) ([]FilteredFileContent, error) {
	var results []FilteredFileContent

	for _, path := range paths {
		opts := &github.RepositoryContentGetOptions{}
		if ref != "" {
			opts.Ref = ref
		}

		fileContent, _, _, err := gc.client.Repositories.GetContents(ctx, owner, repo, path, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get file content for %s: %w", path, err)
		}

		if fileContent == nil {
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
					for _, b := range checkBytes {
						if b == 0 {
							isText = false
							break
						}
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
					for _, b := range checkBytes {
						if b == 0 {
							isText = false
							break
						}
					}

					if isText {
						content = contentStr
					}
				}
			}
		}

		results = append(results, FilteredFileContent{
			Path:    fileContent.GetPath(),
			Size:    fileContent.GetSize(),
			Content: content,
		})
	}

	return results, nil
}

// CloneRepository clones a repository to a local directory
func (gc *GitHubClient) CloneRepository(ctx context.Context, owner, repo, localPath string) (*CloneResult, error) {
	if localPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		localPath = filepath.Join(homeDir, "github-repos", repo)
	}

	// Ensure the parent directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
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
		// Get logs URL
		logsURL, _, err := gc.client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, 1)
		if err != nil {
			gc.logger.Warnf("Failed to get workflow run logs: %v", err)
		} else if logsURL != nil {
			// Download and read logs
			resp, err := http.Get(logsURL.String())
			if err != nil {
				gc.logger.Warnf("Failed to download workflow run logs: %v", err)
			} else {
				defer func() {
					if closeErr := resp.Body.Close(); closeErr != nil {
						gc.logger.Warnf("Failed to close response body: %v", closeErr)
					}
				}()
				logBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					gc.logger.Warnf("Failed to read workflow run logs: %v", err)
				} else {
					logs = string(logBytes)
					// Limit log size to prevent overwhelming the response
					if len(logs) > 50000 { // 50KB limit
						logs = logs[:50000] + "\n... (logs truncated)"
					}
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

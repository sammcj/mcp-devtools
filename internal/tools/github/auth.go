package github

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-github/v76/github"
	"github.com/sammcj/mcp-devtools/internal/utils/httpclient"
	"golang.org/x/oauth2"
)

// GetAuthConfig determines the authentication method and configuration
func GetAuthConfig() (*AuthConfig, error) {
	// Check environment variables for auth method preference
	authMethod := os.Getenv("GITHUB_AUTH_METHOD")

	// Check for GitHub token
	token := os.Getenv("GITHUB_TOKEN")

	// If SSH method is explicitly requested, validate SSH keys
	if authMethod == "ssh" {
		sshPrivateKey := getSSHKeyPath()
		if sshPrivateKey == "" {
			return nil, fmt.Errorf("SSH authentication requested but no SSH key found")
		}
		return &AuthConfig{
			Method:     "ssh",
			SSHKeyPath: sshPrivateKey,
		}, nil
	}

	// If token is available, use token auth
	if token != "" {
		return &AuthConfig{
			Method: "token",
			Token:  token,
		}, nil
	}

	// Fall back to no authentication (public repos only)
	return &AuthConfig{
		Method: "none",
	}, nil
}

// getSSHKeyPath finds the SSH key path according to the priority rules
func getSSHKeyPath() string {
	// Check environment variable first
	if customPath := os.Getenv("GITHUB_SSH_PRIVATE_KEY_PATH"); customPath != "" {
		if _, err := os.Stat(customPath); err == nil {
			return customPath
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Check for ed25519 key first
	ed25519Path := filepath.Join(homeDir, ".ssh", "id_ed25519")
	if _, err := os.Stat(ed25519Path); err == nil {
		return ed25519Path
	}

	// Fall back to RSA key
	rsaPath := filepath.Join(homeDir, ".ssh", "id_rsa")
	if _, err := os.Stat(rsaPath); err == nil {
		return rsaPath
	}

	return ""
}

// NewGitHubClient creates a new GitHub client with appropriate authentication
func NewGitHubClient(ctx context.Context, config *AuthConfig) (*github.Client, error) {
	switch config.Method {
	case "token":
		if config.Token == "" {
			return nil, fmt.Errorf("GitHub token is required for token authentication")
		}

		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: config.Token},
		)
		tc := oauth2.NewClient(ctx, ts)
		return github.NewClient(tc), nil

	case "ssh":
		// For SSH, we still need to use the REST API for most operations
		// SSH is primarily for git operations (cloning, etc.)
		// We'll create a client without authentication for API calls with proxy support
		return github.NewClient(httpclient.NewHTTPClientWithProxy(30 * time.Second)), nil

	case "none":
		// No authentication - public repos only with proxy support
		return github.NewClient(httpclient.NewHTTPClientWithProxy(30 * time.Second)), nil

	default:
		return nil, fmt.Errorf("unsupported authentication method: %s", config.Method)
	}
}

// ValidateRepository parses and validates repository identifier
// Supports formats:
// - owner/repo
// - https://github.com/owner/repo
// - https://github.com/owner/repo.git
// - https://github.com/owner/repo/issues/123
// - https://github.com/owner/repo/pull/456
func ValidateRepository(repository string) (owner, repo string, err error) {
	if repository == "" {
		return "", "", fmt.Errorf("repository cannot be empty")
	}

	// Handle full GitHub URLs
	if len(repository) > 19 && repository[:19] == "https://github.com/" {
		// Remove the GitHub URL prefix
		path := repository[19:]

		// Remove .git suffix if present
		if len(path) > 4 && path[len(path)-4:] == ".git" {
			path = path[:len(path)-4]
		}

		// Split by "/" and take first two parts
		parts := splitPath(path)
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}

		return "", "", fmt.Errorf("invalid GitHub URL format: %s", repository)
	}

	// Handle owner/repo format
	parts := splitPath(repository)
	if len(parts) >= 2 {
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("invalid repository format: %s (expected owner/repo or GitHub URL)", repository)
}

// splitPath splits a path by "/" and returns non-empty parts
func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	var parts []string
	for _, part := range split(path, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// split is a simple string splitting function
func split(s, sep string) []string {
	if s == "" {
		return []string{}
	}

	var result []string
	current := ""

	for i := 0; i < len(s); i++ {
		if i < len(s)-len(sep)+1 && s[i:i+len(sep)] == sep {
			result = append(result, current)
			current = ""
			i += len(sep) - 1
		} else {
			current += string(s[i])
		}
	}

	if current != "" || len(result) == 0 {
		result = append(result, current)
	}

	return result
}

// ExtractIssueNumber extracts issue number from GitHub issue URL
func ExtractIssueNumber(url string) (int, error) {
	if len(url) > 19 && url[:19] == "https://github.com/" {
		path := url[19:]
		parts := splitPath(path)

		// Format: owner/repo/issues/123
		if len(parts) >= 4 && parts[2] == "issues" {
			return parseInt(parts[3])
		}
	}

	return 0, fmt.Errorf("invalid GitHub issue URL: %s", url)
}

// ExtractPullRequestNumber extracts PR number from GitHub PR URL
func ExtractPullRequestNumber(url string) (int, error) {
	if len(url) > 19 && url[:19] == "https://github.com/" {
		path := url[19:]
		parts := splitPath(path)

		// Format: owner/repo/pull/456
		if len(parts) >= 4 && parts[2] == "pull" {
			return parseInt(parts[3])
		}
	}

	return 0, fmt.Errorf("invalid GitHub pull request URL: %s", url)
}

// parseInt converts string to int
func parseInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid number: empty string")
	}
	result := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid number: %s", s)
		}
		result = result*10 + int(r-'0')
	}
	return result, nil
}

// CleanPath cleans and validates a file path
func CleanPath(path string) string {
	if path == "" {
		return path
	}

	// Remove leading and trailing slashes
	for len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	for len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	return path
}

// CreateFileNotFoundError creates a helpful error message for file not found
func CreateFileNotFoundError(owner, repo, path, ref string) error {
	refInfo := ""
	if ref != "" {
		refInfo = fmt.Sprintf(" in ref '%s'", ref)
	} else {
		refInfo = " in the default branch"
	}

	return fmt.Errorf("file '%s' not found in repository %s/%s%s. "+
		"Suggestions: "+
		"1) Verify the file path exists by checking https://github.com/%s/%s/tree/%s "+
		"2) Use the list_directory function to explore the repository structure "+
		"3) Ensure the path doesn't have typos or case sensitivity issues",
		path, owner, repo, refInfo, owner, repo,
		func() string {
			if ref != "" {
				return ref
			}
			return "main"
		}())
}

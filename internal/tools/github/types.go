package github

// No imports needed for types

// GitHubRequest represents the unified request structure for all GitHub operations
type GitHubRequest struct {
	Function   string         `json:"function"`
	Repository string         `json:"repository"`
	Options    map[string]any `json:"options,omitempty"`
}

// Repository represents a GitHub repository
type Repository struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description,omitempty"`
	Private     bool   `json:"private"`
	HTMLURL     string `json:"html_url"`
	CloneURL    string `json:"clone_url"`
	SSHURL      string `json:"ssh_url"`
	Language    string `json:"language,omitempty"`
	Stars       int    `json:"stargazers_count"`
	Forks       int    `json:"forks_count"`
	OpenIssues  int    `json:"open_issues_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Issue represents a GitHub issue
type Issue struct {
	ID        int64   `json:"id"`
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	Body      string  `json:"body,omitempty"`
	State     string  `json:"state"`
	User      User    `json:"user"`
	Assignee  *User   `json:"assignee,omitempty"`
	Labels    []Label `json:"labels"`
	Comments  int     `json:"comments"`
	HTMLURL   string  `json:"html_url"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	ClosedAt  string  `json:"closed_at,omitempty"`
}

// PullRequest represents a GitHub pull request
type PullRequest struct {
	ID        int64   `json:"id"`
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	Body      string  `json:"body,omitempty"`
	State     string  `json:"state"`
	User      User    `json:"user"`
	Assignee  *User   `json:"assignee,omitempty"`
	Labels    []Label `json:"labels"`
	Head      Branch  `json:"head"`
	Base      Branch  `json:"base"`
	Merged    bool    `json:"merged"`
	Mergeable *bool   `json:"mergeable,omitempty"`
	Comments  int     `json:"comments"`
	Commits   int     `json:"commits"`
	Additions int     `json:"additions"`
	Deletions int     `json:"deletions"`
	HTMLURL   string  `json:"html_url"`
	DiffURL   string  `json:"diff_url"`
	PatchURL  string  `json:"patch_url"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	ClosedAt  string  `json:"closed_at,omitempty"`
	MergedAt  string  `json:"merged_at,omitempty"`
}

// User represents a GitHub user
type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
	Type      string `json:"type"`
}

// Label represents a GitHub label
type Label struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
}

// Branch represents a GitHub branch
type Branch struct {
	Label string `json:"label"`
	Ref   string `json:"ref"`
	SHA   string `json:"sha"`
	User  User   `json:"user"`
}

// Comment represents a GitHub comment
type Comment struct {
	ID        int64  `json:"id"`
	Body      string `json:"body"`
	User      User   `json:"user"`
	HTMLURL   string `json:"html_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// FileContent represents the content of a file in a repository
type FileContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	SHA         string `json:"sha"`
	Size        int    `json:"size"`
	URL         string `json:"url"`
	HTMLURL     string `json:"html_url"`
	GitURL      string `json:"git_url"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
	Content     string `json:"content,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
}

// WorkflowRun represents a GitHub Actions workflow run
type WorkflowRun struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion,omitempty"`
	URL          string `json:"url"`
	HTMLURL      string `json:"html_url"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	RunStartedAt string `json:"run_started_at,omitempty"`
}

// CloneResult represents the result of a clone operation
type CloneResult struct {
	Repository string `json:"repository"`
	LocalPath  string `json:"local_path"`
	CloneURL   string `json:"clone_url"`
	Success    bool   `json:"success"`
	Message    string `json:"message,omitempty"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Method     string `json:"method"` // "token", "ssh", or "none"
	Token      string `json:"token,omitempty"`
	SSHKeyPath string `json:"ssh_key_path,omitempty"`
}

// SearchResult represents a generic search result wrapper
type SearchResult struct {
	TotalCount        int  `json:"total_count"`
	IncompleteResults bool `json:"incomplete_results"`
	Items             any  `json:"items"`
}

// Filtered response types for reduced output

// FilteredRepository represents a minimal repository for search results
type FilteredRepository struct {
	ID          int64  `json:"id"`
	FullName    string `json:"full_name"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// FilteredFileContent represents minimal file content
type FilteredFileContent struct {
	Path    string `json:"path"`
	Size    int    `json:"size"`
	Content string `json:"content,omitempty"`
}

// FilteredIssue represents minimal issue for search results
type FilteredIssue struct {
	ID        int64  `json:"id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body,omitempty"`
	State     string `json:"state"`
	Login     string `json:"login"` // user.login
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// FilteredIssueDetails represents minimal issue details for get_issue
type FilteredIssueDetails struct {
	ID         int64  `json:"id"`
	Body       string `json:"body,omitempty"`
	Login      string `json:"login"` // user.login
	Repository string `json:"repository"`
}

// FilteredPullRequest represents minimal PR for search results
type FilteredPullRequest struct {
	ID        int64  `json:"id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body,omitempty"`
	State     string `json:"state"`
	Login     string `json:"login"` // user.login
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// FilteredPullRequestDetails represents minimal PR details for get_pull_request
type FilteredPullRequestDetails struct {
	ID        int64  `json:"id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body,omitempty"`
	State     string `json:"state"`
	Login     string `json:"login"`      // user.login
	HeadLabel string `json:"head_label"` // head.label
	BaseLabel string `json:"base_label"` // base.label
	Comments  int    `json:"comments"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// FileResult represents the result of attempting to fetch a file (success or failure)
type FileResult struct {
	Path    string `json:"path"`
	Size    int    `json:"size,omitempty"`
	Content string `json:"content,omitempty"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// DirectoryItem represents an item in a directory listing
type DirectoryItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "file", "dir", "symlink", etc.
	Size int    `json:"size,omitempty"`
	SHA  string `json:"sha,omitempty"`
}

// DirectoryListing represents the contents of a directory
type DirectoryListing struct {
	Path  string          `json:"path"`
	Items []DirectoryItem `json:"items"`
}

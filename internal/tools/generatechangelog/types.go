package generatechangelog

import "time"

// GenerateChangelogRequest represents the input parameters for changelog generation
type GenerateChangelogRequest struct {
	RepositoryPath          string `json:"repository_path"`
	SinceTag                string `json:"since_tag,omitempty"`
	UntilTag                string `json:"until_tag,omitempty"`
	OutputFormat            string `json:"output_format"`
	SpeculateNextVersion    bool   `json:"speculate_next_version"`
	EnableGitHubIntegration bool   `json:"enable_github_integration"`
	Title                   string `json:"title"`
	OutputFile              string `json:"output_file,omitempty"`
	TimeoutMinutes          int    `json:"timeout_minutes"`
}

// GenerateChangelogResponse represents the output from changelog generation
type GenerateChangelogResponse struct {
	Content        string    `json:"content"`
	Format         string    `json:"format"`
	VersionRange   string    `json:"version_range"`
	ChangeCount    int       `json:"change_count"`
	CurrentVersion string    `json:"current_version,omitempty"`
	NextVersion    string    `json:"next_version,omitempty"`
	RepositoryURL  string    `json:"repository_url,omitempty"`
	ChangesURL     string    `json:"changes_url,omitempty"`
	OutputFile     string    `json:"output_file,omitempty"`
	GenerationTime time.Time `json:"generation_time"`
	RepositoryPath string    `json:"repository_path"`
}

// ChangelogEntry represents a single change in the changelog
type ChangelogEntry struct {
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Author      string   `json:"author,omitempty"`
	PRNumber    int      `json:"pr_number,omitempty"`
	IssueNumber int      `json:"issue_number,omitempty"`
	CommitHash  string   `json:"commit_hash,omitempty"`
	Labels      []string `json:"labels,omitempty"`
}

// ChangelogSection represents a section of changes grouped by type
type ChangelogSection struct {
	Type    string           `json:"type"`
	Title   string           `json:"title"`
	Changes []ChangelogEntry `json:"changes"`
}

// VersionInfo represents version information for the changelog
type VersionInfo struct {
	Current string `json:"current"`
	Next    string `json:"next,omitempty"`
	Range   string `json:"range"`
}

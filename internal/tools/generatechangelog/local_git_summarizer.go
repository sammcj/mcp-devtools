package generatechangelog

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/sirupsen/logrus"
)

// GitCommit represents a git commit for local changelog generation
type GitCommit struct {
	Hash    string
	Subject string
	Body    string
	Author  string
	Date    time.Time
}

// LocalGitSummarizer implements Chronicle's Summarizer interface for local git repositories
type LocalGitSummarizer struct {
	repoPath string
	lastTag  string
	changes  []change.Change
	logger   *logrus.Logger
}

// LastRelease returns the last git tag as a release
func (s *LocalGitSummarizer) LastRelease() (*release.Release, error) {
	if s.lastTag == "" {
		return nil, nil // No releases found
	}

	return &release.Release{
		Version: s.lastTag,
		Date:    time.Now(), // We could parse the tag date, but this is simpler
	}, nil
}

// Release returns a specific release for the given tag
func (s *LocalGitSummarizer) Release(ref string) (*release.Release, error) {
	if ref == "" {
		return s.LastRelease()
	}

	// Check if the tag exists
	cmd := exec.Command("git", "-C", s.repoPath, "show-ref", "--tags", "--verify", "--quiet", "refs/tags/"+ref)
	if err := cmd.Run(); err != nil {
		return nil, nil // Tag doesn't exist
	}

	return &release.Release{
		Version: ref,
		Date:    time.Now(),
	}, nil
}

// Changes returns all changes between two references using git log
func (s *LocalGitSummarizer) Changes(sinceRef, untilRef string) ([]change.Change, error) {
	commits, err := s.getGitCommitsBetween(sinceRef, untilRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get git commits: %w", err)
	}

	var changes []change.Change
	for _, commit := range commits {
		changeType := change.NewType(s.inferChangeTypeFromCommit(commit), change.SemVerUnknown)
		changes = append(changes, change.Change{
			Text:        commit.Subject,
			ChangeTypes: []change.Type{changeType},
			Timestamp:   commit.Date,
		})
	}

	return changes, nil
}

// ReferenceURL returns GitHub URL if the remote is GitHub, otherwise empty
func (s *LocalGitSummarizer) ReferenceURL(tag string) string {
	url := getGitHubRepositoryURL(s.repoPath)
	if url != "" && tag != "" {
		return fmt.Sprintf("%s/releases/tag/%s", url, tag)
	}
	return ""
}

// ChangesURL returns GitHub comparison URL if the remote is GitHub, otherwise empty
func (s *LocalGitSummarizer) ChangesURL(sinceRef, untilRef string) string {
	url := getGitHubRepositoryURL(s.repoPath)
	if url != "" && sinceRef != "" && untilRef != "" {
		return fmt.Sprintf("%s/compare/%s...%s", url, sinceRef, untilRef)
	}
	return ""
}

// getGitCommitsBetween gets commits between two references
func (s *LocalGitSummarizer) getGitCommitsBetween(sinceRef, untilRef string) ([]GitCommit, error) {
	// Build git log command
	args := []string{"-C", s.repoPath, "log", "--oneline", "--pretty=format:%H|%s|%an|%ai"}

	// Add range if specified
	if sinceRef != "" && untilRef != "" {
		args = append(args, fmt.Sprintf("%s..%s", sinceRef, untilRef))
	} else if sinceRef != "" {
		args = append(args, fmt.Sprintf("%s..", sinceRef))
	} else if untilRef != "" {
		args = append(args, untilRef)
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	return s.parseGitLog(string(output))
}

// parseGitLog parses git log output into GitCommit structs
func (s *LocalGitSummarizer) parseGitLog(output string) ([]GitCommit, error) {
	if strings.TrimSpace(output) == "" {
		return []GitCommit{}, nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var commits []GitCommit

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			s.logger.Debugf("Skipping malformed git log line: %s", line)
			continue
		}

		// Parse date
		date, err := time.Parse("2006-01-02 15:04:05 -0700", parts[3])
		if err != nil {
			s.logger.Debugf("Failed to parse date '%s', using current time", parts[3])
			date = time.Now()
		}

		commits = append(commits, GitCommit{
			Hash:    parts[0],
			Subject: parts[1],
			Author:  parts[2],
			Date:    date,
		})
	}

	return commits, nil
}

// inferChangeTypeFromCommit infers the change type from commit message
func (s *LocalGitSummarizer) inferChangeTypeFromCommit(commit GitCommit) string {
	subject := strings.ToLower(commit.Subject)

	// Common patterns for change type detection
	patterns := map[string][]string{
		"breaking-feature": {
			"breaking:", "break:", "breaking change", "major:",
			"!:", "feat!", "fix!", "BREAKING CHANGE:",
		},
		"security-fixes": {
			"security:", "sec:", "vulnerability", "cve-", "security fix",
		},
		"added-feature": {
			"feat:", "feature:", "add:", "new:", "implement:",
			"introduce:", "support:",
		},
		"bug-fix": {
			"fix:", "bug:", "repair:", "resolve:", "correct:",
			"hotfix:", "patch:",
		},
		"deprecated-feature": {
			"deprecate:", "deprecated:", "deprecation:",
		},
		"removed-feature": {
			"remove:", "delete:", "drop:", "eliminate:",
		},
	}

	// Check patterns in order of specificity
	for changeType, keywords := range patterns {
		for _, keyword := range keywords {
			if strings.Contains(subject, keyword) {
				return changeType
			}
		}
	}

	// Check for conventional commit format
	conventionalCommitRe := regexp.MustCompile(`^(feat|fix|docs|style|refactor|perf|test|chore|build|ci)(\(.+\))?!?:`)
	matches := conventionalCommitRe.FindStringSubmatch(subject)
	if len(matches) > 1 {
		commitType := matches[1]
		isBreaking := strings.Contains(matches[0], "!")

		if isBreaking {
			return "breaking-feature"
		}

		switch commitType {
		case "feat":
			return "added-feature"
		case "fix":
			return "bug-fix"
		case "perf":
			return "added-feature" // Performance improvements are features
		default:
			return "unknown"
		}
	}

	return "unknown"
}

// Helper functions for the main tool
func (t *GenerateChangelogTool) getGitCommitsBetween(repoPath, sinceRef, untilRef string) ([]GitCommit, error) {
	summarizer := &LocalGitSummarizer{
		repoPath: repoPath,
		logger:   logrus.New(), // Create a local logger
	}
	return summarizer.getGitCommitsBetween(sinceRef, untilRef)
}

func (t *GenerateChangelogTool) inferChangeTypeFromCommit(commit GitCommit) string {
	summarizer := &LocalGitSummarizer{
		logger: logrus.New(),
	}
	return summarizer.inferChangeTypeFromCommit(commit)
}

func (t *GenerateChangelogTool) getLastGitTag(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "describe", "--tags", "--abbrev=0")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get last git tag: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// getGitHubRepositoryURL returns the GitHub repository URL if the remote origin is GitHub
func getGitHubRepositoryURL(repoPath string) string {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	url := strings.TrimSpace(string(output))
	return normaliseGitHubURL(url)
}

// normaliseGitHubURL converts various GitHub URL formats to the standard web URL
func normaliseGitHubURL(url string) string {
	// Handle SSH format: git@github.com:owner/repo.git
	if after, ok := strings.CutPrefix(url, "git@github.com:"); ok {
		url = after
		url = "https://github.com/" + url
	}

	// Handle HTTPS format: https://github.com/owner/repo.git
	if strings.HasPrefix(url, "https://github.com/") {
		// Already correct format
	} else {
		return "" // Not a GitHub URL
	}

	// Remove .git suffix
	return strings.TrimSuffix(url, ".git")
}

// EnhancedLocalSummarizer is a local git summarizer with optional GitHub metadata enhancement
type EnhancedLocalSummarizer struct {
	repoPath    string
	logger      *logrus.Logger
	githubToken string
}

// LastRelease returns the last git tag as a release
func (s *EnhancedLocalSummarizer) LastRelease() (*release.Release, error) {
	tag, err := s.getLastGitTag()
	if err != nil || tag == "" {
		return nil, nil // No releases found
	}

	return &release.Release{
		Version: tag,
		Date:    time.Now(),
	}, nil
}

// Release returns a specific release for the given tag
func (s *EnhancedLocalSummarizer) Release(ref string) (*release.Release, error) {
	if ref == "" {
		return s.LastRelease()
	}

	// Check if the tag exists
	cmd := exec.Command("git", "-C", s.repoPath, "show-ref", "--tags", "--verify", "--quiet", "refs/tags/"+ref)
	if err := cmd.Run(); err != nil {
		return nil, nil // Tag doesn't exist
	}

	return &release.Release{
		Version: ref,
		Date:    time.Now(),
	}, nil
}

// Changes returns all changes between two references using git log with optional GitHub enhancement
func (s *EnhancedLocalSummarizer) Changes(sinceRef, untilRef string) ([]change.Change, error) {
	commits, err := s.getGitCommitsBetween(sinceRef, untilRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get git commits: %w", err)
	}

	// Process commits with deduplication and enhancement
	changes := s.processCommitsToChanges(commits)

	return changes, nil
}

// ReferenceURL returns the GitHub URL for the reference if available
func (s *EnhancedLocalSummarizer) ReferenceURL(tag string) string {
	url := getGitHubRepositoryURL(s.repoPath)
	if url != "" && tag != "" {
		return fmt.Sprintf("%s/releases/tag/%s", url, tag)
	}
	return ""
}

// ChangesURL returns the GitHub comparison URL if available
func (s *EnhancedLocalSummarizer) ChangesURL(sinceRef, untilRef string) string {
	url := getGitHubRepositoryURL(s.repoPath)
	if url != "" && sinceRef != "" && untilRef != "" {
		return fmt.Sprintf("%s/compare/%s...%s", url, sinceRef, untilRef)
	}
	return ""
}

// Helper methods that delegate to the original LocalGitSummarizer logic
func (s *EnhancedLocalSummarizer) getGitCommitsBetween(sinceRef, untilRef string) ([]GitCommit, error) {
	localSummarizer := &LocalGitSummarizer{
		repoPath: s.repoPath,
		logger:   s.logger,
	}
	return localSummarizer.getGitCommitsBetween(sinceRef, untilRef)
}

func (s *EnhancedLocalSummarizer) inferChangeTypeFromCommit(commit GitCommit) string {
	localSummarizer := &LocalGitSummarizer{
		logger: s.logger,
	}
	return localSummarizer.inferChangeTypeFromCommit(commit)
}

func (s *EnhancedLocalSummarizer) getLastGitTag() (string, error) {
	cmd := exec.Command("git", "-C", s.repoPath, "describe", "--tags", "--abbrev=0")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get last git tag: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// processCommitsToChanges processes commits into changes with deduplication and enhancement
func (s *EnhancedLocalSummarizer) processCommitsToChanges(commits []GitCommit) []change.Change {
	// Track seen messages for deduplication
	seenMessages := make(map[string]*change.Change)
	var changes []change.Change

	githubURL := getGitHubRepositoryURL(s.repoPath)
	hasGitHubIntegration := s.githubToken != "" && githubURL != ""

	for _, commit := range commits {
		// Normalise commit message for deduplication
		normalised := s.normaliseCommitMessage(commit.Subject)

		// Check if we've seen this message before
		if existingChange, exists := seenMessages[normalised]; exists {
			// Update the timestamp to the most recent
			if commit.Date.After(existingChange.Timestamp) {
				existingChange.Timestamp = s.standardiseTimestamp(commit.Date)
			}
			continue // Skip duplicate
		}

		changeType := change.NewType(s.inferChangeTypeFromCommit(commit), s.getSemanticVersionImpact(commit))

		// Enhance commit text with GitHub links if available
		changeText := s.enhanceCommitText(commit, hasGitHubIntegration, githubURL)

		newChange := change.Change{
			Text:        changeText,
			ChangeTypes: []change.Type{changeType},
			Timestamp:   s.standardiseTimestamp(commit.Date),
		}

		seenMessages[normalised] = &newChange
		changes = append(changes, newChange)
	}

	return changes
}

// normaliseCommitMessage normalises commit messages for deduplication
func (s *EnhancedLocalSummarizer) normaliseCommitMessage(message string) string {
	// Convert to lowercase and trim whitespace
	normalised := strings.ToLower(strings.TrimSpace(message))

	// Remove common prefixes that might cause false duplicates
	prefixes := []string{
		"fix:", "feat:", "chore:", "docs:", "style:", "refactor:", "test:",
		"build:", "ci:", "perf:", "revert:", "merge:", "wip:", "hotfix:",
	}

	for _, prefix := range prefixes {
		if after, ok := strings.CutPrefix(normalised, prefix); ok {
			normalised = strings.TrimSpace(after)
			break
		}
	}

	// Remove trailing punctuation and common suffixes
	normalised = strings.TrimRight(normalised, ".!?")
	normalised = strings.TrimSuffix(normalised, " (patch)")
	normalised = strings.TrimSuffix(normalised, " (minor)")
	normalised = strings.TrimSuffix(normalised, " (major)")

	return normalised
}

// standardiseTimestamp converts timestamp to UTC and rounds to minute precision
func (s *EnhancedLocalSummarizer) standardiseTimestamp(t time.Time) time.Time {
	// Convert to UTC and truncate to minute precision for consistency
	return t.UTC().Truncate(time.Minute)
}

// getSemanticVersionImpact determines the semantic version impact based on commit
func (s *EnhancedLocalSummarizer) getSemanticVersionImpact(commit GitCommit) change.SemVerKind {
	subject := strings.ToLower(commit.Subject)

	// Breaking changes (major bump)
	if strings.Contains(subject, "breaking:") || strings.Contains(subject, "breaking change") ||
		strings.Contains(subject, "!:") || strings.Contains(subject, "feat!") || strings.Contains(subject, "fix!") {
		return change.SemVerMajor
	}

	// Security fixes (patch bump)
	if strings.Contains(subject, "security:") || strings.Contains(subject, "vulnerability") ||
		strings.Contains(subject, "cve-") {
		return change.SemVerPatch
	}

	// New features (minor bump)
	if strings.Contains(subject, "feat:") || strings.Contains(subject, "feature:") ||
		strings.Contains(subject, "add:") || strings.Contains(subject, "new:") {
		return change.SemVerMinor
	}

	// Bug fixes (patch bump)
	if strings.Contains(subject, "fix:") || strings.Contains(subject, "bug:") ||
		strings.Contains(subject, "hotfix:") || strings.Contains(subject, "repair:") {
		return change.SemVerPatch
	}

	// Removed features (major bump)
	if strings.Contains(subject, "remove:") || strings.Contains(subject, "delete:") ||
		strings.Contains(subject, "drop:") {
		return change.SemVerMajor
	}

	// Deprecated features (minor bump)
	if strings.Contains(subject, "deprecate:") || strings.Contains(subject, "deprecated:") {
		return change.SemVerMinor
	}

	return change.SemVerUnknown
}

// enhanceCommitText enhances commit text with GitHub links when available
func (s *EnhancedLocalSummarizer) enhanceCommitText(commit GitCommit, hasGitHubIntegration bool, githubURL string) string {
	baseText := commit.Subject

	if !hasGitHubIntegration || githubURL == "" {
		return baseText
	}

	// Add GitHub commit link
	shortHash := commit.Hash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}

	commitURL := fmt.Sprintf("%s/commit/%s", githubURL, commit.Hash)

	// Format: "commit message ([shortHash](commitURL))"
	return fmt.Sprintf("%s ([%s](%s))", baseText, shortHash, commitURL)
}

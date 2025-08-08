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

// ReferenceURL returns empty string since we don't have a web interface
func (s *LocalGitSummarizer) ReferenceURL(tag string) string {
	return ""
}

// ChangesURL returns empty string since we don't have a web interface
func (s *LocalGitSummarizer) ChangesURL(sinceRef, untilRef string) string {
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

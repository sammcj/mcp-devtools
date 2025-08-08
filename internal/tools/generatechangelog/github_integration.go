package generatechangelog

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/sirupsen/logrus"
)

// createGitHubSummarizer creates an enhanced local summarizer with GitHub metadata
func (t *GenerateChangelogTool) createGitHubSummarizer(repoPath string, logger *logrus.Logger) (release.Summarizer, error) {
	// Check for GitHub token
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GitHub integration requires GITHUB_TOKEN environment variable")
	}

	// For now, fall back to enhanced local summarizer with GitHub metadata
	// This avoids Chronicle internal package issues while providing GitHub integration
	return t.createEnhancedLocalSummarizer(repoPath, logger, token)
}

// createEnhancedLocalSummarizer creates a local summarizer with GitHub enhancement capabilities
func (t *GenerateChangelogTool) createEnhancedLocalSummarizer(repoPath string, logger *logrus.Logger, githubToken string) (release.Summarizer, error) {
	// Create the enhanced local summarizer that can optionally fetch GitHub metadata
	return &EnhancedLocalSummarizer{
		repoPath:    repoPath,
		logger:      logger,
		githubToken: githubToken,
	}, nil
}

// createChangelogConfig creates configuration for Chronicle changelog generation
func (t *GenerateChangelogTool) createChangelogConfig(request *GenerateChangelogRequest, repoPath string) release.ChangelogInfoConfig {
	// Create change type titles for proper section formatting
	changeTypeTitles := createChangeTypeTitles()

	return release.ChangelogInfoConfig{
		RepoPath:         repoPath,
		SinceTag:         request.SinceTag,
		UntilTag:         request.UntilTag,
		ChangeTypeTitles: changeTypeTitles,
	}
}

// createChangeTypeTitles creates the section titles for different change types
func createChangeTypeTitles() []change.TypeTitle {
	return []change.TypeTitle{
		{
			ChangeType: change.NewType("security-fixes", change.SemVerPatch),
			Title:      "Security Fixes",
		},
		{
			ChangeType: change.NewType("breaking-feature", change.SemVerMajor),
			Title:      "Breaking Changes",
		},
		{
			ChangeType: change.NewType("removed-feature", change.SemVerMajor),
			Title:      "Removed Features",
		},
		{
			ChangeType: change.NewType("deprecated-feature", change.SemVerMinor),
			Title:      "Deprecated Features",
		},
		{
			ChangeType: change.NewType("added-feature", change.SemVerMinor),
			Title:      "Added Features",
		},
		{
			ChangeType: change.NewType("bug-fix", change.SemVerPatch),
			Title:      "Bug Fixes",
		},
		{
			ChangeType: change.UnknownType,
			Title:      "Additional Changes",
		},
	}
}

// executeChronicle executes chronicle to generate the changelog
func (t *GenerateChangelogTool) executeChronicle(ctx context.Context, logger *logrus.Logger, request *GenerateChangelogRequest, repoPath string) (*GenerateChangelogResponse, error) {
	var summarizer release.Summarizer
	var err error

	// Try GitHub integration if enabled
	if request.EnableGitHubIntegration {
		summarizer, err = t.createGitHubSummarizer(repoPath, logger)
		if err != nil {
			// Suppress log output in stdio mode (critical for MCP protocol)
			if logger != nil {
				logger.WithError(err).Info("GitHub integration failed, falling back to local git")
			}
			summarizer = t.createLocalGitSummarizer(repoPath, logger)
		} else {
			if logger != nil {
				logger.Info("Using GitHub integration for enhanced changelog generation")
			}
		}
	} else {
		// Use local git summarizer
		summarizer = t.createLocalGitSummarizer(repoPath, logger)
		if logger != nil {
			logger.Info("Using local git for changelog generation")
		}
	}

	// Configure changelog generation
	config := t.createChangelogConfig(request, repoPath)

	// Check if explicit range is provided, which bypasses release lookup
	hasExplicitRange := (request.SinceTag != "" && request.UntilTag != "") ||
		(request.SinceTag != "" && t.isCommitReference(request.SinceTag)) ||
		(request.UntilTag != "" && t.isCommitReference(request.UntilTag))

	var currentRelease *release.Release
	var description *release.Description

	if hasExplicitRange {
		// Use explicit range, bypass release discovery
		description, err = t.generateChangelogFromRange(request, summarizer, repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to generate changelog from range: %w", err)
		}
	} else {
		// Try Chronicle's standard approach first
		currentRelease, description, err = release.ChangelogInfo(summarizer, config)

		if err != nil {
			// Check if this is a "no releases" error - provide fallback
			if t.isNoReleaseError(err) {
				description, err = t.generateChangelogFromAllCommits(request, summarizer, repoPath)
				if err != nil {
					return nil, fmt.Errorf("failed to generate changelog fallback: %w", err)
				}
			} else {
				return nil, fmt.Errorf("failed to generate changelog: %w", err)
			}
		}
	}

	if description == nil {
		return nil, fmt.Errorf("failed to generate changelog description")
	}

	// Format output based on requested format
	var content string
	switch request.OutputFormat {
	case "markdown":
		content, err = t.formatAsMarkdown(description, request.Title)
	case "json":
		content, err = t.formatAsJSON(description)
	default:
		return nil, fmt.Errorf("unsupported output format: %s", request.OutputFormat)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to format output: %w", err)
	}

	// Get next version if speculation is enabled
	var nextVersion string
	if request.SpeculateNextVersion {
		nextVersion = t.speculateNextVersion(description, currentRelease)
	}

	// Create response
	response := &GenerateChangelogResponse{
		Content:        content,
		Format:         request.OutputFormat,
		VersionRange:   t.getVersionRangeFromDescription(description),
		ChangeCount:    len(description.Changes),
		CurrentVersion: getCurrentVersionFromDescription(description),
		NextVersion:    nextVersion,
		RepositoryURL:  description.VCSReferenceURL,
		ChangesURL:     description.VCSChangesURL,
		OutputFile:     request.OutputFile,
		GenerationTime: time.Now(),
		RepositoryPath: repoPath,
	}

	// Handle file output if specified
	if request.OutputFile != "" {
		if err := t.writeToFile(request.OutputFile, content); err != nil {
			return nil, fmt.Errorf("failed to write to file: %w", err)
		}
	}

	return response, nil
}

// getVersionRangeFromDescription extracts version range from description
func (t *GenerateChangelogTool) getVersionRangeFromDescription(description *release.Description) string {
	if description.Version != "" {
		return description.Version
	}
	return "HEAD"
}

// getCurrentVersionFromDescription extracts current version from description
func getCurrentVersionFromDescription(description *release.Description) string {
	if description.Version != "" {
		return description.Version
	}
	return "unreleased"
}

// createLocalGitSummarizer creates a local git-based summarizer for repositories without GitHub integration
func (t *GenerateChangelogTool) createLocalGitSummarizer(repoPath string, logger *logrus.Logger) release.Summarizer {
	// Get git commits for changelog generation
	commits, err := t.getGitCommitsBetween(repoPath, "", "")
	if err != nil {
		logger.WithError(err).Warn("Failed to get git commits, using minimal summarizer")
		commits = []GitCommit{}
	}

	// Convert git commits to Chronicle changes
	var changes []change.Change
	for _, commit := range commits {
		// Basic commit-based changelog entry
		changeType := change.NewType(t.inferChangeTypeFromCommit(commit), change.SemVerUnknown)
		changes = append(changes, change.Change{
			Text:        commit.Subject,
			ChangeTypes: []change.Type{changeType},
			Timestamp:   commit.Date,
		})
	}

	// Get the last tag for version info
	lastTag, err := t.getLastGitTag(repoPath)
	if err != nil {
		logger.WithError(err).Debug("No git tags found, starting from beginning of history")
		lastTag = ""
	}

	return &LocalGitSummarizer{
		repoPath: repoPath,
		lastTag:  lastTag,
		changes:  changes,
		logger:   logger,
	}
}

// isCommitReference checks if a string looks like a commit reference (e.g., HEAD~5, abc123, HEAD^)
func (t *GenerateChangelogTool) isCommitReference(ref string) bool {
	if ref == "" {
		return false
	}

	// Common commit reference patterns
	patterns := []string{
		"HEAD", "HEAD~", "HEAD^",
	}

	for _, pattern := range patterns {
		if strings.HasPrefix(ref, pattern) {
			return true
		}
	}

	// Check if it looks like a SHA (hex string, at least 6 chars)
	if len(ref) >= 6 && len(ref) <= 40 {
		for _, r := range ref {
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
				return false
			}
		}
		return true
	}

	return false
}

// isNoReleaseError checks if the error is due to missing releases
func (t *GenerateChangelogTool) isNoReleaseError(err error) bool {
	if err == nil {
		return false
	}
	errorMsg := strings.ToLower(err.Error())
	return strings.Contains(errorMsg, "unable to determine last release") ||
		strings.Contains(errorMsg, "unable to fetch release") ||
		strings.Contains(errorMsg, "no releases found") ||
		strings.Contains(errorMsg, "no tags found")
}

// generateChangelogFromRange generates changelog from explicit commit range
func (t *GenerateChangelogTool) generateChangelogFromRange(request *GenerateChangelogRequest, summarizer release.Summarizer, repoPath string) (*release.Description, error) {
	// Get changes between the specified range
	changes, err := summarizer.Changes(request.SinceTag, request.UntilTag)
	if err != nil {
		return nil, fmt.Errorf("failed to get changes for range %s..%s: %w", request.SinceTag, request.UntilTag, err)
	}

	return &release.Description{
		Release: release.Release{
			Version: request.UntilTag,
			Date:    time.Now(),
		},
		Changes:         changes,
		VCSReferenceURL: summarizer.ReferenceURL(request.UntilTag),
		VCSChangesURL:   summarizer.ChangesURL(request.SinceTag, request.UntilTag),
	}, nil
}

// generateChangelogFromAllCommits generates changelog from all commits when no releases exist
func (t *GenerateChangelogTool) generateChangelogFromAllCommits(request *GenerateChangelogRequest, summarizer release.Summarizer, repoPath string) (*release.Description, error) {
	// Get all changes from beginning of time to HEAD
	since := ""
	until := "HEAD"

	// If since_tag is provided, use it
	if request.SinceTag != "" {
		since = request.SinceTag
	}

	// If until_tag is provided, use it
	if request.UntilTag != "" {
		until = request.UntilTag
	}

	changes, err := summarizer.Changes(since, until)
	if err != nil {
		return nil, fmt.Errorf("failed to get all changes: %w", err)
	}

	return &release.Description{
		Release: release.Release{
			Version: "(Unreleased)",
			Date:    time.Now(),
		},
		Changes:         changes,
		VCSReferenceURL: summarizer.ReferenceURL(until),
		VCSChangesURL:   summarizer.ChangesURL(since, until),
	}, nil
}

// speculateNextVersion analyzes changes to predict the next semantic version
func (t *GenerateChangelogTool) speculateNextVersion(description *release.Description, currentRelease *release.Release) string {
	if description == nil {
		return ""
	}

	// Get the current version, default to 0.0.0 if no release exists
	currentVersion := "0.0.0"
	if currentRelease != nil && currentRelease.Version != "" {
		currentVersion = strings.TrimPrefix(currentRelease.Version, "v")
	}

	// Parse the current version
	major, minor, patch, err := parseSemanticVersion(currentVersion)
	if err != nil {
		// If we can't parse the current version, return empty
		return ""
	}

	// Analyze changes to determine version bump type
	hasMajor := false
	hasMinor := false
	hasPatch := false

	for _, ch := range description.Changes {
		for _, changeType := range ch.ChangeTypes {
			switch changeType.Kind {
			case change.SemVerMajor:
				hasMajor = true
			case change.SemVerMinor:
				hasMinor = true
			case change.SemVerPatch:
				hasPatch = true
			}
		}
	}

	// Determine the next version based on semantic versioning rules
	if hasMajor {
		major++
		minor = 0
		patch = 0
	} else if hasMinor {
		minor++
		patch = 0
	} else if hasPatch || len(description.Changes) > 0 {
		// If there are any changes but no explicit semantic impact, bump patch
		patch++
	} else {
		// No changes, return current version
		return currentVersion
	}

	nextVersion := fmt.Sprintf("%d.%d.%d", major, minor, patch)

	// If original version had 'v' prefix, maintain it
	if currentRelease != nil && strings.HasPrefix(currentRelease.Version, "v") {
		nextVersion = "v" + nextVersion
	}

	return nextVersion
}

// parseSemanticVersion parses a semantic version string into major.minor.patch components
func parseSemanticVersion(version string) (major, minor, patch int, err error) {
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	// Handle pre-release and build metadata by removing them
	if idx := strings.Index(version, "-"); idx != -1 {
		version = version[:idx]
	}
	if idx := strings.Index(version, "+"); idx != -1 {
		version = version[:idx]
	}

	// Use regex to match semantic version pattern
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)
	matches := re.FindStringSubmatch(version)

	if len(matches) != 4 {
		return 0, 0, 0, fmt.Errorf("invalid semantic version: %s", version)
	}

	major, err = strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version: %s", matches[1])
	}

	minor, err = strconv.Atoi(matches[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version: %s", matches[2])
	}

	patch, err = strconv.Atoi(matches[3])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid patch version: %s", matches[3])
	}

	return major, minor, patch, nil
}

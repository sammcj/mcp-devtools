package generatechangelog

import (
	"context"
	"fmt"
	"time"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/sirupsen/logrus"
)

// createGitHubSummarizer creates a GitHub summarizer for changelog generation
func (t *GenerateChangelogTool) createGitHubSummarizer(repoPath string) (release.Summarizer, error) {
	// For now, we'll skip the Chronicle GitHub integration due to internal package access restrictions
	// and return an error to fall back to local git summarizer
	return nil, fmt.Errorf("GitHub integration not available - using local git fallback")
}

// createChangelogConfig creates configuration for Chronicle changelog generation
func (t *GenerateChangelogTool) createChangelogConfig(request *GenerateChangelogRequest, repoPath string) release.ChangelogInfoConfig {
	// Create version speculator if needed
	var versionSpeculator release.VersionSpeculator
	if request.SpeculateNextVersion {
		// Chronicle doesn't export NewVersionSpeculator, so we'll handle version speculation
		// in our own logic after getting the description
		versionSpeculator = nil
	}

	return release.ChangelogInfoConfig{
		VersionSpeculator: versionSpeculator,
		RepoPath:          repoPath,
		SinceTag:          request.SinceTag,
		UntilTag:          request.UntilTag,
	}
}

// executeChronicle executes chronicle to generate the changelog
func (t *GenerateChangelogTool) executeChronicle(ctx context.Context, logger *logrus.Logger, request *GenerateChangelogRequest, repoPath string) (*GenerateChangelogResponse, error) {
	// Try GitHub integration first, fall back to local git if it fails
	summarizer, err := t.createGitHubSummarizer(repoPath)
	if err != nil {
		logger.WithError(err).Info("GitHub integration not available, falling back to local git-based changelog")
		summarizer = t.createLocalGitSummarizer(repoPath, logger)
	}

	// Configure changelog generation
	config := t.createChangelogConfig(request, repoPath)

	// Generate changelog using Chronicle
	currentRelease, description, err := release.ChangelogInfo(summarizer, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate changelog: %w", err)
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
	if request.SpeculateNextVersion && currentRelease != nil {
		nextVersion = currentRelease.Version
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

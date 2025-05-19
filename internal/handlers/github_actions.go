package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

// GitHubActionsHandler handles GitHub Actions version checking
type GitHubActionsHandler struct {
	client HTTPClient
	cache  *sync.Map
	logger *logrus.Logger
}

// NewGitHubActionsHandler creates a new GitHub Actions handler
func NewGitHubActionsHandler(logger *logrus.Logger, cache *sync.Map) *GitHubActionsHandler {
	if cache == nil {
		cache = &sync.Map{}
	}
	return &GitHubActionsHandler{
		client: DefaultHTTPClient,
		cache:  cache,
		logger: logger,
	}
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	HTMLURL     string `json:"html_url"`
}

// GetLatestVersion gets the latest version of GitHub Actions
func (h *GitHubActionsHandler) GetLatestVersion(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	h.logger.Info("Getting latest GitHub Actions versions")

	// Parse actions
	actionsRaw, ok := args["actions"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: actions")
	}

	// Convert to []GitHubAction
	var actions []GitHubAction
	if actionsArr, ok := actionsRaw.([]interface{}); ok {
		for _, actionRaw := range actionsArr {
			if actionMap, ok := actionRaw.(map[string]interface{}); ok {
				var action GitHubAction
				if owner, ok := actionMap["owner"].(string); ok {
					action.Owner = owner
				} else {
					continue
				}
				if repo, ok := actionMap["repo"].(string); ok {
					action.Repo = repo
				} else {
					continue
				}
				if version, ok := actionMap["currentVersion"].(string); ok {
					action.CurrentVersion = StringPtr(version)
				}
				actions = append(actions, action)
			}
		}
	} else {
		return nil, fmt.Errorf("invalid actions format: expected array")
	}

	// Parse include details
	includeDetails := false
	if includeDetailsRaw, ok := args["includeDetails"].(bool); ok {
		includeDetails = includeDetailsRaw
	}

	// Process each action
	results := make([]GitHubActionVersion, 0, len(actions))
	for _, action := range actions {
		h.logger.WithFields(logrus.Fields{
			"owner": action.Owner,
			"repo":  action.Repo,
		}).Debug("Processing GitHub Action")

		// Get latest version
		latestVersion, publishedAt, url, err := h.getLatestVersion(action.Owner, action.Repo)
		if err != nil {
			h.logger.WithFields(logrus.Fields{
				"owner": action.Owner,
				"repo":  action.Repo,
				"error": err.Error(),
			}).Error("Failed to get GitHub Action info")
			results = append(results, GitHubActionVersion{
				Owner:          action.Owner,
				Repo:           action.Repo,
				CurrentVersion: action.CurrentVersion,
				LatestVersion:  "unknown",
			})
			continue
		}

		// Add result
		result := GitHubActionVersion{
			Owner:          action.Owner,
			Repo:           action.Repo,
			CurrentVersion: action.CurrentVersion,
			LatestVersion:  latestVersion,
		}

		// Add details if requested
		if includeDetails {
			result.PublishedAt = StringPtr(publishedAt)
			result.URL = StringPtr(url)
		}

		results = append(results, result)
	}

	// Sort results by owner/repo
	sort.Slice(results, func(i, j int) bool {
		ownerI := strings.ToLower(results[i].Owner)
		ownerJ := strings.ToLower(results[j].Owner)
		if ownerI != ownerJ {
			return ownerI < ownerJ
		}
		return strings.ToLower(results[i].Repo) < strings.ToLower(results[j].Repo)
	})

	return NewToolResultJSON(results)
}

// getLatestVersion gets the latest version of a GitHub Action
func (h *GitHubActionsHandler) getLatestVersion(owner, repo string) (version, publishedAt, url string, err error) {
	// Check cache first
	cacheKey := fmt.Sprintf("github-action:%s/%s", owner, repo)
	if cachedInfo, ok := h.cache.Load(cacheKey); ok {
		h.logger.WithFields(logrus.Fields{
			"owner": owner,
			"repo":  repo,
		}).Debug("Using cached GitHub Action info")
		info := cachedInfo.(map[string]string)
		return info["version"], info["publishedAt"], info["url"], nil
	}

	// Construct URL
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", owner, repo)
	h.logger.WithFields(logrus.Fields{
		"owner":  owner,
		"repo":   repo,
		"apiURL": apiURL,
	}).Debug("Fetching GitHub Action releases")

	// Make request
	headers := map[string]string{
		"Accept": "application/vnd.github.v3+json",
	}
	body, err := MakeRequestWithLogger(h.client, h.logger, "GET", apiURL, headers)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch GitHub Action releases: %w", err)
	}

	// Parse response
	var releases []GitHubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", "", "", fmt.Errorf("failed to parse GitHub Action releases: %w", err)
	}

	// Find latest non-draft, non-prerelease version
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}

		// Cache result
		info := map[string]string{
			"version":     release.TagName,
			"publishedAt": release.PublishedAt,
			"url":         release.HTMLURL,
		}
		h.cache.Store(cacheKey, info)

		return release.TagName, release.PublishedAt, release.HTMLURL, nil
	}

	// If no releases found, try tags
	tagsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags", owner, repo)
	h.logger.WithFields(logrus.Fields{
		"owner":   owner,
		"repo":    repo,
		"tagsURL": tagsURL,
	}).Debug("Fetching GitHub Action tags")

	// Make request
	body, err = MakeRequestWithLogger(h.client, h.logger, "GET", tagsURL, headers)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch GitHub Action tags: %w", err)
	}

	// Parse response
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &tags); err != nil {
		return "", "", "", fmt.Errorf("failed to parse GitHub Action tags: %w", err)
	}

	// Find latest version
	if len(tags) > 0 {
		// Cache result
		url := fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", owner, repo, tags[0].Name)
		info := map[string]string{
			"version":     tags[0].Name,
			"publishedAt": "",
			"url":         url,
		}
		h.cache.Store(cacheKey, info)

		return tags[0].Name, "", url, nil
	}

	return "", "", "", fmt.Errorf("no releases or tags found for: %s/%s", owner, repo)
}

package githubactions

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

// GitHubActionsTool handles GitHub Actions version checking
type GitHubActionsTool struct {
	client packageversions.HTTPClient
}

// Definition returns the tool's definition for MCP registration
func (t *GitHubActionsTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"check_github_actions",
		mcp.WithDescription("Check latest versions for GitHub Actions"),
		mcp.WithArray("actions",
			mcp.Description("Array of GitHub Actions to check"),
			mcp.Required(),
		),
		mcp.WithBoolean("includeDetails",
			mcp.Description("Include additional details like published date and URL"),
			mcp.DefaultBool(false),
		),
	)
}

// Execute executes the tool's logic
func (t *GitHubActionsTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Checking GitHub Actions versions")

	// Parse actions
	actionsRaw, ok := args["actions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing required parameter: actions")
	}

	// Parse include details
	includeDetails := false
	if includeDetailsRaw, ok := args["includeDetails"].(bool); ok {
		includeDetails = includeDetailsRaw
	}

	// Convert to GitHubAction
	var actions []packageversions.GitHubAction
	for _, actionRaw := range actionsRaw {
		if actionMap, ok := actionRaw.(map[string]interface{}); ok {
			var action packageversions.GitHubAction

			// Parse owner
			if owner, ok := actionMap["owner"].(string); ok && owner != "" {
				action.Owner = owner
			} else {
				return nil, fmt.Errorf("missing required parameter: owner")
			}

			// Parse repo
			if repo, ok := actionMap["repo"].(string); ok && repo != "" {
				action.Repo = repo
			} else {
				return nil, fmt.Errorf("missing required parameter: repo")
			}

			// Parse current version
			if currentVersion, ok := actionMap["currentVersion"].(string); ok && currentVersion != "" {
				action.CurrentVersion = packageversions.StringPtr(currentVersion)
			}

			actions = append(actions, action)
		} else if actionStr, ok := actionRaw.(string); ok {
			// Parse action string (e.g., "owner/repo@v1.0.0")
			parts := strings.Split(actionStr, "@")
			if len(parts) == 0 {
				return nil, fmt.Errorf("invalid action format: %s", actionStr)
			}

			// Parse owner/repo
			ownerRepo := parts[0]
			ownerRepoParts := strings.Split(ownerRepo, "/")
			if len(ownerRepoParts) != 2 {
				return nil, fmt.Errorf("invalid action format: %s", actionStr)
			}

			action := packageversions.GitHubAction{
				Owner: ownerRepoParts[0],
				Repo:  ownerRepoParts[1],
			}

			// Parse current version
			if len(parts) > 1 {
				action.CurrentVersion = packageversions.StringPtr(parts[1])
			}

			actions = append(actions, action)
		} else {
			return nil, fmt.Errorf("invalid action format")
		}
	}

	// Get latest versions
	results, err := t.getLatestVersions(logger, cache, actions, includeDetails)
	if err != nil {
		return nil, err
	}

	return packageversions.NewToolResultJSON(results)
}

// getLatestVersions gets the latest versions for GitHub Actions
func (t *GitHubActionsTool) getLatestVersions(logger *logrus.Logger, cache *sync.Map, actions []packageversions.GitHubAction, includeDetails bool) ([]packageversions.GitHubActionVersion, error) {
	var results []packageversions.GitHubActionVersion

	for _, action := range actions {
		// Check cache first
		cacheKey := fmt.Sprintf("github-action:%s/%s", action.Owner, action.Repo)
		if cachedVersion, ok := cache.Load(cacheKey); ok {
			logger.WithFields(logrus.Fields{
				"owner": action.Owner,
				"repo":  action.Repo,
			}).Debug("Using cached GitHub Action version")
			result := cachedVersion.(packageversions.GitHubActionVersion)
			result.CurrentVersion = action.CurrentVersion
			results = append(results, result)
			continue
		}

		// Get latest version
		latestVersion, publishedAt, url, err := t.getLatestVersion(logger, action.Owner, action.Repo)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"owner": action.Owner,
				"repo":  action.Repo,
				"error": err.Error(),
			}).Error("Failed to get GitHub Action version")
			results = append(results, packageversions.GitHubActionVersion{
				Owner:          action.Owner,
				Repo:           action.Repo,
				CurrentVersion: action.CurrentVersion,
				LatestVersion:  "unknown",
			})
			continue
		}

		// Create result
		result := packageversions.GitHubActionVersion{
			Owner:          action.Owner,
			Repo:           action.Repo,
			CurrentVersion: action.CurrentVersion,
			LatestVersion:  latestVersion,
		}

		// Add details if requested
		if includeDetails {
			result.PublishedAt = packageversions.StringPtr(publishedAt)
			result.URL = packageversions.StringPtr(url)
		}

		// Cache result
		cache.Store(cacheKey, result)

		results = append(results, result)
	}

	// Sort results by owner/repo
	sort.Slice(results, func(i, j int) bool {
		ownerRepoI := fmt.Sprintf("%s/%s", results[i].Owner, results[i].Repo)
		ownerRepoJ := fmt.Sprintf("%s/%s", results[j].Owner, results[j].Repo)
		return strings.ToLower(ownerRepoI) < strings.ToLower(ownerRepoJ)
	})

	return results, nil
}

// getLatestVersion gets the latest version for a GitHub Action
func (t *GitHubActionsTool) getLatestVersion(logger *logrus.Logger, owner, repo string) (string, string, string, error) {
	// Construct URL
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	logger.WithFields(logrus.Fields{
		"owner": owner,
		"repo":  repo,
		"url":   apiURL,
	}).Debug("Fetching GitHub Action version")

	// Make request
	headers := map[string]string{
		"Accept": "application/vnd.github.v3+json",
	}
	body, err := packageversions.MakeRequestWithLogger(t.client, logger, "GET", apiURL, headers)
	if err != nil {
		// Try tags if releases not found
		return t.getLatestTag(logger, owner, repo)
	}

	// Parse response
	var release struct {
		TagName     string    `json:"tag_name"`
		PublishedAt time.Time `json:"published_at"`
		HTMLURL     string    `json:"html_url"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return "", "", "", fmt.Errorf("failed to parse GitHub release: %w", err)
	}

	// Format published date
	publishedAt := release.PublishedAt.Format(time.RFC3339)

	return release.TagName, publishedAt, release.HTMLURL, nil
}

// getLatestTag gets the latest tag for a GitHub Action
func (t *GitHubActionsTool) getLatestTag(logger *logrus.Logger, owner, repo string) (string, string, string, error) {
	// Construct URL
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags", owner, repo)
	logger.WithFields(logrus.Fields{
		"owner": owner,
		"repo":  repo,
		"url":   apiURL,
	}).Debug("Fetching GitHub Action tags")

	// Make request
	headers := map[string]string{
		"Accept": "application/vnd.github.v3+json",
	}
	body, err := packageversions.MakeRequestWithLogger(t.client, logger, "GET", apiURL, headers)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch GitHub tags: %w", err)
	}

	// Parse response
	var tags []struct {
		Name       string `json:"name"`
		ZipballURL string `json:"zipball_url"`
		TarballURL string `json:"tarball_url"`
		Commit     struct {
			SHA string `json:"sha"`
			URL string `json:"url"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &tags); err != nil {
		return "", "", "", fmt.Errorf("failed to parse GitHub tags: %w", err)
	}

	// Check if tags exist
	if len(tags) == 0 {
		return "", "", "", fmt.Errorf("no tags found")
	}

	// Get latest tag
	latestTag := tags[0].Name
	url := fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", owner, repo, latestTag)

	// We don't have published date for tags, so use current time
	publishedAt := time.Now().Format(time.RFC3339)

	return latestTag, publishedAt, url, nil
}

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

// SwiftHandler handles Swift package version checking
type SwiftHandler struct {
	client HTTPClient
	cache  *sync.Map
	logger *logrus.Logger
}

// NewSwiftHandler creates a new Swift handler
func NewSwiftHandler(logger *logrus.Logger, cache *sync.Map) *SwiftHandler {
	if cache == nil {
		cache = &sync.Map{}
	}
	return &SwiftHandler{
		client: DefaultHTTPClient,
		cache:  cache,
		logger: logger,
	}
}

// GitHubReleaseResponse represents a response from the GitHub API for releases
type GitHubReleaseResponse []struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Draft   bool   `json:"draft"`
	Prerelease bool `json:"prerelease"`
	PublishedAt string `json:"published_at"`
}

// GetLatestVersion gets the latest version of Swift packages
func (h *SwiftHandler) GetLatestVersion(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	h.logger.Info("Getting latest Swift package versions")

	// Parse dependencies
	depsRaw, ok := args["dependencies"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: dependencies")
	}

	// Convert to []SwiftDependency
	var deps []SwiftDependency
	if depsArr, ok := depsRaw.([]interface{}); ok {
		for _, depRaw := range depsArr {
			if depMap, ok := depRaw.(map[string]interface{}); ok {
				var dep SwiftDependency
				if url, ok := depMap["url"].(string); ok {
					dep.URL = url
				} else {
					continue
				}
				if version, ok := depMap["version"].(string); ok {
					dep.Version = version
				}
				if requirement, ok := depMap["requirement"].(string); ok {
					dep.Requirement = requirement
				}
				deps = append(deps, dep)
			}
		}
	} else {
		return nil, fmt.Errorf("invalid dependencies format: expected array")
	}

	// Parse constraints
	var constraints VersionConstraints
	if constraintsRaw, ok := args["constraints"]; ok {
		if constraintsMap, ok := constraintsRaw.(map[string]interface{}); ok {
			constraints = make(VersionConstraints)
			for name, constraintRaw := range constraintsMap {
				if constraintMap, ok := constraintRaw.(map[string]interface{}); ok {
					var constraint VersionConstraint
					if majorVersion, ok := constraintMap["majorVersion"].(float64); ok {
						majorInt := int(majorVersion)
						constraint.MajorVersion = &majorInt
					}
					if excludePackage, ok := constraintMap["excludePackage"].(bool); ok {
						constraint.ExcludePackage = excludePackage
					}
					constraints[name] = constraint
				}
			}
		}
	}

	// Process each dependency
	results := make([]PackageVersion, 0, len(deps))
	for _, dep := range deps {
		h.logger.WithFields(logrus.Fields{
			"url":     dep.URL,
			"version": dep.Version,
		}).Debug("Processing Swift package")

		// Check if package should be excluded
		if constraint, ok := constraints[dep.URL]; ok && constraint.ExcludePackage {
			results = append(results, PackageVersion{
				Name:       dep.URL,
				Skipped:    true,
				SkipReason: "Package excluded by constraints",
			})
			continue
		}

		// Get latest version
		latestVersion, err := h.getLatestVersion(dep.URL)
		if err != nil {
			h.logger.WithFields(logrus.Fields{
				"url":   dep.URL,
				"error": err.Error(),
			}).Error("Failed to get Swift package info")
			results = append(results, PackageVersion{
				Name:           dep.URL,
				CurrentVersion: StringPtr(dep.Version),
				LatestVersion:  "unknown",
				Registry:       "swift",
				Skipped:        true,
				SkipReason:     fmt.Sprintf("Failed to fetch package info: %v", err),
			})
			continue
		}

		// Apply major version constraint if specified
		if constraint, ok := constraints[dep.URL]; ok && constraint.MajorVersion != nil {
			targetMajor := *constraint.MajorVersion
			latestMajor, _, _, err := ParseVersion(latestVersion)
			if err == nil && latestMajor > targetMajor {
				// Find the latest version with the target major version
				h.logger.WithFields(logrus.Fields{
					"url":          dep.URL,
					"targetMajor":  targetMajor,
					"latestMajor":  latestMajor,
					"latestVersion": latestVersion,
				}).Debug("Applying major version constraint")

				// In a real implementation, this would fetch all versions and filter by major version
				// For now, we'll just append the major version
				latestVersion = fmt.Sprintf("%d.0.0", targetMajor)
			}
		}

		// Add result
		results = append(results, PackageVersion{
			Name:           dep.URL,
			CurrentVersion: StringPtr(dep.Version),
			LatestVersion:  latestVersion,
			Registry:       "swift",
		})
	}

	// Sort results by name
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return NewToolResultJSON(results)
}

// getLatestVersion gets the latest version of a Swift package
func (h *SwiftHandler) getLatestVersion(packageURL string) (string, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("swift:%s", packageURL)
	if cachedVersion, ok := h.cache.Load(cacheKey); ok {
		h.logger.WithField("url", packageURL).Debug("Using cached Swift package version")
		return cachedVersion.(string), nil
	}

	// Parse GitHub URL
	if !strings.Contains(packageURL, "github.com") {
		return "", fmt.Errorf("only GitHub URLs are supported: %s", packageURL)
	}

	// Extract owner and repo
	parts := strings.Split(packageURL, "/")
	if len(parts) < 5 {
		return "", fmt.Errorf("invalid GitHub URL format: %s", packageURL)
	}

	owner := parts[3]
	repo := parts[4]

	// Construct API URL
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", owner, repo)
	h.logger.WithFields(logrus.Fields{
		"url":    packageURL,
		"apiURL": apiURL,
	}).Debug("Fetching Swift package releases")

	// Make request
	headers := map[string]string{
		"Accept": "application/vnd.github.v3+json",
	}
	body, err := MakeRequestWithLogger(h.client, h.logger, "GET", apiURL, headers)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Swift package releases: %w", err)
	}

	// Parse response
	var releases GitHubReleaseResponse
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("failed to parse Swift package releases: %w", err)
	}

	// Find latest non-draft, non-prerelease version
	var latestVersion string
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}

		version := strings.TrimPrefix(release.TagName, "v")
		if latestVersion == "" {
			latestVersion = version
			continue
		}

		// Compare versions
		result, err := CompareVersions(version, latestVersion)
		if err != nil {
			h.logger.WithFields(logrus.Fields{
				"version1": version,
				"version2": latestVersion,
				"error":    err.Error(),
			}).Debug("Failed to compare versions")
			continue
		}

		if result > 0 {
			latestVersion = version
		}
	}

	if latestVersion == "" {
		// If no releases found, try tags
		tagsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags", owner, repo)
		h.logger.WithFields(logrus.Fields{
			"url":     packageURL,
			"tagsURL": tagsURL,
		}).Debug("Fetching Swift package tags")

		// Make request
		body, err := MakeRequestWithLogger(h.client, h.logger, "GET", tagsURL, headers)
		if err != nil {
			return "", fmt.Errorf("failed to fetch Swift package tags: %w", err)
		}

		// Parse response
		var tags []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(body, &tags); err != nil {
			return "", fmt.Errorf("failed to parse Swift package tags: %w", err)
		}

		// Find latest version
		for _, tag := range tags {
			version := strings.TrimPrefix(tag.Name, "v")
			if latestVersion == "" {
				latestVersion = version
				continue
			}

			// Compare versions
			result, err := CompareVersions(version, latestVersion)
			if err != nil {
				h.logger.WithFields(logrus.Fields{
					"version1": version,
					"version2": latestVersion,
					"error":    err.Error(),
				}).Debug("Failed to compare versions")
				continue
			}

			if result > 0 {
				latestVersion = version
			}
		}
	}

	if latestVersion == "" {
		return "", fmt.Errorf("no releases or tags found for: %s", packageURL)
	}

	// Cache result
	h.cache.Store(cacheKey, latestVersion)

	return latestVersion, nil
}

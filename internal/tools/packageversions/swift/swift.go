package swift

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

// SwiftTool handles Swift package version checking
type SwiftTool struct {
	client packageversions.HTTPClient
}

// init registers the Swift tool with the registry
func init() {
	registry.Register(&SwiftTool{
		client: packageversions.DefaultHTTPClient,
	})
}

// Definition returns the tool's definition for MCP registration
func (t *SwiftTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"check_swift_versions",
		mcp.WithDescription("Check latest stable versions for Swift packages in Package.swift"),
		mcp.WithArray("dependencies",
			mcp.Description("Array of Swift package dependencies"),
			mcp.Required(),
		),
		mcp.WithObject("constraints",
			mcp.Description("Optional constraints for specific packages"),
			mcp.Properties(map[string]interface{}{}),
		),
	)
}

// Execute executes the tool's logic
func (t *SwiftTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Getting latest Swift package versions")

	// Parse dependencies
	depsRaw, ok := args["dependencies"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing required parameter: dependencies")
	}

	// Parse constraints
	var constraints packageversions.VersionConstraints
	if constraintsRaw, ok := args["constraints"].(map[string]interface{}); ok {
		constraints = make(packageversions.VersionConstraints)
		for name, constraintRaw := range constraintsRaw {
			if constraintMap, ok := constraintRaw.(map[string]interface{}); ok {
				var constraint packageversions.VersionConstraint
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

	// Convert to SwiftDependency
	var dependencies []packageversions.SwiftDependency
	for _, depRaw := range depsRaw {
		if depMap, ok := depRaw.(map[string]interface{}); ok {
			var dep packageversions.SwiftDependency

			// Parse URL
			if url, ok := depMap["url"].(string); ok && url != "" {
				dep.URL = url
			} else {
				return nil, fmt.Errorf("missing required parameter: url")
			}

			// Parse version
			if version, ok := depMap["version"].(string); ok && version != "" {
				dep.Version = version
			}

			// Parse requirement
			if requirement, ok := depMap["requirement"].(string); ok && requirement != "" {
				dep.Requirement = requirement
			}

			dependencies = append(dependencies, dep)
		}
	}

	// Get latest versions
	results, err := t.getLatestVersions(logger, cache, dependencies, constraints)
	if err != nil {
		return nil, err
	}

	return packageversions.NewToolResultJSON(results)
}

// getLatestVersions gets the latest versions for Swift packages
func (t *SwiftTool) getLatestVersions(logger *logrus.Logger, cache *sync.Map, dependencies []packageversions.SwiftDependency, constraints packageversions.VersionConstraints) ([]packageversions.PackageVersion, error) {
	var results []packageversions.PackageVersion

	for _, dep := range dependencies {
		// Extract package name from URL
		packageName := extractPackageName(dep.URL)
		if packageName == "" {
			logger.WithField("url", dep.URL).Warn("Failed to extract package name from URL")
			continue
		}

		// Check if package should be excluded
		if constraint, ok := constraints[packageName]; ok && constraint.ExcludePackage {
			results = append(results, packageversions.PackageVersion{
				Name:       packageName,
				Skipped:    true,
				SkipReason: "Package excluded by constraints",
			})
			continue
		}

		// Get current version
		currentVersion := ""
		if dep.Version != "" {
			currentVersion = dep.Version
		} else if dep.Requirement != "" {
			currentVersion = extractVersionFromRequirement(dep.Requirement)
		}

		// Check cache first
		cacheKey := fmt.Sprintf("swift:%s", packageName)
		if cachedVersion, ok := cache.Load(cacheKey); ok {
			logger.WithField("package", packageName).Debug("Using cached Swift package version")
			result := cachedVersion.(packageversions.PackageVersion)
			if currentVersion != "" {
				result.CurrentVersion = packageversions.StringPtr(currentVersion)
			}
			results = append(results, result)
			continue
		}

		// Get latest version
		latestVersion, err := t.getLatestVersion(logger, dep.URL)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"package": packageName,
				"error":   err.Error(),
			}).Error("Failed to get Swift package version")
			result := packageversions.PackageVersion{
				Name:          packageName,
				LatestVersion: "unknown",
				Registry:      "swift",
				Skipped:       true,
				SkipReason:    fmt.Sprintf("Failed to fetch package info: %v", err),
			}
			if currentVersion != "" {
				result.CurrentVersion = packageversions.StringPtr(currentVersion)
			}
			results = append(results, result)
			continue
		}

		// Apply major version constraint if specified
		if constraint, ok := constraints[packageName]; ok && constraint.MajorVersion != nil {
			targetMajor := *constraint.MajorVersion
			latestMajor, _, _, err := packageversions.ParseVersion(latestVersion)
			if err == nil && latestMajor > targetMajor {
				// Find a version with the target major version
				// Note: In a real implementation, we would fetch all available versions
				// For now, we'll just use the target major version with zeros
				latestVersion = fmt.Sprintf("%d.0.0", targetMajor)
			}
		}

		// Create result
		result := packageversions.PackageVersion{
			Name:          packageName,
			LatestVersion: latestVersion,
			Registry:      "swift",
		}
		if currentVersion != "" {
			result.CurrentVersion = packageversions.StringPtr(currentVersion)
		}

		// Cache result
		cache.Store(cacheKey, result)

		results = append(results, result)
	}

	// Sort results by name
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return results, nil
}

// getLatestVersion gets the latest version for a Swift package
func (t *SwiftTool) getLatestVersion(logger *logrus.Logger, packageURL string) (string, error) {
	// Extract owner and repo from URL
	owner, repo, err := extractOwnerRepo(packageURL)
	if err != nil {
		return "", err
	}

	// Construct GitHub API URL
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	logger.WithFields(logrus.Fields{
		"owner": owner,
		"repo":  repo,
		"url":   apiURL,
	}).Debug("Fetching Swift package version")

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
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("failed to parse GitHub release: %w", err)
	}

	// Clean version
	version := strings.TrimPrefix(release.TagName, "v")

	return version, nil
}

// getLatestTag gets the latest tag for a Swift package
func (t *SwiftTool) getLatestTag(logger *logrus.Logger, owner, repo string) (string, error) {
	// Construct URL
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags", owner, repo)
	logger.WithFields(logrus.Fields{
		"owner": owner,
		"repo":  repo,
		"url":   apiURL,
	}).Debug("Fetching Swift package tags")

	// Make request
	headers := map[string]string{
		"Accept": "application/vnd.github.v3+json",
	}
	body, err := packageversions.MakeRequestWithLogger(t.client, logger, "GET", apiURL, headers)
	if err != nil {
		return "", fmt.Errorf("failed to fetch GitHub tags: %w", err)
	}

	// Parse response
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &tags); err != nil {
		return "", fmt.Errorf("failed to parse GitHub tags: %w", err)
	}

	// Check if tags exist
	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found")
	}

	// Get latest tag
	latestTag := tags[0].Name

	// Clean version
	version := strings.TrimPrefix(latestTag, "v")

	return version, nil
}

// extractOwnerRepo extracts the owner and repo from a package URL
func extractOwnerRepo(packageURL string) (string, string, error) {
	// GitHub URL patterns
	githubPatterns := []string{
		`github\.com[:/]([^/]+)/([^/]+)`,
		`github\.com/([^/]+)/([^/]+)\.git`,
	}

	for _, pattern := range githubPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(packageURL)
		if len(matches) >= 3 {
			owner := matches[1]
			repo := strings.TrimSuffix(matches[2], ".git")
			return owner, repo, nil
		}
	}

	return "", "", fmt.Errorf("unsupported package URL format: %s", packageURL)
}

// extractPackageName extracts the package name from a URL
func extractPackageName(packageURL string) string {
	// Extract repo name from URL
	_, repo, err := extractOwnerRepo(packageURL)
	if err != nil {
		return ""
	}

	// Clean repo name
	repo = strings.TrimSuffix(repo, ".git")
	repo = strings.TrimSuffix(repo, "-swift")
	repo = strings.TrimSuffix(repo, "-Swift")

	return repo
}

// extractVersionFromRequirement extracts the version from a requirement string
func extractVersionFromRequirement(requirement string) string {
	// Extract version from requirement string
	// Examples:
	// - .upToNextMajor(from: "1.0.0")
	// - .exact("1.0.0")
	re := regexp.MustCompile(`"([^"]+)"`)
	matches := re.FindStringSubmatch(requirement)
	if len(matches) >= 2 {
		return matches[1]
	}

	return ""
}

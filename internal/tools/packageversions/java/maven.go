package java

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

// MavenTool handles Maven package version checking
type MavenTool struct {
	client packageversions.HTTPClient
}

// Definition returns the tool's definition for MCP registration
func (t *MavenTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"check_maven_versions",
		mcp.WithDescription("Check latest stable versions for Java packages in pom.xml"),
		mcp.WithArray("dependencies",
			mcp.Description("Array of Maven dependencies"),
			mcp.Required(),
			mcp.Items(map[string]any{"type": "object"}),
		),
	)
}

// Execute executes the tool's logic
func (t *MavenTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	logger.Info("Getting latest Maven package versions")

	// Parse dependencies
	depsRaw, ok := args["dependencies"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: dependencies")
	}

	// Convert to MavenDependency
	var dependencies []packageversions.MavenDependency
	for _, depRaw := range depsRaw {
		if depMap, ok := depRaw.(map[string]any); ok {
			var dep packageversions.MavenDependency

			// Parse groupId
			if groupId, ok := depMap["groupId"].(string); ok && groupId != "" {
				dep.GroupID = groupId
			} else {
				return nil, fmt.Errorf("missing required parameter: groupId")
			}

			// Parse artifactId
			if artifactId, ok := depMap["artifactId"].(string); ok && artifactId != "" {
				dep.ArtifactID = artifactId
			} else {
				return nil, fmt.Errorf("missing required parameter: artifactId")
			}

			// Parse version
			if version, ok := depMap["version"].(string); ok && version != "" {
				dep.Version = version
			}

			// Parse scope
			if scope, ok := depMap["scope"].(string); ok && scope != "" {
				dep.Scope = scope
			}

			dependencies = append(dependencies, dep)
		}
	}

	// Get latest versions
	results := t.getLatestVersions(logger, cache, dependencies)
	return packageversions.NewToolResultJSON(results)
}

// getLatestVersions gets the latest versions for Maven packages
func (t *MavenTool) getLatestVersions(logger *logrus.Logger, cache *sync.Map, dependencies []packageversions.MavenDependency) []packageversions.PackageVersion {
	var results []packageversions.PackageVersion

	for _, dep := range dependencies {
		// Skip dependencies without version
		if dep.Version == "" {
			continue
		}

		// Create package name
		packageName := fmt.Sprintf("%s:%s", dep.GroupID, dep.ArtifactID)

		// Check cache first
		cacheKey := fmt.Sprintf("maven:%s", packageName)
		if cachedVersion, ok := cache.Load(cacheKey); ok {
			logger.WithField("package", packageName).Debug("Using cached Maven package version")
			result := cachedVersion.(packageversions.PackageVersion)
			result.CurrentVersion = packageversions.StringPtrUnlessLatest(dep.Version)
			results = append(results, result)
			continue
		}

		// Get latest version
		latestVersion, err := t.getLatestVersion(logger, dep.GroupID, dep.ArtifactID)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"package": packageName,
				"error":   err.Error(),
			}).Error("Failed to get Maven package version")
			results = append(results, packageversions.PackageVersion{
				Name:           packageName,
				CurrentVersion: packageversions.StringPtrUnlessLatest(dep.Version),
				LatestVersion:  "unknown",
				Registry:       "maven",
				Skipped:        true,
				SkipReason:     fmt.Sprintf("Failed to fetch package info: %v", err),
			})
			continue
		}

		// Create result
		result := packageversions.PackageVersion{
			Name:           packageName,
			CurrentVersion: packageversions.StringPtrUnlessLatest(dep.Version),
			LatestVersion:  latestVersion,
			Registry:       "maven",
		}

		// Cache result
		cache.Store(cacheKey, result)

		results = append(results, result)
	}

	// Sort results by name
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return results
}

// getLatestVersion gets the latest version for a Maven package
func (t *MavenTool) getLatestVersion(logger *logrus.Logger, groupId, artifactId string) (string, error) {
	// Construct URL
	apiURL := fmt.Sprintf("https://search.maven.org/solrsearch/select?q=g:%s+AND+a:%s&rows=1&wt=json", groupId, artifactId)
	logger.WithFields(logrus.Fields{
		"groupId":    groupId,
		"artifactId": artifactId,
		"url":        apiURL,
	}).Debug("Fetching Maven package version")

	// Make request
	body, err := packageversions.MakeRequestWithLogger(t.client, logger, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Maven package version: %w", err)
	}

	// Parse response
	var response struct {
		Response struct {
			NumFound int `json:"numFound"`
			Docs     []struct {
				LatestVersion string `json:"latestVersion"`
			} `json:"docs"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse Maven package version: %w", err)
	}

	// Check if package exists
	if response.Response.NumFound == 0 || len(response.Response.Docs) == 0 {
		return "", fmt.Errorf("package not found")
	}

	return response.Response.Docs[0].LatestVersion, nil
}

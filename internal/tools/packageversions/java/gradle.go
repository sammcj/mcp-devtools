package java

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

// GradleTool handles Gradle package version checking
type GradleTool struct {
	client packageversions.HTTPClient
}

// init registers the Gradle tool with the registry
func init() {
	registry.Register(&GradleTool{
		client: packageversions.DefaultHTTPClient,
	})
}

// Definition returns the tool's definition for MCP registration
func (t *GradleTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"check_gradle_versions",
		mcp.WithDescription("Check latest stable versions for Java packages in build.gradle"),
		mcp.WithArray("dependencies",
			mcp.Description("Array of Gradle dependencies"),
			mcp.Required(),
		),
	)
}

// Execute executes the tool's logic
func (t *GradleTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Getting latest Gradle package versions")

	// Parse dependencies
	depsRaw, ok := args["dependencies"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing required parameter: dependencies")
	}

	// Convert to GradleDependency
	var dependencies []packageversions.GradleDependency
	for _, depRaw := range depsRaw {
		if depMap, ok := depRaw.(map[string]interface{}); ok {
			var dep packageversions.GradleDependency

			// Parse configuration
			if configuration, ok := depMap["configuration"].(string); ok && configuration != "" {
				dep.Configuration = configuration
			} else {
				return nil, fmt.Errorf("missing required parameter: configuration")
			}

			// Parse group
			if group, ok := depMap["group"].(string); ok && group != "" {
				dep.Group = group
			} else {
				return nil, fmt.Errorf("missing required parameter: group")
			}

			// Parse name
			if name, ok := depMap["name"].(string); ok && name != "" {
				dep.Name = name
			} else {
				return nil, fmt.Errorf("missing required parameter: name")
			}

			// Parse version
			if version, ok := depMap["version"].(string); ok && version != "" {
				dep.Version = version
			}

			dependencies = append(dependencies, dep)
		}
	}

	// Get latest versions
	results, err := t.getLatestVersions(logger, cache, dependencies)
	if err != nil {
		return nil, err
	}

	return packageversions.NewToolResultJSON(results)
}

// getLatestVersions gets the latest versions for Gradle packages
func (t *GradleTool) getLatestVersions(logger *logrus.Logger, cache *sync.Map, dependencies []packageversions.GradleDependency) ([]packageversions.PackageVersion, error) {
	var results []packageversions.PackageVersion

	// Create a Maven tool to reuse its functionality
	mavenTool := &MavenTool{
		client: t.client,
	}

	for _, dep := range dependencies {
		// Skip dependencies without version
		if dep.Version == "" {
			continue
		}

		// Create package name
		packageName := fmt.Sprintf("%s:%s", dep.Group, dep.Name)

		// Check cache first
		cacheKey := fmt.Sprintf("gradle:%s", packageName)
		if cachedVersion, ok := cache.Load(cacheKey); ok {
			logger.WithField("package", packageName).Debug("Using cached Gradle package version")
			result := cachedVersion.(packageversions.PackageVersion)
			result.CurrentVersion = packageversions.StringPtr(dep.Version)
			results = append(results, result)
			continue
		}

		// Get latest version
		latestVersion, err := mavenTool.getLatestVersion(logger, dep.Group, dep.Name)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"package": packageName,
				"error":   err.Error(),
			}).Error("Failed to get Gradle package version")
			results = append(results, packageversions.PackageVersion{
				Name:           packageName,
				CurrentVersion: packageversions.StringPtr(dep.Version),
				LatestVersion:  "unknown",
				Registry:       "gradle",
				Skipped:        true,
				SkipReason:     fmt.Sprintf("Failed to fetch package info: %v", err),
			})
			continue
		}

		// Create result
		result := packageversions.PackageVersion{
			Name:           packageName,
			CurrentVersion: packageversions.StringPtr(dep.Version),
			LatestVersion:  latestVersion,
			Registry:       "gradle",
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

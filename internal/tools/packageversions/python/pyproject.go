package python

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

// PyProjectTool handles Python package version checking for pyproject.toml files
type PyProjectTool struct {
	client packageversions.HTTPClient
}

// Definition returns the tool's definition for MCP registration
func (t *PyProjectTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"check_pyproject_versions",
		mcp.WithDescription("Check latest stable versions for Python packages in pyproject.toml"),
		mcp.WithObject("dependencies",
			mcp.Description("Dependencies object from pyproject.toml"),
			mcp.Properties(map[string]interface{}{}),
			mcp.Required(),
		),
	)
}

// Execute executes the tool's logic
func (t *PyProjectTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Getting latest Python package versions from pyproject.toml")

	// Parse dependencies
	depsRaw, ok := args["dependencies"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: dependencies")
	}

	// Convert to map[string]interface{}
	depsMap, ok := depsRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid dependencies format: expected object")
	}

	// Extract dependencies
	var packages []Package

	// Process main dependencies
	if mainDeps, ok := depsMap["dependencies"].(map[string]interface{}); ok {
		for name, version := range mainDeps {
			if vStr, ok := version.(string); ok {
				packages = append(packages, Package{
					Name:    name,
					Version: vStr,
				})
			}
		}
	}

	// Process optional dependencies
	if optDeps, ok := depsMap["optional-dependencies"].(map[string]interface{}); ok {
		for _, group := range optDeps {
			if groupDeps, ok := group.(map[string]interface{}); ok {
				for name, version := range groupDeps {
					if vStr, ok := version.(string); ok {
						packages = append(packages, Package{
							Name:    name,
							Version: vStr,
						})
					}
				}
			}
		}
	}

	// Process dev dependencies
	if devDeps, ok := depsMap["dev-dependencies"].(map[string]interface{}); ok {
		for name, version := range devDeps {
			if vStr, ok := version.(string); ok {
				packages = append(packages, Package{
					Name:    name,
					Version: vStr,
				})
			}
		}
	}

	// Get latest versions
	results, err := t.getLatestVersions(logger, cache, packages)
	if err != nil {
		return nil, err
	}

	return packageversions.NewToolResultJSON(results)
}

// getLatestVersions gets the latest versions for Python packages
func (t *PyProjectTool) getLatestVersions(logger *logrus.Logger, cache *sync.Map, packages []Package) ([]packageversions.PackageVersion, error) {
	var results []packageversions.PackageVersion

	// Create a Python tool to reuse its functionality
	pythonTool := &PythonTool{
		client: t.client,
	}

	for _, pkg := range packages {
		// Clean version string
		version := cleanPyProjectVersion(pkg.Version)

		// Check cache first
		cacheKey := fmt.Sprintf("python:%s", pkg.Name)
		if cachedVersion, ok := cache.Load(cacheKey); ok {
			logger.WithField("package", pkg.Name).Debug("Using cached Python package version")
			result := cachedVersion.(packageversions.PackageVersion)
			if version != "" {
				result.CurrentVersion = packageversions.StringPtr(version)
			}
			results = append(results, result)
			continue
		}

		// Get latest version
		latestVersion, err := pythonTool.getLatestVersion(logger, pkg.Name)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"package": pkg.Name,
				"error":   err.Error(),
			}).Error("Failed to get Python package version")
			result := packageversions.PackageVersion{
				Name:          pkg.Name,
				LatestVersion: "unknown",
				Registry:      "pypi",
				Skipped:       true,
				SkipReason:    fmt.Sprintf("Failed to fetch package info: %v", err),
			}
			if version != "" {
				result.CurrentVersion = packageversions.StringPtr(version)
			}
			results = append(results, result)
			continue
		}

		// Create result
		result := packageversions.PackageVersion{
			Name:          pkg.Name,
			LatestVersion: latestVersion,
			Registry:      "pypi",
		}
		if version != "" {
			result.CurrentVersion = packageversions.StringPtr(version)
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

// cleanPyProjectVersion cleans a version string from pyproject.toml
func cleanPyProjectVersion(version string) string {
	// Remove quotes
	version = strings.Trim(version, "\"'")

	// Remove version specifiers
	for _, prefix := range []string{">=", "<=", ">", "<", "==", "~=", "!="} {
		if strings.HasPrefix(version, prefix) {
			version = strings.TrimPrefix(version, prefix)
			break
		}
	}

	// Remove any extra constraints
	if idx := strings.IndexAny(version, ",;"); idx != -1 {
		version = version[:idx]
	}

	return strings.TrimSpace(version)
}

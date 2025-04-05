package go_tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

// GoTool handles Go package version checking
type GoTool struct {
	client packageversions.HTTPClient
}

// init registers the Go tool with the registry
func init() {
	registry.Register(&GoTool{
		client: packageversions.DefaultHTTPClient,
	})
}

// Definition returns the tool's definition for MCP registration
func (t *GoTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"check_go_versions",
		mcp.WithDescription("Check latest stable versions for Go packages in go.mod"),
		mcp.WithObject("dependencies",
			mcp.Description("Dependencies from go.mod"),
			mcp.Properties(map[string]interface{}{}),
			mcp.Required(),
		),
	)
}

// Execute executes the tool's logic
func (t *GoTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Getting latest Go package versions")

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

	// Extract require section
	var requires []packageversions.GoRequire
	if requireRaw, ok := depsMap["require"].([]interface{}); ok {
		for _, req := range requireRaw {
			if reqMap, ok := req.(map[string]interface{}); ok {
				var require packageversions.GoRequire

				// Parse path
				if path, ok := reqMap["path"].(string); ok && path != "" {
					require.Path = path
				} else {
					continue
				}

				// Parse version
				if version, ok := reqMap["version"].(string); ok && version != "" {
					require.Version = version
				} else {
					continue
				}

				requires = append(requires, require)
			}
		}
	}

	// Get latest versions
	results, err := t.getLatestVersions(logger, cache, requires)
	if err != nil {
		return nil, err
	}

	return packageversions.NewToolResultJSON(results)
}

// getLatestVersions gets the latest versions for Go packages
func (t *GoTool) getLatestVersions(logger *logrus.Logger, cache *sync.Map, requires []packageversions.GoRequire) ([]packageversions.PackageVersion, error) {
	var results []packageversions.PackageVersion

	for _, require := range requires {
		// Skip standard library packages
		if !strings.Contains(require.Path, ".") {
			continue
		}

		// Check cache first
		cacheKey := fmt.Sprintf("go:%s", require.Path)
		if cachedVersion, ok := cache.Load(cacheKey); ok {
			logger.WithField("package", require.Path).Debug("Using cached Go package version")
			result := cachedVersion.(packageversions.PackageVersion)
			result.CurrentVersion = packageversions.StringPtr(require.Version)
			results = append(results, result)
			continue
		}

		// Get latest version
		latestVersion, err := t.getLatestVersion(logger, require.Path)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"package": require.Path,
				"error":   err.Error(),
			}).Error("Failed to get Go package version")
			results = append(results, packageversions.PackageVersion{
				Name:           require.Path,
				CurrentVersion: packageversions.StringPtr(require.Version),
				LatestVersion:  "unknown",
				Registry:       "go",
				Skipped:        true,
				SkipReason:     fmt.Sprintf("Failed to fetch package info: %v", err),
			})
			continue
		}

		// Create result
		result := packageversions.PackageVersion{
			Name:           require.Path,
			CurrentVersion: packageversions.StringPtr(require.Version),
			LatestVersion:  latestVersion,
			Registry:       "go",
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

// getLatestVersion gets the latest version for a Go package
func (t *GoTool) getLatestVersion(logger *logrus.Logger, packagePath string) (string, error) {
	// Construct URL
	apiURL := fmt.Sprintf("https://proxy.golang.org/%s/@latest", packagePath)
	logger.WithFields(logrus.Fields{
		"package": packagePath,
		"url":     apiURL,
	}).Debug("Fetching Go package version")

	// Make request
	body, err := packageversions.MakeRequestWithLogger(t.client, logger, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Go package version: %w", err)
	}

	// Parse response
	var response struct {
		Version string `json:"Version"`
		Time    string `json:"Time"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse Go package version: %w", err)
	}

	return response.Version, nil
}

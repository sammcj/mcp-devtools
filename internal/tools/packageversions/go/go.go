package go_tool

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

// GoTool handles Go package version checking
type GoTool struct {
	client packageversions.HTTPClient
}

// NewGoTool creates a new go tool with the given HTTP client
func NewGoTool(client packageversions.HTTPClient) *GoTool {
	if client == nil {
		client = packageversions.DefaultHTTPClient
	}
	return &GoTool{
		client: client,
	}
}

// Definition returns the tool's definition for MCP registration
func (t *GoTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"check_go_versions",
		mcp.WithDescription("Check latest stable versions for Go packages in go.mod"),
		mcp.WithObject("dependencies",
			mcp.Description("Dependencies from go.mod"),
			mcp.Properties(map[string]any{}),
			mcp.Required(),
		),
	)
}

// Execute executes the tool's logic
func (t *GoTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	logger.Info("Getting latest Go package versions")

	// Parse dependencies
	depsRaw, ok := args["dependencies"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: dependencies")
	}

	// Convert to map[string]interface{}
	depsMap, ok := depsRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid dependencies format: expected object")
	}

	var requires []packageversions.GoRequire

	// Handle different input formats
	if requireRaw, ok := depsMap["require"].([]any); ok {
		// Complex format: structured go.mod with require array
		logger.Debug("Processing complex go.mod format with require array")
		for _, req := range requireRaw {
			if reqMap, ok := req.(map[string]any); ok {
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
	} else {
		// Simple format: key-value pairs are dependencies
		logger.Debug("Processing simple dependencies format")
		for path, versionRaw := range depsMap {
			logger.WithFields(logrus.Fields{
				"path":    path,
				"version": versionRaw,
			}).Debug("Processing dependency")

			if version, ok := versionRaw.(string); ok {
				requires = append(requires, packageversions.GoRequire{
					Path:    path,
					Version: version,
				})
			}
		}
	}

	// Get latest versions
	results := t.getLatestVersions(logger, cache, requires)

	return packageversions.NewToolResultJSON(results)
}

// getLatestVersions gets the latest versions for Go packages
func (t *GoTool) getLatestVersions(logger *logrus.Logger, cache *sync.Map, requires []packageversions.GoRequire) []packageversions.PackageVersion {
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
			result.CurrentVersion = packageversions.StringPtrUnlessLatest(require.Version)
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
				CurrentVersion: packageversions.StringPtrUnlessLatest(require.Version),
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
			CurrentVersion: packageversions.StringPtrUnlessLatest(require.Version),
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

	return results
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

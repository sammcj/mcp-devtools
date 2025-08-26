package python

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

// PythonTool handles Python package version checking
type PythonTool struct {
	client packageversions.HTTPClient
}

// NewPythonTool creates a new python tool with the given HTTP client
func NewPythonTool(client packageversions.HTTPClient) *PythonTool {
	if client == nil {
		client = packageversions.DefaultHTTPClient
	}
	return &PythonTool{
		client: client,
	}
}

// Definition returns the tool's definition for MCP registration
func (t *PythonTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"check_python_versions",
		mcp.WithDescription("Check latest stable versions for Python packages"),
		mcp.WithArray("requirements",
			mcp.Description("Array of requirements from requirements.txt"),
			mcp.Required(),
		),
	)
}

// Execute executes the tool's logic
func (t *PythonTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	logger.Info("Getting latest Python package versions")

	// Parse requirements
	requirementsRaw, ok := args["requirements"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: requirements")
	}

	// Convert to []string
	var requirements []string
	for _, req := range requirementsRaw {
		if reqStr, ok := req.(string); ok && reqStr != "" {
			requirements = append(requirements, reqStr)
		}
	}

	// Parse requirements
	packages, err := t.parseRequirements(requirements)
	if err != nil {
		return nil, err
	}

	// Get latest versions
	results, err := t.getLatestVersions(logger, cache, packages)
	if err != nil {
		return nil, err
	}

	return packageversions.NewToolResultJSON(results)
}

// Package represents a Python package
type Package struct {
	Name    string
	Version string
}

// parseRequirements parses requirements.txt lines into packages
func (t *PythonTool) parseRequirements(requirements []string) ([]Package, error) {
	var packages []Package

	for _, req := range requirements {
		// Skip empty lines and comments
		req = strings.TrimSpace(req)
		if req == "" || strings.HasPrefix(req, "#") {
			continue
		}

		// Skip options and editable installs
		if strings.HasPrefix(req, "-") || strings.HasPrefix(req, "--") {
			continue
		}

		// Parse package name and version
		// Format: package[extras]==version
		re := regexp.MustCompile(`^([a-zA-Z0-9_.-]+)(?:\[[^\]]*\])?(?:([<>=!~]+)([a-zA-Z0-9_.-]+))?`)
		matches := re.FindStringSubmatch(req)
		if len(matches) < 2 {
			continue
		}

		name := matches[1]
		version := ""
		if len(matches) > 3 {
			version = matches[3]
		}

		packages = append(packages, Package{
			Name:    name,
			Version: version,
		})
	}

	return packages, nil
}

// getLatestVersions gets the latest versions for Python packages
func (t *PythonTool) getLatestVersions(logger *logrus.Logger, cache *sync.Map, packages []Package) ([]packageversions.PackageVersion, error) {
	var results []packageversions.PackageVersion

	for _, pkg := range packages {
		// Check cache first
		cacheKey := fmt.Sprintf("python:%s", pkg.Name)
		if cachedVersion, ok := cache.Load(cacheKey); ok {
			logger.WithField("package", pkg.Name).Debug("Using cached Python package version")
			result := cachedVersion.(packageversions.PackageVersion)
			if pkg.Version != "" {
				result.CurrentVersion = packageversions.StringPtr(pkg.Version)
			}
			results = append(results, result)
			continue
		}

		// Get latest version
		latestVersion, err := t.getLatestVersion(logger, pkg.Name)
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
			if pkg.Version != "" {
				result.CurrentVersion = packageversions.StringPtr(pkg.Version)
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
		if pkg.Version != "" {
			result.CurrentVersion = packageversions.StringPtr(pkg.Version)
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

// getLatestVersion gets the latest version for a Python package
func (t *PythonTool) getLatestVersion(logger *logrus.Logger, packageName string) (string, error) {
	// Construct URL
	apiURL := fmt.Sprintf("https://pypi.org/pypi/%s/json", packageName)
	logger.WithFields(logrus.Fields{
		"package": packageName,
		"url":     apiURL,
	}).Debug("Fetching Python package version")

	// Make request
	body, err := packageversions.MakeRequestWithLogger(t.client, logger, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Python package version: %w", err)
	}

	// Parse response
	var response struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse Python package version: %w", err)
	}

	return response.Info.Version, nil
}

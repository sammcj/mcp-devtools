package npm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

const (
	// NpmRegistryURL is the base URL for the npm registry
	NpmRegistryURL = "https://registry.npmjs.org"
)

// NpmTool handles npm package version checking
type NpmTool struct {
	client packageversions.HTTPClient
}

// init registers the npm tool with the registry
func init() {
	registry.Register(&NpmTool{
		client: packageversions.DefaultHTTPClient,
	})
}

// Definition returns the tool's definition for MCP registration
func (t *NpmTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"check_npm_versions",
		mcp.WithDescription("Check latest stable versions for npm packages"),
		mcp.WithObject("dependencies",
			mcp.Description("Dependencies object from package.json"),
			mcp.Properties(map[string]interface{}{}),
			mcp.Required(),
		),
		mcp.WithObject("constraints",
			mcp.Description("Optional constraints for specific packages"),
			mcp.Properties(map[string]interface{}{}),
		),
	)
}

// Execute executes the tool's logic
func (t *NpmTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Getting latest npm package versions")

	// Parse dependencies
	depsRaw, ok := args["dependencies"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: dependencies")
	}

	// Convert to map[string]string
	depsMap := make(map[string]string)
	if deps, ok := depsRaw.(map[string]interface{}); ok {
		for name, version := range deps {
			if vStr, ok := version.(string); ok {
				depsMap[name] = vStr
			} else {
				depsMap[name] = fmt.Sprintf("%v", version)
			}
		}
	} else {
		return nil, fmt.Errorf("invalid dependencies format: expected object")
	}

	// Parse constraints
	var constraints packageversions.VersionConstraints
	if constraintsRaw, ok := args["constraints"]; ok {
		if constraintsMap, ok := constraintsRaw.(map[string]interface{}); ok {
			constraints = make(packageversions.VersionConstraints)
			for name, constraintRaw := range constraintsMap {
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
	}

	// Process each dependency
	results := make([]packageversions.PackageVersion, 0, len(depsMap))
	for name, version := range depsMap {
		logger.WithFields(logrus.Fields{
			"package": name,
			"version": version,
		}).Debug("Processing npm package")

		// Check if package should be excluded
		if constraint, ok := constraints[name]; ok && constraint.ExcludePackage {
			results = append(results, packageversions.PackageVersion{
				Name:       name,
				Skipped:    true,
				SkipReason: "Package excluded by constraints",
			})
			continue
		}

		// Clean version string
		currentVersion := packageversions.CleanVersion(version)

		// Get package info
		info, err := t.getPackageInfo(logger, cache, name)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"package": name,
				"error":   err.Error(),
			}).Error("Failed to get npm package info")
			results = append(results, packageversions.PackageVersion{
				Name:           name,
				CurrentVersion: packageversions.StringPtr(currentVersion),
				LatestVersion:  "unknown",
				Registry:       "npm",
				Skipped:        true,
				SkipReason:     fmt.Sprintf("Failed to fetch package info: %v", err),
			})
			continue
		}

		// Get latest version
		latestVersion := info.DistTags["latest"]
		if latestVersion == "" {
			// If no latest tag, use the highest version
			versions := make([]string, 0, len(info.Versions))
			for v := range info.Versions {
				versions = append(versions, v)
			}
			sort.Strings(versions)
			if len(versions) > 0 {
				latestVersion = versions[len(versions)-1]
			}
		}

		// Apply major version constraint if specified
		if constraint, ok := constraints[name]; ok && constraint.MajorVersion != nil {
			targetMajor := *constraint.MajorVersion
			latestMajor, _, _, err := packageversions.ParseVersion(latestVersion)
			if err == nil && latestMajor > targetMajor {
				// Find the latest version with the target major version
				versions := make([]string, 0, len(info.Versions))
				for v := range info.Versions {
					major, _, _, err := packageversions.ParseVersion(v)
					if err == nil && major == targetMajor {
						versions = append(versions, v)
					}
				}
				sort.Strings(versions)
				if len(versions) > 0 {
					latestVersion = versions[len(versions)-1]
				}
			}
		}

		// Add result
		results = append(results, packageversions.PackageVersion{
			Name:           name,
			CurrentVersion: packageversions.StringPtr(currentVersion),
			LatestVersion:  latestVersion,
			Registry:       "npm",
		})
	}

	// Sort results by name
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return packageversions.NewToolResultJSON(results)
}

// NpmPackageInfo represents information about an npm package
type NpmPackageInfo struct {
	Name     string            `json:"name"`
	DistTags map[string]string `json:"dist-tags"`
	Versions map[string]struct {
		Version string `json:"version"`
	} `json:"versions"`
}

// getPackageInfo gets information about an npm package
func (t *NpmTool) getPackageInfo(logger *logrus.Logger, cache *sync.Map, packageName string) (*NpmPackageInfo, error) {
	// Check cache first
	if cachedInfo, ok := cache.Load(fmt.Sprintf("npm:%s", packageName)); ok {
		logger.WithField("package", packageName).Debug("Using cached npm package info")
		return cachedInfo.(*NpmPackageInfo), nil
	}

	// Construct URL
	packageURL := fmt.Sprintf("%s/%s", NpmRegistryURL, url.PathEscape(packageName))
	logger.WithFields(logrus.Fields{
		"package": packageName,
		"url":     packageURL,
	}).Debug("Fetching npm package info")

	// Make request
	body, err := packageversions.MakeRequestWithLogger(t.client, logger, "GET", packageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch npm package info: %w", err)
	}

	// Parse response
	var info NpmPackageInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse npm package info: %w", err)
	}

	// Cache result
	cache.Store(fmt.Sprintf("npm:%s", packageName), &info)

	return &info, nil
}

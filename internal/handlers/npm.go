package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

const (
	// NpmRegistryURL is the base URL for the npm registry
	NpmRegistryURL = "https://registry.npmjs.org"
)

// NpmHandler handles npm package version checking
type NpmHandler struct {
	client HTTPClient
	cache  *sync.Map
	logger *logrus.Logger
}

// NewNpmHandler creates a new npm handler
func NewNpmHandler(logger *logrus.Logger, cache *sync.Map) *NpmHandler {
	if cache == nil {
		cache = &sync.Map{}
	}
	return &NpmHandler{
		client: DefaultHTTPClient,
		cache:  cache,
		logger: logger,
	}
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
func (h *NpmHandler) getPackageInfo(packageName string) (*NpmPackageInfo, error) {
	// Check cache first
	if cachedInfo, ok := h.cache.Load(fmt.Sprintf("npm:%s", packageName)); ok {
		h.logger.WithField("package", packageName).Debug("Using cached npm package info")
		return cachedInfo.(*NpmPackageInfo), nil
	}

	// Construct URL
	packageURL := fmt.Sprintf("%s/%s", NpmRegistryURL, url.PathEscape(packageName))
	h.logger.WithFields(logrus.Fields{
		"package": packageName,
		"url":     packageURL,
	}).Debug("Fetching npm package info")

	// Make request
	body, err := MakeRequestWithLogger(h.client, h.logger, "GET", packageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch npm package info: %w", err)
	}

	// Parse response
	var info NpmPackageInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse npm package info: %w", err)
	}

	// Cache result
	h.cache.Store(fmt.Sprintf("npm:%s", packageName), &info)

	return &info, nil
}

// GetLatestVersion gets the latest version of npm packages
func (h *NpmHandler) GetLatestVersion(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	h.logger.Info("Getting latest npm package versions")

	// Parse dependencies
	depsRaw, ok := args["dependencies"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: dependencies")
	}

	// Convert to map[string]string
	depsMap := make(map[string]string)
	if deps, ok := depsRaw.(map[string]any); ok {
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
	var constraints VersionConstraints
	if constraintsRaw, ok := args["constraints"]; ok {
		if constraintsMap, ok := constraintsRaw.(map[string]any); ok {
			constraints = make(VersionConstraints)
			for name, constraintRaw := range constraintsMap {
				if constraintMap, ok := constraintRaw.(map[string]any); ok {
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
	results := make([]PackageVersion, 0, len(depsMap))
	for name, version := range depsMap {
		h.logger.WithFields(logrus.Fields{
			"package": name,
			"version": version,
		}).Debug("Processing npm package")

		// Check if package should be excluded
		if constraint, ok := constraints[name]; ok && constraint.ExcludePackage {
			results = append(results, PackageVersion{
				Name:       name,
				Skipped:    true,
				SkipReason: "Package excluded by constraints",
			})
			continue
		}

		// Clean version string
		currentVersion := CleanVersion(version)

		// Get package info
		info, err := h.getPackageInfo(name)
		if err != nil {
			h.logger.WithFields(logrus.Fields{
				"package": name,
				"error":   err.Error(),
			}).Error("Failed to get npm package info")
			results = append(results, PackageVersion{
				Name:           name,
				CurrentVersion: StringPtr(currentVersion),
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
			latestMajor, _, _, err := ParseVersion(latestVersion)
			if err == nil && latestMajor > targetMajor {
				// Find the latest version with the target major version
				versions := make([]string, 0, len(info.Versions))
				for v := range info.Versions {
					major, _, _, err := ParseVersion(v)
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
		results = append(results, PackageVersion{
			Name:           name,
			CurrentVersion: StringPtr(currentVersion),
			LatestVersion:  latestVersion,
			Registry:       "npm",
		})
	}

	// Sort results by name
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return NewToolResultJSON(results)
}

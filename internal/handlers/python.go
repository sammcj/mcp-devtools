package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

const (
	// PyPIURL is the base URL for the PyPI API
	PyPIURL = "https://pypi.org/pypi"
)

// PythonHandler handles Python package version checking
type PythonHandler struct {
	client HTTPClient
	cache  *sync.Map
	logger *logrus.Logger
}

// NewPythonHandler creates a new Python handler
func NewPythonHandler(logger *logrus.Logger, cache *sync.Map) *PythonHandler {
	if cache == nil {
		cache = &sync.Map{}
	}
	return &PythonHandler{
		client: DefaultHTTPClient,
		cache:  cache,
		logger: logger,
	}
}

// PyPIPackageInfo represents information about a PyPI package
type PyPIPackageInfo struct {
	Info struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"info"`
	Releases map[string][]struct {
		PackageType string `json:"packagetype"`
	} `json:"releases"`
}

// getPackageInfo gets information about a PyPI package
func (h *PythonHandler) getPackageInfo(packageName string) (*PyPIPackageInfo, error) {
	// Check cache first
	if cachedInfo, ok := h.cache.Load(fmt.Sprintf("pypi:%s", packageName)); ok {
		h.logger.WithField("package", packageName).Debug("Using cached PyPI package info")
		return cachedInfo.(*PyPIPackageInfo), nil
	}

	// Construct URL
	packageURL := fmt.Sprintf("%s/%s/json", PyPIURL, packageName)
	h.logger.WithFields(logrus.Fields{
		"package": packageName,
		"url":     packageURL,
	}).Debug("Fetching PyPI package info")

	// Make request
	body, err := MakeRequestWithLogger(h.client, h.logger, "GET", packageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PyPI package info: %w", err)
	}

	// Parse response
	var info PyPIPackageInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse PyPI package info: %w", err)
	}

	// Cache result
	h.cache.Store(fmt.Sprintf("pypi:%s", packageName), &info)

	return &info, nil
}

// parseRequirement parses a Python requirement string
func parseRequirement(req string) (name string, version string, err error) {
	// Extract package name and version constraint
	re := regexp.MustCompile(`^([a-zA-Z0-9_.-]+)(?:\s*([<>=!~^].*)?)?$`)
	matches := re.FindStringSubmatch(req)
	if len(matches) < 2 {
		return "", "", fmt.Errorf("invalid requirement format: %s", req)
	}

	name = matches[1]
	if len(matches) > 2 && matches[2] != "" {
		version = strings.TrimSpace(matches[2])
	}

	return name, version, nil
}

// GetLatestVersionFromRequirements gets the latest version of Python packages from requirements.txt
func (h *PythonHandler) GetLatestVersionFromRequirements(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	h.logger.Info("Getting latest Python package versions from requirements.txt")

	// Parse requirements
	reqsRaw, ok := args["requirements"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: requirements")
	}

	// Convert to []string
	var reqs []string
	if reqsArr, ok := reqsRaw.([]interface{}); ok {
		for _, req := range reqsArr {
			if reqStr, ok := req.(string); ok {
				reqs = append(reqs, reqStr)
			} else {
				reqs = append(reqs, fmt.Sprintf("%v", req))
			}
		}
	} else {
		return nil, fmt.Errorf("invalid requirements format: expected array")
	}

	// Process each requirement
	results := make([]PackageVersion, 0, len(reqs))
	for _, req := range reqs {
		// Skip comments and empty lines
		req = strings.TrimSpace(req)
		if req == "" || strings.HasPrefix(req, "#") {
			continue
		}

		// Parse requirement
		name, version, err := parseRequirement(req)
		if err != nil {
			h.logger.WithFields(logrus.Fields{
				"requirement": req,
				"error":       err.Error(),
			}).Error("Failed to parse Python requirement")
			results = append(results, PackageVersion{
				Name:       req,
				Skipped:    true,
				SkipReason: fmt.Sprintf("Failed to parse requirement: %v", err),
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
			}).Error("Failed to get PyPI package info")
			results = append(results, PackageVersion{
				Name:           name,
				CurrentVersion: StringPtr(currentVersion),
				LatestVersion:  "unknown",
				Registry:       "pypi",
				Skipped:        true,
				SkipReason:     fmt.Sprintf("Failed to fetch package info: %v", err),
			})
			continue
		}

		// Get latest version
		latestVersion := info.Info.Version

		// Add result
		results = append(results, PackageVersion{
			Name:           name,
			CurrentVersion: StringPtr(currentVersion),
			LatestVersion:  latestVersion,
			Registry:       "pypi",
		})
	}

	// Sort results by name
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return NewToolResultJSON(results)
}

// GetLatestVersionFromPyProject gets the latest version of Python packages from pyproject.toml
func (h *PythonHandler) GetLatestVersionFromPyProject(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	h.logger.Info("Getting latest Python package versions from pyproject.toml")

	// Parse dependencies
	depsRaw, ok := args["dependencies"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: dependencies")
	}

	// Convert to PyProjectDependencies
	var pyProjectDeps PyProjectDependencies
	if depsMap, ok := depsRaw.(map[string]interface{}); ok {
		// Parse main dependencies
		if mainDeps, ok := depsMap["dependencies"].(map[string]interface{}); ok {
			pyProjectDeps.Dependencies = make(map[string]string)
			for name, version := range mainDeps {
				if vStr, ok := version.(string); ok {
					pyProjectDeps.Dependencies[name] = vStr
				} else {
					pyProjectDeps.Dependencies[name] = fmt.Sprintf("%v", version)
				}
			}
		}

		// Parse optional dependencies
		if optDeps, ok := depsMap["optional-dependencies"].(map[string]interface{}); ok {
			pyProjectDeps.OptionalDependencies = make(map[string]map[string]string)
			for group, deps := range optDeps {
				if depsMap, ok := deps.(map[string]interface{}); ok {
					pyProjectDeps.OptionalDependencies[group] = make(map[string]string)
					for name, version := range depsMap {
						if vStr, ok := version.(string); ok {
							pyProjectDeps.OptionalDependencies[group][name] = vStr
						} else {
							pyProjectDeps.OptionalDependencies[group][name] = fmt.Sprintf("%v", version)
						}
					}
				}
			}
		}

		// Parse dev dependencies
		if devDeps, ok := depsMap["dev-dependencies"].(map[string]interface{}); ok {
			pyProjectDeps.DevDependencies = make(map[string]string)
			for name, version := range devDeps {
				if vStr, ok := version.(string); ok {
					pyProjectDeps.DevDependencies[name] = vStr
				} else {
					pyProjectDeps.DevDependencies[name] = fmt.Sprintf("%v", version)
				}
			}
		}
	} else {
		return nil, fmt.Errorf("invalid dependencies format: expected object")
	}

	// Process all dependencies
	results := make([]PackageVersion, 0)

	// Process main dependencies
	for name, version := range pyProjectDeps.Dependencies {
		result, err := h.processPackage(name, version)
		if err != nil {
			h.logger.WithFields(logrus.Fields{
				"package": name,
				"error":   err.Error(),
			}).Error("Failed to process Python package")
		} else {
			results = append(results, result)
		}
	}

	// Process optional dependencies
	for group, deps := range pyProjectDeps.OptionalDependencies {
		for name, version := range deps {
			result, err := h.processPackage(name, version)
			if err != nil {
				h.logger.WithFields(logrus.Fields{
					"package": name,
					"group":   group,
					"error":   err.Error(),
				}).Error("Failed to process Python package")
			} else {
				// Add group info to result
				result.Name = fmt.Sprintf("%s (optional:%s)", name, group)
				results = append(results, result)
			}
		}
	}

	// Process dev dependencies
	for name, version := range pyProjectDeps.DevDependencies {
		result, err := h.processPackage(name, version)
		if err != nil {
			h.logger.WithFields(logrus.Fields{
				"package": name,
				"error":   err.Error(),
			}).Error("Failed to process Python package")
		} else {
			// Add dev info to result
			result.Name = fmt.Sprintf("%s (dev)", name)
			results = append(results, result)
		}
	}

	// Sort results by name
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return NewToolResultJSON(results)
}

// processPackage processes a single Python package
func (h *PythonHandler) processPackage(name, version string) (PackageVersion, error) {
	// Clean version string
	currentVersion := CleanVersion(version)

	// Get package info
	info, err := h.getPackageInfo(name)
	if err != nil {
		return PackageVersion{
			Name:           name,
			CurrentVersion: StringPtr(currentVersion),
			LatestVersion:  "unknown",
			Registry:       "pypi",
			Skipped:        true,
			SkipReason:     fmt.Sprintf("Failed to fetch package info: %v", err),
		}, err
	}

	// Get latest version
	latestVersion := info.Info.Version

	return PackageVersion{
		Name:           name,
		CurrentVersion: StringPtr(currentVersion),
		LatestVersion:  latestVersion,
		Registry:       "pypi",
	}, nil
}

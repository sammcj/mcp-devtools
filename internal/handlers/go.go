package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

const (
	// GoProxyURL is the base URL for the Go proxy API
	GoProxyURL = "https://proxy.golang.org"
)

// GoHandler handles Go package version checking
type GoHandler struct {
	client HTTPClient
	cache  *sync.Map
	logger *logrus.Logger
}

// NewGoHandler creates a new Go handler
func NewGoHandler(logger *logrus.Logger, cache *sync.Map) *GoHandler {
	if cache == nil {
		cache = &sync.Map{}
	}
	return &GoHandler{
		client: DefaultHTTPClient,
		cache:  cache,
		logger: logger,
	}
}

// GoModuleInfo represents information about a Go module
type GoModuleInfo struct {
	Version  string   `json:"Version"`
	Time     string   `json:"Time"`
	Versions []string `json:"Versions"`
}

// getLatestVersion gets the latest version of a Go module
func (h *GoHandler) getLatestVersion(modulePath string) (string, error) {
	// Check cache first
	if cachedVersion, ok := h.cache.Load(fmt.Sprintf("go:%s", modulePath)); ok {
		h.logger.WithField("module", modulePath).Debug("Using cached Go module version")
		return cachedVersion.(string), nil
	}

	// Construct URL
	moduleURL := fmt.Sprintf("%s/%s/@latest", GoProxyURL, modulePath)
	h.logger.WithFields(logrus.Fields{
		"module": modulePath,
		"url":    moduleURL,
	}).Debug("Fetching Go module info")

	// Make request
	body, err := MakeRequestWithLogger(h.client, h.logger, "GET", moduleURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Go module info: %w", err)
	}

	// Parse response
	var info GoModuleInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return "", fmt.Errorf("failed to parse Go module info: %w", err)
	}

	// Cache result
	h.cache.Store(fmt.Sprintf("go:%s", modulePath), info.Version)

	return info.Version, nil
}

// GetLatestVersion gets the latest version of Go packages
func (h *GoHandler) GetLatestVersion(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	h.logger.Info("Getting latest Go package versions")

	// Parse dependencies
	depsRaw, ok := args["dependencies"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: dependencies")
	}

	// Always set a default module name
	goModule := GoModule{
		Module: "github.com/sammcj/mcp-package-version-go",
	}

	// Log the raw dependencies for debugging
	h.logger.WithField("dependencies", fmt.Sprintf("%+v", depsRaw)).Debug("Raw dependencies")

	// Handle different input formats
	if depsMap, ok := depsRaw.(map[string]any); ok {
		// Check if this is the complex format with a module field
		if moduleName, ok := depsMap["module"].(string); ok {
			goModule.Module = moduleName

			// Parse require
			if requireRaw, ok := depsMap["require"].([]any); ok {
				for _, reqRaw := range requireRaw {
					if reqMap, ok := reqRaw.(map[string]any); ok {
						var req GoRequire
						if path, ok := reqMap["path"].(string); ok {
							req.Path = path
						} else {
							continue
						}
						if version, ok := reqMap["version"].(string); ok {
							req.Version = version
						}
						goModule.Require = append(goModule.Require, req)
					}
				}
			}

			// Parse replace
			if replaceRaw, ok := depsMap["replace"].([]any); ok {
				for _, repRaw := range replaceRaw {
					if repMap, ok := repRaw.(map[string]any); ok {
						var rep GoReplace
						if old, ok := repMap["old"].(string); ok {
							rep.Old = old
						} else {
							continue
						}
						if new, ok := repMap["new"].(string); ok {
							rep.New = new
						} else {
							continue
						}
						if version, ok := repMap["version"].(string); ok {
							rep.Version = version
						}
						goModule.Replace = append(goModule.Replace, rep)
					}
				}
			}
		} else {
			// Simple format: key-value pairs are dependencies
			for path, versionRaw := range depsMap {
				h.logger.WithFields(logrus.Fields{
					"path":    path,
					"version": versionRaw,
				}).Debug("Processing dependency")

				if version, ok := versionRaw.(string); ok {
					goModule.Require = append(goModule.Require, GoRequire{
						Path:    path,
						Version: version,
					})
				}
			}
		}
	} else {
		return nil, fmt.Errorf("invalid dependencies format: expected object, got %T", depsRaw)
	}

	// Log the parsed module for debugging
	h.logger.WithField("module", fmt.Sprintf("%+v", goModule)).Debug("Parsed module")

	// Process each require dependency
	results := make([]PackageVersion, 0, len(goModule.Require))
	for _, req := range goModule.Require {
		h.logger.WithFields(logrus.Fields{
			"module":  req.Path,
			"version": req.Version,
		}).Debug("Processing Go module")

		// Check if module is replaced
		var isReplaced bool
		var replacedBy string
		var replacedVersion string
		for _, rep := range goModule.Replace {
			if rep.Old == req.Path {
				isReplaced = true
				replacedBy = rep.New
				replacedVersion = rep.Version
				break
			}
		}

		// If module is replaced, use the replacement
		if isReplaced {
			results = append(results, PackageVersion{
				Name:           req.Path,
				CurrentVersion: StringPtr(req.Version),
				LatestVersion:  fmt.Sprintf("replaced by %s@%s", replacedBy, replacedVersion),
				Registry:       "go",
				Skipped:        true,
				SkipReason:     "Module is replaced",
			})
			continue
		}

		// Get latest version
		latestVersion, err := h.getLatestVersion(req.Path)
		if err != nil {
			h.logger.WithFields(logrus.Fields{
				"module": req.Path,
				"error":  err.Error(),
			}).Error("Failed to get Go module info")
			results = append(results, PackageVersion{
				Name:           req.Path,
				CurrentVersion: StringPtr(req.Version),
				LatestVersion:  "unknown",
				Registry:       "go",
				Skipped:        true,
				SkipReason:     fmt.Sprintf("Failed to fetch module info: %v", err),
			})
			continue
		}

		// Add result
		results = append(results, PackageVersion{
			Name:           req.Path,
			CurrentVersion: StringPtr(req.Version),
			LatestVersion:  latestVersion,
			Registry:       "go",
		})
	}

	// Sort results by name
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return NewToolResultJSON(results)
}

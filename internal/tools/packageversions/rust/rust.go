package rust

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

const CratesIOAPIURL = "https://crates.io/api/v1"

// RustTool handles Rust crate version checking
type RustTool struct {
	client packageversions.HTTPClient
}

// NewRustTool creates a new rust tool with the given HTTP client
func NewRustTool(client packageversions.HTTPClient) *RustTool {
	if client == nil {
		client = packageversions.DefaultHTTPClient
	}
	return &RustTool{
		client: client,
	}
}

// Definition returns the tool's definition for MCP registration
func (t *RustTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"check_rust_versions",
		mcp.WithDescription("Check latest stable versions for Rust crates"),
		mcp.WithObject("dependencies",
			mcp.Description("Dependencies from Cargo.toml"),
			mcp.Properties(map[string]interface{}{}),
			mcp.Required(),
		),
	)
}

// Execute executes the tool's logic
func (t *RustTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Getting latest Rust crate versions")

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
			} else if vMap, ok := version.(map[string]interface{}); ok {
				// Handle complex dependency format: { version = "1.0", features = ["derive"] }
				if v, ok := vMap["version"].(string); ok {
					depsMap[name] = v
				} else {
					depsMap[name] = "latest"
				}
			} else {
				depsMap[name] = fmt.Sprintf("%v", version)
			}
		}
	} else {
		return nil, fmt.Errorf("invalid dependencies format: expected object")
	}

	// Check if detailed information is requested
	includeDetails, _ := args["includeDetails"].(bool)

	// Process each dependency
	results := make([]packageversions.PackageVersion, 0, len(depsMap))
	for name, version := range depsMap {
		// Skip empty crate names
		if strings.TrimSpace(name) == "" {
			logger.Warn("Skipping empty crate name")
			continue
		}

		// Validate crate name format
		if !isValidCrateName(name) {
			logger.WithField("crate", name).Warn("Skipping invalid crate name")
			results = append(results, packageversions.PackageVersion{
				Name:          name,
				LatestVersion: "unknown",
				Registry:      "crates.io",
				Skipped:       true,
				SkipReason:    "Invalid crate name format",
			})
			continue
		}

		logger.WithFields(logrus.Fields{
			"crate":   name,
			"version": version,
		}).Debug("Processing Rust crate")

		// Clean version string
		currentVersion := packageversions.CleanVersion(version)

		// Get crate info
		info, err := t.getCrateInfo(logger, cache, name)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"crate": name,
				"error": err.Error(),
			}).Error("Failed to get Rust crate info")
			results = append(results, packageversions.PackageVersion{
				Name:           name,
				CurrentVersion: packageversions.StringPtr(currentVersion),
				LatestVersion:  "unknown",
				Registry:       "crates.io",
				Skipped:        true,
				SkipReason:     fmt.Sprintf("Failed to fetch crate info: %v", err),
			})
			continue
		}

		// Create base result
		result := packageversions.PackageVersion{
			Name:           name,
			CurrentVersion: packageversions.StringPtr(currentVersion),
			LatestVersion:  info.Crate.MaxVersion,
			Registry:       "crates.io",
		}

		// Add detailed information if requested
		if includeDetails {
			details := &packageversions.PackageDetails{
				Description:   packageversions.StringPtr(info.Crate.Description),
				Homepage:      packageversions.StringPtr(info.Crate.Homepage),
				Repository:    packageversions.StringPtr(info.Crate.Repository),
				Documentation: packageversions.StringPtr(info.Crate.Documentation),
				Downloads:     packageversions.Int64Ptr(info.Crate.Downloads),
				CreatedAt:     packageversions.StringPtr(info.Crate.CreatedAt),
				UpdatedAt:     packageversions.StringPtr(info.Crate.UpdatedAt),
				NumVersions:   packageversions.IntPtr(info.Crate.NumVersions),
				Keywords:      info.Crate.Keywords,
			}

			// Add Rust-specific details
			rustDetails := &packageversions.RustDetails{
				Categories:      info.Crate.Categories,
				RecentDownloads: packageversions.Int64Ptr(info.Crate.RecentDownloads),
			}

			// Add latest version details if available
			if len(info.Versions) > 0 {
				latestVersion := info.Versions[0]
				details.License = packageversions.StringPtr(latestVersion.License)
				details.PublishedAt = packageversions.StringPtr(latestVersion.CreatedAt)
				rustDetails.RustVersion = packageversions.StringPtr(latestVersion.RustVersion)
				rustDetails.Edition = packageversions.StringPtr(latestVersion.Edition)
				rustDetails.CrateSize = packageversions.Int64Ptr(latestVersion.CrateSize)
				if latestVersion.PublishedBy.Login != "" {
					details.Publisher = packageversions.StringPtr(fmt.Sprintf("%s (%s)", latestVersion.PublishedBy.Name, latestVersion.PublishedBy.Login))
				}
			}

			details.Rust = rustDetails
			result.Details = details
		}

		results = append(results, result)
	}

	// Sort results by name
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return packageversions.NewToolResultJSON(results)
}

// Version represents a crate version from crates.io
type Version struct {
	Num         string `json:"num"`
	License     string `json:"license"`
	CreatedAt   string `json:"created_at"`
	Downloads   int64  `json:"downloads"`
	Yanked      bool   `json:"yanked"`
	RustVersion string `json:"rust_version"`
	Edition     string `json:"edition"`
	CrateSize   int64  `json:"crate_size"`
	PublishedBy struct {
		Login  string `json:"login"`
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
		URL    string `json:"url"`
	} `json:"published_by"`
}

// CrateInfo represents information about a Rust crate from crates.io
type CrateInfo struct {
	Crate struct {
		Name            string   `json:"name"`
		MaxVersion      string   `json:"max_version"`
		Description     string   `json:"description"`
		Homepage        string   `json:"homepage"`
		Repository      string   `json:"repository"`
		Documentation   string   `json:"documentation"`
		Downloads       int64    `json:"downloads"`
		RecentDownloads int64    `json:"recent_downloads"`
		CreatedAt       string   `json:"created_at"`
		UpdatedAt       string   `json:"updated_at"`
		NumVersions     int      `json:"num_versions"`
		Keywords        []string `json:"keywords"`
		Categories      []string `json:"categories"`
	} `json:"crate"`
	Versions []Version `json:"versions"`
}

// getCrateInfo gets information about a Rust crate from crates.io
func (t *RustTool) getCrateInfo(logger *logrus.Logger, cache *sync.Map, crateName string) (*CrateInfo, error) {
	// Check cache first
	if cachedInfo, ok := cache.Load(fmt.Sprintf("rust:%s", crateName)); ok {
		logger.WithField("crate", crateName).Debug("Using cached Rust crate info")
		return cachedInfo.(*CrateInfo), nil
	}

	// Construct URL with proper escaping to prevent URL injection
	crateURL := fmt.Sprintf("%s/crates/%s", CratesIOAPIURL, url.PathEscape(crateName))
	logger.WithFields(logrus.Fields{
		"crate": crateName,
		"url":   crateURL,
	}).Debug("Fetching Rust crate info")

	// Make request
	body, err := packageversions.MakeRequestWithLogger(t.client, logger, "GET", crateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Rust crate info: %w", err)
	}

	// Parse response
	var info CrateInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse Rust crate info: %w", err)
	}

	// Filter out yanked versions to ensure we don't suggest unavailable versions
	var nonYankedVersions []Version
	for _, version := range info.Versions {
		if !version.Yanked {
			nonYankedVersions = append(nonYankedVersions, version)
		}
	}
	info.Versions = nonYankedVersions

	// Cache result
	cache.Store(fmt.Sprintf("rust:%s", crateName), &info)

	return &info, nil
}

// isValidCrateName validates Rust crate names according to crates.io rules
func isValidCrateName(name string) bool {
	// Basic validation - crates.io names are ASCII, hyphens, underscores
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	return matched && len(name) > 0 && len(name) <= 64
}

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
	// MavenCentralURL is the base URL for the Maven Central API
	MavenCentralURL = "https://search.maven.org/solrsearch/select"
)

// JavaHandler handles Java package version checking
type JavaHandler struct {
	client HTTPClient
	cache  *sync.Map
	logger *logrus.Logger
}

// NewJavaHandler creates a new Java handler
func NewJavaHandler(logger *logrus.Logger, cache *sync.Map) *JavaHandler {
	if cache == nil {
		cache = &sync.Map{}
	}
	return &JavaHandler{
		client: DefaultHTTPClient,
		cache:  cache,
		logger: logger,
	}
}

// MavenSearchResponse represents a response from the Maven Central API
type MavenSearchResponse struct {
	Response struct {
		NumFound int `json:"numFound"`
		Docs     []struct {
			ID        string   `json:"id"`
			GroupID   string   `json:"g"`
			ArtifactID string   `json:"a"`
			Version   string   `json:"v"`
			Versions  []string `json:"versions,omitempty"`
		} `json:"docs"`
	} `json:"response"`
}

// getLatestVersion gets the latest version of a Maven artifact
func (h *JavaHandler) getLatestVersion(groupID, artifactID string) (string, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("maven:%s:%s", groupID, artifactID)
	if cachedVersion, ok := h.cache.Load(cacheKey); ok {
		h.logger.WithFields(logrus.Fields{
			"groupId":    groupID,
			"artifactId": artifactID,
		}).Debug("Using cached Maven artifact version")
		return cachedVersion.(string), nil
	}

	// Construct URL
	queryURL := fmt.Sprintf("%s?q=g:%s+AND+a:%s&core=gav&rows=1&wt=json", MavenCentralURL, groupID, artifactID)
	h.logger.WithFields(logrus.Fields{
		"groupId":    groupID,
		"artifactId": artifactID,
		"url":        queryURL,
	}).Debug("Fetching Maven artifact info")

	// Make request
	body, err := MakeRequestWithLogger(h.client, h.logger, "GET", queryURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Maven artifact info: %w", err)
	}

	// Parse response
	var response MavenSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse Maven artifact info: %w", err)
	}

	// Check if artifact was found
	if response.Response.NumFound == 0 || len(response.Response.Docs) == 0 {
		return "", fmt.Errorf("artifact not found: %s:%s", groupID, artifactID)
	}

	// Get latest version
	latestVersion := response.Response.Docs[0].Version

	// Cache result
	h.cache.Store(cacheKey, latestVersion)

	return latestVersion, nil
}

// GetLatestVersionFromMaven gets the latest version of Java packages from Maven
func (h *JavaHandler) GetLatestVersionFromMaven(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	h.logger.Info("Getting latest Maven package versions")

	// Parse dependencies
	depsRaw, ok := args["dependencies"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: dependencies")
	}

	// Convert to []MavenDependency
	var deps []MavenDependency
	if depsArr, ok := depsRaw.([]interface{}); ok {
		for _, depRaw := range depsArr {
			if depMap, ok := depRaw.(map[string]interface{}); ok {
				var dep MavenDependency
				if groupID, ok := depMap["groupId"].(string); ok {
					dep.GroupID = groupID
				} else {
					continue
				}
				if artifactID, ok := depMap["artifactId"].(string); ok {
					dep.ArtifactID = artifactID
				} else {
					continue
				}
				if version, ok := depMap["version"].(string); ok {
					dep.Version = version
				}
				if scope, ok := depMap["scope"].(string); ok {
					dep.Scope = scope
				}
				deps = append(deps, dep)
			}
		}
	} else {
		return nil, fmt.Errorf("invalid dependencies format: expected array")
	}

	// Process each dependency
	results := make([]PackageVersion, 0, len(deps))
	for _, dep := range deps {
		h.logger.WithFields(logrus.Fields{
			"groupId":    dep.GroupID,
			"artifactId": dep.ArtifactID,
			"version":    dep.Version,
		}).Debug("Processing Maven dependency")

		// Get latest version
		latestVersion, err := h.getLatestVersion(dep.GroupID, dep.ArtifactID)
		if err != nil {
			h.logger.WithFields(logrus.Fields{
				"groupId":    dep.GroupID,
				"artifactId": dep.ArtifactID,
				"error":      err.Error(),
			}).Error("Failed to get Maven artifact info")
			results = append(results, PackageVersion{
				Name:           fmt.Sprintf("%s:%s", dep.GroupID, dep.ArtifactID),
				CurrentVersion: StringPtr(dep.Version),
				LatestVersion:  "unknown",
				Registry:       "maven",
				Skipped:        true,
				SkipReason:     fmt.Sprintf("Failed to fetch artifact info: %v", err),
			})
			continue
		}

		// Add result
		name := fmt.Sprintf("%s:%s", dep.GroupID, dep.ArtifactID)
		if dep.Scope != "" {
			name = fmt.Sprintf("%s (%s)", name, dep.Scope)
		}
		results = append(results, PackageVersion{
			Name:           name,
			CurrentVersion: StringPtr(dep.Version),
			LatestVersion:  latestVersion,
			Registry:       "maven",
		})
	}

	// Sort results by name
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return NewToolResultJSON(results)
}

// GetLatestVersionFromGradle gets the latest version of Java packages from Gradle
func (h *JavaHandler) GetLatestVersionFromGradle(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
	h.logger.Info("Getting latest Gradle package versions")

	// Parse dependencies
	depsRaw, ok := args["dependencies"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: dependencies")
	}

	// Convert to []GradleDependency
	var deps []GradleDependency
	if depsArr, ok := depsRaw.([]interface{}); ok {
		for _, depRaw := range depsArr {
			if depMap, ok := depRaw.(map[string]interface{}); ok {
				var dep GradleDependency
				if config, ok := depMap["configuration"].(string); ok {
					dep.Configuration = config
				} else {
					continue
				}
				if group, ok := depMap["group"].(string); ok {
					dep.Group = group
				} else {
					continue
				}
				if name, ok := depMap["name"].(string); ok {
					dep.Name = name
				} else {
					continue
				}
				if version, ok := depMap["version"].(string); ok {
					dep.Version = version
				}
				deps = append(deps, dep)
			}
		}
	} else {
		return nil, fmt.Errorf("invalid dependencies format: expected array")
	}

	// Process each dependency
	results := make([]PackageVersion, 0, len(deps))
	for _, dep := range deps {
		h.logger.WithFields(logrus.Fields{
			"group":         dep.Group,
			"name":          dep.Name,
			"version":       dep.Version,
			"configuration": dep.Configuration,
		}).Debug("Processing Gradle dependency")

		// Get latest version
		latestVersion, err := h.getLatestVersion(dep.Group, dep.Name)
		if err != nil {
			h.logger.WithFields(logrus.Fields{
				"group": dep.Group,
				"name":  dep.Name,
				"error": err.Error(),
			}).Error("Failed to get Maven artifact info")
			results = append(results, PackageVersion{
				Name:           fmt.Sprintf("%s:%s", dep.Group, dep.Name),
				CurrentVersion: StringPtr(dep.Version),
				LatestVersion:  "unknown",
				Registry:       "gradle",
				Skipped:        true,
				SkipReason:     fmt.Sprintf("Failed to fetch artifact info: %v", err),
			})
			continue
		}

		// Add result
		name := fmt.Sprintf("%s:%s", dep.Group, dep.Name)
		if dep.Configuration != "" {
			name = fmt.Sprintf("%s (%s)", name, dep.Configuration)
		}
		results = append(results, PackageVersion{
			Name:           name,
			CurrentVersion: StringPtr(dep.Version),
			LatestVersion:  latestVersion,
			Registry:       "gradle",
		})
	}

	// Sort results by name
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return NewToolResultJSON(results)
}

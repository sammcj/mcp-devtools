package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

// DockerHandler handles Docker image version checking
type DockerHandler struct {
	client HTTPClient
	cache  *sync.Map
	logger *logrus.Logger
}

// NewDockerHandler creates a new Docker handler
func NewDockerHandler(logger *logrus.Logger, cache *sync.Map) *DockerHandler {
	if cache == nil {
		cache = &sync.Map{}
	}
	return &DockerHandler{
		client: DefaultHTTPClient,
		cache:  cache,
		logger: logger,
	}
}

// DockerHubTagsResponse represents a response from the Docker Hub API
type DockerHubTagsResponse struct {
	Count    int    `json:"count"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Results  []struct {
		Name        string    `json:"name"`
		FullSize    int64     `json:"full_size"`
		LastUpdated time.Time `json:"last_updated"`
		Images      []struct {
			Digest       string `json:"digest"`
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
			Size         int64  `json:"size"`
		} `json:"images"`
	} `json:"results"`
}

// GHCRTagsResponse represents a response from the GitHub Container Registry API
type GHCRTagsResponse struct {
	Tags []string `json:"tags"`
}

// GetLatestVersion gets information about Docker image tags
func (h *DockerHandler) GetLatestVersion(ctx context.Context, args map[string]any) (*mcp.CallToolResult, error) {
	h.logger.Info("Getting Docker image tag information")

	// Parse image
	image, ok := args["image"].(string)
	if !ok || image == "" {
		return nil, fmt.Errorf("missing required parameter: image")
	}

	// Parse registry
	registry := "dockerhub"
	if registryRaw, ok := args["registry"].(string); ok && registryRaw != "" {
		registry = registryRaw
	}

	// Parse custom registry
	customRegistry := ""
	if customRegistryRaw, ok := args["customRegistry"].(string); ok {
		customRegistry = customRegistryRaw
	}

	// Parse limit
	limit := 10
	if limitRaw, ok := args["limit"].(float64); ok {
		limit = int(limitRaw)
	}

	// Parse filter tags
	var filterTags []string
	if filterTagsRaw, ok := args["filterTags"].([]any); ok {
		for _, tagRaw := range filterTagsRaw {
			if tag, ok := tagRaw.(string); ok {
				filterTags = append(filterTags, tag)
			}
		}
	}

	// Parse include digest
	includeDigest := false
	if includeDigestRaw, ok := args["includeDigest"].(bool); ok {
		includeDigest = includeDigestRaw
	}

	// Get tags based on registry
	var tags []DockerImageVersion
	var err error
	switch registry {
	case "dockerhub":
		tags, err = h.getDockerHubTags(image, limit, filterTags, includeDigest)
	case "ghcr":
		tags, err = h.getGHCRTags(image, limit, filterTags, includeDigest)
	case "custom":
		if customRegistry == "" {
			return nil, fmt.Errorf("missing required parameter for custom registry: customRegistry")
		}
		tags, err = h.getCustomRegistryTags(image, customRegistry, limit, filterTags, includeDigest)
	default:
		return nil, fmt.Errorf("invalid registry: %s", registry)
	}

	if err != nil {
		return nil, err
	}

	return NewToolResultJSON(tags)
}

// getDockerHubTags gets tags from Docker Hub
func (h *DockerHandler) getDockerHubTags(image string, limit int, filterTags []string, includeDigest bool) ([]DockerImageVersion, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("dockerhub:%s", image)
	if cachedTags, ok := h.cache.Load(cacheKey); ok {
		h.logger.WithField("image", image).Debug("Using cached Docker Hub tags")
		return h.filterTags(cachedTags.([]DockerImageVersion), limit, filterTags), nil
	}

	// Parse image name
	var namespace, repo string
	parts := strings.Split(image, "/")
	if len(parts) == 1 {
		namespace = "library"
		repo = parts[0]
	} else {
		namespace = parts[0]
		repo = strings.Join(parts[1:], "/")
	}

	// Construct URL
	tagsURL := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags?page_size=100", namespace, repo)
	h.logger.WithFields(logrus.Fields{
		"image": image,
		"url":   tagsURL,
	}).Debug("Fetching Docker Hub tags")

	// Make request
	body, err := MakeRequestWithLogger(h.client, h.logger, "GET", tagsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Docker Hub tags: %w", err)
	}

	// Parse response
	var response DockerHubTagsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Docker Hub tags: %w", err)
	}

	// Convert to DockerImageVersion
	var tags []DockerImageVersion
	for _, result := range response.Results {
		tag := DockerImageVersion{
			Name:     image,
			Tag:      result.Name,
			Registry: "dockerhub",
		}

		// Add digest if requested
		if includeDigest && len(result.Images) > 0 {
			digest := result.Images[0].Digest
			tag.Digest = &digest
		}

		// Add created date
		created := result.LastUpdated.Format(time.RFC3339)
		tag.Created = &created

		// Add size
		if len(result.Images) > 0 {
			size := fmt.Sprintf("%d", result.Images[0].Size)
			tag.Size = &size
		}

		tags = append(tags, tag)
	}

	// Cache result
	h.cache.Store(cacheKey, tags)

	return h.filterTags(tags, limit, filterTags), nil
}

// getGHCRTags gets tags from GitHub Container Registry
func (h *DockerHandler) getGHCRTags(image string, limit int, filterTags []string, includeDigest bool) ([]DockerImageVersion, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("ghcr:%s", image)
	if cachedTags, ok := h.cache.Load(cacheKey); ok {
		h.logger.WithField("image", image).Debug("Using cached GHCR tags")
		return h.filterTags(cachedTags.([]DockerImageVersion), limit, filterTags), nil
	}

	// Parse image name
	if !strings.HasPrefix(image, "ghcr.io/") {
		image = "ghcr.io/" + image
	}

	// Extract owner and repo
	parts := strings.Split(strings.TrimPrefix(image, "ghcr.io/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GHCR image format: %s", image)
	}

	owner := parts[0]
	repo := parts[1]

	// Construct URL
	tagsURL := fmt.Sprintf("https://ghcr.io/v2/%s/%s/tags/list", owner, repo)
	h.logger.WithFields(logrus.Fields{
		"image": image,
		"url":   tagsURL,
	}).Debug("Fetching GHCR tags")

	// Make request
	headers := map[string]string{
		"Accept": "application/vnd.github.v3+json",
	}
	body, err := MakeRequestWithLogger(h.client, h.logger, "GET", tagsURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GHCR tags: %w", err)
	}

	// Parse response
	var response GHCRTagsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse GHCR tags: %w", err)
	}

	// Convert to DockerImageVersion
	var tags []DockerImageVersion
	for _, tag := range response.Tags {
		tags = append(tags, DockerImageVersion{
			Name:     image,
			Tag:      tag,
			Registry: "ghcr",
		})
	}

	// Cache result
	h.cache.Store(cacheKey, tags)

	return h.filterTags(tags, limit, filterTags), nil
}

// getCustomRegistryTags gets tags from a custom registry
func (h *DockerHandler) getCustomRegistryTags(image, registry string, limit int, filterTags []string, includeDigest bool) ([]DockerImageVersion, error) {
	// This is a placeholder for custom registry implementation
	// In a real implementation, this would fetch data from the specified registry
	return []DockerImageVersion{
		{
			Name:     image,
			Tag:      "latest",
			Registry: registry,
		},
	}, nil
}

// filterTags filters tags based on regex patterns and limit
func (h *DockerHandler) filterTags(tags []DockerImageVersion, limit int, filterTags []string) []DockerImageVersion {
	if len(filterTags) == 0 && limit >= len(tags) {
		return tags
	}

	var filteredTags []DockerImageVersion
	for _, tag := range tags {
		// Apply regex filters
		if len(filterTags) > 0 {
			var match bool
			for _, pattern := range filterTags {
				re, err := regexp.Compile(pattern)
				if err != nil {
					h.logger.WithFields(logrus.Fields{
						"pattern": pattern,
						"error":   err.Error(),
					}).Error("Invalid regex pattern")
					continue
				}
				if re.MatchString(tag.Tag) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		filteredTags = append(filteredTags, tag)
		if len(filteredTags) >= limit {
			break
		}
	}

	return filteredTags
}

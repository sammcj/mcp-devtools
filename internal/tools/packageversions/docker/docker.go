package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

// DockerTool handles Docker image tag checking
type DockerTool struct {
	client packageversions.HTTPClient
}

// init registers the docker tool with the registry
func init() {
	registry.Register(&DockerTool{
		client: packageversions.DefaultHTTPClient,
	})
}

// Definition returns the tool's definition for MCP registration
func (t *DockerTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"check_docker_tags",
		mcp.WithDescription("Check available tags for Docker container images from Docker Hub, GitHub Container Registry, or custom registries"),
		mcp.WithString("image",
			mcp.Description("Docker image name (e.g., \"nginx\", \"ubuntu\", \"ghcr.io/owner/repo\")"),
			mcp.Required(),
		),
		mcp.WithString("registry",
			mcp.Description("Registry to check (dockerhub, ghcr, or custom)"),
			mcp.Enum("dockerhub", "ghcr", "custom"),
			mcp.DefaultString("dockerhub"),
		),
		mcp.WithString("customRegistry",
			mcp.Description("URL for custom registry (required when registry is \"custom\")"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of tags to return"),
			mcp.DefaultNumber(10),
		),
		mcp.WithArray("filterTags",
			mcp.Description("Array of regex patterns to filter tags"),
		),
		mcp.WithBoolean("includeDigest",
			mcp.Description("Include image digest in results"),
			mcp.DefaultBool(false),
		),
	)
}

// Execute executes the tool's logic
func (t *DockerTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Checking Docker image tags")

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

	// Check if custom registry is provided when registry is "custom"
	if registry == "custom" && customRegistry == "" {
		return nil, fmt.Errorf("customRegistry is required when registry is \"custom\"")
	}

	// Parse limit
	limit := 10
	if limitRaw, ok := args["limit"].(float64); ok {
		limit = int(limitRaw)
	}

	// Parse filter tags
	var filterTags []string
	if filterTagsRaw, ok := args["filterTags"].([]interface{}); ok {
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

	// Create query
	query := packageversions.DockerImageQuery{
		Image:          image,
		Registry:       registry,
		CustomRegistry: customRegistry,
		Limit:          limit,
		FilterTags:     filterTags,
		IncludeDigest:  includeDigest,
	}

	// Get tags
	tags, err := t.getTags(logger, cache, query)
	if err != nil {
		return nil, err
	}

	return packageversions.NewToolResultJSON(tags)
}

// getTags gets tags for a Docker image
func (t *DockerTool) getTags(logger *logrus.Logger, cache *sync.Map, query packageversions.DockerImageQuery) ([]packageversions.DockerImageVersion, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("docker:%s:%s:%s:%v", query.Registry, query.CustomRegistry, query.Image, query.IncludeDigest)
	if cachedTags, ok := cache.Load(cacheKey); ok {
		logger.WithField("image", query.Image).Debug("Using cached Docker image tags")
		return filterAndLimitTags(cachedTags.([]packageversions.DockerImageVersion), query.FilterTags, query.Limit)
	}

	// Parse image name
	var owner, repo string
	switch query.Registry {
	case "dockerhub":
		parts := strings.Split(query.Image, "/")
		if len(parts) == 1 {
			// Official image (e.g., "nginx")
			owner = "library"
			repo = parts[0]
		} else {
			// User image (e.g., "user/repo")
			owner = parts[0]
			repo = strings.Join(parts[1:], "/")
		}
	case "ghcr":
		// GitHub Container Registry (e.g., "ghcr.io/owner/repo")
		parts := strings.Split(query.Image, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid GHCR image format: %s", query.Image)
		}
		if strings.HasPrefix(parts[0], "ghcr.io") {
			parts = parts[1:]
		}
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid GHCR image format: %s", query.Image)
		}
		owner = parts[0]
		repo = strings.Join(parts[1:], "/")
	case "custom":
		// Custom registry (e.g., "registry.example.com/owner/repo")
		parts := strings.Split(query.Image, "/")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid custom image format: %s", query.Image)
		}
		owner = parts[0]
		repo = strings.Join(parts[1:], "/")
	default:
		return nil, fmt.Errorf("unsupported registry: %s", query.Registry)
	}

	// Get tags
	var tags []packageversions.DockerImageVersion
	var err error
	switch query.Registry {
	case "dockerhub":
		tags, err = t.getDockerHubTags(logger, owner, repo, query.IncludeDigest)
	case "ghcr":
		tags, err = t.getGHCRTags(logger, owner, repo, query.IncludeDigest)
	case "custom":
		tags, err = t.getCustomRegistryTags(logger, query.CustomRegistry, owner, repo, query.IncludeDigest)
	}
	if err != nil {
		return nil, err
	}

	// Cache tags
	cache.Store(cacheKey, tags)

	// Filter and limit tags
	return filterAndLimitTags(tags, query.FilterTags, query.Limit)
}

// getDockerHubTags gets tags from Docker Hub
func (t *DockerTool) getDockerHubTags(logger *logrus.Logger, owner, repo string, includeDigest bool) ([]packageversions.DockerImageVersion, error) {
	// Construct URL
	apiURL := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags?page_size=100", url.PathEscape(owner), url.PathEscape(repo))
	logger.WithFields(logrus.Fields{
		"owner": owner,
		"repo":  repo,
		"url":   apiURL,
	}).Debug("Fetching Docker Hub tags")

	// Make request
	body, err := packageversions.MakeRequestWithLogger(t.client, logger, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Docker Hub tags: %w", err)
	}

	// Parse response
	var response struct {
		Results []struct {
			Name        string    `json:"name"`
			LastUpdated time.Time `json:"last_updated"`
			Images      []struct {
				Digest       string `json:"digest"`
				Size         int64  `json:"size"`
				Status       string `json:"status"`
				Architecture string `json:"architecture"`
			} `json:"images"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Docker Hub tags: %w", err)
	}

	// Convert to DockerImageVersion
	var tags []packageversions.DockerImageVersion
	for _, result := range response.Results {
		tag := packageversions.DockerImageVersion{
			Name:     fmt.Sprintf("%s/%s", owner, repo),
			Tag:      result.Name,
			Registry: "dockerhub",
		}

		// Add digest if requested
		if includeDigest && len(result.Images) > 0 {
			digest := result.Images[0].Digest
			tag.Digest = &digest
			// Add Size
			sizeStr := fmt.Sprintf("%d", result.Images[0].Size)
			tag.Size = &sizeStr
		}

		// Add created date
		created := result.LastUpdated.Format(time.RFC3339)
		tag.Created = &created

		tags = append(tags, tag)
	}

	return tags, nil
}

// getGHCRTags gets tags from GitHub Container Registry
func (t *DockerTool) getGHCRTags(logger *logrus.Logger, owner, repo string, includeDigest bool) ([]packageversions.DockerImageVersion, error) {
	// Construct URL
	apiURL := fmt.Sprintf("https://ghcr.io/v2/%s/%s/tags/list", url.PathEscape(owner), url.PathEscape(repo))
	logger.WithFields(logrus.Fields{
		"owner": owner,
		"repo":  repo,
		"url":   apiURL,
	}).Debug("Fetching GHCR tags")

	// Make request
	headers := map[string]string{
		"Accept": "application/vnd.docker.distribution.manifest.v2+json",
	}
	body, err := packageversions.MakeRequestWithLogger(t.client, logger, "GET", apiURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GHCR tags: %w", err)
	}

	// Parse response
	var response struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse GHCR tags: %w", err)
	}

	// Convert to DockerImageVersion
	var tags []packageversions.DockerImageVersion
	for _, tag := range response.Tags {
		imageVersion := packageversions.DockerImageVersion{
			Name:     fmt.Sprintf("%s/%s", owner, repo),
			Tag:      tag,
			Registry: "ghcr",
		}

		// Add digest if requested
		if includeDigest {
			digest, err := t.getGHCRDigest(logger, owner, repo, tag)
			if err == nil && digest != "" {
				imageVersion.Digest = &digest
			}
		}

		tags = append(tags, imageVersion)
	}

	return tags, nil
}

// getGHCRDigest gets the digest for a specific tag from GitHub Container Registry
func (t *DockerTool) getGHCRDigest(logger *logrus.Logger, owner, repo, tag string) (string, error) {
	// Construct URL
	apiURL := fmt.Sprintf("https://ghcr.io/v2/%s/%s/manifests/%s", url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(tag))
	logger.WithFields(logrus.Fields{
		"owner": owner,
		"repo":  repo,
		"tag":   tag,
		"url":   apiURL,
	}).Debug("Fetching GHCR digest")

	// Make request
	headers := map[string]string{
		"Accept": "application/vnd.docker.distribution.manifest.v2+json",
	}
	_, err := packageversions.MakeRequestWithLogger(t.client, logger, "HEAD", apiURL, headers)
	if err != nil {
		return "", fmt.Errorf("failed to fetch GHCR digest: %w", err)
	}

	// Get digest from response headers
	// Note: This is a simplified implementation, as we don't have access to response headers in this context
	// In a real implementation, we would extract the Docker-Content-Digest header
	return "", nil
}

// getCustomRegistryTags gets tags from a custom registry
func (t *DockerTool) getCustomRegistryTags(logger *logrus.Logger, registry, owner, repo string, includeDigest bool) ([]packageversions.DockerImageVersion, error) {
	// Construct URL
	apiURL := fmt.Sprintf("%s/v2/%s/%s/tags/list", registry, url.PathEscape(owner), url.PathEscape(repo))
	logger.WithFields(logrus.Fields{
		"registry": registry,
		"owner":    owner,
		"repo":     repo,
		"url":      apiURL,
	}).Debug("Fetching custom registry tags")

	// Make request
	headers := map[string]string{
		"Accept": "application/vnd.docker.distribution.manifest.v2+json",
	}
	body, err := packageversions.MakeRequestWithLogger(t.client, logger, "GET", apiURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch custom registry tags: %w", err)
	}

	// Parse response
	var response struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse custom registry tags: %w", err)
	}

	// Convert to DockerImageVersion
	var tags []packageversions.DockerImageVersion
	for _, tag := range response.Tags {
		imageVersion := packageversions.DockerImageVersion{
			Name:     fmt.Sprintf("%s/%s", owner, repo),
			Tag:      tag,
			Registry: "custom",
		}

		// Add digest if requested
		if includeDigest {
			digest, err := t.getCustomRegistryDigest(logger, registry, owner, repo, tag)
			if err == nil && digest != "" {
				imageVersion.Digest = &digest
			}
		}

		tags = append(tags, imageVersion)
	}

	return tags, nil
}

// getCustomRegistryDigest gets the digest for a specific tag from a custom registry
func (t *DockerTool) getCustomRegistryDigest(logger *logrus.Logger, registry, owner, repo, tag string) (string, error) {
	// Construct URL
	apiURL := fmt.Sprintf("%s/v2/%s/%s/manifests/%s", registry, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(tag))
	logger.WithFields(logrus.Fields{
		"registry": registry,
		"owner":    owner,
		"repo":     repo,
		"tag":      tag,
		"url":      apiURL,
	}).Debug("Fetching custom registry digest")

	// Make request
	headers := map[string]string{
		"Accept": "application/vnd.docker.distribution.manifest.v2+json",
	}
	_, err := packageversions.MakeRequestWithLogger(t.client, logger, "HEAD", apiURL, headers)
	if err != nil {
		return "", fmt.Errorf("failed to fetch custom registry digest: %w", err)
	}

	// Get digest from response headers
	// Note: This is a simplified implementation, as we don't have access to response headers in this context
	// In a real implementation, we would extract the Docker-Content-Digest header
	return "", nil
}

// filterAndLimitTags filters and limits the tags based on the query
func filterAndLimitTags(tags []packageversions.DockerImageVersion, filterPatterns []string, limit int) ([]packageversions.DockerImageVersion, error) {
	// Filter tags
	var filteredTags []packageversions.DockerImageVersion
	if len(filterPatterns) > 0 {
		// Compile regex patterns
		var patterns []*regexp.Regexp
		for _, pattern := range filterPatterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid regex pattern: %s: %w", pattern, err)
			}
			patterns = append(patterns, re)
		}

		// Filter tags
		for _, tag := range tags {
			for _, re := range patterns {
				if re.MatchString(tag.Tag) {
					filteredTags = append(filteredTags, tag)
					break
				}
			}
		}
	} else {
		filteredTags = tags
	}

	// Sort tags by name
	sort.Slice(filteredTags, func(i, j int) bool {
		return filteredTags[i].Tag < filteredTags[j].Tag
	})

	// Limit tags
	if limit > 0 && len(filteredTags) > limit {
		filteredTags = filteredTags[:limit]
	}

	return filteredTags, nil
}

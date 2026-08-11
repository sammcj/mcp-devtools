package go_tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/sammcj/mcp-devtools/internal/mcpapi"
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
func (t *GoTool) Definition() mcpapi.Tool {
	return mcpapi.NewTool(
		"check_go_versions",
		mcpapi.WithDescription("Check latest stable versions for Go packages in go.mod, including module deprecations and newer major versions published under a different import path"),
		mcpapi.WithObject("dependencies",
			mcpapi.Description("Dependencies from go.mod"),
			mcpapi.Properties(map[string]any{}),
			mcpapi.Required(),
		),
	)
}

// Execute executes the tool's logic
func (t *GoTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcpapi.CallToolResult, error) {
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
	results := t.getLatestVersions(ctx, logger, cache, requires)

	return packageversions.NewToolResultJSON(results)
}

// getLatestVersions gets the latest versions for Go packages
func (t *GoTool) getLatestVersions(ctx context.Context, logger *logrus.Logger, cache *sync.Map, requires []packageversions.GoRequire) []packageversions.PackageVersion {
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
		info, err := t.getModuleInfo(ctx, logger, require.Path)
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
			LatestVersion:  info.latestVersion,
			Registry:       "go",
			Deprecated:     info.deprecated,
			NewerMajor:     info.newerMajor,
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

// moduleInfo is the subset of module metadata this tool reports.
type moduleInfo struct {
	latestVersion string
	deprecated    string // upstream deprecation reason, empty if not deprecated
	newerMajor    string // e.g. "github.com/golang-jwt/jwt/v5 v5.3.1"
}

// getModuleInfo looks up module metadata, preferring the pkg.go.dev API because
// it reports deprecations and newer major versions. The module proxy is used as
// a fallback since the pkg.go.dev API is still in beta and rate limited.
func (t *GoTool) getModuleInfo(ctx context.Context, logger *logrus.Logger, packagePath string) (moduleInfo, error) {
	info, err := t.queryPkgGoDev(ctx, logger, packagePath)
	if err == nil {
		return info, nil
	}

	logger.WithFields(logrus.Fields{
		"package": packagePath,
		"error":   err.Error(),
	}).Debug("pkg.go.dev lookup failed, falling back to the module proxy")

	version, proxyErr := t.queryModuleProxy(ctx, logger, packagePath)
	if proxyErr != nil {
		return moduleInfo{}, fmt.Errorf("pkg.go.dev: %v; module proxy: %w", err, proxyErr)
	}
	return moduleInfo{latestVersion: version}, nil
}

// moduleVersion is one entry of a pkg.go.dev version listing. Each entry carries
// the latest version of its own module path, which differs from the entry's own
// version.
type moduleVersion struct {
	ModulePath        string `json:"modulePath"`
	Version           string `json:"version"`
	LatestVersion     string `json:"latestVersion"`
	Deprecated        bool   `json:"deprecated"`
	DeprecationReason string `json:"deprecationReason"`
}

// queryPkgGoDev fetches the newest published version of a module across all of
// its major versions in a single request, then resolves the same-major latest
// separately only when a newer major exists.
func (t *GoTool) queryPkgGoDev(ctx context.Context, logger *logrus.Logger, packagePath string) (moduleInfo, error) {
	items, err := t.listVersions(ctx, logger, packagePath, "1")
	if err != nil {
		return moduleInfo{}, err
	}
	if len(items) == 0 {
		return moduleInfo{}, fmt.Errorf("no versions returned for %s", packagePath)
	}

	item := items[0]
	// latestVersion excludes retracted releases, version is only the newest published one.
	latest := item.LatestVersion
	if latest == "" {
		latest = item.Version
	}

	if item.ModulePath == packagePath {
		info := moduleInfo{latestVersion: latest}
		if item.Deprecated {
			info.deprecated = item.DeprecationReason
		}
		return info, nil
	}

	// A newer major version lives at a different import path, so upgrading to it
	// needs a code change. Report it separately from the same-major latest.
	sameMajor, deprecated := t.resolveSameMajor(ctx, logger, packagePath)
	if sameMajor == "" {
		return moduleInfo{}, fmt.Errorf("could not resolve the latest %s release", packagePath)
	}
	return moduleInfo{
		latestVersion: sameMajor,
		deprecated:    deprecated,
		newerMajor:    fmt.Sprintf("%s %s", item.ModulePath, latest),
	}, nil
}

// resolveSameMajor returns the latest version of packagePath itself, plus its
// deprecation reason, for when a newer major exists at a different import path.
// Deprecation is only reported on version listings, so the listing is searched
// rather than asking for the module directly. An empty version means both
// lookups failed.
func (t *GoTool) resolveSameMajor(ctx context.Context, logger *logrus.Logger, packagePath string) (version, deprecated string) {
	// A listing covers every major version of a module, newest major first, so
	// this reads past the newer major's releases to reach the requested one.
	items, err := t.listVersions(ctx, logger, packagePath, "100")
	if err == nil {
		var latest string
		for _, item := range items {
			if item.ModulePath != packagePath {
				continue
			}
			if latest == "" {
				latest = item.LatestVersion
			}
			// Deprecation is declared per version, so only the latest release
			// of this major says whether it is deprecated today.
			if item.Version == latest {
				if item.Deprecated {
					return latest, item.DeprecationReason
				}
				return latest, ""
			}
		}
		if latest != "" {
			return latest, ""
		}
	}

	// Listings are paginated, so a module with a long release history in its
	// newest major may not reach the requested one on the first page.
	fallback, err := t.queryPkgGoDevModule(ctx, logger, packagePath)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"package": packagePath,
			"error":   err.Error(),
		}).Debug("Could not resolve the same-major latest version")
		return "", ""
	}
	return fallback, ""
}

// listVersions fetches a page of a module's version listing.
func (t *GoTool) listVersions(ctx context.Context, logger *logrus.Logger, packagePath, limit string) ([]moduleVersion, error) {
	body, err := t.fetchJSON(ctx, logger, pkgGoDevURL("versions", packagePath, url.Values{"limit": {limit}}))
	if err != nil {
		return nil, err
	}

	var response struct {
		Items []moduleVersion `json:"items"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse pkg.go.dev response: %w", err)
	}
	return response.Items, nil
}

// queryPkgGoDevModule returns the latest version of the given module path only,
// without considering other major versions.
func (t *GoTool) queryPkgGoDevModule(ctx context.Context, logger *logrus.Logger, packagePath string) (string, error) {
	body, err := t.fetchJSON(ctx, logger, pkgGoDevURL("module", packagePath, nil))
	if err != nil {
		return "", err
	}

	var response struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse pkg.go.dev response: %w", err)
	}
	if response.Version == "" {
		return "", fmt.Errorf("pkg.go.dev returned no version for %s", packagePath)
	}
	return response.Version, nil
}

// queryModuleProxy asks the Go module proxy for the latest version of a module.
func (t *GoTool) queryModuleProxy(ctx context.Context, logger *logrus.Logger, packagePath string) (string, error) {
	proxyURL := &url.URL{
		Scheme: "https",
		Host:   "proxy.golang.org",
		Path:   "/" + escapeModuleProxyPath(packagePath) + "/@latest",
	}

	body, err := t.fetchJSON(ctx, logger, proxyURL.String())
	if err != nil {
		return "", err
	}

	var response struct {
		Version string `json:"Version"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse module proxy response: %w", err)
	}
	if response.Version == "" {
		return "", fmt.Errorf("module proxy returned no version for %s", packagePath)
	}
	return response.Version, nil
}

func (t *GoTool) fetchJSON(ctx context.Context, logger *logrus.Logger, reqURL string) ([]byte, error) {
	logger.WithField("url", reqURL).Debug("Fetching Go module metadata")
	return packageversions.MakeRequestWithContext(ctx, t.client, logger, "GET", reqURL, nil)
}

// pkgGoDevURL builds a pkg.go.dev API URL. Building it through url.URL escapes
// the module path so a crafted go.mod entry cannot inject query parameters.
func pkgGoDevURL(endpoint, modulePath string, query url.Values) string {
	u := &url.URL{
		Scheme: "https",
		Host:   "pkg.go.dev",
		Path:   fmt.Sprintf("/v1beta/%s/%s", endpoint, modulePath),
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	return u.String()
}

// escapeModuleProxyPath applies the module proxy's case encoding, where each
// uppercase letter becomes '!' followed by its lowercase form. Without this the
// proxy returns 404 for modules such as github.com/Masterminds/semver.
func escapeModuleProxyPath(modulePath string) string {
	var b strings.Builder
	for _, r := range modulePath {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('!')
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

package packageversions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// DefaultCooldownHours is the default cooldown period (72 hours / 3 days)
	DefaultCooldownHours = 72

	// OSVAPITimeout is the timeout for OSV API requests
	OSVAPITimeout = 1500 * time.Millisecond

	// OSVURL is the OSV API endpoint
	OSVURL = "https://api.osv.dev/v1/query"

	// OSVCacheDuration is how long to cache OSV results
	OSVCacheDuration = 5 * time.Minute
)

// CooldownConfig holds the cooldown configuration
type CooldownConfig struct {
	Hours      int
	Ecosystems map[string]bool
}

// CooldownInfo contains information about cooldown applied to a version
type CooldownInfo struct {
	Applied        bool    `json:"applied"`
	Reason         string  `json:"reason,omitempty"`
	NewerVersion   *string `json:"newerVersion,omitempty"`
	PublishedAt    *string `json:"publishedAt,omitempty"`
	CooldownEndsAt *string `json:"cooldownEndsAt,omitempty"`
}

// VersionWithDate pairs a version string with its publish date
type VersionWithDate struct {
	Version     string
	PublishedAt time.Time
}

// osvCacheEntry holds a cached OSV result
type osvCacheEntry struct {
	hasVulns  bool
	expiresAt time.Time
}

var (
	cooldownConfig     *CooldownConfig
	cooldownConfigOnce sync.Once
	osvCache           sync.Map
)

// GetCooldownConfig returns the cooldown configuration from environment variables
func GetCooldownConfig() *CooldownConfig {
	cooldownConfigOnce.Do(func() {
		cooldownConfig = loadCooldownConfig()
	})
	return cooldownConfig
}

// loadCooldownConfig loads cooldown configuration from environment
func loadCooldownConfig() *CooldownConfig {
	config := &CooldownConfig{
		Hours:      DefaultCooldownHours,
		Ecosystems: make(map[string]bool),
	}

	// Parse PACKAGE_COOLDOWN_HOURS
	if hoursStr := os.Getenv("PACKAGE_COOLDOWN_HOURS"); hoursStr != "" {
		if hours, err := strconv.Atoi(hoursStr); err == nil && hours >= 0 {
			config.Hours = hours
		}
	}

	// Parse PACKAGE_COOLDOWN_ECOSYSTEMS (default: "npm")
	ecosystemsStr := os.Getenv("PACKAGE_COOLDOWN_ECOSYSTEMS")
	if ecosystemsStr == "" {
		ecosystemsStr = "npm" // Default to npm only
	}

	// Handle "none" or empty to disable all
	if ecosystemsStr != "none" {
		for ecosystem := range strings.SplitSeq(ecosystemsStr, ",") {
			ecosystem = strings.TrimSpace(strings.ToLower(ecosystem))
			if ecosystem != "" {
				config.Ecosystems[ecosystem] = true
			}
		}
	}

	return config
}

// IsEcosystemCooldownEnabled checks if cooldown is enabled for a specific ecosystem
func (c *CooldownConfig) IsEcosystemCooldownEnabled(ecosystem string) bool {
	if c.Hours == 0 {
		return false
	}
	return c.Ecosystems[strings.ToLower(ecosystem)]
}

// GetCooldownDuration returns the cooldown duration
func (c *CooldownConfig) GetCooldownDuration() time.Duration {
	return time.Duration(c.Hours) * time.Hour
}

// ApplyCooldown applies cooldown logic to a list of versions and returns the appropriate version
// Returns: selected version, cooldown info, error
func ApplyCooldown(
	logger *logrus.Logger,
	client HTTPClient,
	ecosystem string,
	packageName string,
	versions []VersionWithDate,
	latestVersion string,
) (string, *CooldownInfo, error) {
	config := GetCooldownConfig()

	// If cooldown not enabled for this ecosystem, return latest
	if !config.IsEcosystemCooldownEnabled(ecosystem) {
		return latestVersion, nil, nil
	}

	// If no versions with dates, can't apply cooldown
	if len(versions) == 0 {
		return latestVersion, nil, nil
	}

	cooldownThreshold := time.Now().Add(-config.GetCooldownDuration())

	// Find the latest version and check if it's within cooldown
	var latestVersionDate *VersionWithDate
	for i := range versions {
		if versions[i].Version == latestVersion {
			latestVersionDate = &versions[i]
			break
		}
	}

	// If we can't find the latest version's date, return it without cooldown
	if latestVersionDate == nil {
		logger.WithFields(logrus.Fields{
			"package":   packageName,
			"ecosystem": ecosystem,
		}).Debug("Could not find publish date for latest version, skipping cooldown")
		return latestVersion, nil, nil
	}

	// If latest version is outside cooldown window, return it
	if latestVersionDate.PublishedAt.Before(cooldownThreshold) {
		return latestVersion, nil, nil
	}

	// Latest is within cooldown - find the most recent version outside cooldown
	var cooldownVersion *VersionWithDate
	for i := range versions {
		v := &versions[i]
		if v.PublishedAt.Before(cooldownThreshold) {
			if cooldownVersion == nil || v.PublishedAt.After(cooldownVersion.PublishedAt) {
				cooldownVersion = v
			}
		}
	}

	// If no version outside cooldown, we have to return latest
	if cooldownVersion == nil {
		logger.WithFields(logrus.Fields{
			"package":   packageName,
			"ecosystem": ecosystem,
		}).Debug("All versions within cooldown window, returning latest")
		return latestVersion, &CooldownInfo{
			Applied: false,
			Reason:  "All versions within cooldown window",
		}, nil
	}

	// Check if the cooldown version has known vulnerabilities
	hasVulns, err := checkOSVVulnerabilities(logger, client, ecosystem, packageName, cooldownVersion.Version)
	if err != nil {
		// Log error but continue with cooldown version (fail safe)
		logger.WithFields(logrus.Fields{
			"package":   packageName,
			"ecosystem": ecosystem,
			"version":   cooldownVersion.Version,
			"error":     err.Error(),
		}).Debug("OSV check failed, using cooldown version")
	}

	if hasVulns {
		// Cooldown version has vulnerabilities, recommend latest instead
		logger.WithFields(logrus.Fields{
			"package":         packageName,
			"ecosystem":       ecosystem,
			"cooldownVersion": cooldownVersion.Version,
			"latestVersion":   latestVersion,
		}).Info("Cooldown version has known vulnerabilities, recommending latest")

		return latestVersion, &CooldownInfo{
			Applied: false,
			Reason:  fmt.Sprintf("Cooldown version %s has known vulnerabilities", cooldownVersion.Version),
		}, nil
	}

	// Return the cooldown version with metadata
	publishedAtStr := latestVersionDate.PublishedAt.Format(time.RFC3339)
	cooldownEndsAt := latestVersionDate.PublishedAt.Add(config.GetCooldownDuration()).Format(time.RFC3339)

	return cooldownVersion.Version, &CooldownInfo{
		Applied:        true,
		Reason:         fmt.Sprintf("Version %s published %s ago, cooldown requires %d hours", latestVersion, formatDuration(time.Since(latestVersionDate.PublishedAt)), config.Hours),
		NewerVersion:   &latestVersion,
		PublishedAt:    &publishedAtStr,
		CooldownEndsAt: &cooldownEndsAt,
	}, nil
}

// osvRequest represents a request to the OSV API
type osvRequest struct {
	Package osvPackage `json:"package"`
	Version string     `json:"version"`
}

// osvPackage represents a package in the OSV API
type osvPackage struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
}

// osvResponse represents a response from the OSV API
type osvResponse struct {
	Vulns []any `json:"vulns"`
}

// checkOSVVulnerabilities checks if a package version has known vulnerabilities
func checkOSVVulnerabilities(
	logger *logrus.Logger,
	client HTTPClient,
	ecosystem string,
	packageName string,
	version string,
) (bool, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("osv:%s:%s:%s", ecosystem, packageName, version)
	if cached, ok := osvCache.Load(cacheKey); ok {
		entry := cached.(osvCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.hasVulns, nil
		}
		// Expired, delete from cache
		osvCache.Delete(cacheKey)
	}

	// Map our ecosystem names to OSV ecosystem names
	osvEcosystem := mapToOSVEcosystem(ecosystem)
	if osvEcosystem == "" {
		return false, fmt.Errorf("unsupported ecosystem for OSV: %s", ecosystem)
	}

	// Create request
	reqBody := osvRequest{
		Package: osvPackage{
			Ecosystem: osvEcosystem,
			Name:      packageName,
		},
		Version: version,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("failed to marshal OSV request: %w", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), OSVAPITimeout)
	defer cancel()

	// Make request
	req, err := http.NewRequestWithContext(ctx, "POST", OSVURL, strings.NewReader(string(reqBytes)))
	if err != nil {
		return false, fmt.Errorf("failed to create OSV request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("OSV request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("OSV returned status %d", resp.StatusCode)
	}

	var osvResp osvResponse
	if err := json.NewDecoder(resp.Body).Decode(&osvResp); err != nil {
		return false, fmt.Errorf("failed to decode OSV response: %w", err)
	}

	hasVulns := len(osvResp.Vulns) > 0

	// Cache the result
	osvCache.Store(cacheKey, osvCacheEntry{
		hasVulns:  hasVulns,
		expiresAt: time.Now().Add(OSVCacheDuration),
	})

	logger.WithFields(logrus.Fields{
		"package":   packageName,
		"ecosystem": ecosystem,
		"version":   version,
		"hasVulns":  hasVulns,
		"vulnCount": len(osvResp.Vulns),
	}).Debug("OSV vulnerability check completed")

	return hasVulns, nil
}

// mapToOSVEcosystem maps our ecosystem names to OSV ecosystem names
func mapToOSVEcosystem(ecosystem string) string {
	switch strings.ToLower(ecosystem) {
	case "npm":
		return "npm"
	case "python", "python-pyproject":
		return "PyPI"
	case "go":
		return "Go"
	case "rust":
		return "crates.io"
	case "java-maven", "java-gradle":
		return "Maven"
	default:
		return ""
	}
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%d hours", hours)
	}
	days := hours / 24
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// ResetCooldownConfigForTesting resets the cooldown config singleton (for testing only)
func ResetCooldownConfigForTesting() {
	cooldownConfigOnce = sync.Once{}
	cooldownConfig = nil
}

// ClearOSVCacheForTesting clears the OSV cache (for testing only)
func ClearOSVCacheForTesting() {
	osvCache = sync.Map{}
}

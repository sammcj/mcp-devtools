package security

import (
	"os"
	"path/filepath"
	"strings"
)

// DenyListChecker manages file and domain access control
func (d *DenyListChecker) compilePatterns() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Compile file patterns
	d.compiledFiles = make([]PatternMatcher, 0, len(d.filePatterns))
	for _, pattern := range d.filePatterns {
		// Expand home directory references
		expandedPattern := d.expandHomePath(pattern)
		matcher := NewFilePathMatcher(expandedPattern)
		d.compiledFiles = append(d.compiledFiles, matcher)
	}

	// Compile domain patterns
	d.compiledDomains = make([]PatternMatcher, 0, len(d.domainPatterns))
	for _, pattern := range d.domainPatterns {
		var matcher PatternMatcher
		if strings.Contains(pattern, "*") {
			matcher = NewGlobMatcher(pattern)
		} else {
			matcher = NewLiteralMatcher(pattern)
		}
		d.compiledDomains = append(d.compiledDomains, matcher)
	}

	return nil
}

// IsFileBlocked checks if a file path is blocked by deny rules
func (d *DenyListChecker) IsFileBlocked(filePath string) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	// Expand and clean the requested path
	expandedPath := d.expandHomePath(filePath)
	cleanPath := filepath.Clean(expandedPath)
	absPath, _ := filepath.Abs(cleanPath)

	// Check against compiled patterns
	for _, matcher := range d.compiledFiles {
		if matcher.Match(absPath) || matcher.Match(cleanPath) || matcher.Match(filePath) {
			return true
		}
	}

	// Check against original patterns for backward compatibility
	for _, pattern := range d.filePatterns {
		expandedPattern := d.expandHomePath(pattern)
		cleanPattern := filepath.Clean(expandedPattern)

		// Check various path representations
		if d.pathMatches(absPath, cleanPattern) ||
			d.pathMatches(cleanPath, cleanPattern) ||
			d.pathMatches(filePath, expandedPattern) {
			return true
		}
	}

	return false
}

// IsDomainBlocked checks if a domain is blocked by deny rules
func (d *DenyListChecker) IsDomainBlocked(domain string) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	domain = strings.ToLower(strings.TrimSpace(domain))

	// Check against compiled patterns
	for _, matcher := range d.compiledDomains {
		if matcher.Match(domain) {
			return true
		}
	}

	// Check against original patterns with wildcard support
	for _, pattern := range d.domainPatterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))

		if strings.HasPrefix(pattern, "*.") {
			// Wildcard subdomain matching
			baseDomain := strings.TrimPrefix(pattern, "*.")
			if domain == baseDomain || strings.HasSuffix(domain, "."+baseDomain) {
				return true
			}
		} else if domain == pattern {
			// Exact match
			return true
		}
	}

	return false
}

// expandHomePath expands ~ to the user's home directory
func (d *DenyListChecker) expandHomePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// pathMatches checks if a path matches a pattern
func (d *DenyListChecker) pathMatches(path, pattern string) bool {
	// Direct string match
	if strings.Contains(path, pattern) {
		return true
	}

	// Glob pattern match
	if matched, _ := filepath.Match(pattern, path); matched {
		return true
	}

	// Check if the path starts with the pattern (directory matching)
	if strings.HasPrefix(path, pattern) {
		return true
	}

	// Check if the pattern is a parent directory of the path
	if strings.HasPrefix(path, pattern+string(filepath.Separator)) {
		return true
	}

	return false
}

// UpdateDenyLists updates the deny lists with new patterns
func (d *DenyListChecker) UpdateDenyLists(files, domains []string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.filePatterns = files
	d.domainPatterns = domains

	return d.compilePatterns()
}

// GetDenyLists returns current deny list patterns
func (d *DenyListChecker) GetDenyLists() (files, domains []string) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	// Return copies to prevent external modification
	files = make([]string, len(d.filePatterns))
	copy(files, d.filePatterns)

	domains = make([]string, len(d.domainPatterns))
	copy(domains, d.domainPatterns)

	return files, domains
}

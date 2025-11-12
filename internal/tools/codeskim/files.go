//go:build cgo && (darwin || (linux && amd64))

package codeskim

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// supportedExts is the set of file extensions that code_skim can process
var supportedExts = map[string]bool{
	".py":    true,
	".go":    true,
	".js":    true,
	".jsx":   true,
	".ts":    true,
	".tsx":   true,
	".rs":    true,
	".c":     true,
	".h":     true,
	".cpp":   true,
	".cc":    true,
	".cxx":   true,
	".hpp":   true,
	".hxx":   true,
	".hh":    true,
	".sh":    true,
	".bash":  true,
	".html":  true,
	".htm":   true,
	".css":   true,
	".swift": true,
	".java":  true,
	".yml":   true,
	".yaml":  true,
	".hcl":   true,
	".tf":    true,
}

// ResolveFiles resolves the source parameter to a list of files to process
// Handles:
// - Single file path
// - Directory path (recursively finds all supported files)
// - Glob pattern
// - Array of any combination of the above
func ResolveFiles(source any) ([]string, error) {
	// Handle array of sources
	if sources, ok := source.([]any); ok {
		var allFiles []string
		seen := make(map[string]bool)
		for _, src := range sources {
			srcStr, ok := src.(string)
			if !ok {
				return nil, fmt.Errorf("source array item must be a string")
			}
			files, err := resolveSingleSource(srcStr)
			if err != nil {
				return nil, err
			}
			// Deduplicate files
			for _, file := range files {
				if !seen[file] {
					seen[file] = true
					allFiles = append(allFiles, file)
				}
			}
		}
		return allFiles, nil
	}

	// Handle string source
	if srcStr, ok := source.(string); ok {
		return resolveSingleSource(srcStr)
	}

	return nil, fmt.Errorf("source must be a string or array of strings")
}

// resolveSingleSource resolves a single source path
func resolveSingleSource(source string) ([]string, error) {
	// Check if source exists
	info, err := os.Stat(source)
	if err == nil {
		// Path exists
		if info.IsDir() {
			// Directory - find all supported files recursively
			return findSupportedFiles(source)
		}
		// Single file
		return []string{source}, nil
	}

	// Path doesn't exist - might be a glob pattern
	if strings.ContainsAny(source, "*?[]") {
		return resolveGlob(source)
	}

	// Not a valid path or glob
	return nil, fmt.Errorf("source not found: %s (not a file, directory, or valid glob pattern)", source)
}

// findSupportedFiles recursively finds all supported source files in a directory
func findSupportedFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != dir {
			return filepath.SkipDir
		}

		// Skip hidden files
		if !d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		// Check if file has supported extension
		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if supportedExts[ext] {
				files = append(files, path)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", dir, err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no supported files found in directory: %s", dir)
	}

	return files, nil
}

// resolveGlob resolves a glob pattern to a list of files
func resolveGlob(pattern string) ([]string, error) {
	// Validate glob pattern complexity (prevent abuse with excessive recursion)
	if err := validateGlobPattern(pattern); err != nil {
		return nil, err
	}

	// Use doublestar for ** support
	matches, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no files match glob pattern: %s", pattern)
	}

	// Filter to only supported files
	var files []string

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}

		// Skip directories
		if info.IsDir() {
			continue
		}

		// Check extension
		ext := strings.ToLower(filepath.Ext(match))
		if supportedExts[ext] {
			files = append(files, match)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no supported files match glob pattern: %s", pattern)
	}

	return files, nil
}

// validateGlobPattern validates a glob pattern for excessive complexity
func validateGlobPattern(pattern string) error {
	const (
		maxRecursiveWildcards = 5   // Maximum number of ** patterns
		maxPatternLength      = 500 // Maximum pattern length
	)

	// Check pattern length
	if len(pattern) > maxPatternLength {
		return fmt.Errorf("glob pattern too long: %d characters (max: %d)", len(pattern), maxPatternLength)
	}

	// Count recursive wildcards (**)
	recursiveCount := strings.Count(pattern, "**")
	if recursiveCount > maxRecursiveWildcards {
		return fmt.Errorf("too many recursive wildcards (**) in pattern: %d (max: %d)", recursiveCount, maxRecursiveWildcards)
	}

	return nil
}

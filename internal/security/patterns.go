package security

import (
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// LiteralMatcher matches exact strings
type LiteralMatcher struct {
	pattern string
}

func NewLiteralMatcher(pattern string) *LiteralMatcher {
	return &LiteralMatcher{pattern: pattern}
}

func (m *LiteralMatcher) Match(content string) bool {
	return strings.Contains(content, m.pattern)
}

func (m *LiteralMatcher) String() string {
	return "literal:" + m.pattern
}

// ContainsMatcher matches substrings with intelligent home directory expansion
type ContainsMatcher struct {
	pattern  string
	patterns []string // Expanded patterns for home directory matching
}

func NewContainsMatcher(pattern string) *ContainsMatcher {
	matcher := &ContainsMatcher{pattern: pattern}
	matcher.patterns = matcher.generateSearchPatterns(pattern)
	return matcher
}

func (m *ContainsMatcher) Match(content string) bool {
	lowerContent := strings.ToLower(content)

	// Check all expanded patterns
	for _, pattern := range m.patterns {
		if strings.Contains(lowerContent, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

func (m *ContainsMatcher) generateSearchPatterns(pattern string) []string {
	patterns := []string{pattern} // Always include original pattern

	// Auto-detect and expand home directory patterns
	if strings.Contains(pattern, "~") || strings.Contains(pattern, "$HOME") {
		patterns = append(patterns, m.expandHomeDirectoryPatterns(pattern)...)
	}

	return patterns
}

func (m *ContainsMatcher) expandHomeDirectoryPatterns(pattern string) []string {
	var expanded []string

	// Handle tilde patterns: ~/path -> various expansions
	if strings.Contains(pattern, "~/") {
		relativePath := strings.Replace(pattern, "~/", "", 1)
		expanded = append(expanded,
			"$HOME/"+relativePath,   // $HOME expansion
			"${HOME}/"+relativePath, // ${HOME} expansion
		)

		// Also add just the relative path for broader matching
		if relativePath != "" {
			expanded = append(expanded, "/"+relativePath) // Common path separator
		}
	}

	// Handle $HOME patterns: $HOME/path -> tilde and other expansions
	if strings.Contains(pattern, "$HOME") {
		tildeExpanded := strings.ReplaceAll(pattern, "$HOME", "~")
		expanded = append(expanded, tildeExpanded)

		braceExpanded := strings.ReplaceAll(pattern, "$HOME", "${HOME}")
		expanded = append(expanded, braceExpanded)

		// Extract relative path for broader matching
		if strings.HasPrefix(pattern, "$HOME/") {
			relativePath := strings.TrimPrefix(pattern, "$HOME/")
			if relativePath != "" {
				expanded = append(expanded, "/"+relativePath)
			}
		}
	}

	// Handle ${HOME} patterns
	if strings.Contains(pattern, "${HOME}") {
		tildeExpanded := strings.ReplaceAll(pattern, "${HOME}", "~")
		expanded = append(expanded, tildeExpanded)

		dollarExpanded := strings.ReplaceAll(pattern, "${HOME}", "$HOME")
		expanded = append(expanded, dollarExpanded)

		// Extract relative path for broader matching
		if strings.HasPrefix(pattern, "${HOME}/") {
			relativePath := strings.TrimPrefix(pattern, "${HOME}/")
			if relativePath != "" {
				expanded = append(expanded, "/"+relativePath)
			}
		}
	}

	return expanded
}

func (m *ContainsMatcher) String() string {
	return "contains:" + m.pattern
}

// PrefixMatcher matches string prefixes
type PrefixMatcher struct {
	pattern string
}

func NewPrefixMatcher(pattern string) *PrefixMatcher {
	return &PrefixMatcher{pattern: pattern}
}

func (m *PrefixMatcher) Match(content string) bool {
	return strings.HasPrefix(strings.ToLower(content), strings.ToLower(m.pattern))
}

func (m *PrefixMatcher) String() string {
	return "prefix:" + m.pattern
}

// SuffixMatcher matches string suffixes
type SuffixMatcher struct {
	pattern string
}

func NewSuffixMatcher(pattern string) *SuffixMatcher {
	return &SuffixMatcher{pattern: pattern}
}

func (m *SuffixMatcher) Match(content string) bool {
	return strings.HasSuffix(strings.ToLower(content), strings.ToLower(m.pattern))
}

func (m *SuffixMatcher) String() string {
	return "suffix:" + m.pattern
}

// FilePathMatcher matches file paths with expansion
type FilePathMatcher struct {
	pattern string
}

func NewFilePathMatcher(pattern string) *FilePathMatcher {
	return &FilePathMatcher{pattern: pattern}
}

func (m *FilePathMatcher) Match(content string) bool {
	// Generate all possible path patterns to match
	patterns := m.generatePathPatterns(m.pattern)

	// Check for exact match or path references
	for _, pattern := range patterns {
		if strings.Contains(content, pattern) {
			return true
		}
	}

	// Check for glob-style matching
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		for _, pattern := range patterns {
			if matched, _ := filepath.Match(pattern, line); matched {
				return true
			}
		}
	}

	return false
}

func (m *FilePathMatcher) generatePathPatterns(path string) []string {
	patterns := []string{path} // Always include original pattern

	// Handle tilde expansion
	if strings.HasPrefix(path, "~/") {
		// Generate common home directory patterns
		relativePath := strings.TrimPrefix(path, "~/")
		patterns = append(patterns,
			"$HOME/"+relativePath,    // $HOME expansion
			"${HOME}/"+relativePath,  // ${HOME} expansion
			"/home/*/"+relativePath,  // Common Linux pattern
			"/Users/*/"+relativePath, // Common macOS pattern
		)
	}

	// Handle $HOME patterns
	if strings.Contains(path, "$HOME") {
		homeExpanded := strings.ReplaceAll(path, "$HOME", "~")
		patterns = append(patterns, homeExpanded)
	}

	// Handle ${HOME} patterns
	if strings.Contains(path, "${HOME}") {
		homeExpanded := strings.ReplaceAll(path, "${HOME}", "~")
		patterns = append(patterns, homeExpanded)
		homeExpanded2 := strings.ReplaceAll(path, "${HOME}", "$HOME")
		patterns = append(patterns, homeExpanded2)
	}

	return patterns
}

func (m *FilePathMatcher) String() string {
	return "filepath:" + m.pattern
}

// URLMatcher matches URLs
type URLMatcher struct {
	pattern string
}

func NewURLMatcher(pattern string) *URLMatcher {
	return &URLMatcher{pattern: pattern}
}

func (m *URLMatcher) Match(content string) bool {
	// Check for direct string match first
	if strings.Contains(content, m.pattern) {
		return true
	}

	// Parse URLs from content and check each one
	urls := m.extractURLs(content)
	for _, u := range urls {
		if strings.Contains(u, m.pattern) {
			return true
		}

		// Parse the URL and check components
		if parsed, err := url.Parse(u); err == nil {
			if strings.Contains(parsed.Scheme, m.pattern) ||
				strings.Contains(parsed.Host, m.pattern) ||
				strings.Contains(parsed.Path, m.pattern) {
				return true
			}
		}
	}

	return false
}

func (m *URLMatcher) extractURLs(content string) []string {
	// Simple URL extraction using regex
	urlRegex := regexp.MustCompile(`https?://[^\s<>"]+`)
	return urlRegex.FindAllString(content, -1)
}

func (m *URLMatcher) String() string {
	return "url:" + m.pattern
}

// EntropyMatcher matches content based on entropy
type EntropyMatcher struct {
	threshold float64
	maxSize   int
}

func NewEntropyMatcher(threshold float64) *EntropyMatcher {
	return &EntropyMatcher{threshold: threshold, maxSize: 65536} // Default 64KB
}

func NewEntropyMatcherWithMaxSize(threshold float64, maxSize int) *EntropyMatcher {
	return &EntropyMatcher{threshold: threshold, maxSize: maxSize}
}

func (m *EntropyMatcher) Match(content string) bool {
	// Apply size limit for entropy analysis
	analysisContent := content
	if m.maxSize > 0 && len(content) > m.maxSize {
		analysisContent = content[:m.maxSize]
	}

	// Split content into words/tokens and check entropy of each
	words := strings.Fields(analysisContent)
	for _, word := range words {
		if len(word) > 20 && m.calculateEntropy(word) >= m.threshold {
			return true
		}
	}

	// Also check lines for high entropy
	lines := strings.Split(analysisContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 40 && m.calculateEntropy(line) >= m.threshold {
			return true
		}
	}

	return false
}

func (m *EntropyMatcher) calculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	// Count character frequencies
	freq := make(map[rune]float64)
	for _, char := range s {
		freq[char]++
	}

	// Calculate Shannon entropy
	entropy := 0.0
	length := float64(len(s))
	for _, count := range freq {
		probability := count / length
		entropy -= probability * math.Log2(probability)
	}

	return entropy
}

func (m *EntropyMatcher) String() string {
	return "entropy:" + string(rune(m.threshold))
}

// RegexMatcher matches using regular expressions
type RegexMatcher struct {
	pattern string
	regex   *regexp.Regexp
}

func NewRegexMatcher(pattern string) (*RegexMatcher, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	return &RegexMatcher{
		pattern: pattern,
		regex:   regex,
	}, nil
}

func (m *RegexMatcher) Match(content string) bool {
	return m.regex.MatchString(content)
}

func (m *RegexMatcher) String() string {
	return "regex:" + m.pattern
}

// GlobMatcher matches using glob patterns
type GlobMatcher struct {
	pattern string
}

func NewGlobMatcher(pattern string) *GlobMatcher {
	return &GlobMatcher{pattern: pattern}
}

func (m *GlobMatcher) Match(content string) bool {
	// Split content into words and lines to check each against the glob pattern
	words := strings.Fields(content)
	for _, word := range words {
		if matched, _ := filepath.Match(m.pattern, word); matched {
			return true
		}
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matched, _ := filepath.Match(m.pattern, line); matched {
			return true
		}
	}

	return false
}

func (m *GlobMatcher) String() string {
	return "glob:" + m.pattern
}

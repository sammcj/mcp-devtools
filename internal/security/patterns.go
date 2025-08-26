package security

import (
	"context"
	"encoding/base64"
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
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
		if after, ok := strings.CutPrefix(pattern, "$HOME/"); ok {
			relativePath := after
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
		if after, ok := strings.CutPrefix(pattern, "${HOME}/"); ok {
			relativePath := after
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
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
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
	if after, ok := strings.CutPrefix(path, "~/"); ok {
		// Generate common home directory patterns
		relativePath := after
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
	// Simple URL extraction using regex with timeout protection
	urlRegex := regexp.MustCompile(`https?://[^\s<>"]+`)

	// Use timeout protection for URL extraction
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	resultChan := make(chan []string, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// If regex panics, return empty slice
				resultChan <- []string{}
			}
		}()
		resultChan <- urlRegex.FindAllString(content, -1)
	}()

	select {
	case result := <-resultChan:
		return result
	case <-ctx.Done():
		// Timeout occurred - return empty slice to fail safe
		return []string{}
	}
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
	words := strings.FieldsSeq(analysisContent)
	for word := range words {
		if len(word) > 20 && m.calculateEntropy(word) >= m.threshold {
			return true
		}
	}

	// Also check lines for high entropy
	lines := strings.SplitSeq(analysisContent, "\n")
	for line := range lines {
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

// RegexMatcher matches using regular expressions with timeout protection
type RegexMatcher struct {
	pattern string
	regex   *regexp.Regexp
	timeout time.Duration
}

func NewRegexMatcher(pattern string) (*RegexMatcher, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	return &RegexMatcher{
		pattern: pattern,
		regex:   regex,
		timeout: 100 * time.Millisecond, // Default 100ms timeout to prevent ReDoS
	}, nil
}

func NewRegexMatcherWithTimeout(pattern string, timeout time.Duration) (*RegexMatcher, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	return &RegexMatcher{
		pattern: pattern,
		regex:   regex,
		timeout: timeout,
	}, nil
}

func (m *RegexMatcher) Match(content string) bool {
	return m.MatchWithTimeout(content, m.timeout)
}

func (m *RegexMatcher) MatchWithTimeout(content string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resultChan := make(chan bool, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				// If regex panics, consider it a non-match
				resultChan <- false
			}
		}()
		resultChan <- m.regex.MatchString(content)
	}()

	select {
	case result := <-resultChan:
		return result
	case <-ctx.Done():
		// Timeout occurred - consider it a non-match to fail safe
		return false
	}
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
	words := strings.FieldsSeq(content)
	for word := range words {
		if matched, _ := filepath.Match(m.pattern, word); matched {
			return true
		}
	}

	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
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

// isLikelyBase64 performs fast heuristic checks to identify potential base64 content
// Uses character analysis and length requirements to avoid expensive operations on non-base64 data
func isLikelyBase64(content string) bool {
	// Trim whitespace for analysis
	originalLength := len(content)
	content = strings.TrimSpace(content)

	// Quick length checks - base64 minimum viable size and reasonable maximum
	if len(content) < 16 || len(content) > 1048576 { // 1MB max for heuristic check
		if logrus.GetLevel() <= logrus.DebugLevel && originalLength > 16 {
			logrus.WithFields(logrus.Fields{
				"content_length": len(content),
				"reason":         "length_check_failed",
				"min_length":     16,
				"max_length":     1048576,
			}).Debug("isLikelyBase64: failed length check")
		}
		return false
	}

	// Base64 strings are typically multiples of 4 characters
	if len(content)%4 != 0 {
		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.WithFields(logrus.Fields{
				"content_length": len(content),
				"modulo_4":       len(content) % 4,
				"reason":         "not_multiple_of_4",
			}).Debug("isLikelyBase64: failed modulo check")
		}
		return false
	}

	// Fast character set validation - cheaper than regex
	validChars := 0
	paddingChars := 0

	for i, r := range content {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '+' || r == '/' {
			validChars++
		} else if r == '=' {
			// Padding characters should only be at the end
			if i < len(content)-2 {
				if logrus.GetLevel() <= logrus.DebugLevel {
					logrus.WithFields(logrus.Fields{
						"content_length":    len(content),
						"padding_position":  i,
						"expected_position": len(content) - 2,
						"reason":            "padding_not_at_end",
					}).Debug("isLikelyBase64: failed padding position check")
				}
				return false
			}
			paddingChars++
		} else {
			if logrus.GetLevel() <= logrus.DebugLevel {
				logrus.WithFields(logrus.Fields{
					"content_length": len(content),
					"invalid_char":   string(r),
					"char_position":  i,
					"reason":         "invalid_character",
				}).Debug("isLikelyBase64: failed character validation")
			}
			return false
		}
	}

	// At least 90% valid base64 characters, max 2 padding chars
	validPercentage := float64(validChars) / float64(len(content))
	isLikely := validPercentage >= 0.90 && paddingChars <= 2

	if logrus.GetLevel() <= logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"content_length":   len(content),
			"valid_chars":      validChars,
			"padding_chars":    paddingChars,
			"valid_percentage": validPercentage,
			"threshold":        0.90,
			"is_likely_base64": isLikely,
			"content_preview":  content[:min(50, len(content))],
		}).Debug("isLikelyBase64: heuristic check completed")
	}

	return isLikely
}

// safeBase64Decode safely decodes base64 content with strict size limits
// Returns decoded content and success flag, prevents memory bombs and malformed input attacks
func safeBase64Decode(content string, maxDecodedSize int) ([]byte, bool) {
	// Trim whitespace
	originalLength := len(content)
	content = strings.TrimSpace(content)

	if logrus.GetLevel() <= logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"original_length":  originalLength,
			"trimmed_length":   len(content),
			"max_decoded_size": maxDecodedSize,
			"content_preview":  content[:min(50, len(content))],
		}).Debug("safeBase64Decode: starting decode attempt")
	}

	// Pre-flight size check - base64 decoded size is roughly 3/4 of encoded size
	estimatedSize := (len(content) * 3) / 4
	if estimatedSize > maxDecodedSize {
		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.WithFields(logrus.Fields{
				"estimated_size":   estimatedSize,
				"max_decoded_size": maxDecodedSize,
				"reason":           "estimated_size_exceeds_limit",
			}).Debug("safeBase64Decode: pre-flight size check failed")
		}
		return nil, false
	}

	// Attempt decode with Go's standard library (which has built-in safety checks)
	decoded, err := base64.StdEncoding.DecodeString(content)
	encodingUsed := "standard"
	if err != nil {
		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.WithError(err).Debug("safeBase64Decode: standard encoding failed, trying URL-safe")
		}
		// Try URL-safe base64 as fallback
		decoded, err = base64.URLEncoding.DecodeString(content)
		encodingUsed = "url-safe"
		if err != nil {
			if logrus.GetLevel() <= logrus.DebugLevel {
				logrus.WithError(err).Debug("safeBase64Decode: both standard and URL-safe decoding failed")
			}
			return nil, false
		}
	}

	// Final size check on actual decoded content
	if len(decoded) > maxDecodedSize {
		if logrus.GetLevel() <= logrus.DebugLevel {
			logrus.WithFields(logrus.Fields{
				"actual_decoded_size": len(decoded),
				"max_decoded_size":    maxDecodedSize,
				"reason":              "actual_size_exceeds_limit",
			}).Debug("safeBase64Decode: final size check failed")
		}
		return nil, false
	}

	if logrus.GetLevel() <= logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"encoding_used":     encodingUsed,
			"original_length":   len(content),
			"decoded_length":    len(decoded),
			"decoded_preview":   string(decoded[:min(50, len(decoded))]),
			"decode_successful": true,
		}).Debug("safeBase64Decode: decoding completed successfully")
	}

	return decoded, true
}

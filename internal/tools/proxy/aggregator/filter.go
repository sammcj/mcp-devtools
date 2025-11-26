package aggregator

import (
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

// Filter handles tool filtering based on include and ignore patterns.
type Filter struct {
	includePatterns []*regexp.Regexp
	ignorePatterns  []*regexp.Regexp
}

// NewFilter creates a new filter with the given include and ignore patterns.
func NewFilter(includeTools, ignoreTools []string) *Filter {
	f := &Filter{
		includePatterns: make([]*regexp.Regexp, 0, len(includeTools)),
		ignorePatterns:  make([]*regexp.Regexp, 0, len(ignoreTools)),
	}

	for _, pattern := range includeTools {
		regex := patternToRegex(pattern)
		f.includePatterns = append(f.includePatterns, regex)
		logrus.WithField("pattern", pattern).Debug("added include pattern")
	}

	for _, pattern := range ignoreTools {
		regex := patternToRegex(pattern)
		f.ignorePatterns = append(f.ignorePatterns, regex)
		logrus.WithField("pattern", pattern).Debug("added ignore pattern")
	}

	return f
}

// ShouldInclude returns true if the tool should be included based on filter rules.
// Logic:
// - If include patterns exist, tool must match at least one include pattern
// - If ignore patterns exist, tool must NOT match any ignore pattern
// - If both exist, tool must match include AND not match ignore
func (f *Filter) ShouldInclude(toolName string) bool {
	// Check include patterns first (if any)
	if len(f.includePatterns) > 0 {
		matched := false
		for _, pattern := range f.includePatterns {
			if pattern.MatchString(toolName) {
				matched = true
				logrus.WithFields(logrus.Fields{
					"tool":    toolName,
					"pattern": pattern.String(),
				}).Debug("tool matched include pattern")
				break
			}
		}
		if !matched {
			logrus.WithField("tool", toolName).Debug("tool did not match any include pattern")
			return false
		}
	}

	// Check ignore patterns (if any)
	for _, pattern := range f.ignorePatterns {
		if pattern.MatchString(toolName) {
			logrus.WithFields(logrus.Fields{
				"tool":    toolName,
				"pattern": pattern.String(),
			}).Debug("tool matched ignore pattern")
			return false
		}
	}

	return true
}

// patternToRegex converts a glob pattern to a case-insensitive regex.
func patternToRegex(pattern string) *regexp.Regexp {
	// Escape special regex characters except *
	escaped := regexp.QuoteMeta(pattern)
	// Replace \* with .*
	escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
	// Use (?i) flag for case-insensitive matching
	return regexp.MustCompile("(?i)^" + escaped + "$")
}

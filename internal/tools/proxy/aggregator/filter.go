package aggregator

import (
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

// Filter handles tool filtering based on ignore patterns.
type Filter struct {
	patterns []*regexp.Regexp
}

// NewFilter creates a new filter with the given ignore patterns.
func NewFilter(ignorePatterns []string) *Filter {
	f := &Filter{
		patterns: make([]*regexp.Regexp, 0, len(ignorePatterns)),
	}

	for _, pattern := range ignorePatterns {
		regex := patternToRegex(pattern)
		f.patterns = append(f.patterns, regex)
		logrus.WithField("pattern", pattern).Debug("added ignore pattern")
	}

	return f
}

// ShouldInclude returns true if the tool should be included (not ignored).
func (f *Filter) ShouldInclude(toolName string) bool {
	for _, pattern := range f.patterns {
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

// patternToRegex converts a glob pattern to a regex.
func patternToRegex(pattern string) *regexp.Regexp {
	// Escape special regex characters except *
	escaped := regexp.QuoteMeta(pattern)
	// Replace \* with .*
	escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
	return regexp.MustCompile("^" + escaped + "$")
}

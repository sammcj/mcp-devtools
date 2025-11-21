package unit

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNoStdoutStderrWrites ensures no code writes to stdout/stderr in ways that could break stdio mode
func TestNoStdoutStderrWrites(t *testing.T) {
	// Patterns that are NEVER allowed in main code
	forbiddenPatterns := []struct {
		pattern     *regexp.Regexp
		description string
	}{
		{
			pattern:     regexp.MustCompile(`\bfmt\.Print\b`),
			description: "fmt.Print (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\bfmt\.Println\b`),
			description: "fmt.Println (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\bfmt\.Printf\b`),
			description: "fmt.Printf (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\bfmt\.Fprint\(os\.Stdout`),
			description: "fmt.Fprint(os.Stdout, ...) (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\bfmt\.Fprintf\(os\.Stdout`),
			description: "fmt.Fprintf(os.Stdout, ...) (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\bfmt\.Fprintln\(os\.Stdout`),
			description: "fmt.Fprintln(os.Stdout, ...) (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\bfmt\.Fprint\(os\.Stderr`),
			description: "fmt.Fprint(os.Stderr, ...) (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\bfmt\.Fprintf\(os\.Stderr`),
			description: "fmt.Fprintf(os.Stderr, ...) (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\bfmt\.Fprintln\(os\.Stderr`),
			description: "fmt.Fprintln(os.Stderr, ...) (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\bos\.Stdout\.Write\b`),
			description: "os.Stdout.Write (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\bos\.Stderr\.Write\b`),
			description: "os.Stderr.Write (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\blog\.Print\b`),
			description: "log.Print (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\blog\.Println\b`),
			description: "log.Println (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\blog\.Printf\b`),
			description: "log.Printf (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\bprint\(`),
			description: "print() (use logger instead)",
		},
		{
			pattern:     regexp.MustCompile(`\bprintln\(`),
			description: "println() (use logger instead)",
		},
	}

	// Files/paths that are allowed to use these patterns (CLI commands, tests, etc.)
	allowedPaths := []string{
		"tests/",                      // All test files
		"main.go",                     // CLI command handlers (version, security-config-*)
		".github/",                    // GitHub workflows and docs
		"docs/",                       // Documentation
		"README.md",                   // Readme
		"internal/tools/error_logger", // Error logger writes to file, not stdout/stderr
	}

	// Specific exceptions with comments explaining why they're safe
	allowedExceptions := map[string][]string{
		"config.go": {
			"print('available')", // Python print in shell command string
			"print(docling.",     // Python print in shell command string
			"print(torch.",       // Python print in shell command string
		},
		"main.go": {
			"fmt.Printf(\"mcp-devtools version",          // version command
			"fmt.Printf(\"Commit:",                       // version command
			"fmt.Printf(\"Built:",                        // version command
			"fmt.Printf(\"User config file does not",     // security-config-diff command
			"fmt.Println(\"A default configuration",      // security-config-diff command
			"fmt.Println(\"✅ User configuration matches", // security-config-diff command
			"fmt.Println(\"📋 Configuration",              // security-config-diff command
			"fmt.Printf(\"User config:",                  // security-config-diff command
			"fmt.Println(\"Default config:",              // security-config-diff command
			"fmt.Println(\"User config size:",            // security-config-diff command
			"fmt.Println(\"Default config size:",         // security-config-diff command
			"fmt.Printf(\"⚠️  Warning:",                  // security-config-diff command
			"fmt.Printf(\"📄 Version difference:",         // security-config-diff command
			"fmt.Printf(\"📊 Rules:",                      // security-config-diff command
			"fmt.Printf(\"🆕 New rules",                   // security-config-diff command
			"fmt.Printf(\"🆕 New setting",                 // security-config-diff command
			"fmt.Println(\"\\n🔄 Updating",                // security-config-diff command
			"fmt.Printf(\"📦 Backup created:",             // security-config-diff command
			"fmt.Printf(\"✅ Configuration updated:",      // security-config-diff command
			"fmt.Println(\"⚠️  Note:",                    // security-config-diff command
			"fmt.Println(\"\\n💡 To update",               // security-config-diff command
			"fmt.Printf(\"   mcp-devtools",               // security-config-diff command
			"fmt.Printf(\"❌ Configuration file",          // security-config-validate command
			"fmt.Println(\"💡 The file will",              // security-config-validate command
			"fmt.Printf(\"🔍 Validating",                  // security-config-validate command
			"fmt.Println(\"=\" + strings.Repeat",         // security-config-validate command
			"fmt.Printf(\"❌ YAML parsing",                // security-config-validate command
			"fmt.Printf(\"\\n📄 File has",                 // security-config-validate command
			"fmt.Printf(\"⚠️  Line %d contains tabs",     // security-config-validate command
			"fmt.Printf(\"⚠️  Line %d may have",          // security-config-validate command
			"fmt.Println(\"✅ Configuration is valid\")",  // security-config-validate command
			"fmt.Println(\"\\n📊 Configuration",           // security-config-validate command
			"fmt.Println(\"========================",     // security-config-validate command
			"fmt.Printf(\"Version:",                      // security-config-validate command
			"fmt.Printf(\"Security enabled:",             // security-config-validate command
			"fmt.Printf(\"Default action:",               // security-config-validate command
			"fmt.Printf(\"Auto reload:",                  // security-config-validate command
			"fmt.Printf(\"Max content size:",             // security-config-validate command
			"fmt.Printf(\"Max scan size:",                // security-config-validate command
			"fmt.Printf(\"Size exceeded behaviour:",      // security-config-validate command
			"fmt.Printf(\"Rules defined:",                // security-config-validate command
			"fmt.Printf(\"Trusted domains:",              // security-config-validate command
			"fmt.Printf(\"Denied files:",                 // security-config-validate command
			"fmt.Printf(\"Denied domains:",               // security-config-validate command
			"fmt.Println(\"\\n✅ Configuration",           // security-config-validate command
		},
	}

	violations := []string{}

	// Walk the entire project
	err := filepath.Walk("../..", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories, non-Go files
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Get relative path for easier checking
		relPath, err := filepath.Rel("../..", path)
		if err != nil {
			relPath = path
		}

		// Check if this path is in the allowed list
		isAllowed := false
		for _, allowed := range allowedPaths {
			if strings.Contains(relPath, allowed) {
				isAllowed = true
				break
			}
		}
		if isAllowed {
			return nil
		}

		// Read and scan the file
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// Skip comments
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}

			// Check each forbidden pattern
			for _, forbidden := range forbiddenPatterns {
				if forbidden.pattern.MatchString(line) {
					// Check if this specific line is in the allowed exceptions
					isException := false
					if exceptions, ok := allowedExceptions[filepath.Base(relPath)]; ok {
						for _, exception := range exceptions {
							if strings.Contains(line, exception) {
								isException = true
								break
							}
						}
					}

					if !isException {
						violations = append(violations, formatViolation(relPath, lineNum, line, forbidden.description))
					}
				}
			}
		}

		return scanner.Err()
	})

	assert.NoError(t, err, "Failed to walk directory tree")

	// Report all violations
	if len(violations) > 0 {
		t.Errorf("\n❌ Found %d stdio safety violation(s):\n\n%s\n\n"+
			"These patterns can break the MCP stdio protocol.\n"+
			"Use logger instead, or add to allowedExceptions in tests/unit/stdio_safety_test.go if truly necessary.",
			len(violations),
			strings.Join(violations, "\n"))
	}
}

// formatViolation formats a violation message with context
func formatViolation(file string, line int, content, pattern string) string {
	return strings.TrimSpace(strings.ReplaceAll(`
  File: `+file+`
  Line: `+strconv.Itoa(line)+`
  Pattern: `+pattern+`
  Code: `+strings.TrimSpace(content)+`
`, "\t", "  "))
}

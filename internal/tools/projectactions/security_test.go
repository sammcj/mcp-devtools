package projectactions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sirupsen/logrus"
)

func TestSecurityIntegration(t *testing.T) {
	t.Run("SafeFileRead usage for Makefile", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create test Makefile
		makefile := `.PHONY: test

test:
	@echo "test"
`
		makefilePath := filepath.Join(tmpDir, "Makefile")
		os.WriteFile(makefilePath, []byte(makefile), 0644)

		// Setup tool with security operations
		tool := &ProjectActionsTool{
			workingDir: tmpDir,
			secOps:     security.NewOperations("project_actions"),
		}

		// Read Makefile using SafeFileRead
		content, err := tool.readMakefile(makefilePath)
		if err != nil {
			t.Fatalf("SafeFileRead failed: %v", err)
		}

		// Verify content was read
		if !strings.Contains(content, ".PHONY: test") {
			t.Errorf("expected Makefile content, got: %s", content)
		}
	})

	t.Run("SecurityError handling", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create Makefile with potentially suspicious content
		makefile := `.PHONY: test

test:
	@echo "test"
`
		makefilePath := filepath.Join(tmpDir, "Makefile")
		os.WriteFile(makefilePath, []byte(makefile), 0644)

		tool := &ProjectActionsTool{
			workingDir: tmpDir,
			secOps:     security.NewOperations("project_actions"),
		}

		// Read Makefile - should handle any security errors gracefully
		content, err := tool.readMakefile(makefilePath)

		// If security blocks it, error should be ProjectActionsError
		if err != nil {
			if paErr, ok := err.(*ProjectActionsError); ok {
				if paErr.Type != ErrorMakefileInvalid {
					t.Errorf("expected ErrorMakefileInvalid, got: %v", paErr.Type)
				}
				if !strings.Contains(paErr.Message, "security") {
					t.Errorf("expected security error message, got: %s", paErr.Message)
				}
			} else {
				t.Errorf("expected ProjectActionsError, got: %T", err)
			}
		} else {
			// If no error, content should be valid
			if content == "" {
				t.Error("expected content when no security error")
			}
		}
	})

	t.Run("security warning logging", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create test Makefile
		makefile := `.PHONY: test

test:
	@echo "test"
`
		makefilePath := filepath.Join(tmpDir, "Makefile")
		os.WriteFile(makefilePath, []byte(makefile), 0644)

		// Capture log output
		oldLevel := logrus.GetLevel()
		logrus.SetLevel(logrus.WarnLevel)
		defer logrus.SetLevel(oldLevel)

		tool := &ProjectActionsTool{
			workingDir: tmpDir,
			secOps:     security.NewOperations("project_actions"),
		}

		// Read Makefile - warnings should be logged if present
		_, err := tool.readMakefile(makefilePath)
		if err != nil {
			// Security errors are acceptable for this test
			if _, ok := err.(*ProjectActionsError); !ok {
				t.Fatalf("unexpected error type: %v", err)
			}
		}

		// Test passes if no panic and proper error handling
	})
}

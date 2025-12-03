package projectactions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateMakefile(t *testing.T) {
	languages := []string{"python", "rust", "go", "nodejs"}

	for _, lang := range languages {
		t.Run(lang+" template generation", func(t *testing.T) {
			tmpDir := t.TempDir()
			tool := &ProjectActionsTool{workingDir: tmpDir}

			err := tool.generateMakefile(lang)
			if err != nil {
				t.Fatalf("failed to generate Makefile for %s: %v", lang, err)
			}

			// Verify Makefile was created
			makefilePath := filepath.Join(tmpDir, "Makefile")
			content, err := os.ReadFile(makefilePath)
			if err != nil {
				t.Fatalf("failed to read generated Makefile: %v", err)
			}

			// Verify .PHONY line exists
			if !strings.Contains(string(content), ".PHONY:") {
				t.Error("generated Makefile missing .PHONY declaration")
			}

			// Verify common targets exist
			commonTargets := []string{"default", "test", "lint"}
			for _, target := range commonTargets {
				if !strings.Contains(string(content), target) {
					t.Errorf("generated Makefile missing target: %s", target)
				}
			}
		})
	}

	t.Run("verify tab indentation in all templates", func(t *testing.T) {
		for lang, template := range makefileTemplates {
			lines := strings.Split(template, "\n")
			for i, line := range lines {
				// Skip empty lines
				if line == "" {
					continue
				}
				// Skip target lines (lines with : that don't start with tab)
				if strings.Contains(line, ":") && !strings.HasPrefix(line, "\t") {
					continue
				}
				// Command lines should start with tab
				if !strings.HasPrefix(line, "\t") {
					t.Errorf("%s template line %d should start with tab, got: %q", lang, i+1, line)
				}
			}
		}
	})

	t.Run("invalid language rejection", func(t *testing.T) {
		tmpDir := t.TempDir()
		tool := &ProjectActionsTool{workingDir: tmpDir}

		err := tool.generateMakefile("invalid")
		if err == nil {
			t.Error("expected error for invalid language, got nil")
		}
	})
}

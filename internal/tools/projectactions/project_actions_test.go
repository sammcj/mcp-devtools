package projectactions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateWorkingDirectory(t *testing.T) {
	tool := &ProjectActionsTool{}

	t.Run("valid directory acceptance", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := tool.validateWorkingDirectory(tmpDir)
		if err != nil {
			t.Errorf("expected no error for valid directory, got: %v", err)
		}
	})

	t.Run("system directory rejection", func(t *testing.T) {
		systemDirs := []string{"/", "/bin", "/lib", "/usr", "/etc", "/var", "/sys", "/proc", "/dev", "/boot", "/sbin"}
		for _, dir := range systemDirs {
			err := tool.validateWorkingDirectory(dir)
			if err == nil {
				t.Errorf("expected error for system directory %s, got nil", dir)
			}
		}
	})

	t.Run("writability check", func(t *testing.T) {
		tmpDir := t.TempDir()
		readOnlyDir := filepath.Join(tmpDir, "readonly")
		if err := os.Mkdir(readOnlyDir, 0555); err != nil {
			t.Fatalf("failed to create read-only directory: %v", err)
		}

		err := tool.validateWorkingDirectory(readOnlyDir)
		if err == nil {
			t.Error("expected error for non-writable directory, got nil")
		}
	})
}

func TestParsePhonyTargets(t *testing.T) {
	tool := &ProjectActionsTool{}

	t.Run("valid target name extraction", func(t *testing.T) {
		makefile := `.PHONY: build test lint
.PHONY: clean

build:
	go build
`
		targets, err := tool.parsePhonyTargets(makefile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"build", "test", "lint", "clean"}
		if len(targets) != len(expected) {
			t.Errorf("expected %d targets, got %d", len(expected), len(targets))
		}
		for i, target := range expected {
			if targets[i] != target {
				t.Errorf("expected target %s, got %s", target, targets[i])
			}
		}
	})

	t.Run("invalid target name rejection", func(t *testing.T) {
		makefile := `.PHONY: valid-target invalid@target
`
		_, err := tool.parsePhonyTargets(makefile)
		if err == nil {
			t.Error("expected error for invalid target name, got nil")
		}
	})

	t.Run("malformed Makefile handling", func(t *testing.T) {
		// Empty Makefile
		targets, err := tool.parsePhonyTargets("")
		if err != nil {
			t.Errorf("unexpected error for empty Makefile: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("expected 0 targets for empty Makefile, got %d", len(targets))
		}

		// No .PHONY targets
		makefile := `build:
	go build
`
		targets, err = tool.parsePhonyTargets(makefile)
		if err != nil {
			t.Errorf("unexpected error for Makefile without .PHONY: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("expected 0 targets for Makefile without .PHONY, got %d", len(targets))
		}
	})
}

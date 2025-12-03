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

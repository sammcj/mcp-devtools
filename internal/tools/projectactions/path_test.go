package projectactions

import (
	"strings"
	"testing"
)

func TestValidateAndResolvePath(t *testing.T) {
	tmpDir := t.TempDir()
	tool := &ProjectActionsTool{workingDir: tmpDir}

	t.Run("path resolution with filepath.Clean/Abs", func(t *testing.T) {
		// Test normal path
		path, err := tool.validateAndResolvePath("test.txt")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(path, tmpDir) {
			t.Errorf("expected path to contain working dir %s, got %s", tmpDir, path)
		}

		// Test path with ./ prefix
		path, err = tool.validateAndResolvePath("./subdir/file.txt")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(path, tmpDir) {
			t.Errorf("expected path to contain working dir, got %s", path)
		}
	})

	t.Run("path traversal prevention", func(t *testing.T) {
		// Test ../ traversal
		_, err := tool.validateAndResolvePath("../outside.txt")
		if err == nil {
			t.Error("expected error for ../ traversal, got nil")
		}

		// Test multiple ../ traversal
		_, err = tool.validateAndResolvePath("../../outside.txt")
		if err == nil {
			t.Error("expected error for ../../ traversal, got nil")
		}
	})

	t.Run("working directory containment", func(t *testing.T) {
		// Test subdirectory (should succeed)
		path, err := tool.validateAndResolvePath("subdir/file.txt")
		if err != nil {
			t.Errorf("unexpected error for subdirectory: %v", err)
		}
		if !strings.HasPrefix(path, tmpDir) {
			t.Errorf("expected path to start with working dir %s, got %s", tmpDir, path)
		}

		// Test path that tries to escape via symlink-like pattern
		_, err = tool.validateAndResolvePath("subdir/../../etc/passwd")
		if err == nil {
			t.Error("expected error for escape attempt, got nil")
		}
	})
}

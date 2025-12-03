package projectactions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMakeTargetExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("create temp directory with test Makefile", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Create test Makefile
		makefile := `.PHONY: test echo

test:
	@echo "test passed"

echo:
	@echo "hello world"
`
		makefilePath := filepath.Join(tmpDir, "Makefile")
		err := os.WriteFile(makefilePath, []byte(makefile), 0644)
		if err != nil {
			t.Fatalf("failed to create test Makefile: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(makefilePath); err != nil {
			t.Errorf("Makefile not created: %v", err)
		}
	})

	t.Run("execute make target and verify output", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Create test Makefile
		makefile := `.PHONY: echo

echo:
	@echo "hello world"
`
		makefilePath := filepath.Join(tmpDir, "Makefile")
		os.WriteFile(makefilePath, []byte(makefile), 0644)

		// Setup tool
		tool := &ProjectActionsTool{
			workingDir:      tmpDir,
			makefileTargets: []string{"echo"},
		}

		// Execute target
		ctx := context.Background()
		result, err := tool.executeMakeTarget(ctx, "echo", false)
		if err != nil {
			t.Fatalf("failed to execute make target: %v", err)
		}

		// Verify output
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
		if result.Stdout == "" {
			t.Error("expected stdout output, got empty string")
		}
	})

	t.Run("verify exit code and timing capture", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Create test Makefile with sleep
		makefile := `.PHONY: slow

slow:
	@sleep 0.1
	@echo "done"
`
		makefilePath := filepath.Join(tmpDir, "Makefile")
		os.WriteFile(makefilePath, []byte(makefile), 0644)

		// Setup tool
		tool := &ProjectActionsTool{
			workingDir:      tmpDir,
			makefileTargets: []string{"slow"},
		}

		// Execute target
		ctx := context.Background()
		result, err := tool.executeMakeTarget(ctx, "slow", false)
		if err != nil {
			t.Fatalf("failed to execute make target: %v", err)
		}

		// Verify exit code
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}

		// Verify timing capture
		if result.Duration < 100*time.Millisecond {
			t.Errorf("expected duration >= 100ms, got %v", result.Duration)
		}

		// Verify working directory
		if result.WorkingDir != tmpDir {
			t.Errorf("expected working dir %s, got %s", tmpDir, result.WorkingDir)
		}
	})
}

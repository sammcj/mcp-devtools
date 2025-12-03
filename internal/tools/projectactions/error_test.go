package projectactions

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestErrorHandling(t *testing.T) {
	t.Run("command failure propagation", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Create Makefile with failing target
		makefile := `.PHONY: fail

fail:
	@exit 42
`
		os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefile), 0644)

		tool := &ProjectActionsTool{
			workingDir:      tmpDir,
			makefileTargets: []string{"fail"},
		}

		ctx := context.Background()
		result, err := tool.executeMakeTarget(ctx, "fail", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify non-zero exit code is propagated
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit code for failed command")
		}
	})

	t.Run("no automatic retry on failure", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Create Makefile that increments counter on each run
		counterFile := filepath.Join(tmpDir, "counter.txt")
		os.WriteFile(counterFile, []byte("0"), 0644)
		
		makefile := `.PHONY: count-and-fail

count-and-fail:
	@echo $$(( $$(cat counter.txt) + 1 )) > counter.txt
	@exit 1
`
		os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefile), 0644)

		tool := &ProjectActionsTool{
			workingDir:      tmpDir,
			makefileTargets: []string{"count-and-fail"},
		}

		ctx := context.Background()
		result, _ := tool.executeMakeTarget(ctx, "count-and-fail", false)

		// Verify command failed
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit code")
		}

		// Verify counter was only incremented once (no retry)
		counter, _ := os.ReadFile(counterFile)
		if strings.TrimSpace(string(counter)) != "1" {
			t.Errorf("expected counter=1 (no retry), got: %s", counter)
		}
	})

	t.Run("immediate error return", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Initialize git repo
		cmd := exec.Command("git", "init")
		cmd.Dir = tmpDir
		cmd.Run()

		tool := &ProjectActionsTool{
			workingDir:           tmpDir,
			maxCommitMessageSize: DefaultMaxCommitMessageSize,
		}

		ctx := context.Background()

		// Test invalid path returns immediately
		_, err := tool.executeGitAdd(ctx, []string{"../escape.txt"}, false)
		if err == nil {
			t.Error("expected immediate error for invalid path")
		}
		if !strings.Contains(err.Error(), "escape") {
			t.Errorf("expected error about path escape, got: %v", err)
		}

		// Test oversized commit message returns immediately
		largeMsg := strings.Repeat("x", DefaultMaxCommitMessageSize+1)
		_, err = tool.executeGitCommit(ctx, largeMsg, false)
		if err == nil {
			t.Error("expected immediate error for oversized message")
		}
		if !strings.Contains(err.Error(), "exceeds") {
			t.Errorf("expected error about size limit, got: %v", err)
		}

		// Test invalid target returns immediately
		tool.makefileTargets = []string{"valid"}
		_, err = tool.executeMakeTarget(ctx, "invalid", false)
		if err == nil {
			t.Error("expected immediate error for invalid target")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected error about target not found, got: %v", err)
		}
	})
}

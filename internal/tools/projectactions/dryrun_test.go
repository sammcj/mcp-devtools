package projectactions

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDryRunMode(t *testing.T) {
	t.Run("dry-run displays commands without execution", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create test Makefile
		makefile := `.PHONY: test

test:
	@echo "should not execute"
`
		os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefile), 0644)

		// Setup tool
		tool := &ProjectActionsTool{
			workingDir:      tmpDir,
			makefileTargets: []string{"test"},
		}

		// Execute in dry-run mode
		ctx := context.Background()
		result, err := tool.executeMakeTarget(ctx, "test", true)
		if err != nil {
			t.Fatalf("dry-run failed: %v", err)
		}

		// Verify command is shown
		if !strings.Contains(result.Command, "make test") {
			t.Errorf("expected command to contain 'make test', got: %s", result.Command)
		}

		// Verify no execution (stdout should be empty)
		if result.Stdout != "" {
			t.Errorf("expected no stdout in dry-run, got: %s", result.Stdout)
		}

		// Verify exit code is 0 for dry-run
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0 for dry-run, got: %d", result.ExitCode)
		}
	})

	t.Run("dry-run validates all parameters", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Initialize git repo
		cmd := exec.Command("git", "init")
		cmd.Dir = tmpDir
		cmd.Run()
		cmd = exec.Command("git", "config", "user.name", "Test User")
		cmd.Dir = tmpDir
		cmd.Run()
		cmd = exec.Command("git", "config", "user.email", "test@example.com")
		cmd.Dir = tmpDir
		cmd.Run()

		// Create test file
		testFile := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(testFile, []byte("content"), 0644)

		// Setup tool
		tool := &ProjectActionsTool{
			workingDir:           tmpDir,
			maxCommitMessageSize: DefaultMaxCommitMessageSize,
		}

		ctx := context.Background()

		// Test git add dry-run with valid path
		result, err := tool.executeGitAdd(ctx, []string{"test.txt"}, true)
		if err != nil {
			t.Errorf("dry-run git add with valid path failed: %v", err)
		}
		if !strings.Contains(result.Command, "git add") {
			t.Errorf("expected command to contain 'git add', got: %s", result.Command)
		}

		// Test git add dry-run with invalid path (should fail validation)
		_, err = tool.executeGitAdd(ctx, []string{"../escape.txt"}, true)
		if err == nil {
			t.Error("expected error for path escaping working directory in dry-run")
		}

		// Test git commit dry-run with valid message
		result, err = tool.executeGitCommit(ctx, "Test commit", true)
		if err != nil {
			t.Errorf("dry-run git commit with valid message failed: %v", err)
		}
		if !strings.Contains(result.Command, "git commit") {
			t.Errorf("expected command to contain 'git commit', got: %s", result.Command)
		}

		// Test git commit dry-run with oversized message
		largeMsg := strings.Repeat("x", DefaultMaxCommitMessageSize+1)
		_, err = tool.executeGitCommit(ctx, largeMsg, true)
		if err == nil {
			t.Error("expected error for oversized commit message in dry-run")
		}
	})
}

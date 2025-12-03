package projectactions

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		result, err := tool.executeMakeTarget(ctx, "echo", false, "")
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
		result, err := tool.executeMakeTarget(ctx, "slow", false, "")
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

func TestGitOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("initialize test git repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Initialize git repo
		cmd := exec.Command("git", "init")
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Configure git user
		exec.Command("git", "config", "user.name", "Test User").Run()
		exec.Command("git", "config", "user.email", "test@example.com").Run()

		// Verify .git directory exists
		gitDir := filepath.Join(tmpDir, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			t.Errorf(".git directory not created: %v", err)
		}
	})

	t.Run("git add with multiple files", func(t *testing.T) {
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

		// Create test files
		file1 := filepath.Join(tmpDir, "file1.txt")
		file2 := filepath.Join(tmpDir, "file2.txt")
		os.WriteFile(file1, []byte("content1"), 0644)
		os.WriteFile(file2, []byte("content2"), 0644)

		// Setup tool
		tool := &ProjectActionsTool{
			workingDir: tmpDir,
		}

		// Execute git add
		ctx := context.Background()
		result, err := tool.executeGitAdd(ctx, []string{"file1.txt", "file2.txt"}, false, "")
		if err != nil {
			t.Fatalf("failed to execute git add: %v", err)
		}

		// Verify exit code
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}

		// Verify files are staged
		cmd = exec.Command("git", "status", "--porcelain")
		cmd.Dir = tmpDir
		output, _ := cmd.Output()
		statusOutput := string(output)
		if !strings.Contains(statusOutput, "file1.txt") || !strings.Contains(statusOutput, "file2.txt") {
			t.Errorf("files not staged, git status: %s", statusOutput)
		}
	})

	t.Run("git commit with stdin message", func(t *testing.T) {
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

		// Create and stage a file
		file := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(file, []byte("test content"), 0644)
		cmd = exec.Command("git", "add", "test.txt")
		cmd.Dir = tmpDir
		cmd.Run()

		// Setup tool
		tool := &ProjectActionsTool{
			workingDir:           tmpDir,
			maxCommitMessageSize: DefaultMaxCommitMessageSize,
		}

		// Execute git commit
		ctx := context.Background()
		commitMsg := "Test commit message\n\nWith multiple lines"
		result, err := tool.executeGitCommit(ctx, commitMsg, false, "")
		if err != nil {
			t.Fatalf("failed to execute git commit: %v", err)
		}

		// Verify exit code
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}

		// Verify commit was created
		cmd = exec.Command("git", "log", "--oneline")
		cmd.Dir = tmpDir
		output, _ := cmd.Output()
		if len(output) == 0 {
			t.Error("no commits found after git commit")
		}
	})

	t.Run("verify git state after operations", func(t *testing.T) {
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

		// Create test files
		file1 := filepath.Join(tmpDir, "file1.txt")
		file2 := filepath.Join(tmpDir, "file2.txt")
		os.WriteFile(file1, []byte("content1"), 0644)
		os.WriteFile(file2, []byte("content2"), 0644)

		// Setup tool
		tool := &ProjectActionsTool{
			workingDir:           tmpDir,
			maxCommitMessageSize: DefaultMaxCommitMessageSize,
		}

		// Add files
		ctx := context.Background()
		_, err := tool.executeGitAdd(ctx, []string{"file1.txt", "file2.txt"}, false, "")
		if err != nil {
			t.Fatalf("failed to add files: %v", err)
		}

		// Commit files
		_, err = tool.executeGitCommit(ctx, "Initial commit", false, "")
		if err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		// Verify clean working tree
		cmd = exec.Command("git", "status", "--porcelain")
		cmd.Dir = tmpDir
		output, _ := cmd.Output()
		if len(output) > 0 {
			t.Errorf("expected clean working tree, got: %s", string(output))
		}

		// Verify commit exists
		cmd = exec.Command("git", "log", "--format=%s")
		cmd.Dir = tmpDir
		output, _ = cmd.Output()
		if !strings.Contains(string(output), "Initial commit") {
			t.Errorf("commit message not found in log: %s", string(output))
		}

		// Verify both files are in commit
		cmd = exec.Command("git", "ls-tree", "--name-only", "HEAD")
		cmd.Dir = tmpDir
		output, _ = cmd.Output()
		files := string(output)
		if !strings.Contains(files, "file1.txt") || !strings.Contains(files, "file2.txt") {
			t.Errorf("expected both files in commit, got: %s", files)
		}
	})
}

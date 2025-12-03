package projectactions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// ProjectActionsTool provides security-aware execution of project development tasks
type ProjectActionsTool struct {
	workingDir           string
	makefileTargets      []string
	maxCommitMessageSize int
}

// init registers the tool with the registry
func init() {
	tool := &ProjectActionsTool{}
	if err := tool.checkToolAvailability(); err != nil {
		logrus.Warn(err.Error())
	}
	registry.Register(tool)
}

// Definition returns the tool's definition for MCP registration
func (t *ProjectActionsTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"project_actions",
		mcp.WithDescription("Execute project development tasks (tests, linters, formatters) and git operations through a project's Makefile. Provides security-aware access to make targets and git add/commit operations."),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
}

// Execute handles tool invocation
func (t *ProjectActionsTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	logger.Info("Executing project_actions")
	return mcp.NewToolResultText("Not yet implemented"), nil
}

// validateWorkingDirectory validates the working directory is safe to use
func (t *ProjectActionsTool) validateWorkingDirectory(dir string) error {
	// Resolve to absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}

	// Block system directories
	systemDirs := []string{"/", "/bin", "/lib", "/usr", "/etc", "/var", "/sys", "/proc", "/dev", "/boot", "/sbin"}
	for _, sysDir := range systemDirs {
		if absDir == sysDir {
			return fmt.Errorf("working directory cannot be a system directory: %s", absDir)
		}
	}

	// Check writability via owner and permissions
	info, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("failed to stat working directory: %w", err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("failed to get file system info for working directory")
	}

	// Check if owner is current user
	if stat.Uid != uint32(os.Getuid()) {
		return fmt.Errorf("working directory not writable by current user: %s", absDir)
	}

	// Check owner write bit
	if info.Mode().Perm()&0200 == 0 {
		return fmt.Errorf("working directory not writable by current user: %s", absDir)
	}

	return nil
}

// checkToolAvailability verifies required tools are on PATH
func (t *ProjectActionsTool) checkToolAvailability() error {
	if _, err := exec.LookPath("make"); err != nil {
		return fmt.Errorf("make not found on PATH - install make to use this tool")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found on PATH - install git to use git operations")
	}
	return nil
}

package projectactions

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sirupsen/logrus"
)

// ProjectActionsTool provides security-aware execution of project development tasks
type ProjectActionsTool struct {
	workingDir           string
	makefileTargets      []string
	maxCommitMessageSize int
	secOps               *security.Operations
}

const (
	DefaultMaxCommitMessageSize = 16 * 1024 // 16 KB
)

// init registers the tool with the registry
func init() {
	maxSize := DefaultMaxCommitMessageSize
	if envSize := os.Getenv("PROJECT_ACTIONS_MAX_COMMIT_SIZE"); envSize != "" {
		if size, err := fmt.Sscanf(envSize, "%d", &maxSize); err == nil && size == 1 {
			// Successfully parsed
		}
	}

	tool := &ProjectActionsTool{
		secOps:               security.NewOperations("project_actions"),
		maxCommitMessageSize: maxSize,
	}
	if err := tool.checkToolAvailability(); err != nil {
		logrus.Warn(err.Error())
	}
	registry.Register(tool)
}

// Definition returns the tool's definition for MCP registration
func (t *ProjectActionsTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"project_actions",
		mcp.WithDescription("Execute project development tasks (tests, linters, formatters) through a project's Makefile and limited git operations. Operations: make targets (from .PHONY), 'add' (git add files), 'commit' (git commit), 'generate' (create Makefile). Requires security tool enabled."),
		mcp.WithString("operation",
			mcp.Required(),
			mcp.Description("Operation to perform: any .PHONY target from Makefile, 'add', 'commit', or 'generate'"),
		),
		mcp.WithString("working_directory",
			mcp.Description("Working directory (default: current directory)"),
		),
		mcp.WithArray("paths",
			mcp.Description("File paths for 'add' operation (relative to working directory)"),
		),
		mcp.WithString("message",
			mcp.Description("Commit message for 'commit' operation"),
		),
		mcp.WithString("language",
			mcp.Description("Language for 'generate' operation: python, rust, go, nodejs"),
		),
		mcp.WithBoolean("dry_run",
			mcp.Description("Preview commands without execution"),
		),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
}

// Execute handles tool invocation
func (t *ProjectActionsTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Parse operation parameter
	operation, ok := args["operation"].(string)
	if !ok || operation == "" {
		return mcp.NewToolResultError("operation parameter is required"), nil
	}

	// Parse working directory
	workingDir := "."
	if wd, ok := args["working_directory"].(string); ok && wd != "" {
		workingDir = wd
	}

	// Validate and set working directory
	t.workingDir = workingDir
	if err := t.validateWorkingDirectory(t.workingDir); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Parse dry_run
	dryRun, _ := args["dry_run"].(bool)

	// Route to appropriate handler
	var result *CommandResult
	var err error

	switch operation {
	case "add":
		paths, _ := args["paths"].([]interface{})
		var pathStrs []string
		for _, p := range paths {
			if ps, ok := p.(string); ok {
				pathStrs = append(pathStrs, ps)
			}
		}
		result, err = t.executeGitAdd(ctx, pathStrs, dryRun)

	case "commit":
		message, _ := args["message"].(string)
		result, err = t.executeGitCommit(ctx, message, dryRun)

	case "generate":
		language, _ := args["language"].(string)
		if err := t.generateMakefile(language); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		// Read and parse generated Makefile
		makefilePath := filepath.Join(t.workingDir, "Makefile")
		content, err := t.readMakefile(makefilePath)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		t.makefileTargets, err = t.parsePhonyTargets(content)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Generated Makefile in %s with targets: %s", t.workingDir, strings.Join(t.makefileTargets, ", "))), nil

	default:
		// Try as make target - read Makefile if not already loaded
		if len(t.makefileTargets) == 0 {
			makefilePath := filepath.Join(t.workingDir, "Makefile")
			content, err := t.readMakefile(makefilePath)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			t.makefileTargets, err = t.parsePhonyTargets(content)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		result, err = t.executeMakeTarget(ctx, operation, dryRun)
	}

	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Display working directory in output
	output := fmt.Sprintf("Working directory: %s\n\nCommand: %s\n", result.WorkingDir, result.Command)
	if result.Stdout != "" {
		output += fmt.Sprintf("\nStdout:\n%s", result.Stdout)
	}
	if result.Stderr != "" {
		output += fmt.Sprintf("\nStderr:\n%s", result.Stderr)
	}
	if result.ExitCode != 0 {
		output += fmt.Sprintf("\nExit code: %d", result.ExitCode)
	}
	if result.Duration > 0 {
		output += fmt.Sprintf("\nDuration: %s", result.Duration)
	}

	return mcp.NewToolResultText(output), nil
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
			return fmt.Errorf(ErrMsgSystemDir, absDir)
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
		return fmt.Errorf(ErrMsgNotWritable, absDir)
	}

	// Check owner write bit
	if info.Mode().Perm()&0200 == 0 {
		return fmt.Errorf(ErrMsgNotWritable, absDir)
	}

	return nil
}

// checkToolAvailability verifies required tools are on PATH
func (t *ProjectActionsTool) checkToolAvailability() error {
	if _, err := exec.LookPath("make"); err != nil {
		return fmt.Errorf(ErrMsgMakeNotFound)
	}
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf(ErrMsgGitNotFound)
	}
	return nil
}

// readMakefile reads the Makefile using security-aware file operations
func (t *ProjectActionsTool) readMakefile(makefilePath string) (string, error) {
	result, err := t.secOps.SafeFileRead(makefilePath)
	if err != nil {
		// Check if it's a security error
		if secErr, ok := err.(*security.SecurityError); ok {
			return "", &ProjectActionsError{
				Type:    ErrorMakefileInvalid,
				Message: fmt.Sprintf("Makefile blocked by security: %s", secErr.Message),
				Cause:   err,
			}
		}
		return "", err
	}

	// Log security warnings if present
	if result.SecurityResult != nil {
		logrus.WithFields(logrus.Fields{
			"security_id": result.SecurityResult.ID,
			"message":     result.SecurityResult.Message,
		}).Warn("Security warning for Makefile")
	}

	return string(result.Content), nil
}

// parsePhonyTargets extracts and validates .PHONY target names from Makefile content
func (t *ProjectActionsTool) parsePhonyTargets(makefileContent string) ([]string, error) {
	// Regex to match .PHONY lines
	phonyRegex := regexp.MustCompile(`(?m)^\.PHONY:\s*(.+)$`)
	matches := phonyRegex.FindAllStringSubmatch(makefileContent, -1)

	if len(matches) == 0 {
		return []string{}, nil
	}

	// Target name validation regex (alphanumeric, hyphen, underscore only)
	targetRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	var targets []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		// Split targets by whitespace
		parts := strings.Fields(match[1])
		for _, target := range parts {
			// Validate target name
			if !targetRegex.MatchString(target) {
				return nil, &ProjectActionsError{
					Type:    ErrorInvalidTarget,
					Message: fmt.Sprintf("invalid target name '%s': must contain only alphanumeric, hyphen, or underscore characters", target),
				}
			}
			targets = append(targets, target)
		}
	}

	return targets, nil
}

// generateMakefile generates a language-specific Makefile and writes it to the working directory
func (t *ProjectActionsTool) generateMakefile(language string) error {
	// Validate language parameter
	template, ok := makefileTemplates[language]
	if !ok {
		return &ProjectActionsError{
			Type:    ErrorMakefileInvalid,
			Message: fmt.Sprintf("invalid language '%s': supported languages are python, rust, go, nodejs", language),
		}
	}

	// Write Makefile to working directory
	makefilePath := filepath.Join(t.workingDir, "Makefile")
	if err := os.WriteFile(makefilePath, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to write Makefile: %w", err)
	}

	return nil
}

// executeMakeTarget executes a make target with streaming output
func (t *ProjectActionsTool) executeMakeTarget(ctx context.Context, target string, dryRun bool) (*CommandResult, error) {
	// Validate target exists in makefileTargets
	found := false
	for _, t := range t.makefileTargets {
		if t == target {
			found = true
			break
		}
	}
	if !found {
		return nil, &ProjectActionsError{
			Type:    ErrorInvalidTarget,
			Message: fmt.Sprintf(ErrMsgInvalidTarget, target),
		}
	}

	// Build command
	cmd := exec.CommandContext(ctx, "make", target)
	cmd.Dir = t.workingDir

	if dryRun {
		return &CommandResult{
			Command:    fmt.Sprintf("make %s", target),
			WorkingDir: t.workingDir,
		}, nil
	}

	// Execute with streaming output
	return t.executeCommand(ctx, cmd)
}

// executeCommand executes a command with real-time streaming output
func (t *ProjectActionsTool) executeCommand(ctx context.Context, cmd *exec.Cmd) (*CommandResult, error) {
	// Set up separate stdout/stderr pipes
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var wg sync.WaitGroup

	// Start command
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Stream stdout in real-time
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(&stdoutBuf, stdoutPipe)
	}()

	// Stream stderr in real-time
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(&stderrBuf, stderrPipe)
	}()

	// Wait for streaming to complete
	wg.Wait()

	// Wait for command to finish and capture exit code
	err = cmd.Wait()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, err
		}
	}

	return &CommandResult{
		Command:    cmd.String(),
		Stdout:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
		ExitCode:   exitCode,
		Duration:   duration,
		WorkingDir: cmd.Dir,
	}, nil
}

// validateAndResolvePath validates and resolves a relative path to absolute
func (t *ProjectActionsTool) validateAndResolvePath(relativePath string) (string, error) {
	// Clean path
	cleanPath := filepath.Clean(relativePath)

	// Resolve to absolute path
	absPath, err := filepath.Abs(filepath.Join(t.workingDir, cleanPath))
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Verify path is within working directory
	absWorkingDir, err := filepath.Abs(t.workingDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve working directory: %w", err)
	}

	relToWorkDir, err := filepath.Rel(absWorkingDir, absPath)
	if err != nil || strings.HasPrefix(relToWorkDir, "..") {
		return "", &ProjectActionsError{
			Type:    ErrorInvalidPath,
			Message: fmt.Sprintf(ErrMsgPathEscape, relativePath),
		}
	}

	return absPath, nil
}

// executeGitAdd executes git add for multiple files in a single batch operation
func (t *ProjectActionsTool) executeGitAdd(ctx context.Context, paths []string, dryRun bool) (*CommandResult, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no paths provided for git add")
	}

	// Validate and resolve each path
	var validPaths []string
	for _, path := range paths {
		absPath, err := t.validateAndResolvePath(path)
		if err != nil {
			return nil, err
		}
		validPaths = append(validPaths, absPath)
	}

	// Build single git add command with all paths
	args := append([]string{"add"}, validPaths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = t.workingDir

	if dryRun {
		return &CommandResult{
			Command:    fmt.Sprintf("git add %s", strings.Join(validPaths, " ")),
			WorkingDir: t.workingDir,
		}, nil
	}

	// Execute with streaming output
	result, err := t.executeCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrMsgGitFailed, err)
	}
	if result.ExitCode != 0 {
		result.Stderr = fmt.Sprintf("%s\n%s", ErrMsgGitFailed, result.Stderr)
	}
	return result, nil
}

// executeGitCommit executes git commit with message passed via stdin
func (t *ProjectActionsTool) executeGitCommit(ctx context.Context, message string, dryRun bool) (*CommandResult, error) {
	// Validate message size
	if len(message) > t.maxCommitMessageSize {
		return nil, &ProjectActionsError{
			Type:    ErrorCommitTooLarge,
			Message: fmt.Sprintf(ErrMsgCommitTooLarge, t.maxCommitMessageSize/1024),
		}
	}

	// Build command with --file=- to read from stdin
	cmd := exec.CommandContext(ctx, "git", "commit", "--file=-")
	cmd.Dir = t.workingDir
	cmd.Stdin = strings.NewReader(message)

	if dryRun {
		return &CommandResult{
			Command:    "git commit --file=-",
			WorkingDir: t.workingDir,
		}, nil
	}

	// Execute with streaming output
	result, err := t.executeCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrMsgGitFailed, err)
	}
	if result.ExitCode != 0 {
		result.Stderr = fmt.Sprintf("%s\n%s", ErrMsgGitFailed, result.Stderr)
	}
	return result, nil
}

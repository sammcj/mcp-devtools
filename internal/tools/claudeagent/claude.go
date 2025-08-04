package claudeagent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// ClaudeTool implements the tools.Tool interface for the claude CLI
type ClaudeTool struct{}

const (
	defaultModel = "sonnet"
)

// init registers the tool with the registry if enabled
func init() {
	enabledAgents := os.Getenv("ENABLE_AGENTS")
	if strings.Contains(enabledAgents, "claude") {
		registry.Register(&ClaudeTool{})
	}
}

// Definition returns the tool's definition for MCP registration
func (t *ClaudeTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"claude-agent",
		mcp.WithDescription("Provides access to Claude Code AI Agent. You can call out to this tool to treat Claude as a sub-agent for tasks like reviewing completed implementations and for help with troubleshooting when stubborn problems."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("A clear, concise prompt to send to Claude CLI to instruct the AI Agent to perform a specific task. Can include @file, @directory/, @./ references for context."),
		),
		mcp.WithString("override-model",
			mcp.Description(fmt.Sprintf("Force Claude to use a different model. Default: %s.", defaultModel)),
		),
		mcp.WithBoolean("yolo-mode",
			mcp.Description("Optional: Bypass all permission checks and allow the agent to write and execute anything"),
			mcp.DefaultBool(false),
		),
		mcp.WithBoolean("continue-last-conversation",
			mcp.Description("Optional: Continue the most recent conversation."),
			mcp.DefaultBool(false),
		),
		mcp.WithString("resume-specific-session",
			mcp.Description("Optional: Resume a conversation - provide a session ID."),
		),
		mcp.WithArray("include-directories",
			mcp.Description("Optional: Fully qualified paths to additional directories to allow the agent to access."),
		),
	)
}

// Execute executes the tool's logic by calling the claude CLI
func (t *ClaudeTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Executing claude tool")

	timeoutStr := os.Getenv("AGENT_TIMEOUT")
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil || timeout <= 0 {
		timeout = 180 // Default to 3 minutes
	}

	prompt, ok := args["prompt"].(string)
	if !ok || prompt == "" {
		return nil, fmt.Errorf("prompt is a required parameter")
	}

	model, _ := args["override-model"].(string)
	if model == "" {
		model = defaultModel
	}

	yoloMode, _ := args["yolo-mode"].(bool)
	continueLast, _ := args["continue-last-conversation"].(bool)
	resumeSession, _ := args["resume-specific-session"].(string)
	includeDirs, _ := args["include-directories"].([]interface{})

	var sessionID string
	if !continueLast && resumeSession == "" {
		sessionID = uuid.New().String()
	}

	output, err := t.runClaude(ctx, logger, time.Duration(timeout)*time.Second, prompt, model, yoloMode, continueLast, resumeSession, sessionID, includeDirs)
	if err != nil {
		if err == context.DeadlineExceeded {
			timeoutMsg := fmt.Sprintf("\n\nThe Claude Agent hit the configured timeout of %d seconds, output may be truncated!", timeout)
			return mcp.NewToolResultText(output + timeoutMsg), nil
		}
		logger.WithError(err).Error("Claude tool execution failed")
		return nil, fmt.Errorf("claude command failed: %w", err)
	}

	if sessionID != "" {
		output = fmt.Sprintf("%s\n\nSession ID: %s", output, sessionID)
	}

	return mcp.NewToolResultText(output), nil
}

func (t *ClaudeTool) runClaude(ctx context.Context, logger *logrus.Logger, timeout time.Duration, prompt, model string, yoloMode, continueLast bool, resumeSession, sessionID string, includeDirs []interface{}) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmdArgs := []string{"--print"}

	if model != "" {
		cmdArgs = append(cmdArgs, "--model", model)
	}
	if yoloMode {
		cmdArgs = append(cmdArgs, "--dangerously-skip-permissions")
	}
	if continueLast {
		cmdArgs = append(cmdArgs, "--continue")
	}
	if resumeSession != "" {
		cmdArgs = append(cmdArgs, "--resume", resumeSession)
	}
	if sessionID != "" {
		cmdArgs = append(cmdArgs, "--session-id", sessionID)
	}
	for _, dir := range includeDirs {
		if d, ok := dir.(string); ok {
			cmdArgs = append(cmdArgs, "--add-dir", d)
		}
	}

	if systemPrompt := os.Getenv("CLAUDE_SYSTEM_PROMPT"); systemPrompt != "" {
		cmdArgs = append(cmdArgs, "--append-system-prompt", systemPrompt)
	}
	if permissionMode := os.Getenv("CLAUDE_PERMISSION_MODE"); permissionMode != "" {
		cmdArgs = append(cmdArgs, "--permission-mode", permissionMode)
	}

	logger.Infof("Running command: claude %v", cmdArgs)

	cmd := exec.CommandContext(ctx, "claude", cmdArgs...)
	cmd.Stdin = strings.NewReader(prompt)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return outb.String(), context.DeadlineExceeded
		}
		stderr := errb.String()
		if stderr != "" {
			return "", fmt.Errorf("error: %v, stderr: %s", err, stderr)
		}
		return "", fmt.Errorf("error: %v", err)
	}

	return outb.String(), nil
}

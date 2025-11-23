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
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// ClaudeTool implements the tools.Tool interface for the claude CLI
type ClaudeTool struct{}

const (
	defaultModel               = "sonnet"
	DefaultMaxResponseSize     = 2 * 1024 * 1024 // 2MB default limit
	AgentMaxResponseSizeEnvVar = "AGENT_MAX_RESPONSE_SIZE"
)

// init registers the tool with the registry
func init() {
	registry.Register(&ClaudeTool{})
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
		tools.AddConditionalParameter("yolo-mode",
			"Optional: Bypass all permission checks and allow the agent to write and execute anything"),
		mcp.WithBoolean("continue-last-conversation",
			mcp.Description("Optional: Continue the most recent conversation."),
			mcp.DefaultBool(false),
		),
		mcp.WithString("resume-specific-session",
			mcp.Description("Optional: Resume a conversation - provide a session ID."),
		),
		mcp.WithArray("include-directories",
			mcp.Description("Optional: Fully qualified paths to additional directories to allow the agent to access."),
			mcp.WithStringItems(),
		),
		// Destructive tool annotations
		mcp.WithReadOnlyHintAnnotation(false),   // Agent can execute arbitrary commands
		mcp.WithDestructiveHintAnnotation(true), // Can perform destructive operations via external agent
		mcp.WithIdempotentHintAnnotation(false), // Agent operations are not idempotent
		mcp.WithOpenWorldHintAnnotation(true),   // Agent can interact with external systems
	)
}

// Execute executes the tool's logic by calling the claude CLI
func (t *ClaudeTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
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

	yoloModeParam, _ := args["yolo-mode"].(bool)
	yoloMode := tools.GetEffectivePermissionsValue(yoloModeParam)
	continueLast, _ := args["continue-last-conversation"].(bool)
	resumeSession, _ := args["resume-specific-session"].(string)
	includeDirs, _ := args["include-directories"].([]any)

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

	// Apply response size limits
	output = t.ApplyResponseSizeLimit(output, logger)

	return mcp.NewToolResultText(output), nil
}

func (t *ClaudeTool) runClaude(ctx context.Context, logger *logrus.Logger, timeout time.Duration, prompt, model string, yoloMode, continueLast bool, resumeSession, sessionID string, includeDirs []any) (string, error) {
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
			return "", fmt.Errorf("error: %w, stderr: %s", err, stderr)
		}
		return "", fmt.Errorf("error: %w", err)
	}

	return outb.String(), nil
}

// GetMaxResponseSize returns the configured maximum response size
func (t *ClaudeTool) GetMaxResponseSize() int {
	if sizeStr := os.Getenv(AgentMaxResponseSizeEnvVar); sizeStr != "" {
		if size, err := strconv.Atoi(sizeStr); err == nil && size > 0 {
			return size
		}
	}
	return DefaultMaxResponseSize
}

// ApplyResponseSizeLimit truncates the response if it exceeds the configured size limit
func (t *ClaudeTool) ApplyResponseSizeLimit(output string, logger *logrus.Logger) string {
	maxSize := t.GetMaxResponseSize()

	// Check if output exceeds limit
	if len(output) <= maxSize {
		return output
	}

	// Truncate and add informative message
	truncated := output[:maxSize]

	// Try to truncate at a natural boundary (line break) within the last 100 characters
	if lastNewline := strings.LastIndex(truncated[maxSize-100:], "\n"); lastNewline != -1 {
		truncated = truncated[:maxSize-100+lastNewline]
	}

	sizeInMB := float64(maxSize) / (1024 * 1024)
	message := fmt.Sprintf("\n\n[RESPONSE TRUNCATED: Output exceeded %.1fMB limit. Original size: %.1fMB. Use AGENT_MAX_RESPONSE_SIZE environment variable to adjust limit.]",
		sizeInMB, float64(len(output))/(1024*1024))

	logger.Warnf("Claude agent response truncated from %d bytes to %d bytes due to size limit", len(output), len(truncated))

	return truncated + message
}

// ProvideExtendedInfo provides detailed usage information for the claude agent tool
func (t *ClaudeTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Get Claude to review a completed implementation",
				Arguments: map[string]any{
					"prompt": "Please review the authentication implementation in @src/auth/ and check for security best practices, edge cases, and potential improvements.",
				},
				ExpectedResult: "Claude agent analyzes the authentication code and provides detailed feedback on security, implementation quality, and suggestions for improvement",
			},
			{
				Description: "Ask Claude to help debug a specific issue",
				Arguments: map[string]any{
					"prompt":         "I'm getting a 'connection refused' error when trying to connect to the database. Here's the error: @logs/database.log. Can you help troubleshoot?",
					"override-model": "opus",
				},
				ExpectedResult: "Claude agent examines the log file and provides specific debugging steps and potential solutions for the database connection issue",
			},
			{
				Description: "Continue a previous conversation",
				Arguments: map[string]any{
					"prompt":                     "Thanks for the previous suggestions. I've implemented the caching layer you recommended. Can you now help me add monitoring?",
					"continue-last-conversation": true,
				},
				ExpectedResult: "Claude agent continues from the previous conversation context and helps with adding monitoring to the caching implementation",
			},
			{
				Description: "Get help with a complex refactoring task in yolo mode",
				Arguments: map[string]any{
					"prompt":    "I need to refactor this legacy payment processing code in @src/payments/ to use the new payment gateway API. Please help me migrate it while preserving all existing functionality.",
					"yolo-mode": true,
				},
				ExpectedResult: "Claude agent performs the refactoring with full permissions, making necessary code changes and updates across the payment processing system",
			},
			{
				Description: "Resume a specific session with additional directory access",
				Arguments: map[string]any{
					"prompt":                  "Now let's work on the frontend components that integrate with the API we just built.",
					"resume-specific-session": "abc123-def456-ghi789",
					"include-directories":     []string{"/Users/username/project/frontend", "/Users/username/project/shared"},
				},
				ExpectedResult: "Claude agent resumes the specified session and gains access to frontend and shared directories to work on UI integration",
			},
		},
		CommonPatterns: []string{
			"Use @file or @directory/ syntax to provide context about specific files or directories",
			"Use continue-last-conversation for iterative work on the same topic",
			"Use override-model to get different perspectives (sonnet for speed, opus for complex analysis)",
			"Use yolo-mode only when you trust Claude to make changes without permission prompts",
			"Include specific error messages, logs, or symptoms in your prompts for better debugging help",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Agent timeout after 3 minutes",
				Solution: "Complex tasks may need more time. Set AGENT_TIMEOUT environment variable to increase timeout (in seconds). Example: AGENT_TIMEOUT=600 for 10 minutes.",
			},
			{
				Problem:  "Response truncated due to size limit",
				Solution: "Large responses are truncated for memory safety. Increase AGENT_MAX_RESPONSE_SIZE environment variable (in bytes) or ask Claude to provide more concise responses.",
			},
			{
				Problem:  "Permission denied errors in yolo mode",
				Solution: "Even in yolo-mode, Claude respects file system permissions. Ensure the process has necessary read/write permissions to the files and directories being accessed.",
			},
			{
				Problem:  "Session ID not found when resuming",
				Solution: "Session IDs expire after some time. Use continue-last-conversation for recent sessions, or start a new conversation if the session ID is no longer valid.",
			},
		},
		ParameterDetails: map[string]string{
			"prompt":                     "Clear, specific instruction to Claude. Use @file or @directory/ syntax for context. Be detailed about what you want Claude to analyze, implement, or help with.",
			"override-model":             "Model selection affects response quality vs speed. 'sonnet' (default, fast), 'haiku' (fastest), 'opus' (most capable). Choose based on task complexity.",
			"yolo-mode":                  "Bypasses permission prompts, allowing Claude to write/modify files directly. Use with caution - only when you trust Claude's judgement completely.",
			"continue-last-conversation": "Resumes most recent conversation with context. Useful for iterative work where previous context is important for the current task.",
			"resume-specific-session":    "Resumes specific session by ID. Session IDs are provided in previous responses. Use when you need to return to a specific conversation thread.",
			"include-directories":        "Array of additional directory paths Claude can access. Expands Claude's context beyond default allowed directories for comprehensive analysis.",
		},
		WhenToUse:    "Use when you need external, expert AI assistance for code review, debugging complex issues, architectural decisions, refactoring guidance, or when you're stuck on challenging implementation problems or when directly asked to use Claude Code or Claude CLI by the user.",
		WhenNotToUse: "Don't use for simple questions that don't require code analysis, when you need real-time responses, or for tasks that require access to external APIs or services Claude can't access.",
	}
}

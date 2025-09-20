package codexagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// CodexTool implements the tools.Tool interface for Codex CLI
type CodexTool struct{}

const (
	DefaultTimeout             = 180             // 3 minutes default timeout
	DefaultMaxResponseSize     = 2 * 1024 * 1024 // 2MB default limit
	AgentMaxResponseSizeEnvVar = "AGENT_MAX_RESPONSE_SIZE"
	AgentTimeoutEnvVar         = "AGENT_TIMEOUT"
)

// init registers the tool with the registry
func init() {
	registry.Register(&CodexTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *CodexTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"codex-agent",
		mcp.WithDescription("Provides integration with the Codex CLI through MCP. Enables AI agents to leverage Codex's capabilities for code analysis, generation, and assistance. Exclusively uses 'codex exec' command for non-interactive execution."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("A clear, concise prompt to send to Codex CLI to instruct the AI Agent to perform a specific task. Supports @file, @directory/ syntax for file references."),
		),
		mcp.WithString("override-model",
			mcp.Description("Model to override the default. No default model is provided, allowing Codex to use user's configured default."),
		),
		mcp.WithString("sandbox",
			mcp.Description("Sandbox policy for execution security. Options: read-only, workspace-write, danger-full-access. WARNING: Controls execution security context."),
		),
		mcp.WithBoolean("full-auto",
			mcp.Description("Enable low-friction sandboxed automatic execution. In exec mode, implies --sandbox workspace-write."),
			mcp.DefaultBool(false),
		),
		mcp.WithBoolean("yolo-mode",
			mcp.Description("DANGER: Bypass all approvals and sandbox restrictions (maps to --dangerously-bypass-approvals-and-sandbox). Use with extreme caution."),
			mcp.DefaultBool(false),
		),
		mcp.WithBoolean("resume",
			mcp.Description("Continue the most recent session using --last flag."),
			mcp.DefaultBool(false),
		),
		mcp.WithString("session-id",
			mcp.Description("Specify a session identifier for resuming specific sessions."),
		),
		mcp.WithString("profile",
			mcp.Description("Configuration profile to use from config.toml."),
		),
		mcp.WithArray("config",
			mcp.Description("Configuration overrides in key=value format. Supports dotted path notation for nested values with JSON parsing."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithArray("images",
			mcp.Description("Image files to attach to the prompt. Multiple files supported."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithString("cd",
			mcp.Description("Working directory for Codex execution. Directory must exist."),
		),
		mcp.WithBoolean("skip-git-repo-check",
			mcp.Description("Skip git repository validation checks."),
			mcp.DefaultBool(false),
		),
	)
}

// Execute executes the tool's logic by calling the Codex CLI
func (t *CodexTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Check if codex-agent tool is enabled (disabled by default for security)
	if !tools.IsToolEnabled("codex-agent") {
		return nil, fmt.Errorf("codex agent tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'codex-agent'")
	}

	logger.Info("Executing Codex tool")

	// Get timeout from environment or use default
	timeout := t.GetTimeout()

	// Validate required prompt parameter
	prompt, ok := args["prompt"].(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("prompt is a required parameter and cannot be empty")
	}

	// Parse optional parameters
	overrideModel, _ := args["override-model"].(string)
	sandbox, _ := args["sandbox"].(string)
	fullAuto, _ := args["full-auto"].(bool)
	yoloMode, _ := args["yolo-mode"].(bool)
	resume, _ := args["resume"].(bool)
	sessionID, _ := args["session-id"].(string)
	profile, _ := args["profile"].(string)
	cdDir, _ := args["cd"].(string)
	skipGitRepoCheck, _ := args["skip-git-repo-check"].(bool)

	// Handle config array parameter
	var config []string
	if configInterface, exists := args["config"]; exists {
		if configArray, ok := configInterface.([]any); ok {
			for _, item := range configArray {
				if configStr, ok := item.(string); ok {
					config = append(config, configStr)
				}
			}
		}
	}

	// Handle images array parameter
	var images []string
	if imagesInterface, exists := args["images"]; exists {
		if imagesArray, ok := imagesInterface.([]any); ok {
			for _, item := range imagesArray {
				if imageStr, ok := item.(string); ok {
					images = append(images, imageStr)
				}
			}
		}
	}

	// Validate cd directory exists if provided (Requirement 6.3)
	if cdDir != "" {
		if err := validateDirectory(cdDir); err != nil {
			return nil, fmt.Errorf("invalid directory for cd parameter: %w", err)
		}
	}

	output, err := t.runCodex(ctx, logger, time.Duration(timeout)*time.Second, prompt, overrideModel, sandbox, fullAuto, yoloMode, resume, sessionID, profile, config, images, cdDir, skipGitRepoCheck)
	if err != nil {
		if err == context.DeadlineExceeded {
			timeoutMsg := fmt.Sprintf("\n\nThe Codex Agent hit the configured timeout of %d seconds, output may be truncated!", timeout)
			return mcp.NewToolResultText(output + timeoutMsg), nil
		}
		logger.WithError(err).Error("Codex tool execution failed")
		return nil, fmt.Errorf("codex command failed: %w", err)
	}

	// Apply response size limits
	output = t.ApplyResponseSizeLimit(output, logger)

	// Include session ID in response (Requirement 4.4)
	if resume && sessionID != "" {
		output = fmt.Sprintf("Resumed Session ID: %s\n\n%s", sessionID, output)
	} else if resume {
		output = fmt.Sprintf("Resumed most recent session\n\n%s", output)
	} else {
		output = fmt.Sprintf("New Codex session started\n\n%s", output)
	}

	return mcp.NewToolResultText(output), nil
}

// runCodex executes the Codex CLI with the specified parameters
func (t *CodexTool) runCodex(ctx context.Context, logger *logrus.Logger, timeout time.Duration, prompt, overrideModel, sandbox string, fullAuto, yoloMode, resume bool, sessionID, profile string, config, images []string, cdDir string, skipGitRepoCheck bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command arguments - JSON flag must come before subcommands
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "exec", "--json")

	// Add session management subcommands
	if resume {
		cmdArgs = append(cmdArgs, "resume")
		if sessionID != "" {
			cmdArgs = append(cmdArgs, sessionID)
		} else {
			cmdArgs = append(cmdArgs, "--last")
		}
	}

	// Add model override if specified
	if overrideModel != "" {
		cmdArgs = append(cmdArgs, "-m", overrideModel)
	}

	// Add sandbox configuration
	if sandbox != "" {
		cmdArgs = append(cmdArgs, "-s", sandbox)
	}

	// Add security flags (order matters - yolo-mode overrides other settings)
	if yoloMode {
		cmdArgs = append(cmdArgs, "--dangerously-bypass-approvals-and-sandbox")
	} else if fullAuto {
		cmdArgs = append(cmdArgs, "--full-auto")
	}

	// Add configuration settings
	if profile != "" {
		cmdArgs = append(cmdArgs, "-p", profile)
	}
	for _, cfg := range config {
		cmdArgs = append(cmdArgs, "-c", cfg)
	}

	// Add file attachments
	for _, image := range images {
		cmdArgs = append(cmdArgs, "-i", image)
	}

	// Add working directory
	if cdDir != "" {
		cmdArgs = append(cmdArgs, "-C", cdDir)
	}

	// Add optional flags
	if skipGitRepoCheck {
		cmdArgs = append(cmdArgs, "--skip-git-repo-check")
	}

	// Add prompt as the final argument
	cmdArgs = append(cmdArgs, prompt)

	logger.Debugf("Running Codex with args: %v", cmdArgs)

	cmd := exec.CommandContext(ctx, "codex", cmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Capture output from both stdout and stderr
	output := stdout.String()
	errorOutput := stderr.String()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return output, context.DeadlineExceeded
		}

		// Enhanced error handling for common scenarios
		if strings.Contains(errorOutput, "command not found") || strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("codex CLI not found. Please install Codex CLI and ensure it's available in your PATH")
		}

		if strings.Contains(errorOutput, "not authenticated") || strings.Contains(errorOutput, "authentication") {
			return "", fmt.Errorf("codex authentication failed. Please ensure you are authenticated and Codex is configured properly. Error: %s", errorOutput)
		}

		// Include stderr in error for debugging
		if errorOutput != "" {
			return "", fmt.Errorf("codex CLI error: %s. Stderr: %s", err.Error(), errorOutput)
		}

		return "", fmt.Errorf("codex CLI error: %w", err)
	}

	// Process JSONL output from Codex
	return t.ProcessJSONLOutput(output, logger)
}

// ProcessJSONLOutput parses and formats JSONL output from Codex
// Exported for testing
func (t *CodexTool) ProcessJSONLOutput(output string, logger *logrus.Logger) (string, error) {
	if strings.TrimSpace(output) == "" {
		return "", nil
	}

	// Split into lines and process each JSON line
	lines := strings.Split(output, "\n")
	var processedOutput strings.Builder
	var hasAgentMessage bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to parse as JSON and extract agent messages
		var jsonData map[string]any
		if err := json.Unmarshal([]byte(line), &jsonData); err != nil {
			// Skip malformed JSON lines
			logger.Debugf("Skipping malformed JSON line: %s", err)
			continue
		}

		// Extract message if it's an agent_message type
		if msg, ok := jsonData["msg"].(map[string]any); ok {
			if msgType, ok := msg["type"].(string); ok && msgType == "agent_message" {
				if message, ok := msg["message"].(string); ok {
					processedOutput.WriteString(message)
					processedOutput.WriteString("\n")
					hasAgentMessage = true
				}
			}
		}
	}

	result := strings.TrimSpace(processedOutput.String())

	// If no agent messages were found, fall back to raw output
	if !hasAgentMessage {
		logger.Debug("No agent messages found in JSONL, falling back to raw output")
		return output, nil
	}

	return result, nil
}

// GetTimeout returns the configured timeout or default
func (t *CodexTool) GetTimeout() int {
	if timeoutStr := os.Getenv(AgentTimeoutEnvVar); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			return timeout
		}
	}
	return DefaultTimeout
}

// GetMaxResponseSize returns the configured maximum response size
func (t *CodexTool) GetMaxResponseSize() int {
	if sizeStr := os.Getenv(AgentMaxResponseSizeEnvVar); sizeStr != "" {
		if size, err := strconv.Atoi(sizeStr); err == nil && size > 0 {
			return size
		}
	}
	return DefaultMaxResponseSize
}

// ApplyResponseSizeLimit truncates the response if it exceeds the configured size limit
func (t *CodexTool) ApplyResponseSizeLimit(output string, logger *logrus.Logger) string {
	// Handle empty output
	if output == "" {
		return ""
	}

	maxSize := t.GetMaxResponseSize()

	// Check if output exceeds limit
	if len(output) <= maxSize {
		return output
	}

	// Find a good truncation point - try to truncate at a line boundary
	truncateAt := maxSize

	// Look for line break within last 100 characters of the limit
	searchStart := max(maxSize-100, 0)

	// Find the last newline in the search window
	searchWindow := output[searchStart:maxSize]
	if lastNewline := strings.LastIndexAny(searchWindow, "\n\r"); lastNewline != -1 {
		truncateAt = searchStart + lastNewline
	} else if maxSize < len(output) {
		// If no newline found and we need to truncate, just use the maxSize
		truncateAt = maxSize
	}

	truncated := output[:truncateAt]
	originalSize := len(output)
	truncatedSize := len(truncated)

	// Format the size for display
	var limitDisplay string

	if maxSize >= 1024*1024 {
		limitDisplay = fmt.Sprintf("%.1fMB", float64(maxSize)/(1024*1024))
	} else if maxSize >= 1024 {
		limitDisplay = fmt.Sprintf("%.1fKB", float64(maxSize)/1024)
	} else {
		limitDisplay = fmt.Sprintf("%dB", maxSize)
	}

	// Build truncation message matching test expectations
	message := fmt.Sprintf("\n\n[RESPONSE TRUNCATED: Output exceeded %s limit. Original: %d, Truncated: %d]",
		limitDisplay, originalSize, truncatedSize)

	logger.Warnf("Codex agent response truncated from %d bytes to %d bytes due to size limit", originalSize, truncatedSize)

	return truncated + message
}

// ProvideExtendedInfo provides detailed usage information for the Codex agent tool
func (t *CodexTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Ask Codex to analyze code for potential improvements",
				Arguments: map[string]any{
					"prompt": "Please review this function for efficiency and suggest optimizations: @myfile.py",
				},
				ExpectedResult: "Codex analyzes the referenced file and provides specific suggestions for optimization and best practices",
			},
			{
				Description: "Use Codex with sandbox restrictions for safe code generation",
				Arguments: map[string]any{
					"prompt":  "Generate a Python function to validate email addresses",
					"sandbox": "read-only",
				},
				ExpectedResult: "Codex generates code while restricted to read-only operations for safety",
			},
			{
				Description: "Continue a previous Codex session",
				Arguments: map[string]any{
					"prompt": "Thanks for the previous suggestions. Can you now help me implement the caching layer you mentioned?",
					"resume": true,
				},
				ExpectedResult: "Codex continues the previous conversation context and provides implementation guidance",
			},
			{
				Description: "Use full-auto mode for streamlined development",
				Arguments: map[string]any{
					"prompt":    "Create a REST API endpoint for user authentication with proper error handling",
					"full-auto": true,
				},
				ExpectedResult: "Codex automatically handles the implementation with workspace-write sandbox permissions",
			},
			{
				Description: "Specify a custom model and working directory",
				Arguments: map[string]any{
					"prompt":         "Analyze the project structure and suggest architectural improvements",
					"override-model": "claude-3.5-sonnet",
					"cd":             "/path/to/project",
				},
				ExpectedResult: "Codex uses the specified model and analyzes the project from the given directory",
			},
		},
		CommonPatterns: []string{
			"Use 'resume: true' to continue conversations in the same context",
			"Set 'sandbox: read-only' for safe code analysis without execution",
			"Use 'full-auto: true' for streamlined development with automatic permissions",
			"Specify 'override-model' to use different Claude models for specific tasks",
			"Use '@file' syntax in prompts to reference specific files",
			"Set 'cd' parameter to change working directory for project analysis",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Codex CLI not found error",
				Solution: "Install Codex CLI and ensure it's available in your PATH. The tool does not verify installation beforehand.",
			},
			{
				Problem:  "Authentication errors",
				Solution: "Ensure you are authenticated and Codex is properly configured. Check your Codex configuration.",
			},
			{
				Problem:  "Tool is not enabled",
				Solution: "Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'codex-agent'",
			},
			{
				Problem:  "Permission denied errors",
				Solution: "Check your sandbox settings. Use 'sandbox: workspace-write' or 'full-auto: true' for write operations.",
			},
			{
				Problem:  "Session not found errors",
				Solution: "Verify the session-id exists or use 'resume: true' without session-id to resume the last session.",
			},
		},
		ParameterDetails: map[string]string{
			"prompt":              "Required. The instruction for Codex. Supports @file, @directory/ syntax for file references.",
			"override-model":      "Optional. Override the default model. No default provided to respect user configuration.",
			"sandbox":             "Optional. Security policy: read-only, workspace-write, danger-full-access. Controls execution permissions.",
			"full-auto":           "Optional. Enable automatic execution with workspace-write sandbox. Convenience flag for development.",
			"yolo-mode":           "Optional. DANGER: Bypasses all security restrictions. Use with extreme caution.",
			"resume":              "Optional. Continue the most recent session. Uses --last flag when true.",
			"session-id":          "Optional. Specify session identifier for resuming specific sessions.",
			"profile":             "Optional. Configuration profile from config.toml to use.",
			"config":              "Optional. Array of configuration overrides in key=value format. Supports nested paths.",
			"images":              "Optional. Array of image files to attach to the prompt.",
			"cd":                  "Optional. Working directory for execution. Must exist.",
			"skip-git-repo-check": "Optional. Skip git repository validation checks.",
		},
		WhenToUse:    "Use Codex agent when you need AI-powered code analysis, generation, or assistance with full Codex CLI capabilities including sandbox policies and session management.",
		WhenNotToUse: "Avoid for simple text generation tasks where other tools suffice. Be cautious with yolo-mode in production environments.",
	}
}

// validateDirectory checks if the provided path exists and is a directory
func validateDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", path)
		}
		return fmt.Errorf("cannot access directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	return nil
}

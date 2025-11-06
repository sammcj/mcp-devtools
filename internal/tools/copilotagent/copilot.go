package copilotagent

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

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// CopilotTool implements the tools.Tool interface for GitHub Copilot CLI
type CopilotTool struct{}

const (
	DefaultTimeout             = 180             // 3 minutes default timeout
	DefaultMaxResponseSize     = 2 * 1024 * 1024 // 2MB default limit
	AgentMaxResponseSizeEnvVar = "AGENT_MAX_RESPONSE_SIZE"
	AgentTimeoutEnvVar         = "AGENT_TIMEOUT"
)

// init registers the tool with the registry
func init() {
	registry.Register(&CopilotTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *CopilotTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"copilot-agent",
		mcp.WithDescription("Provides access to GitHub Copilot CLI through MCP. Enables AI agents to leverage Copilot's capabilities for code analysis, generation, and assistance."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("A clear, concise prompt to send to Copilot CLI to instruct the AI Agent to perform a specific task."),
		),
		mcp.WithString("override-model",
			mcp.Description("Model to override the default. No default model is provided, allowing Copilot to use user's configured default."),
		),
		mcp.WithBoolean("resume",
			mcp.Description("Continue the most recent session using --continue flag."),
			mcp.DefaultBool(false),
		),
		mcp.WithString("session-id",
			mcp.Description("Specify a session identifier for resuming specific sessions."),
		),
		tools.AddConditionalPermissionsParameter("yolo-mode",
			"Trust all tools without confirmation (maps to --allow-all-tools)."),
		mcp.WithArray("allow-tool",
			mcp.Description("Specific tool permissions to grant (maps to --allow-tool flags)."),
			mcp.WithStringItems(),
		),
		mcp.WithArray("deny-tool",
			mcp.Description("Specific tool permissions to deny (maps to --deny-tool flags)."),
			mcp.WithStringItems(),
		),
		mcp.WithArray("include-directories",
			mcp.Description("Additional directories to grant access to (maps to --add-dir flags)."),
			mcp.WithStringItems(),
		),
		mcp.WithArray("disable-mcp-server",
			mcp.Description("MCP servers to disable during execution (maps to --disable-mcp-server flags)."),
			mcp.WithStringItems(),
		),
		// Destructive tool annotations
		mcp.WithReadOnlyHintAnnotation(false),   // Agent can execute arbitrary commands via Copilot
		mcp.WithDestructiveHintAnnotation(true), // Can perform destructive operations via external agent
		mcp.WithIdempotentHintAnnotation(false), // Agent operations are not idempotent
		mcp.WithOpenWorldHintAnnotation(true),   // Agent can interact with external systems
	)
}

// Execute executes the tool's logic by calling the Copilot CLI
func (t *CopilotTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Check if copilot-agent tool is enabled (disabled by default for security)
	if !tools.IsToolEnabled("copilot-agent") {
		return nil, fmt.Errorf("copilot agent tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'copilot-agent'")
	}

	logger.Info("Executing Copilot tool")

	// Get timeout from environment or use default
	timeout := t.GetTimeout()

	// Validate required prompt parameter
	prompt, ok := args["prompt"].(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("prompt is a required parameter and cannot be empty")
	}

	output, err := t.runCopilot(ctx, logger, time.Duration(timeout)*time.Second, prompt, args)
	if err != nil {
		if err == context.DeadlineExceeded {
			timeoutMsg := fmt.Sprintf("\n\nThe Copilot Agent hit the configured timeout of %d seconds, output may be truncated!", timeout)
			return mcp.NewToolResultText(output + timeoutMsg), nil
		}
		logger.WithError(err).Error("Copilot tool execution failed")
		return nil, fmt.Errorf("copilot command failed: %w", err)
	}

	// Filter output to remove Copilot-specific metadata
	output = t.FilterOutput(output)

	// Apply response size limits
	output = t.ApplyResponseSizeLimit(output, logger)

	return mcp.NewToolResultText(output), nil
}

// runCopilot executes the Copilot CLI with the specified parameters
func (t *CopilotTool) runCopilot(ctx context.Context, logger *logrus.Logger, timeout time.Duration, prompt string, args map[string]any) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command arguments inline (matching Q Developer pattern)
	cmdArgs := []string{"--prompt", prompt, "--no-color"}

	// Model selection
	if model, ok := args["override-model"].(string); ok && model != "" {
		cmdArgs = append(cmdArgs, "--model", model)
	}

	// Session management - session-id takes priority
	if sessionID, ok := args["session-id"].(string); ok && sessionID != "" {
		cmdArgs = append(cmdArgs, "--resume", sessionID)
	} else if resume, ok := args["resume"].(bool); ok && resume {
		cmdArgs = append(cmdArgs, "--continue")
	}

	// Permission management
	yoloModeParam, _ := args["yolo-mode"].(bool)
	yoloMode := tools.GetEffectivePermissionsValue(yoloModeParam)
	if yoloMode {
		cmdArgs = append(cmdArgs, "--allow-all-tools")
	}

	// Array parameters - MCP provides as []any
	if allowTools, ok := args["allow-tool"].([]any); ok {
		for _, tool := range allowTools {
			if t, ok := tool.(string); ok {
				cmdArgs = append(cmdArgs, "--allow-tool", t)
			}
		}
	}

	// Similar pattern for deny-tool
	if denyTools, ok := args["deny-tool"].([]any); ok {
		for _, tool := range denyTools {
			if t, ok := tool.(string); ok {
				cmdArgs = append(cmdArgs, "--deny-tool", t)
			}
		}
	}

	// Include directories (no validation per requirements)
	if includeDirs, ok := args["include-directories"].([]any); ok {
		for _, dir := range includeDirs {
			if d, ok := dir.(string); ok {
				cmdArgs = append(cmdArgs, "--add-dir", d)
			}
		}
	}

	// Disable MCP servers
	if servers, ok := args["disable-mcp-server"].([]any); ok {
		for _, server := range servers {
			if s, ok := server.(string); ok {
				cmdArgs = append(cmdArgs, "--disable-mcp-server", s)
			}
		}
	}

	logger.Debugf("Running Copilot with args: %v", cmdArgs)

	cmd := exec.CommandContext(ctx, "copilot", cmdArgs...)

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
			return "", fmt.Errorf("copilot CLI not found. Please install Copilot CLI and ensure it's available in your PATH")
		}

		if strings.Contains(errorOutput, "not authenticated") || strings.Contains(errorOutput, "authentication") {
			return "", fmt.Errorf("copilot authentication failed. Please ensure you are authenticated. Error: %s", errorOutput)
		}

		// Include stderr in error for debugging
		if errorOutput != "" {
			return "", fmt.Errorf("copilot CLI error: %s. Stderr: %s", err.Error(), errorOutput)
		}

		return "", fmt.Errorf("copilot CLI error: %w", err)
	}

	return output, nil
}

// FilterOutput removes Copilot-specific metadata from output
func (t *CopilotTool) FilterOutput(output string) string {
	lines := strings.Split(output, "\n")

	// Find the index of the last progress indicator
	lastProgressIdx := -1
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check if line starts with progress indicator (using rune to handle Unicode)
		if len(trimmedLine) > 0 {
			runes := []rune(trimmedLine)
			firstChar := runes[0]
			if firstChar == '●' || firstChar == '✓' || firstChar == '✗' || firstChar == '↪' {
				lastProgressIdx = i
			}
		}
	}

	// Extract content after last progress indicator
	var contentLines []string
	startIdx := lastProgressIdx + 1
	if lastProgressIdx == -1 {
		startIdx = 0 // No progress indicators found, use all content
	}

	// If last progress indicator line has content after the indicator, extract it
	if lastProgressIdx >= 0 && lastProgressIdx < len(lines) {
		progressLine := strings.TrimSpace(lines[lastProgressIdx])
		// Remove the progress indicator character and get remaining content
		if len(progressLine) > 0 {
			runes := []rune(progressLine)
			if len(runes) > 1 {
				afterIndicator := strings.TrimSpace(string(runes[1:]))
				if afterIndicator != "" {
					contentLines = append(contentLines, afterIndicator)
				}
			}
		}
	}

	// Extract remaining content from lines after the progress indicator
	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)

		// Stop at usage statistics section
		if strings.HasPrefix(trimmedLine, "Total usage est") {
			break
		}

		// Skip command execution lines
		if strings.HasPrefix(trimmedLine, "$") {
			continue
		}

		contentLines = append(contentLines, line)
	}

	// Join and clean up
	result := strings.Join(contentLines, "\n")
	result = strings.TrimSpace(result)

	// Collapse multiple consecutive empty lines to single
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return result
}

// GetTimeout returns the configured timeout or default
func (t *CopilotTool) GetTimeout() int {
	if timeoutStr := os.Getenv(AgentTimeoutEnvVar); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			return timeout
		}
	}
	return DefaultTimeout
}

// GetMaxResponseSize returns the configured maximum response size
func (t *CopilotTool) GetMaxResponseSize() int {
	if sizeStr := os.Getenv(AgentMaxResponseSizeEnvVar); sizeStr != "" {
		if size, err := strconv.Atoi(sizeStr); err == nil && size > 0 {
			return size
		}
	}
	return DefaultMaxResponseSize
}

// ApplyResponseSizeLimit truncates the response if it exceeds the configured size limit
func (t *CopilotTool) ApplyResponseSizeLimit(output string, logger *logrus.Logger) string {
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

	logger.Warnf("Copilot agent response truncated from %d bytes to %d bytes due to size limit", originalSize, truncatedSize)

	return truncated + message
}

// ProvideExtendedInfo provides detailed usage information for the Copilot agent tool
func (t *CopilotTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Get Copilot to analyze code for potential improvements",
				Arguments: map[string]any{
					"prompt": "Please review this function for efficiency and suggest optimisations: [paste your code here]",
				},
				ExpectedResult: "Copilot analyses the code and provides specific suggestions for optimisation and best practices",
			},
			{
				Description: "Ask Copilot to help debug an issue",
				Arguments: map[string]any{
					"prompt": "I'm getting a TypeError when calling this API. Can you help identify the issue?",
				},
				ExpectedResult: "Copilot provides debugging assistance",
			},
			{
				Description: "Continue a previous conversation",
				Arguments: map[string]any{
					"prompt": "Thanks for the previous suggestions. Can you now help me implement the caching layer you mentioned?",
					"resume": true,
				},
				ExpectedResult: "Copilot continues the previous conversation context and provides implementation guidance",
			},
			{
				Description: "Use a specific model for code generation",
				Arguments: map[string]any{
					"prompt":         "Generate a REST API handler for user authentication",
					"override-model": "gpt-5",
				},
				ExpectedResult: "Copilot uses the specified model to generate code",
			},
			{
				Description: "Allow Copilot to execute specific tools",
				Arguments: map[string]any{
					"prompt":     "Analyse the codebase and run tests to verify functionality",
					"allow-tool": []string{"shell(npm test)", "shell(go test)"},
				},
				ExpectedResult: "Copilot can execute the specified test commands during analysis",
			},
			{
				Description: "Resume a specific session",
				Arguments: map[string]any{
					"prompt":     "Continue working on the authentication module",
					"session-id": "abc123def456",
				},
				ExpectedResult: "Copilot resumes the specific session and continues the previous context",
			},
		},
		CommonPatterns: []string{
			"Use 'resume: true' to continue the most recent conversation",
			"Use 'session-id' to resume a specific session by ID",
			"Set 'yolo-mode: true' to allow Copilot to execute all tools without confirmation",
			"Use 'allow-tool' to grant specific tool permissions (e.g., 'shell(git:*)')",
			"Specify 'override-model' to use a different model for specific tasks",
			"Use 'include-directories' to grant access to additional directories outside the project",
			"Use 'disable-mcp-server' to prevent conflicts with specific MCP servers",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Copilot CLI not found error",
				Solution: "Install Copilot CLI and ensure it's available in your PATH. The tool does not verify installation beforehand.",
			},
			{
				Problem:  "Authentication errors",
				Solution: "Ensure you are authenticated with GitHub. Run 'gh auth login' to authenticate.",
			},
			{
				Problem:  "Tool is not enabled",
				Solution: "Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'copilot-agent'",
			},
			{
				Problem:  "Response truncated messages",
				Solution: "Increase AGENT_MAX_RESPONSE_SIZE environment variable or break down large requests into smaller parts",
			},
			{
				Problem:  "Session not found",
				Solution: "Verify the session ID is correct. Use 'resume: true' to continue the most recent session instead.",
			},
		},
		ParameterDetails: map[string]string{
			"prompt":              "Required instruction for Copilot. Cannot be empty.",
			"override-model":      "Choose specific model - no validation, passed directly to Copilot",
			"resume":              "Continue most recent session (uses --continue flag)",
			"session-id":          "Resume specific session by ID (uses --resume flag, takes priority over 'resume')",
			"yolo-mode":           "Trust all tools automatically (maps to --allow-all-tools, security consideration - use carefully)",
			"allow-tool":          "Array of tool permission patterns to allow (e.g., 'shell(git:*)')",
			"deny-tool":           "Array of tool permission patterns to deny",
			"include-directories": "Array of additional directory paths to grant access to (no path validation)",
			"disable-mcp-server":  "Array of MCP server names to disable during execution",
		},
		WhenToUse:    "Use Copilot agent for code assistance, generation, and analysis. Copilot has broad programming knowledge and can help with various languages and frameworks.",
		WhenNotToUse: "Avoid for tasks requiring specific agent capabilities (AWS knowledge - use Q Developer, Google services - use Gemini, etc.). Note that Copilot requires GitHub authentication.",
	}
}

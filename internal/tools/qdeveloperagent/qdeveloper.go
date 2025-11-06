package qdeveloperagent

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

// QDeveloperTool implements the tools.Tool interface for AWS Q Developer CLI
type QDeveloperTool struct{}

const (
	DefaultTimeout             = 300             // 5 minutes default timeout
	DefaultMaxResponseSize     = 2 * 1024 * 1024 // 2MB default limit
	AgentMaxResponseSizeEnvVar = "AGENT_MAX_RESPONSE_SIZE"
	AgentTimeoutEnvVar         = "AGENT_TIMEOUT"
)

// init registers the tool with the registry
func init() {
	registry.Register(&QDeveloperTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *QDeveloperTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"q-developer-agent",
		mcp.WithDescription("Provides access to AWS Q Developer CLI through MCP. Enables AI agents to leverage Q Developer's capabilities for code analysis, generation, and assistance."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("A clear, concise prompt to send to Q Developer CLI to instruct the AI Agent to perform a specific task."),
		),
		mcp.WithBoolean("resume",
			mcp.Description("Continue the previous conversation from this directory."),
			mcp.DefaultBool(false),
		),
		mcp.WithString("agent",
			mcp.Description("Context profile to use for the conversation."),
		),
		mcp.WithString("override-model",
			mcp.Description("Model to use. Available models: claude-3.5-sonnet, claude-3.7-sonnet, claude-sonnet-4 (default)."),
		),
		tools.AddConditionalParameter("yolo-mode",
			"Trust all tools without confirmation (maps to --trust-all-tools)."),
		mcp.WithString("trust-tools",
			mcp.Description("Comma-separated list of specific tools to trust."),
		),
		mcp.WithBoolean("verbose",
			mcp.Description("Enable verbose logging for detailed output."),
			mcp.DefaultBool(false),
		),
		// Destructive tool annotations
		mcp.WithReadOnlyHintAnnotation(false),   // Agent can execute arbitrary commands via Q Developer
		mcp.WithDestructiveHintAnnotation(true), // Can perform destructive operations via external agent
		mcp.WithIdempotentHintAnnotation(false), // Agent operations are not idempotent
		mcp.WithOpenWorldHintAnnotation(true),   // Agent can interact with external systems
	)
}

// Execute executes the tool's logic by calling the Q Developer CLI
func (t *QDeveloperTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	logger.Info("Executing Q Developer tool")

	// Get timeout from environment or use default
	timeout := t.GetTimeout()

	// Validate required prompt parameter
	prompt, ok := args["prompt"].(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("prompt is a required parameter and cannot be empty")
	}

	// Parse optional parameters
	resume, _ := args["resume"].(bool)
	agent, _ := args["agent"].(string)
	overrideModel, _ := args["override-model"].(string)
	yoloModeParam, _ := args["yolo-mode"].(bool)
	yoloMode := tools.GetEffectivePermissionsValue(yoloModeParam)
	trustTools, _ := args["trust-tools"].(string)
	verbose, _ := args["verbose"].(bool)

	output, err := t.runQDeveloper(ctx, logger, time.Duration(timeout)*time.Second, prompt, resume, agent, overrideModel, yoloMode, trustTools, verbose)
	if err != nil {
		if err == context.DeadlineExceeded {
			timeoutMsg := fmt.Sprintf("\n\nThe Q Developer Agent hit the configured timeout of %d seconds, output may be truncated!", timeout)
			return mcp.NewToolResultText(output + timeoutMsg), nil
		}
		logger.WithError(err).Error("Q Developer tool execution failed")
		return nil, fmt.Errorf("q Developer command failed: %w", err)
	}

	// Apply response size limits
	output = t.ApplyResponseSizeLimit(output, logger)

	return mcp.NewToolResultText(output), nil
}

// runQDeveloper executes the Q Developer CLI with the specified parameters
func (t *QDeveloperTool) runQDeveloper(ctx context.Context, logger *logrus.Logger, timeout time.Duration, prompt string, resume bool, agent, overrideModel string, yoloMode bool, trustTools string, verbose bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command arguments
	cmdArgs := []string{"chat", "--no-interactive"}

	if resume {
		cmdArgs = append(cmdArgs, "--resume")
	}
	if agent != "" {
		cmdArgs = append(cmdArgs, "--agent", agent)
	}
	if overrideModel != "" {
		cmdArgs = append(cmdArgs, "--model", overrideModel)
	}
	if yoloMode {
		cmdArgs = append(cmdArgs, "--trust-all-tools")
	}
	if trustTools != "" {
		cmdArgs = append(cmdArgs, "--trust-tools", trustTools)
	}
	if verbose {
		cmdArgs = append(cmdArgs, "--verbose")
	}

	// Add prompt as the final argument
	cmdArgs = append(cmdArgs, prompt)

	logger.Debugf("Running Q Developer with args: %v", cmdArgs)

	cmd := exec.CommandContext(ctx, "q", cmdArgs...)

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
			return "", fmt.Errorf("q Developer CLI not found. Please install Q Developer CLI and ensure it's available in your PATH")
		}

		if strings.Contains(errorOutput, "not authenticated") || strings.Contains(errorOutput, "authentication") {
			return "", fmt.Errorf("q Developer authentication failed. Please ensure you are authenticated with AWS and Q Developer is configured properly. Error: %s", errorOutput)
		}

		// Include stderr in error for debugging
		if errorOutput != "" {
			return "", fmt.Errorf("q Developer CLI error: %s. Stderr: %s", err.Error(), errorOutput)
		}

		return "", fmt.Errorf("q Developer CLI error: %w", err)
	}

	return output, nil
}

// GetTimeout returns the configured timeout or default
func (t *QDeveloperTool) GetTimeout() int {
	if timeoutStr := os.Getenv(AgentTimeoutEnvVar); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			return timeout
		}
	}
	return DefaultTimeout
}

// GetMaxResponseSize returns the configured maximum response size
func (t *QDeveloperTool) GetMaxResponseSize() int {
	if sizeStr := os.Getenv(AgentMaxResponseSizeEnvVar); sizeStr != "" {
		if size, err := strconv.Atoi(sizeStr); err == nil && size > 0 {
			return size
		}
	}
	return DefaultMaxResponseSize
}

// ApplyResponseSizeLimit truncates the response if it exceeds the configured size limit
func (t *QDeveloperTool) ApplyResponseSizeLimit(output string, logger *logrus.Logger) string {
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

	logger.Warnf("Q Developer agent response truncated from %d bytes to %d bytes due to size limit", originalSize, truncatedSize)

	return truncated + message
}

// ProvideExtendedInfo provides detailed usage information for the Q Developer agent tool
func (t *QDeveloperTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Get Q Developer to analyse code for potential improvements",
				Arguments: map[string]any{
					"prompt": "Please review this function for efficiency and suggest optimisations: [paste your code here]",
				},
				ExpectedResult: "Q Developer analyses the code and provides specific suggestions for optimisation and best practices",
			},
			{
				Description: "Ask Q Developer to help debug an issue",
				Arguments: map[string]any{
					"prompt":  "I'm getting a TypeError when calling this API. Can you help identify the issue?",
					"verbose": true,
				},
				ExpectedResult: "Q Developer provides debugging assistance with detailed logging enabled",
			},
			{
				Description: "Continue a previous conversation in the same directory",
				Arguments: map[string]any{
					"prompt": "Thanks for the previous suggestions. Can you now help me implement the caching layer you mentioned?",
					"resume": true,
				},
				ExpectedResult: "Q Developer continues the previous conversation context and provides implementation guidance",
			},
		},
		CommonPatterns: []string{
			"Use 'resume: true' to continue conversations in the same working directory",
			"Set 'yolo-mode: true' to allow Q Developer to execute tools without confirmation",
			"Use 'verbose: true' for detailed logging when troubleshooting issues",
			"Specify 'override-model' to use a different Claude model for specific tasks",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Q Developer CLI not found error",
				Solution: "Install Q Developer CLI and ensure it's available in your PATH.",
			},
			{
				Problem:  "Authentication errors",
				Solution: "Ensure you are authenticated with AWS and Q Developer is properly configured. Run 'q auth' to check authentication status.",
			},
			{
				Problem:  "Tool is not enabled",
				Solution: "Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'q-developer-agent'",
			},
			{
				Problem:  "Response truncated messages",
				Solution: "Increase AGENT_MAX_RESPONSE_SIZE environment variable or break down large requests into smaller parts",
			},
		},
		ParameterDetails: map[string]string{
			"prompt":         "Required instruction for Q Developer. Cannot be empty.",
			"resume":         "Continue previous conversation from current directory (directory-based sessions)",
			"agent":          "Context profile to use - see Q Developer documentation for available agents",
			"override-model": "Choose specific Claude model, e.g.: claude-4.5-sonnet, claude-4.5-haiku",
			"yolo-mode":      "Trust all tools automatically (security consideration - use carefully)",
			"trust-tools":    "Comma-separated list of specific tools to trust",
			"verbose":        "Enable detailed logging for troubleshooting",
		},
		WhenToUse:    "Use Q Developer agent for AWS-specific code assistance, cloud architecture guidance, and when you need Q Developer's specialised knowledge of AWS services and best practices.",
		WhenNotToUse: "Avoid for non-AWS related tasks where other agents might be more appropriate. Q Developer does not support @ syntax for file references like other agents.",
	}
}

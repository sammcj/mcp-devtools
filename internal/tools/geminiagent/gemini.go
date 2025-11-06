package geminiagent

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

// GeminiTool implements the tools.Tool interface for the gemini CLI
type GeminiTool struct{}

const (
	defaultModel               = "gemini-2.5-pro"
	flashModel                 = "gemini-2.5-flash"
	DefaultTimeout             = 300             // 5 minutes default timeout
	DefaultMaxResponseSize     = 2 * 1024 * 1024 // 2MB default limit
	AgentTimeoutEnvVar         = "AGENT_TIMEOUT"
	AgentMaxResponseSizeEnvVar = "AGENT_MAX_RESPONSE_SIZE"
)

// init registers the tool with the registry
func init() {
	registry.Register(&GeminiTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *GeminiTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"gemini-agent",
		mcp.WithDescription("Provides access to Google's Gemini CLI for AI capabilities. You can call out to this tool to treat Gemini as a sub-agent for tasks like reviewing completed implementations and for help with troubleshooting when stuck on stubborn problems."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("A clear, concise prompt to send to Gemini CLI to instruct the AI Agent to perform a specific task. Can include @file, @directory/, @./ references for context."),
		),
		mcp.WithString("override-model",
			mcp.Description(fmt.Sprintf("Force Gemini to use a different model. Default: %s.", defaultModel)),
		),
		mcp.WithBoolean("sandbox",
			mcp.Description("Run the command in the Gemini sandbox (Default: False)"),
			mcp.DefaultBool(false),
		),
		tools.AddConditionalParameter("yolo-mode",
			"Allow Gemini to make changes and run commands without confirmation. Only use if you want Gemini to make changes. Defaults to read-only mode."),
		mcp.WithBoolean("include-all-files",
			mcp.Description("Recursively includes all files within the current directory as context for the prompt."),
			mcp.DefaultBool(false),
		),
		// Destructive tool annotations
		mcp.WithReadOnlyHintAnnotation(false),   // Agent can execute arbitrary commands via Gemini
		mcp.WithDestructiveHintAnnotation(true), // Can perform destructive operations via external agent
		mcp.WithIdempotentHintAnnotation(false), // Agent operations are not idempotent
		mcp.WithOpenWorldHintAnnotation(true),   // Agent can interact with external systems
	)
}

// Execute executes the tool's logic by calling the gemini CLI
func (t *GeminiTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Check if gemini-agent tool is enabled
	if !tools.IsToolEnabled("gemini-agent") {
		return nil, fmt.Errorf("gemini agent tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'gemini-agent'")
	}

	logger.Info("Executing gemini tool")

	// Get timeout from environment or use default
	timeout := t.GetTimeout()

	prompt, ok := args["prompt"].(string)
	if !ok || prompt == "" {
		return nil, fmt.Errorf("prompt is a required parameter")
	}

	model, _ := args["override-model"].(string)
	if model == "" {
		model = defaultModel
	}

	sandbox, _ := args["sandbox"].(bool)
	yoloModeParam, _ := args["yolo-mode"].(bool)
	yoloMode := tools.GetEffectivePermissionsValue(yoloModeParam)
	includeAllFiles, _ := args["include-all-files"].(bool)

	// Initial attempt
	output, err := t.runGemini(ctx, logger, time.Duration(timeout)*time.Second, prompt, model, sandbox, yoloMode, includeAllFiles)
	if err != nil {
		// Check for quota error and attempt fallback to flash model
		if strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") && model != flashModel {
			logger.Warnf("Gemini API quota exceeded for model %s, falling back to %s", model, flashModel)
			output, err = t.runGemini(ctx, logger, time.Duration(timeout)*time.Second, prompt, flashModel, sandbox, yoloMode, includeAllFiles)
		} else if err == context.DeadlineExceeded {
			timeoutMsg := fmt.Sprintf("\n\nThe Gemini Agent hit the configured timeout of %d seconds, output may be truncated!", timeout)
			return mcp.NewToolResultText(output + timeoutMsg), nil
		}
	}

	if err != nil {
		logger.WithError(err).Error("Gemini tool execution failed")
		return nil, fmt.Errorf("gemini command failed: %w", err)
	}

	// Apply response size limits
	output = t.ApplyResponseSizeLimit(output, logger)

	return mcp.NewToolResultText(output), nil
}

func (t *GeminiTool) runGemini(ctx context.Context, logger *logrus.Logger, timeout time.Duration, prompt, model string, sandbox, yoloMode, includeAllFiles bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmdArgs := []string{}
	if model != "" {
		cmdArgs = append(cmdArgs, "-m", model)
	}
	if sandbox {
		cmdArgs = append(cmdArgs, "-s")
	}
	if yoloMode {
		cmdArgs = append(cmdArgs, "--yolo")
	}
	if includeAllFiles {
		cmdArgs = append(cmdArgs, "--all-files")
	}
	cmdArgs = append(cmdArgs, "-p", prompt)

	logger.Infof("Running command: gemini %v", cmdArgs)

	cmd := exec.CommandContext(ctx, "gemini", cmdArgs...)
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

	// Strip unwanted startup messages
	output := outb.String()
	lines := strings.Split(output, "\n")
	var cleanLines []string
	linesToSkip := 5
	for i, line := range lines {
		if i < linesToSkip && (strings.Contains(line, "Data collection is disabled.") || strings.Contains(line, "Loaded cached credentials.")) {
			continue
		}
		cleanLines = append(cleanLines, line)
	}

	return strings.Join(cleanLines, "\n"), nil
}

// GetTimeout returns the configured timeout or default
func (t *GeminiTool) GetTimeout() int {
	if timeoutStr := os.Getenv(AgentTimeoutEnvVar); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			return timeout
		}
	}
	return DefaultTimeout
}

// GetMaxResponseSize returns the configured maximum response size
func (t *GeminiTool) GetMaxResponseSize() int {
	if sizeStr := os.Getenv(AgentMaxResponseSizeEnvVar); sizeStr != "" {
		if size, err := strconv.Atoi(sizeStr); err == nil && size > 0 {
			return size
		}
	}
	return DefaultMaxResponseSize
}

// ApplyResponseSizeLimit truncates the response if it exceeds the configured size limit
func (t *GeminiTool) ApplyResponseSizeLimit(output string, logger *logrus.Logger) string {
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

	logger.Warnf("Gemini agent response truncated from %d bytes to %d bytes due to size limit", len(output), len(truncated))

	return truncated + message
}

// ProvideExtendedInfo provides detailed usage information for the gemini agent tool
func (t *GeminiTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Get Gemini to analyze code with full project context",
				Arguments: map[string]any{
					"prompt":            "Please analyze the performance bottlenecks in this web application and suggest optimizations.",
					"include-all-files": true,
					"override-model":    "gemini-2.5-pro",
				},
				ExpectedResult: "Gemini analyzes all project files for performance issues and provides specific optimization recommendations with code examples",
			},
			{
				Description: "Ask Gemini to help debug with sandbox mode",
				Arguments: map[string]any{
					"prompt":  "I'm getting intermittent test failures in my Jest test suite. Can you help identify the root cause and fix the timing issues? @tests/",
					"sandbox": true,
				},
				ExpectedResult: "Gemini runs in isolated sandbox environment to safely analyze test failures and provide debugging assistance",
			},
			{
				Description: "Let Gemini make actual changes to fix issues",
				Arguments: map[string]any{
					"prompt":         "There are several TypeScript errors in @src/components/ that need to be fixed. Please resolve all type issues and update the interfaces accordingly.",
					"yolo-mode":      true,
					"override-model": "gemini-2.5-pro",
				},
				ExpectedResult: "Gemini analyzes TypeScript errors and makes actual file changes to fix type issues throughout the components directory",
			},
			{
				Description: "Quick analysis with flash model for speed",
				Arguments: map[string]any{
					"prompt":         "Can you quickly review this API endpoint @src/api/users.js and check if it follows REST conventions?",
					"override-model": "gemini-2.5-flash",
				},
				ExpectedResult: "Fast analysis using Gemini Flash model to provide quick feedback on API design and REST compliance",
			},
			{
				Description: "Comprehensive code review with context",
				Arguments: map[string]any{
					"prompt": "Please conduct a thorough security review of the authentication system in @src/auth/ and identify any vulnerabilities or improvements needed.",
				},
				ExpectedResult: "Detailed security analysis of authentication code with specific vulnerability findings and remediation suggestions",
			},
		},
		CommonPatterns: []string{
			"Use @file or @directory/ syntax to provide specific context to Gemini",
			"Use include-all-files for comprehensive project analysis when context is important",
			"Use yolo-mode when you want Gemini to actually fix issues, not just suggest fixes",
			"Use sandbox mode for safe exploration of potentially risky operations",
			"Use gemini-2.5-flash model for quick responses, gemini-2.5-pro for complex analysis",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Tool not available/gemini command not found",
				Solution: "The gemini agent requires Gemini CLI to be installed and ENABLE_ADDITIONAL_TOOLS environment variable to include 'gemini-agent'. Install Gemini CLI and set ENABLE_ADDITIONAL_TOOLS=gemini-agent.",
			},
			{
				Problem:  "RESOURCE_EXHAUSTED quota exceeded error",
				Solution: "Gemini API quota limits reached. The tool automatically falls back to gemini-2.5-flash when quota is exceeded. Wait for quota reset or upgrade your Google AI API plan.",
			},
			{
				Problem:  "Agent timeout after 3 minutes",
				Solution: "Complex analysis may need more time. Set AGENT_TIMEOUT environment variable to increase timeout (in seconds). Example: AGENT_TIMEOUT=600 for 10 minutes.",
			},
			{
				Problem:  "Response truncated due to size limit",
				Solution: "Large responses are truncated for memory safety. Increase AGENT_MAX_RESPONSE_SIZE environment variable (in bytes) or ask Gemini for more concise responses.",
			},
			{
				Problem:  "Permission denied errors in yolo mode",
				Solution: "Even in yolo-mode, Gemini respects file system permissions. Ensure the process has necessary read/write permissions to files and directories being modified.",
			},
			{
				Problem:  "Data collection disabled messages in output",
				Solution: "These are normal Gemini CLI startup messages that are automatically filtered out. They don't affect functionality but indicate Gemini's privacy settings.",
			},
		},
		ParameterDetails: map[string]string{
			"prompt":            "Clear, specific instruction to Gemini. Use @file or @directory/ syntax for context. Be detailed about what analysis, implementation, or assistance you need.",
			"override-model":    "Gemini model selection affects capability vs speed. 'gemini-2.5-pro' (default, most capable), 'gemini-2.5-flash' (faster, good for simple tasks). Tool auto-falls back to flash on quota limits.",
			"sandbox":           "Runs Gemini in isolated environment for safe exploration. Use when testing potentially risky operations or when you want isolated execution context.",
			"yolo-mode":         "Allows Gemini to make actual file changes and run commands. Use with caution - only when you want Gemini to implement fixes, not just suggest them.",
			"include-all-files": "Recursively includes all files in current directory as context. Powerful for comprehensive analysis but may hit token limits on large projects.",
		},
		WhenToUse:    "Use when you need Google's Gemini AI for code analysis, debugging assistance, comprehensive project reviews, or when you want a different AI perspective from Claude for complex problems beyond your capabilities or if directly asked to use Gemini by the user.",
		WhenNotToUse: "Don't use when you need real-time responses, for simple questions that don't require AI analysis, or when working with highly sensitive code that shouldn't be sent to Google's APIs.",
	}
}

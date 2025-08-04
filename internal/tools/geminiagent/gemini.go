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
	"github.com/sirupsen/logrus"
)

// GeminiTool implements the tools.Tool interface for the gemini CLI
type GeminiTool struct{}

const (
	defaultModel               = "gemini-2.5-pro"
	flashModel                 = "gemini-2.5-flash"
	DefaultMaxResponseSize     = 2 * 1024 * 1024 // 2MB default limit
	AgentMaxResponseSizeEnvVar = "AGENT_MAX_RESPONSE_SIZE"
)

// init registers the tool with the registry if enabled
func init() {
	enabledAgents := os.Getenv("ENABLE_AGENTS")
	if strings.Contains(enabledAgents, "gemini") {
		registry.Register(&GeminiTool{})
	}
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
		mcp.WithString("model",
			mcp.Description(fmt.Sprintf("Force Gemini to use a different model. Default: %s.", defaultModel)),
		),
		mcp.WithBoolean("sandbox",
			mcp.Description("Run the command in the Gemini sandbox (Default: False)"),
			mcp.DefaultBool(false),
		),
		mcp.WithBoolean("yolo-mode",
			mcp.Description("Allow Gemini to make changes and run commands without confirmation. Only use if you want Gemini to make changes. Defaults to read-only mode."),
			mcp.DefaultBool(false),
		),
		mcp.WithBoolean("include-all-files",
			mcp.Description("Recursively includes all files within the current directory as context for the prompt."),
			mcp.DefaultBool(false),
		),
	)
}

// Execute executes the tool's logic by calling the gemini CLI
func (t *GeminiTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Executing gemini tool")

	timeoutStr := os.Getenv("AGENT_TIMEOUT")
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil || timeout <= 0 {
		timeout = 180 // Default to 3 minutes
	}

	prompt, ok := args["prompt"].(string)
	if !ok || prompt == "" {
		return nil, fmt.Errorf("prompt is a required parameter")
	}

	model, _ := args["model"].(string)
	if model == "" {
		model = defaultModel
	}

	sandbox, _ := args["sandbox"].(bool)
	yoloMode, _ := args["yolo-mode"].(bool)
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

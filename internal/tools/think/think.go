package think

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// ThinkTool implements a simple thinking tool for structured reasoning
type ThinkTool struct{}

const (
	// DefaultMaxThoughtLength is the default maximum length for thought input advertised to agents
	DefaultMaxThoughtLength = 2000
	// ThinkMaxLengthEnvVar is the environment variable for configuring max thought length
	ThinkMaxLengthEnvVar = "THINK_MAX_LENGTH"
	// ThinkLengthSafetyBuffer is added to the configured max length when validating
	// AI agents are not precise at counting characters, so we allow some overage
	// whilst still telling them the lower limit to discourage overly long thoughts
	ThinkLengthSafetyBuffer = 500
)

// getMaxThoughtLength returns the configured maximum thought length
func getMaxThoughtLength() int {
	if envValue := os.Getenv(ThinkMaxLengthEnvVar); envValue != "" {
		if value, err := strconv.Atoi(envValue); err == nil && value > 0 {
			return value
		}
	}
	return DefaultMaxThoughtLength
}

// init registers the think tool
func init() {
	registry.Register(&ThinkTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *ThinkTool) Definition() mcp.Tool {
	maxLen := getMaxThoughtLength()
	return mcp.NewTool(
		"think",
		mcp.WithDescription(fmt.Sprintf(`A concise scratchpad for reasoning when you have a single complex question or decision. Does not retrieve information or modify anything - just records the thought.

Keep thoughts brief and focused: aim for 2-4 sentences (~50-150 words). State what you need to reason about, your conclusion or next step, and why. Do NOT include multi-step analyses, inline code blocks, or exhaustive breakdowns.

For multi-step reasoning, revision, or branching analysis, use sequential_thinking instead.

Maximum length: %d characters (~300 words). Exceeding this will be rejected.`, maxLen)),
		mcp.WithString("thought",
			mcp.Required(),
			mcp.MaxLength(maxLen),
			mcp.Description("A brief reasoning note: 2-4 sentences covering what you're considering and your conclusion. Not for lengthy analysis - use sequential_thinking for that."),
		),
		mcp.WithString("how_hard",
			mcp.Description("How hard to think about the problem. Options: 'hard' (default), 'harder', 'ultra'."),
			mcp.Enum("hard", "harder", "ultra"),
		),
		// Read-only annotations for internal thought processing tool
		mcp.WithReadOnlyHintAnnotation(true),     // Only processes thoughts internally, doesn't modify environment
		mcp.WithDestructiveHintAnnotation(false), // No destructive operations
		mcp.WithIdempotentHintAnnotation(true),   // Stateless: same input produces same output
		mcp.WithOpenWorldHintAnnotation(false),   // No external interactions, internal processing only
	)
}

// Execute executes the think tool
func (t *ThinkTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Parse and validate parameters
	request, err := t.parseRequest(args)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Create response with timestamp
	response := &ThinkResponse{
		Thought:   request.Thought,
		Timestamp: time.Now(),
	}

	return t.newToolResultText(request.HowHard, response.Thought)
}

// parseRequest parses and validates the tool arguments
func (t *ThinkTool) parseRequest(args map[string]any) (*ThinkRequest, error) {
	// Parse thought (required)
	thought, ok := args["thought"].(string)
	if !ok || thought == "" {
		return nil, fmt.Errorf("missing required parameter 'thought'. Provide your reasoning or analysis as a string (e.g., {\"thought\": \"Need to analyse the API response structure before processing\"})")
	}

	// Validate thought length with safety buffer
	// We advertise maxLength to agents but accept up to maxLength + buffer
	// because AI agents are imprecise at counting characters
	configuredMaxLength := getMaxThoughtLength()
	actualMaxLength := configuredMaxLength + ThinkLengthSafetyBuffer
	if len(thought) > actualMaxLength {
		return nil, fmt.Errorf("'thought' exceeds maximum length of %d characters (you sent %d chars, ~%d words). The think tool is for brief 2-4 sentence notes. For multi-step reasoning, use sequential_thinking instead", configuredMaxLength, len(thought), len(strings.Fields(thought)))
	}

	// Parse how_hard (optional, defaults to "hard")
	howHard := "hard"
	if howHardValue, exists := args["how_hard"]; exists {
		if howHardStr, ok := howHardValue.(string); ok {
			switch howHardStr {
			case "hard", "harder", "ultra":
				howHard = howHardStr
			default:
				return nil, fmt.Errorf("invalid 'how_hard' parameter: must be 'hard' (default), 'harder', or 'ultra', but got '%s'. Use 'harder' or 'ultra' for more complex reasoning", howHardStr)
			}
		} else {
			return nil, fmt.Errorf("invalid 'how_hard' parameter: must be a string ('hard', 'harder', or 'ultra')")
		}
	}

	return &ThinkRequest{
		Thought: thought,
		HowHard: howHard,
	}, nil
}

// newToolResultText creates a new tool result with text content
func (t *ThinkTool) newToolResultText(howHard, thought string) (*mcp.CallToolResult, error) {
	var toolName string
	if howHard == "ultra" {
		toolName = "ultrathink"
	} else {
		toolName = fmt.Sprintf("think %s", howHard)
	}
	formattedThought := fmt.Sprintf("I should use the %s tool on this problem: %s", toolName, thought)
	return mcp.NewToolResultText(formattedThought), nil
}

package think

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// ThinkTool implements a simple thinking tool for structured reasoning
type ThinkTool struct{}

const (
	// DefaultMaxThoughtLength is the default maximum length for thought input
	DefaultMaxThoughtLength = 2000
	// ThinkMaxLengthEnvVar is the environment variable for configuring max thought length
	ThinkMaxLengthEnvVar = "THINK_MAX_LENGTH"
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
		mcp.WithDescription(fmt.Sprintf(`Use the tool to think about something. It will not obtain new information or change the database, but just append the thought to the log. Use it when complex reasoning or some cache memory is needed.

This tool is particularly valuable for:
- Breaking down complex multi-step problems
- Reasoning through policy decisions or constraints
- Planning sequential actions where mistakes are costly
- Processing and reflecting on information gathered from previous tool calls

Maximum thought length is %d characters.

Use this tool as a structured thinking space during complex workflows, especially when you need to pause and reason about what you've learned before proceeding.`, maxLen)),
		mcp.WithString("thought",
			mcp.Required(),
			mcp.MaxLength(maxLen),
			mcp.Description("A thought to think about."),
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
		return nil, fmt.Errorf("missing or invalid required parameter: thought")
	}

	// Validate thought length
	maxLength := getMaxThoughtLength()
	if len(thought) > maxLength {
		return nil, fmt.Errorf("thought exceeds maximum length of %d characters (got %d)", maxLength, len(thought))
	}

	// Parse how_hard (optional, defaults to "hard")
	howHard := "hard"
	if howHardValue, exists := args["how_hard"]; exists {
		if howHardStr, ok := howHardValue.(string); ok {
			switch howHardStr {
			case "hard", "harder", "ultra":
				howHard = howHardStr
			default:
				return nil, fmt.Errorf("invalid how_hard parameter: must be 'hard', 'harder', or 'ultra', got '%s'", howHardStr)
			}
		} else {
			return nil, fmt.Errorf("invalid how_hard parameter: must be a string")
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

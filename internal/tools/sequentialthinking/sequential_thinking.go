package sequentialthinking

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// ThoughtData represents a single thought in the sequential thinking process
type ThoughtData struct {
	Thought       string `json:"thought"`
	ThoughtNumber int    `json:"thoughtNumber"`
	Continue      bool   `json:"continue"`
	IsRevision    bool   `json:"isRevision,omitempty"`
	RevisedText   string `json:"revisedText,omitempty"`
	ExploreLabel  string `json:"exploreLabel,omitempty"`
}

// SequentialThinkingTool implements the tools.Tool interface for sequential thinking
type SequentialThinkingTool struct {
	thoughtHistory      []ThoughtData
	branches            map[string][]ThoughtData
	disableLogging      bool
	thoughtHistoryMutex sync.RWMutex
}

// init registers the tool with the registry
func init() {
	registry.Register(&SequentialThinkingTool{
		branches: make(map[string][]ThoughtData),
	})
}

// Definition returns the tool's definition for MCP registration
func (t *SequentialThinkingTool) Definition() mcp.Tool {
	tool := mcp.NewTool(
		"sequential_thinking",
		mcp.WithDescription(`Solve complex problems step-by-step with automatic thought tracking, revision detection, and branch management. Ideal for multi-step reasoning, planning, and iterative refinement. Use 'get_usage' action for detailed instructions.`),
		mcp.WithString("action",
			mcp.Description("Action to perform: 'think' or 'get_usage'"),
			mcp.Enum("think", "get_usage"),
		),
		mcp.WithString("thought",
			mcp.Description("Your current thinking step (required for 'think' action)"),
		),
		mcp.WithBoolean("continue",
			mcp.Description("Whether more thinking is needed after this step (required for 'think' action)"),
		),
		mcp.WithString("revise",
			mcp.Description("Brief text snippet from previous thought to revise (optional)"),
		),
		mcp.WithString("explore",
			mcp.Description("Label for exploring alternative approach (optional)"),
		),

		// Non-destructive writing annotations
		mcp.WithReadOnlyHintAnnotation(false),    // Stores thinking state and history
		mcp.WithDestructiveHintAnnotation(false), // Doesn't destroy previous thoughts
		mcp.WithIdempotentHintAnnotation(false),  // Each thought adds new content
		mcp.WithOpenWorldHintAnnotation(false),   // Works with local state management
	)
	return tool
}

// Execute executes the tool's logic
func (t *SequentialThinkingTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Check if sequential-thinking tool is enabled
	if !tools.IsToolEnabled("sequential-thinking") {
		return nil, fmt.Errorf("sequential thinking tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'sequential-thinking'")
	}

	logger.Info("Executing sequential thinking tool")

	// Get action parameter (defaults to "think" for backward compatibility)
	action, ok := args["action"].(string)
	if !ok || action == "" {
		action = "think"
	}

	switch action {
	case "get_usage":
		return t.getUsage(), nil
	case "think":
		// Check for environment variable to disable thought logging
		t.disableLogging = strings.ToLower(os.Getenv("DISABLE_THOUGHT_LOGGING")) == "true"

		// Validate and parse the thought data
		thoughtData, err := t.validateThoughtData(args)
		if err != nil {
			return nil, fmt.Errorf("invalid thought data: %w", err)
		}

		// Process the thought
		result, err := t.processThought(thoughtData, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to process thought: %w", err)
		}

		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}

		return mcp.NewToolResultText(string(jsonBytes)), nil
	default:
		return nil, fmt.Errorf("invalid action: %s. Must be 'think' or 'get_usage'", action)
	}
}

// validateThoughtData validates and converts the input arguments to ThoughtData
func (t *SequentialThinkingTool) validateThoughtData(args map[string]any) (*ThoughtData, error) {
	thought, ok := args["thought"].(string)
	if !ok || thought == "" {
		return nil, fmt.Errorf("thought is required and must be a non-empty string")
	}

	continueThinking, ok := args["continue"].(bool)
	if !ok {
		return nil, fmt.Errorf("continue is required and must be a boolean")
	}

	data := &ThoughtData{
		Thought:  thought,
		Continue: continueThinking,
	}

	// Handle optional revision parameter
	if revise, ok := args["revise"].(string); ok && revise != "" {
		data.IsRevision = true
		data.RevisedText = revise
	}

	// Handle optional exploration parameter
	if explore, ok := args["explore"].(string); ok && explore != "" {
		data.ExploreLabel = explore
	}

	return data, nil
}

// processThought processes a thought and maintains the thought history
func (t *SequentialThinkingTool) processThought(thoughtData *ThoughtData, logger *logrus.Logger) (map[string]any, error) {
	t.thoughtHistoryMutex.Lock()
	defer t.thoughtHistoryMutex.Unlock()

	// Initialize branches map if nil
	if t.branches == nil {
		t.branches = make(map[string][]ThoughtData)
	}

	// Auto-manage thought numbering
	thoughtData.ThoughtNumber = len(t.thoughtHistory) + 1

	// Handle exploration (branching)
	var currentBranch string
	if thoughtData.ExploreLabel != "" {
		currentBranch = thoughtData.ExploreLabel
		if t.branches[currentBranch] == nil {
			t.branches[currentBranch] = []ThoughtData{}
		}
		t.branches[currentBranch] = append(t.branches[currentBranch], *thoughtData)
	}

	// Add thought to main history
	t.thoughtHistory = append(t.thoughtHistory, *thoughtData)

	// Format and log the thought if logging is enabled
	// Note: We use the logger instead of stderr to avoid MCP stdio protocol violations
	if !t.disableLogging {
		formattedThought := t.formatThought(thoughtData)
		logger.WithFields(logrus.Fields{
			"thought_number": thoughtData.ThoughtNumber,
			"is_revision":    thoughtData.IsRevision,
			"revised_text":   thoughtData.RevisedText,
			"explore_label":  thoughtData.ExploreLabel,
		}).Info("Sequential thinking step:\n" + formattedThought)
	}

	// Collect branch names
	branchNames := make([]string, 0, len(t.branches))
	for branchName := range t.branches {
		branchNames = append(branchNames, branchName)
	}

	// Create result
	result := map[string]any{
		"thoughtNumber":        thoughtData.ThoughtNumber,
		"totalThoughts":        len(t.thoughtHistory), // Auto-calculated
		"continue":             thoughtData.Continue,
		"branches":             branchNames,
		"thoughtHistoryLength": len(t.thoughtHistory),
	}

	return result, nil
}

// formatThought formats a thought for display with colours and structure
func (t *SequentialThinkingTool) formatThought(thoughtData *ThoughtData) string {
	var prefix, context string

	// Set up colours
	yellow := color.New(color.FgYellow).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	blue := color.New(color.FgBlue).SprintFunc()

	if thoughtData.IsRevision {
		prefix = yellow("🔄 Revision")
		if thoughtData.RevisedText != "" {
			context = fmt.Sprintf(" (revising: %.20s...)", thoughtData.RevisedText)
		}
	} else if thoughtData.ExploreLabel != "" {
		prefix = green("🌿 Explore")
		context = fmt.Sprintf(" (%s)", thoughtData.ExploreLabel)
	} else {
		prefix = blue("💭 Thought")
	}

	header := fmt.Sprintf("%s %d%s", prefix, thoughtData.ThoughtNumber, context)

	// Calculate border length based on content
	headerLen := len(fmt.Sprintf("💭 Thought %d%s", thoughtData.ThoughtNumber, context))
	thoughtLen := len(thoughtData.Thought)
	borderLen := max(thoughtLen, headerLen)
	borderLen += 4 // Add padding

	border := strings.Repeat("─", borderLen)

	return fmt.Sprintf(`
┌%s┐
│ %s%s │
├%s┤
│ %s%s │
└%s┘`,
		border,
		header, strings.Repeat(" ", borderLen-len(header)-2),
		border,
		thoughtData.Thought, strings.Repeat(" ", borderLen-len(thoughtData.Thought)-2),
		border)
}

// getUsage returns detailed usage instructions for the sequential thinking tool
func (t *SequentialThinkingTool) getUsage() *mcp.CallToolResult {
	usage := `# Sequential Thinking Tool - Detailed Usage Guide

## Purpose
A tool for dynamic and reflective problem-solving through sequential thinking.
This tool helps analyse problems through a flexible thinking process with auto-managed state.
Focus on your thinking content - the tool handles numbering, tracking, and branching automatically.

## When to Use This Tool
- Breaking down complex problems into steps
- Planning and design with room for revision
- Analysis that might need course correction
- Problems where the full scope might not be clear initially
- Problems that require a multi-step solution
- Tasks that need to maintain context over multiple steps
- Situations where irrelevant information needs to be filtered out

## Example Usage

**Step 1: Initial Analysis**
- thought: "Let me break down this API design problem..."
- continue: true

**Step 2: Revision**
- thought: "Actually, I missed security considerations..."
- continue: true
- revise: "API design problem" (snippet from previous thought)

**Step 3: Exploration**
- thought: "What if we used GraphQL instead?"
- continue: true
- explore: "graphql-alternative"

**Step 4: Final Decision**
- thought: "After comparing both approaches, REST is better..."
- continue: false

## Parameters Explained
- **thought**: Your current thinking step content (required)
- **continue**: Whether more thinking is needed after this step - true/false (required)
- **revise**: Brief text snippet from previous thought you're revising (optional)
- **explore**: Label for exploring alternative approach/branch (optional)

## Key Features
- **Auto-managed numbering**: No need to track thought numbers manually
- **Automatic totals**: Tool calculates total thoughts dynamically
- **Smart revision detection**: Just provide snippet of what you're revising
- **Simple branching**: Use explore labels to track alternative approaches
- **Progress tracking**: Tool maintains complete thinking history
- **Formatted logging**: Visual output shows thinking structure

## Best Practices
1. Focus on your thinking content, not mechanics
2. Use "revise" when reconsidering previous thoughts
3. Use "explore" when trying alternative approaches
4. Express uncertainty naturally in your thoughts
5. Set continue=false only when truly done with satisfactory answer
6. Keep thoughts focused and clear
7. Don't worry about numbering - the tool handles it

## Environment Variables
- **DISABLE_THOUGHT_LOGGING**: Set to "true" to disable formatted console output
- **ENABLE_ADDITIONAL_TOOLS**: Must include "sequential-thinking" to enable this tool`

	return mcp.NewToolResultText(usage)
}

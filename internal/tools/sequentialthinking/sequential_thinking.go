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
	"github.com/sirupsen/logrus"
)

// ThoughtData represents a single thought in the sequential thinking process
type ThoughtData struct {
	Thought           string `json:"thought"`
	ThoughtNumber     int    `json:"thoughtNumber"`
	NextThoughtNeeded bool   `json:"nextThoughtNeeded"`
	IsRevision        bool   `json:"isRevision,omitempty"`
	RevisedText       string `json:"revisedText,omitempty"`
	ExploreLabel      string `json:"exploreLabel,omitempty"`
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
		mcp.WithDescription(`A multi-step reasoning tool for dynamic and reflective problem-solving through sequential thoughts. Each step should be a focused, concise observation or decision (1-3 sentences per step, ~50-100 words). Use sparingly only when needed for the most complex problems.

Helps with complex problems where you need to expand the problem space by breaking down complex problems into steps`),
		mcp.WithString("action",
			mcp.Description("Action to perform: 'think' or 'get_usage'"),
			mcp.Enum("think", "get_usage"),
		),
		mcp.WithString("thought",
			mcp.Description("A single focused reasoning step: 1-3 sentences covering one observation, decision, or question. Use multiple steps for longer analysis rather than one long step."),
		),
		mcp.WithBoolean("nextThoughtNeeded",
			mcp.Description("Whether another thought step is needed (required for 'think' action)"),
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

	nextThoughtNeeded, ok := args["nextThoughtNeeded"].(bool)
	if !ok {
		return nil, fmt.Errorf("nextThoughtNeeded is required and must be a boolean")
	}

	data := &ThoughtData{
		Thought:           thought,
		NextThoughtNeeded: nextThoughtNeeded,
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

	// Initialise branches map if nil
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
		"nextThoughtNeeded":    thoughtData.NextThoughtNeeded,
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
A tool for dynamic and reflective problem-solving through sequential thoughts.
This tool helps analyse problems through a flexible thinking process with auto-managed state.
Each thought can build on, question, or revise previous insights as understanding deepens.
Focus on your thinking content - the tool handles numbering, tracking, and branching automatically.

## When to Use This Tool (Only for complex problems!)
- Breaking down complex problems into steps
- Planning and design with room for revision
- Analysis that might need course correction
- Problems where the full scope might not be clear initially
- Problems that require a multi-step solution
- Tasks that need to maintain context over multiple steps
- Situations where irrelevant information needs to be filtered out

## Key Features
- **Smart revision detection**: Just provide snippet of what you're revising
- **Simple branching**: Use explore labels to track alternative approaches
- **Progress tracking**: Tool maintains complete thinking history
- **Hypothesis workflow**: Generate hypotheses, verify them, repeat until satisfied

## Example Usage

**Step 1: Initial Analysis**
- thought: "Let me break down this API design problem..."
- nextThoughtNeeded: true

**Step 2: Revision**
- thought: "Actually, I missed security considerations..."
- nextThoughtNeeded: true
- revise: "API design problem" (snippet from previous thought)

**Step 3: Exploration**
- thought: "What if we used GraphQL instead?"
- nextThoughtNeeded: true
- explore: "graphql-alternative"

**Step 4: Final Decision**
- thought: "After comparing both approaches, REST is better because..."
- nextThoughtNeeded: false

## Parameters Explained
- **thought**: Your current thinking step content (required)
- **nextThoughtNeeded**: Whether another thought step is needed - true/false (required)
- **revise**: Brief text snippet from previous thought you're revising (optional)
- **explore**: Label for exploring alternative approach/branch (optional)

## You Should
1. Feel free to question or revise previous thoughts
2. Don't hesitate to add more thoughts if needed
3. Express uncertainty when present
4. Mark thoughts that revise previous thinking or branch into new paths
5. Ignore information that is irrelevant to the current step
6. Generate a solution hypothesis when appropriate
7. Verify the hypothesis based on your reasoning steps
8. Repeat the process until satisfied with the solution
9. Provide a single, ideally correct answer as the final output
10. Only set nextThoughtNeeded to false when truly done and satisfied

## Best Practices
1. Focus on your thinking content, not mechanics
2. Use "revise" when reconsidering previous thoughts
3. Use "explore" when trying alternative approaches
4. Keep thoughts focused and clear
5. Don't worry about numbering - the tool handles it`

	return mcp.NewToolResultText(usage)
}

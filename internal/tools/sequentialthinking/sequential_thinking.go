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
	Thought           string `json:"thought"`
	ThoughtNumber     int    `json:"thoughtNumber"`
	TotalThoughts     int    `json:"totalThoughts"`
	NextThoughtNeeded bool   `json:"nextThoughtNeeded"`
	IsRevision        bool   `json:"isRevision,omitempty"`
	RevisesThought    int    `json:"revisesThought,omitempty"`
	BranchFromThought int    `json:"branchFromThought,omitempty"`
	BranchID          string `json:"branchId,omitempty"`
	NeedsMoreThoughts bool   `json:"needsMoreThoughts,omitempty"`
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
	return mcp.NewTool(
		"sequential_thinking",
		mcp.WithDescription(`Dynamic problem-solving through sequential thinking with branching and revision.

Use for:
- Stubborn or complex multi-step problems requiring systematic breakdown
- Analysis needing course correction or alternative approaches
- Complex planning with iterative refinement and uncertainty handling

Features: thought revision, branching, progress tracking, formatted logging. Use 'get_usage' action for detailed usage instructions.`),
		mcp.WithString("action",
			mcp.Description("Action to perform: 'think' or 'get_usage'"),
			mcp.Enum("think", "get_usage"),
		),
		mcp.WithString("thought",
			mcp.Description("Your current thinking step (required for 'think' action)"),
		),
		mcp.WithBoolean("nextThoughtNeeded",
			mcp.Description("Whether another thought step is needed (required for 'think' action)"),
		),
		mcp.WithNumber("thoughtNumber",
			mcp.Description("Current thought number (required for 'think' action)"),
		),
		mcp.WithNumber("totalThoughts",
			mcp.Description("Estimated total thoughts needed (required for 'think' action)"),
		),
		mcp.WithBoolean("isRevision",
			mcp.Description("Whether this revises previous thinking"),
		),
		mcp.WithNumber("revisesThought",
			mcp.Description("Which thought is being reconsidered"),
		),
		mcp.WithNumber("branchFromThought",
			mcp.Description("Branching point thought number"),
		),
		mcp.WithString("branchId",
			mcp.Description("Branch identifier"),
		),
		mcp.WithBoolean("needsMoreThoughts",
			mcp.Description("If more thoughts are needed"),
		),
	)
}

// Execute executes the tool's logic
func (t *SequentialThinkingTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
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
func (t *SequentialThinkingTool) validateThoughtData(args map[string]interface{}) (*ThoughtData, error) {
	thought, ok := args["thought"].(string)
	if !ok || thought == "" {
		return nil, fmt.Errorf("thought is required and must be a non-empty string")
	}

	nextThoughtNeeded, ok := args["nextThoughtNeeded"].(bool)
	if !ok {
		return nil, fmt.Errorf("nextThoughtNeeded is required and must be a boolean")
	}

	thoughtNumber, ok := args["thoughtNumber"].(float64)
	if !ok || thoughtNumber < 1 {
		return nil, fmt.Errorf("thoughtNumber is required and must be a positive integer")
	}

	totalThoughts, ok := args["totalThoughts"].(float64)
	if !ok || totalThoughts < 1 {
		return nil, fmt.Errorf("totalThoughts is required and must be a positive integer")
	}

	data := &ThoughtData{
		Thought:           thought,
		ThoughtNumber:     int(thoughtNumber),
		TotalThoughts:     int(totalThoughts),
		NextThoughtNeeded: nextThoughtNeeded,
	}

	// Parse optional fields
	if isRevision, ok := args["isRevision"].(bool); ok {
		data.IsRevision = isRevision
	}

	if revisesThought, ok := args["revisesThought"].(float64); ok && revisesThought >= 1 {
		data.RevisesThought = int(revisesThought)
	}

	if branchFromThought, ok := args["branchFromThought"].(float64); ok && branchFromThought >= 1 {
		data.BranchFromThought = int(branchFromThought)
	}

	if branchID, ok := args["branchId"].(string); ok && branchID != "" {
		data.BranchID = branchID
	}

	if needsMoreThoughts, ok := args["needsMoreThoughts"].(bool); ok {
		data.NeedsMoreThoughts = needsMoreThoughts
	}

	return data, nil
}

// processThought processes a thought and maintains the thought history
func (t *SequentialThinkingTool) processThought(thoughtData *ThoughtData, logger *logrus.Logger) (map[string]interface{}, error) {
	t.thoughtHistoryMutex.Lock()
	defer t.thoughtHistoryMutex.Unlock()

	// Initialize branches map if nil
	if t.branches == nil {
		t.branches = make(map[string][]ThoughtData)
	}

	// Adjust total thoughts if current thought number exceeds it
	if thoughtData.ThoughtNumber > thoughtData.TotalThoughts {
		thoughtData.TotalThoughts = thoughtData.ThoughtNumber
	}

	// Add thought to history
	t.thoughtHistory = append(t.thoughtHistory, *thoughtData)

	// Handle branching
	if thoughtData.BranchFromThought > 0 && thoughtData.BranchID != "" {
		if t.branches[thoughtData.BranchID] == nil {
			t.branches[thoughtData.BranchID] = []ThoughtData{}
		}
		t.branches[thoughtData.BranchID] = append(t.branches[thoughtData.BranchID], *thoughtData)
	}

	// Format and log the thought if logging is enabled
	// Note: We use the logger instead of stderr to avoid MCP stdio protocol violations
	if !t.disableLogging {
		formattedThought := t.formatThought(thoughtData)
		logger.WithFields(logrus.Fields{
			"thought_number": thoughtData.ThoughtNumber,
			"total_thoughts": thoughtData.TotalThoughts,
			"is_revision":    thoughtData.IsRevision,
			"branch_id":      thoughtData.BranchID,
		}).Info("Sequential thinking step:\n" + formattedThought)
	}

	// Collect branch names
	branchNames := make([]string, 0, len(t.branches))
	for branchName := range t.branches {
		branchNames = append(branchNames, branchName)
	}

	// Create result
	result := map[string]interface{}{
		"thoughtNumber":        thoughtData.ThoughtNumber,
		"totalThoughts":        thoughtData.TotalThoughts,
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
		if thoughtData.RevisesThought > 0 {
			context = fmt.Sprintf(" (revising thought %d)", thoughtData.RevisesThought)
		}
	} else if thoughtData.BranchFromThought > 0 {
		prefix = green("🌿 Branch")
		context = fmt.Sprintf(" (from thought %d", thoughtData.BranchFromThought)
		if thoughtData.BranchID != "" {
			context += fmt.Sprintf(", ID: %s", thoughtData.BranchID)
		}
		context += ")"
	} else {
		prefix = blue("💭 Thought")
	}

	header := fmt.Sprintf("%s %d/%d%s", prefix, thoughtData.ThoughtNumber, thoughtData.TotalThoughts, context)

	// Calculate border length based on content
	headerLen := len(fmt.Sprintf("💭 Thought %d/%d%s", thoughtData.ThoughtNumber, thoughtData.TotalThoughts, context))
	thoughtLen := len(thoughtData.Thought)
	borderLen := headerLen
	if thoughtLen > headerLen {
		borderLen = thoughtLen
	}
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
This tool helps analyse problems through a flexible thinking process that can adapt and evolve.
Each thought can build on, question, or revise previous insights as understanding deepens.

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
- thoughtNumber: 1, totalThoughts: 3, nextThoughtNeeded: true

**Step 2: Revision**
- thought: "Actually, I missed security considerations..."
- thoughtNumber: 2, totalThoughts: 4 (adjusted up)
- isRevision: true, revisesThought: 1 (revising step 1)

**Step 3: Branching**
- thought: "What if we used GraphQL instead?"
- thoughtNumber: 3, branchFromThought: 2, branchId: "graphql-alternative"

**Step 4: Final Decision**
- thought: "After comparing both approaches, REST is better..."
- thoughtNumber: 4, nextThoughtNeeded: false (completed)

## Parameters Explained
- **thought**: Your current thinking step, which can include:
  * Regular analytical steps
  * Revisions of previous thoughts
  * Questions about previous decisions
  * Realisations about needing more analysis
  * Changes in approach
  * Hypothesis generation & verification
- **nextThoughtNeeded**: True if you need more thinking, even if at what seemed like the end
- **thoughtNumber**: Current number in sequence (can go beyond initial total if needed)
- **totalThoughts**: Current estimate of thoughts needed (can be adjusted up/down)
- **isRevision**: A boolean indicating if this thought revises previous thinking
- **revisesThought**: If isRevision is true, which thought number is being reconsidered
- **branchFromThought**: If branching, which thought number is the branching point
- **branchId**: Identifier for the current branch (if any)
- **needsMoreThoughts**: If reaching end but realising more thoughts needed

## Best Practices
1. Don't hesitate to add more thoughts if needed, even at the "end"
2. Express uncertainty when present
3. Mark thoughts that revise previous thinking or branch into new paths
4. Ignore information that is irrelevant to the current step
5. Generate a solution hypothesis when appropriate
6. Verify the hypothesis based on the Chain of Thought steps
7. Repeat the process until satisfied with the solution
8. Provide a single, ideally correct answer as the final output
9. Only set nextThoughtNeeded to false when truly done and a satisfactory answer is reached
10. Be concise and clear, don't waste tokens`

	return mcp.NewToolResultText(usage)
}

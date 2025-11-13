package benchmarks

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/pkoukk/tiktoken-go"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"

	// Import all tools to trigger registration
	_ "github.com/sammcj/mcp-devtools/internal/tools/api"
	_ "github.com/sammcj/mcp-devtools/internal/tools/aws_documentation"
	_ "github.com/sammcj/mcp-devtools/internal/tools/calculator"
	_ "github.com/sammcj/mcp-devtools/internal/tools/claudeagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/codeskim"
	_ "github.com/sammcj/mcp-devtools/internal/tools/codexagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/copilotagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/docprocessing"
	_ "github.com/sammcj/mcp-devtools/internal/tools/excel"
	_ "github.com/sammcj/mcp-devtools/internal/tools/filelength"
	_ "github.com/sammcj/mcp-devtools/internal/tools/filesystem"
	_ "github.com/sammcj/mcp-devtools/internal/tools/geminiagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/github"
	_ "github.com/sammcj/mcp-devtools/internal/tools/internetsearch/unified"
	_ "github.com/sammcj/mcp-devtools/internal/tools/m2e"
	_ "github.com/sammcj/mcp-devtools/internal/tools/memory"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packagedocs"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/unified"
	_ "github.com/sammcj/mcp-devtools/internal/tools/pdf"
	_ "github.com/sammcj/mcp-devtools/internal/tools/qdeveloperagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/securityoverride"
	_ "github.com/sammcj/mcp-devtools/internal/tools/sequentialthinking"
	_ "github.com/sammcj/mcp-devtools/internal/tools/shadcnui"
	_ "github.com/sammcj/mcp-devtools/internal/tools/terraform_documentation"
	_ "github.com/sammcj/mcp-devtools/internal/tools/think"
	_ "github.com/sammcj/mcp-devtools/internal/tools/utilities/toolhelp"
	_ "github.com/sammcj/mcp-devtools/internal/tools/webfetch"
)

var (
	perToolMax    = flag.Int("per-tool-max", 800, "Maximum tokens per tool before failing")
	totalMax      = flag.Int("total-max", 10000, "Maximum total tokens for all tools before failing")
	warnThreshold = flag.Int("warn-threshold", 500, "Threshold for warning about high token tools")
)

// ToolTokenCost represents token cost breakdown for a tool
type ToolTokenCost struct {
	Name         string
	TotalTokens  int
	DescTokens   int
	ParamsTokens int
	FullJSON     string
	DescJSON     string
	ParamsJSON   string
}

// getColorCode returns ANSI color code based on token count
func getColorCode(tokens int, warnThreshold int) string {
	switch {
	case tokens <= 200:
		return "\033[32m" // Green
	case tokens <= warnThreshold:
		return "\033[33m" // Yellow
	case tokens <= 600:
		return "\033[38;5;208m" // Orange
	default:
		return "\033[31m" // Red
	}
}

// getStatusEmoji returns emoji based on token count
func getStatusEmoji(tokens int, warnThreshold int) string {
	switch {
	case tokens <= 200:
		return "🟢"
	case tokens <= warnThreshold:
		return "🟡"
	case tokens <= 600:
		return "🟠"
	default:
		return "🔴"
	}
}

// getStatusText returns status text based on token count
func getStatusText(tokens int, warnThreshold int) string {
	switch {
	case tokens <= 200:
		return "OK"
	case tokens <= warnThreshold:
		return "OK"
	case tokens <= 600:
		return "HIGH"
	default:
		return "HIGH"
	}
}

const resetColor = "\033[0m"

// countTokens uses tiktoken to estimate token count
func countTokens(text string) (int, error) {
	tkm, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return 0, fmt.Errorf("failed to get encoding: %w", err)
	}

	tokens := tkm.Encode(text, nil, nil)
	return len(tokens), nil
}

// analyseToolTokenCost analyses token cost for a single tool
func analyseToolTokenCost(tool tools.Tool) (*ToolTokenCost, error) {
	def := tool.Definition()

	// Serialise full tool definition (as MCP would send it)
	fullJSON, err := json.Marshal(def)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool definition: %w", err)
	}

	// Serialise just description
	descJSON, err := json.Marshal(def.Description)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal description: %w", err)
	}

	// Serialise just parameters
	paramsJSON, err := json.Marshal(def.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input schema: %w", err)
	}

	// Count tokens
	totalTokens, err := countTokens(string(fullJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to count total tokens: %w", err)
	}

	descTokens, err := countTokens(string(descJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to count description tokens: %w", err)
	}

	paramsTokens, err := countTokens(string(paramsJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to count params tokens: %w", err)
	}

	return &ToolTokenCost{
		Name:         def.Name,
		TotalTokens:  totalTokens,
		DescTokens:   descTokens,
		ParamsTokens: paramsTokens,
		FullJSON:     string(fullJSON),
		DescJSON:     string(descJSON),
		ParamsJSON:   string(paramsJSON),
	}, nil
}

func TestToolTokenCost(t *testing.T) {
	// Initialise the registry
	logger := &logrus.Logger{}
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.ErrorLevel)
	registry.Init(logger)

	// Get all registered tools
	allTools := registry.GetTools()
	if len(allTools) == 0 {
		t.Fatal("No tools registered - check imports")
	}

	// Analyse each tool
	var costs []*ToolTokenCost
	var totalTokens int
	var failed bool

	for _, tool := range allTools {
		cost, err := analyseToolTokenCost(tool)
		if err != nil {
			t.Errorf("Failed to analyse tool %s: %v", tool.Definition().Name, err)
			continue
		}
		costs = append(costs, cost)
		totalTokens += cost.TotalTokens
	}

	// Sort by total tokens (descending)
	sort.Slice(costs, func(i, j int) bool {
		return costs[i].TotalTokens > costs[j].TotalTokens
	})

	// Print header
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("Tool Token Cost Analysis")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Print table header
	fmt.Printf("%-30s %9s %8s %8s %8s\n", "Tool", "Total", "Desc", "Params", "Status")
	fmt.Println(strings.Repeat("─", 80))

	// Count distribution
	var lowCount, mediumCount, highCount, veryHighCount int

	// Print each tool
	for _, cost := range costs {
		emoji := getStatusEmoji(cost.TotalTokens, *warnThreshold)
		colorCode := getColorCode(cost.TotalTokens, *warnThreshold)
		status := getStatusText(cost.TotalTokens, *warnThreshold)

		fmt.Printf("%-30s %s%s %6d%s %8d %8d %8s\n",
			cost.Name,
			colorCode,
			emoji,
			cost.TotalTokens,
			resetColor,
			cost.DescTokens,
			cost.ParamsTokens,
			status,
		)

		// Update distribution counts
		switch {
		case cost.TotalTokens <= 200:
			lowCount++
		case cost.TotalTokens <= *warnThreshold:
			mediumCount++
		case cost.TotalTokens <= 600:
			highCount++
		default:
			veryHighCount++
		}
	}

	fmt.Println(strings.Repeat("─", 80))
	fmt.Println()

	// Print summary
	fmt.Println("Summary:")
	fmt.Printf("  Total Tools: %d\n", len(costs))
	fmt.Printf("  Total Tokens: %d\n", totalTokens)
	if len(costs) > 0 {
		fmt.Printf("  Average per Tool: %d\n", totalTokens/len(costs))
	}
	fmt.Println()

	// Print distribution
	fmt.Println("Distribution:")
	fmt.Printf("  🟢 Low (0-200):        %2d tools (%d%%)\n",
		lowCount, (lowCount*100)/len(costs))
	fmt.Printf("  🟡 Medium (201-%d):   %2d tools (%d%%)\n",
		*warnThreshold, mediumCount, (mediumCount*100)/len(costs))
	fmt.Printf("  🟠 High (%d-600):    %2d tools (%d%%)\n",
		*warnThreshold+1, highCount, (highCount*100)/len(costs))
	fmt.Printf("  🔴 Very High (>600):   %2d tools (%d%%)\n",
		veryHighCount, (veryHighCount*100)/len(costs))
	fmt.Println()

	// Print context impact
	fmt.Println("Context Impact:")
	fmt.Printf("  vs 200k context: %.2f%%\n", float64(totalTokens)/200000*100)
	fmt.Printf("  vs 100k context: %.2f%%\n", float64(totalTokens)/100000*100)
	fmt.Printf("  vs  50k context: %.2f%%\n", float64(totalTokens)/50000*100)
	fmt.Println()

	// Check thresholds
	fmt.Println("Threshold Checks:")

	// Check per-tool max
	var maxTool *ToolTokenCost
	for _, cost := range costs {
		if maxTool == nil || cost.TotalTokens > maxTool.TotalTokens {
			maxTool = cost
		}

		if cost.TotalTokens > *perToolMax {
			fmt.Printf("  ❌ Tool '%s' exceeds per-tool max: %d > %d tokens\n",
				cost.Name, cost.TotalTokens, *perToolMax)
			failed = true
		}
	}

	if maxTool != nil && maxTool.TotalTokens <= *perToolMax {
		fmt.Printf("  ✅ Per-tool max: %d tokens (PASS - highest: %s with %d tokens)\n",
			*perToolMax, maxTool.Name, maxTool.TotalTokens)
	} else if maxTool != nil {
		fmt.Printf("  ❌ Per-tool max: %d tokens (FAIL - highest: %s with %d tokens)\n",
			*perToolMax, maxTool.Name, maxTool.TotalTokens)
	}

	// Check total max
	if totalTokens > *totalMax {
		fmt.Printf("  ❌ Total max: %d tokens (FAIL - actual: %d tokens)\n",
			*totalMax, totalTokens)
		failed = true
	} else {
		fmt.Printf("  ✅ Total max: %d tokens (PASS - actual: %d tokens)\n",
			*totalMax, totalTokens)
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Fail test if thresholds exceeded
	if failed {
		t.Fatal("Token cost thresholds exceeded")
	}

	// Always exit with appropriate code when run with -v
	if testing.Verbose() {
		if failed {
			os.Exit(1)
		}
	}
}

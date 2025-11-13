// go:build listtools
//go:build listtools

package benchmarks

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"

	// Import all tool packages to register them
	_ "github.com/sammcj/mcp-devtools/internal/imports"
)

// TestListTools outputs all tool definitions as seen by MCP clients
func TestListTools(t *testing.T) {
	// Create a silent logger
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	logger.SetOutput(os.Stderr)

	// Initialise the registry
	registry.Init(logger)

	// Get enabled tools
	enabledTools := registry.GetEnabledTools()

	// Get sorted tool names
	var toolNames []string
	for name := range enabledTools {
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)

	// Build output structure
	output := struct {
		Count int              `json:"count"`
		Tools []map[string]any `json:"tools"`
	}{
		Count: len(toolNames),
		Tools: make([]map[string]any, 0, len(toolNames)),
	}

	// Collect tool definitions
	for _, name := range toolNames {
		tool := enabledTools[name]
		def := tool.Definition()

		// Convert to map for JSON serialisation
		toolData := map[string]any{
			"name":        def.Name,
			"description": def.Description,
			"inputSchema": def.InputSchema,
		}

		// Add annotations if any are set
		if def.Annotations.Title != "" ||
			def.Annotations.ReadOnlyHint != nil ||
			def.Annotations.DestructiveHint != nil ||
			def.Annotations.IdempotentHint != nil ||
			def.Annotations.OpenWorldHint != nil {
			toolData["annotations"] = def.Annotations
		}

		output.Tools = append(output.Tools, toolData)
	}

	// Print JSON output
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		t.Fatalf("Error encoding JSON: %v", err)
	}

	// Also print summary to stderr for informational purposes
	fmt.Fprintf(os.Stderr, "\nTotal tools: %d\n", len(toolNames))
}

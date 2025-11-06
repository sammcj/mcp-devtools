package registry

import (
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

var (
	// toolRegistry is a map of tool names to tool implementations
	toolRegistry = make(map[string]tools.Tool) // Initialise here

	// disabledFunctions is a set of function names to disable
	disabledFunctions = make(map[string]bool)

	// logger is the shared logger instance
	logger *logrus.Logger

	// cache is the shared cache instance
	cache *sync.Map
)

// Init initialises the registry and shared resources
func Init(l *logrus.Logger) {
	logger = l
	cache = &sync.Map{}

	// Parse DISABLED_FUNCTIONS environment variable
	parseDisabledFunctions()
}

// parseDisabledFunctions parses the DISABLED_FUNCTIONS environment variable
func parseDisabledFunctions() {
	disabledEnv := os.Getenv("DISABLED_FUNCTIONS")
	if disabledEnv == "" {
		return
	}

	// Split by comma and trim whitespace
	functions := strings.SplitSeq(disabledEnv, ",")
	for function := range functions {
		function = strings.TrimSpace(function)
		if function != "" {
			disabledFunctions[function] = true
			if logger != nil {
				logger.WithField("function", function).Debug("Function disabled via DISABLED_FUNCTIONS environment variable")
			}
		}
	}

	if logger != nil && len(disabledFunctions) > 0 {
		logger.WithField("count", len(disabledFunctions)).Debug("Parsed disabled functions from environment")
	}
}

// Register adds a tool implementation to the registry
func Register(tool tools.Tool) {
	// No need to check for nil if toolRegistry is Initialised at declaration.
	// If it could somehow be nil due to other logic, the check can remain,
	// but the primary initialization is now at var declaration.
	// For safety, keeping the nil check might be okay, but it shouldn't be hit.
	if toolRegistry == nil { // This should ideally not be necessary now
		toolRegistry = make(map[string]tools.Tool)
	}
	toolRegistry[tool.Definition().Name] = tool
}

// GetTool retrieves a tool by name, returns false if disabled
func GetTool(name string) (tools.Tool, bool) {
	// Check if function is disabled
	if disabledFunctions[name] {
		return nil, false
	}
	tool, ok := toolRegistry[name]
	return tool, ok
}

// GetTools returns all registered tools, excluding disabled ones
func GetTools() map[string]tools.Tool {
	filteredTools := make(map[string]tools.Tool)
	for name, tool := range toolRegistry {
		// Skip disabled functions
		if disabledFunctions[name] {
			continue
		}
		filteredTools[name] = tool
	}
	return filteredTools
}

// GetEnabledTools returns all tools that are enabled for MCP server registration
func GetEnabledTools() map[string]tools.Tool {
	filteredTools := make(map[string]tools.Tool)
	for name, tool := range toolRegistry {
		// Skip disabled functions
		if disabledFunctions[name] {
			continue
		}

		// Skip tools that require enablement but aren't enabled
		if requiresEnablement(name) && !isToolEnabled(name) {
			continue
		}

		filteredTools[name] = tool
	}
	return filteredTools
}

// GetLogger returns the shared logger instance
func GetLogger() *logrus.Logger {
	return logger
}

// GetCache returns the shared cache instance
func GetCache() *sync.Map {
	return cache
}

// GetEnabledToolNames returns a sorted list of enabled tool names
func GetEnabledToolNames() []string {
	var names []string
	for name := range toolRegistry {
		// Skip disabled functions
		if disabledFunctions[name] {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetToolNamesWithExtendedHelp returns a sorted list of enabled tool names that provide extended help
func GetToolNamesWithExtendedHelp() []string {
	var names []string
	for name, tool := range toolRegistry {
		// Skip disabled functions
		if disabledFunctions[name] {
			continue
		}

		// Skip tools that require enablement but are not enabled
		if requiresEnablement(name) && !isToolEnabled(name) {
			continue
		}

		// Only include tools that implement ExtendedHelpProvider
		if _, ok := tool.(tools.ExtendedHelpProvider); ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// requiresEnablement checks if a tool requires enablement via ENABLE_ADDITIONAL_TOOLS
func requiresEnablement(toolName string) bool {
	additionalTools := []string{
		"filesystem",
		"security",
		"security_override",
		"sbom",
		"vulnerability_scan",
		"claude-agent",
		"codex-agent",
		"copilot-agent",
		"gemini-agent",
		"q-developer-agent",
		"generate_changelog",
		"process_document",
		"pdf",
		"memory",
		"aws_documentation",
		"terraform_documentation",
		"shadcn",
		"murican_to_english",
		"excel",
		"find_long_files",
	}

	// Normalize the tool name (lowercase, replace underscores with hyphens)
	normalisedToolName := strings.ToLower(strings.ReplaceAll(toolName, "_", "-"))

	for _, tool := range additionalTools {
		// Normalize the additional tool name (lowercase, replace underscores with hyphens)
		normalisedAdditionalTool := strings.ToLower(strings.ReplaceAll(tool, "_", "-"))
		if normalisedToolName == normalisedAdditionalTool {
			return true
		}
	}
	return false
}

// isToolEnabled checks if a tool is enabled via the ENABLE_ADDITIONAL_TOOLS environment variable
func isToolEnabled(toolName string) bool {
	enabledTools := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	if enabledTools == "" {
		return false
	}

	// Normalize the tool name (lowercase, replace underscores with hyphens)
	normalisedToolName := strings.ToLower(strings.ReplaceAll(toolName, "_", "-"))

	// Split by comma and check each tool
	toolsList := strings.SplitSeq(enabledTools, ",")
	for tool := range toolsList {
		// Normalize the tool from env var (trim spaces, lowercase, replace underscores with hyphens)
		normalizedTool := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tool), "_", "-"))
		if normalizedTool == normalisedToolName {
			return true
		}
	}

	return false
}

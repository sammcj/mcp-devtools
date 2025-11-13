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

	// disabledTools is a set of tool names to disable
	disabledTools = make(map[string]bool)

	// logger is the shared logger instance
	logger *logrus.Logger

	// cache is the shared cache instance
	cache *sync.Map
)

// Init initialises the registry and shared resources
func Init(l *logrus.Logger) {
	logger = l
	cache = &sync.Map{}

	// Parse DISABLED_TOOLS environment variable
	parseDisabledTools()
}

// parseDisabledTools parses the DISABLED_TOOLS and DISABLED_FUNCTIONS (legacy) environment variables
func parseDisabledTools() {
	// Clear the map first to ensure we start fresh
	disabledTools = make(map[string]bool)

	disabledEnv := os.Getenv("DISABLED_TOOLS")
	legacyEnv := os.Getenv("DISABLED_FUNCTIONS")

	// Helper function to parse and add tools to the disabled set
	parseAndAdd := func(envValue, source string) {
		if envValue == "" {
			return
		}

		tools := strings.SplitSeq(envValue, ",")
		for tool := range tools {
			tool = strings.TrimSpace(tool)
			if tool != "" {
				disabledTools[tool] = true
				if logger != nil {
					logger.WithField("tool", tool).WithField("source", source).Debug("Tool disabled")
				}
			}
		}
	}

	// Parse legacy env var first (if set, warn about deprecation)
	if legacyEnv != "" {
		if logger != nil {
			logger.Warn("DISABLED_FUNCTIONS environment variable is deprecated, please use DISABLED_TOOLS instead")
		}
		parseAndAdd(legacyEnv, "DISABLED_FUNCTIONS")
	}

	// Parse current env var (can override or add to legacy)
	parseAndAdd(disabledEnv, "DISABLED_TOOLS")

	if logger != nil && len(disabledTools) > 0 {
		logger.WithField("count", len(disabledTools)).Debug("Parsed disabled tools from environment")
	}
}

// requiresEnablement checks if a tool requires enablement via ENABLE_ADDITIONAL_TOOLS.
// When adding new tools that should be disabled by default, add their names to the additionalTools list.
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
		"magic_ui",
		"murican_to_english",
		"excel",
		"code_skim",
	}

	// Normalise the tool name (lowercase, replace underscores with hyphens)
	normalisedToolName := strings.ToLower(strings.ReplaceAll(toolName, "_", "-"))

	for _, tool := range additionalTools {
		// Normalise the additional tool name (lowercase, replace underscores with hyphens)
		normalisedAdditionalTool := strings.ToLower(strings.ReplaceAll(tool, "_", "-"))
		if normalisedToolName == normalisedAdditionalTool {
			return true
		}
	}
	return false
}

// ShouldRegisterTool checks if a tool should be registered based on:
// 1. DISABLED_TOOLS or DISABLED_FUNCTIONS (legacy) - explicit disable, highest priority
// 2. Tool's enablement requirement
// 3. ENABLE_ADDITIONAL_TOOLS (explicit enable)
func ShouldRegisterTool(toolName string) bool {
	// Check DISABLED_TOOLS/DISABLED_FUNCTIONS first (explicit disable wins)
	if disabledTools[toolName] {
		if logger != nil {
			logger.WithField("tool", toolName).Debug("Tool disabled via environment variable")
		}
		return false
	}

	// Check if tool requires enablement
	if requiresEnablement(toolName) {
		// Must be explicitly enabled
		enabled := isToolEnabled(toolName)
		if logger != nil {
			if enabled {
				logger.WithField("tool", toolName).Debug("Tool enabled via ENABLE_ADDITIONAL_TOOLS")
			} else {
				logger.WithField("tool", toolName).Debug("Tool requires enablement but is not enabled")
			}
		}
		return enabled
	}

	// Enabled by default
	if logger != nil {
		logger.WithField("tool", toolName).Debug("Tool enabled by default")
	}
	return true
}

// Register adds a tool implementation to the registry if it should be registered
func Register(tool tools.Tool) {
	if toolRegistry == nil {
		toolRegistry = make(map[string]tools.Tool)
	}

	toolName := tool.Definition().Name

	// Check if tool should be registered
	if !ShouldRegisterTool(toolName) {
		if logger != nil {
			logger.WithField("tool", toolName).Debug("Tool not registered (disabled or requires enablement)")
		}
		return
	}

	toolRegistry[toolName] = tool
	if logger != nil {
		logger.WithField("tool", toolName).Debug("Tool successfully registered")
	}
}

// GetTool retrieves a tool by name, returns false if disabled
func GetTool(name string) (tools.Tool, bool) {
	// Check if function is disabled
	if disabledTools[name] {
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
		if disabledTools[name] {
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
		if disabledTools[name] {
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
		if disabledTools[name] {
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
		if disabledTools[name] {
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

// isToolEnabled checks if a tool is enabled via the ENABLE_ADDITIONAL_TOOLS environment variable
func isToolEnabled(toolName string) bool {
	enabledTools := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	if enabledTools == "" {
		return false
	}

	// Check if "all" is specified to enable all tools
	if strings.TrimSpace(strings.ToLower(enabledTools)) == "all" {
		return true
	}

	// Normalise the tool name (lowercase, replace underscores with hyphens)
	normalisedToolName := strings.ToLower(strings.ReplaceAll(toolName, "_", "-"))

	// Split by comma and check each tool
	toolsList := strings.SplitSeq(enabledTools, ",")
	for tool := range toolsList {
		// Normalise the tool from env var (trim spaces, lowercase, replace underscores with hyphens)
		normalisedTool := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tool), "_", "-"))
		if normalisedTool == normalisedToolName {
			return true
		}
	}

	return false
}

package tools

import (
	"os"
	"strings"
)

// IsToolEnabled checks if a tool is enabled via the ENABLE_ADDITIONAL_TOOLS environment variable.
// The environment variable should contain a comma-separated list of tool names.
// Tool names are case-insensitive and spaces are ignored.
//
// Example: ENABLE_ADDITIONAL_TOOLS="claude-agent,gemini-agent,filesystem,vulnerability_scan,sbom,aws"
//
// Supported tool names:
// - claude-agent
// - gemini-agent
// - filesystem
// - vulnerability_scan
// - sbom
// - aws
// - security
// - security_override
// - memory
// - changelog
// - process_document
// - pdf


func IsToolEnabled(toolName string) bool {
	enabledTools := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	if enabledTools == "" {
		return false
	}

	// Normalise the tool name (lowercase, replace underscores with hyphens)
	normalisedToolName := strings.ToLower(strings.ReplaceAll(toolName, "_", "-"))

	// Split by comma and check each tool
	toolsList := strings.Split(enabledTools, ",")
	for _, tool := range toolsList {
		// Normalise the tool from env var (trim spaces, lowercase, replace underscores with hyphens)
		NormalisedTool := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tool), "_", "-"))
		if NormalisedTool == normalisedToolName {
			return true
		}
	}

	return false
}

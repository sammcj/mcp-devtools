package tools

import (
	"os"
	"strings"
)

// IsToolEnabled checks if a tool is enabled via the ENABLE_ADDITIONAL_TOOLS environment variable.
// The environment variable should contain a comma-separated list of tool names.
// Tool names are case-insensitive and spaces are ignored.
//
// Example: ENABLE_ADDITIONAL_TOOLS="claude-agent,gemini-agent,filesystem,vulnerability_scan,sbom,aws,api"
//
// Supported tool names:
// - api
// - aws_documentation
// - changelog
// - claude-agent
// - codex-agent
// - filesystem
// - gemini-agent
// - memory
// - pdf
// - process_document
// - q-developer-agent
// - sbom
// - security
// - security_override
// - sequential-thinking
// - terraform_documentation
// - vulnerability_scan

func IsToolEnabled(toolName string) bool {
	enabledTools := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	if enabledTools == "" {
		return false
	}

	// Normalise the tool name (lowercase, replace underscores with hyphens)
	normalisedToolName := strings.ToLower(strings.ReplaceAll(toolName, "_", "-"))

	// Split by comma and check each tool
	toolsList := strings.SplitSeq(enabledTools, ",")
	for tool := range toolsList {
		// Normalise the tool from env var (trim spaces, lowercase, replace underscores with hyphens)
		NormalisedTool := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tool), "_", "-"))
		if NormalisedTool == normalisedToolName {
			return true
		}
	}

	return false
}

package tools

import (
	"os"
	"strings"
	"sync"
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
// - copilot-agent
// - excel
// - filesystem
// - gemini-agent
// - kiro-agent
// - memory
// - murican_to_english
// - pdf
// - process_document
// - sbom
// - security
// - security_override
// - sequential-thinking
// - shadcn
// - terraform_documentation
// - vulnerability_scan

// cachedEnabledTools is parsed once from the environment on first access.
var (
	cachedEnabledToolsOnce sync.Once
	cachedEnabledTools     map[string]bool
	cachedEnableAll        bool
)

func ensureEnabledToolsParsed() {
	cachedEnabledToolsOnce.Do(func() {
		cachedEnabledTools = make(map[string]bool)
		envVal := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
		if envVal == "" {
			return
		}
		if strings.TrimSpace(strings.ToLower(envVal)) == "all" {
			cachedEnableAll = true
			return
		}
		for tool := range strings.SplitSeq(envVal, ",") {
			normalised := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tool), "_", "-"))
			if normalised != "" {
				cachedEnabledTools[normalised] = true
			}
		}
	})
}

func IsToolEnabled(toolName string) bool {
	ensureEnabledToolsParsed()

	if cachedEnableAll {
		return true
	}
	if len(cachedEnabledTools) == 0 {
		return false
	}

	normalised := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(toolName), "_", "-"))
	return cachedEnabledTools[normalised]
}

// ResetEnabledToolsCache resets the cached enablement state so the environment
// variable is re-parsed on the next call. Intended for testing only.
func ResetEnabledToolsCache() {
	cachedEnabledToolsOnce = sync.Once{}
	cachedEnabledTools = nil
	cachedEnableAll = false
}

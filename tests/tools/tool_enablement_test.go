package tools_test

import (
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/tests/testutils"

	// Import all agent tools to ensure they're registered
	_ "github.com/sammcj/mcp-devtools/internal/tools/claudeagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/codexagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/copilotagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/geminiagent"
	_ "github.com/sammcj/mcp-devtools/internal/tools/kiroagent"
)

// TestTools_DisabledByDefault_DynamicCheck verifies that tools requiring enablement
// are properly excluded from GetEnabledTools() unless ENABLE_ADDITIONAL_TOOLS is set.
// This test would have caught the copilot-agent bug where it was missing from the
// requiresEnablement list in registry.go
func TestTools_DisabledByDefault_DynamicCheck(t *testing.T) {
	// Save original environment
	originalEnabled := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	defer func() {
		if originalEnabled == "" {
			_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		} else {
			_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", originalEnabled)
		}
	}()

	logger := testutils.CreateTestLogger()

	// Step 1: Without ENABLE_ADDITIONAL_TOOLS, find tools that are disabled
	_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
	registry.Init(logger)

	allTools := registry.GetTools()
	enabledToolsWithoutEnv := registry.GetEnabledTools()

	// Find tools that are registered but not enabled (should require enablement)
	var disabledTools []string
	for toolName := range allTools {
		if _, isEnabled := enabledToolsWithoutEnv[toolName]; !isEnabled {
			disabledTools = append(disabledTools, toolName)
		}
	}

	t.Logf("Found %d tools that are disabled by default: %v", len(disabledTools), disabledTools)

	if len(disabledTools) == 0 {
		t.Log("No tools require enablement (all registered tools are enabled by default)")
		return
	}

	// Step 2: For each disabled tool, verify it CAN be enabled with ENABLE_ADDITIONAL_TOOLS
	for _, toolName := range disabledTools {
		t.Run("can_enable_"+toolName, func(t *testing.T) {
			_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", toolName)
			registry.Init(logger)

			newEnabledTools := registry.GetEnabledTools()
			_, nowEnabled := newEnabledTools[toolName]

			if !nowEnabled {
				t.Errorf("FAIL: Tool %q is disabled by default but CANNOT be enabled with ENABLE_ADDITIONAL_TOOLS=%q\n"+
					"  Either:\n"+
					"  1. Missing from requiresEnablement() list in internal/registry/registry.go, OR\n"+
					"  2. Missing tools.IsToolEnabled(%q) check in Execute() method",
					toolName, toolName, toolName)
			}
		})
	}

	// Step 3: CRITICAL - Verify disabled tools are actually disabled by default
	t.Run("disabled_tools_not_in_enabled_list", func(t *testing.T) {
		_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		registry.Init(logger)

		allTools := registry.GetTools()
		enabledTools := registry.GetEnabledTools()

		for _, toolName := range disabledTools {
			// Tool should be registered
			if _, registered := allTools[toolName]; !registered {
				t.Errorf("Tool %q should be registered in GetTools()", toolName)
				continue
			}

			// Tool should NOT be enabled without env var
			if _, enabled := enabledTools[toolName]; enabled {
				t.Errorf("FAIL: Tool %q should NOT be in GetEnabledTools() without ENABLE_ADDITIONAL_TOOLS\n"+
					"  This means it's missing from the requiresEnablement() list in internal/registry/registry.go\n"+
					"  Add %q to the additionalTools slice in requiresEnablement()",
					toolName, toolName)
			}
		}
	})

	// Step 4: CRITICAL - Verify known agent tools are disabled by default
	// This catches the copilot-agent bug where a tool is missing from requiresEnablement list
	t.Run("agent_tools_must_be_disabled_by_default", func(t *testing.T) {
		_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		registry.Init(logger)

		allTools := registry.GetTools()
		enabledTools := registry.GetEnabledTools()

		// These tools MUST require enablement (have tools.IsToolEnabled check in Execute)
		agentToolsThatMustRequireEnablement := []string{
			"claude-agent",
			"codex-agent",
			"copilot-agent",
			"gemini-agent",
			"q-developer-agent",
		}

		for _, agentName := range agentToolsThatMustRequireEnablement {
			// Skip if tool isn't registered (may not be imported in all builds)
			if _, registered := allTools[agentName]; !registered {
				continue
			}

			// Tool MUST NOT be in enabledTools without env var
			if _, enabled := enabledTools[agentName]; enabled {
				t.Errorf("FAIL: Agent tool %q is in GetEnabledTools() without ENABLE_ADDITIONAL_TOOLS\n"+
					"  This means it's missing from the requiresEnablement() list in internal/registry/registry.go\n"+
					"  Add %q to the additionalTools slice in requiresEnablement()",
					agentName, agentName)
			}
		}
	})
}

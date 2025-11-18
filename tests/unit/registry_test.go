package unit_test

import (
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestRegistry_Init(t *testing.T) {
	logger := testutils.CreateTestLogger()

	// Test basic initialisation
	registry.Init(logger)

	// Test that logger is set
	retrievedLogger := registry.GetLogger()
	testutils.AssertNotNil(t, retrievedLogger)

	// Test that cache is set
	cache := registry.GetCache()
	testutils.AssertNotNil(t, cache)
}

func TestRegistry_RegisterAndGetTool(t *testing.T) {
	// Enable the mock tool (mock tools are not in defaultTools list)
	defer testutils.WithEnv(t, "ENABLE_ADDITIONAL_TOOLS", "test-tool")()

	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Create a mock tool
	mockTool := testutils.NewMockTool("test-tool")

	// Register the tool
	registry.Register(mockTool)

	// Test that we can retrieve it
	retrievedTool, ok := registry.GetTool("test-tool")
	testutils.AssertEqual(t, true, ok)
	testutils.AssertNotNil(t, retrievedTool)
	testutils.AssertEqual(t, "test-tool", retrievedTool.Definition().Name)
}

func TestRegistry_GetTool_NotFound(t *testing.T) {
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Test getting a non-existent tool
	_, ok := registry.GetTool("non-existent-tool")
	testutils.AssertEqual(t, false, ok)
}

func TestRegistry_GetTools(t *testing.T) {
	// Enable the mock tools (mock tools are not in defaultTools list)
	defer testutils.WithEnv(t, "ENABLE_ADDITIONAL_TOOLS", "tool-1,tool-2")()

	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Register multiple tools
	tool1 := testutils.NewMockTool("tool-1")
	tool2 := testutils.NewMockTool("tool-2")

	registry.Register(tool1)
	registry.Register(tool2)

	// Get all tools
	tools := registry.GetTools()
	testutils.AssertNotNil(t, tools)

	// Should contain our registered tools (may contain others from real registrations)
	_, ok1 := tools["tool-1"]
	_, ok2 := tools["tool-2"]
	testutils.AssertEqual(t, true, ok1)
	testutils.AssertEqual(t, true, ok2)
}

func TestRegistry_DisabledFunctions(t *testing.T) {
	// Set up disabled functions and enable mock tools
	defer testutils.WithEnv(t, "DISABLED_TOOLS", "disabled-tool,another-disabled-tool")()
	defer testutils.WithEnv(t, "ENABLE_ADDITIONAL_TOOLS", "enabled-tool,disabled-tool")()

	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Register tools, including disabled ones
	enabledTool := testutils.NewMockTool("enabled-tool")
	disabledTool := testutils.NewMockTool("disabled-tool")

	registry.Register(enabledTool)
	registry.Register(disabledTool)

	// Test that enabled tool is available
	_, ok := registry.GetTool("enabled-tool")
	testutils.AssertEqual(t, true, ok)

	// Test that disabled tool is not available (DISABLED_TOOLS has priority over ENABLE_ADDITIONAL_TOOLS)
	_, ok = registry.GetTool("disabled-tool")
	testutils.AssertEqual(t, false, ok)

	// Test that GetTools excludes disabled tools
	tools := registry.GetTools()
	_, enabledExists := tools["enabled-tool"]
	_, disabledExists := tools["disabled-tool"]

	testutils.AssertEqual(t, true, enabledExists)
	testutils.AssertEqual(t, false, disabledExists)
}

func TestRegistry_DisabledFunctions_WithSpaces(t *testing.T) {
	// Save original environment
	originalDisabled := os.Getenv("DISABLED_TOOLS")
	defer func() {
		if originalDisabled == "" {
			_ = os.Unsetenv("DISABLED_TOOLS")
		} else {
			_ = os.Setenv("DISABLED_TOOLS", originalDisabled)
		}
	}()

	// Set up disabled functions with spaces
	_ = os.Setenv("DISABLED_TOOLS", " disabled-tool , another-disabled-tool ")

	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Register disabled tool
	disabledTool := testutils.NewMockTool("disabled-tool")
	registry.Register(disabledTool)

	// Test that disabled tool is not available (spaces should be trimmed)
	_, ok := registry.GetTool("disabled-tool")
	testutils.AssertEqual(t, false, ok)
}

func TestRegistry_DisabledFunctions_Empty(t *testing.T) {
	// Save original environment
	originalDisabled := os.Getenv("DISABLED_TOOLS")
	originalEnabled := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	defer func() {
		if originalDisabled == "" {
			_ = os.Unsetenv("DISABLED_TOOLS")
		} else {
			_ = os.Setenv("DISABLED_TOOLS", originalDisabled)
		}
		if originalEnabled == "" {
			_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		} else {
			_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", originalEnabled)
		}
	}()

	// Set empty disabled functions
	_ = os.Setenv("DISABLED_TOOLS", "")
	// Enable mock tool (mock tools are not in defaultTools list)
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "test-tool")

	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Register tool
	tool := testutils.NewMockTool("test-tool")
	registry.Register(tool)

	// Test that tool is available when DISABLED_TOOLS is empty
	_, ok := registry.GetTool("test-tool")
	testutils.AssertEqual(t, true, ok)
}

func TestRegistry_LegacyDisabledFunctions(t *testing.T) {
	// TODO: This can be removed when DISABLED_FUNCTIONS is fully deprecated
	// Save original environment
	originalDisabled := os.Getenv("DISABLED_TOOLS")
	originalLegacy := os.Getenv("DISABLED_FUNCTIONS")
	originalEnabled := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	defer func() {
		if originalDisabled == "" {
			_ = os.Unsetenv("DISABLED_TOOLS")
		} else {
			_ = os.Setenv("DISABLED_TOOLS", originalDisabled)
		}
		if originalLegacy == "" {
			_ = os.Unsetenv("DISABLED_FUNCTIONS")
		} else {
			_ = os.Setenv("DISABLED_FUNCTIONS", originalLegacy)
		}
		if originalEnabled == "" {
			_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		} else {
			_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", originalEnabled)
		}
	}()

	// Clear DISABLED_TOOLS and set legacy DISABLED_FUNCTIONS
	_ = os.Unsetenv("DISABLED_TOOLS")
	_ = os.Setenv("DISABLED_FUNCTIONS", "legacy-disabled-tool")
	// Enable mock tools (mock tools are not in defaultTools list)
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "enabled-tool,legacy-disabled-tool")

	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Register tools
	enabledTool := testutils.NewMockTool("enabled-tool")
	disabledTool := testutils.NewMockTool("legacy-disabled-tool")
	registry.Register(enabledTool)
	registry.Register(disabledTool)

	// Test that legacy env var works
	_, ok := registry.GetTool("enabled-tool")
	testutils.AssertEqual(t, true, ok)

	_, ok = registry.GetTool("legacy-disabled-tool")
	testutils.AssertEqual(t, false, ok)
}

func TestRegistry_LegacyDisabledFunctions_MergesWithNew(t *testing.T) {
	// TODO: This can be removed when DISABLED_FUNCTIONS is fully deprecated
	// Save original environment
	originalDisabled := os.Getenv("DISABLED_TOOLS")
	originalLegacy := os.Getenv("DISABLED_FUNCTIONS")
	originalEnabled := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	defer func() {
		if originalDisabled == "" {
			_ = os.Unsetenv("DISABLED_TOOLS")
		} else {
			_ = os.Setenv("DISABLED_TOOLS", originalDisabled)
		}
		if originalLegacy == "" {
			_ = os.Unsetenv("DISABLED_FUNCTIONS")
		} else {
			_ = os.Setenv("DISABLED_FUNCTIONS", originalLegacy)
		}
		if originalEnabled == "" {
			_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		} else {
			_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", originalEnabled)
		}
	}()

	// Set both env vars
	_ = os.Setenv("DISABLED_TOOLS", "new-disabled-tool")
	_ = os.Setenv("DISABLED_FUNCTIONS", "legacy-disabled-tool")
	// Enable mock tools (mock tools are not in defaultTools list)
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "enabled-tool,legacy-disabled-tool,new-disabled-tool")

	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Register tools
	enabledTool := testutils.NewMockTool("enabled-tool")
	legacyDisabledTool := testutils.NewMockTool("legacy-disabled-tool")
	newDisabledTool := testutils.NewMockTool("new-disabled-tool")
	registry.Register(enabledTool)
	registry.Register(legacyDisabledTool)
	registry.Register(newDisabledTool)

	// Test that both env vars work together
	_, ok := registry.GetTool("enabled-tool")
	testutils.AssertEqual(t, true, ok)

	_, ok = registry.GetTool("legacy-disabled-tool")
	testutils.AssertEqual(t, false, ok)

	_, ok = registry.GetTool("new-disabled-tool")
	testutils.AssertEqual(t, false, ok)
}

func TestRegistry_Cache_Operations(t *testing.T) {
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	cache := registry.GetCache()
	testutils.AssertNotNil(t, cache)

	// Test basic cache operations
	key := "test-key"
	value := "test-value"

	// Store value
	cache.Store(key, value)

	// Retrieve value
	retrieved, ok := cache.Load(key)
	testutils.AssertEqual(t, true, ok)
	testutils.AssertEqual(t, value, retrieved)

	// Delete value
	cache.Delete(key)

	// Verify deletion
	_, ok = cache.Load(key)
	testutils.AssertEqual(t, false, ok)
}

func TestRegistry_Shared_Resources(t *testing.T) {
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Test that logger and cache are shared across calls
	logger1 := registry.GetLogger()
	logger2 := registry.GetLogger()
	testutils.AssertEqual(t, logger1, logger2)

	cache1 := registry.GetCache()
	cache2 := registry.GetCache()
	testutils.AssertEqual(t, cache1, cache2)
}

func TestRegistry_ShouldRegisterTool(t *testing.T) {
	// Test the ShouldRegisterTool function with various scenarios

	// Save original environment
	originalDisabled := os.Getenv("DISABLED_TOOLS")
	originalEnabled := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	defer func() {
		if originalDisabled == "" {
			_ = os.Unsetenv("DISABLED_TOOLS")
		} else {
			_ = os.Setenv("DISABLED_TOOLS", originalDisabled)
		}
		if originalEnabled == "" {
			_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		} else {
			_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", originalEnabled)
		}
	}()

	logger := testutils.CreateTestLogger()

	t.Run("tool_enabled_by_default", func(t *testing.T) {
		_ = os.Unsetenv("DISABLED_TOOLS")
		_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		registry.Init(logger)

		// Test a tool that's enabled by default (not in requiresEnablement list)
		result := registry.ShouldRegisterTool("internet_search")
		testutils.AssertEqual(t, true, result)
	})

	t.Run("tool_disabled_via_DISABLED_TOOLS", func(t *testing.T) {
		_ = os.Setenv("DISABLED_TOOLS", "internet_search")
		_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		registry.Init(logger)

		// Tool should be blocked by DISABLED_TOOLS (highest priority)
		result := registry.ShouldRegisterTool("internet_search")
		testutils.AssertEqual(t, false, result)
	})

	t.Run("tool_requires_enablement_not_enabled", func(t *testing.T) {
		_ = os.Unsetenv("DISABLED_TOOLS")
		_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		registry.Init(logger)

		// Test a tool that requires enablement but is not enabled
		result := registry.ShouldRegisterTool("kiro-agent")
		testutils.AssertEqual(t, false, result)
	})

	t.Run("tool_requires_enablement_is_enabled", func(t *testing.T) {
		_ = os.Unsetenv("DISABLED_TOOLS")
		_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "kiro-agent")
		registry.Init(logger)

		// Tool requires enablement and is explicitly enabled
		result := registry.ShouldRegisterTool("kiro-agent")
		testutils.AssertEqual(t, true, result)
	})

	t.Run("DISABLED_TOOLS_overrides_ENABLE_ADDITIONAL_TOOLS", func(t *testing.T) {
		_ = os.Setenv("DISABLED_TOOLS", "kiro-agent")
		_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "kiro-agent")
		registry.Init(logger)

		// DISABLED_TOOLS has highest priority
		result := registry.ShouldRegisterTool("kiro-agent")
		testutils.AssertEqual(t, false, result)
	})

	t.Run("multiple_tools_in_ENABLE_ADDITIONAL_TOOLS", func(t *testing.T) {
		_ = os.Unsetenv("DISABLED_TOOLS")
		_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "kiro-agent,claude-agent,gemini-agent")
		registry.Init(logger)

		// All listed tools should be enabled
		testutils.AssertEqual(t, true, registry.ShouldRegisterTool("kiro-agent"))
		testutils.AssertEqual(t, true, registry.ShouldRegisterTool("claude-agent"))
		testutils.AssertEqual(t, true, registry.ShouldRegisterTool("gemini-agent"))

		// Tool not in the list should not be enabled
		testutils.AssertEqual(t, false, registry.ShouldRegisterTool("codex-agent"))
	})
}

func TestRegistry_DisabledByDefault_Tools(t *testing.T) {
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

	// CRITICAL TEST: Dynamically verify that tools requiring enablement are NOT in GetEnabledTools()
	// This would have caught the copilot-agent bug where it was missing from requiresEnablement list
	t.Run("tools_requiring_enablement_are_disabled_by_default", func(t *testing.T) {
		_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		registry.Init(logger)

		allTools := registry.GetTools()
		enabledTools := registry.GetEnabledTools()

		// Find tools that are registered but not enabled (should require enablement)
		var disabledTools []string
		for toolName := range allTools {
			if _, isEnabled := enabledTools[toolName]; !isEnabled {
				disabledTools = append(disabledTools, toolName)
			}
		}

		if len(disabledTools) > 0 {
			t.Logf("Found %d tools that are disabled by default: %v", len(disabledTools), disabledTools)

			// Now verify each disabled tool can be enabled when ENABLE_ADDITIONAL_TOOLS is set
			for _, toolName := range disabledTools {
				_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", toolName)
				registry.Init(logger)

				newEnabledTools := registry.GetEnabledTools()
				if _, nowEnabled := newEnabledTools[toolName]; !nowEnabled {
					t.Errorf("Tool %q is disabled by default but CANNOT be enabled with ENABLE_ADDITIONAL_TOOLS=%q\n"+
						"  This means the tool is missing from requiresEnablement() list in internal/registry/registry.go",
						toolName, toolName)
				}
			}
		} else {
			t.Log("No tools require enablement (all registered tools are enabled by default)")
		}
	})

	// Test that all tools in GetTools() are either in GetEnabledTools() OR can be enabled via env var
	t.Run("all_registered_tools_are_accessible", func(t *testing.T) {
		_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		registry.Init(logger)

		allTools := registry.GetTools()
		enabledTools := registry.GetEnabledTools()

		t.Logf("Total tools registered: %d", len(allTools))
		t.Logf("Tools enabled by default: %d", len(enabledTools))
		t.Logf("Tools requiring enablement: %d", len(allTools)-len(enabledTools))

		// Verify count makes sense
		if len(enabledTools) > len(allTools) {
			t.Errorf("GetEnabledTools() returned more tools (%d) than GetTools() (%d) - this should never happen",
				len(enabledTools), len(allTools))
		}
	})
}

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
	// Save original environment
	originalDisabled := os.Getenv("DISABLED_FUNCTIONS")
	defer func() {
		if originalDisabled == "" {
			_ = os.Unsetenv("DISABLED_FUNCTIONS")
		} else {
			_ = os.Setenv("DISABLED_FUNCTIONS", originalDisabled)
		}
	}()

	// Set up disabled functions
	_ = os.Setenv("DISABLED_FUNCTIONS", "disabled-tool,another-disabled-tool")

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

	// Test that disabled tool is not available
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
	originalDisabled := os.Getenv("DISABLED_FUNCTIONS")
	defer func() {
		if originalDisabled == "" {
			_ = os.Unsetenv("DISABLED_FUNCTIONS")
		} else {
			_ = os.Setenv("DISABLED_FUNCTIONS", originalDisabled)
		}
	}()

	// Set up disabled functions with spaces
	_ = os.Setenv("DISABLED_FUNCTIONS", " disabled-tool , another-disabled-tool ")

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
	originalDisabled := os.Getenv("DISABLED_FUNCTIONS")
	defer func() {
		if originalDisabled == "" {
			_ = os.Unsetenv("DISABLED_FUNCTIONS")
		} else {
			_ = os.Setenv("DISABLED_FUNCTIONS", originalDisabled)
		}
	}()

	// Set empty disabled functions
	_ = os.Setenv("DISABLED_FUNCTIONS", "")

	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Register tool
	tool := testutils.NewMockTool("test-tool")
	registry.Register(tool)

	// Test that tool is available when DISABLED_FUNCTIONS is empty
	_, ok := registry.GetTool("test-tool")
	testutils.AssertEqual(t, true, ok)
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

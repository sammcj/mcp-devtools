package tools_test

import (
	"fmt"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/shadcnui"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestUnifiedShadcnTool_Definition(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	definition := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "shadcn", definition.Name)
	testutils.AssertNotNil(t, definition.Description)

	// Test that description contains key phrases
	desc := definition.Description
	if !testutils.Contains(desc, "shadcn/ui components") || !testutils.Contains(desc, "list") {
		t.Errorf("Expected description to contain key phrases about shadcn/ui components and actions, got: %s", desc)
	}

	// Test input schema exists
	testutils.AssertNotNil(t, definition.InputSchema)
}

func TestUnifiedShadcnTool_Execute_MissingAction(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")
}

func TestUnifiedShadcnTool_Execute_EmptyAction(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action": "",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")
}

func TestUnifiedShadcnTool_Execute_InvalidActionType(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action": 123, // Invalid type
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")
}

func TestUnifiedShadcnTool_Execute_InvalidAction(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action": "invalid",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "invalid action: invalid")
}

func TestUnifiedShadcnTool_Execute_SearchMissingQuery(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action": "search",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "query parameter is required for search action")
}

func TestUnifiedShadcnTool_Execute_SearchEmptyQuery(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action": "search",
		"query":  "",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "query parameter is required for search action")
}

func TestUnifiedShadcnTool_Execute_DetailsMissingComponentName(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action": "details",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "componentName parameter is required for details action")
}

func TestUnifiedShadcnTool_Execute_DetailsEmptyComponentName(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action":        "details",
		"componentName": "",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "componentName parameter is required for details action")
}

func TestUnifiedShadcnTool_Execute_ExamplesMissingComponentName(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action": "examples",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "componentName parameter is required for examples action")
}

func TestUnifiedShadcnTool_Execute_ExamplesEmptyComponentName(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]interface{}{
		"action":        "examples",
		"componentName": "",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "componentName parameter is required for examples action")
}

// Note: Tests that require actual HTTP requests are omitted to avoid external dependencies
// and nil pointer issues with uninitialized HTTP client. The core parameter validation
// logic is already tested through the other test functions above.

// Test utility functions and data structures directly

func TestShadcnUITypes_ComponentInfo(t *testing.T) {
	// Test ComponentInfo structure
	info := shadcnui.ComponentInfo{
		Name:        "button",
		Description: "A clickable button component",
		URL:         "https://ui.shadcn.com/docs/components/button",
	}

	testutils.AssertEqual(t, "button", info.Name)
	testutils.AssertEqual(t, "A clickable button component", info.Description)
	testutils.AssertEqual(t, "https://ui.shadcn.com/docs/components/button", info.URL)
}

func TestShadcnUITypes_ComponentExample(t *testing.T) {
	// Test ComponentExample structure
	example := shadcnui.ComponentExample{
		Title:       "Basic Button",
		Code:        `<Button>Click me</Button>`,
		Description: "A simple button example",
	}

	testutils.AssertEqual(t, "Basic Button", example.Title)
	testutils.AssertEqual(t, `<Button>Click me</Button>`, example.Code)
	testutils.AssertEqual(t, "A simple button example", example.Description)
}

func TestShadcnUITypes_ComponentProp(t *testing.T) {
	// Test ComponentProp structure
	prop := shadcnui.ComponentProp{
		Type:        "variant",
		Description: "The visual style of the button",
		Required:    false,
		Default:     "default",
		Example:     "primary",
	}

	testutils.AssertEqual(t, "variant", prop.Type)
	testutils.AssertEqual(t, "The visual style of the button", prop.Description)
	testutils.AssertEqual(t, false, prop.Required)
	testutils.AssertEqual(t, "default", prop.Default)
	testutils.AssertEqual(t, "primary", prop.Example)
}

// Test caching behavior with mock data
func TestUnifiedShadcnTool_CacheBehavior(t *testing.T) {
	cache := testutils.CreateTestCache()

	// Test that cache operations don't cause errors
	// We can't test the full flow without mocking HTTP, but we can test cache structure

	// Simulate cache operations
	testData := []shadcnui.ComponentInfo{
		{
			Name: "button",
			URL:  "https://ui.shadcn.com/docs/components/button",
		},
		{
			Name: "input",
			URL:  "https://ui.shadcn.com/docs/components/input",
		},
	}

	// Store test data in cache
	cache.Store("shadcnui:list_components", shadcnui.CacheEntry{
		Data: testData,
	})

	// Verify cache storage
	if cachedData, ok := cache.Load("shadcnui:list_components"); ok {
		entry := cachedData.(shadcnui.CacheEntry)
		components := entry.Data.([]shadcnui.ComponentInfo)
		testutils.AssertEqual(t, 2, len(components))
		testutils.AssertEqual(t, "button", components[0].Name)
		testutils.AssertEqual(t, "input", components[1].Name)
	} else {
		t.Error("Expected cached data to be found")
	}
}

// Test action parameter validation comprehensively
func TestUnifiedShadcnTool_ActionValidation(t *testing.T) {
	invalidActions := []string{"invalid", "notfound", "wrong", ""}

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test invalid actions (should fail on validation before HTTP calls)
	for _, action := range invalidActions {
		args := map[string]interface{}{
			"action": action,
		}

		_, err := tool.Execute(ctx, logger, cache, args)

		testutils.AssertError(t, err)
		if action == "" {
			testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")
		} else {
			testutils.AssertErrorContains(t, err, fmt.Sprintf("invalid action: %s", action))
		}
	}
}

// Test parameter type validation
func TestUnifiedShadcnTool_ParameterTypes(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test invalid action type
	args := map[string]interface{}{
		"action": 123,
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")

	// Test invalid query type for search
	args = map[string]interface{}{
		"action": "search",
		"query":  123,
	}

	_, err = tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "query parameter is required for search action")

	// Test invalid componentName type for details
	args = map[string]interface{}{
		"action":        "details",
		"componentName": 123,
	}

	_, err = tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "componentName parameter is required for details action")

	// Test invalid componentName type for examples
	args = map[string]interface{}{
		"action":        "examples",
		"componentName": []string{"test"},
	}

	_, err = tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "componentName parameter is required for examples action")
}

// Test edge cases
func TestUnifiedShadcnTool_EdgeCases(t *testing.T) {
	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test with nil arguments
	_, err := tool.Execute(ctx, logger, cache, nil)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")

	// Test with empty arguments
	args := map[string]interface{}{}
	_, err = tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")

	// Note: Tests for whitespace-only parameters are omitted as they may trigger
	// HTTP calls depending on the validation logic implementation.
}

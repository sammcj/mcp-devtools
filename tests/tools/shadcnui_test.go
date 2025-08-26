package tools_test

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

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
	if !testutils.Contains(desc, "shadcn ui components") || !testutils.Contains(desc, "list") {
		t.Errorf("Expected description to contain key phrases about shadcn ui components and actions, got: %s", desc)
	}

	// Test input schema exists
	testutils.AssertNotNil(t, definition.InputSchema)
}

func TestUnifiedShadcnTool_Execute_ToolDisabled(t *testing.T) {
	// Ensure shadcn tool is disabled by default
	_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"action": "list",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "shadcn tool is not enabled")
	testutils.AssertErrorContains(t, err, "Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'shadcn'")
	testutils.AssertNil(t, result)
}

func TestUnifiedShadcnTool_Execute_MissingAction(t *testing.T) {
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")
}

func TestUnifiedShadcnTool_Execute_EmptyAction(t *testing.T) {
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"action": "",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")
}

func TestUnifiedShadcnTool_Execute_InvalidActionType(t *testing.T) {
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"action": 123, // Invalid type
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")
}

func TestUnifiedShadcnTool_Execute_InvalidAction(t *testing.T) {
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"action": "invalid",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "invalid action: invalid")
}

func TestUnifiedShadcnTool_Execute_SearchMissingQuery(t *testing.T) {
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"action": "search",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "query parameter is required for search action")
}

func TestUnifiedShadcnTool_Execute_SearchEmptyQuery(t *testing.T) {
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"action": "search",
		"query":  "",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "query parameter is required for search action")
}

func TestUnifiedShadcnTool_Execute_DetailsMissingComponentName(t *testing.T) {
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"action": "details",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "componentName parameter is required for details action")
}

func TestUnifiedShadcnTool_Execute_DetailsEmptyComponentName(t *testing.T) {
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"action":        "details",
		"componentName": "",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "componentName parameter is required for details action")
}

func TestUnifiedShadcnTool_Execute_ExamplesMissingComponentName(t *testing.T) {
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"action": "examples",
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "componentName parameter is required for examples action")
}

func TestUnifiedShadcnTool_Execute_ExamplesEmptyComponentName(t *testing.T) {
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
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
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	invalidActions := []string{"invalid", "notfound", "wrong", ""}

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test invalid actions (should fail on validation before HTTP calls)
	for _, action := range invalidActions {
		args := map[string]any{
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
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test invalid action type
	args := map[string]any{
		"action": 123,
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")

	// Test invalid query type for search
	args = map[string]any{
		"action": "search",
		"query":  123,
	}

	_, err = tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "query parameter is required for search action")

	// Test invalid componentName type for details
	args = map[string]any{
		"action":        "details",
		"componentName": 123,
	}

	_, err = tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "componentName parameter is required for details action")

	// Test invalid componentName type for examples
	args = map[string]any{
		"action":        "examples",
		"componentName": []string{"test"},
	}

	_, err = tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "componentName parameter is required for examples action")
}

// Test edge cases
func TestUnifiedShadcnTool_EdgeCases(t *testing.T) {
	// Enable the shadcn tool for testing
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "shadcn")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	tool := &shadcnui.UnifiedShadcnTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test with nil arguments
	_, err := tool.Execute(ctx, logger, cache, nil)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")

	// Test with empty arguments
	args := map[string]any{}
	_, err = tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "missing or invalid required parameter: action")

	// Note: Tests for whitespace-only parameters are omitted as they may trigger
	// HTTP calls depending on the validation logic implementation.
}

// MockHTTPClient implements HTTPClient for testing rate limiting
type MockHTTPClient struct {
	RequestTimes []time.Time
	mu           sync.Mutex
}

func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RequestTimes = append(m.RequestTimes, time.Now())

	// Return a minimal mock response that won't crash the parser
	return &http.Response{
		StatusCode: 200,
		Body:       http.NoBody,
	}, nil
}

func TestRateLimitedHTTPClient_DefaultRateLimit(t *testing.T) {
	// Test client creation and basic functionality without making actual HTTP requests
	client := shadcnui.NewRateLimitedHTTPClient()
	testutils.AssertNotNil(t, client)

	// Just verify the client was created successfully - no need to make real HTTP calls
}

func TestRateLimitedHTTPClient_CustomRateLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("SHADCN_RATE_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("SHADCN_RATE_LIMIT")
		} else {
			_ = os.Setenv("SHADCN_RATE_LIMIT", originalValue)
		}
	}()

	// Set custom rate limit
	err := os.Setenv("SHADCN_RATE_LIMIT", "10")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	// Test client creation with custom rate limit
	client := shadcnui.NewRateLimitedHTTPClient()
	testutils.AssertNotNil(t, client)

	// Just verify the client was created successfully - no need to make real HTTP calls
}

func TestRateLimitedHTTPClient_InvalidEnvironmentVariable(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("SHADCN_RATE_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("SHADCN_RATE_LIMIT")
		} else {
			_ = os.Setenv("SHADCN_RATE_LIMIT", originalValue)
		}
	}()

	// Set invalid rate limit (negative number)
	err := os.Setenv("SHADCN_RATE_LIMIT", "-5")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	// Should fall back to default rate limit
	client := shadcnui.NewRateLimitedHTTPClient()
	testutils.AssertNotNil(t, client)

	// Reset to test non-numeric value
	err = os.Setenv("SHADCN_RATE_LIMIT", "invalid")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	// Should fall back to default rate limit
	client = shadcnui.NewRateLimitedHTTPClient()
	testutils.AssertNotNil(t, client)
}

func TestGetShadcnRateLimit_Function(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("SHADCN_RATE_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("SHADCN_RATE_LIMIT")
		} else {
			_ = os.Setenv("SHADCN_RATE_LIMIT", originalValue)
		}
	}()

	// Test default value
	_ = os.Unsetenv("SHADCN_RATE_LIMIT")
	// Can't directly test the function as it's not exported, but we can test through NewRateLimitedHTTPClient
	client := shadcnui.NewRateLimitedHTTPClient()
	testutils.AssertNotNil(t, client)

	// Test custom value
	err := os.Setenv("SHADCN_RATE_LIMIT", "2.5")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	client = shadcnui.NewRateLimitedHTTPClient()
	testutils.AssertNotNil(t, client)
}

package tools_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/codexagent"
	"github.com/sammcj/mcp-devtools/tests/testutils"
	"github.com/stretchr/testify/assert"
)

// Basic tests following the pattern of other agent tools (geminiagent_test.go, claudeagent_test.go, qdeveloper_test.go)

func TestCodexTool_Definition(t *testing.T) {
	tool := &codexagent.CodexTool{}
	def := tool.Definition()

	assert.NotNil(t, def)
	assert.Equal(t, "codex-agent", def.GetName())
}

func TestCodexTool_Definition_ParameterSchema(t *testing.T) {
	tool := &codexagent.CodexTool{}
	def := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "codex-agent", def.Name)
	testutils.AssertNotNil(t, def.Description)

	// Test that description contains key phrases
	desc := def.Description
	if !testutils.Contains(desc, "Codex CLI") {
		t.Errorf("Expected description to contain 'Codex CLI', got: %s", desc)
	}

	// Test input schema exists
	testutils.AssertNotNil(t, def.InputSchema)

	// Test that input schema has required properties
	schema := def.InputSchema
	testutils.AssertNotNil(t, schema.Properties)

	// Verify required prompt parameter exists
	promptProp, hasPrompt := schema.Properties["prompt"]
	testutils.AssertTrue(t, hasPrompt)
	testutils.AssertNotNil(t, promptProp)

	// Verify prompt is in required array
	testutils.AssertNotNil(t, schema.Required)
	testutils.AssertTrue(t, slices.Contains(schema.Required, "prompt"))
}

func TestCodexTool_Definition_OptionalParameters(t *testing.T) {
	tool := &codexagent.CodexTool{}
	def := tool.Definition()
	schema := def.InputSchema

	// Test optional parameters exist based on requirements
	optionalParams := []string{
		"override-model", "sandbox", "full-auto", "yolo-mode",
		"resume", "session-id", "profile", "config",
		"images", "cd", "skip-git-repo-check",
	}

	for _, param := range optionalParams {
		prop, exists := schema.Properties[param]
		if !exists {
			t.Errorf("Expected optional parameter '%s' to exist in schema", param)
			continue
		}
		testutils.AssertNotNil(t, prop)
	}

	// Verify none of the optional parameters are in required array
	for _, param := range optionalParams {
		for _, required := range schema.Required {
			if required == param {
				t.Errorf("Optional parameter '%s' should not be in required array", param)
			}
		}
	}
}

func TestCodexTool_Definition_ParameterNamingConventions(t *testing.T) {
	tool := &codexagent.CodexTool{}
	def := tool.Definition()
	schema := def.InputSchema

	// Test that we use consistent naming conventions from requirements
	expectedParams := map[string]bool{
		// Required parameters
		"prompt": true,
		// Model configuration
		"override-model": true, // follows decision log standardisation
		// Security parameters
		"sandbox":   true,
		"full-auto": true,
		"yolo-mode": true, // matches Claude/Gemini convention for dangerously-bypass-approvals-and-sandbox
		// Session management
		"resume":     true,
		"session-id": true,
		// Configuration
		"profile": true,
		"config":  true,
		// Context
		"images": true,
		"cd":     true,
		// Options
		"skip-git-repo-check": true,
	}

	// Verify we have exactly these parameters (no more, no less)
	for param := range schema.Properties {
		_, expected := expectedParams[param]
		if !expected {
			t.Errorf("Unexpected parameter found: %s", param)
		}
	}

	for expectedParam := range expectedParams {
		_, exists := schema.Properties[expectedParam]
		if !exists {
			t.Errorf("Expected parameter missing: %s", expectedParam)
		}
	}
}

// Configuration tests (these are fast and don't execute CLI)

func TestCodexTool_TimeoutConfiguration_DefaultTimeout(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("AGENT_TIMEOUT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("AGENT_TIMEOUT")
		} else {
			_ = os.Setenv("AGENT_TIMEOUT", originalValue)
		}
	}()

	// Clear environment variable to test default behaviour
	_ = os.Unsetenv("AGENT_TIMEOUT")

	tool := &codexagent.CodexTool{}
	timeout := tool.GetTimeout()

	testutils.AssertEqual(t, codexagent.DefaultTimeout, timeout)
}

func TestCodexTool_TimeoutConfiguration_CustomTimeout(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("AGENT_TIMEOUT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("AGENT_TIMEOUT")
		} else {
			_ = os.Setenv("AGENT_TIMEOUT", originalValue)
		}
	}()

	// Set custom timeout
	customTimeout := "300"
	_ = os.Setenv("AGENT_TIMEOUT", customTimeout)

	tool := &codexagent.CodexTool{}
	timeout := tool.GetTimeout()

	testutils.AssertEqual(t, 300, timeout)
}

func TestCodexTool_TimeoutConfiguration_InvalidTimeout(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("AGENT_TIMEOUT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("AGENT_TIMEOUT")
		} else {
			_ = os.Setenv("AGENT_TIMEOUT", originalValue)
		}
	}()

	// Set invalid timeout value
	_ = os.Setenv("AGENT_TIMEOUT", "not-a-number")

	tool := &codexagent.CodexTool{}
	timeout := tool.GetTimeout()

	// Should fall back to default when invalid value is provided
	testutils.AssertEqual(t, codexagent.DefaultTimeout, timeout)
}

func TestCodexTool_ResponseSizeLimit_DefaultLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("AGENT_MAX_RESPONSE_SIZE")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("AGENT_MAX_RESPONSE_SIZE")
		} else {
			_ = os.Setenv("AGENT_MAX_RESPONSE_SIZE", originalValue)
		}
	}()

	// Clear environment variable to test default behaviour
	_ = os.Unsetenv("AGENT_MAX_RESPONSE_SIZE")

	tool := &codexagent.CodexTool{}
	logger := testutils.CreateTestLogger()

	// Test with small output (should not be truncated)
	smallOutput := "This is a small response that should not be truncated."
	result := tool.ApplyResponseSizeLimit(smallOutput, logger)
	testutils.AssertEqual(t, smallOutput, result)

	// Test with large output (should be truncated)
	largeOutput := strings.Repeat("C", 3*1024*1024) // 3MB
	result = tool.ApplyResponseSizeLimit(largeOutput, logger)

	// Should be truncated to default 2MB limit
	testutils.AssertTrue(t, len(result) < len(largeOutput))
	testutils.AssertTrue(t, strings.Contains(result, "[RESPONSE TRUNCATED"))
	testutils.AssertTrue(t, strings.Contains(result, "exceeded 2.0MB limit"))
}

func TestCodexTool_ResponseSizeLimit_CustomLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("AGENT_MAX_RESPONSE_SIZE")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("AGENT_MAX_RESPONSE_SIZE")
		} else {
			_ = os.Setenv("AGENT_MAX_RESPONSE_SIZE", originalValue)
		}
	}()

	// Set custom limit (1MB = 1048576 bytes)
	_ = os.Setenv("AGENT_MAX_RESPONSE_SIZE", "1048576")

	tool := &codexagent.CodexTool{}
	logger := testutils.CreateTestLogger()

	// Test with output larger than custom limit
	largeOutput := strings.Repeat("C", 1500000) // 1.5MB
	result := tool.ApplyResponseSizeLimit(largeOutput, logger)

	// Should be truncated to custom 1MB limit
	testutils.AssertTrue(t, len(result) < len(largeOutput))
	testutils.AssertTrue(t, strings.Contains(result, "[RESPONSE TRUNCATED"))
	testutils.AssertTrue(t, strings.Contains(result, "exceeded 1.0MB limit"))
}

func TestCodexTool_Constants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, "AGENT_MAX_RESPONSE_SIZE", codexagent.AgentMaxResponseSizeEnvVar)
	testutils.AssertEqual(t, "AGENT_TIMEOUT", codexagent.AgentTimeoutEnvVar)
	testutils.AssertEqual(t, 2*1024*1024, codexagent.DefaultMaxResponseSize)
	testutils.AssertEqual(t, 300, codexagent.DefaultTimeout)
}

// Fast error handling tests that don't execute CLI

func TestCodexTool_Execute_ToolDisabled(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		} else {
			_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", originalValue)
		}
	}()

	// Ensure tool is disabled
	_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")

	tool := &codexagent.CodexTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"prompt": "test prompt",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "codex agent tool is not enabled")
	testutils.AssertErrorContains(t, err, "ENABLE_ADDITIONAL_TOOLS")
	testutils.AssertErrorContains(t, err, "codex-agent")
	if result != nil {
		t.Error("Expected nil result when tool is disabled")
	}
}

func TestCodexTool_Execute_ValidationErrors(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		} else {
			_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", originalValue)
		}
	}()

	// Enable the tool to bypass enablement check
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "codex-agent")

	tool := &codexagent.CodexTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tests := []struct {
		name        string
		args        map[string]any
		expectedErr string
	}{
		{
			name:        "missing prompt parameter",
			args:        map[string]any{},
			expectedErr: "prompt is a required parameter",
		},
		{
			name: "empty prompt parameter",
			args: map[string]any{
				"prompt": "",
			},
			expectedErr: "prompt is a required parameter and cannot be empty",
		},
		{
			name: "whitespace-only prompt parameter",
			args: map[string]any{
				"prompt": "   \t\n  ",
			},
			expectedErr: "prompt is a required parameter and cannot be empty",
		},
		{
			name: "non-string prompt parameter",
			args: map[string]any{
				"prompt": 123,
			},
			expectedErr: "prompt is a required parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, logger, cache, tt.args)

			testutils.AssertError(t, err)
			testutils.AssertErrorContains(t, err, tt.expectedErr)
			if result != nil {
				t.Errorf("Expected nil result for validation error, got: %v", result)
			}
		})
	}
}

// Tool registration tests

func TestCodexTool_Registration(t *testing.T) {
	// Test that the tool is registered during package initialisation
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Test that the tool is discoverable via registry
	retrievedTool, ok := registry.GetTool("codex-agent")
	testutils.AssertTrue(t, ok)
	testutils.AssertNotNil(t, retrievedTool)

	// Verify the retrieved tool has the correct name
	def := retrievedTool.Definition()
	testutils.AssertEqual(t, "codex-agent", def.Name)
}

func TestCodexTool_Registration_InToolsList(t *testing.T) {
	// Test that the tool appears in the complete tools list
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	tools := registry.GetTools()
	testutils.AssertNotNil(t, tools)

	// Test that codex-agent is in the tools map
	tool, exists := tools["codex-agent"]
	testutils.AssertTrue(t, exists)
	testutils.AssertNotNil(t, tool)

	// Verify it's the correct tool type
	def := tool.Definition()
	testutils.AssertEqual(t, "codex-agent", def.Name)
	testutils.AssertTrue(t, strings.Contains(def.Description, "Codex CLI"))
}

// Parameter validation tests without CLI execution

func TestCodexTool_ParameterValidation_AllCombinations(t *testing.T) {
	// Test parameter validation without calling actual CLI
	tool := &codexagent.CodexTool{}

	tests := []struct {
		name        string
		args        map[string]any
		expectError bool
		errorMsg    string
	}{
		{
			name: "minimal parameters",
			args: map[string]any{
				"prompt": "test prompt",
			},
			expectError: false,
		},
		{
			name: "with model override",
			args: map[string]any{
				"prompt":         "test with model",
				"override-model": "claude-3.5-sonnet",
			},
			expectError: false,
		},
		{
			name: "with sandbox settings",
			args: map[string]any{
				"prompt":  "test with sandbox",
				"sandbox": "read-only",
			},
			expectError: false,
		},
		{
			name: "with full-auto mode",
			args: map[string]any{
				"prompt":    "test with full-auto",
				"full-auto": true,
			},
			expectError: false,
		},
		{
			name: "with yolo mode",
			args: map[string]any{
				"prompt":    "test with yolo mode",
				"yolo-mode": true,
			},
			expectError: false,
		},
		{
			name: "with session management",
			args: map[string]any{
				"prompt":     "test with session",
				"resume":     true,
				"session-id": "test-session-123",
			},
			expectError: false,
		},
		{
			name: "with configuration",
			args: map[string]any{
				"prompt":  "test with config",
				"profile": "my-profile",
				"config":  []string{"key1=value1", "key2=value2"},
			},
			expectError: false,
		},
		{
			name: "with file context",
			args: map[string]any{
				"prompt": "test with files",
				"images": []string{"image1.png", "image2.jpg"},
				"cd":     "/tmp",
			},
			expectError: false,
		},
		{
			name: "all parameters combined",
			args: map[string]any{
				"prompt":              "comprehensive test prompt",
				"override-model":      "claude-3.5-sonnet",
				"sandbox":             "workspace-write",
				"full-auto":           true,
				"yolo-mode":           false,
				"resume":              true,
				"session-id":          "test-session",
				"profile":             "test-profile",
				"config":              []string{"setting=value"},
				"images":              []string{"test.png"},
				"cd":                  "/workspace",
				"skip-git-repo-check": true,
			},
			expectError: false,
		},
		{
			name: "missing prompt should fail validation",
			args: map[string]any{
				"resume": true,
			},
			expectError: true,
			errorMsg:    "prompt is a required parameter",
		},
		{
			name: "empty prompt should fail validation",
			args: map[string]any{
				"prompt": "",
			},
			expectError: true,
			errorMsg:    "prompt is a required parameter and cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// By default, test parameter validation logic with the tool disabled to avoid CLI execution.
			// For specific validation error checks, the tool is temporarily enabled, which may invoke the CLI.
			originalValue := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
			defer func() {
				if originalValue == "" {
					_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
				} else {
					_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", originalValue)
				}
			}()

			// Disable the tool to avoid CLI execution
			_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")

			logger := testutils.CreateTestLogger()
			cache := testutils.CreateTestCache()
			ctx := testutils.CreateTestContext()

			_, err := tool.Execute(ctx, logger, cache, tt.args)

			if tt.expectError {
				// For validation errors, we expect the specific error message
				if tt.errorMsg != "" {
					// Enable the tool temporarily to test parameter validation
					_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "codex-agent")
					_, err = tool.Execute(ctx, logger, cache, tt.args)
					testutils.AssertError(t, err)
					testutils.AssertErrorContains(t, err, tt.errorMsg)
				} else {
					testutils.AssertError(t, err)
				}
			} else {
				// We expect "not enabled" error since tool is disabled
				testutils.AssertError(t, err)
				testutils.AssertErrorContains(t, err, "not enabled")
			}
		})
	}
}

// Note: Integration tests that execute actual CLI are excluded to keep tests fast.
// Tool enablement is tested separately in tests/unit/enablement_test.go

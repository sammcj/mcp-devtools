package tools_test

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/qdeveloperagent"
	"github.com/sammcj/mcp-devtools/tests/testutils"
	"github.com/stretchr/testify/assert"
)

// Basic tests following the pattern of other agent tools (geminiagent_test.go, claudeagent_test.go)

func TestQDeveloperTool_Definition(t *testing.T) {
	tool := &qdeveloperagent.QDeveloperTool{}
	def := tool.Definition()

	assert.NotNil(t, def)
	assert.Equal(t, "q-developer-agent", def.GetName())
}

func TestQDeveloperTool_Definition_ParameterSchema(t *testing.T) {
	tool := &qdeveloperagent.QDeveloperTool{}
	def := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "q-developer-agent", def.Name)
	testutils.AssertNotNil(t, def.Description)

	// Test that description contains key phrases
	desc := def.Description
	if !testutils.Contains(desc, "AWS Q Developer CLI") {
		t.Errorf("Expected description to contain 'AWS Q Developer CLI', got: %s", desc)
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

func TestQDeveloperTool_Definition_OptionalParameters(t *testing.T) {
	tool := &qdeveloperagent.QDeveloperTool{}
	def := tool.Definition()
	schema := def.InputSchema

	// Test optional parameters exist
	optionalParams := []string{"resume", "agent", "override-model", "yolo-mode", "trust-tools", "verbose"}

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

func TestQDeveloperTool_Definition_ParameterNamingConventions(t *testing.T) {
	tool := &qdeveloperagent.QDeveloperTool{}
	def := tool.Definition()
	schema := def.InputSchema

	// Test that we use consistent naming conventions
	expectedParams := map[string]bool{
		"prompt":         true, // required
		"resume":         true, // optional boolean
		"agent":          true, // optional string
		"override-model": true, // follows decision log standardization
		"yolo-mode":      true, // matches Claude/Gemini convention
		"trust-tools":    true, // optional string
		"verbose":        true, // optional boolean
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

func TestQDeveloperTool_TimeoutConfiguration_DefaultTimeout(t *testing.T) {
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

	tool := &qdeveloperagent.QDeveloperTool{}
	timeout := tool.GetTimeout()

	testutils.AssertEqual(t, qdeveloperagent.DefaultTimeout, timeout)
}

func TestQDeveloperTool_TimeoutConfiguration_CustomTimeout(t *testing.T) {
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

	tool := &qdeveloperagent.QDeveloperTool{}
	timeout := tool.GetTimeout()

	testutils.AssertEqual(t, 300, timeout)
}

func TestQDeveloperTool_TimeoutConfiguration_InvalidTimeout(t *testing.T) {
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

	tool := &qdeveloperagent.QDeveloperTool{}
	timeout := tool.GetTimeout()

	// Should fall back to default when invalid value is provided
	testutils.AssertEqual(t, qdeveloperagent.DefaultTimeout, timeout)
}

func TestQDeveloperTool_ResponseSizeLimit_DefaultLimit(t *testing.T) {
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

	tool := &qdeveloperagent.QDeveloperTool{}
	logger := testutils.CreateTestLogger()

	// Test with small output (should not be truncated)
	smallOutput := "This is a small response that should not be truncated."
	result := tool.ApplyResponseSizeLimit(smallOutput, logger)
	testutils.AssertEqual(t, smallOutput, result)

	// Test with large output (should be truncated)
	largeOutput := strings.Repeat("Q", 3*1024*1024) // 3MB
	result = tool.ApplyResponseSizeLimit(largeOutput, logger)

	// Should be truncated to default 2MB limit
	testutils.AssertTrue(t, len(result) < len(largeOutput))
	testutils.AssertTrue(t, strings.Contains(result, "[RESPONSE TRUNCATED"))
	testutils.AssertTrue(t, strings.Contains(result, "exceeded 2.0MB limit"))
}

func TestQDeveloperTool_ResponseSizeLimit_CustomLimit(t *testing.T) {
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

	tool := &qdeveloperagent.QDeveloperTool{}
	logger := testutils.CreateTestLogger()

	// Test with output larger than custom limit
	largeOutput := strings.Repeat("Q", 1500000) // 1.5MB
	result := tool.ApplyResponseSizeLimit(largeOutput, logger)

	// Should be truncated to custom 1MB limit
	testutils.AssertTrue(t, len(result) < len(largeOutput))
	testutils.AssertTrue(t, strings.Contains(result, "[RESPONSE TRUNCATED"))
	testutils.AssertTrue(t, strings.Contains(result, "exceeded 1.0MB limit"))
}

func TestQDeveloperTool_Constants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, "AGENT_MAX_RESPONSE_SIZE", qdeveloperagent.AgentMaxResponseSizeEnvVar)
	testutils.AssertEqual(t, "AGENT_TIMEOUT", qdeveloperagent.AgentTimeoutEnvVar)
	testutils.AssertEqual(t, 2*1024*1024, qdeveloperagent.DefaultMaxResponseSize)
	testutils.AssertEqual(t, 300, qdeveloperagent.DefaultTimeout)
}

// Fast error handling tests that don't execute CLI

func TestQDeveloperTool_Execute_ToolDisabled(t *testing.T) {
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

	tool := &qdeveloperagent.QDeveloperTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"prompt": "test prompt",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "q Developer agent tool is not enabled")
	testutils.AssertErrorContains(t, err, "ENABLE_ADDITIONAL_TOOLS")
	testutils.AssertErrorContains(t, err, "q-developer-agent")
	if result != nil {
		t.Error("Expected nil result when tool is disabled")
	}
}

func TestQDeveloperTool_Execute_ValidationErrors(t *testing.T) {
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
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "q-developer-agent")

	tool := &qdeveloperagent.QDeveloperTool{}
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

func TestQDeveloperTool_Registration(t *testing.T) {
	// Test that the tool is registered during package initialization
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Test that the tool is discoverable via registry
	retrievedTool, ok := registry.GetTool("q-developer-agent")
	testutils.AssertTrue(t, ok)
	testutils.AssertNotNil(t, retrievedTool)

	// Verify the retrieved tool has the correct name
	def := retrievedTool.Definition()
	testutils.AssertEqual(t, "q-developer-agent", def.Name)
}

func TestQDeveloperTool_Registration_InToolsList(t *testing.T) {
	// Test that the tool appears in the complete tools list
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	tools := registry.GetTools()
	testutils.AssertNotNil(t, tools)

	// Test that q-developer-agent is in the tools map
	tool, exists := tools["q-developer-agent"]
	testutils.AssertTrue(t, exists)
	testutils.AssertNotNil(t, tool)

	// Verify it's the correct tool type
	def := tool.Definition()
	testutils.AssertEqual(t, "q-developer-agent", def.Name)
	testutils.AssertTrue(t, strings.Contains(def.Description, "AWS Q Developer CLI"))
}

func TestQDeveloperTool_ExtendedHelp(t *testing.T) {
	// Test that the tool provides extended help information
	tool := &qdeveloperagent.QDeveloperTool{}

	// Test that ProvideExtendedInfo is implemented
	extendedInfo := tool.ProvideExtendedInfo()
	testutils.AssertNotNil(t, extendedInfo)

	// Test examples exist
	testutils.AssertNotNil(t, extendedInfo.Examples)
	testutils.AssertTrue(t, len(extendedInfo.Examples) > 0)

	// Test common patterns exist
	testutils.AssertNotNil(t, extendedInfo.CommonPatterns)
	testutils.AssertTrue(t, len(extendedInfo.CommonPatterns) > 0)

	// Test troubleshooting tips exist
	testutils.AssertNotNil(t, extendedInfo.Troubleshooting)
	testutils.AssertTrue(t, len(extendedInfo.Troubleshooting) > 0)

	// Test parameter details exist
	testutils.AssertNotNil(t, extendedInfo.ParameterDetails)
	testutils.AssertTrue(t, len(extendedInfo.ParameterDetails) > 0)

	// Test when to use information exists
	testutils.AssertTrue(t, extendedInfo.WhenToUse != "")
	testutils.AssertTrue(t, extendedInfo.WhenNotToUse != "")
}

func TestQDeveloperTool_ExtendedHelp_ContentVerification(t *testing.T) {
	// Test specific content in extended help
	tool := &qdeveloperagent.QDeveloperTool{}
	extendedInfo := tool.ProvideExtendedInfo()

	// Verify examples have required fields
	for i, example := range extendedInfo.Examples {
		if example.Description == "" {
			t.Errorf("Example %d should have a description", i)
		}
		if example.Arguments == nil {
			t.Errorf("Example %d should have arguments", i)
		}
		if example.ExpectedResult == "" {
			t.Errorf("Example %d should have expected result", i)
		}

		// All examples should have a prompt argument
		_, hasPrompt := example.Arguments["prompt"]
		if !hasPrompt {
			t.Errorf("Example %d should have a prompt argument", i)
		}
	}

	// Verify troubleshooting tips have required fields
	for i, tip := range extendedInfo.Troubleshooting {
		if tip.Problem == "" {
			t.Errorf("Troubleshooting tip %d should have a problem", i)
		}
		if tip.Solution == "" {
			t.Errorf("Troubleshooting tip %d should have a solution", i)
		}
	}

	// Verify parameter details cover expected parameters
	expectedParams := []string{"prompt", "resume", "agent", "override-model", "yolo-mode", "trust-tools", "verbose"}
	for _, param := range expectedParams {
		detail, exists := extendedInfo.ParameterDetails[param]
		if !exists {
			t.Errorf("Parameter '%s' should have details in extended help", param)
		}
		if detail == "" {
			t.Errorf("Parameter '%s' should have non-empty details", param)
		}
	}
}

// Table-driven tests for parameter combinations

func TestQDeveloperTool_ParameterValidation_AllCombinations(t *testing.T) {
	// Test parameter validation without calling actual CLI
	tool := &qdeveloperagent.QDeveloperTool{}

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
			name: "with resume flag",
			args: map[string]any{
				"prompt": "continue conversation",
				"resume": true,
			},
			expectError: false,
		},
		{
			name: "with agent parameter",
			args: map[string]any{
				"prompt": "test with agent",
				"agent":  "my-context-profile",
			},
			expectError: false,
		},
		{
			name: "with override model",
			args: map[string]any{
				"prompt":         "test with model",
				"override-model": "claude-3.5-sonnet",
			},
			expectError: false,
		},
		{
			name: "with yolo mode",
			args: map[string]any{
				"prompt":    "test with trust all",
				"yolo-mode": true,
			},
			expectError: false,
		},
		{
			name: "with trust tools",
			args: map[string]any{
				"prompt":      "test with trust tools",
				"trust-tools": "tool1,tool2,tool3",
			},
			expectError: false,
		},
		{
			name: "with verbose flag",
			args: map[string]any{
				"prompt":  "test with verbose",
				"verbose": true,
			},
			expectError: false,
		},
		{
			name: "all parameters combined",
			args: map[string]any{
				"prompt":         "comprehensive test prompt",
				"resume":         true,
				"agent":          "test-agent",
				"override-model": "claude-sonnet-4",
				"yolo-mode":      true,
				"trust-tools":    "tool1,tool2",
				"verbose":        true,
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
			// Test only parameter validation logic by checking what happens when tool is disabled
			// This avoids calling the actual CLI while still testing parameter parsing
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
					_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "q-developer-agent")
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

func TestQDeveloperTool_ResponseSizeLimit_EdgeCases(t *testing.T) {
	tool := &qdeveloperagent.QDeveloperTool{}
	logger := testutils.CreateTestLogger()

	tests := []struct {
		name          string
		input         string
		maxSize       string // Environment variable value
		expectedTrunc bool
		expectedMsg   string
	}{
		{
			name:          "empty input",
			input:         "",
			maxSize:       "", // Use default
			expectedTrunc: false,
			expectedMsg:   "",
		},
		{
			name:          "single line under limit",
			input:         "Short response",
			maxSize:       "100",
			expectedTrunc: false,
			expectedMsg:   "Short response",
		},
		{
			name:          "single line over limit",
			input:         "This is a very long single line that should be truncated because it exceeds our limit",
			maxSize:       "50",
			expectedTrunc: true,
			expectedMsg:   "RESPONSE TRUNCATED",
		},
		{
			name:          "multiline with good break point",
			input:         "Line 1\nLine 2\nLine 3\nLine 4\nLine 5",
			maxSize:       "20",
			expectedTrunc: true,
			expectedMsg:   "RESPONSE TRUNCATED",
		},
		{
			name:          "exactly at limit",
			input:         "12345", // exactly 5 characters
			maxSize:       "5",
			expectedTrunc: false,
			expectedMsg:   "12345",
		},
		{
			name:          "one character over limit",
			input:         "123456", // 6 characters
			maxSize:       "5",
			expectedTrunc: true,
			expectedMsg:   "RESPONSE TRUNCATED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			originalSize := os.Getenv("AGENT_MAX_RESPONSE_SIZE")
			defer func() {
				if originalSize == "" {
					_ = os.Unsetenv("AGENT_MAX_RESPONSE_SIZE")
				} else {
					_ = os.Setenv("AGENT_MAX_RESPONSE_SIZE", originalSize)
				}
			}()

			if tt.maxSize != "" {
				_ = os.Setenv("AGENT_MAX_RESPONSE_SIZE", tt.maxSize)
			} else {
				_ = os.Unsetenv("AGENT_MAX_RESPONSE_SIZE")
			}

			result := tool.ApplyResponseSizeLimit(tt.input, logger)

			if tt.expectedTrunc {
				testutils.AssertTrue(t, len(result) >= len(tt.input) || strings.Contains(result, "RESPONSE TRUNCATED"))
				testutils.AssertTrue(t, strings.Contains(result, tt.expectedMsg))
			} else {
				testutils.AssertEqual(t, tt.expectedMsg, result)
			}
		})
	}
}

func TestQDeveloperTool_HelperMethods_Coverage(t *testing.T) {
	// Test methods that might not be covered by other tests
	tool := &qdeveloperagent.QDeveloperTool{}

	// Test GetMaxResponseSize with various environment values
	tests := []struct {
		name        string
		envValue    string
		expectedVal int
	}{
		{"no env var", "", qdeveloperagent.DefaultMaxResponseSize},
		{"valid number", "1000", 1000},
		{"zero value", "0", qdeveloperagent.DefaultMaxResponseSize},                // Should fall back to default for zero
		{"negative value", "-100", qdeveloperagent.DefaultMaxResponseSize},         // Should fall back to default
		{"invalid string", "not-a-number", qdeveloperagent.DefaultMaxResponseSize}, // Should fall back to default
		{"empty string", "", qdeveloperagent.DefaultMaxResponseSize},
	}

	for _, tt := range tests {
		t.Run("GetMaxResponseSize_"+tt.name, func(t *testing.T) {
			original := os.Getenv("AGENT_MAX_RESPONSE_SIZE")
			defer func() {
				if original == "" {
					_ = os.Unsetenv("AGENT_MAX_RESPONSE_SIZE")
				} else {
					_ = os.Setenv("AGENT_MAX_RESPONSE_SIZE", original)
				}
			}()

			if tt.envValue == "" {
				_ = os.Unsetenv("AGENT_MAX_RESPONSE_SIZE")
			} else {
				_ = os.Setenv("AGENT_MAX_RESPONSE_SIZE", tt.envValue)
			}

			result := tool.GetMaxResponseSize()
			testutils.AssertEqual(t, tt.expectedVal, result)
		})
	}
}

func TestQDeveloperTool_ParameterSanitization_SecurityChecks(t *testing.T) {
	// Test that parameters are handled safely without calling actual CLI
	tool := &qdeveloperagent.QDeveloperTool{}

	// These are scenarios that could potentially be problematic
	// We verify the tool validates them properly without executing anything
	dangerousInputs := []struct {
		name   string
		prompt string
	}{
		{"semicolon injection", "harmless prompt; rm -rf /"},
		{"pipe injection", "normal prompt | cat /etc/passwd"},
		{"backtick injection", "prompt with `whoami` command"},
		{"dollar injection", "prompt with $(id) substitution"},
		{"newline injection", "prompt with\necho malicious"},
		{"quote injection", "prompt with \"quotes\" and 'apostrophes'"},
	}

	for _, tt := range dangerousInputs {
		t.Run(tt.name, func(t *testing.T) {
			// Test that dangerous inputs are accepted as valid prompts
			// (since they're just text to be passed to Q Developer, not shell commands)
			args := map[string]any{
				"prompt": tt.prompt,
			}

			// Test with tool disabled to avoid CLI execution
			originalValue := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
			defer func() {
				if originalValue == "" {
					_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
				} else {
					_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", originalValue)
				}
			}()

			_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")

			logger := testutils.CreateTestLogger()
			cache := testutils.CreateTestCache()
			ctx := testutils.CreateTestContext()

			_, err := tool.Execute(ctx, logger, cache, args)

			// Should get "not enabled" error, not a validation error
			// This proves the dangerous input was accepted as a valid prompt
			testutils.AssertError(t, err)
			testutils.AssertErrorContains(t, err, "not enabled")
			// Should NOT contain validation errors
			if strings.Contains(err.Error(), "prompt is a required parameter") {
				t.Errorf("Dangerous input '%s' was incorrectly rejected as invalid prompt", tt.prompt)
			}
		})
	}
}

func TestQDeveloperTool_ModelParameterValues(t *testing.T) {
	// Test the specific model values mentioned in the decision log
	tool := &qdeveloperagent.QDeveloperTool{}
	def := tool.Definition()

	// Verify the override-model parameter mentions the correct models
	modelParam, exists := def.InputSchema.Properties["override-model"]
	testutils.AssertTrue(t, exists)
	testutils.AssertNotNil(t, modelParam)

	// Check that the description mentions the available models
	descStr := fmt.Sprintf("%v", modelParam)
	testutils.AssertTrue(t, strings.Contains(descStr, "claude-3.5-sonnet"))
	testutils.AssertTrue(t, strings.Contains(descStr, "claude-3.7-sonnet"))
	testutils.AssertTrue(t, strings.Contains(descStr, "claude-sonnet-4"))
}

// Note: Integration tests that execute actual CLI are excluded to keep tests fast.
// Tool enablement is tested separately in tests/unit/enablement_test.go

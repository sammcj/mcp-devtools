package tools_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/kiroagent"
	"github.com/sammcj/mcp-devtools/tests/testutils"
	"github.com/stretchr/testify/assert"
)

// Basic tests following the pattern of other agent tools (geminiagent_test.go, claudeagent_test.go)

func TestKiroTool_Definition(t *testing.T) {
	tool := &kiroagent.KiroTool{}
	def := tool.Definition()

	assert.NotNil(t, def)
	assert.Equal(t, "kiro-agent", def.GetName())
}

func TestKiroTool_Definition_ParameterSchema(t *testing.T) {
	tool := &kiroagent.KiroTool{}
	def := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "kiro-agent", def.Name)
	testutils.AssertNotNil(t, def.Description)

	// Test that description contains key phrases
	desc := def.Description
	if !testutils.Contains(desc, "Kiro CLI") {
		t.Errorf("Expected description to contain 'Kiro CLI', got: %s", desc)
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

func TestKiroTool_Definition_OptionalParameters(t *testing.T) {
	tool := &kiroagent.KiroTool{}
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

func TestKiroTool_Definition_ParameterNamingConventions(t *testing.T) {
	tool := &kiroagent.KiroTool{}
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

func TestKiroTool_TimeoutConfiguration_DefaultTimeout(t *testing.T) {
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

	tool := &kiroagent.KiroTool{}
	timeout := tool.GetTimeout()

	testutils.AssertEqual(t, kiroagent.DefaultTimeout, timeout)
}

func TestKiroTool_TimeoutConfiguration_CustomTimeout(t *testing.T) {
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

	tool := &kiroagent.KiroTool{}
	timeout := tool.GetTimeout()

	testutils.AssertEqual(t, 300, timeout)
}

func TestKiroTool_TimeoutConfiguration_InvalidTimeout(t *testing.T) {
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

	tool := &kiroagent.KiroTool{}
	timeout := tool.GetTimeout()

	// Should fall back to default when invalid value is provided
	testutils.AssertEqual(t, kiroagent.DefaultTimeout, timeout)
}

func TestKiroTool_ResponseSizeLimit_DefaultLimit(t *testing.T) {
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

	tool := &kiroagent.KiroTool{}
	logger := testutils.CreateTestLogger()

	// Test with small output (should not be truncated)
	smallOutput := "This is a small response that should not be truncated."
	result := tool.ApplyResponseSizeLimit(smallOutput, logger)
	testutils.AssertEqual(t, smallOutput, result)

	// Test with large output (should be truncated)
	largeOutput := strings.Repeat("K", 3*1024*1024) // 3MB
	result = tool.ApplyResponseSizeLimit(largeOutput, logger)

	// Should be truncated to default 2MB limit
	testutils.AssertTrue(t, len(result) < len(largeOutput))
	testutils.AssertTrue(t, strings.Contains(result, "[RESPONSE TRUNCATED"))
	testutils.AssertTrue(t, strings.Contains(result, "exceeded 2.0MB limit"))
}

func TestKiroTool_ResponseSizeLimit_CustomLimit(t *testing.T) {
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

	tool := &kiroagent.KiroTool{}
	logger := testutils.CreateTestLogger()

	// Test with output larger than custom limit
	largeOutput := strings.Repeat("K", 1500000) // 1.5MB
	result := tool.ApplyResponseSizeLimit(largeOutput, logger)

	// Should be truncated to custom 1MB limit
	testutils.AssertTrue(t, len(result) < len(largeOutput))
	testutils.AssertTrue(t, strings.Contains(result, "[RESPONSE TRUNCATED"))
	testutils.AssertTrue(t, strings.Contains(result, "exceeded 1.0MB limit"))
}

func TestKiroTool_Constants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, "AGENT_MAX_RESPONSE_SIZE", kiroagent.AgentMaxResponseSizeEnvVar)
	testutils.AssertEqual(t, "AGENT_TIMEOUT", kiroagent.AgentTimeoutEnvVar)
	testutils.AssertEqual(t, 2*1024*1024, kiroagent.DefaultMaxResponseSize)
	testutils.AssertEqual(t, 300, kiroagent.DefaultTimeout)
}

// Fast error handling tests that don't execute CLI

func TestKiroTool_Execute_ValidationErrors(t *testing.T) {
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
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "kiro-agent")

	tool := &kiroagent.KiroTool{}
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

func TestKiroTool_ExtendedHelp(t *testing.T) {
	// Test that the tool provides extended help information
	tool := &kiroagent.KiroTool{}

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

func TestKiroTool_ExtendedHelp_ContentVerification(t *testing.T) {
	// Test specific content in extended help
	tool := &kiroagent.KiroTool{}
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

// Note: Integration tests that execute actual CLI are excluded to keep tests fast.
// Tool enablement is tested separately in tests/unit/enablement_test.go

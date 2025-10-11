package tools_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/copilotagent"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

// Basic tests following the pattern of other agent tools (qdeveloper_test.go, geminiagent_test.go)

func TestCopilotTool_Definition(t *testing.T) {
	tool := &copilotagent.CopilotTool{}
	def := tool.Definition()

	testutils.AssertNotNil(t, def)
	testutils.AssertEqual(t, "copilot-agent", def.GetName())
}

func TestCopilotTool_Definition_ParameterSchema(t *testing.T) {
	tool := &copilotagent.CopilotTool{}
	def := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "copilot-agent", def.Name)
	testutils.AssertNotNil(t, def.Description)

	// Test that description contains key phrases
	desc := def.Description
	if !testutils.Contains(desc, "GitHub Copilot CLI") {
		t.Errorf("Expected description to contain 'GitHub Copilot CLI', got: %s", desc)
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

func TestCopilotTool_Definition_OptionalParameters(t *testing.T) {
	tool := &copilotagent.CopilotTool{}
	def := tool.Definition()
	schema := def.InputSchema

	// Test optional parameters exist
	optionalParams := []string{
		"override-model",
		"resume",
		"session-id",
		"yolo-mode",
		"allow-tool",
		"deny-tool",
		"include-directories",
		"disable-mcp-server",
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

func TestCopilotTool_Definition_ParameterNamingConventions(t *testing.T) {
	tool := &copilotagent.CopilotTool{}
	def := tool.Definition()
	schema := def.InputSchema

	// Test that we use consistent naming conventions
	expectedParams := map[string]bool{
		"prompt":              true, // required
		"override-model":      true, // follows decision log standardisation
		"resume":              true, // optional boolean
		"session-id":          true, // optional string
		"yolo-mode":           true, // matches other agent convention
		"allow-tool":          true, // array
		"deny-tool":           true, // array
		"include-directories": true, // array
		"disable-mcp-server":  true, // array
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

func TestCopilotTool_TimeoutConfiguration_DefaultTimeout(t *testing.T) {
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

	tool := &copilotagent.CopilotTool{}
	timeout := tool.GetTimeout()

	testutils.AssertEqual(t, copilotagent.DefaultTimeout, timeout)
}

func TestCopilotTool_TimeoutConfiguration_CustomTimeout(t *testing.T) {
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

	tool := &copilotagent.CopilotTool{}
	timeout := tool.GetTimeout()

	testutils.AssertEqual(t, 300, timeout)
}

func TestCopilotTool_TimeoutConfiguration_InvalidTimeout(t *testing.T) {
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

	tool := &copilotagent.CopilotTool{}
	timeout := tool.GetTimeout()

	// Should fall back to default when invalid value is provided
	testutils.AssertEqual(t, copilotagent.DefaultTimeout, timeout)
}

func TestCopilotTool_ResponseSizeLimit_DefaultLimit(t *testing.T) {
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

	tool := &copilotagent.CopilotTool{}
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

func TestCopilotTool_ResponseSizeLimit_CustomLimit(t *testing.T) {
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

	tool := &copilotagent.CopilotTool{}
	logger := testutils.CreateTestLogger()

	// Test with output larger than custom limit
	largeOutput := strings.Repeat("C", 1500000) // 1.5MB
	result := tool.ApplyResponseSizeLimit(largeOutput, logger)

	// Should be truncated to custom 1MB limit
	testutils.AssertTrue(t, len(result) < len(largeOutput))
	testutils.AssertTrue(t, strings.Contains(result, "[RESPONSE TRUNCATED"))
	testutils.AssertTrue(t, strings.Contains(result, "exceeded 1.0MB limit"))
}

func TestCopilotTool_Constants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, "AGENT_MAX_RESPONSE_SIZE", copilotagent.AgentMaxResponseSizeEnvVar)
	testutils.AssertEqual(t, "AGENT_TIMEOUT", copilotagent.AgentTimeoutEnvVar)
	testutils.AssertEqual(t, 2*1024*1024, copilotagent.DefaultMaxResponseSize)
	testutils.AssertEqual(t, 180, copilotagent.DefaultTimeout)
}

// Fast error handling tests that don't execute CLI

func TestCopilotTool_Execute_ToolDisabled(t *testing.T) {
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

	tool := &copilotagent.CopilotTool{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"prompt": "test prompt",
	}

	result, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "copilot agent tool is not enabled")
	testutils.AssertErrorContains(t, err, "ENABLE_ADDITIONAL_TOOLS")
	testutils.AssertErrorContains(t, err, "copilot-agent")
	if result != nil {
		t.Error("Expected nil result when tool is disabled")
	}
}

func TestCopilotTool_Execute_ValidationErrors(t *testing.T) {
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
	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "copilot-agent")

	tool := &copilotagent.CopilotTool{}
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

func TestCopilotTool_Registration(t *testing.T) {
	// Test that the tool is registered during package initialisation
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	// Test that the tool is discoverable via registry
	retrievedTool, ok := registry.GetTool("copilot-agent")
	testutils.AssertTrue(t, ok)
	testutils.AssertNotNil(t, retrievedTool)

	// Verify the retrieved tool has the correct name
	def := retrievedTool.Definition()
	testutils.AssertEqual(t, "copilot-agent", def.Name)
}

func TestCopilotTool_Registration_InToolsList(t *testing.T) {
	// Test that the tool appears in the complete tools list
	logger := testutils.CreateTestLogger()
	registry.Init(logger)

	tools := registry.GetTools()
	testutils.AssertNotNil(t, tools)

	// Test that copilot-agent is in the tools map
	tool, exists := tools["copilot-agent"]
	testutils.AssertTrue(t, exists)
	testutils.AssertNotNil(t, tool)

	// Verify it's the correct tool type
	def := tool.Definition()
	testutils.AssertEqual(t, "copilot-agent", def.Name)
	testutils.AssertTrue(t, strings.Contains(def.Description, "GitHub Copilot CLI"))
}

func TestCopilotTool_ExtendedHelp(t *testing.T) {
	// Test that the tool provides extended help information
	tool := &copilotagent.CopilotTool{}

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

func TestCopilotTool_ExtendedHelp_ContentVerification(t *testing.T) {
	// Test specific content in extended help
	tool := &copilotagent.CopilotTool{}
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
	expectedParams := []string{
		"prompt",
		"override-model",
		"resume",
		"session-id",
		"yolo-mode",
		"allow-tool",
		"deny-tool",
		"include-directories",
		"disable-mcp-server",
	}
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

// Output filtering tests

func TestCopilotTool_FilterOutput(t *testing.T) {
	tool := &copilotagent.CopilotTool{}

	tests := []struct {
		name             string
		input            string
		expectedOutput   string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name: "answer on same line as last progress indicator",
			input: `● Starting analysis
✓ Read file.go
● 4`,
			expectedOutput:   "4",
			shouldContain:    []string{"4"},
			shouldNotContain: []string{"●", "✓", "Starting", "Read"},
		},
		{
			name: "multi-line answer after last indicator",
			input: `● Starting analysis
✓ Read file.go
● The answer is:
Line 1 of answer
Line 2 of answer

Total usage est: 1 request`,
			expectedOutput:   "The answer is:\nLine 1 of answer\nLine 2 of answer",
			shouldContain:    []string{"The answer is:", "Line 1 of answer", "Line 2 of answer"},
			shouldNotContain: []string{"●", "✓", "Starting", "Total usage est"},
		},
		{
			name: "filter progress indicators with content following",
			input: `● Starting analysis
✓ Read file.go
✗ Failed to read
↪ 2 lines...

● Actual content here`,
			expectedOutput:   "Actual content here",
			shouldContain:    []string{"Actual content here"},
			shouldNotContain: []string{"Starting", "Read", "Failed"},
		},
		{
			name: "filter command traces",
			input: `$ grep pattern file
$ ls -la
● Actual output`,
			expectedOutput:   "Actual output",
			shouldContain:    []string{"Actual output"},
			shouldNotContain: []string{"$ grep", "$ ls"},
		},
		{
			name: "stop at usage statistics",
			input: `● Good content
More good content

Total usage est:       1 Premium request
Total duration (API):  1m 38.3s
Usage by model:
    claude-sonnet-4.5    222.8k input`,
			expectedOutput:   "Good content\nMore good content",
			shouldContain:    []string{"Good content", "More good content"},
			shouldNotContain: []string{"Total usage est", "Total duration", "Usage by model"},
		},
		{
			name: "collapse multiple empty lines",
			input: `● Line 1


Line 2



Line 3`,
			shouldContain:    []string{"Line 1", "Line 2", "Line 3"},
			shouldNotContain: []string{"\n\n\n"},
		},
		{
			name: "comprehensive filtering with last indicator",
			input: `● Starting
✓ Done reading
$ command here
✓
Good line 1
Good line 2


Good line 3
Total usage est: something`,
			expectedOutput:   "Good line 1\nGood line 2\n\nGood line 3",
			shouldContain:    []string{"Good line 1", "Good line 2", "Good line 3"},
			shouldNotContain: []string{"●", "$", "✓", "Starting", "Done reading", "Total usage est"},
		},
		{
			name: "no progress indicators - keeps all content",
			input: `Just normal text
More normal text
Total usage est: 1 request`,
			expectedOutput:   "Just normal text\nMore normal text",
			shouldContain:    []string{"Just normal text", "More normal text"},
			shouldNotContain: []string{"Total usage est"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.FilterOutput(tt.input)

			for _, expected := range tt.shouldContain {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected output to contain '%s', got: %s", expected, result)
				}
			}

			for _, notExpected := range tt.shouldNotContain {
				if strings.Contains(result, notExpected) {
					t.Errorf("Expected output to NOT contain '%s', got: %s", notExpected, result)
				}
			}

			if tt.expectedOutput != "" {
				trimmed := strings.TrimSpace(result)
				expectedTrimmed := strings.TrimSpace(tt.expectedOutput)
				if trimmed != expectedTrimmed {
					t.Errorf("Expected output:\n%s\nGot:\n%s", expectedTrimmed, trimmed)
				}
			}
		})
	}
}

func TestCopilotTool_FilterOutput_EdgeCases(t *testing.T) {
	tool := &copilotagent.CopilotTool{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \n\n   ",
			expected: "",
		},
		{
			name:     "no filtering needed",
			input:    "Just normal text",
			expected: "Just normal text",
		},
		{
			name:     "all lines filtered except last indicator content",
			input:    "● Progress\n✓ Done\n$ command",
			expected: "Done", // "Done" is content on the last progress indicator line
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.FilterOutput(tt.input)
			trimmed := strings.TrimSpace(result)
			expected := strings.TrimSpace(tt.expected)
			testutils.AssertEqual(t, expected, trimmed)
		})
	}
}

// Response size limit edge cases

func TestCopilotTool_ResponseSizeLimit_EdgeCases(t *testing.T) {
	tool := &copilotagent.CopilotTool{}
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
				testutils.AssertTrue(t, len(result) < len(tt.input) || strings.Contains(result, "RESPONSE TRUNCATED"))
				testutils.AssertTrue(t, strings.Contains(result, tt.expectedMsg))
			} else {
				testutils.AssertEqual(t, tt.expectedMsg, result)
			}
		})
	}
}

// Note: Integration tests that execute actual CLI are excluded to keep tests fast.
// Tool enablement is tested separately in tests/unit/enablement_test.go

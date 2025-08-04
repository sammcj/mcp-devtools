package tools_test

import (
	"os"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/claudeagent"
	"github.com/sammcj/mcp-devtools/internal/tools/geminiagent"
	"github.com/sammcj/mcp-devtools/tests/testutils"
	"github.com/sirupsen/logrus"
)

func TestClaudeAgentTool_ResponseSizeLimit_DefaultLimit(t *testing.T) {
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

	tool := &claudeagent.ClaudeTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stdout)

	// Test with small output (should not be truncated)
	smallOutput := "This is a small response that should not be truncated."
	result := tool.ApplyResponseSizeLimit(smallOutput, logger)
	testutils.AssertEqual(t, smallOutput, result)

	// Test with large output (should be truncated)
	largeOutput := strings.Repeat("A", 3*1024*1024) // 3MB
	result = tool.ApplyResponseSizeLimit(largeOutput, logger)

	// Should be truncated to default 2MB limit
	testutils.AssertTrue(t, len(result) < len(largeOutput))
	testutils.AssertTrue(t, strings.Contains(result, "[RESPONSE TRUNCATED"))
	testutils.AssertTrue(t, strings.Contains(result, "exceeded 2.0MB limit"))
}

func TestClaudeAgentTool_ResponseSizeLimit_CustomLimit(t *testing.T) {
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
	err := os.Setenv("AGENT_MAX_RESPONSE_SIZE", "1048576")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &claudeagent.ClaudeTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stdout)

	// Test with output larger than custom limit
	largeOutput := strings.Repeat("B", 1500000) // 1.5MB
	result := tool.ApplyResponseSizeLimit(largeOutput, logger)

	// Should be truncated to custom 1MB limit
	testutils.AssertTrue(t, len(result) < len(largeOutput))
	testutils.AssertTrue(t, strings.Contains(result, "[RESPONSE TRUNCATED"))
	testutils.AssertTrue(t, strings.Contains(result, "exceeded 1.0MB limit"))
}

func TestClaudeAgentTool_ResponseSizeLimit_InvalidEnvironmentVariable(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("AGENT_MAX_RESPONSE_SIZE")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("AGENT_MAX_RESPONSE_SIZE")
		} else {
			_ = os.Setenv("AGENT_MAX_RESPONSE_SIZE", originalValue)
		}
	}()

	// Set invalid environment variable value
	err := os.Setenv("AGENT_MAX_RESPONSE_SIZE", "invalid")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &claudeagent.ClaudeTool{}

	// Should fall back to default when invalid value is provided
	maxSize := tool.GetMaxResponseSize()
	testutils.AssertEqual(t, claudeagent.DefaultMaxResponseSize, maxSize)
}

func TestClaudeAgentTool_ResponseSizeLimit_ZeroEnvironmentVariable(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("AGENT_MAX_RESPONSE_SIZE")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("AGENT_MAX_RESPONSE_SIZE")
		} else {
			_ = os.Setenv("AGENT_MAX_RESPONSE_SIZE", originalValue)
		}
	}()

	// Set zero environment variable value
	err := os.Setenv("AGENT_MAX_RESPONSE_SIZE", "0")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &claudeagent.ClaudeTool{}

	// Should fall back to default when zero value is provided
	maxSize := tool.GetMaxResponseSize()
	testutils.AssertEqual(t, claudeagent.DefaultMaxResponseSize, maxSize)
}

func TestClaudeAgentTool_ResponseSizeLimit_NaturalTruncation(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("AGENT_MAX_RESPONSE_SIZE")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("AGENT_MAX_RESPONSE_SIZE")
		} else {
			_ = os.Setenv("AGENT_MAX_RESPONSE_SIZE", originalValue)
		}
	}()

	// Set very small limit for testing
	err := os.Setenv("AGENT_MAX_RESPONSE_SIZE", "200")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &claudeagent.ClaudeTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stdout)

	// Create output with natural line breaks
	largeOutput := strings.Repeat("Line of text that should be truncated.\n", 10)
	result := tool.ApplyResponseSizeLimit(largeOutput, logger)

	// Should be truncated and contain truncation message
	testutils.AssertTrue(t, len(result) < len(largeOutput))
	testutils.AssertTrue(t, strings.Contains(result, "[RESPONSE TRUNCATED"))
}

func TestGeminiAgentTool_ResponseSizeLimit_DefaultLimit(t *testing.T) {
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

	tool := &geminiagent.GeminiTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stdout)

	// Test with small output (should not be truncated)
	smallOutput := "This is a small response that should not be truncated."
	result := tool.ApplyResponseSizeLimit(smallOutput, logger)
	testutils.AssertEqual(t, smallOutput, result)

	// Test with large output (should be truncated)
	largeOutput := strings.Repeat("G", 3*1024*1024) // 3MB
	result = tool.ApplyResponseSizeLimit(largeOutput, logger)

	// Should be truncated to default 2MB limit
	testutils.AssertTrue(t, len(result) < len(largeOutput))
	testutils.AssertTrue(t, strings.Contains(result, "[RESPONSE TRUNCATED"))
	testutils.AssertTrue(t, strings.Contains(result, "exceeded 2.0MB limit"))
}

func TestGeminiAgentTool_ResponseSizeLimit_CustomLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("AGENT_MAX_RESPONSE_SIZE")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("AGENT_MAX_RESPONSE_SIZE")
		} else {
			_ = os.Setenv("AGENT_MAX_RESPONSE_SIZE", originalValue)
		}
	}()

	// Set custom limit (512KB = 524288 bytes)
	err := os.Setenv("AGENT_MAX_RESPONSE_SIZE", "524288")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &geminiagent.GeminiTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stdout)

	// Test with output larger than custom limit
	largeOutput := strings.Repeat("G", 800000) // 800KB
	result := tool.ApplyResponseSizeLimit(largeOutput, logger)

	// Should be truncated to custom 512KB limit
	testutils.AssertTrue(t, len(result) < len(largeOutput))
	testutils.AssertTrue(t, strings.Contains(result, "[RESPONSE TRUNCATED"))
	testutils.AssertTrue(t, strings.Contains(result, "exceeded 0.5MB limit"))
}

func TestGeminiAgentTool_ResponseSizeLimit_InvalidEnvironmentVariable(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("AGENT_MAX_RESPONSE_SIZE")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("AGENT_MAX_RESPONSE_SIZE")
		} else {
			_ = os.Setenv("AGENT_MAX_RESPONSE_SIZE", originalValue)
		}
	}()

	// Set invalid environment variable value
	err := os.Setenv("AGENT_MAX_RESPONSE_SIZE", "not-a-number")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &geminiagent.GeminiTool{}

	// Should fall back to default when invalid value is provided
	maxSize := tool.GetMaxResponseSize()
	testutils.AssertEqual(t, geminiagent.DefaultMaxResponseSize, maxSize)
}

func TestAgentConstants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, "AGENT_MAX_RESPONSE_SIZE", claudeagent.AgentMaxResponseSizeEnvVar)
	testutils.AssertEqual(t, "AGENT_MAX_RESPONSE_SIZE", geminiagent.AgentMaxResponseSizeEnvVar)
	testutils.AssertEqual(t, 2*1024*1024, claudeagent.DefaultMaxResponseSize)
	testutils.AssertEqual(t, 2*1024*1024, geminiagent.DefaultMaxResponseSize)
}

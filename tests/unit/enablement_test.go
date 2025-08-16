package unit_test

import (
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/stretchr/testify/assert"
)

func TestIsToolEnabled(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("ENABLE_ADDITIONAL_TOOLS")
	defer func() {
		if originalValue != "" {
			_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", originalValue)
		} else {
			_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
		}
	}()

	tests := []struct {
		name     string
		envValue string
		toolName string
		expected bool
	}{
		{
			name:     "empty environment variable",
			envValue: "",
			toolName: "sbom",
			expected: false,
		},
		{
			name:     "single tool enabled",
			envValue: "sbom",
			toolName: "sbom",
			expected: true,
		},
		{
			name:     "multiple tools enabled - first",
			envValue: "sbom,vulnerability_scan,filesystem",
			toolName: "sbom",
			expected: true,
		},
		{
			name:     "multiple tools enabled - middle",
			envValue: "sbom,vulnerability_scan,filesystem",
			toolName: "vulnerability_scan",
			expected: true,
		},
		{
			name:     "multiple tools enabled - last",
			envValue: "sbom,vulnerability_scan,filesystem",
			toolName: "filesystem",
			expected: true,
		},
		{
			name:     "tool not in list",
			envValue: "sbom,vulnerability_scan",
			toolName: "filesystem",
			expected: false,
		},
		{
			name:     "case insensitive matching",
			envValue: "SBOM,VULNERABILITY_SCAN",
			toolName: "sbom",
			expected: true,
		},
		{
			name:     "spaces are ignored",
			envValue: " sbom , vulnerability_scan , filesystem ",
			toolName: "vulnerability_scan",
			expected: true,
		},
		{
			name:     "underscore vs hyphen normalisation - env var has underscore",
			envValue: "vulnerability_scan",
			toolName: "vulnerability-scan",
			expected: true,
		},
		{
			name:     "underscore vs hyphen normalisation - env var has hyphen",
			envValue: "vulnerability-scan",
			toolName: "vulnerability_scan",
			expected: true,
		},
		{
			name:     "agent tools with hyphens",
			envValue: "claude-agent,gemini-agent",
			toolName: "claude-agent",
			expected: true,
		},
		{
			name:     "agent tools with underscores",
			envValue: "claude_agent,gemini_agent",
			toolName: "claude-agent",
			expected: true,
		},
		{
			name:     "mixed case and separators",
			envValue: "Claude-Agent,GEMINI_AGENT,filesystem,SBOM",
			toolName: "claude-agent",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the environment variable
			if tt.envValue == "" {
				_ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS")
			} else {
				_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", tt.envValue)
			}

			result := tools.IsToolEnabled(tt.toolName)
			assert.Equal(t, tt.expected, result, "Tool %q should be %v with env %q", tt.toolName, tt.expected, tt.envValue)
		})
	}
}

func TestIsToolEnabled_AllSupportedTools(t *testing.T) {
	// Test that all expected tool names work correctly
	supportedTools := []string{
		"claude-agent",
		"gemini-agent",
		"filesystem",
		"vulnerability_scan",
		"vulnerability-scan", // Normalised version
		"sbom",
	}

	_ = os.Setenv("ENABLE_ADDITIONAL_TOOLS", "claude-agent,gemini-agent,filesystem,vulnerability_scan,sbom")
	defer func() { _ = os.Unsetenv("ENABLE_ADDITIONAL_TOOLS") }()

	for _, toolName := range supportedTools {
		t.Run("tool_"+toolName, func(t *testing.T) {
			result := tools.IsToolEnabled(toolName)
			assert.True(t, result, "Tool %q should be enabled", toolName)
		})
	}
}

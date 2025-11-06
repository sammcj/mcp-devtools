package unit

import (
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools"
)

func TestGetAgentPermissionsMode(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected tools.PermissionsMode
	}{
		{"Unset (empty)", "", tools.PermissionsModeDefault},
		{"Yolo mode", "yolo", tools.PermissionsModeEnabled},
		{"Disabled - disabled", "disabled", tools.PermissionsModeDisabled},
		{"Disabled - false", "false", tools.PermissionsModeDisabled},
		{"Invalid value", "invalid", tools.PermissionsModeDefault},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tools.AgentPermissionsModeEnvVar, tt.envValue)
			defer os.Unsetenv(tools.AgentPermissionsModeEnvVar)

			result := tools.GetAgentPermissionsMode()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestShouldExposePermissionsParameter(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"Unset - should expose", "", true},
		{"Yolo mode - should not expose", "yolo", false},
		{"Disabled mode - should not expose", "disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tools.AgentPermissionsModeEnvVar, tt.envValue)
			defer os.Unsetenv(tools.AgentPermissionsModeEnvVar)

			result := tools.ShouldExposePermissionsParameter()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetEffectivePermissionsValue(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		paramValue bool
		expected   bool
	}{
		{"Unset - param true", "", true, true},
		{"Unset - param false", "", false, false},
		{"Yolo mode - ignores param false", "yolo", false, true},
		{"Yolo mode - ignores param true", "yolo", true, true},
		{"Disabled mode - ignores param true", "disabled", true, false},
		{"Disabled mode - ignores param false", "disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(tools.AgentPermissionsModeEnvVar, tt.envValue)
			defer os.Unsetenv(tools.AgentPermissionsModeEnvVar)

			result := tools.GetEffectivePermissionsValue(tt.paramValue)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

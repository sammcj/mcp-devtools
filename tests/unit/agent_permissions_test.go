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
		{"Default - empty", "", tools.PermissionsModeDefault},
		{"Default - explicit", "default", tools.PermissionsModeDefault},
		{"Enabled - enabled", "enabled", tools.PermissionsModeEnabled},
		{"Enabled - true", "true", tools.PermissionsModeEnabled},
		{"Enabled - yolo", "yolo", tools.PermissionsModeEnabled},
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
		{"Default mode - should expose", "default", true},
		{"Enabled mode - should not expose", "enabled", false},
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
		{"Default mode - param true", "default", true, true},
		{"Default mode - param false", "default", false, false},
		{"Enabled mode - ignores param false", "enabled", false, true},
		{"Enabled mode - ignores param true", "enabled", true, true},
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

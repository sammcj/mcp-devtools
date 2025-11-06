package tools

import (
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// PermissionsMode represents the agent permissions mode configuration
type PermissionsMode int

const (
	// PermissionsModeDefault allows the agent to control yolo mode via parameter
	PermissionsModeDefault PermissionsMode = iota
	// PermissionsModeEnabled forces yolo mode on, parameter is hidden
	PermissionsModeEnabled
	// PermissionsModeDisabled forces yolo mode off, parameter is hidden
	PermissionsModeDisabled
)

const (
	// AgentPermissionsModeEnvVar is the environment variable name for controlling agent permissions mode
	AgentPermissionsModeEnvVar = "AGENT_PERMISSIONS_MODE"
)

// GetAgentPermissionsMode returns the current permissions mode from environment variable
// Valid values: "default", "enabled", "disabled" (case-insensitive)
// Aliases: "true"/"yolo" for enabled, "false" for disabled
// Returns PermissionsModeDefault if unset or invalid value
func GetAgentPermissionsMode() PermissionsMode {
	mode := strings.TrimSpace(strings.ToLower(os.Getenv(AgentPermissionsModeEnvVar)))

	switch mode {
	case "enabled", "true", "yolo":
		return PermissionsModeEnabled
	case "disabled", "false":
		return PermissionsModeDisabled
	case "default", "":
		return PermissionsModeDefault
	default:
		// Invalid value, default to safe default mode
		return PermissionsModeDefault
	}
}

// ShouldExposePermissionsParameter returns whether the yolo-mode parameter should be exposed to the agent
func ShouldExposePermissionsParameter() bool {
	return GetAgentPermissionsMode() == PermissionsModeDefault
}

// GetEffectivePermissionsValue returns the effective yolo mode value based on environment and parameter
// paramValue is the value provided by the agent (only used in default mode)
func GetEffectivePermissionsValue(paramValue bool) bool {
	mode := GetAgentPermissionsMode()

	switch mode {
	case PermissionsModeEnabled:
		return true
	case PermissionsModeDisabled:
		return false
	case PermissionsModeDefault:
		return paramValue
	default:
		return paramValue
	}
}

// AddConditionalPermissionsParameter adds the yolo-mode parameter to tool definition only if in default mode
// parameterName is the name of the parameter (e.g., "yolo-mode")
// description is the parameter description
func AddConditionalPermissionsParameter(parameterName, description string, options ...mcp.ToolOption) mcp.ToolOption {
	if !ShouldExposePermissionsParameter() {
		// Return a no-op option when permissions mode is forced
		return func(t *mcp.Tool) {}
	}

	// Return the actual parameter definition
	return mcp.WithBoolean(parameterName,
		mcp.Description(description),
		mcp.DefaultBool(false),
	)
}

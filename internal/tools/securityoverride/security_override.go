package securityoverride

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// SecurityOverrideTool handles security override requests
type SecurityOverrideTool struct{}

// init registers the security override tool
func init() {
	registry.Register(&SecurityOverrideTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *SecurityOverrideTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"security_override",
		mcp.WithDescription(`Allowlist security warnings and blocks when they are false positives or when the user has verified the content is safe. All overrides are logged with justification for audit purposes.`),
		mcp.WithString("security_id",
			mcp.Required(),
			mcp.Description("Security warning/block ID from security log (e.g., sec_warn_a1b2c3)"),
		),
		mcp.WithString("justification",
			mcp.Required(),
			mcp.Description("Detailed justification for why this override is necessary"),
		),
		mcp.WithString("action",
			mcp.Description("Override action: 'bypass' (ignore this instance), 'allowlist' (ignore future similar patterns)"),
			mcp.DefaultString("bypass"),
			mcp.Enum("bypass", "allowlist"),
		),
		// Destructive tool annotations
		mcp.WithReadOnlyHintAnnotation(false),   // Modifies security configuration
		mcp.WithDestructiveHintAnnotation(true), // Can override security controls
		mcp.WithIdempotentHintAnnotation(false), // Override effects are not reversible
		mcp.WithOpenWorldHintAnnotation(false),  // Works with local security system
	)
}

// Execute processes security override requests
func (t *SecurityOverrideTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	// Check if security system is enabled
	if !tools.IsToolEnabled("security") {
		return nil, fmt.Errorf("security system is not enabled. Ask the user to set ENABLE_ADDITIONAL_TOOLS environment variable to include 'security'")
	}

	// Check if security override tool is explicitly enabled
	if !tools.IsToolEnabled("security_override") {
		return nil, fmt.Errorf("security override tool is not enabled. Ask the user to set ENABLE_ADDITIONAL_TOOLS environment variable to include 'security_override'")
	}

	// Check if global security manager is available
	if security.GlobalSecurityManager == nil {
		return nil, fmt.Errorf("security system is not initialised")
	}

	// Parse parameters
	securityID, ok := args["security_id"].(string)
	if !ok || securityID == "" {
		return nil, fmt.Errorf("missing required parameter: security_id")
	}

	justification, ok := args["justification"].(string)
	if !ok || justification == "" {
		return nil, fmt.Errorf("missing required parameter: justification")
	}

	action := "bypass"
	if actionRaw, ok := args["action"].(string); ok && actionRaw != "" {
		action = actionRaw
	}

	logger.WithFields(logrus.Fields{
		"security_id":   securityID,
		"action":        action,
		"justification": justification,
	}).Info("Processing security override request")

	// Find the security log entry
	overrideManager := security.GlobalSecurityManager.GetOverrideManager()
	logEntry, err := overrideManager.FindSecurityLogEntry(securityID)
	if err != nil {
		return nil, fmt.Errorf("security ID %s not found in logs: %w", securityID, err)
	}

	// Prevent overriding access control blocks
	if logEntry.Type == "file_access_block" || logEntry.Type == "domain_access_block" {
		return nil, fmt.Errorf("cannot override access control block for %s. Access control policies in access_control section cannot be overridden by agents. Contact system administrator to modify main security configuration", logEntry.Source)
	}

	// Create override entry
	override := security.SecurityOverride{
		Type:            logEntry.Action,
		Action:          action,
		Justification:   justification,
		CreatedAt:       time.Now(),
		CreatedBy:       "mcp-tools",
		OriginalPattern: extractPatternFromLogEntry(logEntry),
		OriginalSource:  logEntry.Source,
	}

	// Save override
	if err := overrideManager.SaveOverride(override, securityID); err != nil {
		return nil, fmt.Errorf("failed to save override: %w", err)
	}

	// Create response
	result := map[string]any{
		"status":        "override_created",
		"security_id":   securityID,
		"action":        action,
		"message":       fmt.Sprintf("Security %s for ID %s has been overridden.", logEntry.Action, securityID),
		"override_type": action,
	}

	if action == "allowlist" {
		result["note"] = "Future content matching similar patterns will be automatically allowed"
	}

	logger.WithFields(logrus.Fields{
		"security_id": securityID,
		"action":      action,
	}).Info("Security override created successfully")

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// extractPatternFromLogEntry extracts the relevant pattern from a log entry
func extractPatternFromLogEntry(entry *security.SecurityLogEntry) string {
	// Try to extract meaningful pattern information from the analysis
	if entry.Analysis != nil {
		// If there are commands, use the first one as the pattern
		if len(entry.Analysis.Commands) > 0 {
			return entry.Analysis.Commands[0].Raw
		}

		// Otherwise use risk factors as pattern
		if len(entry.Analysis.RiskFactors) > 0 {
			return entry.Analysis.RiskFactors[0]
		}
	}

	// Fall back to source
	return entry.Source
}

// ProvideExtendedInfo provides detailed usage information for the security override tool
func (t *SecurityOverrideTool) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		Examples: []tools.ToolExample{
			{
				Description: "Bypass a specific security warning",
				Arguments: map[string]any{
					"security_id":   "sec_warn_1705315800_a1b2c3",
					"justification": "This is the official Docker installation script from docs.docker.com, verified as safe",
					"action":        "bypass",
				},
				ExpectedResult: "Creates a bypass override for this specific security event. The warning will not appear again for this exact content.",
			},
			{
				Description: "Allowlist a pattern to prevent future warnings",
				Arguments: map[string]any{
					"security_id":   "sec_warn_1705315801_d4e5f6",
					"justification": "Development environment requires access to test SSH keys in ~/.ssh/test_* for automated testing",
					"action":        "allowlist",
				},
				ExpectedResult: "Creates an allowlist entry that will prevent future warnings for similar patterns. Use carefully as this affects all future content.",
			},
			{
				Description: "Override a content analysis block",
				Arguments: map[string]any{
					"security_id":   "sec_block_1705315802_g7h8i9",
					"justification": "This base64 content is a legitimate configuration file for our application, not malicious code",
					"action":        "bypass",
				},
				ExpectedResult: "Overrides the security block, allowing the content to be processed. The override is logged for audit purposes.",
			},
		},
		CommonPatterns: []string{
			"Always provide detailed justification explaining why the content is safe",
			"Use 'bypass' for one-time overrides, 'allowlist' for patterns you want to permanently allow",
			"Check the security log or warning message to get the correct security_id",
			"Be cautious with 'allowlist' action as it affects all future similar content",
			"Document overrides in your team's security procedures for audit trails",
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Cannot override access control block error",
				Solution: "Access control blocks for sensitive files (SSH keys, credentials) cannot be overridden by agents. These are system-level policies. Ask the user to modify the MCP DevTools security configuration if access is genuinely needed.",
			},
			{
				Problem:  "Security ID not found in logs",
				Solution: "Ensure you're using the exact security ID from the warning message. Security IDs are only valid for recent events. If the ID is old, trigger the security warning again to get a new ID.",
			},
			{
				Problem:  "Security system not enabled error",
				Solution: "The security system requires 'security' in the ENABLE_ADDITIONAL_TOOLS environment variable. Ask the user to enable it in their MCP configuration.",
			},
			{
				Problem:  "Override doesn't seem to work",
				Solution: "Bypass overrides only work for the exact same content. If the content changed slightly, you may need a new override or use 'allowlist' action for pattern-based matching.",
			},
		},
		ParameterDetails: map[string]string{
			"security_id":   "Exact security event ID from warning messages (format: sec_action_timestamp_random). Required for tracking and audit purposes.",
			"justification": "Detailed explanation of why this override is safe and necessary. This is logged for security audits and should be comprehensive.",
			"action":        "Override type: 'bypass' creates one-time exception for specific content, 'allowlist' creates permanent exception for similar patterns. Default is 'bypass'.",
		},
		WhenToUse:    "Use when security warnings or blocks are false positives, or when you've verified that flagged content is safe despite containing potentially dangerous patterns. Always provide thorough justification for audit purposes.",
		WhenNotToUse: "Don't use to bypass legitimate security concerns. Don't use if you haven't verified the content is actually safe. Cannot be used for access control blocks on sensitive files - those require configuration changes.",
	}
}

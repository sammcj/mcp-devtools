package toolhelp

// ToolHelpRequest represents the input parameters for the devtools_help tool
type ToolHelpRequest struct {
	ToolName string `json:"tool_name"`
}

// ToolHelpResponse represents the output of the devtools_help tool
type ToolHelpResponse struct {
	ToolName        string            `json:"tool_name"`
	BasicInfo       map[string]any    `json:"basic_info"`
	ExtendedInfo    *ExtendedHelpData `json:"extended_info,omitempty"`
	HasExtendedInfo bool              `json:"has_extended_info"`
	Message         string            `json:"message,omitempty"`
}

// ExtendedHelpData represents the extended information about a tool
type ExtendedHelpData struct {
	Examples         []ToolExampleData     `json:"examples,omitempty"`
	CommonPatterns   []string              `json:"common_patterns,omitempty"`
	Troubleshooting  []TroubleshootingData `json:"troubleshooting,omitempty"`
	ParameterDetails map[string]string     `json:"parameter_details,omitempty"`
	WhenToUse        string                `json:"when_to_use,omitempty"`
	WhenNotToUse     string                `json:"when_not_to_use,omitempty"`
}

// ToolExampleData represents a usage example for a tool
type ToolExampleData struct {
	Description    string         `json:"description"`
	Arguments      map[string]any `json:"arguments"`
	ExpectedResult string         `json:"expected_result,omitempty"`
}

// TroubleshootingData represents a troubleshooting tip for a tool
type TroubleshootingData struct {
	Problem  string `json:"problem"`
	Solution string `json:"solution"`
}

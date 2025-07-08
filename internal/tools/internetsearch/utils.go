package internetsearch

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// NewToolResultJSON creates a new tool result with JSON content
func NewToolResultJSON(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

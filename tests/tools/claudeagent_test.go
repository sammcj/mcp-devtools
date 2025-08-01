package tools_test

import (
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/claudeagent"
	"github.com/stretchr/testify/assert"
)

func TestClaudeAgentTool_Definition(t *testing.T) {
	tool := &claudeagent.ClaudeTool{}
	def := tool.Definition()

	assert.NotNil(t, def)
	assert.Equal(t, "claude-agent", def.GetName())
}

package tools_test

import (
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/geminiagent"
	"github.com/stretchr/testify/assert"
)

func TestGeminiAgentTool_Definition(t *testing.T) {
	tool := &geminiagent.GeminiTool{}
	def := tool.Definition()

	assert.NotNil(t, def)
	assert.Equal(t, "gemini-agent", def.GetName())
}

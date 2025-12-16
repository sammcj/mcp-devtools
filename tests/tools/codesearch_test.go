//go:build cgo && (darwin || (linux && amd64))

package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/codesearch"
	"github.com/sammcj/mcp-devtools/internal/tools/codesearch/index"
	"github.com/sammcj/mcp-devtools/internal/tools/codesearch/vectorstore"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeSearchToolDefinition(t *testing.T) {
	t.Setenv("ENABLE_ADDITIONAL_TOOLS", "code_search")

	tool := &codesearch.CodeSearchTool{}
	def := tool.Definition()

	// Verify tool name
	assert.Equal(t, "code_search", def.Name)

	// Verify description is under 200 chars (per docs/creating-new-tools.md)
	assert.Less(t, len(def.Description), 200, "description should be under 200 characters")
	assert.NotEmpty(t, def.Description)

	// Verify required parameters exist
	schema := def.InputSchema
	require.NotNil(t, schema)

	// Check required parameters exist
	_, hasAction := schema.Properties["action"]
	assert.True(t, hasAction, "should have action parameter")
	_, hasSource := schema.Properties["source"]
	assert.True(t, hasSource, "should have source parameter")
	_, hasQuery := schema.Properties["query"]
	assert.True(t, hasQuery, "should have query parameter")
}

func TestCodeSearchStatus(t *testing.T) {
	t.Setenv("ENABLE_ADDITIONAL_TOOLS", "code_search")

	tool := &codesearch.CodeSearchTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	args := map[string]any{
		"action": "status",
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)

	// Parse response to verify structure
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var status codesearch.StatusResponse
	err = json.Unmarshal([]byte(textContent.Text), &status)
	require.NoError(t, err)
}

func TestCodeSearchMissingAction(t *testing.T) {
	t.Setenv("ENABLE_ADDITIONAL_TOOLS", "code_search")

	tool := &codesearch.CodeSearchTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	args := map[string]any{}

	_, err := tool.Execute(context.Background(), logger, cache, args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "action")
}

func TestCodeSearchIndexRequiresSource(t *testing.T) {
	t.Setenv("ENABLE_ADDITIONAL_TOOLS", "code_search")

	tool := &codesearch.CodeSearchTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	args := map[string]any{
		"action": "index",
	}

	_, err := tool.Execute(context.Background(), logger, cache, args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source")
}

func TestCodeSearchSearchRequiresQuery(t *testing.T) {
	t.Setenv("ENABLE_ADDITIONAL_TOOLS", "code_search")

	tool := &codesearch.CodeSearchTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	args := map[string]any{
		"action": "search",
	}

	_, err := tool.Execute(context.Background(), logger, cache, args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query")
}

func TestCodeSearchClear(t *testing.T) {
	t.Setenv("ENABLE_ADDITIONAL_TOOLS", "code_search")

	tool := &codesearch.CodeSearchTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Clear with no source (clears all)
	args := map[string]any{
		"action": "clear",
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Parse response
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var clearResult vectorstore.ClearResult
	err = json.Unmarshal([]byte(textContent.Text), &clearResult)
	require.NoError(t, err)
}

func TestCodeSearchIndexSmallFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Setenv("ENABLE_ADDITIONAL_TOOLS", "code_search")

	tool := &codesearch.CodeSearchTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	testContent := `package test

// HelloWorld prints a greeting
func HelloWorld() {
	println("Hello, World!")
}

// Add adds two numbers
func Add(a, b int) int {
	return a + b
}
`
	err := os.WriteFile(testFile, []byte(testContent), 0600)
	require.NoError(t, err)

	// Index the file
	args := map[string]any{
		"action": "index",
		"source": []any{testFile},
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Parse response
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var indexResult index.IndexResult
	err = json.Unmarshal([]byte(textContent.Text), &indexResult)
	require.NoError(t, err)

	assert.Equal(t, 1, indexResult.IndexedFiles)
	assert.Greater(t, indexResult.IndexedItems, 0)
}

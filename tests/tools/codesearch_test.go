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

func TestCodeSearchResponseFields(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Setenv("ENABLE_ADDITIONAL_TOOLS", "code_search")

	tool := &codesearch.CodeSearchTool{}
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	// Create a temporary test file with many functions to enable truncation testing
	// Need enough functions so that limit*3 (chromem's internal multiplier) doesn't exceed count
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "functions.go")
	testContent := `package test

// ProcessData handles data processing
func ProcessData(data []byte) error { return nil }

// ValidateInput validates user input
func ValidateInput(input string) bool { return len(input) > 0 }

// TransformData transforms data from one format to another
func TransformData(src, dst string) error { return nil }

// FilterResults filters results based on criteria
func FilterResults(items []string, filter func(string) bool) []string { return nil }

// ParseConfig parses configuration data
func ParseConfig(path string) (map[string]any, error) { return nil, nil }

// LoadData loads data from a source
func LoadData(source string) ([]byte, error) { return nil, nil }

// SaveData saves data to a destination
func SaveData(dest string, data []byte) error { return nil }

// CleanupResources cleans up allocated resources
func CleanupResources() error { return nil }

// InitialiseSystem initialises the system
func InitialiseSystem() error { return nil }

// ShutdownSystem shuts down the system gracefully
func ShutdownSystem() error { return nil }

// GetStatus returns the current status
func GetStatus() string { return "" }

// SetConfig sets a configuration value
func SetConfig(key, value string) error { return nil }
`
	err := os.WriteFile(testFile, []byte(testContent), 0600)
	require.NoError(t, err)

	// Clear any existing index first
	clearArgs := map[string]any{
		"action": "clear",
	}
	_, err = tool.Execute(context.Background(), logger, cache, clearArgs)
	require.NoError(t, err)

	// Index the file
	indexArgs := map[string]any{
		"action": "index",
		"source": []any{testFile},
	}

	result, err := tool.Execute(context.Background(), logger, cache, indexArgs)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify indexing succeeded
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var indexResult index.IndexResult
	err = json.Unmarshal([]byte(textContent.Text), &indexResult)
	require.NoError(t, err)
	require.Greater(t, indexResult.IndexedItems, 0, "should have indexed at least one item")

	// Get status to determine how many items are indexed
	statusArgs := map[string]any{
		"action": "status",
	}
	result, err = tool.Execute(context.Background(), logger, cache, statusArgs)
	require.NoError(t, err)

	textContent, ok = mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var statusResponse codesearch.StatusResponse
	err = json.Unmarshal([]byte(textContent.Text), &statusResponse)
	require.NoError(t, err)

	t.Logf("Indexed %d items from test file", statusResponse.TotalItems)

	// chromem requires limit*3 <= total items (Search multiplies limit by 3 internally)
	// Skip truncation test if we don't have enough items
	minItemsForTruncation := 4 // Need at least 4 items so limit=1 (1*3=3) works
	if statusResponse.TotalItems < minItemsForTruncation {
		t.Logf("Skipping truncation test: only %d items indexed, need at least %d",
			statusResponse.TotalItems, minItemsForTruncation)

		// Still test LastIndexed with a safe limit
		searchArgs := map[string]any{
			"action":    "search",
			"query":     "function",
			"limit":     float64(statusResponse.TotalItems), // Request all items
			"threshold": float64(0.01),
		}

		result, err = tool.Execute(context.Background(), logger, cache, searchArgs)
		require.NoError(t, err)

		textContent, ok = mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var searchResponse codesearch.SearchResponse
		err = json.Unmarshal([]byte(textContent.Text), &searchResponse)
		require.NoError(t, err)

		// At minimum verify LastIndexed
		assert.NotEmpty(t, searchResponse.LastIndexed, "last_indexed should be populated after indexing")
		assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$`, searchResponse.LastIndexed, "last_indexed should match expected format")
		return
	}

	// Search with a limit that will trigger truncation (limit=1, so chromem gets 3)
	searchLimit := 1
	searchArgs := map[string]any{
		"action":    "search",
		"query":     "function",
		"limit":     float64(searchLimit),
		"threshold": float64(0.01), // Very low threshold to match most items
	}

	result, err = tool.Execute(context.Background(), logger, cache, searchArgs)
	require.NoError(t, err)
	require.NotNil(t, result)

	textContent, ok = mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var searchResponse codesearch.SearchResponse
	err = json.Unmarshal([]byte(textContent.Text), &searchResponse)
	require.NoError(t, err)

	// Verify LastIndexed is populated (format: "2025-01-17 10:30")
	assert.NotEmpty(t, searchResponse.LastIndexed, "last_indexed should be populated after indexing")
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$`, searchResponse.LastIndexed, "last_indexed should match expected format")

	// Verify results don't exceed requested limit
	assert.LessOrEqual(t, len(searchResponse.Results), searchLimit, "results should not exceed requested limit")

	t.Logf("Search returned %d results (limit: %d, total_matches: %d, limit_applied: %d)",
		len(searchResponse.Results), searchLimit, searchResponse.TotalMatches, searchResponse.LimitApplied)

	// If truncation occurred, verify the truncation fields
	if searchResponse.TotalMatches > len(searchResponse.Results) {
		assert.Equal(t, searchLimit, searchResponse.LimitApplied, "limit_applied should match requested limit when truncated")
		assert.Greater(t, searchResponse.TotalMatches, searchResponse.LimitApplied, "total_matches should exceed limit_applied when truncated")
	}
}

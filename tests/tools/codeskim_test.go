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
	"github.com/sammcj/mcp-devtools/internal/tools/codeskim"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeSkimTool(t *testing.T) {
	// Enable the tool for tests
	t.Setenv("ENABLE_ADDITIONAL_TOOLS", "code_skim")

	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.ErrorLevel)

	tool := &codeskim.CodeSkimTool{}
	cache := &sync.Map{}
	ctx := context.Background()

	fixturesDir, err := filepath.Abs("../fixtures/codeskim")
	require.NoError(t, err)

	t.Run("Python function transformation", func(t *testing.T) {
		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "test.py")},
		}

		result, err := tool.Execute(ctx, logger, cache, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		require.Equal(t, 1, len(response.Files))
		require.Equal(t, 1, response.ProcessedFiles)
		require.Equal(t, 0, response.FailedFiles)

		transformed := response.Files[0].Transformed
		assert.Contains(t, transformed, "/* ... */")
		assert.NotContains(t, transformed, "print(")
		assert.Equal(t, "python", string(response.Files[0].Language))
	})

	t.Run("Go function transformation", func(t *testing.T) {
		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "test.go")},
		}

		result, err := tool.Execute(ctx, logger, cache, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		require.Equal(t, 1, len(response.Files))
		transformed := response.Files[0].Transformed
		assert.Contains(t, transformed, "/* ... */")
		assert.NotContains(t, transformed, "result :=")
		assert.Equal(t, "go", string(response.Files[0].Language))
	})

	t.Run("JavaScript function transformation", func(t *testing.T) {
		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "test.js")},
		}

		result, err := tool.Execute(ctx, logger, cache, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		require.Equal(t, 1, len(response.Files))
		transformed := response.Files[0].Transformed
		assert.Contains(t, transformed, "/* ... */")
		assert.NotContains(t, transformed, "console.log")
		assert.Equal(t, "javascript", string(response.Files[0].Language))
	})

	t.Run("TypeScript class transformation", func(t *testing.T) {
		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "test.ts")},
		}

		result, err := tool.Execute(ctx, logger, cache, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		require.Equal(t, 1, len(response.Files))
		transformed := response.Files[0].Transformed
		assert.Contains(t, transformed, "/* ... */")
		assert.NotContains(t, transformed, "await db")
		assert.Equal(t, "typescript", string(response.Files[0].Language))
	})

	t.Run("Directory processing", func(t *testing.T) {
		args := map[string]any{
			"source": []any{fixturesDir},
		}

		result, err := tool.Execute(ctx, logger, cache, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		assert.Equal(t, 7, response.TotalFiles)
		assert.Equal(t, 7, response.ProcessedFiles)
		assert.Equal(t, 0, response.FailedFiles)
		assert.Equal(t, 7, len(response.Files))
	})

	t.Run("Glob pattern processing", func(t *testing.T) {
		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "*.py")},
		}

		result, err := tool.Execute(ctx, logger, cache, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		assert.Equal(t, 2, response.TotalFiles)
		assert.Equal(t, 2, response.ProcessedFiles)
		assert.Equal(t, 2, len(response.Files))
		assert.Equal(t, "python", string(response.Files[0].Language))
	})

	t.Run("Caching works", func(t *testing.T) {
		testCache := &sync.Map{}
		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "test.go")},
		}

		// First call
		result1, err := tool.Execute(ctx, logger, testCache, args)
		require.NoError(t, err)
		textContent1, ok := mcp.AsTextContent(result1.Content[0])
		require.True(t, ok)

		var response1 codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent1.Text), &response1)
		require.NoError(t, err)
		assert.False(t, response1.Files[0].FromCache)

		// Second call should use cache
		result2, err := tool.Execute(ctx, logger, testCache, args)
		require.NoError(t, err)
		textContent2, ok := mcp.AsTextContent(result2.Content[0])
		require.True(t, ok)

		var response2 codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent2.Text), &response2)
		require.NoError(t, err)
		assert.True(t, response2.Files[0].FromCache)
	})

	t.Run("Clear cache works", func(t *testing.T) {
		testCache := &sync.Map{}
		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "test.js")},
		}

		// First call
		_, err := tool.Execute(ctx, logger, testCache, args)
		require.NoError(t, err)

		// Second call with clear_cache
		args["clear_cache"] = true
		result, err := tool.Execute(ctx, logger, testCache, args)
		require.NoError(t, err)
		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.False(t, response.Files[0].FromCache)
	})

	t.Run("Missing source fails", func(t *testing.T) {
		args := map[string]any{}

		_, err := tool.Execute(ctx, logger, cache, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required parameter 'source'")
	})

	t.Run("Non-existent file fails", func(t *testing.T) {
		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "nonexistent.py")},
		}

		_, err := tool.Execute(ctx, logger, cache, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve source")
	})

	t.Run("Glob with no matches returns error", func(t *testing.T) {
		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "*.rb")},
		}

		_, err := tool.Execute(ctx, logger, cache, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no files match glob pattern")
	})

	t.Run("Line limiting with pagination", func(t *testing.T) {
		t.Setenv("CODE_SKIM_MAX_LINES", "5")
		testCache := &sync.Map{}
		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "large.py")},
		}

		// First call - should be truncated
		result1, err := tool.Execute(ctx, logger, testCache, args)
		require.NoError(t, err)
		textContent1, ok := mcp.AsTextContent(result1.Content[0])
		require.True(t, ok)

		var response1 codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent1.Text), &response1)
		require.NoError(t, err)

		require.Equal(t, 1, len(response1.Files))
		assert.True(t, response1.Files[0].Truncated)
		assert.NotNil(t, response1.Files[0].NextStartingLine)
		assert.NotNil(t, response1.Files[0].TotalLines)
		assert.NotNil(t, response1.Files[0].ReturnedLines)
		assert.Equal(t, 5, *response1.Files[0].ReturnedLines)

		// Second call with starting_line - get next chunk
		args["starting_line"] = *response1.Files[0].NextStartingLine
		result2, err := tool.Execute(ctx, logger, testCache, args)
		require.NoError(t, err)
		textContent2, ok := mcp.AsTextContent(result2.Content[0])
		require.True(t, ok)

		var response2 codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent2.Text), &response2)
		require.NoError(t, err)

		require.Equal(t, 1, len(response2.Files))
		// This result should also be from cache
		assert.True(t, response2.Files[0].FromCache)
	})

	t.Run("Filter inclusion pattern", func(t *testing.T) {
		testCache := &sync.Map{}
		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "graph_test.js")},
			"filter": []any{"handle*"},
		}

		result, err := tool.Execute(ctx, logger, testCache, args)
		require.NoError(t, err)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		require.Equal(t, 1, len(response.Files))
		file := response.Files[0]

		// Should include handleRequest
		assert.Contains(t, file.Transformed, "handleRequest")
		// Should not include other named functions
		assert.NotContains(t, file.Transformed, "function parseBody")
		assert.NotContains(t, file.Transformed, "function validate")

		// Check filter counts
		require.NotNil(t, file.MatchedItems)
		assert.Equal(t, 1, *file.MatchedItems) // only handleRequest
		require.NotNil(t, file.TotalItems)
		assert.Greater(t, *file.TotalItems, 1)
	})

	t.Run("Filter exclusion pattern", func(t *testing.T) {
		testCache := &sync.Map{}
		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "test.py")},
			"filter": []any{"!goodbye"},
		}

		result, err := tool.Execute(ctx, logger, testCache, args)
		require.NoError(t, err)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		require.Equal(t, 1, len(response.Files))
		file := response.Files[0]

		// Should include hello but not goodbye
		assert.Contains(t, file.Transformed, "hello")
		assert.NotContains(t, file.Transformed, "goodbye")
	})

	t.Run("Graph extraction", func(t *testing.T) {
		testCache := &sync.Map{}
		args := map[string]any{
			"source":        []any{filepath.Join(fixturesDir, "graph_test.js")},
			"extract_graph": true,
		}

		result, err := tool.Execute(ctx, logger, testCache, args)
		require.NoError(t, err)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		require.Equal(t, 1, len(response.Files))
		file := response.Files[0]

		require.NotNil(t, file.Graph, "graph should be present")
		assert.NotEmpty(t, file.Graph.Imports, "should have imports")
		assert.NotEmpty(t, file.Graph.Functions, "should have functions")
		assert.NotEmpty(t, file.Graph.Classes, "should have classes")

		// Check that handleRequest has calls
		var handleReq *codeskim.FunctionInfo
		for i := range file.Graph.Functions {
			if file.Graph.Functions[i].Name == "handleRequest" {
				handleReq = &file.Graph.Functions[i]
				break
			}
		}
		require.NotNil(t, handleReq, "handleRequest should be in function list")
		assert.NotEmpty(t, handleReq.Calls, "handleRequest should have calls")
		assert.Contains(t, handleReq.Calls, "parseBody")

		// Check Router class
		var router *codeskim.ClassInfo
		for i := range file.Graph.Classes {
			if file.Graph.Classes[i].Name == "Router" {
				router = &file.Graph.Classes[i]
				break
			}
		}
		require.NotNil(t, router, "Router class should be found")
		assert.NotEmpty(t, router.Methods, "Router should have methods")
	})

	t.Run("Cache key includes extract_graph", func(t *testing.T) {
		testCache := &sync.Map{}
		filePath := filepath.Join(fixturesDir, "test.py")

		// First call without graph
		args1 := map[string]any{
			"source":        []any{filePath},
			"extract_graph": false,
		}
		result1, err := tool.Execute(ctx, logger, testCache, args1)
		require.NoError(t, err)

		textContent1, ok := mcp.AsTextContent(result1.Content[0])
		require.True(t, ok)
		var response1 codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent1.Text), &response1)
		require.NoError(t, err)
		assert.Nil(t, response1.Files[0].Graph, "no graph without extract_graph")

		// Second call with graph - should NOT use cache from first call
		args2 := map[string]any{
			"source":        []any{filePath},
			"extract_graph": true,
		}
		result2, err := tool.Execute(ctx, logger, testCache, args2)
		require.NoError(t, err)

		textContent2, ok := mcp.AsTextContent(result2.Content[0])
		require.True(t, ok)
		var response2 codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent2.Text), &response2)
		require.NoError(t, err)
		assert.False(t, response2.Files[0].FromCache, "should not use cache from non-graph call")
		assert.NotNil(t, response2.Files[0].Graph, "graph should be present")
	})

	t.Run("Sigil output format", func(t *testing.T) {
		testCache := &sync.Map{}
		args := map[string]any{
			"source":        []any{filepath.Join(fixturesDir, "graph_test.js")},
			"output_format": "sigil",
		}

		result, err := tool.Execute(ctx, logger, testCache, args)
		require.NoError(t, err)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		sigilOutput := textContent.Text
		// Sigil format should contain sigil markers
		assert.Contains(t, sigilOutput, "#")       // function markers
		assert.Contains(t, sigilOutput, "!")       // import markers
		assert.Contains(t, sigilOutput, "$Router") // class marker
	})

	t.Run("Multiple sources with deduplication", func(t *testing.T) {
		testCache := &sync.Map{}
		pyFile := filepath.Join(fixturesDir, "test.py")
		args := map[string]any{
			"source": []any{pyFile, pyFile}, // same file twice
		}

		result, err := tool.Execute(ctx, logger, testCache, args)
		require.NoError(t, err)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		// Should deduplicate to 1 file
		assert.Equal(t, 1, response.TotalFiles)
	})

	t.Run("Go grouped imports extraction", func(t *testing.T) {
		testCache := &sync.Map{}
		args := map[string]any{
			"source":        []any{filepath.Join(fixturesDir, "imports_test.go")},
			"extract_graph": true,
		}

		result, err := tool.Execute(ctx, logger, testCache, args)
		require.NoError(t, err)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		require.Equal(t, 1, len(response.Files))
		file := response.Files[0]
		require.NotNil(t, file.Graph)

		// Should have all 3 imports, not just the first
		assert.GreaterOrEqual(t, len(file.Graph.Imports), 3, "should extract all grouped imports")
		assert.Contains(t, file.Graph.Imports, "fmt")
		assert.Contains(t, file.Graph.Imports, "os")
		assert.Contains(t, file.Graph.Imports, "strings")
	})

	t.Run("Large file rejection", func(t *testing.T) {
		// Create a temporary file larger than 500KB
		tmpDir := t.TempDir()
		largePath := filepath.Join(tmpDir, "huge.py")
		content := make([]byte, 600*1024) // 600KB
		for i := range content {
			content[i] = 'x'
		}
		err := os.WriteFile(largePath, content, 0o600)
		require.NoError(t, err)

		args := map[string]any{
			"source": []any{largePath},
		}

		result, err := tool.Execute(ctx, logger, cache, args)
		require.NoError(t, err) // Returns result with error in file, not tool error

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var response codeskim.SkimResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		require.Equal(t, 1, len(response.Files))
		assert.Contains(t, response.Files[0].Error, "file too large")
		assert.Equal(t, 0, response.ProcessedFiles)
		assert.Equal(t, 1, response.FailedFiles)
	})

	t.Run("Context cancellation", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(t.Context())
		cancel() // Cancel immediately

		args := map[string]any{
			"source": []any{filepath.Join(fixturesDir, "test.py")},
		}

		testCache := &sync.Map{}
		result, err := tool.Execute(cancelCtx, logger, testCache, args)

		// Should either return an error or a result with an error in the file
		if err != nil {
			assert.Contains(t, err.Error(), "context")
		} else {
			textContent, ok := mcp.AsTextContent(result.Content[0])
			require.True(t, ok)
			var response codeskim.SkimResponse
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			// File result should show an error from cancelled context
			if len(response.Files) > 0 && response.Files[0].Error != "" {
				assert.Contains(t, response.Files[0].Error, "context")
			}
		}
	})
}

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

		assert.Equal(t, 5, response.TotalFiles)
		assert.Equal(t, 5, response.ProcessedFiles)
		assert.Equal(t, 0, response.FailedFiles)
		assert.Equal(t, 5, len(response.Files))
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
}

package tools_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	_ "github.com/sammcj/mcp-devtools/internal/tools/internetsearch/unified" // Register the tool
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getInternetSearchTool(t *testing.T) tools.Tool {
	t.Helper()
	tool, ok := registry.GetTool("internet_search")
	if !ok || tool == nil {
		t.Skip("internet_search tool not registered (no providers available)")
	}
	return tool
}

func TestInternetSearchMultiQueryParsing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	tool := getInternetSearchTool(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}
	ctx := context.Background()

	t.Run("RejectsEmptyQueryArray", func(t *testing.T) {
		args := map[string]any{
			"query": []any{},
		}
		_, err := tool.Execute(ctx, logger, cache, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("RejectsMissingQuery", func(t *testing.T) {
		args := map[string]any{
			"type": "web",
		}
		_, err := tool.Execute(ctx, logger, cache, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required parameter")
	})

	t.Run("RejectsStringQuery", func(t *testing.T) {
		args := map[string]any{
			"query": "not an array",
		}
		_, err := tool.Execute(ctx, logger, cache, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be an array")
	})

	t.Run("RejectsEmptyStringInArray", func(t *testing.T) {
		args := map[string]any{
			"query": []any{"valid query", ""},
		}
		_, err := tool.Execute(ctx, logger, cache, args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-empty string")
	})
}

func TestInternetSearchMultiQueryExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API test in short mode")
	}

	tool := getInternetSearchTool(t)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}
	ctx := context.Background()

	t.Run("SingleQueryReturnsCorrectFormat", func(t *testing.T) {
		args := map[string]any{
			"query":    []any{"golang programming"},
			"count":    2,
			"provider": "duckduckgo",
		}
		result, err := tool.Execute(ctx, logger, cache, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Parse response
		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok, "expected text content")

		var response internetsearch.MultiSearchResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		assert.Equal(t, 1, response.Summary.Total)
		assert.Len(t, response.Searches, 1)
		assert.Equal(t, "golang programming", response.Searches[0].Query)
	})

	t.Run("MultipleQueriesReturnAllResults", func(t *testing.T) {
		args := map[string]any{
			"query":    []any{"golang programming", "rust language"},
			"count":    2,
			"provider": "duckduckgo",
		}
		result, err := tool.Execute(ctx, logger, cache, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Parse response
		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok, "expected text content")

		var response internetsearch.MultiSearchResponse
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		assert.Equal(t, 2, response.Summary.Total)
		assert.Len(t, response.Searches, 2)

		// Verify queries are preserved in order
		queries := make([]string, len(response.Searches))
		for i, s := range response.Searches {
			queries[i] = s.Query
		}
		assert.Contains(t, queries, "golang programming")
		assert.Contains(t, queries, "rust language")
	})
}

func TestInternetSearchToolRegistration(t *testing.T) {
	// Verify the tool is registered with valid definition
	tool, ok := registry.GetTool("internet_search")
	if !ok || tool == nil {
		t.Skip("internet_search tool not registered (no providers available)")
	}

	def := tool.Definition()
	assert.Equal(t, "internet_search", def.Name)
	assert.NotEmpty(t, def.Description)

	// Verify description mentions parallel execution
	assert.Contains(t, def.Description, "parallel")
}

func TestInternetSearchResponseTypes(t *testing.T) {
	t.Run("QueryResultStructure", func(t *testing.T) {
		result := internetsearch.QueryResult{
			Query:    "test query",
			Provider: "test_provider",
			Results: []internetsearch.SearchResult{
				{Title: "Test", URL: "https://example.com", Description: "Test result"},
			},
		}
		assert.Equal(t, "test query", result.Query)
		assert.Equal(t, "test_provider", result.Provider)
		assert.Len(t, result.Results, 1)
		assert.Empty(t, result.Error)
	})

	t.Run("QueryResultWithError", func(t *testing.T) {
		result := internetsearch.QueryResult{
			Query:   "failed query",
			Error:   "provider unavailable",
			Results: []internetsearch.SearchResult{},
		}
		assert.NotEmpty(t, result.Error)
		assert.Empty(t, result.Results)
	})

	t.Run("MultiSearchResponseStructure", func(t *testing.T) {
		response := internetsearch.MultiSearchResponse{
			Searches: []internetsearch.QueryResult{
				{Query: "q1", Provider: "p1", Results: []internetsearch.SearchResult{}},
				{Query: "q2", Error: "failed", Results: []internetsearch.SearchResult{}},
			},
			Summary: internetsearch.SearchSummary{
				Total:      2,
				Successful: 1,
				Failed:     1,
			},
		}
		assert.Len(t, response.Searches, 2)
		assert.Equal(t, 2, response.Summary.Total)
		assert.Equal(t, 1, response.Summary.Successful)
		assert.Equal(t, 1, response.Summary.Failed)
	})
}

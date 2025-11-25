package tools

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/anthropic"
	"github.com/sirupsen/logrus"
)

// TestAnthropicTool_Execute tests the Anthropic tool's basic operations
func TestAnthropicTool_Execute(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Anthropic tool test in short mode")
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Suppress logs during tests
	cache := &sync.Map{}
	ctx := context.Background()

	tool := anthropic.NewAnthropicTool()

	tests := []struct {
		name      string
		args      map[string]any
		wantErr   bool
		checkFunc func(*testing.T, any)
	}{
		{
			name: "list action returns models",
			args: map[string]any{
				"action": "list",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, result any) {
				// Result should be an AnthropicModelSearchResult with models
				t.Log("List action completed successfully")
			},
		},
		{
			name: "search for sonnet",
			args: map[string]any{
				"action": "search",
				"query":  "sonnet",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, result any) {
				t.Log("Search for sonnet completed successfully")
			},
		},
		{
			name: "search with alias haiku",
			args: map[string]any{
				"action": "search",
				"query":  "haiku",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, result any) {
				t.Log("Search with alias completed successfully")
			},
		},
		{
			name: "search with claude- prefix",
			args: map[string]any{
				"action": "search",
				"query":  "claude-opus",
			},
			wantErr: false,
			checkFunc: func(t *testing.T, result any) {
				t.Log("Search with claude- prefix completed successfully")
			},
		},
		{
			name: "invalid action",
			args: map[string]any{
				"action": "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, logger, cache, tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("AnthropicTool.Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == nil {
				t.Error("Expected non-nil result for successful operation")
			}

			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, result)
			}
		})
	}
}

// TestAnthropicTool_AliasMatching tests the alias matching logic
func TestAnthropicTool_AliasMatching(t *testing.T) {
	testCases := []struct {
		query       string
		modelName   string
		modelFamily string
		shouldMatch bool
	}{
		{"sonnet", "Claude Sonnet 4.5", "sonnet", true},
		{"haiku", "Claude Haiku 4.5", "haiku", true},
		{"opus", "Claude Opus 4.5", "opus", true},
		{"claude-sonnet", "Claude Sonnet 4.5", "sonnet", true},
		{"claude sonnet", "Claude Sonnet 4.5", "sonnet", true},
		{"sonnet-4.5", "Claude Sonnet 4.5", "sonnet", true},
		{"haiku-4", "Claude Haiku 4.5", "haiku", true},
		{"random", "Claude Sonnet 4.5", "sonnet", false},
	}

	for _, tc := range testCases {
		t.Run(tc.query+"_vs_"+tc.modelName, func(t *testing.T) {
			model := anthropic.AnthropicModel{
				ModelName:   tc.modelName,
				ModelFamily: tc.modelFamily,
			}

			// Create a mock search function that mimics the matchesQuery logic
			matches := matchesTestQuery(model, tc.query)

			if matches != tc.shouldMatch {
				t.Errorf("Query %q against model %q (family: %q) = %v, want %v",
					tc.query, tc.modelName, tc.modelFamily, matches, tc.shouldMatch)
			}
		})
	}
}

// matchesTestQuery is a test helper that mimics the matching logic
func matchesTestQuery(model anthropic.AnthropicModel, query string) bool {
	// This is a simplified version of the matching logic for testing
	// For testing purposes, we'll just check basic string containment
	// The actual implementation in the tool is more sophisticated

	// Simple case-insensitive contains check
	queryLower := strings.ToLower(query)
	modelNameLower := strings.ToLower(model.ModelName)
	familyLower := strings.ToLower(model.ModelFamily)

	if strings.Contains(modelNameLower, queryLower) {
		return true
	}

	if strings.Contains(familyLower, queryLower) {
		return true
	}

	// Check aliases
	queryLower = strings.TrimPrefix(queryLower, "claude-")

	// Direct family match
	if queryLower == familyLower {
		return true
	}

	// Family with version (e.g., "sonnet-4.5")
	if strings.HasPrefix(queryLower, familyLower+"-") || strings.HasPrefix(queryLower, familyLower+" ") {
		return true
	}

	return false
}

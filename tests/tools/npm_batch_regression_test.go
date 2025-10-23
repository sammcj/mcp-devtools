package tools

import (
	"context"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions/npm"
	"github.com/sirupsen/logrus"
)

// TestNpmBatchRequestRegression tests the exact scenario from the bug report
// where multiple npm packages failed with "unexpected end of JSON input"
func TestNpmBatchRequestRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping external npm registry test in short mode")
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise
	cache := &sync.Map{}

	tool := npm.NewNpmTool(nil) // Use default client

	// Exact payload from bug report
	args := map[string]any{
		"dependencies": map[string]any{
			"typescript":                       "^4.9.5",
			"@typescript-eslint/eslint-plugin": "^7.13.0",
			"@typescript-eslint/parser":        "^7.13.0",
			"eslint":                           "^8.57.0",
			"jest":                             "29.7.0",
			"ts-jest":                          "29.3.0",
			"@types/node":                      "22.14.1",
			"browser-sync":                     "3.0.4",
			"concurrently":                     "9.1.2",
		},
	}

	result, err := tool.Execute(context.Background(), logger, cache, args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Parse result to check for failures
	if len(result.Content) == 0 {
		t.Fatal("Expected content in result")
	}

	// Extract text from MCP content
	// mcp.TextContent has a Text field (not method)
	resultText := ""
	for _, content := range result.Content {
		if tc, ok := content.(mcp.TextContent); ok {
			resultText += tc.Text
		} else {
			t.Logf("Unexpected content type: %T", content)
		}
	}

	if len(resultText) == 0 {
		t.Fatalf("Expected non-empty result text, got %d content items", len(result.Content))
	}

	// Verify no packages have "unknown" version due to parse errors
	if contains(resultText, `"latestVersion": "unknown"`) && contains(resultText, "unexpected end of JSON input") {
		t.Errorf("Bug still present: packages failing with 'unexpected end of JSON input'")
	}

	// Verify we got valid results for at least some packages
	if !contains(resultText, `"latestVersion"`) {
		t.Error("Expected to find latestVersion in results")
	}

	t.Logf("Successfully processed batch npm request with %d packages", len(args["dependencies"].(map[string]any)))
}

// TestNpmBatchRequestConcurrent tests concurrent batch requests to verify thread safety
func TestNpmBatchRequestConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent external npm registry test in short mode")
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	tool := npm.NewNpmTool(nil)

	// Run multiple concurrent batch requests
	const numGoroutines = 5
	errChan := make(chan error, numGoroutines)
	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Add(1)
		go func(routineNum int) {
			defer wg.Done()

			args := map[string]any{
				"dependencies": map[string]any{
					"react":  "^18.0.0",
					"lodash": "^4.17.21",
					"axios":  "^1.0.0",
				},
			}

			result, err := tool.Execute(context.Background(), logger, cache, args)
			if err != nil {
				errChan <- err
				return
			}

			if result == nil {
				errChan <- nil
				return
			}

			// Verify result has content
			if len(result.Content) == 0 {
				errChan <- nil
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	errorCount := 0
	for err := range errChan {
		if err != nil {
			t.Errorf("Concurrent request failed: %v", err)
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("%d out of %d concurrent requests failed", errorCount, numGoroutines)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

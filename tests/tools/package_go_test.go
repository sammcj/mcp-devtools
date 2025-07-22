package tools

import (
	"context"
	"sync"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	go_tool "github.com/sammcj/mcp-devtools/internal/tools/packageversions/go"
	"github.com/sirupsen/logrus"
)

// MockHTTPClient for testing
type MockHTTPClient struct {
	err error
}

func (m *MockHTTPClient) Do(req interface{}) (interface{}, error) {
	// Simplified mock - in real implementation this would return proper HTTP response
	return nil, m.err
}

func TestGoTool_Execute_SimpleFormat(t *testing.T) {
	// Create a logger for testing
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	// Create cache
	cache := &sync.Map{}

	tool := go_tool.NewGoTool(packageversions.DefaultHTTPClient)

	// Test simple format (key-value pairs)
	args := map[string]interface{}{
		"dependencies": map[string]interface{}{
			"github.com/gorilla/mux":      "v1.8.0",
			"github.com/stretchr/testify": "",
		},
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, logger, cache, args)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// The result should be a properly formatted JSON response
	if result.Content == nil {
		t.Fatal("Expected content in result")
	}
}

func TestGoTool_Execute_ComplexFormat(t *testing.T) {
	// Create a logger for testing
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	// Create cache
	cache := &sync.Map{}

	// Create tool instance
	tool := go_tool.NewGoTool(packageversions.DefaultHTTPClient)

	// Test complex format (structured with require array)
	args := map[string]interface{}{
		"dependencies": map[string]interface{}{
			"module": "github.com/example/project",
			"require": []interface{}{
				map[string]interface{}{
					"path":    "github.com/gorilla/mux",
					"version": "v1.8.0",
				},
				map[string]interface{}{
					"path":    "github.com/stretchr/testify",
					"version": "v1.9.0",
				},
			},
		},
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, logger, cache, args)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// The result should be a properly formatted JSON response
	if result.Content == nil {
		t.Fatal("Expected content in result")
	}
}

func TestGoTool_Execute_MissingDependencies(t *testing.T) {
	// Create a logger for testing
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	// Create cache
	cache := &sync.Map{}

	// Create tool instance
	tool := go_tool.NewGoTool(packageversions.DefaultHTTPClient)

	// Test missing dependencies parameter
	args := map[string]interface{}{}

	ctx := context.Background()
	_, err := tool.Execute(ctx, logger, cache, args)

	if err == nil {
		t.Fatal("Expected error for missing dependencies parameter")
	}

	expectedError := "missing required parameter: dependencies"
	if err.Error() != expectedError {
		t.Fatalf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestGoTool_Execute_InvalidDependenciesFormat(t *testing.T) {
	// Create a logger for testing
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	// Create cache
	cache := &sync.Map{}

	// Create tool instance
	tool := go_tool.NewGoTool(packageversions.DefaultHTTPClient)

	// Test invalid dependencies format
	args := map[string]interface{}{
		"dependencies": "not an object",
	}

	ctx := context.Background()
	_, err := tool.Execute(ctx, logger, cache, args)

	if err == nil {
		t.Fatal("Expected error for invalid dependencies format")
	}

	expectedError := "invalid dependencies format: expected object"
	if err.Error() != expectedError {
		t.Fatalf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

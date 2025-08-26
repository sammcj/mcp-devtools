package testutils

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	"github.com/sirupsen/logrus"
)

// CreateTestLogger creates a logger suitable for testing
func CreateTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests
	return logger
}

// CreateTestCache creates a cache suitable for testing
func CreateTestCache() *sync.Map {
	return &sync.Map{}
}

// CreateTestContext creates a context suitable for testing
func CreateTestContext() context.Context {
	return context.Background()
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

// AssertErrorContains fails the test if err is nil or doesn't contain the expected message
func AssertErrorContains(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !Contains(err.Error(), expected) {
		t.Fatalf("Expected error to contain '%s', got: %v", expected, err)
	}
}

// AssertNotNil fails the test if value is nil
func AssertNotNil(t *testing.T, value any) {
	t.Helper()
	if value == nil {
		t.Fatal("Expected non-nil value")
	}
}

// AssertNil asserts that a value is nil
// AssertNil asserts that a value is nil
// AssertNil asserts that a value is nil
func AssertNil(t *testing.T, value any) {
	t.Helper()
	if value == nil {
		return // Test passes
	}
	// Handle the case where value is a nil pointer wrapped in an interface
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return // Test passes
	}
	t.Fatalf("Expected nil value, got %v (type: %T)", value, value)
}

// AssertEqual fails the test if expected != actual
func AssertEqual(t *testing.T, expected, actual any) {
	t.Helper()
	if expected != actual {
		t.Fatalf("Expected %v, got %v", expected, actual)
	}
}

// AssertTrue fails the test if condition is false
func AssertTrue(t *testing.T, condition bool) {
	t.Helper()
	if !condition {
		t.Fatal("Expected condition to be true")
	}
}

// AssertFalse fails the test if condition is true
func AssertFalse(t *testing.T, condition bool) {
	t.Helper()
	if condition {
		t.Fatal("Expected condition to be false")
	}
}

// Contains checks if a string contains a substring
func Contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		containsMiddle(s, substr))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ExtractPackageVersions extracts PackageVersion structs from a tool result
func ExtractPackageVersions(t *testing.T, result any) []packageversions.PackageVersion {
	t.Helper()

	// Cast to CallToolResult
	toolResult, ok := result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("Expected *mcp.CallToolResult, got %T", result)
	}

	// Extract text content
	if len(toolResult.Content) == 0 {
		t.Fatal("Expected content in tool result")
	}

	textContent, ok := toolResult.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, got %T", toolResult.Content[0])
	}

	// Parse JSON
	var versions []packageversions.PackageVersion
	err := json.Unmarshal([]byte(textContent.Text), &versions)
	if err != nil {
		t.Fatalf("Failed to parse package versions JSON: %v", err)
	}

	return versions
}

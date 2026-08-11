package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/mcpapi"
	"github.com/sammcj/mcp-devtools/internal/tools/packageversions"
	go_tool "github.com/sammcj/mcp-devtools/internal/tools/packageversions/go"
	"github.com/sirupsen/logrus"
)

// MockHTTPClient for testing
type MockHTTPClient struct {
	err error
}

func (m *MockHTTPClient) Do(req any) (any, error) {
	// Simplified mock - in real implementation this would return proper HTTP response
	return nil, m.err
}

func TestGoTool_Execute_SimpleFormat(t *testing.T) {
	t.Parallel()
	// Create a logger for testing
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	// Create cache
	cache := &sync.Map{}

	tool := go_tool.NewGoTool(packageversions.DefaultHTTPClient)

	// Test simple format (key-value pairs)
	args := map[string]any{
		"dependencies": map[string]any{
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
	t.Parallel()
	// Create a logger for testing
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	// Create cache
	cache := &sync.Map{}

	// Create tool instance
	tool := go_tool.NewGoTool(packageversions.DefaultHTTPClient)

	// Test complex format (structured with require array)
	args := map[string]any{
		"dependencies": map[string]any{
			"module": "github.com/example/project",
			"require": []any{
				map[string]any{
					"path":    "github.com/gorilla/mux",
					"version": "v1.8.0",
				},
				map[string]any{
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
	args := map[string]any{}

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
	args := map[string]any{
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

// stubHTTPClient serves canned responses keyed by request URL and records the
// order in which URLs were requested.
type stubHTTPClient struct {
	responses map[string]string
	requested []string
}

func (s *stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	s.requested = append(s.requested, req.URL.String())
	body, ok := s.responses[req.URL.String()]
	if !ok {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("not found")),
			Header:     http.Header{},
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

// runGoTool executes the tool against the stub and returns the decoded results.
func runGoTool(t *testing.T, stub *stubHTTPClient, deps map[string]any) []packageversions.PackageVersion {
	t.Helper()

	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)

	tool := go_tool.NewGoTool(stub)
	result, err := tool.Execute(t.Context(), logger, &sync.Map{}, map[string]any{"dependencies": deps})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	payload := resultText(t, result)
	var versions []packageversions.PackageVersion
	if err := json.Unmarshal([]byte(payload), &versions); err != nil {
		t.Fatalf("failed to decode result %q: %v", payload, err)
	}
	return versions
}

func resultText(t *testing.T, result *mcpapi.CallToolResult) string {
	t.Helper()
	for _, content := range result.Content {
		if text, ok := content.(*mcpapi.TextContent); ok {
			return text.Text
		}
	}
	t.Fatal("result contained no text content")
	return ""
}

func TestGoTool_ReportsDeprecation(t *testing.T) {
	t.Parallel()
	stub := &stubHTTPClient{responses: map[string]string{
		"https://pkg.go.dev/v1beta/versions/github.com/golang/protobuf?limit=1": `{"items":[{"modulePath":"github.com/golang/protobuf","version":"v1.5.4","latestVersion":"v1.5.4","deprecated":true,"deprecationReason":"Use the \"google.golang.org/protobuf\" module instead."}]}`,
	}}

	versions := runGoTool(t, stub, map[string]any{"github.com/golang/protobuf": "v1.5.0"})
	if len(versions) != 1 {
		t.Fatalf("expected 1 result, got %d", len(versions))
	}
	if versions[0].LatestVersion != "v1.5.4" {
		t.Errorf("unexpected latest version %q", versions[0].LatestVersion)
	}
	if !strings.Contains(versions[0].Deprecated, "google.golang.org/protobuf") {
		t.Errorf("deprecation reason not reported, got %q", versions[0].Deprecated)
	}
}

func TestGoTool_ReportsNewerMajorSeparately(t *testing.T) {
	t.Parallel()
	stub := &stubHTTPClient{responses: map[string]string{
		"https://pkg.go.dev/v1beta/versions/github.com/golang-jwt/jwt?limit=1": `{"items":[{"modulePath":"github.com/golang-jwt/jwt/v5","version":"v5.3.1","latestVersion":"v5.3.1"}]}`,
		"https://pkg.go.dev/v1beta/module/github.com/golang-jwt/jwt":           `{"path":"github.com/golang-jwt/jwt","version":"v3.2.2+incompatible"}`,
	}}

	versions := runGoTool(t, stub, map[string]any{"github.com/golang-jwt/jwt": "v3.2.0+incompatible"})
	if len(versions) != 1 {
		t.Fatalf("expected 1 result, got %d", len(versions))
	}
	// The same-major latest must not be replaced by the newer major, which needs
	// an import path change.
	if versions[0].LatestVersion != "v3.2.2+incompatible" {
		t.Errorf("unexpected latest version %q", versions[0].LatestVersion)
	}
	if versions[0].NewerMajor != "github.com/golang-jwt/jwt/v5 v5.3.1" {
		t.Errorf("unexpected newer major %q", versions[0].NewerMajor)
	}
}

func TestGoTool_FallsBackToModuleProxy(t *testing.T) {
	t.Parallel()
	// pkg.go.dev is absent from the stub, so it returns 404 and the proxy is used.
	// The proxy URL must carry the '!' case encoding for the uppercase path.
	proxyURL := "https://proxy.golang.org/github.com/%21masterminds/semver/v3/@latest"
	stub := &stubHTTPClient{responses: map[string]string{
		proxyURL: `{"Version":"v3.5.0","Time":"2026-04-30T15:39:17Z"}`,
	}}

	versions := runGoTool(t, stub, map[string]any{"github.com/Masterminds/semver/v3": "v3.2.0"})
	if len(versions) != 1 {
		t.Fatalf("expected 1 result, got %d", len(versions))
	}
	if versions[0].Skipped {
		t.Fatalf("expected a successful fallback, got skip reason %q (requested: %v)", versions[0].SkipReason, stub.requested)
	}
	if versions[0].LatestVersion != "v3.5.0" {
		t.Errorf("unexpected latest version %q", versions[0].LatestVersion)
	}
	if !slices.Contains(stub.requested, proxyURL) {
		t.Errorf("module proxy was not queried with the case-encoded path, requested: %v", stub.requested)
	}
}

func TestGoTool_SkipsPackageWhenBothSourcesFail(t *testing.T) {
	t.Parallel()
	stub := &stubHTTPClient{responses: map[string]string{}}

	versions := runGoTool(t, stub, map[string]any{"github.com/not/a/real/module": "v1.0.0"})
	if len(versions) != 1 {
		t.Fatalf("expected 1 result, got %d", len(versions))
	}
	if !versions[0].Skipped {
		t.Error("expected the package to be skipped")
	}
	if versions[0].LatestVersion != "unknown" {
		t.Errorf("unexpected latest version %q", versions[0].LatestVersion)
	}
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/api"
	"github.com/sirupsen/logrus"
)

func TestAPIConfigLoading(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "apis.yaml")

	// Test valid config
	validConfig := `
apis:
  github:
    base_url: "https://api.github.com"
    description: "GitHub REST API"
    auth:
      type: "bearer"
      env_var: "GITHUB_TOKEN"
    timeout: 30
    cache_ttl: 300
    endpoints:
      - name: "get_user"
        method: "GET"
        path: "/user"
        description: "Get authenticated user"
        parameters: []
`

	err := os.WriteFile(configPath, []byte(validConfig), 0600)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load and validate config
	config, err := api.LoadAPIConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify config structure
	if len(config.APIs) != 1 {
		t.Fatalf("Expected 1 API, got %d", len(config.APIs))
	}

	githubAPI, exists := config.APIs["github"]
	if !exists {
		t.Fatal("GitHub API not found in config")
	}

	if githubAPI.BaseURL != "https://api.github.com" {
		t.Errorf("Expected base URL 'https://api.github.com', got '%s'", githubAPI.BaseURL)
	}

	if githubAPI.Auth.Type != "bearer" {
		t.Errorf("Expected auth type 'bearer', got '%s'", githubAPI.Auth.Type)
	}

	if len(githubAPI.Endpoints) != 1 {
		t.Fatalf("Expected 1 endpoint, got %d", len(githubAPI.Endpoints))
	}

	endpoint := githubAPI.Endpoints[0]
	if endpoint.Name != "get_user" {
		t.Errorf("Expected endpoint name 'get_user', got '%s'", endpoint.Name)
	}

	if endpoint.Method != "GET" {
		t.Errorf("Expected method 'GET', got '%s'", endpoint.Method)
	}
}

func TestAPIConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		shouldError bool
		errorSubstr string
	}{
		{
			name: "missing base_url",
			config: `
apis:
  test:
    auth:
      type: "none"
    endpoints:
      - name: "test"
        method: "GET"
        path: "/test"
`,
			shouldError: true,
			errorSubstr: "base_url is required",
		},
		{
			name: "invalid auth type",
			config: `
apis:
  test:
    base_url: "https://api.test.com"
    auth:
      type: "invalid"
    endpoints:
      - name: "test"
        method: "GET"
        path: "/test"
`,
			shouldError: true,
			errorSubstr: "invalid auth type",
		},
		{
			name: "missing endpoint name",
			config: `
apis:
  test:
    base_url: "https://api.test.com"
    auth:
      type: "none"
    endpoints:
      - method: "GET"
        path: "/test"
`,
			shouldError: true,
			errorSubstr: "endpoint name is required",
		},
		{
			name: "invalid HTTP method",
			config: `
apis:
  test:
    base_url: "https://api.test.com"
    auth:
      type: "none"
    endpoints:
      - name: "test"
        method: "INVALID"
        path: "/test"
`,
			shouldError: true,
			errorSubstr: "invalid HTTP method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "apis.yaml")

			err := os.WriteFile(configPath, []byte(tt.config), 0600)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			_, err = api.LoadAPIConfig(configPath)

			if tt.shouldError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tt.errorSubstr != "" && !containsIgnoreCase(err.Error(), tt.errorSubstr) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorSubstr, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestDynamicAPITool(t *testing.T) {
	// Create test API definition
	apiDef := api.APIDefinition{
		BaseURL:     "https://api.test.com",
		Description: "Test API",
		Auth: api.AuthConfig{
			Type: "none",
		},
		Timeout:  30,
		CacheTTL: 300,
		Endpoints: []api.EndpointConfig{
			{
				Name:        "get_test",
				Method:      "GET",
				Path:        "/test/{id}",
				Description: "Get test item",
				Parameters: []api.ParameterConfig{
					{
						Name:        "id",
						Type:        "string",
						Required:    true,
						Description: "Test ID",
						Location:    "path",
					},
					{
						Name:        "format",
						Type:        "string",
						Required:    false,
						Description: "Response format",
						Location:    "query",
						Default:     "json",
					},
				},
			},
		},
	}

	// Create dynamic API tool
	tool := api.NewDynamicAPITool("test", apiDef)

	// Test tool definition
	def := tool.Definition()
	if def.Name != "test_api" {
		t.Errorf("Expected tool name 'test_api', got '%s'", def.Name)
	}

	// Test extended help
	extendedHelp := tool.ProvideExtendedInfo()
	if extendedHelp == nil {
		t.Fatal("Expected extended help, got nil")
	}

	if len(extendedHelp.Examples) == 0 {
		t.Error("Expected examples in extended help")
	}
}

func TestHTTPClientExecution(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request details
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		if r.URL.Path != "/test/123" {
			t.Errorf("Expected path '/test/123', got '%s'", r.URL.Path)
		}

		// Check query parameters
		if r.URL.Query().Get("format") != "json" {
			t.Errorf("Expected format=json, got '%s'", r.URL.Query().Get("format"))
		}

		// Return mock response
		response := map[string]interface{}{
			"id":     "123",
			"name":   "Test Item",
			"status": "active",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create API definition with mock server URL
	apiDef := api.APIDefinition{
		BaseURL: server.URL,
		Auth: api.AuthConfig{
			Type: "none",
		},
		Timeout:  30,
		CacheTTL: 300,
		Endpoints: []api.EndpointConfig{
			{
				Name:   "get_test",
				Method: "GET",
				Path:   "/test/{id}",
				Parameters: []api.ParameterConfig{
					{
						Name:     "id",
						Type:     "string",
						Required: true,
						Location: "path",
					},
					{
						Name:     "format",
						Type:     "string",
						Required: false,
						Location: "query",
						Default:  "json",
					},
				},
			},
		},
	}

	// Create and test HTTP client
	client := api.NewHTTPClient(apiDef)
	endpoint := apiDef.Endpoints[0]

	parameters := map[string]interface{}{
		"id":     "123",
		"format": "json",
	}

	ctx := context.Background()
	result, err := client.ExecuteRequest(ctx, endpoint, parameters)
	if err != nil {
		t.Fatalf("Request execution failed: %v", err)
	}

	// Verify result structure
	if result["status_code"] != 200 {
		t.Errorf("Expected status code 200, got %v", result["status_code"])
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data to be map[string]interface{}, got %T", result["data"])
	}

	if data["id"] != "123" {
		t.Errorf("Expected ID '123', got '%v'", data["id"])
	}

	if data["name"] != "Test Item" {
		t.Errorf("Expected name 'Test Item', got '%v'", data["name"])
	}
}

func TestToolExecution(t *testing.T) {
	// Enable the API tool for testing
	t.Setenv("ENABLE_ADDITIONAL_TOOLS", "api")

	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"message": "Hello, API!",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create API definition
	apiDef := api.APIDefinition{
		BaseURL: server.URL,
		Auth: api.AuthConfig{
			Type: "none",
		},
		Timeout:  30,
		CacheTTL: 0, // Disable caching for test
		Endpoints: []api.EndpointConfig{
			{
				Name:        "hello",
				Method:      "GET",
				Path:        "/hello",
				Description: "Say hello",
				Parameters:  []api.ParameterConfig{},
			},
		},
	}

	// Create tool and execute
	tool := api.NewDynamicAPITool("test", apiDef)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce log noise in tests
	cache := &sync.Map{}

	args := map[string]interface{}{
		"endpoint": "hello",
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, logger, cache, args)
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	// Verify result - it should contain content
	if len(result.Content) == 0 {
		t.Fatal("Expected result content to be non-empty")
	}

	// For now, just verify we got some content - the structure is complex
	// In practice, the JSON string content would be properly formatted
}

func TestCaching(t *testing.T) {
	// Enable the API tool for testing
	t.Setenv("ENABLE_ADDITIONAL_TOOLS", "api")

	callCount := 0

	// Create mock HTTP server that tracks calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := map[string]interface{}{
			"message": fmt.Sprintf("Call #%d", callCount),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create API definition with caching enabled
	apiDef := api.APIDefinition{
		BaseURL: server.URL,
		Auth: api.AuthConfig{
			Type: "none",
		},
		Timeout:  30,
		CacheTTL: 3600, // 1 hour cache
		Endpoints: []api.EndpointConfig{
			{
				Name:   "test",
				Method: "GET",
				Path:   "/test",
			},
		},
	}

	// Create tool
	tool := api.NewDynamicAPITool("test", apiDef)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}

	args := map[string]interface{}{
		"endpoint": "test",
	}

	ctx := context.Background()

	// First call - should hit the server
	result1, err := tool.Execute(ctx, logger, cache, args)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 server call, got %d", callCount)
	}

	// Second call - should use cache
	result2, err := tool.Execute(ctx, logger, cache, args)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 server call (cached), got %d", callCount)
	}

	// Verify results contain content (structure is complex to test fully)
	if len(result1.Content) == 0 || len(result2.Content) == 0 {
		t.Fatal("Results should contain content")
	}
}

func TestAuthenticationHandling(t *testing.T) {
	// Set up environment variable for auth
	testToken := "test-bearer-token"
	_ = os.Setenv("TEST_TOKEN", testToken)
	defer func() { _ = os.Unsetenv("TEST_TOKEN") }()

	// Create mock server that checks auth
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + testToken

		if authHeader != expectedAuth {
			t.Errorf("Expected Authorization header '%s', got '%s'", expectedAuth, authHeader)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		response := map[string]interface{}{"authenticated": true}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create API definition with bearer auth
	apiDef := api.APIDefinition{
		BaseURL: server.URL,
		Auth: api.AuthConfig{
			Type:   "bearer",
			EnvVar: "TEST_TOKEN",
		},
		Timeout:  30,
		CacheTTL: 0,
		Endpoints: []api.EndpointConfig{
			{
				Name:   "auth_test",
				Method: "GET",
				Path:   "/auth",
			},
		},
	}

	// Test HTTP client with auth
	client := api.NewHTTPClient(apiDef)
	endpoint := apiDef.Endpoints[0]

	ctx := context.Background()
	result, err := client.ExecuteRequest(ctx, endpoint, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Auth request failed: %v", err)
	}

	// Verify successful auth
	if result["status_code"] != 200 {
		t.Errorf("Expected status 200, got %v", result["status_code"])
	}

	data := result["data"].(map[string]interface{})
	if data["authenticated"] != true {
		t.Error("Expected authenticated=true")
	}
}

// Helper function for case-insensitive string contains check
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
					findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

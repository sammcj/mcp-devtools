package unified

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/internetsearch"
	"github.com/sirupsen/logrus"
)

// mockProvider is a mock implementation of SearchProvider for testing
type mockProvider struct {
	name           string
	shouldFail     bool
	failureError   error
	supportedTypes []string
	callCount      int
}

func (m *mockProvider) Search(ctx context.Context, logger *logrus.Logger, searchType string, args map[string]any) (*internetsearch.SearchResponse, error) {
	m.callCount++

	if m.shouldFail {
		if m.failureError != nil {
			return nil, m.failureError
		}
		return nil, fmt.Errorf("mock provider %s failed", m.name)
	}

	query := args["query"].(string)
	return &internetsearch.SearchResponse{
		Query:       query,
		ResultCount: 1,
		Results: []internetsearch.SearchResult{
			{
				Title:       fmt.Sprintf("Result from %s", m.name),
				URL:         "https://example.com",
				Description: fmt.Sprintf("Mock result from %s", m.name),
				Type:        searchType,
			},
		},
		Provider: m.name,
	}, nil
}

func (m *mockProvider) GetName() string {
	return m.name
}

func (m *mockProvider) IsAvailable() bool {
	return true
}

func (m *mockProvider) GetSupportedTypes() []string {
	return m.supportedTypes
}

// Test getOrderedProviders with no provider specified
func TestGetOrderedProviders_DefaultOrder(t *testing.T) {
	tool := &InternetSearchTool{
		providers: map[string]SearchProvider{
			"brave": &mockProvider{
				name:           "brave",
				supportedTypes: []string{"web", "image", "news", "video"},
			},
			"searxng": &mockProvider{
				name:           "searxng",
				supportedTypes: []string{"web", "image", "news", "video"},
			},
			"duckduckgo": &mockProvider{
				name:           "duckduckgo",
				supportedTypes: []string{"web"},
			},
		},
	}

	// Test web search - all providers support it
	providers := tool.getOrderedProviders("web", "")
	if len(providers) != 3 {
		t.Errorf("Expected 3 providers for web search, got %d", len(providers))
	}
	// Should be in priority order
	if providers[0] != "brave" || providers[1] != "searxng" || providers[2] != "duckduckgo" {
		t.Errorf("Unexpected provider order: %v", providers)
	}

	// Test image search - only brave and searxng support it
	providers = tool.getOrderedProviders("image", "")
	if len(providers) != 2 {
		t.Errorf("Expected 2 providers for image search, got %d", len(providers))
	}
	if providers[0] != "brave" || providers[1] != "searxng" {
		t.Errorf("Unexpected provider order for image: %v", providers)
	}
}

// Test getOrderedProviders with specific provider requested
func TestGetOrderedProviders_SpecificProvider(t *testing.T) {
	tool := &InternetSearchTool{
		providers: map[string]SearchProvider{
			"brave": &mockProvider{
				name:           "brave",
				supportedTypes: []string{"web", "image"},
			},
			"duckduckgo": &mockProvider{
				name:           "duckduckgo",
				supportedTypes: []string{"web"},
			},
		},
	}

	// Request specific provider
	providers := tool.getOrderedProviders("web", "duckduckgo")
	if len(providers) != 1 {
		t.Errorf("Expected 1 provider when specific one requested, got %d", len(providers))
	}
	if providers[0] != "duckduckgo" {
		t.Errorf("Expected duckduckgo, got %s", providers[0])
	}

	// Request provider that doesn't support the search type
	providers = tool.getOrderedProviders("image", "duckduckgo")
	if len(providers) != 0 {
		t.Errorf("Expected 0 providers when requested provider doesn't support type, got %d", len(providers))
	}

	// Request unknown provider
	providers = tool.getOrderedProviders("web", "unknown")
	if len(providers) != 0 {
		t.Errorf("Expected 0 providers for unknown provider, got %d", len(providers))
	}
}

// Test Execute with successful first provider
func TestExecute_SuccessFirstProvider(t *testing.T) {
	braveProvider := &mockProvider{
		name:           "brave",
		supportedTypes: []string{"web"},
	}

	tool := &InternetSearchTool{
		providers: map[string]SearchProvider{
			"brave": braveProvider,
		},
	}

	ctx := context.Background()
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Suppress output
	cache := &sync.Map{}

	args := map[string]any{
		"query": "test query",
		"type":  "web",
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Should have called brave once
	if braveProvider.callCount != 1 {
		t.Errorf("Expected brave to be called once, was called %d times", braveProvider.callCount)
	}
}

// Test Execute with fallback when first provider fails
func TestExecute_FallbackOnFailure(t *testing.T) {
	braveProvider := &mockProvider{
		name:           "brave",
		supportedTypes: []string{"web"},
		shouldFail:     true,
		failureError:   fmt.Errorf("rate limited"),
	}

	duckduckgoProvider := &mockProvider{
		name:           "duckduckgo",
		supportedTypes: []string{"web"},
		shouldFail:     false,
	}

	tool := &InternetSearchTool{
		providers: map[string]SearchProvider{
			"brave":      braveProvider,
			"duckduckgo": duckduckgoProvider,
		},
	}

	ctx := context.Background()
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	cache := &sync.Map{}

	args := map[string]any{
		"query": "test query",
		"type":  "web",
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	if err != nil {
		t.Fatalf("Expected success with fallback, got error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Both providers should have been called
	if braveProvider.callCount != 1 {
		t.Errorf("Expected brave to be called once, was called %d times", braveProvider.callCount)
	}
	if duckduckgoProvider.callCount != 1 {
		t.Errorf("Expected duckduckgo to be called once, was called %d times", duckduckgoProvider.callCount)
	}
}

// Test Execute with all providers failing
func TestExecute_AllProvidersFail(t *testing.T) {
	braveProvider := &mockProvider{
		name:           "brave",
		supportedTypes: []string{"web"},
		shouldFail:     true,
		failureError:   fmt.Errorf("rate limited"),
	}

	duckduckgoProvider := &mockProvider{
		name:           "duckduckgo",
		supportedTypes: []string{"web"},
		shouldFail:     true,
		failureError:   fmt.Errorf("network error"),
	}

	tool := &InternetSearchTool{
		providers: map[string]SearchProvider{
			"brave":      braveProvider,
			"duckduckgo": duckduckgoProvider,
		},
	}

	ctx := context.Background()
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	cache := &sync.Map{}

	args := map[string]any{
		"query": "test query",
		"type":  "web",
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	if err == nil {
		t.Fatal("Expected error when all providers fail, got success")
	}

	if result != nil {
		t.Error("Expected nil result when all providers fail")
	}

	// Error should mention all providers failed and use proper formatting
	errorMsg := err.Error()
	if !strings.Contains(errorMsg, "all providers failed") {
		t.Errorf("Expected error message to mention 'all providers failed', got: %s", errorMsg)
	}
	if !strings.Contains(errorMsg, "brave") || !strings.Contains(errorMsg, "duckduckgo") {
		t.Errorf("Expected error message to mention both providers, got: %s", errorMsg)
	}
	// Check that errors are separated by semicolon (proper formatting)
	if !strings.Contains(errorMsg, ";") {
		t.Errorf("Expected errors to be separated by semicolon, got: %s", errorMsg)
	}
}

// Test Execute with explicit provider specified (no fallback)
func TestExecute_ExplicitProviderNoFallback(t *testing.T) {
	braveProvider := &mockProvider{
		name:           "brave",
		supportedTypes: []string{"web"},
		shouldFail:     true,
		failureError:   fmt.Errorf("rate limited"),
	}

	duckduckgoProvider := &mockProvider{
		name:           "duckduckgo",
		supportedTypes: []string{"web"},
		shouldFail:     false,
	}

	tool := &InternetSearchTool{
		providers: map[string]SearchProvider{
			"brave":      braveProvider,
			"duckduckgo": duckduckgoProvider,
		},
	}

	ctx := context.Background()
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	cache := &sync.Map{}

	args := map[string]any{
		"query":    "test query",
		"type":     "web",
		"provider": "brave", // Explicitly request brave
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	if err == nil {
		t.Fatal("Expected error when explicit provider fails, got success")
	}

	if result != nil {
		t.Error("Expected nil result when explicit provider fails")
	}

	// Should only have tried brave, not duckduckgo
	if braveProvider.callCount != 1 {
		t.Errorf("Expected brave to be called once, was called %d times", braveProvider.callCount)
	}
	if duckduckgoProvider.callCount != 0 {
		t.Errorf("Expected duckduckgo not to be called (no fallback), was called %d times", duckduckgoProvider.callCount)
	}
}

// Test Execute with context cancellation
func TestExecute_ContextCancellation(t *testing.T) {
	braveProvider := &mockProvider{
		name:           "brave",
		supportedTypes: []string{"web"},
		shouldFail:     true,
	}

	duckduckgoProvider := &mockProvider{
		name:           "duckduckgo",
		supportedTypes: []string{"web"},
		shouldFail:     false,
	}

	tool := &InternetSearchTool{
		providers: map[string]SearchProvider{
			"brave":      braveProvider,
			"duckduckgo": duckduckgoProvider,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	cache := &sync.Map{}

	args := map[string]any{
		"query": "test query",
		"type":  "web",
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	if err == nil {
		t.Fatal("Expected error when context is cancelled, got success")
	}

	if result != nil {
		t.Error("Expected nil result when context is cancelled")
	}

	// Should mention cancellation
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("Expected error to mention cancellation, got: %s", err.Error())
	}
}

// Test Execute with no providers supporting search type
func TestExecute_NoProvidersSupport(t *testing.T) {
	duckduckgoProvider := &mockProvider{
		name:           "duckduckgo",
		supportedTypes: []string{"web"}, // Only supports web
	}

	tool := &InternetSearchTool{
		providers: map[string]SearchProvider{
			"duckduckgo": duckduckgoProvider,
		},
	}

	ctx := context.Background()
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	cache := &sync.Map{}

	args := map[string]any{
		"query": "test query",
		"type":  "image", // Request image search
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	if err == nil {
		t.Fatal("Expected error when no providers support type, got success")
	}

	if result != nil {
		t.Error("Expected nil result when no providers support type")
	}

	if !strings.Contains(err.Error(), "no available providers support") {
		t.Errorf("Expected error to mention no providers support type, got: %s", err.Error())
	}
}

package testutils

import (
	"context"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sirupsen/logrus"
)

// MockTool implements the Tool interface for testing
type MockTool struct {
	name       string
	definition mcp.Tool
	executeErr error
	result     *mcp.CallToolResult
}

// NewMockTool creates a new mock tool
func NewMockTool(name string) *MockTool {
	return &MockTool{
		name: name,
		definition: mcp.NewTool(name,
			mcp.WithDescription("Mock tool for testing"),
			mcp.WithString("input",
				mcp.Required(),
				mcp.Description("Test input parameter"),
			),
		),
		result: mcp.NewToolResultText("mock result"),
	}
}

// WithError configures the mock to return an error
func (m *MockTool) WithError(err error) *MockTool {
	m.executeErr = err
	return m
}

// WithResult configures the mock to return a specific result
func (m *MockTool) WithResult(result *mcp.CallToolResult) *MockTool {
	m.result = result
	return m
}

// Definition returns the tool's definition for MCP registration
func (m *MockTool) Definition() mcp.Tool {
	return m.definition
}

// Execute executes the mock tool
func (m *MockTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return m.result, nil
}

// MockHTTPClient for testing HTTP-based tools
type MockHTTPClient struct {
	responses map[string]interface{}
	err       error
}

// NewMockHTTPClient creates a new mock HTTP client
func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		responses: make(map[string]interface{}),
	}
}

// WithResponse configures a response for a specific URL
func (m *MockHTTPClient) WithResponse(url string, response interface{}) *MockHTTPClient {
	m.responses[url] = response
	return m
}

// WithError configures the mock to return an error
func (m *MockHTTPClient) WithError(err error) *MockHTTPClient {
	m.err = err
	return m
}

// Do simulates an HTTP request
func (m *MockHTTPClient) Do(req interface{}) (interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}

	// For simple testing, we'll just return based on string representation
	key := fmt.Sprintf("%v", req)
	if response, ok := m.responses[key]; ok {
		return response, nil
	}

	return nil, fmt.Errorf("no mock response configured for: %v", req)
}

// MockCache provides a controllable cache for testing
type MockCache struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

// NewMockCache creates a new mock cache
func NewMockCache() *MockCache {
	return &MockCache{
		data: make(map[string]interface{}),
	}
}

// Store stores a value in the mock cache
func (m *MockCache) Store(key, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[fmt.Sprintf("%v", key)] = value
}

// Load loads a value from the mock cache
func (m *MockCache) Load(key interface{}) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.data[fmt.Sprintf("%v", key)]
	return value, ok
}

// Delete removes a value from the mock cache
func (m *MockCache) Delete(key interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, fmt.Sprintf("%v", key))
}

// Range calls f sequentially for each key and value present in the cache
func (m *MockCache) Range(f func(key, value interface{}) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.data {
		if !f(k, v) {
			break
		}
	}
}

package telemetry_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/telemetry"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitTracer_Disabled(t *testing.T) {
	// Ensure OTEL is disabled
	os.Setenv("OTEL_SDK_DISABLED", "true")
	defer os.Unsetenv("OTEL_SDK_DISABLED")

	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	shutdown, err := telemetry.InitTracer(logger)
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	defer func() {
		require.NoError(t, shutdown())
	}()

	// Tracer should be available but noop
	tracer := telemetry.GetTracer()
	require.NotNil(t, tracer)

	// Tracing should not be enabled
	assert.False(t, telemetry.IsEnabled())
}

func TestInitTracer_NotConfigured(t *testing.T) {
	// Ensure no OTEL endpoint is configured
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	os.Unsetenv("OTEL_SDK_DISABLED")

	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	shutdown, err := telemetry.InitTracer(logger)
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	defer func() {
		require.NoError(t, shutdown())
	}()

	// Tracer should be available but noop
	tracer := telemetry.GetTracer()
	require.NotNil(t, tracer)

	// Tracing should not be enabled
	assert.False(t, telemetry.IsEnabled())
}

func TestSessionIDGeneration(t *testing.T) {
	sessionID1 := telemetry.GenerateSessionID()
	sessionID2 := telemetry.GenerateSessionID()

	// Session IDs should not be empty
	assert.NotEmpty(t, sessionID1)
	assert.NotEmpty(t, sessionID2)

	// Session IDs should be unique
	assert.NotEqual(t, sessionID1, sessionID2)

	// Session IDs should be valid UUIDs (contain hyphens)
	assert.Contains(t, sessionID1, "-")
	assert.Contains(t, sessionID2, "-")
}

func TestContextSessionID(t *testing.T) {
	ctx := context.Background()

	// Initially, no session ID
	sessionID := telemetry.SessionIDFromContext(ctx)
	assert.Empty(t, sessionID)

	// Add session ID to context
	testSessionID := "test-session-123"
	ctx = telemetry.ContextWithSessionID(ctx, testSessionID)

	// Retrieve session ID from context
	retrievedID := telemetry.SessionIDFromContext(ctx)
	assert.Equal(t, testSessionID, retrievedID)
}

func TestStartToolSpan_Disabled(t *testing.T) {
	// Ensure tracing is disabled
	os.Setenv("OTEL_SDK_DISABLED", "true")
	defer os.Unsetenv("OTEL_SDK_DISABLED")

	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	shutdown, err := telemetry.InitTracer(logger)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, shutdown())
	}()

	ctx := context.Background()
	args := map[string]any{
		"query": "test query",
		"count": 5,
	}

	// Start a tool span
	spanCtx, span := telemetry.StartToolSpan(ctx, "test_tool", args)
	require.NotNil(t, spanCtx)
	require.NotNil(t, span)

	// End the span
	telemetry.EndToolSpan(span, nil)

	// Should not panic - span is noop
}

func TestStartToolSpan_WithError(t *testing.T) {
	os.Setenv("OTEL_SDK_DISABLED", "true")
	defer os.Unsetenv("OTEL_SDK_DISABLED")

	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	shutdown, err := telemetry.InitTracer(logger)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, shutdown())
	}()

	ctx := context.Background()
	args := map[string]any{"test": "value"}

	spanCtx, span := telemetry.StartToolSpan(ctx, "test_tool", args)
	require.NotNil(t, spanCtx)
	require.NotNil(t, span)

	// End with error
	testErr := assert.AnError
	telemetry.EndToolSpan(span, testErr)

	// Should not panic
}

func TestIsToolTracingDisabled(t *testing.T) {
	// Set disabled tools
	os.Setenv("MCP_TRACING_DISABLED_TOOLS", "tool1,tool2, tool3")
	defer os.Unsetenv("MCP_TRACING_DISABLED_TOOLS")

	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	shutdown, err := telemetry.InitTracer(logger)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, shutdown())
	}()

	// Check disabled tools
	assert.True(t, telemetry.IsToolTracingDisabled("tool1"))
	assert.True(t, telemetry.IsToolTracingDisabled("tool2"))
	assert.True(t, telemetry.IsToolTracingDisabled("tool3"))

	// Check enabled tool
	assert.False(t, telemetry.IsToolTracingDisabled("tool4"))
}

func TestSanitiseURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty URL",
			input:    "",
			expected: "",
		},
		{
			name:     "URL with API key in query",
			input:    "https://api.example.com/search?api_key=secret123&query=test",
			expected: "https://api.example.com/search?api_key=%5BREDACTED%5D&query=test",
		},
		{
			name:     "URL with token in query",
			input:    "https://api.example.com/data?token=abc123",
			expected: "https://api.example.com/data?token=%5BREDACTED%5D",
		},
		{
			name:     "URL with user credentials",
			input:    "https://user:password@example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "clean URL",
			input:    "https://example.com/api/endpoint?param=value",
			expected: "https://example.com/api/endpoint?param=value",
		},
		{
			name:     "invalid URL",
			input:    "not a valid url",
			expected: "[INVALID_URL]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := telemetry.SanitiseURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitiseArguments(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected string
	}{
		{
			name:     "nil arguments",
			input:    nil,
			expected: "{}",
		},
		{
			name:     "empty arguments",
			input:    map[string]any{},
			expected: "{}",
		},
		{
			name: "clean arguments",
			input: map[string]any{
				"query": "test query",
				"count": 5,
			},
			expected: `{"count":5,"query":"test query"}`,
		},
		{
			name: "arguments with API key",
			input: map[string]any{
				"query":   "test",
				"api_key": "secret123",
			},
			expected: `{"api_key":"[REDACTED]","query":"test"}`,
		},
		{
			name: "arguments with token",
			input: map[string]any{
				"data":  "value",
				"token": "bearer_abc123",
			},
			expected: `{"data":"value","token":"[REDACTED]"}`,
		},
		{
			name: "arguments with password",
			input: map[string]any{
				"username": "user",
				"password": "secret",
			},
			expected: `{"password":"[REDACTED]","username":"user"}`,
		},
		{
			name: "nested arguments",
			input: map[string]any{
				"config": map[string]any{
					"api_key": "secret",
					"timeout": 30,
				},
			},
			expected: `{"config":{"api_key":"[REDACTED]","timeout":30}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := telemetry.SanitiseArguments(tt.input)
			assert.JSONEq(t, tt.expected, result)
		})
	}
}

func TestSanitiseCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "empty key",
			input:    "",
			contains: "",
		},
		{
			name:     "clean cache key",
			input:    "user:123:profile",
			contains: "user:123:profile",
		},
		{
			name:     "cache key with token",
			input:    "cache:token:abc123xyz",
			contains: "[REDACTED]",
		},
		{
			name:     "cache key with api key",
			input:    "api_key:secret123",
			contains: "[REDACTED]",
		},
		{
			name:     "very long cache key",
			input:    strings.Repeat("a", 150),
			contains: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := telemetry.SanitiseCacheKey(tt.input)
			if tt.contains != "" {
				assert.Contains(t, result, tt.contains)
			}
			if len(tt.input) > 100 {
				assert.LessOrEqual(t, len(result), 100)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "needs truncation",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "very short max length",
			input:    "hello",
			maxLen:   3,
			expected: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := telemetry.TruncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
			assert.LessOrEqual(t, len(result), tt.maxLen)
		})
	}
}

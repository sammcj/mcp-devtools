package telemetry_test

import (
	"context"
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/telemetry"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestStartSessionSpan_Disabled(t *testing.T) {
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
	sessionID := telemetry.GenerateSessionID()

	// Start session span
	resultCtx, sessionSpan := telemetry.StartSessionSpan(ctx, sessionID, "stdio")
	require.NotNil(t, resultCtx)
	require.NotNil(t, sessionSpan)

	// Span should be noop when disabled
	spanContext := sessionSpan.SpanContext()
	assert.False(t, spanContext.IsValid())

	// End session span
	telemetry.EndSessionSpan(sessionSpan, 0, 0, 0)

	// Should not panic
}

func TestStartSessionSpan_ReturnsNoopSpan(t *testing.T) {
	// Even when tracing would be enabled, we can't easily test with a real backend
	// But we can test that the function doesn't panic and returns non-nil values
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
	sessionID := "test-session-123"

	// Start session span
	resultCtx, sessionSpan := telemetry.StartSessionSpan(ctx, sessionID, "http")
	require.NotNil(t, resultCtx)
	require.NotNil(t, sessionSpan)

	// The returned span is a noop span (real span was already ended)
	// Even when tracing is disabled, we should get a valid noop span
	assert.NotNil(t, sessionSpan)
}

func TestEndSessionSpan_ClearsGlobalState(t *testing.T) {
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
	sessionID := telemetry.GenerateSessionID()

	// Start session span
	_, sessionSpan := telemetry.StartSessionSpan(ctx, sessionID, "stdio")
	require.NotNil(t, sessionSpan)

	// End session span (should clear global state)
	telemetry.EndSessionSpan(sessionSpan, 5, 1, 12345)

	// Should not panic - even if called multiple times
	telemetry.EndSessionSpan(sessionSpan, 0, 0, 0)
}

func TestToolSpan_WithSessionSpan_Disabled(t *testing.T) {
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
	sessionID := telemetry.GenerateSessionID()

	// Start session span
	_, sessionSpan := telemetry.StartSessionSpan(ctx, sessionID, "stdio")
	require.NotNil(t, sessionSpan)

	// Start tool span (should use session span context internally)
	args := map[string]any{"query": "test"}
	toolCtx, toolSpan := telemetry.StartToolSpan(ctx, "test_tool", args)
	require.NotNil(t, toolCtx)
	require.NotNil(t, toolSpan)

	// End tool span
	telemetry.EndToolSpan(toolSpan, nil)

	// End session span
	telemetry.EndSessionSpan(sessionSpan, 1, 0, 100)

	// Should not panic
}

func TestSessionSpan_MultipleTools(t *testing.T) {
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
	sessionID := telemetry.GenerateSessionID()

	// Start session span
	_, sessionSpan := telemetry.StartSessionSpan(ctx, sessionID, "stdio")
	require.NotNil(t, sessionSpan)

	// Start multiple tool spans
	for i := range 3 {
		args := map[string]any{"index": i}
		toolCtx, toolSpan := telemetry.StartToolSpan(ctx, "test_tool", args)
		require.NotNil(t, toolCtx)
		require.NotNil(t, toolSpan)

		// End tool span
		telemetry.EndToolSpan(toolSpan, nil)
	}

	// End session span
	telemetry.EndSessionSpan(sessionSpan, 3, 0, 500)

	// Should not panic
}

func TestSessionSpan_WithDifferentTransports(t *testing.T) {
	transports := []string{"stdio", "http", "sse"}

	for _, transport := range transports {
		t.Run(transport, func(t *testing.T) {
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
			sessionID := telemetry.GenerateSessionID()

			// Start session span with specific transport
			_, sessionSpan := telemetry.StartSessionSpan(ctx, sessionID, transport)
			require.NotNil(t, sessionSpan)

			// End session span
			telemetry.EndSessionSpan(sessionSpan, 0, 0, 0)

			// Should not panic
		})
	}
}

func TestSessionSpan_EmptySessionID(t *testing.T) {
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

	// Start session span with empty session ID (should still work)
	_, sessionSpan := telemetry.StartSessionSpan(ctx, "", "stdio")
	require.NotNil(t, sessionSpan)

	// End session span
	telemetry.EndSessionSpan(sessionSpan, 0, 0, 0)

	// Should not panic
}

func TestSessionSpan_NilSpanEnd(t *testing.T) {
	os.Setenv("OTEL_SDK_DISABLED", "true")
	defer os.Unsetenv("OTEL_SDK_DISABLED")

	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	shutdown, err := telemetry.InitTracer(logger)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, shutdown())
	}()

	// End session span with nil span (should not panic)
	var nilSpan trace.Span
	telemetry.EndSessionSpan(nilSpan, 0, 0, 0)

	// Should not panic
}

func TestSessionSpan_ContextIsolation(t *testing.T) {
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

	// Start first session
	sessionID1 := telemetry.GenerateSessionID()
	_, sessionSpan1 := telemetry.StartSessionSpan(ctx, sessionID1, "stdio")

	// Start tool span in first session
	args := map[string]any{"session": 1}
	_, toolSpan1 := telemetry.StartToolSpan(ctx, "tool1", args)

	// End first session
	telemetry.EndToolSpan(toolSpan1, nil)
	telemetry.EndSessionSpan(sessionSpan1, 1, 0, 100)

	// Start second session (should have clean state)
	sessionID2 := telemetry.GenerateSessionID()
	_, sessionSpan2 := telemetry.StartSessionSpan(ctx, sessionID2, "http")

	// Start tool span in second session
	args2 := map[string]any{"session": 2}
	_, toolSpan2 := telemetry.StartToolSpan(ctx, "tool2", args2)

	// End second session
	telemetry.EndToolSpan(toolSpan2, nil)
	telemetry.EndSessionSpan(sessionSpan2, 1, 0, 200)

	// Should not panic - sessions should be isolated
}

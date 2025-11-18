package telemetry

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sammcj/mcp-devtools/internal/telemetry"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsInitialisation(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name           string
		otlpEndpoint   string
		metricsGroups  string
		shouldEnable   bool
		expectedGroups map[string]bool
	}{
		{
			name:          "Metrics disabled - no endpoint",
			otlpEndpoint:  "",
			metricsGroups: "",
			shouldEnable:  false,
			expectedGroups: map[string]bool{
				"tool":    true,
				"session": true,
			},
		},
		{
			name:          "Metrics enabled - default groups",
			otlpEndpoint:  "http://localhost:4318",
			metricsGroups: "",
			shouldEnable:  true,
			expectedGroups: map[string]bool{
				"tool":    true,
				"session": true,
			},
		},
		{
			name:          "Metrics enabled - custom groups",
			otlpEndpoint:  "http://localhost:4318",
			metricsGroups: "tool,cache,security",
			shouldEnable:  true,
			expectedGroups: map[string]bool{
				"tool":     true,
				"cache":    true,
				"security": true,
			},
		},
		{
			name:          "Metrics enabled - only tool group",
			otlpEndpoint:  "http://localhost:4318",
			metricsGroups: "tool",
			shouldEnable:  true,
			expectedGroups: map[string]bool{
				"tool": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			if tt.otlpEndpoint != "" {
				os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", tt.otlpEndpoint)
			} else {
				os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
			}

			if tt.metricsGroups != "" {
				os.Setenv("MCP_METRICS_GROUPS", tt.metricsGroups)
			} else {
				os.Unsetenv("MCP_METRICS_GROUPS")
			}

			// Test initialisation with timeout to catch deadlocks
			done := make(chan struct{})
			var shutdown func() error
			var err error

			go func() {
				shutdown, err = telemetry.InitMetrics(logger)
				close(done)
			}()

			select {
			case <-done:
				// Initialisation completed
				if tt.shouldEnable && tt.otlpEndpoint != "" {
					// If OTLP endpoint is configured, we expect an error since we're not running a collector
					// But it should NOT deadlock
					assert.NotNil(t, shutdown, "Shutdown function should be returned even on error")
				} else {
					// No endpoint configured, should succeed with noop
					require.NoError(t, err, "InitMetrics should succeed when disabled")
					require.NotNil(t, shutdown, "Shutdown function should be returned")
				}

				// Verify metrics enabled state
				assert.Equal(t, tt.shouldEnable && err == nil, telemetry.IsMetricsEnabled(), "Metrics enabled state mismatch")

				// Clean up
				if shutdown != nil {
					_ = shutdown()
				}

			case <-time.After(2 * time.Second):
				t.Fatal("InitMetrics deadlocked or took too long (>2s)")
			}

			// Clean up environment
			os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
			os.Unsetenv("MCP_METRICS_GROUPS")
		})
	}
}

func TestMetricsRecordingDisabled(t *testing.T) {
	// Ensure metrics are disabled
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	shutdown, err := telemetry.InitMetrics(logger)
	require.NoError(t, err)
	defer func() {
		_ = shutdown()
	}()

	ctx := context.Background()

	// These should not panic when metrics are disabled
	telemetry.RecordToolCall(ctx, "test_tool", "stdio", true, 100.0)
	telemetry.RecordToolError(ctx, "test_tool", "network")
	telemetry.RecordSessionStart(ctx, "stdio")
	telemetry.RecordSessionEnd(ctx, "stdio", 60.0, 5)
	telemetry.RecordCacheOperation(ctx, "test_tool", "get", true)
	telemetry.RecordSecurityCheck(ctx, "allow", "url", 10.0)
	telemetry.RecordSecurityRuleTrigger(ctx, "test_rule", "warn")
}

func TestErrorCategorisation(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "Network error - connection refused",
			err:      assert.AnError, // Using assert.AnError as placeholder
			expected: "internal",     // Will be categorised as internal since it doesn't match patterns
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := telemetry.CategoriseToolError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMetricsShutdown(t *testing.T) {
	// Test shutdown when metrics are disabled
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	shutdown, err := telemetry.InitMetrics(logger)
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	defer func() {
		_ = shutdown()
	}()

	// Shutdown should not error
	err = shutdown()
	assert.NoError(t, err)

	// Multiple shutdowns should be safe
	err = shutdown()
	assert.NoError(t, err)
}

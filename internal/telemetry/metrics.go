package telemetry

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

const (
	// Metric export interval in seconds (configurable via environment variable)
	defaultMetricExportInterval = 60 * time.Second
)

var (
	// Metrics state
	metricsMutex        sync.RWMutex
	globalMeterProvider *sdkmetric.MeterProvider
	globalMeter         metric.Meter
	metricsEnabled      bool

	// Enabled metric groups (opt-in)
	enabledMetricGroups map[string]bool

	// Phase 1: Core Tool Metrics
	toolCallsCounter      metric.Int64Counter
	toolDurationHistogram metric.Float64Histogram
	toolErrorsCounter     metric.Int64Counter

	// Phase 2: Session & Resource Metrics
	activeSessionsGauge  metric.Int64UpDownCounter
	sessionDurationHist  metric.Float64Histogram
	sessionToolCountHist metric.Int64Histogram

	// Phase 3: Cache Metrics
	cacheOpsCounter metric.Int64Counter
	// cacheHitRatioGauge metric.Float64Gauge // Reserved for future implementation

	// Phase 3: Security Metrics (opt-in)
	securityChecksCounter metric.Int64Counter
	securityRuleTriggers  metric.Int64Counter
	securityCheckDuration metric.Float64Histogram
)

// InitMetrics initialises the OpenTelemetry meter provider
// Should be called after InitTracer in main.go
// Returns a shutdown function and an error if initialisation fails
func InitMetrics(logger *logrus.Logger) (func() error, error) {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()

	// Parse enabled metric groups (always, regardless of whether metrics are enabled)
	enabledMetricGroups = parseEnabledMetricGroups()
	if len(enabledMetricGroups) > 0 {
		logger.WithField("enabled_groups", enabledMetricGroups).Debug("OTEL Metrics: Enabled groups configured")
	} else {
		// Default: enable tool and session metrics if MCP_METRICS_GROUPS not set
		enabledMetricGroups = map[string]bool{
			"tool":    true,
			"session": true,
		}
		logger.Debug("OTEL Metrics: Using default groups (tool, session)")
	}

	// Check if metrics are enabled (same endpoint as tracing)
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		logger.Debug("OTEL Metrics: Not configured, using noop meter")
		metricsEnabled = false
		globalMeter = otel.GetMeterProvider().Meter("mcp-devtools")
		return func() error { return nil }, nil
	}

	metricsEnabled = true
	logger.WithField("endpoint", endpoint).Info("OTEL Metrics: Initialising meter")

	// Determine protocol (http or grpc) - reuse logic from tracing
	protocol := getOTLPProtocol()
	logger.WithField("protocol", protocol).Debug("OTEL Metrics: Using protocol")

	// Create OTLP metric exporter
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var exporter sdkmetric.Exporter
	var err error

	switch protocol {
	case "grpc":
		exporter, err = otlpmetricgrpc.New(ctx)
	case "http/protobuf", "http":
		exporter, err = otlpmetrichttp.New(ctx)
	default:
		logger.WithField("protocol", protocol).Warn("OTEL Metrics: Unknown protocol, defaulting to http")
		exporter, err = otlpmetrichttp.New(ctx)
	}

	if err != nil {
		logger.WithError(err).Warn("OTEL Metrics: Failed to create exporter, falling back to noop meter")
		metricsEnabled = false
		globalMeter = otel.GetMeterProvider().Meter("mcp-devtools")
		return func() error { return nil }, err
	}

	// Create resource with service information (reuse from tracing)
	serviceName := getServiceName()
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(getServiceVersion()),
			attribute.String("deployment.environment", getDeploymentEnvironment()),
		),
		resource.WithFromEnv(), // Allow additional attributes from OTEL_RESOURCE_ATTRIBUTES
	)
	if err != nil {
		logger.WithError(err).Warn("OTEL Metrics: Failed to create resource, using default")
		res = resource.Default()
	}

	// Get metric export interval
	exportInterval := getMetricExportInterval(logger)

	// Create meter provider with periodic reader
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(exportInterval),
		)),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(meterProvider)
	globalMeterProvider = meterProvider
	globalMeter = meterProvider.Meter("mcp-devtools")

	logger.Info("OTEL Metrics: Meter initialised successfully")

	// Unlock mutex before calling initMetricInstruments to avoid deadlock
	metricsMutex.Unlock()

	// Initialise metric instruments
	err = initMetricInstruments(logger)

	// Re-lock mutex for deferred unlock
	metricsMutex.Lock()

	if err != nil {
		logger.WithError(err).Error("OTEL Metrics: Failed to initialise instruments")
		return func() error { return nil }, err
	}

	// Return shutdown function
	return func() error {
		metricsMutex.Lock()
		defer metricsMutex.Unlock()

		if globalMeterProvider != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := globalMeterProvider.Shutdown(shutdownCtx); err != nil {
				logger.WithError(err).Error("OTEL Metrics: Failed to shutdown meter provider")
				return err
			}
			logger.Debug("OTEL Metrics: Meter provider shutdown successfully")
		}
		return nil
	}, nil
}

// initMetricInstruments creates all metric instruments
// Called once when metrics are enabled
// IMPORTANT: Caller must NOT hold metricsMutex as this function accesses enabledMetricGroups
func initMetricInstruments(logger *logrus.Logger) error {
	var err error

	if !metricsEnabled {
		return nil
	}

	meter := globalMeter

	// Phase 1: Core Tool Metrics
	// Check enabledMetricGroups directly without mutex (safe - already set in InitMetrics)
	if enabledMetricGroups["tool"] {
		toolCallsCounter, err = meter.Int64Counter(
			"mcp.tool.calls",
			metric.WithDescription("Total tool invocations"),
			metric.WithUnit("{call}"),
		)
		if err != nil {
			logger.WithError(err).Error("OTEL Metrics: Failed to create mcp.tool.calls counter")
			return err
		}

		toolDurationHistogram, err = meter.Float64Histogram(
			"mcp.tool.duration",
			metric.WithDescription("Tool execution duration"),
			metric.WithUnit("ms"),
			metric.WithExplicitBucketBoundaries(10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000),
		)
		if err != nil {
			logger.WithError(err).Error("OTEL Metrics: Failed to create mcp.tool.duration histogram")
			return err
		}

		toolErrorsCounter, err = meter.Int64Counter(
			"mcp.tool.errors",
			metric.WithDescription("Tool execution errors by type"),
			metric.WithUnit("{error}"),
		)
		if err != nil {
			logger.WithError(err).Error("OTEL Metrics: Failed to create mcp.tool.errors counter")
			return err
		}

		logger.Debug("OTEL Metrics: Tool metrics initialised")
	}

	// Phase 2: Session Metrics
	if enabledMetricGroups["session"] {
		activeSessionsGauge, err = meter.Int64UpDownCounter(
			"mcp.session.active",
			metric.WithDescription("Active concurrent sessions"),
			metric.WithUnit("{session}"),
		)
		if err != nil {
			logger.WithError(err).Error("OTEL Metrics: Failed to create mcp.session.active gauge")
			return err
		}

		sessionDurationHist, err = meter.Float64Histogram(
			"mcp.session.duration",
			metric.WithDescription("Session duration"),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(10, 30, 60, 300, 600, 1800, 3600, 7200),
		)
		if err != nil {
			logger.WithError(err).Error("OTEL Metrics: Failed to create mcp.session.duration histogram")
			return err
		}

		sessionToolCountHist, err = meter.Int64Histogram(
			"mcp.session.tool_count",
			metric.WithDescription("Number of tools executed per session"),
			metric.WithUnit("{tool}"),
			metric.WithExplicitBucketBoundaries(1, 2, 5, 10, 20, 50, 100, 200),
		)
		if err != nil {
			logger.WithError(err).Error("OTEL Metrics: Failed to create mcp.session.tool_count histogram")
			return err
		}

		logger.Debug("OTEL Metrics: Session metrics initialised")
	}

	// Phase 3: Cache Metrics
	if enabledMetricGroups["cache"] {
		cacheOpsCounter, err = meter.Int64Counter(
			"cache.operations",
			metric.WithDescription("Cache operations"),
			metric.WithUnit("{operation}"),
		)
		if err != nil {
			logger.WithError(err).Error("OTEL Metrics: Failed to create cache.operations counter")
			return err
		}

		// Note: cache.hit_ratio gauge would be implemented here
		// For now, hit ratio can be calculated from cache.operations counter in queries

		logger.Debug("OTEL Metrics: Cache metrics initialised")
	}

	// Phase 3: Security Metrics (enabled by default if security group is enabled)
	if enabledMetricGroups["security"] {
		securityChecksCounter, err = meter.Int64Counter(
			"security.checks",
			metric.WithDescription("Security framework checks"),
			metric.WithUnit("{check}"),
		)
		if err != nil {
			logger.WithError(err).Error("OTEL Metrics: Failed to create security.checks counter")
			return err
		}

		securityRuleTriggers, err = meter.Int64Counter(
			"security.rule.triggers",
			metric.WithDescription("Security rule triggers"),
			metric.WithUnit("{trigger}"),
		)
		if err != nil {
			logger.WithError(err).Error("OTEL Metrics: Failed to create security.rule.triggers counter")
			return err
		}

		securityCheckDuration, err = meter.Float64Histogram(
			"security.check.duration",
			metric.WithDescription("Security check duration"),
			metric.WithUnit("ms"),
			metric.WithExplicitBucketBoundaries(1, 5, 10, 25, 50, 100, 250),
		)
		if err != nil {
			logger.WithError(err).Error("OTEL Metrics: Failed to create security.check.duration histogram")
			return err
		}

		logger.Debug("OTEL Metrics: Security metrics initialised")
	}

	return nil
}

// IsMetricsEnabled returns true if metrics collection is enabled
func IsMetricsEnabled() bool {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	return metricsEnabled
}

// GetMeter returns the global meter instance
func GetMeter() metric.Meter {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()

	if globalMeter == nil {
		return otel.GetMeterProvider().Meter("mcp-devtools")
	}
	return globalMeter
}

// RecordToolCall records a tool invocation metric
func RecordToolCall(ctx context.Context, toolName string, transport string, success bool, durationMs float64) {
	if !IsMetricsEnabled() || !isMetricGroupEnabled("tool") {
		return
	}

	result := "success"
	if !success {
		result = "error"
	}

	if toolCallsCounter != nil {
		toolCallsCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("tool.name", toolName),
				attribute.String("transport", transport),
				attribute.String("result", result),
			),
		)
	}

	if toolDurationHistogram != nil {
		toolDurationHistogram.Record(ctx, durationMs,
			metric.WithAttributes(
				attribute.String("tool.name", toolName),
				attribute.String("transport", transport),
			),
		)
	}
}

// RecordToolError records a categorised tool error
func RecordToolError(ctx context.Context, toolName string, errorType string) {
	if !IsMetricsEnabled() || !isMetricGroupEnabled("tool") {
		return
	}

	if toolErrorsCounter != nil {
		toolErrorsCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("tool.name", toolName),
				attribute.String("error.type", errorType),
			),
		)
	}
}

// CategoriseToolError maps errors to metric-friendly categories
func CategoriseToolError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	// Network errors
	if strings.Contains(errStr, "dial tcp") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") {
		return "network"
	}

	// Timeout errors
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") {
		return "timeout"
	}

	// Validation errors
	if strings.Contains(errStr, "invalid") ||
		strings.Contains(errStr, "validation") ||
		strings.Contains(errStr, "required parameter") {
		return "validation"
	}

	// Security errors
	if strings.Contains(errStr, "access denied") ||
		strings.Contains(errStr, "blocked by security") {
		return "security"
	}

	// External API errors (HTTP status codes)
	if strings.Contains(errStr, "status code: 4") ||
		strings.Contains(errStr, "status code: 5") {
		return "external_api"
	}

	// Default: internal error
	return "internal"
}

// RecordSessionStart increments active sessions counter
func RecordSessionStart(ctx context.Context, transport string) {
	if !IsMetricsEnabled() || !isMetricGroupEnabled("session") {
		return
	}

	if activeSessionsGauge != nil {
		activeSessionsGauge.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("transport", transport),
			),
		)
	}
}

// RecordSessionEnd decrements active sessions and records duration and tool count
func RecordSessionEnd(ctx context.Context, transport string, durationSeconds float64, toolCount int64) {
	if !IsMetricsEnabled() || !isMetricGroupEnabled("session") {
		return
	}

	if activeSessionsGauge != nil {
		activeSessionsGauge.Add(ctx, -1,
			metric.WithAttributes(
				attribute.String("transport", transport),
			),
		)
	}

	if sessionDurationHist != nil {
		sessionDurationHist.Record(ctx, durationSeconds,
			metric.WithAttributes(
				attribute.String("transport", transport),
			),
		)
	}

	if sessionToolCountHist != nil {
		sessionToolCountHist.Record(ctx, toolCount,
			metric.WithAttributes(
				attribute.String("transport", transport),
			),
		)
	}
}

// RecordCacheOperation records a cache operation metric
func RecordCacheOperation(ctx context.Context, toolName string, operation string, hit bool) {
	if !IsMetricsEnabled() || !isMetricGroupEnabled("cache") {
		return
	}

	result := "miss"
	if hit {
		result = "hit"
	}

	if cacheOpsCounter != nil {
		cacheOpsCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("tool.name", toolName),
				attribute.String("operation", operation),
				attribute.String("result", result),
			),
		)
	}
}

// RecordSecurityCheck records a security framework check metric
func RecordSecurityCheck(ctx context.Context, action string, sourceType string, durationMs float64) {
	if !IsMetricsEnabled() || !isMetricGroupEnabled("security") {
		return
	}

	if securityChecksCounter != nil {
		securityChecksCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("action", action),
				attribute.String("source.type", sourceType),
			),
		)
	}

	if securityCheckDuration != nil {
		securityCheckDuration.Record(ctx, durationMs,
			metric.WithAttributes(
				attribute.String("action", action),
			),
		)
	}
}

// RecordSecurityRuleTrigger records a security rule trigger
func RecordSecurityRuleTrigger(ctx context.Context, ruleName string, action string) {
	if !IsMetricsEnabled() || !isMetricGroupEnabled("security") {
		return
	}

	if securityRuleTriggers != nil {
		securityRuleTriggers.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("rule.name", ruleName),
				attribute.String("action", action),
			),
		)
	}
}

// Helper functions

func parseEnabledMetricGroups() map[string]bool {
	enabled := make(map[string]bool)
	enabledStr := os.Getenv("MCP_METRICS_GROUPS")
	if enabledStr == "" {
		return enabled
	}

	groups := strings.SplitSeq(enabledStr, ",")
	for group := range groups {
		group = strings.TrimSpace(group)
		if group != "" {
			enabled[group] = true
		}
	}

	return enabled
}

func isMetricGroupEnabled(group string) bool {
	metricsMutex.RLock()
	defer metricsMutex.RUnlock()
	return enabledMetricGroups[group]
}

func getMetricExportInterval(logger *logrus.Logger) time.Duration {
	intervalStr := os.Getenv("OTEL_METRIC_EXPORT_INTERVAL")
	if intervalStr == "" {
		return defaultMetricExportInterval
	}

	// Try parsing as duration string (e.g., "60s", "1m", "60")
	// If it's just a number, assume seconds
	duration, err := time.ParseDuration(intervalStr)
	if err != nil {
		// Try adding "s" suffix for bare numbers
		duration, err = time.ParseDuration(intervalStr + "s")
		if err != nil {
			logger.WithField("interval", intervalStr).Warn("OTEL Metrics: Invalid export interval, using default")
			return defaultMetricExportInterval
		}
	}

	return duration
}

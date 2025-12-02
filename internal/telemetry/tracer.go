package telemetry

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	sessionIDKey contextKey = "mcp.session.id"

	// Span attribute size limits
	defaultMaxAttributeSize = 4096  // 4KB default for span attributes
	minAttributeSize        = 1024  // 1KB minimum
	maxAttributeSize        = 65536 // 64KB maximum
)

var (
	// globalMutex protects access to global tracer variables
	globalMutex sync.RWMutex
	// global tracer instance
	globalTracer trace.Tracer
	// global tracer provider for shutdown
	globalTracerProvider *sdktrace.TracerProvider
	// disabled tools (won't create spans)
	disabledTools map[string]bool
	// is tracing enabled
	tracingEnabled bool
	// global session span context for stdio transport
	// Tool spans use this as their parent context
	globalSessionSpanContext trace.SpanContext
	// global session ID for attributes
	globalSessionID string
)

// otelErrorHandler adapts OTEL SDK errors to our logging system
// This prevents OTEL from logging to stderr, which would break stdio protocol
type otelErrorHandler struct {
	logger *logrus.Logger
}

func (h *otelErrorHandler) Handle(err error) {
	if err == nil {
		return
	}
	// Use Debug level - in stdio mode this goes to file, not stderr
	// This is critical for stdio mode where stderr output breaks the MCP protocol
	h.logger.WithError(err).Debug("OTEL: SDK error occurred")
}

// InitTracer initialises the OpenTelemetry tracer based on environment variables
// Returns a shutdown function and an error if initialisation fails.
// The application can continue with a noop tracer even if initialisation fails.
func InitTracer(logger *logrus.Logger) (func() error, error) {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	// Clear any previous session data (in case of restart)
	globalSessionSpanContext = trace.SpanContext{}
	globalSessionID = ""

	// Parse disabled tools (always, regardless of whether tracing is enabled)
	disabledTools = parseDisabledTools()
	if len(disabledTools) > 0 {
		logger.WithField("disabled_tools", disabledTools).Debug("OTEL: Disabled tools configured")
	}

	// Check if OTEL is explicitly disabled
	if isDisabled := os.Getenv("OTEL_SDK_DISABLED"); strings.ToLower(isDisabled) == "true" {
		logger.Debug("OTEL: Explicitly disabled via OTEL_SDK_DISABLED")
		globalTracer = noop.NewTracerProvider().Tracer("mcp-devtools")
		tracingEnabled = false
		return func() error { return nil }, nil
	}

	// Check if OTEL endpoint is configured (required for enabling tracing)
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		logger.Debug("OTEL: Not configured (OTEL_EXPORTER_OTLP_ENDPOINT not set), using noop tracer")
		globalTracer = noop.NewTracerProvider().Tracer("mcp-devtools")
		tracingEnabled = false
		return func() error { return nil }, nil
	}

	tracingEnabled = true
	logger.WithField("endpoint", endpoint).Info("OTEL: Initialising tracer")

	// Configure OTEL SDK to use our logger instead of stderr
	// CRITICAL: Prevents OTEL from logging to stderr in stdio mode, which would break MCP protocol
	otel.SetErrorHandler(&otelErrorHandler{logger: logger})

	// Determine protocol (http or grpc)
	protocol := getOTLPProtocol()
	logger.WithField("protocol", protocol).Debug("OTEL: Using protocol")

	// Create OTLP exporter
	var exporter *otlptrace.Exporter
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch protocol {
	case "grpc":
		exporter, err = otlptracegrpc.New(ctx)
	case "http/protobuf", "http":
		exporter, err = otlptracehttp.New(ctx)
	default:
		logger.WithField("protocol", protocol).Warn("OTEL: Unknown protocol, defaulting to http")
		exporter, err = otlptracehttp.New(ctx)
	}

	if err != nil {
		logger.WithError(err).Warn("OTEL: Failed to create exporter, falling back to noop tracer")
		globalTracer = noop.NewTracerProvider().Tracer("mcp-devtools")
		tracingEnabled = false
		return func() error { return nil }, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
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
		logger.WithError(err).Warn("OTEL: Failed to create resource, using default")
		res = resource.Default()
	}

	// Create sampler
	sampler := createSampler(logger)

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter), // Use batch processor for better performance
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator for W3C Trace Context
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create tracer
	globalTracer = tp.Tracer("mcp-devtools")
	globalTracerProvider = tp

	logger.Info("OTEL: Tracer initialised successfully")

	// Return shutdown function
	return func() error {
		globalMutex.Lock()
		defer globalMutex.Unlock()

		if globalTracerProvider != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := globalTracerProvider.Shutdown(shutdownCtx); err != nil {
				logger.WithError(err).Error("OTEL: Failed to shutdown tracer provider")
				return fmt.Errorf("failed to shutdown tracer provider: %w", err)
			}
			logger.Debug("OTEL: Tracer provider shutdown successfully")
		}
		return nil
	}, nil
}

// GetTracer returns the global tracer instance
// Returns a noop tracer if not initialised
func GetTracer() trace.Tracer {
	globalMutex.RLock()
	defer globalMutex.RUnlock()

	if globalTracer == nil {
		// Return noop tracer if not initialised
		return noop.NewTracerProvider().Tracer("mcp-devtools")
	}
	return globalTracer
}

// IsEnabled returns true if tracing is enabled
func IsEnabled() bool {
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	return tracingEnabled
}

// IsToolTracingDisabled returns true if tracing is disabled for the specified tool
// via the MCP_TRACING_DISABLED_TOOLS environment variable
func IsToolTracingDisabled(toolName string) bool {
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	return disabledTools[toolName]
}

// GetTextMapPropagator returns the global text map propagator for extracting/injecting trace context
func GetTextMapPropagator() propagation.TextMapPropagator {
	return otel.GetTextMapPropagator()
}

// GenerateSessionID generates a new unique session ID
func GenerateSessionID() string {
	return uuid.New().String()
}

// ContextWithSessionID adds a session ID to the context
func ContextWithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// SessionIDFromContext retrieves the session ID from the context
func SessionIDFromContext(ctx context.Context) string {
	if sessionID, ok := ctx.Value(sessionIDKey).(string); ok {
		return sessionID
	}
	return ""
}

// StartSessionSpan creates a session span that acts as parent for all tool spans
// The span is ended immediately to ensure it exports before child tool spans
// Returns a noop span since the real span is already ended
func StartSessionSpan(ctx context.Context, sessionID string, transport string) (context.Context, trace.Span) {
	if !tracingEnabled {
		// Return noop span
		return ctx, trace.SpanFromContext(ctx)
	}

	tracer := GetTracer()

	// Create a session span
	ctx, sessionSpan := tracer.Start(ctx, SpanNameSession,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(AttrMCPSessionID, sessionID),
			attribute.String(AttrMCPTransport, transport),
		),
	)

	// Store the span context BEFORE ending
	sessionSpanContext := sessionSpan.SpanContext()

	// End the span immediately so it exports to backend before child tool spans
	// This ensures the parent is available when children arrive
	sessionSpan.End()

	// Force flush to ensure session span is exported immediately
	// This is critical to prevent "invalid parent span IDs" warnings
	globalMutex.RLock()
	tp := globalTracerProvider
	globalMutex.RUnlock()

	if tp != nil {
		// Use a short timeout for force flush to avoid blocking
		flushCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tp.ForceFlush(flushCtx) // Ignore errors - tracing is best-effort
	}

	// Store globally so tool spans can reference it as parent
	globalMutex.Lock()
	globalSessionSpanContext = sessionSpanContext
	globalSessionID = sessionID
	globalMutex.Unlock()

	// Return noop span since real span is already ended
	return ctx, trace.SpanFromContext(ctx)
}

// EndSessionSpan clears global session data
// The session span was already ended in StartSessionSpan
func EndSessionSpan(span trace.Span, toolCount int, errorCount int, duration int64) {
	// Clear global session data
	globalMutex.Lock()
	globalSessionSpanContext = trace.SpanContext{}
	globalSessionID = ""
	globalMutex.Unlock()
}

// StartToolSpan creates a new span for tool execution
// If a session span context exists, tool spans will be children of the session span
// Returns the span and a modified context. The caller MUST call span.End() when done.
func StartToolSpan(ctx context.Context, toolName string, args map[string]any) (context.Context, trace.Span) {
	if !tracingEnabled || IsToolTracingDisabled(toolName) {
		// Return noop span
		return ctx, trace.SpanFromContext(ctx)
	}

	// Get global session span context if available
	globalMutex.RLock()
	sessionSpanCtx := globalSessionSpanContext
	sessionID := globalSessionID
	globalMutex.RUnlock()

	tracer := GetTracer()

	// Configure span options
	spanOpts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindInternal),
	}

	// If we have a session span context, establish parent-child relationship
	// We need to propagate the trace context properly
	if sessionSpanCtx.IsValid() {
		// Extract trace context using text map propagator
		// This creates a proper parent relationship by injecting the trace context
		carrier := propagation.MapCarrier{}
		prop := otel.GetTextMapPropagator()

		// Inject session span context into carrier
		sessionCtx := trace.ContextWithSpanContext(context.Background(), sessionSpanCtx)
		prop.Inject(sessionCtx, carrier)

		// Extract into our tool context - this properly sets up parent-child relationship
		ctx = prop.Extract(ctx, carrier)
	}

	// Start tool span - will be child of session span if context has session trace
	ctx, span := tracer.Start(ctx, SpanNameToolExecute, spanOpts...)

	// Add standard attributes
	span.SetAttributes(
		attribute.String(AttrMCPToolName, toolName),
	)

	// Add session ID if available
	if sessionID != "" {
		span.SetAttributes(attribute.String(AttrMCPSessionID, sessionID))
	}

	// Add sanitised arguments (only if not too large)
	sanitisedArgs := SanitiseArguments(args, toolName)
	maxAttrSize := getMaxAttributeSize()
	if len(sanitisedArgs) <= maxAttrSize {
		span.SetAttributes(attribute.String("mcp.tool.arguments", sanitisedArgs))
	} else {
		// Arguments too large, truncate
		truncated := TruncateString(sanitisedArgs, maxAttrSize)
		span.SetAttributes(
			attribute.String("mcp.tool.arguments", truncated),
			attribute.Bool("mcp.tool.arguments.truncated", true),
		)
	}

	return ctx, span
}

// EndToolSpan ends a tool execution span with success or error
func EndToolSpan(span trace.Span, err error) {
	if span == nil {
		return
	}

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(
			attribute.Bool(AttrMCPToolSuccess, false),
			attribute.String(AttrMCPToolError, err.Error()),
		)
	} else {
		span.SetStatus(codes.Ok, "")
		span.SetAttributes(attribute.Bool(AttrMCPToolSuccess, true))
	}

	span.End()
}

// Helper functions

func parseDisabledTools() map[string]bool {
	disabled := make(map[string]bool)
	disabledStr := os.Getenv("MCP_TRACING_DISABLED_TOOLS")
	if disabledStr == "" {
		return disabled
	}

	tools := strings.SplitSeq(disabledStr, ",")
	for tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool != "" {
			disabled[tool] = true
		}
	}

	return disabled
}

func getOTLPProtocol() string {
	protocol := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
	if protocol == "" {
		// Check endpoint to guess protocol
		endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if strings.Contains(endpoint, ":4317") {
			return "grpc" // Default gRPC port
		}
		return "http/protobuf" // Default to HTTP
	}
	return protocol
}

func getServiceName() string {
	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		return name
	}
	return "mcp-devtools"
}

func getServiceVersion() string {
	// This will be set by the main package during build
	// For now, return "dev" as a placeholder
	if version := os.Getenv("MCP_VERSION"); version != "" {
		return version
	}
	return "dev"
}

func getDeploymentEnvironment() string {
	// Check for common environment variable names
	for _, envVar := range []string{"ENVIRONMENT", "ENV", "DEPLOYMENT_ENV"} {
		if env := os.Getenv(envVar); env != "" {
			return env
		}
	}

	// Parse from OTEL_RESOURCE_ATTRIBUTES if set
	if attrs := os.Getenv("OTEL_RESOURCE_ATTRIBUTES"); attrs != "" {
		pairs := strings.SplitSeq(attrs, ",")
		for pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 && kv[0] == "deployment.environment" {
				return kv[1]
			}
		}
	}

	return "development"
}

func createSampler(logger *logrus.Logger) sdktrace.Sampler {
	samplerType := os.Getenv("OTEL_TRACES_SAMPLER")
	if samplerType == "" {
		// Default: always on
		return sdktrace.AlwaysSample()
	}

	samplerArg := os.Getenv("OTEL_TRACES_SAMPLER_ARG")

	switch samplerType {
	case "always_on":
		return sdktrace.AlwaysSample()
	case "always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		if samplerArg != "" {
			// Parse ratio (0.0 to 1.0)
			// For simplicity, we'll use the default parser
			return sdktrace.TraceIDRatioBased(parseFloat(samplerArg, 1.0))
		}
		return sdktrace.AlwaysSample()
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio":
		ratio := parseFloat(samplerArg, 1.0)
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	default:
		logger.WithField("sampler", samplerType).Warn("OTEL: Unknown sampler type, using always_on")
		return sdktrace.AlwaysSample()
	}
}

func parseFloat(s string, defaultVal float64) float64 {
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return defaultVal
	}
	if f < 0.0 {
		return 0.0
	}
	if f > 1.0 {
		return 1.0
	}
	return f
}

func getMaxAttributeSize() int {
	sizeStr := os.Getenv("MCP_TRACING_MAX_ATTRIBUTE_SIZE")
	if sizeStr == "" {
		return defaultMaxAttributeSize
	}

	var size int
	if _, err := fmt.Sscanf(sizeStr, "%d", &size); err != nil {
		return defaultMaxAttributeSize
	}

	if size < minAttributeSize {
		return minAttributeSize
	}
	if size > maxAttributeSize {
		return maxAttributeSize
	}

	return size
}

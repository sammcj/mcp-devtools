package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sammcj/mcp-devtools/internal/telemetry"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TestW3CTraceContextPropagation validates that trace context is properly propagated
// across HTTP boundaries using W3C Trace Context headers
func TestW3CTraceContextPropagation(t *testing.T) {
	// Enable tracing for this test
	setupOTELEndpoint(t)
	defer cleanupOTELEndpoint()

	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.DebugLevel)

	// Initialise tracer
	shutdown, err := telemetry.InitTracer(logger)
	if err != nil {
		t.Fatalf("Failed to initialise tracer: %v", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			t.Errorf("Failed to shutdown tracer: %v", err)
		}
	}()

	if !telemetry.IsEnabled() {
		t.Skip("Tracing not enabled, skipping trace context propagation test")
	}

	// Create a mock HTTP server that validates trace context headers
	var receivedTraceParent string
	var receivedTraceState string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTraceParent = r.Header.Get("traceparent")
		receivedTraceState = r.Header.Get("tracestate")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	// Create a parent span to simulate an incoming request
	tracer := telemetry.GetTracer()
	ctx, parentSpan := tracer.Start(context.Background(), "test.parent.request",
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer parentSpan.End()

	// Get the span context to verify later
	parentSpanCtx := parentSpan.SpanContext()

	// Create an HTTP client with OTEL instrumentation
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	client = telemetry.WrapHTTPClient(client)

	// Make request with trace context
	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Validate that trace headers were sent
	if receivedTraceParent == "" {
		t.Error("Expected traceparent header to be propagated, but it was empty")
	}

	// Parse traceparent header (format: version-trace_id-span_id-trace_flags)
	// Example: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
	var version, traceID, spanID, flags string
	n, err := fmt.Sscanf(receivedTraceParent, "%2s-%32s-%16s-%2s", &version, &traceID, &spanID, &flags)
	if err != nil || n != 4 {
		t.Errorf("Invalid traceparent format: %q (parsed %d fields, err: %v)", receivedTraceParent, n, err)
	}

	// Validate version
	if version != "00" {
		t.Errorf("Expected traceparent version 00, got %q", version)
	}

	// Validate trace ID matches parent span
	expectedTraceID := parentSpanCtx.TraceID().String()
	if traceID != expectedTraceID {
		t.Errorf("Trace ID mismatch: expected %s, got %s", expectedTraceID, traceID)
	}

	// Validate span ID is NOT the parent span ID (it should be the child span)
	parentSpanID := parentSpanCtx.SpanID().String()
	if spanID == parentSpanID {
		t.Errorf("Child span ID should differ from parent span ID, both are %s", spanID)
	}

	t.Logf("✅ W3C Trace Context propagated successfully:")
	t.Logf("   traceparent: %s", receivedTraceParent)
	t.Logf("   Trace ID: %s (matches parent: %v)", traceID, traceID == expectedTraceID)
	t.Logf("   Span ID: %s (differs from parent: %v)", spanID, spanID != parentSpanID)
	if receivedTraceState != "" {
		t.Logf("   tracestate: %s", receivedTraceState)
	}
}

// TestHTTPTraceContextExtraction validates that incoming trace context is extracted
// from HTTP request headers
func TestHTTPTraceContextExtraction(t *testing.T) {
	// Enable tracing for this test
	setupOTELEndpoint(t)
	defer cleanupOTELEndpoint()

	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.DebugLevel)

	// Initialise tracer
	shutdown, err := telemetry.InitTracer(logger)
	if err != nil {
		t.Fatalf("Failed to initialise tracer: %v", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			t.Errorf("Failed to shutdown tracer: %v", err)
		}
	}()

	if !telemetry.IsEnabled() {
		t.Skip("Tracing not enabled, skipping trace context extraction test")
	}

	// Create a mock incoming request with trace context
	incomingTraceParent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	incomingTraceState := "vendor=value"

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("traceparent", incomingTraceParent)
	req.Header.Set("tracestate", incomingTraceState)

	// Extract trace context using OTEL propagator
	propagator := telemetry.GetTextMapPropagator()
	ctx := propagator.Extract(context.Background(), propagation.HeaderCarrier(req.Header))

	// Start a span with the extracted context
	tracer := telemetry.GetTracer()
	extractedCtx, span := tracer.Start(ctx, "test.extracted.span")
	defer span.End()

	// Get the span context
	spanCtx := trace.SpanContextFromContext(extractedCtx)

	// Validate that the trace ID matches the incoming trace parent
	expectedTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	actualTraceID := spanCtx.TraceID().String()

	if actualTraceID != expectedTraceID {
		t.Errorf("Extracted trace ID mismatch: expected %s, got %s", expectedTraceID, actualTraceID)
	}

	// Validate that the span is valid and sampled
	if !spanCtx.IsValid() {
		t.Error("Extracted span context is not valid")
	}

	if !spanCtx.IsSampled() {
		t.Error("Extracted span context is not sampled (expected sampled based on trace flags)")
	}

	t.Logf("✅ Trace context extracted successfully:")
	t.Logf("   Original traceparent: %s", incomingTraceParent)
	t.Logf("   Extracted Trace ID: %s (matches: %v)", actualTraceID, actualTraceID == expectedTraceID)
	t.Logf("   Span valid: %v, sampled: %v", spanCtx.IsValid(), spanCtx.IsSampled())
}

// TestDistributedTracingEndToEnd validates end-to-end distributed tracing
// across multiple HTTP hops
func TestDistributedTracingEndToEnd(t *testing.T) {
	// Enable tracing for this test
	setupOTELEndpoint(t)
	defer cleanupOTELEndpoint()

	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.DebugLevel)

	// Initialise tracer
	shutdown, err := telemetry.InitTracer(logger)
	if err != nil {
		t.Fatalf("Failed to initialise tracer: %v", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			t.Errorf("Failed to shutdown tracer: %v", err)
		}
	}()

	if !telemetry.IsEnabled() {
		t.Skip("Tracing not enabled, skipping distributed tracing test")
	}

	// Create a chain of mock servers simulating distributed services
	var traceIDAtService2 string
	var traceIDAtService3 string

	// Service 3 (leaf service)
	service3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context
		propagator := telemetry.GetTextMapPropagator()
		ctx := propagator.Extract(context.Background(), propagation.HeaderCarrier(r.Header))

		// Start span
		tracer := telemetry.GetTracer()
		_, span := tracer.Start(ctx, "service3.operation")
		defer span.End()

		spanCtx := span.SpanContext()
		traceIDAtService3 = spanCtx.TraceID().String()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service": "3"}`))
	}))
	defer service3.Close()

	// Service 2 (intermediate service that calls service 3)
	service2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context
		propagator := telemetry.GetTextMapPropagator()
		ctx := propagator.Extract(context.Background(), propagation.HeaderCarrier(r.Header))

		// Start span
		tracer := telemetry.GetTracer()
		spanCtx, span := tracer.Start(ctx, "service2.operation")
		defer span.End()

		spanContext := span.SpanContext()
		traceIDAtService2 = spanContext.TraceID().String()

		// Call service 3 with propagated context
		client := &http.Client{Timeout: 5 * time.Second}
		client = telemetry.WrapHTTPClient(client)

		req, _ := http.NewRequestWithContext(spanCtx, "GET", service3.URL, nil)
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service": "2"}`))
	}))
	defer service2.Close()

	// Service 1 (entry point)
	tracer := telemetry.GetTracer()
	ctx, service1Span := tracer.Start(context.Background(), "service1.operation",
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer service1Span.End()

	service1SpanCtx := service1Span.SpanContext()
	traceIDAtService1 := service1SpanCtx.TraceID().String()

	// Call service 2 with instrumented client
	client := &http.Client{Timeout: 5 * time.Second}
	client = telemetry.WrapHTTPClient(client)

	req, err := http.NewRequestWithContext(ctx, "GET", service2.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Validate that all services share the same trace ID
	if traceIDAtService2 != traceIDAtService1 {
		t.Errorf("Trace ID mismatch between service1 and service2: %s != %s",
			traceIDAtService1, traceIDAtService2)
	}

	if traceIDAtService3 != traceIDAtService1 {
		t.Errorf("Trace ID mismatch between service1 and service3: %s != %s",
			traceIDAtService1, traceIDAtService3)
	}

	t.Logf("✅ Distributed tracing across 3 services:")
	t.Logf("   Service 1 Trace ID: %s", traceIDAtService1)
	t.Logf("   Service 2 Trace ID: %s (matches: %v)", traceIDAtService2, traceIDAtService2 == traceIDAtService1)
	t.Logf("   Service 3 Trace ID: %s (matches: %v)", traceIDAtService3, traceIDAtService3 == traceIDAtService1)
}

// Helper function to set up OTEL endpoint for testing
func setupOTELEndpoint(t *testing.T) {
	// Use a mock OTLP endpoint for testing
	// In real scenarios, this would be a Jaeger or OTLP collector
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
		t.Log("Set OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 for test")
	}
}

// Helper function to clean up OTEL endpoint
func cleanupOTELEndpoint() {
	// Leave endpoint as-is for debugging
	// Could unset if needed: os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
}

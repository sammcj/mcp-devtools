# Observability with OpenTelemetry

mcp-devtools includes optional distributed tracing and metrics support via OpenTelemetry (OTEL).

- [Observability with OpenTelemetry](#observability-with-opentelemetry)
  - [Configuration](#configuration)
  - [Metrics](#metrics)
  - [What Gets Traced?](#what-gets-traced)
  - [MCP Semantic Conventions](#mcp-semantic-conventions)
  - [Session-Based Correlation](#session-based-correlation)
  - [Privacy \& Security](#privacy--security)
  - [Performance Impact](#performance-impact)
  - [Troubleshooting](#troubleshooting)
  - [Further Reading](#further-reading)

---

With tracing enabled, you can observe:

**Traces (Request-Level Detail):**
- Tool execution latency and errors
- HTTP requests to external APIs
- Security framework checks (opt-in)
- W3C Trace Context propagation across HTTP boundaries

**Metrics (Aggregated Operational Data):**
- Tool invocation counts and error rates
- Latency percentiles (P50/P95/P99)
- Active session tracking
- Cache hit ratios
- Security check overhead (opt-in)

**Key Design Principles:**
- **Disabled by default** - Zero overhead when not configured
- **Environment-based** - Standard OTEL environment variables
- **Privacy-first** - Automatic sanitisation of sensitive data
- **Vendor-neutral** - Works with any OTLP-compatible backend

![otel screenshot in Jaeger](otel.png)

## Configuration

- See [./observability/examples/README.md](./observability/examples/README.md) for example setups with Jaeger and optionally Prometheus + Grafana.

### Standard OTEL Environment Variables

```bash
# Enable tracing (required)
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318  # HTTP
# OR
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317         # gRPC

# Service identification (optional)
OTEL_SERVICE_NAME=mcp-devtools
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production,service.version=1.0.0

# Sampling (optional, default: always on when enabled)
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1  # 10% sampling

# Protocol selection (optional, default: http/protobuf)
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf  # or grpc

# Authentication headers (optional)
OTEL_EXPORTER_OTLP_HEADERS=x-api-key=secret123

# Explicit disable (optional)
OTEL_SDK_DISABLED=false
```

### mcp-devtools Specific Configuration

```bash
# Disable tracing for specific tools (comma-separated)
MCP_TRACING_DISABLED_TOOLS=filesystem,memory

# Maximum span attribute size (default: 4KB, range: 1KB-64KB)
MCP_TRACING_MAX_ATTRIBUTE_SIZE=4096

# Enable security framework tracing (default: false)
MCP_TRACING_SECURITY_ENABLED=true

# Enable specific metric groups (comma-separated, default: tool,session)
# Available groups: tool, session, cache, security
MCP_METRICS_GROUPS=tool,session,cache,security

# Metric export interval (default: 60s)
OTEL_METRIC_EXPORT_INTERVAL=60
```

## Metrics

### Overview

Metrics provide aggregated operational insights that complement request-level traces. Whilst traces help diagnose individual requests, metrics enable trend analysis, capacity planning, and alerting.

**Metrics vs Traces:**
- **Traces**: "Why did this specific request fail?" → Detailed debugging
- **Metrics**: "Is my error rate increasing?" → Operational trends

Both use the same OTLP endpoint and are enabled/disabled together.

### Available Metrics

#### Tool Metrics

**`mcp.tool.calls`** (Counter)
- Total tool invocations
- Labels: `tool.name`, `transport`, `result` (success/error)
- Use case: Track usage patterns, error rates per tool

**`mcp.tool.duration`** (Histogram, milliseconds)
- Tool execution latency distribution
- Labels: `tool.name`, `transport`
- Buckets: `[10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000]`
- Use case: P50/P95/P99 latency analysis, identify slow tools

**`mcp.tool.errors`** (Counter)
- Categorised tool errors
- Labels: `tool.name`, `error.type`
- Error types: `network`, `timeout`, `validation`, `external_api`, `internal`, `security`
- Use case: Identify common failure patterns

#### Session Metrics

**`mcp.session.active`** (UpDownCounter)
- Current concurrent sessions
- Labels: `transport`
- Use case: Monitor load, capacity planning

**`mcp.session.duration`** (Histogram, seconds)
- Session lifetime distribution
- Labels: `transport`
- Buckets: `[10, 30, 60, 300, 600, 1800, 3600, 7200]`
- Use case: Understand typical session lengths

**`mcp.session.tool_count`** (Histogram)
- Tools executed per session
- Buckets: `[1, 2, 5, 10, 20, 50, 100, 200]`
- Use case: Analyse workflow complexity

#### Cache Metrics

**`cache.operations`** (Counter)
- Cache operations
- Labels: `tool.name`, `operation` (get/set/delete), `result` (hit/miss)
- Use case: Cache effectiveness analysis

**`cache.hit_ratio`** (Gauge, 0.0-1.0)
- Real-time cache hit ratio
- Labels: `tool.name`
- Use case: Optimisation opportunities

#### Security Metrics

Enabled when `security` is included in `MCP_METRICS_GROUPS`

**`security.checks`** (Counter)
- Security framework activity
- Labels: `action` (allow/warn/block), `source.type` (url/file/text)

**`security.rule.triggers`** (Counter)
- Rule trigger frequency
- Labels: `rule.name`, `action`

**`security.check.duration`** (Histogram, milliseconds)
- Security check overhead
- Labels: `action`
- Buckets: `[1, 5, 10, 25, 50, 100, 250]`

### Example Queries

#### Prometheus/PromQL

```promql
# Tool error rate (requests/second)
sum(rate(mcp_tool_errors[5m])) by (tool_name, error_type)

# P95 latency by tool
histogram_quantile(0.95,
  sum(rate(mcp_tool_duration_bucket[5m])) by (tool_name, le)
)

# Cache hit ratio
sum(rate(cache_operations{result="hit"}[5m]))
/
sum(rate(cache_operations{operation="get"}[5m]))

# Active sessions by transport
mcp_session_active

# Top 5 slowest tools (P95)
topk(5,
  histogram_quantile(0.95,
    sum(rate(mcp_tool_duration_bucket[5m])) by (tool_name, le)
  )
)

# Error rate threshold alerting
rate(mcp_tool_errors[5m]) > 0.1
```

### Enabling Metric Groups

Metrics are opt-in via the `MCP_METRICS_GROUPS` environment variable:

```bash
# Enable all metric groups
MCP_METRICS_GROUPS=tool,session,cache,security

# Enable only tool and session metrics (default if not specified)
MCP_METRICS_GROUPS=tool,session

# Enable only tool metrics
MCP_METRICS_GROUPS=tool
```

**Default behaviour**: If `MCP_METRICS_GROUPS` is not set, `tool` and `session` metrics are enabled by default.

Available groups: `tool`, `session`, `cache`, `security`

## What Gets Traced?

### Tool Execution

Every tool invocation creates a span with:

- **Tool name** - Which tool was executed
- **Arguments** - Sanitised arguments (no sensitive data)
- **Success/failure** - Whether the tool succeeded
- **Session ID** - For correlating multi-tool workflows
- **Duration** - How long the tool took

Example span attributes:
```
mcp.tool.name: internet_search
mcp.tool.arguments: {"query":"golang best practices","count":5}
mcp.tool.result.success: true
mcp.session.id: a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

### HTTP Requests

All HTTP requests to external APIs create child spans with:

- **HTTP method** - GET, POST, etc.
- **URL** - Sanitised URL (credentials removed)
- **Status code** - Response status
- **Response size** - Size in bytes
- **Target host** - Hostname of external service

This applies to all tools making HTTP requests (internet search, package docs, GitHub, AWS docs, etc.).

### Security Framework (Opt-in)

When `MCP_TRACING_SECURITY_ENABLED=true`, security content analysis creates spans with:

- **Content size** - Size of content analysed
- **Security action** - allow/warn/block
- **Risk factors** - Which rules matched

### W3C Trace Context Propagation

HTTP transport automatically:

- **Extracts** trace context from incoming HTTP request headers (`traceparent`, `tracestate`)
- **Propagates** trace context to all outbound HTTP requests
- **Supports** Baggage headers for optional metadata

This enables distributed tracing when mcp-devtools is part of a larger OTEL-instrumented system.

## MCP Semantic Conventions

mcp-devtools follows standardised attribute naming for ecosystem interoperability:

### MCP Tool Attributes

| Attribute                 | Type    | Description                |
|---------------------------|---------|----------------------------|
| `mcp.tool.name`           | string  | Tool identifier            |
| `mcp.tool.arguments`      | string  | Sanitised arguments (JSON) |
| `mcp.tool.result.success` | boolean | Execution success          |
| `mcp.tool.result.error`   | string  | Error message if failed    |
| `mcp.session.id`          | string  | Session identifier         |

### HTTP Attributes

Standard OTEL HTTP semantic conventions are used:

| Attribute            | Type   | Description            |
|----------------------|--------|------------------------|
| `http.method`        | string | HTTP method            |
| `http.url`           | string | Sanitised URL          |
| `http.status_code`   | int    | Response status        |
| `http.response.size` | int    | Response size in bytes |
| `net.peer.name`      | string | Target hostname        |

### Security Attributes

| Attribute               | Type   | Description               |
|-------------------------|--------|---------------------------|
| `security.action`       | string | allow/warn/block          |
| `security.content.size` | int    | Size of content scanned   |
| `security.risk.factor`  | string | First matched risk factor |

## Session-Based Correlation

### How Sessions are Grouped

mcp-devtools uses **W3C Trace Context** for session correlation. Related tool calls are automatically grouped through:

1. **stdio Transport**: Single session span created at startup
   - Session span is created when stdio server starts
   - Session span context is stored globally
   - All tool calls during the process lifetime become **child spans** of the session span
   - **All tool spans share the same trace ID** (the session's trace ID)
   - Each tool call has a unique span ID but inherits the parent trace ID
   - Session ID included in all spans for additional filtering

2. **HTTP Transport**: Trace context propagation
   - Client sends `traceparent` header with requests
   - All tool calls with same trace ID are automatically grouped
   - Works with any OTEL-compatible client

### Session Correlation in Practice

**Example workflow** (AI agent calling multiple tools):

```
Trace ID: 4bf92f3577b34da6a3ce929d0e0e4736 (single trace for entire session)
Session ID: abc123

Session Span (parent):
└─ mcp.session (stdio transport)
   ├─ internet_search (200ms) - child span
   │  └─ HTTP GET brave.com (180ms)
   ├─ fetch_url (350ms) - child span
   │  └─ HTTP GET example.com (320ms)
   └─ think (50ms) - child span

All spans share trace ID: 4bf92f3577b34da6a3ce929d0e0e4736
Each span has unique span ID
```

**In Jaeger UI**:
- View by trace ID `4bf92f3577b34da6a3ce929d0e0e4736` to see the complete workflow
- All tool calls appear as children of the session span
- Timing waterfall shows the execution sequence
- Filter by `mcp.session.id:abc123` for additional session filtering
- Filter by service="mcp-devtools" to see all activity

**In AWS X-Ray**:
- Service map shows: `mcp-devtools` → external APIs
- Trace view shows complete workflow with timing breakdown
- Group by trace ID for session-level analysis

### Technical Implementation

**How it works:**

1. **Initialisation**: On startup, any previous session span context is cleared to prevent cross-session contamination
2. **Session span creation**: When stdio transport starts, a session span is created with session metadata
3. **Immediate span end**: The session span is ended immediately after creation
4. **Force flush**: `ForceFlush()` is called on the tracer provider to ensure the session span exports to the backend immediately
5. **Span context storage**: The session span's context is stored globally for tool spans to reference
6. **Tool execution**: When `StartToolSpan()` is called, it retrieves the global session span context
7. **Trace context propagation**: The session span context is injected into a W3C Trace Context carrier, then extracted into the tool's context using the text map propagator
8. **Parent-child relationship**: The extracted trace context ensures tool spans inherit the session's trace ID and become proper children
9. **Tool span export**: Tool spans export immediately after ending via the batch processor
10. **Session cleanup**: Global span context is cleared when the session ends

This approach creates a proper parent-child hierarchy:
- **Session span exports first** via immediate end + force flush
- **Tool spans inherit trace context** via W3C Trace Context propagation (inject → extract pattern)
- **All spans grouped** under the same trace ID
- **Proper hierarchy** shows tool spans nested under session span
- **No clock skew warnings** because the parent span is already in the backend when children arrive

The key insights:
1. **Force flush is critical**: The OTEL batch processor only exports ended spans asynchronously. Without `ForceFlush()`, the session span might not export before tool spans, causing "invalid parent span IDs" warnings.
2. **W3C Trace Context propagation**: Using the inject→extract pattern properly establishes parent-child relationships across context boundaries, the same mechanism used for distributed tracing.

## Privacy & Security

### Automatic Data Sanitisation

mcp-devtools should not include the following in traces:

- API keys, tokens, passwords
- SSH keys, certificates
- User credentials
- OAuth tokens or secrets

**Sanitisation applied to:**

- **URLs**: Credentials and sensitive query parameters removed
- **Arguments**: Known sensitive fields redacted
- **File paths**: Truncated to avoid exposing directory structures
- **Large attributes**: Truncated to max size with `truncated=true` flag

Example URL sanitisation:
```
Input:  https://user:pass@api.example.com/search?api_key=secret&q=test
Output: https://api.example.com/search?q=test
```

### Sampling for High-Volume Deployments

For production HTTP deployments with high request rates:

```bash
# Sample 1% of traces
OTEL_TRACES_SAMPLER=traceidratio
OTEL_TRACES_SAMPLER_ARG=0.01

# Parent-based sampling (always sample errors)
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1
```

## Performance Impact

**When disabled** (default):
- Zero overhead
- Noop tracer used
- No network calls

**When enabled**:
- Tool execution: < 1ms per span
- HTTP client: < 2ms per request
- Memory: ~100KB per 1000 spans (buffered)
- Export: Async and batched (non-blocking)

**Recommendations:**
- Use sampling for high-volume HTTP deployments
- Disable tracing for specific tools if needed via `MCP_TRACING_DISABLED_TOOLS`
- Monitor OTLP collector health to avoid export retries

## Troubleshooting

### Traces Not Appearing

**Check endpoint is set:**
```bash
echo $OTEL_EXPORTER_OTLP_ENDPOINT
```

**Check collector is running:**
```bash
curl http://localhost:4318/v1/traces  # Should return 405 Method Not Allowed
```

**Check logs for OTEL errors:**
```bash
# Look for "OTEL:" prefix in logs
./bin/mcp-devtools stdio 2>&1 | grep OTEL
```

**Check if tracing is enabled:**
```bash
# Should see "OTEL: Initialising tracer" if enabled
./bin/mcp-devtools stdio 2>&1 | grep "Initialising tracer"
```

### High Memory Usage

- Reduce `OTEL_TRACES_SAMPLER_ARG` to sample fewer traces
- Check OTLP collector is healthy and processing exports
- Reduce `MCP_TRACING_MAX_ATTRIBUTE_SIZE` if argument values are very large
- Disable tracing for high-volume tools via `MCP_TRACING_DISABLED_TOOLS`

### Collector Connection Failures

Tracing is designed to gracefully degrade:

- Application continues to work normally
- Warning logged: "OTEL: Failed to create exporter, falling back to noop tracer"
- No performance impact beyond initial connection attempt

---

## Further Reading

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [Grafana Tempo Documentation](https://grafana.com/docs/tempo/)
- [AWS X-Ray Documentation](https://docs.aws.amazon.com/xray/)
- [AWS X-Ray with OpenTelemetry](https://aws-otel.github.io/docs/getting-started/x-ray)
- [W3C Trace Context Specification](https://www.w3.org/TR/trace-context/)
- [OTLP Specification](https://opentelemetry.io/docs/specs/otlp/)

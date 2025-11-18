# OpenTelemetry Implementation Plan for MCP DevTools

**Version**: 1.0
**Date**: 2025-11-18
**Status**: Planning

---

## Executive Summary

This document outlines the implementation plan for integrating OpenTelemetry (OTEL) distributed tracing into mcp-devtools. The goal is to provide optional, zero-overhead observability that enables performance analysis, workflow debugging, and operational insights for production deployments whilst maintaining the project's "single binary" philosophy.

---

## Context & Motivation

### Current State

**mcp-devtools** is a high-performance MCP server consolidating 30+ developer tools into a single Go binary. Current observability includes:

- **Structured logging** (logrus) with transport-aware output
- **Tool error logging** (optional file-based logging to `~/.mcp-devtools/logs/tool-errors.log`)
- **Basic metrics** (memory limits, session timeouts)

Whilst sufficient for local stdio usage, production HTTP deployments and complex multi-tool workflows lack:

- End-to-end request tracing across tool chains
- Latency breakdown for external API calls
- Performance bottleneck identification
- Correlation of issues across concurrent clients
- Visibility into agent tool workflows (Claude, Kiro, Gemini agents)

### Why OpenTelemetry?

- **Vendor neutrality**: Works with Jaeger, Zipkin, Grafana Tempo, Honeycomb, Datadog, etc.
- **Standard protocol**: OTLP (OpenTelemetry Protocol) is vendor-agnostic
- **Future-proof**: Industry standard backed by CNCF
- **Ecosystem**: Rich instrumentation libraries for Go HTTP, database, etc.
- **Flexibility**: Easy to switch backends or use multiple exporters

**Use Cases Addressed**:

1. **Production HTTP Deployments**: Multi-client scenarios with OAuth authentication
2. **Performance Optimisation**: Systematic measurement of tool execution and external API latency
3. **Complex Workflows**: AI agents calling multiple tools in sequence
4. **Debugging Agent Tools**: Tracing LLM invocations and multi-step reasoning
5. **Multi-Transport Analysis**: Comparing stdio vs HTTP vs SSE performance

---

## Design Principles

### 1. Optional & Zero-Overhead

- **Disabled by default**: No performance impact when not configured
- **Environment-based activation**: Enable via `OTEL_EXPORTER_OTLP_ENDPOINT`
- **Graceful degradation**: Continue operating if collector is unavailable
- **No required dependencies**: OTEL SDK is the only addition

### 2. Single Binary Philosophy

- **No separate binaries**: All OTEL code compiled into main binary
- **No mandatory infrastructure**: Users opt-in to running OTEL collector
- **Conditional compilation**: Consider build tags for completely omitting OTEL

### 3. Standards Compliance

- **W3C Trace Context**: Standard trace header propagation for HTTP transport
- **OTLP Protocol**: gRPC or HTTP exporters to standard collectors
- **Semantic Conventions**: Follow OTEL semantic conventions for span attributes

### 4. Privacy & Security

- **No sensitive data**: Never trace credentials, tokens, API keys
- **Configurable sampling**: Support rate-limiting for high-volume scenarios
- **Opt-out support**: Allow users to disable specific tool tracing

---

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     MCP DevTools Binary                      │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              OTEL Tracer Provider                       │ │
│  │  (Initialised from environment, disabled by default)    │ │
│  └────────────────────────────────────────────────────────┘ │
│                           │                                  │
│                           ▼                                  │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    │
│  │   stdio     │    │    HTTP     │    │     SSE     │    │
│  │  Transport  │    │  Transport  │    │  Transport  │    │
│  │             │    │   (W3C TC)  │    │             │    │
│  └─────────────┘    └─────────────┘    └─────────────┘    │
│         │                  │                   │            │
│         └──────────────────┴───────────────────┘            │
│                           │                                  │
│                           ▼                                  │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              Tool Execution Middleware                  │ │
│  │         (Creates span per tool invocation)              │ │
│  └────────────────────────────────────────────────────────┘ │
│                           │                                  │
│                           ▼                                  │
│  ┌─────────┬─────────┬─────────┬─────────┬─────────┬──────┐ │
│  │  HTTP   │ Security│  Agent  │  Cache  │   DB    │ ...  │ │
│  │ Client  │Framework│  Calls  │ Ops     │ Queries │      │ │
│  │ (auto)  │(manual) │(manual) │(manual) │ (auto)  │      │ │
│  └─────────┴─────────┴─────────┴─────────┴─────────┴──────┘ │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │   OTLP Exporter       │
              │   (gRPC or HTTP)      │
              └───────────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │  OTEL Collector       │
              │  (User-provided)      │
              └───────────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │  Backend (Choice of): │
              │  - Jaeger             │
              │  - Langfuse           │
              │  - Grafana Tempo      │
              │  - Honeycomb          │
              │  - Datadog            │
              │  - etc.               │
              └───────────────────────┘
```

### Key Instrumentation Points

#### 1. Tool Execution (Highest Priority)

**What**: Span for every tool invocation via MCP
**Span Name**: `tool.execute.{tool_name}`
**Attributes**:
- `tool.name`: Tool name (e.g., "internet_search")
- `tool.category`: Tool category (e.g., "search_discovery")
- `tool.arguments`: Sanitised arguments (no sensitive data)
- `tool.result.success`: true/false
- `tool.result.error`: Error message (if failed)
- `tool.cache.hit`: true/false (if applicable)

**Implementation**: Middleware wrapper in `main.go` around `currentTool.Execute()`

#### 2. HTTP External Requests (High Priority)

**What**: Auto-instrumentation of HTTP client calls
**Span Name**: `http.client.{method}`
**Attributes**: Standard HTTP semantic conventions
- `http.method`: GET, POST, etc.
- `http.url`: Sanitised URL (no query params with secrets)
- `http.status_code`: Response status
- `http.response.size`: Response size in bytes
- `net.peer.name`: Target hostname

**Implementation**:
- Use `otelhttp.NewTransport()` wrapper for all HTTP clients
- Applies to: internet search, package docs, GitHub, AWS docs, etc.

#### 3. Security Framework Checks (Medium Priority)

**What**: Trace security rule evaluation
**Span Name**: `security.check`
**Attributes**:
- `security.rule.matched`: Rule name that matched (if any)
- `security.action`: allow/block/warn
- `security.content.size`: Size of content scanned

**Implementation**: Manual instrumentation in `internal/security/`

#### 4. Cache Operations (Medium Priority)

**What**: Trace cache hits/misses
**Span Name**: `cache.{operation}`
**Attributes**:
- `cache.key`: Cache key (sanitised)
- `cache.hit`: true/false
- `cache.operation`: get/set/delete

**Implementation**: Wrapper around `sync.Map` operations

#### 5. Agent Tool Calls (Medium Priority)

**What**: Trace LLM agent invocations
**Span Name**: `agent.{agent_name}.execute`
**Attributes**:
- `agent.name`: claude/kiro/gemini/etc.
- `agent.prompt.size`: Size of prompt in characters
- `agent.response.size`: Size of response
- `agent.model`: Model used (if available)
- `agent.error`: Error message (if failed)

**Implementation**: Manual instrumentation in agent tool packages

#### 6. Session Lifecycle (Low Priority - HTTP Only)

**What**: Trace HTTP session creation/termination
**Span Name**: `session.{lifecycle}`
**Attributes**:
- `session.id`: Session identifier
- `session.timeout`: Configured timeout
- `session.duration`: Actual duration (on termination)

**Implementation**: Instrumentation in `TimeoutSessionManager`

---

## Implementation Strategy

### Phase 1: Core Infrastructure (Foundational)

**Goal**: Establish OTEL initialisation and basic tool tracing

**Tasks**:
1. Add OTEL SDK dependencies to `go.mod`
2. Create `internal/telemetry/` package for tracer initialisation
3. Implement environment-based tracer provider setup
4. Add graceful shutdown handling for trace export
5. Implement tool execution middleware with span creation
6. Add basic span attributes (tool name, success/failure)
7. Create example configuration and documentation

**Success Criteria**:
- OTEL disabled by default (zero overhead)
- When enabled, tool executions create spans
- Spans exported to OTLP collector
- No performance degradation when disabled

### Phase 2: HTTP Client Instrumentation (External APIs)

**Goal**: Auto-instrument all HTTP requests to external services

**Tasks**:
1. Wrap HTTP clients with `otelhttp.NewTransport()`
2. Apply to all tool packages making HTTP requests
3. Sanitise URLs to remove sensitive query parameters
4. Add custom attributes for API-specific metadata
5. Test with internet search, package docs, GitHub tools

**Success Criteria**:
- All HTTP requests create child spans
- External API latency clearly visible
- No sensitive data leaked in span attributes

### Phase 3: Advanced Instrumentation (Optimisation)

**Goal**: Add deeper visibility into internal operations

**Tasks**:
1. Instrument security framework checks
2. Add cache operation tracing
3. Instrument agent tool LLM calls
4. Add custom metrics (optional OTEL metrics support)
5. Implement sampling configuration

**Success Criteria**:
- Security overhead measurable
- Cache hit rate observable
- Agent tool performance analysable

### Phase 4: W3C Trace Context Propagation (HTTP Transport)

**Goal**: Support distributed tracing across HTTP boundaries

**Tasks**:
1. Extract W3C Trace Context from HTTP request headers
2. Propagate trace context to downstream HTTP calls
3. Add support for Baggage header (optional metadata)
4. Test with multi-hop tool workflows

**Success Criteria**:
- Trace context propagated through HTTP transport
- Multi-tool workflows show connected traces
- Works with external OTEL-instrumented services

### Phase 5: Documentation & Examples (Production Ready)

**Goal**: Make OTEL accessible to users

**Tasks**:
1. Write `docs/observability.md` with setup guide
2. Create docker-compose example with Jaeger (our preferred backend)
3. Add grafana/tempo example
4. Document environment variables and configuration
5. Add troubleshooting section
6. Create performance benchmarks (with/without tracing)

**Success Criteria**:
- Users can set up tracing in < 5 minutes
- Multiple backend examples provided
- Performance impact documented

---

## Configuration

### Environment Variables

Following OTEL standard environment variables:

```bash
# Enable tracing by setting OTLP endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318  # HTTP
# OR
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317         # gRPC

# Optional: Service name (default: mcp-devtools)
OTEL_SERVICE_NAME=mcp-devtools

# Optional: Sampling (default: always on when enabled)
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1  # 10% sampling

# Optional: Protocol (default: http/protobuf)
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf  # or grpc

# Optional: Headers for authentication
OTEL_EXPORTER_OTLP_HEADERS=x-api-key=secret123

# Optional: Resource attributes
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production,service.version=1.0.0

# Optional: Disable tracing (explicit)
OTEL_SDK_DISABLED=true
```

### Custom Configuration (mcp-devtools specific)

```bash
# Disable tracing for specific tools (comma-separated)
MCP_TRACING_DISABLED_TOOLS=filesystem,memory

# Maximum span attribute size (default: 4KB)
MCP_TRACING_MAX_ATTRIBUTE_SIZE=4096

# Enable detailed cache tracing (default: false, can be noisy)
MCP_TRACING_CACHE_ENABLED=true

# Enable security framework tracing (default: false)
MCP_TRACING_SECURITY_ENABLED=true
```

---

## Example: Basic Setup

### Running with Jaeger

```bash
# Start Jaeger all-in-one (collector + UI)
docker run -d --name jaeger \
  -p 4318:4318 \
  -p 16686:16686 \
  jaegertracing/jaeger:2.0

# Run mcp-devtools with tracing
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 \
OTEL_SERVICE_NAME=mcp-devtools \
./bin/mcp-devtools --transport http --port 18080

# View traces at http://localhost:16686
```

### Running with Grafana Tempo

```bash
# docker-compose.yml provided in docs/examples/
docker compose -f docs/examples/tempo-compose.yml up -d

# Run mcp-devtools
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 \
./bin/mcp-devtools stdio
```

---

## Security & Privacy Considerations

### Data Sanitisation

**Never trace**:
- API keys, tokens, passwords
- SSH keys, certificates
- User credentials
- PII (personally identifiable information)

**Sanitisation approach**:
- Redact sensitive fields from arguments before adding to span
- URL sanitisation to remove query parameters with secrets
- File path truncation to avoid leaking directory structures
- Use hash/checksum for content identification instead of content itself

### Sampling for High-Volume Deployments

For production HTTP deployments with hundreds of requests per second:

```bash
# Sample 1% of traces (reduces overhead and storage)
OTEL_TRACES_SAMPLER=traceidratio
OTEL_TRACES_SAMPLER_ARG=0.01

# OR: Always sample errors, probabilistically sample success
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1
```

### Performance Impact

**Expected overhead** (when enabled):
- Tool execution: < 1ms per span (negligible for most tools)
- HTTP client: < 2ms per request
- Memory: ~100KB per 1000 spans (buffered before export)

**Mitigation**:
- Async export (non-blocking)
- Batch span processor (reduces network calls)
- Sampling for high-throughput scenarios
- Ability to disable per-tool

---

## Testing Strategy

### Unit Tests

1. Test tracer provider initialisation (enabled/disabled)
2. Test span creation for tool execution
3. Test attribute sanitisation (no secrets leaked)
4. Test graceful degradation (collector unavailable)

### Integration Tests

1. End-to-end trace with Jaeger test collector
2. Multi-tool workflow trace validation
3. HTTP transport trace context propagation
4. Performance benchmark (with/without tracing)

### Load Tests

1. HTTP transport under concurrent load with tracing
2. Verify sampling reduces overhead at high volume
3. Memory usage over extended periods

---

## Migration & Backwards Compatibility

### No Breaking Changes

- OTEL is **completely optional**
- Existing logging and error tracking unchanged
- No configuration changes required for existing users
- Zero impact on binary size for users not enabling tracing

### Graceful Adoption

Users can:
1. **Continue using without tracing**: No action needed
2. **Enable tracing for debugging**: Temporary OTLP endpoint
3. **Production deployment**: Full tracing infrastructure with sampling

---

## Success Metrics

### Adoption Metrics

- Documentation views for observability guide
- GitHub issues mentioning tracing/observability
- Community feedback on tracing usefulness

### Technical Metrics

- Performance overhead: < 5% when enabled, 0% when disabled
- Trace completeness: > 95% of tool executions traced
- Export success rate: > 99% when collector is available
- Graceful degradation: 100% success when collector is unavailable

### Operational Metrics

- P50/P95/P99 latency for tool executions (observable via traces)
- External API latency distribution
- Cache hit rate trends
- Error rate correlation with performance

---

## Risks & Mitigation

### Risk: Performance Overhead

**Mitigation**:
- Disabled by default (zero overhead)
- Async export (non-blocking)
- Sampling support for high volume
- Per-tool disable capability

### Risk: Complexity Creep

**Mitigation**:
- Single package (`internal/telemetry/`) for all OTEL code
- Standard OTEL SDK (no custom tracing logic)
- Clear documentation on when to use tracing
- Optional instrumentation for advanced features

### Risk: Sensitive Data Leakage

**Mitigation**:
- Sanitisation helpers for arguments and URLs
- Unit tests for data leakage
- Security review of all instrumented code
- Clear documentation on what is/isn't traced

### Risk: User Confusion

**Mitigation**:
- Opt-in via environment variables (not flags)
- Examples for popular backends (Jaeger, Tempo, Honeycomb)
- Troubleshooting guide in documentation
- Docker compose examples for quick start

---

## Future Enhancements (Out of Scope)

These are explicitly **not** part of the initial implementation:

1. **OTEL Metrics**: Focus on traces only initially
2. **OTEL Logs**: Continue using logrus for structured logging
3. **Auto-instrumentation**: Manual instrumentation only (Go doesn't support auto-instrumentation well)
4. **Custom exporters**: OTLP only, users choose backend
5. **Trace visualisation**: Use existing tools (Jaeger UI, Grafana, etc.)
6. **Real-time alerting**: Out of scope, use backend capabilities
7. **Custom span processors**: Standard batch processor only

---

## Dependencies

### New Go Dependencies

Note: Use the latest available versions, below versions are just for example purposes.

```go
// go.mod additions
require (
    go.opentelemetry.io/otel v1.32.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.32.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.32.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.32.0
    go.opentelemetry.io/otel/sdk v1.32.0
    go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.57.0
)
```

**Total size impact**: ~2-3MB binary increase (acceptable for optional feature)

### External Dependencies (User-Provided)

- OTEL Collector (optional, user runs separately)
- Jaeger/Tempo/Honeycomb backend (user choice)

---

## Development Checklist

### Phase 1: Core Infrastructure ✓ TO REVIEW

- [ ] Research and select OTEL SDK version (latest stable)
- [ ] Add OTEL dependencies to `go.mod`
- [ ] Create `internal/telemetry/` package structure
- [ ] Implement tracer provider initialisation from environment
- [ ] Implement graceful shutdown and flush on exit
- [ ] Add noop tracer when OTEL disabled (zero overhead)
- [ ] Create tool execution span wrapper in `main.go`
- [ ] Add basic span attributes (tool name, success, duration)
- [ ] Test with local OTLP collector (Jaeger all-in-one)
- [ ] Write unit tests for tracer initialisation
- [ ] Document basic setup in `docs/observability.md`

### Phase 2: HTTP Client Instrumentation

- [ ] Identify all HTTP clients in codebase (internetsearch, packagedocs, github, etc.)
- [ ] Create wrapper utility for HTTP client instrumentation
- [ ] Apply `otelhttp.NewTransport()` to all HTTP clients
- [ ] Implement URL sanitisation (remove sensitive query params)
- [ ] Add custom attributes for API-specific metadata
- [ ] Test with internet search tool (Brave, Google, SearXNG)
- [ ] Test with package documentation tool (Context7)
- [ ] Test with GitHub tool
- [ ] Verify no sensitive data in spans
- [ ] Write integration tests for HTTP instrumentation

### Phase 3: Advanced Instrumentation

- [ ] Implement security framework span creation
- [ ] Add cache operation tracing helpers
- [ ] Instrument agent tools (Claude, Kiro, Gemini)
- [ ] Add sampling configuration support
- [ ] Implement per-tool disable capability (`MCP_TRACING_DISABLED_TOOLS`)
- [ ] Add detailed cache tracing toggle
- [ ] Test cache hit/miss tracing
- [ ] Test security rule evaluation tracing
- [ ] Write unit tests for sanitisation helpers
- [ ] Performance benchmark with/without tracing

### Phase 4: W3C Trace Context (HTTP Transport Only)

- [ ] Implement W3C Trace Context extraction from HTTP headers
- [ ] Propagate trace context to tool execution context
- [ ] Add trace context to outbound HTTP requests
- [ ] Support Baggage header (optional metadata)
- [ ] Test multi-hop tool workflows
- [ ] Test with external OTEL-instrumented services
- [ ] Document trace context propagation
- [ ] Add examples for distributed tracing scenarios

### Phase 5: Documentation & Examples

- [ ] Write comprehensive `docs/observability.md`
- [ ] Create `docs/examples/jaeger-compose.yml`
- [ ] Create `docs/examples/langfuse-compose.yml`
- [ ] Document all environment variables
- [ ] Add troubleshooting section (common issues)
- [ ] Create performance benchmark documentation
- [ ] Add security and privacy best practices
- [ ] Create video/screenshots of trace visualisation
- [ ] Update main `README.md` with observability section

### Phase 6: Testing & Quality Assurance

- [ ] Unit tests for all instrumentation code
- [ ] Integration tests with real OTLP collector
- [ ] Load tests for HTTP transport with tracing
- [ ] Verify sampling reduces overhead appropriately
- [ ] Memory leak testing (extended runs)
- [ ] Test graceful degradation (collector unavailable)
- [ ] Test with multiple concurrent clients (HTTP)
- [ ] Cross-platform testing (macOS, Linux)
- [ ] Run full test suite with tracing enabled
- [ ] Benchmark overhead (should be < 5% when enabled)

### Phase 7: Polish & Release Preparation

- [ ] Code review for security (data leakage)
- [ ] Update `CHANGELOG.md` with observability features
- [ ] Add "Observability" section to main README
- [ ] Create GitHub issue templates for tracing problems
- [ ] Tag release with observability support
- [ ] Announce in release notes
- [ ] Share examples in community discussions

---

## Estimated Effort

**Total Effort**: 3-5 days of focused development

**Breakdown**:
- Phase 1 (Core): 1 day
- Phase 2 (HTTP): 1 day
- Phase 3 (Advanced): 1 day
- Phase 4 (W3C TC): 0.5 day
- Phase 5 (Docs): 1 day
- Phase 6 (Testing): 0.5 day
- Phase 7 (Polish): 0.5 day

**Dependencies**: None (can start immediately)

---

## Conclusion

OpenTelemetry integration provides significant value for production deployments and complex workflows whilst maintaining mcp-devtools' core philosophy of simplicity and performance. By making it optional and following OTEL standards, users gain powerful observability without added complexity for simple use cases.

The phased approach allows incremental delivery of value, starting with basic tool tracing and progressively adding advanced instrumentation. The use of standard OTEL SDK ensures compatibility with the entire observability ecosystem.

**Next Steps**: Review this plan, approve phases, and begin Phase 1 implementation.

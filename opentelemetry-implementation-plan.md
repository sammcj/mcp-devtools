# OpenTelemetry Implementation Plan for MCP DevTools

**Version**: 1.2
**Date**: 2025-12-01
**Status**: Core Implementation Complete (Phases 1-5 ✅) - Testing & Polish Remaining

---

## Executive Summary

This document outlines the implementation plan for integrating OpenTelemetry (OTEL) distributed tracing into mcp-devtools. The goal is to provide optional, zero-overhead observability that enables performance analysis, workflow debugging, and operational insights for production deployments whilst maintaining the project's "single binary" philosophy.

**Update 2025-12-01**: Enhanced plan with MCP community best practices including standardised semantic conventions, token usage tracking for cost optimisation, and session-based correlation for multi-tool workflows.

**Update 2025-12-01 (Latest)**: Core OTEL implementation complete! Phases 1-5 are done with working session correlation, distributed tracing, and comprehensive documentation. Remaining work focuses on LLM token tracking and testing/polish.

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
- **MCP-Specific Conventions**: Define and document MCP/GenAI semantic conventions for ecosystem interoperability

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

### MCP Semantic Conventions

To ensure ecosystem-wide interoperability and consistent trace analysis, we define standardised attribute names for MCP-specific operations. These conventions align with OpenTelemetry's semantic conventions for observability and extend them for Model Context Protocol specifics.

#### Namespace Prefixes

- `mcp.*` - MCP protocol-specific attributes
- `llm.*` - LLM/GenAI-specific attributes
- `cache.*` - Cache operation attributes
- Standard OTEL conventions for HTTP, errors, etc.

#### MCP Tool Attributes

| Attribute                 | Type    | Description                | Example               |
|---------------------------|---------|----------------------------|-----------------------|
| `mcp.tool.name`           | string  | Tool identifier            | "internet_search"     |
| `mcp.tool.category`       | string  | Tool category              | "search_discovery"    |
| `mcp.tool.arguments`      | string  | Sanitised arguments (JSON) | '{"query":"example"}' |
| `mcp.tool.result.success` | boolean | Execution success          | true                  |
| `mcp.tool.result.error`   | string  | Error message if failed    | "API timeout"         |
| `mcp.session.id`          | string  | Session identifier         | "sess_abc123xyz"      |

#### LLM/GenAI Attributes

| Attribute                 | Type   | Description             | Example             |
|---------------------------|--------|-------------------------|---------------------|
| `llm.system`              | string | LLM provider            | "anthropic"         |
| `llm.model`               | string | Model identifier        | "claude-sonnet-4-5" |
| `llm.request.type`        | string | Request type            | "chat"              |
| `llm.usage.input_tokens`  | int    | Input tokens consumed   | 1500                |
| `llm.usage.output_tokens` | int    | Output tokens generated | 800                 |
| `llm.usage.cached_tokens` | int    | Cached tokens used      | 200                 |
| `llm.usage.total_tokens`  | int    | Total tokens            | 2300                |
| `llm.cost.estimated`      | float  | Estimated cost USD      | 0.0234              |
| `llm.temperature`         | float  | Temperature setting     | 0.7                 |
| `llm.max_tokens`          | int    | Max tokens limit        | 4096                |
| `llm.finish_reason`       | string | Completion reason       | "stop"              |

#### Session Attributes

| Attribute                 | Type   | Description         | Example          |
|---------------------------|--------|---------------------|------------------|
| `mcp.session.id`          | string | Unique session ID   | "sess_abc123xyz" |
| `mcp.session.timeout`     | int    | Timeout in seconds  | 3600             |
| `mcp.session.duration`    | int    | Duration in seconds | 245              |
| `mcp.session.tool_count`  | int    | Tools executed      | 5                |
| `mcp.session.error_count` | int    | Failed executions   | 0                |
| `mcp.transport`           | string | Transport type      | "http"           |

#### Implementation

These conventions will be defined as constants in `internal/telemetry/conventions.go`:

```go
package telemetry

const (
    // MCP Tool attributes
    AttrMCPToolName        = "mcp.tool.name"
    AttrMCPToolCategory    = "mcp.tool.category"
    AttrMCPToolArguments   = "mcp.tool.arguments"
    AttrMCPToolSuccess     = "mcp.tool.result.success"
    AttrMCPToolError       = "mcp.tool.result.error"

    // LLM attributes
    AttrLLMSystem          = "llm.system"
    AttrLLMModel           = "llm.model"
    AttrLLMRequestType     = "llm.request.type"
    AttrLLMInputTokens     = "llm.usage.input_tokens"
    AttrLLMOutputTokens    = "llm.usage.output_tokens"
    AttrLLMCachedTokens    = "llm.usage.cached_tokens"
    AttrLLMTotalTokens     = "llm.usage.total_tokens"
    AttrLLMCostEstimated   = "llm.cost.estimated"

    // Session attributes
    AttrMCPSessionID       = "mcp.session.id"
    AttrMCPSessionTimeout  = "mcp.session.timeout"
    AttrMCPSessionDuration = "mcp.session.duration"
    AttrMCPTransport       = "mcp.transport"
)
```

### Key Instrumentation Points

#### 1. Tool Execution (Highest Priority)

**What**: Span for every tool invocation via MCP
**Span Name**: `mcp.tool.execute`
**Attributes** (following MCP semantic conventions):
- `mcp.tool.name`: Tool name (e.g., "internet_search")
- `mcp.tool.category`: Tool category (e.g., "search_discovery")
- `mcp.tool.arguments`: Sanitised arguments (no sensitive data)
- `mcp.tool.result.success`: true/false
- `mcp.tool.result.error`: Error message (if failed)
- `mcp.session.id`: Session identifier (for correlating tools within a session)
- `cache.hit`: true/false (if applicable)

**Implementation**: Middleware wrapper in `main.go` around `currentTool.Execute()`

**Note**: Attribute names use `mcp.` prefix for MCP-specific conventions to enable ecosystem-wide consistency.

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

#### 5. Agent Tool Calls / LLM Invocations (High Priority - Upgraded)

**What**: Trace LLM agent invocations with token usage tracking
**Span Name**: `llm.execute`
**Attributes** (following GenAI semantic conventions):
- `llm.system`: Provider (anthropic/openai/google/etc.)
- `llm.model`: Model identifier (e.g., "claude-sonnet-4")
- `llm.request.type`: Type of request (completion/chat/embedding)
- `llm.usage.input_tokens`: Number of input tokens consumed
- `llm.usage.output_tokens`: Number of output tokens generated
- `llm.usage.cached_tokens`: Number of tokens served from cache (if available)
- `llm.usage.total_tokens`: Total tokens (input + output)
- `llm.cost.estimated`: Estimated cost in USD (if calculable)
- `llm.prompt.size`: Size of prompt in bytes (for non-token-based tracking)
- `llm.response.size`: Size of response in bytes
- `llm.temperature`: Temperature setting (if applicable)
- `llm.max_tokens`: Maximum tokens setting
- `llm.finish_reason`: Completion reason (stop/length/error)
- `mcp.session.id`: Session identifier for correlation
- `error.type`: Error type (if failed)
- `error.message`: Error message (if failed)

**Implementation**: Manual instrumentation in agent tool packages (Claude, Kiro, Gemini, etc.)

**Rationale**: Token usage directly impacts operational costs and latency. This upgraded priority enables cost tracking and context window optimisation, addressing a critical pain point identified in production MCP deployments.

#### 6. Session Lifecycle (Medium Priority - Upgraded for Correlation) ✅ COMPLETED

**What**: Trace session creation/termination and enable cross-tool correlation
**Span Name**: `mcp.session` (single short-lived span per session)
**Attributes**:
- `mcp.session.id`: Unique session identifier (propagated to all tool spans)
- `mcp.transport`: Transport type (stdio/http/sse)

**Implementation**:
- ✅ Session span created at stdio transport startup in `main.go`
- ✅ Session span ended immediately to ensure export before tool spans
- ✅ Session span context stored globally for tool span parent-child relationship
- ✅ All tool spans become children of session span via context propagation
- ✅ Session ID propagated to all tool spans as attribute

**Implementation Approach**:
The implementation uses a **parent-child span relationship** rather than just session IDs:
1. Session span created and ended immediately (exports to backend)
2. Session span context stored globally
3. Tool spans inject session context as parent
4. Result: All tool spans grouped under same trace with proper hierarchy

**Key Insight**: OpenTelemetry's batch processor only exports ended spans. By ending the session span immediately (but storing its context), we ensure the backend receives the parent before any children, eliminating "invalid parent span IDs" warnings.

**Rationale**: Session tracking enables correlation of tool calls within agent workflows. Proper parent-child hierarchy provides better visualisation in backends like Jaeger, showing session span with nested tool executions.

---

## Implementation Strategy

### Phase 1: Core Infrastructure (Foundational)

**Goal**: Establish OTEL initialisation, semantic conventions, and basic tool tracing

**Tasks**:
1. Add OTEL SDK dependencies to `go.mod`
2. Create `internal/telemetry/` package structure:
   - `tracer.go` - Tracer provider initialisation
   - `conventions.go` - MCP/GenAI semantic convention constants
   - `sanitise.go` - Attribute sanitisation helpers
3. Define MCP-specific semantic conventions (see new section below)
4. Implement environment-based tracer provider setup
5. Add graceful shutdown handling for trace export
6. Implement session ID generation and context propagation
7. Implement tool execution middleware with span creation
8. Add span attributes following MCP semantic conventions
9. Create example configuration and documentation

**Success Criteria**:
- OTEL disabled by default (zero overhead)
- When enabled, tool executions create spans with standardised attributes
- Session IDs generated and propagated through context
- Spans exported to OTLP collector
- No performance degradation when disabled
- Semantic conventions documented in code and docs

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

### Phase 3: Advanced Instrumentation (LLM & Internal Operations)

**Goal**: Add deeper visibility into LLM calls and internal operations

**Tasks**:
1. Instrument agent tool LLM calls with token tracking:
   - Capture input/output/cached token counts
   - Calculate estimated costs based on model pricing
   - Track finish reasons and errors
2. Instrument security framework checks
3. Add cache operation tracing
4. Implement sampling configuration
5. Add per-tool disable capability

**Success Criteria**:
- LLM token usage and costs observable for all agent tools
- Security overhead measurable
- Cache hit rate observable
- Agent tool performance and cost analysable
- Sampling reduces overhead appropriately

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

### Future Consideration: Client-Side Observability

The MCP observability architecture can be enhanced with a dual-path approach:

- **Server-side** (current implementation): Tool execution, latency, errors, LLM calls
- **Client-side** (future): Token usage aggregation across servers, context window optimisation, cross-server correlation

Client-side instrumentation would enable:
- MCP clients (Claude Desktop, etc.) to inject W3C Trace Context into requests
- Aggregated view of token usage across multiple MCP servers
- End-to-end visibility from agent decision to tool execution
- Cross-server workflow analysis

This remains out of scope for our server implementation but is noted for ecosystem-wide observability discussions.

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

### Phase 1: Core Infrastructure ✅ COMPLETED

- [x] Research and select OTEL SDK version (latest stable) - Using v1.38.0
- [x] Add OTEL dependencies to `go.mod`
- [x] Create `internal/telemetry/` package structure:
  - [x] `tracer.go` - Tracer provider initialisation
  - [x] `conventions.go` - MCP/GenAI semantic convention constants
  - [x] `sanitise.go` - Attribute sanitisation helpers
- [x] Define MCP semantic conventions in `conventions.go`
- [x] Implement tracer provider initialisation from environment
- [x] Implement graceful shutdown and flush on exit
- [x] Add noop tracer when OTEL disabled (zero overhead)
- [x] Implement session ID generation and context propagation
- [x] Implement session span creation and parent-child correlation
- [x] Create tool execution span wrapper in `main.go`
- [x] Add span attributes following MCP semantic conventions
- [x] Session ID generation implemented (used by stdio transport)
- [x] Write unit tests for tracer initialisation
- [x] Write unit tests for sanitisation helpers
- [x] Test with local OTLP collector (Jaeger) - ✅ Verified working, no warnings
- [x] Document semantic conventions in `docs/observability.md`
- [x] Document basic setup in `docs/observability.md`

**Implementation Notes:**
- Disabled tools are parsed even when tracing is disabled to support configuration testing
- Session IDs are generated using UUID v4 for uniqueness
- Tool execution spans automatically capture sanitised arguments (with size limits)
- Graceful shutdown ensures pending traces are flushed before exit
- All OTEL SDK imports use the new `noop` package instead of deprecated functions
- **Session correlation**: Session span is ended immediately after creation to ensure it exports before tool spans, eliminating "invalid parent span IDs" warnings
- **Parent-child hierarchy**: Tool spans use stored session span context as parent, providing proper trace hierarchy in backends

### Phase 2: HTTP Client Instrumentation ✅ COMPLETED

- [x] Identify all HTTP clients in codebase (internetsearch, packagedocs, github, etc.)
- [x] Create wrapper utility for HTTP client instrumentation (`internal/telemetry/http.go`)
- [x] Apply `otelhttp.NewTransport()` to all HTTP clients via utility functions
- [x] URL sanitisation already implemented in Phase 1 (`sanitise.go`)
- [x] Update `internal/utils/httpclient/proxy.go` to auto-wrap HTTP clients with OTEL
- [x] Update proxy upstream transports (SSE and HTTP) with OTEL instrumentation
- [x] Test with all existing tool tests (passed ✅)
- [x] Verify build succeeds with new instrumentation

**Implementation Notes:**
- Created `WrapHTTPClient()` and `WrapHTTPTransport()` helpers in `internal/telemetry/http.go`
- All tools using `httpclient.NewHTTPClientWithProxy()` automatically get OTEL instrumentation
- Proxy upstream transports (SSE/HTTP) now instrumented for tracing upstream MCP servers
- HTTP instrumentation is zero-overhead when tracing is disabled (noop transport)
- Standard OTEL HTTP semantic conventions are automatically applied by otelhttp package

### Phase 3: Advanced Instrumentation ✅ COMPLETED (Security Framework Only)

- [x] Implement security framework span creation (`AnalyseContentWithContext` method)
- [x] Add opt-in security tracing via `MCP_TRACING_SECURITY_ENABLED` environment variable
- [x] Capture security check attributes (content size, action, risk factors)
- [x] Test security instrumentation with existing tests
- [x] Verify build succeeds

**Implementation Notes:**
- Security tracing is opt-in via `MCP_TRACING_SECURITY_ENABLED=true` to avoid overhead
- New `AnalyseContentWithContext()` method wraps existing `AnalyseContent()` with tracing
- Captures content size, security action (allow/warn/block), and first risk factor
- Existing API remains unchanged to avoid breaking changes
- Zero overhead when security tracing is not enabled

**Deferred Items:**
- ❌ LLM token tracking - Not implemented (requires specific agent tool knowledge and integration)
- ❌ Cache operation tracing - Not implemented (cache is simple sync.Map, minimal value)
- ✅ Per-tool disable capability - Already implemented in Phase 1
- ✅ Sampling configuration - Already implemented in Phase 1

### Phase 4: W3C Trace Context (HTTP Transport Only) ✅ COMPLETED

- [x] Implement W3C Trace Context extraction from HTTP headers
- [x] Propagate trace context to tool execution context
- [x] Add trace context to outbound HTTP requests (via otelhttp in Phase 2)
- [x] Support Baggage header (via composite propagator)
- [x] Test multi-hop tool workflows (all tests passed)
- [x] Verify build succeeds
- [ ] Document trace context propagation - Deferred to Phase 5
- [ ] Add examples for distributed tracing scenarios - Deferred to Phase 5

**Implementation Notes:**
- Added `GetTextMapPropagator()` helper in `internal/telemetry/tracer.go` to access global propagator
- Created `extractTraceContext()` helper in `main.go` for extracting W3C Trace Context from HTTP headers
- Integrated trace context extraction into both `createAuthMiddleware()` and `createOAuthMiddleware()`
- W3C propagation was already configured in Phase 1 using `propagation.NewCompositeTextMapPropagator()` with TraceContext and Baggage
- Outbound HTTP requests automatically propagate trace context via `otelhttp.NewTransport()` from Phase 2
- Zero overhead when tracing is disabled (early return in `extractTraceContext()`)
- All tests passing, build successful

### Phase 5: Documentation & Examples ✅ COMPLETED

- [x] Write comprehensive `docs/observability.md`:
  - [x] Document MCP semantic conventions
  - [x] Document session correlation patterns
  - [x] Document all environment variables (standard OTEL + custom)
  - [x] Add troubleshooting section (common issues)
  - [x] Document performance impact and sampling
  - [x] Add security and privacy best practices
  - [x] Document what data is never traced
  - [x] Explain sanitisation approach
  - [x] Document W3C Trace Context propagation
  - [x] Add distributed tracing examples
- [x] Create `docs/examples/jaeger-compose.yml` with health checks
- [x] Create `docs/examples/tempo-compose.yml` with Grafana UI
- [x] Create `docs/examples/tempo-config.yaml` (Tempo configuration)
- [x] Create `docs/examples/grafana-datasources.yaml` (Grafana datasource setup)
- [ ] Update main `README.md` with observability section - Deferred (not required for core functionality)

**Implementation Notes:**
- Created observability.md with quick start, configuration, troubleshooting, and examples
- Documented all MCP semantic conventions with tables for easy reference
- Included practical examples for distributed tracing and multi-tool workflows
- Created Jaeger docker-compose example (simplest option for local development)
- Created Grafana Tempo example with pre-configured Grafana UI
- All examples include health checks and proper configuration
- Documented privacy/security features (automatic sanitisation)
- Documented performance impact and sampling strategies
- LLM token tracking and Langfuse integration noted as deferred (requires agent tool integration)

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

## Conclusion

OpenTelemetry integration provides significant value for production deployments and complex workflows whilst maintaining mcp-devtools' core philosophy of simplicity and performance. By making it optional and following OTEL standards, users gain powerful observability without added complexity for simple use cases.

The phased approach allows incremental delivery of value, starting with basic tool tracing and progressively adding advanced instrumentation. The use of standard OTEL SDK ensures compatibility with the entire observability ecosystem.

**Key Enhancements in v1.1**:
- Standardised MCP semantic conventions for ecosystem interoperability
- LLM token usage tracking for cost optimisation (critical for production deployments)
- Session-based correlation for understanding multi-tool agent workflows
- Upgraded priorities reflecting real-world MCP operational needs

These enhancements align mcp-devtools with emerging MCP observability best practices whilst maintaining our commitment to optional, zero-overhead instrumentation.

**Next Steps**: Core implementation complete (Phases 1-5). Focus on remaining testing and polish (Phases 6-7).

---

## Remaining Work Summary

### ✅ Completed (Phases 1-5)
- Core OTEL infrastructure with zero-overhead when disabled
- Session-based correlation with parent-child span hierarchy
- Tool execution tracing with MCP semantic conventions
- HTTP client auto-instrumentation for external API calls
- Security framework tracing (opt-in)
- W3C Trace Context propagation for distributed tracing
- Comprehensive documentation with setup guides and examples
- Docker compose examples for Jaeger and Grafana Tempo

### ⏳ Remaining Work

#### Phase 3 Deferred Items (Optional Advanced Features)
- **LLM Token Tracking**: Requires integration with specific agent tool packages (Claude, Kiro, Gemini)
  - Track input/output/cached token counts
  - Calculate estimated costs
  - Capture model parameters and finish reasons
  - **Status**: Deferred - requires agent tool implementation knowledge

- **Cache Operation Tracing**: Tracing sync.Map operations
  - **Status**: Deferred - minimal value, cache is simple

#### Phase 6: Testing & Quality Assurance (Not Started)
- Unit tests for all instrumentation code
- Integration tests with real OTLP collector
- Load tests for HTTP transport with tracing
- Memory leak testing
- Cross-platform testing
- Performance benchmarks

#### Phase 7: Polish & Release Preparation (Not Started)
- Security code review
- Update CHANGELOG.md
- Add observability section to main README
- GitHub issue templates for tracing problems
- Release tagging and announcement

### 🎯 Priority Recommendations

**High Priority** (Production-Ready):
1. Phase 6 testing (especially integration and load tests)
2. Phase 7 release preparation (CHANGELOG, README updates)

**Medium Priority** (Enhanced Observability):
3. LLM token tracking (requires agent tool integration)

**Low Priority** (Nice to Have):
4. Cache operation tracing

package telemetry

// MCP Semantic Conventions for OpenTelemetry tracing
// These attribute names follow the MCP observability best practices and align with
// OpenTelemetry semantic conventions for ecosystem-wide interoperability.

const (
	// MCP Tool attributes - Tool execution specific
	AttrMCPToolName     = "mcp.tool.name"           // Tool identifier (e.g., "internet_search")
	AttrMCPToolCategory = "mcp.tool.category"       // Tool category (e.g., "search_discovery")
	AttrMCPToolSuccess  = "mcp.tool.result.success" // Execution success (boolean)
	AttrMCPToolError    = "mcp.tool.result.error"   // Error message if failed (string)

	// MCP Session attributes - Session tracking and correlation
	AttrMCPSessionID         = "mcp.session.id"          // Unique session identifier
	AttrMCPSessionTimeout    = "mcp.session.timeout"     // Session timeout in seconds
	AttrMCPSessionDuration   = "mcp.session.duration"    // Session duration in seconds (on termination)
	AttrMCPSessionToolCount  = "mcp.session.tool_count"  // Number of tools executed in session
	AttrMCPSessionErrorCount = "mcp.session.error_count" // Number of failed tool executions
	AttrMCPTransport         = "mcp.transport"           // Transport type (stdio/http/sse)

	// LLM/GenAI attributes - LLM invocation tracking
	AttrLLMSystem        = "llm.system"              // LLM provider (e.g., "anthropic", "openai")
	AttrLLMModel         = "llm.model"               // Model identifier (e.g., "claude-sonnet-4-5")
	AttrLLMRequestType   = "llm.request.type"        // Request type (e.g., "chat", "completion")
	AttrLLMInputTokens   = "llm.usage.input_tokens"  // Input tokens consumed
	AttrLLMOutputTokens  = "llm.usage.output_tokens" // Output tokens generated
	AttrLLMCachedTokens  = "llm.usage.cached_tokens" // Cached tokens used
	AttrLLMTotalTokens   = "llm.usage.total_tokens"  // Total tokens (input + output)
	AttrLLMCostEstimated = "llm.cost.estimated"      // Estimated cost in USD
	AttrLLMTemperature   = "llm.temperature"         // Temperature setting
	AttrLLMMaxTokens     = "llm.max_tokens"          // Max tokens limit
	AttrLLMFinishReason  = "llm.finish_reason"       // Completion reason (e.g., "stop", "length")

	// Cache attributes - Cache operation tracking
	AttrCacheHit       = "cache.hit"       // Cache hit (boolean)
	AttrCacheKey       = "cache.key"       // Cache key (sanitised)
	AttrCacheOperation = "cache.operation" // Cache operation (get/set/delete)

	// Security attributes - Security framework tracking
	AttrSecurityRuleMatched = "security.rule.matched" // Rule name that matched
	AttrSecurityAction      = "security.action"       // Security action (allow/block/warn)
	AttrSecurityContentSize = "security.content.size" // Size of content scanned
)

// Span names for consistent span naming across the application
const (
	SpanNameSession       = "mcp.session"      // Session span (parent for all tool calls)
	SpanNameToolExecute   = "mcp.tool.execute" // Tool execution span
	SpanNameHTTPClient    = "http.client"      // HTTP client request span
	SpanNameSecurityCheck = "security.check"   // Security framework check span
	SpanNameCacheOp       = "cache"            // Cache operation span
	SpanNameLLMExecute    = "llm.execute"      // LLM invocation span
)

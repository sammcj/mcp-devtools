# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build and Run
- `make build` - Build the server binary to `bin/mcp-devtools`
- `make run` - Build and run server with stdio transport (default)
- `make run-http` - Build and run server with HTTP transport on port 18080

### Testing
- `make test` - Run all tests (includes external API tests, ~10s)
- `make test-fast` - Run tests without external API calls (~7s)
- `make test-verbose` - Show detailed per-test timing
- `make test-slow` - Show slowest tests sorted by duration
- `go test -short -v ./tests/...` - Run specific test suites

### Code Quality
- `make lint` - Run linting, formatting and modernisation checks

### Dependencies
- `make deps` - Install dependencies
- `make update-deps` - Update dependencies
- `go mod tidy` - Clean up go.mod

## Architecture

This is a modular MCP (Model Context Protocol) server built in Go that provides developer tools through a plugin-like architecture.

Please read the `README.md` for more information, and `docs/creating-new-tools.md` for details on how to create new tools.

### Core Components

1. **Tool Registry** (`internal/registry/`) - Central registry that manages tool registration and discovery
2. **Tool Interface** (`internal/tools/tools.go`) - Defines the interface all tools must implement
3. **Main Server** (`main.go`) - MCP server that supports multiple transports (stdio, SSE, HTTP)

### Tool Categories

Tools are organised into categories under `internal/tools/`, e.g:
- `internetsearch/` - Internet Search API integrations (web, image, news, video, local)
- `packageversions/` - Package version checking across ecosystems (npm, python, go, java, swift, docker, github-actions, bedrock, rust)
- `shadcnui/` - shadcn/ui component information and examples
- `think/` - Structured reasoning tool for AI agents
- `webfetch/` - Web content fetching and conversion to markdown
- `utils/` - Shared utilities for tools (e.g. proxy)
- etc..

### Adding New Tools

1. Create package under `internal/tools/[category]/[toolname]/`
2. Implement the `tools.Tool` interface with `Definition()` and `Execute()` methods
3. Register tool in `init()` function using `registry.Register(&YourTool{})`
4. Import the package in `main.go` to trigger registration

Tools automatically get:
- Shared cache (`sync.Map`)
- Logger (`logrus.Logger`)
- Context and parsed arguments

### Transport Support

The server supports three transport modes:
- **stdio** (default) - Standard input/output for MCP clients
- **http** - Streamable HTTP with optional authentication, optional upgrade to SSE if needed
- **sse** - Legacy Server-Sent Events for web clients (deprecated in favour of streamable HTTP), will be removed in future versions

## Important Files

- `main.go` - Server entry point and transport configuration
- `internal/registry/registry.go` - Tool registry implementation
- `internal/tools/tools.go` - Tool interface definition
- `Makefile` - Build and development commands
- `go.mod` - Go module dependencies

## Testing Strategy

Tests are organised in `tests/` directory:
- `testutils/` - Test helpers and mocks
- `tools/` - Tool-specific tests
- `unit/` - Unit tests for core components

Tests should not depend on external APIs or services unless cleared by the user as necessary.

Use `make test-fast` for development to avoid external API calls.

## Build System

Uses Go modules with version information injected at build time through ldflags in the Makefile. The binary includes version, commit hash, and build date.

## Tool Development Pattern

All tools follow this pattern:
1. Define struct implementing `tools.Tool`
2. Register in `init()` with `registry.Register()`
3. Implement `Definition()` for MCP tool schema
4. Implement `Execute()` for tool logic
5. Use shared logger and cache for consistency

## New Tool Registration Checklist

**CRITICAL**: Before registering a new tool, verify the following:

### Default Enablement Decision
- **By default, ALL new tools are DISABLED** - this is the secure default
- Only add tool to `defaultTools` list in `enabledByDefault()` (registry.go) if the user has explicitly stated it should be enabled by default
- **If tool is NOT added to defaultTools**: It will require `ENABLE_ADDITIONAL_TOOLS` to use (this is correct for most tools)
- Tests enable the tool via `ENABLE_ADDITIONAL_TOOLS` if not in defaultTools list
- When in Doubt - **Leave the tool disabled by default.** It's safer to require explicit enablement than to accidentally expose destructive functionality.

## General Guidelines

- CRITICAL: Ensure that when running in stdio mode that we NEVER log to stdout or stderr, as this will break the MCP protocol.
- Any tools we create must work on both macOS and Linux unless the user states otherwise (only add windows workarounds if they're easy to implement, don't reduce functionality for macOS/Linux and are low maintenance).
- When testing the docprocessing tool, unless otherwise instructed always call it with "clear_file_cache": true and do not enable return_inline_only
- If you're wanting to call a tool you've just made changes to directly (rather than using the command line approach), you have to let the user know to restart the conversation otherwise you'll only have access to the old version of the tool functions directly.
- When adding new tools ensure they are registered in the list of available tools in the server (within their init function), ensure they have a basic unit test, and that they have docs/tools/<toolname>.md with concise, clear information about the tool and that they're mentioned in the main README.md and docs/tools/overview.md.
- Always use British English spelling, we are not American.
- Follow the principle of least privileged security.
- Tool responses should be limited to only include information that is actually useful, there's no point in returning the information an agent provides to call the tool back to them, or any generic information or null / empty fields - these just waste tokens.
- Use 0600 and 0700 permissions for files and directories respectively, unless otherwise specified avoid using 0644 and 0755.
- Unit tests for tools should be located within the tests/tools/ directory, and should be named <toolname>_test.go.
- We should be mindful of the risks of code injection and other security risks when parsing any information from external sources.
- On occasion the user may ask you to build a new tool and provide reference code or information in a provided directory such as `tmp_repo_clones/<dirname>` unless specified otherwise this should only be used for reference and learning purposes, we don't ever want to use code that directory as part of the project's codebase.
- After making changes and performing a build if you need to test the MCP server with the updated changes you MUST either test it from the command line - or STOP and ask the user to restart the MCP client otherwise you won't pick up the latest changes
- When creating new MCP tools make sure descriptions are clear and concise as they are what is used as hints to the AI coding agent using the tool, you should also make good use of MCP's annotations.
- The mcp-go package documentation contains useful examples of using the package which you can lookup when asked to implement specific MCP features https://mcp-go.dev/servers/tools

---

## MCP Development Best Practices

**Build for Workflows, Not Just API Endpoints:**
- Don't simply wrap existing API endpoints - build thoughtful, high-impact workflow tools
- Consolidate related operations (e.g., `schedule_event` that both checks availability and creates event)
- Focus on tools that enable complete tasks, not just individual API calls
- Consider what workflows agents actually need to accomplish

**Optimise for Limited Context:**
- Agents have constrained context windows - make every token count
- Return high-signal information, not exhaustive data dumps
- Provide "concise" vs "detailed" response format options where applicable
- Default to human-readable identifiers over technical codes (names over IDs)
- Consider the agent's context budget as a scarce resource

**Design Actionable Error Messages:**
- Error messages should guide agents toward correct usage patterns
- Suggest specific next steps: "Try using filter='active_only' to reduce results"
- Make errors educational, not just diagnostic
- Help agents learn proper tool usage through clear feedback

**Follow Natural Task Subdivisions:**
- Tool names should reflect how humans think about tasks
- Group related tools with consistent prefixes for discoverability
- Design tools around natural workflows, not just API structure

**Use Evaluation-Driven Development:**
- Create realistic evaluation scenarios early
- Let agent feedback drive tool improvements
- Prototype quickly and iterate based on actual agent performance

To ensure quality, review the code for:
- **DRY Principle**: No duplicated code between tools
- **Composability**: Shared logic extracted into functions
- **Consistency**: Similar operations return similar formats
- **Error Handling**: All external calls have error handling
- **Documentation**: Every tool has comprehensive docstrings/descriptions

---

## Improving Tools with Tool Error Logs

If the user asks you to look at the tool call error logs, they can be found in `~/.mcp-devtools/logs/tool-errors.log` (assuming the user has enabled tool error logging), looking at recent tool calling failures can be useful for improving tools that may be failing often. If looking at the tool error logs, ensure you look for the relevant date/time range when the user was experiencing issues.

---

## Quick Debugging Tips

- You can debug the tool by running it in debug mode interactively, e.g. `rm -f debug.log; pkill -f "mcp-devtools.*" ; echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "fetch_url", "arguments": {"url": "https://go.dev", "max_length": 500, "raw": false}}}' | ./bin/mcp-devtools stdio`, or `BRAVE_API_KEY="ask the user if you need this" ./bin/mcp-devtools stdio <<< '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "brave_search", "arguments": {"type": "web", "query": "cat facts", "count": 1}}}'`

### Lint Github Actions
```bash
actionlint
```

## Go Modernisation Rules for AI Coding Agents

CRITICAL: Follow these rules when writing Go code to avoid outdated patterns that `modernize` would flag:

### Types and Interfaces
- Use `any` instead of `interface{}`
- Use `comparable` for type constraints when appropriate

### String Operations
- Use `strings.CutPrefix(s, prefix)` instead of `if strings.HasPrefix(s, prefix) { s = strings.TrimPrefix(s, prefix) }`
- Use `strings.SplitSeq()` and `strings.FieldsSeq()` in range loops instead of `strings.Split()` and `strings.Fields()`

### Loops and Control Flow
- Use `for range n` instead of `for i := 0; i < n; i++` when index isn't used
- Use `min(a, b)` and `max(a, b)` instead of if/else conditionals

### Slices and Maps
- Use `slices.Contains(slice, element)` instead of manual loops for searching
- Use `slices.Sort(s)` instead of `sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })`
- Use `maps.Copy(dst, src)` instead of manual `for k, v := range src { dst[k] = v }` loops

### Testing
- Use `t.Context()` instead of `context.WithCancel()` in tests

### Formatting
- Use `fmt.Appendf(nil, format, args...)` instead of `[]byte(fmt.Sprintf(format, args...))`

## OpenTelemetry Tracing Patterns

**Session span parent-child relationships**: Session spans in `internal/telemetry/tracer.go` must be ended immediately followed by `ForceFlush()` to ensure they export to the backend before child tool spans. Without force flush, the OTEL batch processor exports asynchronously, causing child spans to arrive before their parent, resulting in "invalid parent span IDs" warnings in Jaeger.

```go
sessionSpan.End()
// CRITICAL: Force flush ensures parent exports before children
if tp != nil {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    _ = tp.ForceFlush(ctx)
}
```

Tool spans inherit trace context via W3C Trace Context propagation (inject→extract pattern) in `StartToolSpan()`.

## IMPORTANT

- YOU MUST ALWAYS run `make lint && make test && make build` etc... to build the project rather than gofmt, go build or test directly, and you MUST always do this before stating you've completed your changes!
- CRITICAL: If the serena tool is available to you, you must use serena for your semantic code retrieval and editing tools

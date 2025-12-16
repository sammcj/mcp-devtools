# GitHub Copilot Instructions for MCP DevTools

## Project Overview

**MCP DevTools** is a single, high-performance MCP (Model Context Protocol) server written in Go that replaces many Node.js and Python-based MCP servers with one efficient binary. It provides access to essential developer tools through a unified, modular interface that can be easily extended with new tools.

**Key Features:**
- Single binary solution replacing multiple resource-heavy servers
- 16+ essential developer tools in one package
- Built in Go for speed, efficiency, and minimal memory footprint
- Fast startup and response times
- Modular tool registry architecture allowing easy addition of new tools
- Supports multiple transports: stdio (default) and streamable HTTP

## Development Setup

### Building the Project

```bash
# Build the server
make build

# The binary will be created at: bin/mcp-devtools
```

### Running the Server

```bash
# Run with stdio transport (default)
make run

# Run with HTTP transport
make run-http
```

### Testing

```bash
# Run all tests (includes external API integration tests, ~10s)
make test

# Run fast tests (skips external dependencies, ~7s)
make test-fast

# Run tests with detailed timing
make test-verbose
```

### Linting and Code Quality

```bash
# Format code
make fmt

# Run linters and modernisation checks
make lint
# This runs: gofmt, golangci-lint, and gopls modernize
```

### Dependencies

```bash
# Install Go dependencies
make deps

# Install all dependencies (Go + Python for document processing)
make install-all
```

## Project Structure

```
mcp-devtools/
├── internal/
│   ├── tools/           # All tool implementations
│   ├── registry/        # Tool registration system
│   ├── security/        # Security framework for file/network operations
│   ├── handlers/        # MCP protocol handlers
│   ├── config/          # Configuration management
│   ├── oauth/           # OAuth functionality
│   ├── cache/           # Caching utilities
│   ├── utils/           # Utility functions
│   └── imports/         # Import management
├── tests/
│   ├── tools/           # Unit tests for tools (REQUIRED for all tools)
│   ├── benchmarks/      # Performance and token cost tests
│   ├── oauth/           # OAuth tests
│   └── unit/            # Unit tests for internal packages
├── docs/
│   └── tools/           # Tool documentation (REQUIRED when adding tools)
├── main.go              # Entry point - import new tools here
├── Makefile             # Build, test, and development commands
└── mcp.json             # MCP server configuration
```

## Contribution Guidelines

### Before Committing

1. **Format your code:** `make fmt`
2. **Run linters:** `make lint` (must pass without errors)
3. **Run tests:** `make test-fast` (must pass all tests)
4. **Build successfully:** `make build` (must compile without errors)

### Code Standards

- Follow Go best practices and idiomatic patterns
- Use British English spelling throughout code and documentation
- No marketing terms like "comprehensive" or "production-grade"
- Focus on clear, concise, actionable technical guidance
- Keep responses token-efficient (avoid returning unnecessary data)

### Adding New Tools

1. Create package under `internal/tools/[category]/[toolname]/`
2. Implement `tools.Tool` interface with `Definition()` and `Execute()` methods
3. Register tool in `init()` function using `registry.Register(&YourTool{})`
4. Import the package in `internal/imports/tools.go` (NOT in `main.go`)
5. Add unit tests in `tests/tools/` directory
6. Add documentation in `docs/tools/` with clear, concise information
7. Update `docs/tools/overview.md`
8. Integrate with security framework if accessing files/URLs
9. Verify token cost with `ENABLE_ADDITIONAL_TOOLS=your_tool_name make benchmark-tokens`

**Important**: Do NOT add tool imports directly to `main.go`. Use the imports registry system in `internal/imports/tools.go` instead.

## Architecture & Structure

This is a modular MCP (Model Context Protocol) server written in Go with a tool registry architecture. Each tool implements the `tools.Tool` interface and registers itself through `internal/registry/`. The main server supports multiple transports (stdio, streamable HTTP).

## ⚠️ CRITICAL: stdio Mode Logging Violations

**MOST IMPORTANT CHECK IN EVERY REVIEW:**

When the server runs in stdio mode (default transport), ANY output to stdout/stderr will break the MCP protocol and cause catastrophic failures. This is the #1 bug to prevent.

### What to Check in EVERY Pull Request:
1. **No direct stdout/stderr writes:**
   - ❌ NEVER: `fmt.Println()`, `fmt.Printf()`, `log.Println()`, `fmt.Fprintf(os.Stdout, ...)`
   - ❌ NEVER: `os.Stdout.Write()`, `os.Stderr.Write()`, `print()`, `println()`
   - ✅ ALWAYS: Use `logger.Info()`, `logger.Debug()`, `logger.Error()`, etc.

2. **No external commands that write to stdout/stderr in stdio mode:**
   - Check all `exec.Command()` calls
   - Ensure stdout/stderr are captured or redirected when server is in stdio mode
   - Consider transport mode when executing external commands

3. **Check third-party libraries:**
   - Some libraries may write to stdout/stderr unexpectedly
   - Review library documentation before adding dependencies
   - Test new dependencies in stdio mode

4. **Verify error handling:**
   - Errors should go to logger, not stderr
   - Stack traces must use logger, not panic/fatal which write to stderr
   - No debug prints left in production code

The only exception is in tests, tests are allowed to write to stdout/stderr.

### Why This Matters:
- stdio transport uses stdin/stdout for MCP protocol messages (JSON-RPC)
- Any extra output corrupts the protocol stream
- Results in "unexpected end of JSON input" and protocol failures
- Very difficult to debug once deployed

**ACTION REQUIRED:** Flag ANY code that might write to stdout/stderr in your review comments with HIGH SEVERITY.

## Critical Review Areas

### Go Code Standards
- Follow Go best practices and idiomatic patterns
- Use proper error handling with wrapped errors
- Implement context cancellation correctly
- Ensure goroutine safety and proper synchronisation
- Use appropriate logging with logrus logger
- Follow the project's naming conventions

### Tool Development Requirements
- All tools MUST implement the `tools.Tool` interface
- Tools MUST register via `registry.Register()` in their `init()` function
- Tools MUST NOT log to stdout / stderr directly (use `logrus` instead)
- Execute methods MUST handle context cancellation
- Tools should use shared cache (`sync.Map`) for performance
- Import new tools in `main.go` to trigger registration
- Tool responses should be limited to only include information that is actually useful, there's no point in returning the information an agent provides to call the tool back to them, or any generic information or null / empty fields - these just waste tokens.

### Security Integration (CRITICAL)
- ALL tools accessing files or fetching content from URLs MUST integrate with the security framework
  - Use `security.NewOperations("tool_name")` for HTTP/file operations
- Handle `SecurityError` types properly in error responses
- Check for file access permissions and domain restrictions
- Any new files should be `0600` and directories `0700` by default to prevent unauthorised access

### MCP Protocol Compliance
- NEVER log to stdout / stderr in stdio mode (breaks MCP protocol, use `logrus` instead)
- Use proper MCP response formats with `mcp.CallToolResult`
- Handle tool arguments validation correctly
- Implement proper JSON schema for tool parameters
- Follow MCP error handling patterns

### Testing Requirements
- Unit tests MUST be in `tests/tools/` directory
- Tests MUST NOT rely on external dependencies
- Test error conditions and edge cases
- Tests should be lightweight and fast

### Performance & Reliability
- Minimise external API calls and dependencies
- Implement proper caching strategies
- Handle rate limiting gracefully
- Use context timeouts for external requests
- Avoid blocking operations in tool execution
- Maintain compatibility with existing tool interfaces
- Consider backward compatibility for configuration changes

### Documentation Standards
- Tool documentation belongs in `docs/tools/`
- Update `docs/tools/overview.md` when adding tools
- Australian English spelling used throughout, No American English spelling used (unless it's a function or parameter to an upstream library)
- Provide clear examples and usage patterns
- Document security requirements and limitations
- Documentation should be concise, favouring clear technical information over verbosity

## Code Quality Checks

### stdio Mode Logging (Check FIRST, EVERY TIME)
- ❌ Scan entire diff for `fmt.Print`, `fmt.Println`, `fmt.Printf`, `log.Print`, `print`, `println`
- ❌ Check for `os.Stdout.Write`, `os.Stderr.Write`, `fmt.Fprintf(os.Stdout`, `fmt.Fprintf(os.Stderr`
- ❌ Review all `exec.Command()` calls - ensure stdout/stderr are captured
- ✅ Confirm all logging uses `logger.Info/Debug/Error/Warn()` methods
- ✅ Verify error paths don't use `panic()` or `log.Fatal()` (writes to stderr)

### General Code Quality
- Verify proper module imports and dependencies
- Check for hardcoded credentials or sensitive data
- Ensure proper resource cleanup (defer statements)
- Validate input parameters thoroughly
- Use appropriate data types and structures
- Follow consistent error message formatting

## Configuration & Environment
- Environment variables should have sensible defaults
- Configuration should be documented in README
- Support both development and production modes
- Handle missing optional dependencies gracefully

## Special Attention Areas

- Security framework integration for new tools
- Transport mode compatibility (stdio/streamable HTTP)
- Tool registry and discovery mechanisms
- Memory management and potential leaks

## Tool Registration Checklist

**CRITICAL**: Before registering a new tool, verify the following:

### Default Enablement Decision
- **By default, ALL new tools are DISABLED** - this is the secure default
- Only add tool to `defaultTools` list in `enabledByDefault()` (registry.go) if the user has explicitly stated it should be enabled by default
- **If tool is NOT added to defaultTools**: It will require `ENABLE_ADDITIONAL_TOOLS` to use (this is correct for most tools)
- Tests enable the tool via `ENABLE_ADDITIONAL_TOOLS` if not in defaultTools list
- When in Doubt - **Leave the tool disabled by default.** It's safer to require explicit enablement than to accidentally expose destructive functionality.

## Go Modernisation Rules

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

## Tool Development Pattern

All tools follow this pattern:
1. Define struct implementing `tools.Tool`
2. Register in `init()` with `registry.Register()`
3. Implement `Definition()` for MCP tool schema
4. Implement `Execute()` for tool logic
5. Use shared logger and cache for consistency

### Tool Descriptions Best Practices
- Tool descriptions should focus on WHAT the tool does
- Tool descriptions should be action-oriented and concise
- ✅ Good: "Returns source code structure by stripping function/method bodies whilst preserving signatures, types, and declarations."
- ❌ Poor: "Transform source code by removing implementation details while preserving structure. Achieves 60-80% token reduction for optimising AI context windows"
- The first describes what the tool does; the second explains why it's useful (which bloats the context unnecessarily)
- Tool descriptions should aim to be under 200 characters where possible; save detailed usage information for Extended Help

### Tool Response Best Practices
- Tool responses should be limited to only include information that is actually useful
- Don't return the information an agent provides to call the tool back to them
- Avoid any generic information or null / empty fields - these just waste tokens

## General Guidelines

- Do not use marketing terms such as 'comprehensive' or 'production-grade' in documentation or code comments.
- Focus on clear, concise actionable technical guidance.
- Always use British English spelling, we are not American.
- Follow the principle of least privileged security.
- Use 0600 and 0700 permissions for files and directories respectively, unless otherwise specified avoid using 0644 and 0755.
- All tools should work on both macOS and Linux unless otherwise specified (we do not need to support Windows).
- Rather than creating lots of tools for one purpose / provider, instead favour creating a single tool with multiple functions and parameters.
- When creating new MCP tools make sure descriptions are clear and concise as they are what is used as hints to the AI coding agent using the tool, you should also make good use of MCP's annotations.
- The mcp-go package documentation contains useful examples of using the package which you can lookup when asked to implement specific MCP features https://mcp-go.dev/servers/tools

## Important Warnings and Reminders

- **CRITICAL**: Ensure that when running in stdio mode that we NEVER log to stdout or stderr, as this will break the MCP protocol.
- When testing the docprocessing tool, unless otherwise instructed always call it with "clear_file_cache": true and do not enable return_inline_only
- If you're wanting to call a tool you've just made changes to directly (rather than using the command line approach), you have to let the user know to restart the conversation otherwise you'll only have access to the old version of the tool functions directly.
- When adding new tools ensure they are registered in the list of available tools in the server (within their init function), ensure they have a basic unit test, and that they have docs/tools/<toolname>.md with concise, clear information about the tool and that they're mentioned in the main README.md and docs/tools/overview.md.
- We should be mindful of the risks of code injection and other security risks when parsing any information from external sources.
- On occasion the user may ask you to build a new tool and provide reference code or information in a provided directory such as `tmp_repo_clones/<dirname>` unless specified otherwise this should only be used for reference and learning purposes, we don't ever want to use code that directory as part of the project's codebase.
- After making changes and performing a build if you need to test the MCP server with the updated changes you MUST either test it from the command line - or STOP and ask the user to restart the MCP client otherwise you won't pick up the latest changes
- **YOU MUST ALWAYS run `make lint && make test && make build` etc... to build the project rather than gofmt, go build or test directly, and you MUST always do this before stating you've completed your changes!**

## Quick Debugging Tips

You can debug the tool by running it in debug mode interactively, e.g.:

```bash
rm -f debug.log; pkill -f "mcp-devtools.*" ; echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "fetch_url", "arguments": {"url": "https://go.dev", "max_length": 500, "raw": false}}}' | ./bin/mcp-devtools stdio
```

Or with API key:

```bash
BRAVE_API_KEY="ask the user if you need this" ./bin/mcp-devtools stdio <<< '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "brave_search", "arguments": {"type": "web", "query": "cat facts", "count": 1}}}'
```

### Lint Github Actions
```bash
actionlint
```

## Review Checklist for Every PR

Before approving any pull request, verify:

1. [ ] **[CRITICAL]** No stdout/stderr writes in stdio mode (see section above)
2. [ ] All new tools implement `tools.Tool` interface correctly
3. [ ] Security framework integration for file/network operations
4. [ ] Documentation updated in `docs/tools/` if required
5. [ ] Error handling uses wrapped errors (`fmt.Errorf` with `%w`)
6. [ ] Context cancellation handled properly
7. [ ] Resource cleanup with defer statements
8. [ ] British English spelling used throughout, No American English spelling used (unless it's a function or parameter to an upstream library)
9. [ ] New tools are disabled by default unless explicitly approved
10. [ ] Tool imports added to `internal/imports/tools.go` (NOT `main.go`)
11. [ ] Go modernisation rules followed (use `any`, `strings.CutPrefix`, etc.)
12. [ ] Tool descriptions are concise and action-oriented (<200 chars where possible)

If you are re-reviewing a PR you've reviewed in the past and your previous comments / suggestions have been addressed or are no longer valid please resolve those previous review comments to keep the review history clean and easy to follow.

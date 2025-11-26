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

1. Implement `tools.Tool` interface in `internal/tools/`
2. Register tool via `registry.Register()` in tool's `init()` function
3. Import tool in `main.go` to trigger registration
4. Add unit tests in `tests/tools/`
5. Add documentation in `docs/tools/`
6. Update `docs/tools/overview.md`
7. Integrate with security framework if accessing files/URLs

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

## General Guidelines

- Do not use marketing terms such as 'comprehensive' or 'production-grade' in documentation or code comments.
- Focus on clear, concise actionable technical guidance.

## Review Checklist for Every PR

Before approving any pull request, verify:

1. [ ] **[CRITICAL]** No stdout/stderr writes in stdio mode (see section above)
2. [ ] All new tools implement `tools.Tool` interface correctly
3. [ ] Security framework integration for file/network operations
4. [ ] Documentation updated in `docs/tools/` if required
5. [ ] Error handling uses wrapped errors (`fmt.Errorf` with `%w`)
6. [ ] Context cancellation handled properly
7. [ ] Resource cleanup with defer statements
8. [ ] Australian English spelling used throughout, No American English spelling used (unless it's a function or parameter to an upstream library)

If you are re-reviewing a PR you've reviewed in the past and your previous comments / suggestions have been addressed or are no longer valid please resolve those previous review comments to keep the review history clean and easy to follow.

# GitHub Copilot Code Review Instructions for MCP DevTools

## Architecture & Structure

This is a modular MCP (Model Context Protocol) server written in Go with a tool registry architecture. Each tool implements the `tools.Tool` interface and registers itself through `internal/registry/`. The main server supports multiple transports (stdio, HTTP, SSE).

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
- Tests should MUST NOT rely on external dependencies
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
- Use British English spelling throughout
- Provide clear examples and usage patterns
- Document security requirements and limitations
- Documentation should be concise, favouring clear technical information over verbosity

## Code Quality Checks

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
- Transport mode compatibility (stdio/HTTP/SSE)
- Tool registry and discovery mechanisms
- Memory management and potential leaks
- Cross-platform compatibility (macOS/Linux support only)

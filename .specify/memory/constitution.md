# MCP DevTools Tool Development Constitution

## Core Principles

### I. Interface Conformity
Every tool MUST implement the `tools.Tool` interface with `Definition()` and `Execute()` methods. Tools define their name, description, and parameters through the MCP framework. No tool shall deviate from this standard interface contract.

### II. Security-First (NON-NEGOTIABLE)
All tools accessing files or making HTTP requests MUST integrate with the security system. Security helper functions (`SafeHTTPGet`, `SafeFileRead`) are the preferred approach, providing 80-90% boilerplate reduction whilst guaranteeing content integrity. Security blocks take precedence over functionality.

### III. Registration & Discovery
Tools register themselves via `init()` functions with the central registry. Imports are managed through the imports registry (`internal/imports/tools.go`), never directly in `main.go`. Disabled-by-default tools require dual registration: enablement check in `Execute()` AND listing in `registry.requiresEnablement()`.

### IV. Test-First Development
All tools require fast, concise unit tests without external dependencies. Tests must verify core functionality, parameter parsing, and error handling. Integration patterns with other tools need explicit test coverage.

### V. Extended Help for Complexity
Complex tools implement the `ExtendedHelpProvider` interface, providing examples, common patterns, troubleshooting tips, and parameter details. Help content guides AI agents and less capable models towards effective tool usage.

### VI. Simplicity & Performance
Single tools with multiple parameters preferred over multiple single-purpose tools. Descriptions must be under 450 characters where possible. Tools must work on macOS and Linux (Windows support not required). No stdout/stderr logging in stdio mode.

## Implementation Standards

### Parameter Schema
Tools use MCP framework parameter types: String, Number, Boolean, Array, Object. Each parameter specifies: Required/Optional status, Description, Default values, Enum constraints, Properties for objects.

### Result Schema
Results return `*mcp.CallToolResult` objects containing structured data. JSON marshalling for complex results. Clear error messages for failures.

### Caching Protocol
Shared `sync.Map` cache available across tool executions. Cache keys must be namespaced to prevent collisions. Cache usage is optional but encouraged for expensive operations.

### Security Integration Checklist
**Helper Functions (Recommended):**
- Create Operations instance: `ops := security.NewOperations("tool_name")`
- Use `ops.SafeHTTPGet/Post()` for HTTP
- Use `ops.SafeFileRead/Write()` for files
- Handle `SecurityError` types
- Log warnings with security IDs
- Process exact content from responses

**Manual Integration (Alternative):**
- Call `CheckFileAccess()` before file operations
- Call `CheckDomainAccess()` before HTTP requests
- Call `AnalyseContent()` with appropriate `SourceContext`
- Handle Block/Warn/Allow actions appropriately

## Development Workflow

### Tool Creation Process
1. Create package under `internal/tools/[category]/[tool-name]/`
2. Implement Tool interface with proper security integration
3. Register in imports registry (`internal/imports/tools.go`)
4. Write comprehensive unit tests
5. Update `docs/tools/overview.md`
6. Implement ExtendedHelp if complexity warrants

### Quality Gates
- Security integration verified for file/network operations
- Unit tests MUST pass without external dependencies
- The total time for unit tests to pass MUST be under 5 seconds
- Tool descriptions concise and clear
- Registration properly configured
- Documentation updated

### Extended Help Criteria
Add extended help when tools have:
- Multiple parameter combinations with different behaviours
- Complex parameter structures (nested objects, specific formats)
- Integration patterns with other tools
- Common error conditions or edge cases
- Context-sensitive behaviour

## Governance

This constitution supersedes all ad-hoc tool development practices. All pull requests must verify compliance with these principles. Security violations result in immediate rejection. Complexity must be justified through extended help documentation. Tools violating the stdio logging restriction will break the MCP protocol and must be fixed immediately.

The security system provides graceful degradation when disabled but MUST be integrated for all applicable tools. Override capability exists through security IDs for blocked content, maintaining audit trails for all security events.

Documentation amendments require corresponding code updates. Breaking changes to the Tool interface require migration plans for all existing tools.

**Version**: 1.0.0 | **Ratified**: 2025-09-18 | **Last Amended**: 2025-09-18

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build and Run
- `make build` - Build the server binary to `bin/mcp-devtools`
- `make run` - Build and run server with stdio transport (default)
- `make run-http` - Build and run server with HTTP transport on port 18080

### Testing
- `make test` - Run all tests including external dependencies
- `go test -short -v ./tests/...` - Run specific test suites

### Code Quality
- `make lint` - Run linting and formatting with gofmt and golangci-lint

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

Tools are organized into categories under `internal/tools/`, e.g:
- `internetsearch/` - Internet Search API integrations (web, image, news, video, local)
- `packageversions/` - Package version checking across ecosystems (npm, python, go, java, swift, docker, github-actions, bedrock, rust)
- `shadcnui/` - shadcn/ui component information and examples
- `think/` - Structured reasoning tool for AI agents
- `webfetch/` - Web content fetching and conversion to markdown
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

## General Guidelines

- CRITICAL: Ensure that when running in stdio mode that we NEVER log to stdout or stderr, as this will break the MCP protocol.
- Any tools we create must work on both macOS and Linux unless the user states otherwise (we don't care about MS Windows).
- When testing the docprocessing tool, unless otherwise instructed always call it with "clear_file_cache": true and do not enable return_inline_only
- If you're wanting to call a tool you've just made changes to directly (rather than using the command line approach), you have to let the user know to restart the conversation otherwise you'll only have access to the old version of the tool functions directly.
- When adding new tools ensure they are registered in the list of available tools in the server (within their init function), ensure they have a basic unit test, and that they have docs/tools/<toolname>.md with concise, clear information about the tool and that they're mentioned in the main README.md and docs/tools/overview.md.
- Always use British English spelling, we are not American.
- Follow the principle of least privileged security.
- Use 0600 and 0700 permissions for files and directories respectively, unless otherwise specified avoid using 0644 and 0755.
- Unit tests for tools should be located within the tests/tools/ directory, and should be named <toolname>_test.go.
- We should be mindful of the risks of code injection and other security risks when parsing any information from external sources.
- On occasion the user may ask you to build a new tool and provide reference code or information in a provided directory such as `tmp_repo_clones/<dirname>` unless specified otherwise this should only be used for reference and learning purposes, we don't ever want to use code that directory as part of the project's codebase.
- When creating new MCP tools make sure descriptions are clear and concise as they are what is used as hints to the AI coding agent using the tool, you should also make good use of MCP's annotations.
- The mcp-go package documentation contains useful examples of using the package which you can lookup when asked to implement specific MCP features https://mcp-go.dev/servers/tools

### Quick Debugging Tips

- You can debug the tool by running it in debug mode interactively, e.g. `rm -f debug.log; pkill -f "mcp-devtools.*" ; echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "fetch_url", "arguments": {"url": "https://go.dev", "max_length": 500, "raw": false}}}' | ./bin/mcp-devtools stdio`, or `BRAVE_API_KEY="ask the user if you need this" ./bin/mcp-devtools stdio <<< '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "brave_search", "arguments": {"type": "web", "query": "cat facts", "count": 1}}}'`

### Lint Github Actions
```bash
actionlint
```

## IMPORTANT

- YOU MUST ALWAYS run `make lint && make test && make build` etc... to build the project rather than gofmt, go build or test directly, and you MUST always do this before stating you've completed your changes!
- CRITICAL: If the serena tool is available to you, you must use serena for your semantic code retrieval and editing tools

# CLI Mode

Run mcp-devtools tools directly from the command line without starting an MCP server. Tools execute in-process with no network overhead.

## Usage

```bash
# List available tools
mcp-devtools cli list

# Show help for a tool
mcp-devtools cli help <tool-name>

# Run a tool with JSON arguments
mcp-devtools cli run <tool-name> '{"key": "value"}'

# Run a tool with flag-style arguments
mcp-devtools cli run <tool-name> --key=value --other-key value

# Shortcut: run a tool without the 'run' subcommand (JSON args only)
mcp-devtools cli <tool-name> '{"key": "value"}'
```

## Output Formats

```bash
# Default: plain text (tool output printed to stdout)
mcp-devtools cli run calculator --expression="2+2"

# JSON: structured output for scripting
mcp-devtools cli --output=json run calculator --expression="2+2"

# JSON list of available tools
mcp-devtools cli --output=json list
```

## Argument Styles

### JSON Arguments

Pass a JSON object as a positional argument. Supports all parameter types including nested objects and arrays.

```bash
mcp-devtools cli run search-packages '{"ecosystem": "npm", "query": "react"}'
mcp-devtools cli run calculator '{"expressions": ["2+2", "3*3", "100/7"]}'
```

### Flag-style Arguments

Use `--key=value` or `--key value` syntax. Parameter names use kebab-case (e.g. `--max-length` maps to the `max_length` parameter).

```bash
mcp-devtools cli run calculator --expression="(100 / 3) + 7.5"
mcp-devtools cli run think --thought "Is this approach correct?"
mcp-devtools cli run fetch-url --url=https://go.dev --max-length=500
```

Boolean parameters can be passed as bare flags:

```bash
mcp-devtools cli run fetch-url --url=https://example.com --raw
```

Array parameters accept JSON arrays:

```bash
mcp-devtools cli run calculator --expressions='["2+2", "3*3"]'
```

### Mixed Arguments

Flags and JSON can be combined. Flag values take precedence over JSON values for the same key.

```bash
mcp-devtools cli run search-packages --ecosystem=npm '{"query": "react"}'
```

## Tool Enablement

CLI mode respects the same tool enablement rules as MCP mode:

- Tools in the default set are available without configuration
- Additional tools require `ENABLE_ADDITIONAL_TOOLS` environment variable
- `DISABLED_TOOLS` environment variable blocks specific tools

```bash
# Enable all tools for CLI use
ENABLE_ADDITIONAL_TOOLS=all mcp-devtools cli list

# Enable specific additional tools
ENABLE_ADDITIONAL_TOOLS=filesystem,github mcp-devtools cli list
```

## Scripting

CLI mode is designed to work in shell scripts and pipelines.

```bash
# Pipe JSON output to jq
mcp-devtools cli --output=json run calculator '{"expression": "2^10"}' | jq '.content[0].text'

# Use in a shell script
result=$(mcp-devtools cli run calculator --expression="365 * 24")
echo "Hours in a year: $result"
```

## How It Differs from MCP Mode

| Aspect    | MCP Mode               | CLI Mode                        |
| --------- | ---------------------- | ------------------------------- |
| Transport | stdio/HTTP/SSE         | Direct in-process               |
| Startup   | Full MCP handshake     | Instant                         |
| Output    | MCP protocol responses | Plain text or JSON              |
| Use case  | AI agent tool calls    | Human use, scripting, pipelines |
| Telemetry | Full tracing + metrics | None                            |

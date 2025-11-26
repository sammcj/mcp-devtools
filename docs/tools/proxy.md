# Proxy Tool

Connect to upstream MCP servers and expose their tools via mcp-devtools, enabling access to external MCP ecosystems with OAuth support, dynamic tool registration, and transparent request proxying.

Example use case: You have an upstream HTTP or SSE MCP server with OAuth (e.g., Atlassian's MCP server) - but your MCP client doesn't properly support HTTP/SSE or OAuth. By using the proxy tool, you can connect to the upstream server, authenticate via OAuth, and expose its tools as native mcp-devtools tools.

## Features

- **Dynamic Tool Registration**: Automatically fetches and registers tools from upstream MCP servers before the main server starts
- **OAuth 2.0/2.1 Support**: Full OAuth flow with PKCE, automatic browser opening, and secure token management
- **Multiple Transport Types**: Support for SSE, HTTP, and streamable HTTP transports
- **Tool Filtering**: Configure which upstream tools to expose or ignore using patterns
- **Transparent Proxying**: Tools appear as native mcp-devtools tools to clients
- **Token Persistence**: Securely stores OAuth tokens and client registration info for seamless reconnection
- **Aggregation**: Combine tools from multiple upstream servers into a single unified interface

_Note: The proxy tool does not utilise the `security` middleware, it provides tools proxied as-is from the configured upstream MCP server(s)._

## Configuration

The proxy tool is configured via the `PROXY_UPSTREAMS` environment variable, which accepts a JSON array of upstream server configurations.

### Basic Configuration

```json
{
  "ENABLE_ADDITIONAL_TOOLS": "proxy",
  "PROXY_UPSTREAMS": "[{\"name\": \"atlassian\", \"url\": \"https://mcp.atlassian.com/v1/sse\", \"transport\": \"http-first\"}]"
}
```

In this example when the MCP Server is started, it will connect to the Atlassian MCP server, open your browser to authenticate via OAuth, fetch the available tools, and register them as tools.

### Configuration Parameters

Each upstream server configuration supports these parameters:

- **`name`** (required): Unique identifier for the upstream server (e.g., "atlassian", "github-mcp")
- **`url`** (required): Server URL endpoint
- **`transport`** (optional): Transport protocol to use:
  - `http-first` (default): Try HTTP first, fall back to SSE if needed
  - `http`: Use streamable HTTP transport only
  - `sse`: Use Server-Sent Events transport only
- **`ignore_patterns`** (optional): Array of glob patterns for tools to exclude
- **`include_patterns`** (optional): Array of glob patterns for tools to include (when specified, only matching tools are exposed)
- **`headers`** (optional): Custom HTTP headers as key-value pairs

### Multiple Upstreams

```json
{
  "ENABLE_ADDITIONAL_TOOLS": "proxy",
  "PROXY_UPSTREAMS": "[
    {
      \"name\": \"example-atlassian-sse\",
      \"url\": \"https://mcp.atlassian.com/v1/sse\",
      \"transport\": \"http-first\"
    },
    {
      \"name\": \"example-http-mcp\",
      \"url\": \"https://mcp.example.com/mcp\",
      \"transport\": \"http\",
      \"ignore_patterns\": [\"debug_*\", \"internal_*\"]
    }
  ]"
}
```

## OAuth Authentication

When connecting to OAuth-enabled MCP servers:

1. The proxy automatically detects OAuth requirements by fetching server metadata
2. Opens your default browser to the authorisation URL
3. Starts a local callback server to receive the authorisation code
4. Exchanges the code for access and refresh tokens
5. Stores tokens securely in `~/.mcp-devtools/oauth/<upstream-name>/`
6. Automatically refreshes tokens when they expire

### OAuth Configuration

For servers that require client credentials, configure them via environment variables:

```bash
# Server-specific client ID (optional, falls back to dynamic registration)
PROXY_<UPSTREAM_NAME>_CLIENT_ID="your-client-id"
PROXY_<UPSTREAM_NAME>_CLIENT_SECRET="your-client-secret"

# Example for Atlassian
PROXY_ATLASSIAN_CLIENT_ID="ari:cloud:mcp::app/abc123"
```

### Token Storage

OAuth tokens and client registration info are stored securely:
- **Location**: `~/.mcp-devtools/oauth/<upstream-name>/`
- **Files**:
  - `tokens.json`: Access token, refresh token, and expiry
  - `client-info.json`: Dynamic client registration details
- **Permissions**: 0600 (read/write for owner only)

## Transport Types

### HTTP-First (Recommended)

Automatically selects the best transport:
- Tries HTTP/streamable HTTP first
- Falls back to SSE if HTTP returns 404/405
- Best for servers that support multiple transports

```json
{"transport": "http-first"}
```

### Streamable HTTP

For servers that support the streamable HTTP protocol:
- Responses may be returned in POST body or via SSE stream
- Efficient for request/response patterns
- Lower overhead than pure SSE

```json
{"transport": "http"}
```

### Server-Sent Events (SSE)

For servers using SSE transport:
- Long-lived connection for bi-directional communication
- Client GETs SSE stream, server sends endpoint URL
- Client POSTs requests to endpoint, receives responses via SSE
- Connection stays alive across multiple requests

```json
{"transport": "sse"}
```

## Tool Filtering

Control which upstream tools are exposed using include/ignore patterns:

### Ignore Patterns

Exclude specific tools from registration:

```json
{
  "name": "example",
  "url": "https://mcp.example.com",
  "ignore_patterns": ["debug_*", "internal_*", "admin_*"]
}
```

### Include Patterns

Only expose specific tools (whitelist mode):

```json
{
  "name": "example",
  "url": "https://mcp.example.com",
  "include_patterns": ["search*", "get*"]
}
```

**Note**: If both `include_patterns` and `ignore_patterns` are specified, tools must match an include pattern AND not match any ignore pattern.

## How It Works

### 1. Registration Phase (Before Server Starts)

1. Proxy reads `PROXY_UPSTREAMS` configuration
2. For each upstream:
   - Connects using the specified transport
   - Performs OAuth flow if required (with browser popup)
   - Fetches available tools via `tools/list`
   - Applies filtering rules
   - Registers each tool as a proxied tool in the registry
3. Main MCP server starts with all tools available

### 2. Execution Phase (When Tools Are Called)

1. Client calls a proxied tool (e.g., `search`)
2. Proxy identifies the upstream server for that tool
3. Proxy forwards the request to the upstream server
4. Proxy receives the response (via HTTP body or SSE stream)
5. Proxy returns the response to the client

### Architecture

```
┌─────────────────┐
│   MCP Client    │
│  (Claude Code)  │
└────────┬────────┘
         │ tools/call: search
         ▼
┌─────────────────────────────────┐
│     mcp-devtools Server         │
│  ┌───────────────────────────┐  │
│  │  Proxied Tool: search     │  │
│  │  (upstream: atlassian)    │  │
│  └───────────┬───────────────┘  │
└──────────────┼──────────────────┘
               │
               ▼
┌──────────────────────────────────┐
│   Upstream MCP Server            │
│   (Atlassian)                    │
│  ┌────────────────────────────┐  │
│  │  SSE/HTTP Transport        │  │
│  │  OAuth Authentication      │  │
│  │  Native Tool: search       │  │
│  └────────────────────────────┘  │
└──────────────────────────────────┘
```

## Examples

### Example 1: Atlassian MCP Server

Connect to Atlassian's MCP server to access Confluence and Jira tools:

```json
{
  "ENABLE_ADDITIONAL_TOOLS": "proxy",
  "PROXY_UPSTREAMS": "[{
    \"name\": \"atlassian\",
    \"url\": \"https://mcp.atlassian.com/v1/sse\",
    \"transport\": \"http-first\"
  }]"
}
```

This exposes 27 Atlassian tools including:
- `search` - Search Confluence and Jira using Rovo Search
- `getConfluenceSpaces` - List Confluence spaces
- `searchConfluenceUsingCql` - Search Confluence with CQL
- `getJiraIssue` - Retrieve Jira issue details
- `createJiraIssue` - Create new Jira issues
- And 22 more Confluence/Jira operations

### Example 2: Multiple Upstreams with Filtering

```json
{
  "ENABLE_ADDITIONAL_TOOLS": "proxy",
  "PROXY_UPSTREAMS": "[
    {
      \"name\": \"atlassian\",
      \"url\": \"https://mcp.atlassian.com/v1/sse\",
      \"transport\": \"http-first\",
      \"include_patterns\": [\"search*\", \"get*\"]
    },
    {
      \"name\": \"internal\",
      \"url\": \"https://internal-mcp.corp/api\",
      \"transport\": \"http\",
      \"ignore_patterns\": [\"debug_*\", \"test_*\"],
      \"headers\": {
        \"X-Environment\": \"production\"
      }
    }
  ]"
}
```

### Example 3: Custom OAuth Client

For servers requiring static client credentials:

```bash
export PROXY_EXAMPLE_CLIENT_ID="client-abc123"
export PROXY_EXAMPLE_CLIENT_SECRET="secret-xyz789"
export PROXY_UPSTREAMS='[{"name": "example", "url": "https://mcp.example.com"}]'
```

## Security Considerations

- **OAuth Token Storage**: Tokens stored with 0600 permissions in user home directory
- **PKCE Flow**: Uses Proof Key for Code Exchange for enhanced security
- **Token Refresh**: Automatically refreshes tokens before expiry
- **Transport Security**: All connections use HTTPS
- **Origin Validation**: SSE endpoint URLs validated against server origin
- **Rate Limiting**: Respects upstream server rate limits
- **Error Handling**: Doesn't expose sensitive auth details in errors

## Environment Variables

### Core Configuration
- **`ENABLE_ADDITIONAL_TOOLS`**: Must include `proxy` to enable the proxy tool
- **`PROXY_UPSTREAMS`**: JSON array of upstream server configurations

### Per-Upstream OAuth (Optional)
- **`PROXY_<UPSTREAM_NAME>_CLIENT_ID`**: Static OAuth client ID
- **`PROXY_<UPSTREAM_NAME>_CLIENT_SECRET`**: Static OAuth client secret

### Debugging
- **`DEBUG`**: Set to `1` to enable verbose logging

## Limitations

- OAuth callback server uses a random port (3000-4000 range) if default unavailable
- Browser must be available for OAuth flows (no headless support)
- Tool names must be unique across all upstreams
- Some MCP servers may have additional authentication requirements

## Implementation Details

### SSE Context Lifecycle

The SSE transport uses a long-lived context for the connection that's separate from individual request contexts. This ensures the SSE event reader stays alive across multiple requests:

- Connection context: `context.Background()` with cancellation
- Request contexts: Used only for timeout logic when waiting for responses
- Event reader: Runs until transport is explicitly closed

### Tool Registration

Tools are registered using `RegisterProxiedTool()` which:
- Bypasses normal `ENABLE_ADDITIONAL_TOOLS` checks
- Stores tools in a separate proxied tools list
- Makes tools available before the MCP server starts
- Ensures tools appear as native first-class tools to clients

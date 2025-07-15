# OAuth 2.0/2.1 Browser Authentication

This guide covers the browser-based authentication functionality in MCP DevTools, which enables interactive user authentication via OAuth 2.0/2.1 providers before the MCP server starts.

## Overview

Browser authentication implements OAuth 2.1 authorization code flow with PKCE (Proof Key for Code Exchange) for enhanced security. It's designed for scenarios where users need to authenticate with identity providers before the MCP server can access protected resources.

## Features

- **OAuth 2.1 Compliant**: Full implementation of OAuth 2.1 authorization code flow
- **PKCE Support**: Implements RFC7636 Proof Key for Code Exchange for enhanced security
- **Browser Integration**: Cross-platform browser launching for authentication
- **Localhost Callback Server**: Temporary HTTP server for handling OAuth callbacks
- **RFC8414 Discovery**: Automatic endpoint discovery from issuer metadata
- **RFC8707 Resource Indicators**: Proper token audience binding
- **MCP 2025-06-18 Compliant**: Follows latest MCP specification requirements

## Authentication Flow

1. **Configuration**: Client is configured with OAuth provider details
2. **Endpoint Discovery**: Discovers authorization/token endpoints via RFC8414
3. **PKCE Generation**: Creates secure code challenge/verifier pair
4. **Callback Server**: Starts temporary localhost HTTP server
5. **Browser Launch**: Opens system browser to authorization URL
6. **User Authentication**: User completes authentication in browser
7. **Code Exchange**: Exchanges authorization code for access token
8. **Cleanup**: Shuts down callback server and completes flow

## Configuration

### Environment Variables

```bash
# Enable browser authentication
OAUTH_BROWSER_AUTH=true

# OAuth client configuration
OAUTH_CLIENT_ID="your-client-id"
OAUTH_CLIENT_SECRET="your-client-secret"  # Optional for public clients
OAUTH_ISSUER="https://auth.example.com"
OAUTH_SCOPE="openid profile"
OAUTH_AUDIENCE="https://mcp.example.com"

# Callback server configuration
OAUTH_CALLBACK_PORT=0  # 0 for random port
OAUTH_AUTH_TIMEOUT=5m

# Security settings
OAUTH_REQUIRE_HTTPS=true  # Set false only for development
```

### CLI Flags

```bash
./mcp-devtools --transport=http \
    --oauth-browser-auth \
    --oauth-client-id="your-client-id" \
    --oauth-issuer="https://auth.example.com" \
    --oauth-audience="https://mcp.example.com" \
    --oauth-scope="openid profile"
```

## Usage Examples

### Basic Browser Authentication

```bash
# Enable browser authentication with minimum configuration
OAUTH_BROWSER_AUTH=true \
OAUTH_CLIENT_ID="mcp-devtools-client" \
OAUTH_ISSUER="https://auth.example.com" \
./mcp-devtools --transport=http
```

### With Custom Scopes and Audience

```bash
# Request specific scopes and set audience for token binding
OAUTH_BROWSER_AUTH=true \
OAUTH_CLIENT_ID="mcp-devtools-client" \
OAUTH_ISSUER="https://auth.example.com" \
OAUTH_SCOPE="mcp:tools mcp:resources" \
OAUTH_AUDIENCE="https://mcp.example.com" \
./mcp-devtools --transport=http
```

### Development Configuration

```bash
# Development setup with HTTP localhost
OAUTH_BROWSER_AUTH=true \
OAUTH_CLIENT_ID="dev-client" \
OAUTH_ISSUER="http://localhost:8080" \
OAUTH_AUDIENCE="http://localhost:18080" \
OAUTH_REQUIRE_HTTPS=false \
./mcp-devtools --transport=http --debug
```

## Security Features

### PKCE (Proof Key for Code Exchange)

- Generates cryptographically secure 256-bit code verifiers
- Uses SHA256 challenge method for maximum security
- Prevents authorization code interception attacks
- Required for all public clients per OAuth 2.1

### Token Audience Binding (RFC8707)

- Explicitly binds tokens to intended resource servers
- Prevents token reuse across different services
- Implements resource parameter in authorization/token requests
- Validates audience claims in received tokens

### Secure Callback Handling

- Uses localhost-only callback URLs for security
- Implements HTTPS enforcement (configurable for development)
- Validates state parameters to prevent CSRF attacks
- Secure token storage and handling

## Integration with MCP Server

When browser authentication is enabled, the flow integrates seamlessly with MCP server startup:

1. **Pre-Startup Authentication**: Authentication completes before MCP server starts
2. **Token Storage**: Access tokens are securely stored for server use
3. **Middleware Integration**: Works with existing OAuth resource server middleware
4. **Transport Compatibility**: Only available for HTTP transport (not stdio)

## Error Handling

The implementation provides comprehensive error handling:

- **Configuration Errors**: Invalid or missing configuration parameters
- **Network Errors**: Connection failures, timeouts, DNS resolution
- **OAuth Errors**: Standard OAuth 2.0 error responses from authorization servers
- **Browser Errors**: Browser launching failures, callback timeouts
- **Server Errors**: Callback server startup/shutdown issues

## Browser Compatibility

Supports cross-platform browser launching:

- **macOS**: Uses `open` command
- **Linux**: Uses `xdg-open` command  
- **Windows**: Uses `rundll32` with `url.dll`

## Callback Server

The temporary callback server provides:

- **Random Port Selection**: Avoids port conflicts
- **Success/Error Pages**: User-friendly feedback pages
- **Automatic Shutdown**: Cleans up after authentication
- **Security Headers**: Appropriate HTTP security headers

## Troubleshooting

### Common Issues

1. **Browser doesn't open**: Check if `xdg-open` (Linux) or `open` (macOS) is available
2. **Callback timeout**: Increase `--oauth-auth-timeout` value
3. **Port conflicts**: Use `--oauth-callback-port=0` for random port
4. **HTTPS errors**: Set `--oauth-require-https=false` for development

### Debug Logging

Enable debug logging to see detailed OAuth flow information:

```bash
./mcp-devtools --debug --oauth-browser-auth \
    --oauth-client-id="test-client" \
    --oauth-issuer="https://auth.example.com"
```

## Standards Compliance

This implementation complies with:

- **OAuth 2.1** (draft-ietf-oauth-v2-1-12): Core authorization framework
- **RFC7636**: Proof Key for Code Exchange (PKCE)
- **RFC8414**: OAuth 2.0 Authorization Server Metadata
- **RFC8707**: Resource Indicators for OAuth 2.0
- **MCP 2025-06-18**: Model Context Protocol authorization specification

## Limitations

- **HTTP Transport Only**: Browser authentication requires HTTP transport
- **Desktop/Server Environments**: Designed for environments with browser access
- **Single User**: Each server instance supports one authenticated user
- **No Token Refresh**: Currently implements only initial authentication flow

---

For implementation details, see the [technical documentation](../../internal/oauth/client/README.md).
# OAuth 2.0/2.1 Authentication for MCP DevTools

MCP DevTools provides comprehensive OAuth 2.0/2.1 support for HTTP-based transports, implementing both resource server and client functionality according to the MCP 2025-06-18 specification.

## Overview

OAuth authentication in MCP DevTools operates in two distinct modes:

**🌐 Browser Authentication Mode (OAuth Client)**
- Interactive user authentication via browser
- Authorisation code flow with PKCE
- Perfect for development and desktop environments
- Authenticates before MCP server starts

**🛡️ Resource Server Mode (OAuth Token Validation)**
- Validates incoming JWT tokens from clients
- Protects MCP resources with OAuth authorisation
- Suitable for production API servers
- Validates tokens on each request

## When Do You Need OAuth?

You may want OAuth if you need:
- **User authentication** for accessing the MCP server
- **Token-based security** for production deployments
- **Integration with existing identity providers** (Authentik, Keycloak, etc.)
- **Compliance with organizational authentication** requirements

Most users can skip OAuth and use simple bearer tokens or run without authentication for development as long as the MCP Server is running locally - or does not have access to sensitive data.

## Architecture Overview

```mermaid
graph TD
    subgraph "MCP DevTools OAuth Architecture"
        direction TB

        subgraph "OAuth Modes"
            BrowserMode[🌐 Browser Authentication Mode<br/>OAuth Client]
            ResourceMode[🛡️ Resource Server Mode<br/>Token Validation]
        end

        subgraph "OAuth Provider"
            AuthProvider[OAuth 2.1 Provider]
            AuthServer[Authorisation Server]
            TokenEndpoint[Token Endpoint]
            JWKSEndpoint[JWKS Endpoint]
        end

        subgraph "Browser Auth Flow"
            Browser[🌏 System Browser]
            CallbackServer[📡 Localhost Callback Server]
            PKCEGen[🔐 PKCE Challenge Generator]
        end

        subgraph "Resource Server Flow"
            TokenValidator[🔍 JWT Token Validator]
            JWKSFetch[🔑 JWKS Fetcher]
            AudienceCheck[🎯 Audience Validator]
        end

        User[👤 User] --> BrowserMode
        MCP_Client[📱 External MCP Client] --> ResourceMode

        BrowserMode --> PKCEGen
        BrowserMode --> Browser
        BrowserMode --> CallbackServer

        Browser --> AuthServer
        CallbackServer --> TokenEndpoint

        ResourceMode --> TokenValidator
        TokenValidator --> JWKSFetch
        TokenValidator --> AudienceCheck

        JWKSFetch --> JWKSEndpoint
        AudienceCheck --> AuthProvider
    end

    classDef browserAuth fill:#e1f5fe,stroke:#0277bd,color:#000
    classDef resourceAuth fill:#f3e5f5,stroke:#7b1fa2,color:#000
    classDef oauthProvider fill:#e8f5e8,stroke:#2e7d32,color:#000
    classDef user fill:#fff3e0,stroke:#ef6c00,color:#000

    class BrowserMode,Browser,CallbackServer,PKCEGen browserAuth
    class ResourceMode,TokenValidator,JWKSFetch,AudienceCheck resourceAuth
    class AuthProvider,AuthServer,TokenEndpoint,JWKSEndpoint oauthProvider
    class User,MCP_Client user
```

## Quick Start

### Browser Authentication (Development/Desktop)

For interactive authentication during server startup:

```bash
# Enable browser authentication
OAUTH_BROWSER_AUTH=true
OAUTH_CLIENT_ID="your-client-id"
OAUTH_ISSUER="https://auth.example.com"
OAUTH_AUDIENCE="https://mcp.example.com"

./mcp-devtools --transport=http
```

The server will open your browser for authentication before starting.

### Resource Server (Production)

For validating external client tokens:

```bash
# Enable resource server mode
OAUTH_ENABLED=true
OAUTH_ISSUER="https://auth.example.com"
OAUTH_AUDIENCE="https://mcp.example.com"
OAUTH_JWKS_URL="https://auth.example.com/.well-known/jwks.json"

./mcp-devtools --transport=http
```

## Configuration Options

### Environment Variables

| Variable              | Description                   | Browser Auth | Resource Server |
|-----------------------|-------------------------------|--------------|-----------------|
| `OAUTH_BROWSER_AUTH`  | Enable browser authentication | ✅ Required   | ❌               |
| `OAUTH_ENABLED`       | Enable token validation       | ❌            | ✅ Required      |
| `OAUTH_CLIENT_ID`     | OAuth client identifier       | ✅ Required   | ❌               |
| `OAUTH_CLIENT_SECRET` | OAuth client secret           | 🔶 Optional  | ❌               |
| `OAUTH_ISSUER`        | OAuth issuer URL              | ✅ Required   | ✅ Required      |
| `OAUTH_AUDIENCE`      | Token audience                | ✅ Required   | ✅ Required      |
| `OAUTH_JWKS_URL`      | JWKS endpoint for validation  | ❌            | ✅ Required      |
| `OAUTH_SCOPE`         | Requested scopes              | 🔶 Optional  | ❌               |
| `OAUTH_CALLBACK_PORT` | Callback server port          | 🔶 Optional  | ❌               |
| `OAUTH_REQUIRE_HTTPS` | Enforce HTTPS                 | 🔶 Optional  | 🔶 Optional     |

### CLI Flags

All environment variables have corresponding CLI flags:

```bash
./mcp-devtools --transport=http \
    --oauth-browser-auth \
    --oauth-client-id="your-client-id" \
    --oauth-issuer="https://auth.example.com"
```

## OAuth Modes Comparison

| Scenario                  | Browser Auth   | Resource Server | Both                   |
|---------------------------|----------------|-----------------|------------------------|
| **Development/Testing**   | ✅ Primary      | 🔶 Optional     | ✅ Recommended          |
| **Desktop Applications**  | ✅ Required     | ❌ Not needed    | 🔶 If serving APIs     |
| **Production API Server** | ❌ Not suitable | ✅ Required      | ❌ Choose one           |
| **Microservice**          | ❌ Not suitable | ✅ Required      | ❌ Resource server only |
| **CLI Tools**             | ✅ Perfect fit  | ❌ Not needed    | ❌ Browser auth only    |

## Available Endpoints

When OAuth is enabled, metadata endpoints are available:

- `/.well-known/oauth-authorization-server` - Authorization server metadata (RFC8414)
- `/.well-known/oauth-protected-resource` - Protected resource metadata (RFC9728)
- `/oauth/register` - Dynamic client registration (RFC7591) _(if enabled)_

## Detailed Guides

- **[OAuth Provider Setup with Authentik](authentik-setup.md)** - Complete setup guide for Authentik
- **[Browser Authentication Details](browser-auth.md)** - Comprehensive browser authentication documentation
- **[API Documentation](../api/README.md)** - Technical implementation details and tool registry

## Authentication Flow Diagrams

(Authentik used as example OAuth provider in these diagrams)

### Browser Authentication Flow

```mermaid
sequenceDiagram
    participant User as 👤 User
    participant MCPServer as MCP DevTools Server
    participant Browser as 🌏 System Browser
    participant Callback as 📡 Callback Server
    participant Authentik as Authentik (OAuth Provider)

    Note over User, Authentik: Browser Authentication Mode Startup
    User->>MCPServer: Start with --oauth-browser-auth
    MCPServer->>MCPServer: Generate PKCE challenge
    MCPServer->>Callback: Start localhost callback server
    MCPServer->>Authentik: Discover OAuth endpoints
    Authentik-->>MCPServer: Authorisation & token endpoints

    Note over User, Authentik: Browser-Based Authentication
    MCPServer->>Browser: Launch browser with auth URL + PKCE
    Browser->>Authentik: Authorisation request + code_challenge
    Note right of Authentik: User login & consent
    Authentik->>Browser: Redirect with authorisation code
    Browser->>Callback: GET /callback?code=xyz&state=abc

    Note over User, Authentik: Token Exchange
    Callback->>MCPServer: Authorisation code received
    MCPServer->>Authentik: Exchange code + PKCE verifier for token
    Authentik-->>MCPServer: Access token (JWT)
    MCPServer->>MCPServer: Store token securely
    MCPServer->>Callback: Stop callback server

    Note over User, Authentik: MCP Server Ready
    MCPServer-->>User: Authentication complete, server ready
```

## Standards Compliance

This implementation follows these RFCs:

- **OAuth 2.1** (draft-ietf-oauth-v2-1-12): Core authorization framework
- **RFC8414**: OAuth 2.0 Authorization Server Metadata
- **RFC9728**: OAuth 2.0 Protected Resource Metadata
- **RFC7591**: OAuth 2.0 Dynamic Client Registration Protocol
- **RFC8707**: Resource Indicators for OAuth 2.0
- **MCP 2025-06-18**: Model Context Protocol authorization specification

## Security Considerations

1. **Use HTTPS in Production**: Always use HTTPS for OAuth endpoints in production
2. **Short Token Lifetimes**: Configure short access token lifetimes (10-15 minutes)
3. **Scope Restrictions**: Limit OAuth scopes to minimum required permissions
4. **Audience Validation**: Ensure tokens are bound to the correct resource server
5. **Regular Key Rotation**: Rotate signing keys regularly in your OAuth provider

---

**Note**: OAuth support is completely optional and disabled by default. The server works perfectly without OAuth for development and simple deployments.

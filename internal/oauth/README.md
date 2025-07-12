# OAuth 2.0/2.1 Implementation for MCP DevTools

This package implements OAuth 2.0/2.1 authorization for MCP DevTools according to the MCP 2025-06-18 specification. It provides optional OAuth support for HTTP-based transports only.

## Overview

The implementation includes:

- **OAuth 2.1 Resource Server**: Validates JWT access tokens with audience checking
- **OAuth 2.0 Authorization Server Metadata** (RFC8414): Provides server metadata
- **OAuth 2.0 Protected Resource Metadata** (RFC9728): Advertises OAuth configuration
- **Dynamic Client Registration** (RFC7591): Optional client registration endpoint
- **PKCE Support**: Code challenge/verifier validation for authorization code flow
- **JWT Token Validation**: With JWKS support and audience validation
- **Resource Indicators** (RFC8707): Proper token audience binding

## Package Structure

```
internal/oauth/
├── types/          # OAuth types and interfaces
├── server/         # OAuth resource server implementation
├── metadata/       # RFC8414/RFC9728 metadata providers
├── registration/   # RFC7591 dynamic client registration
└── validation/     # Token validation, PKCE, crypto utilities
```

## Usage

### CLI Configuration

Enable OAuth 2.1 with command-line flags:

```bash
# Basic OAuth configuration
./mcp-devtools --transport=http \
    --oauth-enabled \
    --oauth-issuer="https://auth.example.com" \
    --oauth-audience="https://mcp.example.com" \
    --oauth-jwks-url="https://auth.example.com/.well-known/jwks.json"

# With dynamic client registration
./mcp-devtools --transport=http \
    --oauth-enabled \
    --oauth-issuer="https://auth.example.com" \
    --oauth-audience="https://mcp.example.com" \
    --oauth-jwks-url="https://auth.example.com/.well-known/jwks.json" \
    --oauth-dynamic-registration

# Development mode (allows HTTP)
./mcp-devtools --transport=http \
    --oauth-enabled \
    --oauth-issuer="http://localhost:8080" \
    --oauth-audience="http://localhost:18080" \
    --oauth-jwks-url="http://localhost:8080/.well-known/jwks.json" \
    --oauth-require-https=false
```

### Available Endpoints

When OAuth is enabled, the following endpoints are available:

- `/.well-known/oauth-authorization-server` - Authorization server metadata (RFC8414)
- `/.well-known/oauth-protected-resource` - Protected resource metadata (RFC9728)
- `/oauth/register` - Dynamic client registration (RFC7591) _(if enabled)_

### Client Authentication

Clients must include a valid Bearer token in the Authorization header:

```http
GET /http HTTP/1.1
Host: mcp.example.com:18080
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...
```

### Token Requirements

Access tokens must:

1. Be valid JWT tokens signed by the configured issuer
2. Include the correct audience claim (matching `--oauth-audience`)
3. Not be expired (`exp` claim)
4. Be issued by the configured issuer (`iss` claim)

## Security Features

### MCP Specification Compliance

- ✅ **Authorization is Optional**: Can be disabled (default)
- ✅ **HTTP Transport Only**: STDIO transport uses environment credentials
- ✅ **HTTPS Enforcement**: Required for all OAuth endpoints (configurable for development)
- ✅ **Token Audience Validation**: Prevents token reuse across services
- ✅ **PKCE Support**: Protects against authorization code interception
- ✅ **WWW-Authenticate Headers**: Proper 401 responses with metadata URLs
- ✅ **Resource Parameter**: RFC8707 resource indicators for token binding

### Security Best Practices

1. **Token Audience Binding**: Tokens are validated for the specific resource server
2. **No Token Passthrough**: Tokens are never forwarded to upstream services
3. **HTTPS Enforcement**: All OAuth endpoints require HTTPS (except localhost)
4. **Secure Token Storage**: Implementations should follow OAuth 2.1 security guidelines
5. **Short-lived Tokens**: Authorization servers should issue short-lived access tokens

## Error Handling

The implementation returns proper OAuth 2.0 error responses:

- **401 Unauthorized**: Missing, invalid, or expired tokens
- **403 Forbidden**: Valid token but insufficient permissions
- **400 Bad Request**: Malformed requests

Error responses include:
- `WWW-Authenticate` header with resource metadata URL
- JSON error body with `error` and `error_description` fields
- Appropriate HTTP status codes

## Example Configuration

### Authorization Server Metadata Response

```json
{
  "issuer": "https://auth.example.com",
  "authorization_endpoint": "https://mcp.example.com/oauth/authorize",
  "token_endpoint": "https://mcp.example.com/oauth/token", 
  "jwks_uri": "https://mcp.example.com/.well-known/jwks.json",
  "registration_endpoint": "https://mcp.example.com/oauth/register",
  "response_types_supported": ["code"],
  "grant_types_supported": ["authorization_code", "refresh_token"],
  "token_endpoint_auth_methods_supported": ["client_secret_basic", "client_secret_post", "none"],
  "code_challenge_methods_supported": ["S256", "plain"]
}
```

### Protected Resource Metadata Response

```json
{
  "resource": "https://mcp.example.com",
  "authorization_servers": ["https://auth.example.com"],
  "bearer_methods_supported": ["header"],
  "resource_documentation": "https://mcp.example.com/docs",
  "resource_signing_alg_values_supported": ["RS256", "RS384", "RS512"]
}
```

## Testing

Run OAuth-specific tests:

```bash
make test-fast
```

The test suite includes:
- Metadata endpoint validation
- PKCE challenge/verification
- Dynamic client registration
- Token validation logic
- WWW-Authenticate header generation
- Error response formatting

## Integration

### Adding OAuth to Custom Handlers

```go
// Check if request has valid OAuth claims
claims, ok := oauthserver.GetClaims(ctx)
if !ok {
    // No valid OAuth claims
    return
}

// Check for specific scope
if !oauthserver.HasScope(ctx, "mcp:tools") {
    // Insufficient permissions
    return
}

// Use claims
clientID := claims.ClientID
userID := claims.Subject
```

### Scope-based Authorization

```go
// Require specific scope for handler
handler := oauthserver.RequireScope("admin")(myHandler)
```

## Backward Compatibility

OAuth is completely optional and disabled by default. Existing deployments using simple Bearer token authentication continue to work unchanged. OAuth only activates when `--oauth-enabled` is specified.

## Standards Compliance

This implementation follows these RFCs:

- **OAuth 2.1** (draft-ietf-oauth-v2-1-12): Core authorization framework
- **RFC8414**: OAuth 2.0 Authorization Server Metadata  
- **RFC9728**: OAuth 2.0 Protected Resource Metadata
- **RFC7591**: OAuth 2.0 Dynamic Client Registration Protocol
- **RFC8707**: Resource Indicators for OAuth 2.0
- **MCP 2025-06-18**: Model Context Protocol authorization specification
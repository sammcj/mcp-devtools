# API Tool

Configure and access arbitrary REST APIs through dynamically generated MCP tools.

## Overview

The API tool allows you to configure any REST API in YAML and have it automatically exposed as an MCP tool. Each configured API becomes discoverable by AI agents without requiring code changes.

**Key Features:**
- **Zero-code configuration**: Add new APIs by editing YAML only
- **Dynamic registration**: APIs appear as `${apiname}_api` tools at startup
- **Multi-endpoint support**: Single tool handles multiple API endpoints
- **Authentication support**: Bearer tokens, API keys, and basic auth
- **Security integration**: Full domain validation and content analysis
- **Intelligent caching**: Configurable TTL with automatic cache management

## Configuration

Create `~/.mcp-devtools/apis.yaml` with your API definitions:

### Basic Structure

```yaml
apis:
  api_name:
    base_url: "https://api.example.com"
    description: "API description"
    auth:
      type: "bearer|api_key|basic|none"
      env_var: "ENV_VAR_NAME"
    timeout: 30        # seconds, default: 30
    cache_ttl: 300     # seconds, default: 300
    endpoints:
      - name: "endpoint_name"
        method: "GET|POST|PUT|PATCH|DELETE"
        path: "/api/path/{param}"
        description: "Endpoint description"
        parameters: []   # see parameter configuration below
```

### Authentication Types

**Bearer Token:**
```yaml
auth:
  type: "bearer"
  env_var: "API_TOKEN"  # Environment variable containing the token
```

**API Key:**
```yaml
auth:
  type: "api_key"
  env_var: "API_KEY"
  header: "X-API-Key"     # optional, default: "X-API-Key"
  location: "header"      # "header" or "query", default: "header"
```

**Basic Authentication:**
```yaml
auth:
  type: "basic"
  username: "$USERNAME_VAR"  # Environment variable reference
  password: "$PASSWORD_VAR"  # Environment variable reference
```

**No Authentication:**
```yaml
auth:
  type: "none"
```

### Parameter Configuration

Parameters can be placed in different locations:

```yaml
parameters:
  - name: "id"
    type: "string"          # string, number, boolean, array, object
    required: true
    description: "Resource ID"
    location: "path"        # path, query, header, body
  - name: "limit"
    type: "number"
    required: false
    description: "Number of results"
    location: "query"
    default: 10
  - name: "format"
    type: "string"
    required: false
    description: "Response format"
    location: "query"
    enum: ["json", "xml"]   # Restrict to specific values
```

**Parameter Locations:**
- `path`: URL path parameters (e.g., `/users/{id}`)
- `query`: URL query parameters (e.g., `?limit=10`)
- `header`: HTTP headers
- `body`: Request body (POST/PUT/PATCH only)

### Request Body Configuration

For endpoints that accept request bodies:

```yaml
endpoints:
  - name: "create_issue"
    method: "POST"
    path: "/repos/{owner}/{repo}/issues"
    body:
      type: "json"                    # json, form, raw
      content_type: "application/json" # optional override
    parameters:
      - name: "title"
        location: "body"
        type: "string"
        required: true
```

## Examples

### GitHub API Configuration

```yaml
apis:
  github:
    base_url: "https://api.github.com"
    description: "GitHub REST API access"
    auth:
      type: "bearer"
      env_var: "GITHUB_TOKEN"
    timeout: 30
    cache_ttl: 300
    headers:
      Accept: "application/vnd.github.v3+json"
    endpoints:
      - name: "get_user"
        method: "GET"
        path: "/user"
        description: "Get authenticated user information"
        parameters: []
      - name: "create_issue"
        method: "POST"
        path: "/repos/{owner}/{repo}/issues"
        description: "Create a new issue"
        body:
          type: "json"
        parameters:
          - name: "owner"
            type: "string"
            required: true
            location: "path"
            description: "Repository owner"
          - name: "repo"
            type: "string"
            required: true
            location: "path"
            description: "Repository name"
          - name: "title"
            type: "string"
            required: true
            location: "body"
            description: "Issue title"
          - name: "body"
            type: "string"
            required: false
            location: "body"
            description: "Issue body text"
          - name: "labels"
            type: "array"
            required: false
            location: "body"
            description: "Issue labels"
```

### Slack API Configuration

```yaml
apis:
  slack:
    base_url: "https://slack.com/api"
    description: "Slack Web API access"
    auth:
      type: "bearer"
      env_var: "SLACK_BOT_TOKEN"
    endpoints:
      - name: "send_message"
        method: "POST"
        path: "/chat.postMessage"
        description: "Send a message to a channel"
        body:
          type: "json"
        parameters:
          - name: "channel"
            type: "string"
            required: true
            location: "body"
            description: "Channel ID or name"
          - name: "text"
            type: "string"
            required: true
            location: "body"
            description: "Message text"
          - name: "thread_ts"
            type: "string"
            required: false
            location: "body"
            description: "Thread timestamp for replies"
```

### Custom Internal API

```yaml
apis:
  internal:
    base_url: "https://internal.company.com/api/v1"
    description: "Internal company API"
    auth:
      type: "api_key"
      env_var: "INTERNAL_API_KEY"
      header: "X-Internal-Key"
    timeout: 15
    cache_ttl: 600
    endpoints:
      - name: "get_metrics"
        method: "GET"
        path: "/metrics/{service}"
        description: "Get service metrics"
        parameters:
          - name: "service"
            type: "string"
            required: true
            location: "path"
            description: "Service name"
            enum: ["web", "api", "db", "cache"]
          - name: "period"
            type: "string"
            required: false
            location: "query"
            default: "1h"
            description: "Time period (1h, 24h, 7d)"
```

## Usage

Once configured, APIs appear as MCP tools named `${apiname}_api`:

**Example: Using GitHub API**
```json
{
  "tool": "github_api",
  "arguments": {
    "endpoint": "get_user"
  }
}
```

**Example: Creating a GitHub issue**
```json
{
  "tool": "github_api", 
  "arguments": {
    "endpoint": "create_issue",
    "owner": "sammcj",
    "repo": "mcp-devtools",
    "title": "Bug report",
    "body": "Found an issue with the API tool",
    "labels": ["bug", "api"]
  }
}
```

**Example: Sending Slack message**
```json
{
  "tool": "slack_api",
  "arguments": {
    "endpoint": "send_message",
    "channel": "#general",
    "text": "API tool is working great!"
  }
}
```

## Response Format

All API tools return structured responses:

```json
{
  "status_code": 200,
  "headers": {
    "content-type": "application/json"
  },
  "data": {
    // API response data
  },
  "endpoint": "endpoint_name",
  "api": "api_name", 
  "cached": false
}
```

## Security

The API tool integrates fully with the MCP DevTools security framework:

- **Domain validation**: All requests validated against security policies
- **Content analysis**: Response content scanned for security risks  
- **Access control**: Blocked domains and file patterns respected
- **Audit logging**: All API calls logged for security review

If a request is blocked by security policy, the error message includes a security ID that can be used with the `security_override` tool if needed.

## Caching

- **Automatic caching**: Responses cached based on `cache_ttl` setting
- **Cache key**: Generated from API name, endpoint, and parameters
- **TTL management**: Expired entries automatically removed
- **Cache bypass**: Set `cache_ttl: 0` to disable caching for an API

## Environment Variables

All authentication credentials must be stored in environment variables:

```bash
# GitHub API
export GITHUB_TOKEN="ghp_your_token_here"

# Slack API  
export SLACK_BOT_TOKEN="xoxb-your-bot-token"

# Custom APIs
export INTERNAL_API_KEY="your-internal-key"
export EXTERNAL_API_USER="username"
export EXTERNAL_API_PASS="password"
```

## Troubleshooting

**Common Issues:**

1. **"Tool not found"**: Check that `apis.yaml` exists and contains valid configuration
2. **Authentication errors**: Verify environment variables are set correctly
3. **Invalid endpoint**: Ensure endpoint name matches configuration exactly
4. **Missing parameters**: Check that all required parameters are provided
5. **Security blocks**: Use security ID with `security_override` tool if needed

**Debug Steps:**

1. Verify configuration file syntax with a YAML validator
2. Test API credentials with curl or similar tool
3. Check MCP DevTools logs for detailed error messages
4. Validate parameter types match API expectations

## Configuration Validation

The configuration is automatically validated at startup:

- **Required fields**: `base_url`, endpoint names, methods, and paths
- **Authentication**: Credentials existence and format
- **Parameter types**: Valid types and locations
- **HTTP methods**: Standard HTTP verbs only
- **URL patterns**: Valid path parameter syntax

## Advanced Features

**Custom Headers:**
```yaml
apis:
  api_name:
    headers:
      User-Agent: "MyApp/1.0"
      Accept: "application/json"
    endpoints:
      - name: "endpoint"
        headers:
          X-Custom: "endpoint-specific"
```

**Multiple Authentication:**
```yaml
# Mix different auth types for different APIs
apis:
  service_a:
    auth:
      type: "bearer"
      env_var: "SERVICE_A_TOKEN"
  service_b:
    auth:
      type: "api_key"
      env_var: "SERVICE_B_KEY"
      location: "query"
```

The API tool transforms any REST API into a native MCP tool, making external services seamlessly accessible to AI agents through a unified, secure interface.
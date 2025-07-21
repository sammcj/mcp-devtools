# Confluence Search Tool

Search Confluence and retrieve content converted to Markdown format.

## Overview

The Confluence Search tool allows you to search your Confluence instance and retrieve content in a readable Markdown format. It supports multiple authentication methods and can search across different content types.

## Features

- **Multi-format Search**: Search by query string or direct URL
- **Content Type Filtering**: Filter by pages, blog posts, comments, or attachments
- **Space Filtering**: Limit searches to specific Confluence spaces
- **Markdown Conversion**: Automatically converts Confluence content to clean Markdown
- **Multiple Authentication Methods**: Basic auth, session cookies, browser automation, or OAuth
- **Caching**: Built-in caching for improved performance

## Configuration

### Environment Variables

The Confluence tool requires configuration through environment variables. At minimum, you need:

```bash
# Required
CONFLUENCE_URL="https://your-confluence.atlassian.net"

# Choose one authentication method:

# Option 1: Basic Authentication (API Token)
CONFLUENCE_USERNAME="your-email@example.com"
CONFLUENCE_TOKEN="your-api-token"

# Option 2: Session Cookies
CONFLUENCE_SESSION_COOKIES="session=abc123; other-cookie=xyz789"

# Option 3: Browser Automation
CONFLUENCE_BROWSER_TYPE="chrome"  # chrome, firefox, edge, brave

# Option 4: OAuth (requires OAuth setup)
CONFLUENCE_OAUTH_CLIENT_ID="your-client-id"
CONFLUENCE_OAUTH_ISSUER_URL="https://auth.example.com"
```

### Authentication Methods

#### 1. Basic Authentication (Recommended)
Most secure and reliable method using API tokens:

```bash
CONFLUENCE_URL="https://your-confluence.atlassian.net"
CONFLUENCE_USERNAME="your-email@example.com"
CONFLUENCE_TOKEN="your-api-token"
```

To get an API token:
1. Go to [Atlassian Account Settings](https://id.atlassian.com/manage-profile/security/api-tokens)
2. Create API token
3. Use your email as username and the token as password

#### 2. Session Cookies
Use existing browser session cookies:

```bash
CONFLUENCE_URL="https://your-confluence.atlassian.net"
CONFLUENCE_SESSION_COOKIES="session=abc123; other-cookie=xyz789"
```

#### 3. Browser Automation
Automatically extract cookies from your browser:

```bash
CONFLUENCE_URL="https://your-confluence.atlassian.net"
CONFLUENCE_BROWSER_TYPE="chrome"  # or firefox, edge, brave
```

#### 4. OAuth
For enterprise setups with centralised authentication:

```bash
CONFLUENCE_URL="https://your-confluence.atlassian.net"
CONFLUENCE_OAUTH_CLIENT_ID="your-client-id"
CONFLUENCE_OAUTH_ISSUER_URL="https://auth.example.com"
```

## Usage

### Basic Search

```json
{
  "query": "authentication setup"
}
```

### Advanced Search Options

```json
{
  "query": "API documentation",
  "space_key": "DEV",
  "max_results": 5,
  "content_types": ["page", "blogpost"]
}
```

### Direct URL Access

```json
{
  "query": "https://your-confluence.atlassian.net/display/SPACE/Page+Title"
}
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `query` | string | Yes | - | Search query or direct Confluence URL |
| `space_key` | string | No | - | Limit search to specific space (e.g., "DEV", "DOCS") |
| `max_results` | number | No | 3 | Maximum results to return (1-10) |
| `content_types` | array | No | ["page"] | Content types to search: "page", "blogpost", "comment", "attachment" |

## Response Format

```json
{
  "query": "authentication setup",
  "total_count": 2,
  "message": "Found 2 results for 'authentication setup'",
  "results": [
    {
      "id": "123456",
      "type": "page",
      "title": "Authentication Setup Guide",
      "url": "https://confluence.example.com/rest/api/content/123456",
      "web_url": "https://confluence.example.com/display/DEV/Authentication+Setup+Guide",
      "last_modified": "2024-01-15T10:30:00Z",
      "content": "# Authentication Setup Guide\n\nThis guide covers...",
      "content_preview": "Authentication Setup Guide - This guide covers setting up authentication...",
      "space": {
        "key": "DEV",
        "name": "Development",
        "type": "global"
      },
      "author": {
        "account_id": "user123",
        "display_name": "John Doe",
        "email": "john.doe@example.com"
      }
    }
  ]
}
```

## Content Conversion

The tool automatically converts Confluence content to Markdown:

- **Headers**: `<h1>` → `# Header`
- **Bold/Italic**: `<strong>` → `**bold**`, `<em>` → `*italic*`
- **Links**: `<a href="...">text</a>` → `[text](...)`
- **Lists**: `<ul><li>` → `- item`
- **Code blocks**: `<pre><code>` → ` ```code``` `
- **Blockquotes**: `<blockquote>` → `> quote`
- **Confluence Macros**: Converted to appropriate Markdown equivalents

### Confluence Macro Support

Common Confluence macros are converted:

- **Code Macro**: Converted to fenced code blocks with language
- **Info/Note/Warning Macros**: Converted to blockquotes
- **Table of Contents**: Converted to "Table of Contents" heading
- **Expand Macro**: Content extracted and included

## Examples

### Search for Documentation

```json
{
  "query": "API documentation",
  "space_key": "DEV",
  "max_results": 3
}
```

### Find Recent Blog Posts

```json
{
  "query": "release notes",
  "content_types": ["blogpost"],
  "max_results": 5
}
```

### Access Specific Page

```json
{
  "query": "https://confluence.example.com/display/DEV/Setup+Guide"
}
```

### Search Multiple Content Types

```json
{
  "query": "troubleshooting",
  "content_types": ["page", "blogpost", "comment"],
  "max_results": 10
}
```

## Error Handling

The tool provides detailed error messages for common issues:

- **Configuration errors**: Missing URL or authentication
- **Authentication failures**: Invalid credentials or expired tokens
- **Permission errors**: Insufficient access to content
- **Network errors**: Connection timeouts or server issues
- **Content errors**: Invalid URLs or missing pages

## Performance

- **Caching**: Results are cached to improve performance
- **Rate Limiting**: Respects Confluence API rate limits
- **Batch Processing**: Efficiently processes multiple results
- **Content Streaming**: Large content is processed in chunks

## Troubleshooting

### Common Issues

1. **"CONFLUENCE_URL environment variable is required"**
   - Set the `CONFLUENCE_URL` environment variable

2. **"Authentication failed"**
   - Check your credentials or API token
   - Verify the username/email is correct
   - Ensure the API token has proper permissions

3. **"Access denied"**
   - Check if you have permission to access the content
   - Verify space permissions in Confluence

4. **"Content not found"**
   - Check if the URL or page exists
   - Verify the space key is correct

### Debug Mode

Enable debug logging to troubleshoot issues:

```bash
mcp-devtools --debug
```

## Security Considerations

- **API Tokens**: Store securely, rotate regularly
- **Session Cookies**: Have limited lifetime, may expire
- **Browser Automation**: Requires browser to be logged in
- **OAuth**: Most secure for enterprise environments
- **Network**: Use HTTPS URLs only
- **Permissions**: Tool respects Confluence permissions

## Integration Examples

### With Claude Desktop

```json
{
  "mcpServers": {
    "dev-tools": {
      "command": "/path/to/mcp-devtools",
      "env": {
        "CONFLUENCE_URL": "https://your-confluence.atlassian.net",
        "CONFLUENCE_USERNAME": "your-email@example.com",
        "CONFLUENCE_TOKEN": "your-api-token"
      }
    }
  }
}
```

### With Cline

The tool integrates seamlessly with Cline for documentation research and content analysis.

## Limitations

- **Rate Limits**: Subject to Confluence API rate limits
- **Content Size**: Very large pages may be truncated
- **Macro Support**: Some complex macros may not convert perfectly
- **Permissions**: Can only access content you have permission to view
- **Version Support**: Tested with Confluence Cloud and Server 7.0+

## Related Tools

- **[Web Fetch](web-fetch.md)**: For general web content retrieval
- **[Internet Search](internet-search.md)**: For broader web searches
- **[Document Processing](document-processing.md)**: For processing downloaded documents

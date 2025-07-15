# Web Fetch Tool

The Web Fetch tool retrieves content from web URLs and converts it to clean, readable Markdown format with enhanced pagination support for large content.

## Overview

Perfect for extracting content from documentation sites, blog posts, articles, and other web resources. The tool handles HTML conversion, pagination of large content, and provides clean Markdown output suitable for further processing.

## Features

- **HTML to Markdown**: Clean conversion with preserved structure
- **Pagination Support**: Handle large content with chunked responses
- **Content Preview**: See what comes next in paginated responses
- **Raw HTML Option**: Get original HTML when needed
- **Smart Caching**: 15-minute cache for repeated requests
- **Error Handling**: Robust handling of network issues and redirects

## Usage Examples

While intended to be activated via a prompt to an agent, below are some example JSON tool calls.

### Basic URL Fetch
```json
{
  "name": "fetch_url",
  "arguments": {
    "url": "https://docs.example.com/api-guide"
  }
}
```

### Fetch with Length Limit
```json
{
  "name": "fetch_url",
  "arguments": {
    "url": "https://blog.example.com/long-article",
    "max_length": 3000
  }
}
```

### Raw HTML Extraction
```json
{
  "name": "fetch_url",
  "arguments": {
    "url": "https://example.com/complex-page",
    "raw": true
  }
}
```

### Paginated Content Access
```json
{
  "name": "fetch_url",
  "arguments": {
    "url": "https://documentation.site.com/comprehensive-guide",
    "start_index": 6000,
    "max_length": 4000
  }
}
```

## Parameters Reference

### Core Parameters
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `url` | string | Required | HTTP/HTTPS URL to fetch |
| `max_length` | number | 6000 | Maximum characters to return |
| `raw` | boolean | false | Return raw HTML instead of Markdown |
| `start_index` | number | 0 | Starting character index for pagination |

### URL Requirements
- Must be `http://` or `https://` protocol
- Publicly accessible (no authentication required)
- Returns HTML content (not binary files)

### Length and Pagination
- **Default**: 6000 characters maximum
- **Range**: Up to 1,000,000 characters per request
- **Pagination**: Use `start_index` for accessing content beyond max_length

## Response Format

### Standard Response
```json
{
  "url": "https://docs.example.com/api-guide",
  "content": "# API Guide\n\nThis guide covers...",
  "content_type": "text/html",
  "status_code": 200,
  "title": "API Guide - Documentation",
  "pagination": {
    "total_lines": 150,
    "start_line": 1,
    "end_line": 85,
    "remaining_lines": 65,
    "next_chunk_preview": "## Advanced Topics\nThis section covers..."
  }
}
```

### Paginated Response
```json
{
  "url": "https://blog.example.com/comprehensive-tutorial",
  "content": "Content starting from character 3000...",
  "pagination": {
    "total_lines": 500,
    "start_line": 125,
    "end_line": 200,
    "remaining_lines": 300,
    "next_chunk_preview": "## Next Section\nContinuing with..."
  }
}
```

### Error Response
```json
{
  "url": "https://invalid-site.example.com",
  "error": "Failed to fetch URL: DNS resolution failed",
  "status_code": 0
}
```

## Common Use Cases

### Documentation Research
Fetch technical documentation for analysis:
```json
{
  "name": "fetch_url",
  "arguments": {
    "url": "https://kubernetes.io/docs/concepts/overview/",
    "max_length": 8000
  }
}
```

### Blog Post Analysis
Extract articles for content analysis:
```json
{
  "name": "fetch_url",
  "arguments": {
    "url": "https://martinfowler.com/articles/microservices.html",
    "max_length": 10000
  }
}
```

### API Documentation
Get API documentation for reference:
```json
{
  "name": "fetch_url",
  "arguments": {
    "url": "https://developer.github.com/v3/repos/",
    "max_length": 5000
  }
}
```

### Large Content Processing
Handle large documents with pagination:
```json
// First chunk
{
  "name": "fetch_url",
  "arguments": {
    "url": "https://example.com/comprehensive-guide",
    "max_length": 5000
  }
}

// Next chunk based on pagination info
{
  "name": "fetch_url",
  "arguments": {
    "url": "https://example.com/comprehensive-guide",
    "start_index": 5000,
    "max_length": 5000
  }
}
```

## Workflow Integration

### Research Workflow
```bash
# 1. Search for relevant content
internet_search "kubernetes ingress configuration best practices"

# 2. Fetch detailed documentation from results
fetch_url "https://kubernetes.io/docs/concepts/services-networking/ingress/"

# 3. Analyse and store insights
think "The documentation shows three main configuration approaches. Let me extract the key differences and recommended practices."

# 4. Store findings
memory create_entities --data '{"entities": [{"name": "Kubernetes_Ingress_Config", "observations": ["Supports path-based routing", "Requires ingress controller"]}]}'
```

### Documentation Analysis Workflow
```bash
# 1. Fetch multiple related documents
fetch_url "https://docs.docker.com/compose/compose-file/"
fetch_url "https://docs.docker.com/compose/environment-variables/"

# 2. Compare and analyse
think "Comparing the compose file documentation with environment variable handling, I can see best practices for production deployments."

# 3. Extract actionable insights
package_search --ecosystem="docker" --query="nginx" --action="tags"
```

### Learning Workflow
```bash
# 1. Fetch tutorial content
fetch_url "https://go.dev/tour/concurrency/1" --max_length=3000

# 2. Get additional examples
fetch_url "https://gobyexample.com/goroutines" --max_length=2000

# 3. Synthesise learning
think "Both sources explain goroutines, but the Go tour focuses on syntax while Go by Example shows practical patterns. I'll combine both approaches."

# 4. Store knowledge
memory create_entities --namespace="learning" --data='{"entities": [{"name": "Go_Goroutines", "observations": ["Lightweight threads", "Use channels for communication"]}]}'
```

## Advanced Features

### Redirect Handling
The tool automatically follows redirects and informs you:
```json
{
  "url": "https://short.link/example",
  "final_url": "https://real-destination.com/page",
  "content": "...",
  "redirected": true
}
```

### Content Type Detection
Handles various content types:
- **HTML pages**: Converted to Markdown
- **Plain text**: Returned as-is
- **JSON/XML**: Formatted appropriately
- **Unsupported types**: Clear error message

### Caching Behaviour
- **Cache duration**: 15 minutes for identical URLs
- **Cache key**: URL + parameters (max_length, raw, start_index)
- **Cache benefits**: Faster responses, reduced server load
- **Cache bypass**: Automatic for different parameters

## Error Handling

### Network Errors
```json
{
  "error": "Network timeout after 30 seconds",
  "url": "https://slow-server.example.com",
  "retry_suggestion": "Try again later or check network connectivity"
}
```

### HTTP Errors
```json
{
  "error": "HTTP 404: Page not found",
  "url": "https://example.com/missing-page",
  "status_code": 404
}
```

### Content Errors
```json
{
  "error": "Content too large (5MB), maximum allowed is 1MB",
  "url": "https://example.com/huge-page",
  "size_limit": 1048576
}
```

## Performance Tips

### Optimise Request Size
```json
// Good: Request appropriate amount
{"max_length": 5000}

// Avoid: Unnecessarily large requests
{"max_length": 100000}
```

### Use Pagination Effectively
```json
// Good: Process in manageable chunks
{"max_length": 4000, "start_index": 0}
{"max_length": 4000, "start_index": 4000}

// Avoid: Single massive request
{"max_length": 50000}
```

### Leverage Caching
```json
// First request: Fetches from web
{"url": "https://example.com", "max_length": 3000}

// Second request within 15 minutes: Returns cached result
{"url": "https://example.com", "max_length": 3000}
```

## Content Quality

### Markdown Conversion Quality
- **Headings**: Properly converted to # syntax
- **Lists**: Bullet points and numbered lists preserved
- **Links**: Maintained with proper syntax
- **Code blocks**: Preserved with syntax highlighting hints
- **Tables**: Converted to Markdown table format
- **Images**: Alt text preserved, src URLs included

### Content Cleaning
- **Removes**: Navigation elements, advertisements, footers
- **Preserves**: Main content, headings, structured data
- **Standardises**: Consistent formatting and spacing
- **Maintains**: Original content structure and flow

## Security Considerations

- **URL Validation**: Only HTTP/HTTPS URLs accepted
- **Content Limits**: Maximum content size enforced
- **Timeout Protection**: Prevents hanging requests
- **No File Downloads**: Only web page content, not file downloads
- **Public Content Only**: No authentication or cookie support

---

For technical implementation details, see the [Web Fetch source documentation](../../internal/tools/webfetch/).

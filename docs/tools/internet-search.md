# Internet Search Tool

The Internet Search tool provides a unified interface for searching across multiple search providers, supporting web, image, news, video, and local search capabilities.

## Overview

Instead of managing separate tools for different search providers, the Internet Search tool gives you access to multiple search engines through a single interface. It automatically handles provider-specific requirements and normalises results.

## Supported Providers

### Brave Search
- **Web Search**: General web search with fresh results
- **Image Search**: Search for images with metadata
- **News Search**: Recent news articles and events
- **Video Search**: Video content with metadata
- **Local Search**: Local businesses and points of interest (Pro API required)

### SearXNG
- **Web Search**: Privacy-focused search aggregation
- **Image Search**: Images via SearXNG instance
- **News Search**: News articles via SearXNG
- **Video Search**: Video content via SearXNG

### DuckDuckGo
- **Web Search**: Privacy-focused web search (no API key required)

## Configuration

Example MCP Client Configuration:

```json
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "/path/to/mcp-devtools",
      "env": {
        "BRAVE_API_KEY": "your-brave-api-key",
        "SEARXNG_BASE_URL": "https://your-searxng-instance.com"
      }
    }
  }
}
```

### Brave Search Setup
Get your API key from [Brave Search API](https://brave.com/search/api/) and set:

```bash
BRAVE_API_KEY="your-brave-api-key"
```

### SearXNG Setup
For self-hosted or public SearXNG instances:

```bash
SEARXNG_BASE_URL="https://your-searxng-instance.com"
# Optional authentication:
SEARXNG_USERNAME="your-username"
SEARXNG_PASSWORD="your-password"
```

### DuckDuckGo
No configuration required - works out of the box.

## Usage Examples

While intended to be activated via a prompt to an agent, below are some example JSON tool calls.

### Web Search
```json
{
  "name": "internet_search",
  "arguments": {
    "type": "web",
    "query": "golang best practices",
    "count": 10,
    "provider": "brave",
    "freshness": "pw"
  }
}
```

### Image Search
```json
{
  "name": "internet_search",
  "arguments": {
    "type": "image",
    "query": "golang gopher mascot",
    "count": 3,
    "provider": "searxng"
  }
}
```

### News Search
```json
{
  "name": "internet_search",
  "arguments": {
    "type": "news",
    "query": "artificial intelligence breakthrough",
    "count": 10,
    "provider": "brave",
    "freshness": "pd"
  }
}
```

### Video Search
```json
{
  "name": "internet_search",
  "arguments": {
    "type": "video",
    "query": "golang tutorial",
    "count": 10,
    "provider": "searxng"
  }
}
```

### Local Search (Brave Pro Required)
```json
{
  "name": "internet_search",
  "arguments": {
    "type": "local",
    "query": "coffee shops near Fitzroy",
    "count": 5,
    "provider": "brave"
  }
}
```

### DuckDuckGo Web Search
```json
{
  "name": "internet_search",
  "arguments": {
    "type": "web",
    "query": "golang best practices",
    "count": 5,
    "provider": "duckduckgo"
  }
}
```

## Parameters Reference

### Core Parameters
- **`type`** (required): Search type - `web`, `image`, `news`, `video`, `local`
- **`query`** (required): Search query string
- **`provider`** (optional): Provider to use - `brave`, `searxng`, `duckduckgo`
- **`count`** (optional): Number of results to return

### Brave-Specific Parameters
- **`freshness`**: Time filter for results
  - `pd`: Past 24 hours
  - `pw`: Past week
  - `pm`: Past month
  - `py`: Past year
  - `YYYY-MM-DDtoYYYY-MM-DD`: Custom date range
- **`offset`**: Pagination offset (web search only)

### SearXNG-Specific Parameters
- **`time_range`**: Time filter - `day`, `week`, `month`, `year`

## Search Types

### Web Search
Best for general information gathering, research, and finding relevant websites.

**Example Results:**
- Page titles and descriptions
- URLs and snippets
- Publication dates
- Source metadata

### Image Search
Find relevant images with metadata for presentations, documentation, or reference.

**Example Results:**
- Image URLs and thumbnails
- Alt text and descriptions
- Source websites
- Image dimensions

### News Search
Stay updated with recent events and news articles from various sources.

**Example Results:**
- Article headlines and summaries
- Publication dates and sources
- Author information
- News category tags

### Video Search
Discover educational content, tutorials, and relevant video material.

**Example Results:**
- Video titles and descriptions
- Video URLs and thumbnails
- Duration and view counts
- Source platforms

### Local Search
Find local businesses, restaurants, and services (Brave Pro API required).

**Example Results:**
- Business names and descriptions
- Addresses and contact information
- Ratings and reviews
- Opening hours

## Provider Selection Guide

### When to Use Brave Search
- **Best for**: Fresh, comprehensive results with strong English content
- **Pros**: Fast, reliable, good metadata, local search support
- **Cons**: Requires API key, usage limits based on plan

### When to Use SearXNG
- **Best for**: Privacy-focused search, aggregated results
- **Pros**: No tracking, multiple search engines, self-hostable
- **Cons**: Requires instance setup, variable performance

### When to Use DuckDuckGo
- **Best for**: Quick web searches without setup
- **Pros**: No API key required, privacy-focused, reliable
- **Cons**: Limited to web search only, fewer customisation options

## Common Use Cases

### Research Workflow
1. Start with web search to get overview
2. Use news search for recent developments
3. Find images for visual references
4. Look up video tutorials for complex topics

### Content Creation
1. Web search for background information
2. Image search for visual assets
3. News search for current events
4. Video search for tutorial references

### Development Research
1. Web search for technical documentation
2. Video search for coding tutorials
3. News search for technology updates
4. Image search for architecture diagrams

## Rate Limits and Quotas

### Brave Search
- **Free Plan**: 2,000 queries/month
- **Pro Plan**: 20,000 queries/month + local search
- **Enterprise**: Custom limits

### SearXNG
- Depends on instance configuration
- Self-hosted instances have no built-in limits

### DuckDuckGo
- No official limits for reasonable usage
- Automatic rate limiting applies

## Error Handling

The tool provides clear error messages for common issues:
- Missing API keys
- Invalid query parameters
- Network connectivity problems
- Provider-specific errors
- Rate limit exceeded

## Performance Tips

1. **Use appropriate result counts** - Request only what you need
2. **Leverage freshness filters** - Reduce processing time for time-sensitive queries
3. **Choose providers wisely** - Match provider strengths to your use case
4. **Cache results** - The tool includes intelligent caching for repeated queries

---

For technical implementation details, see the [Internet Search source documentation](../../internal/tools/internetsearch/README.md).

# Internet Search Tools

This package contains tools for searching the internet using various search providers.

## Available Providers

### Brave Search

Only made available when the `BRAVE_API_KEY` environment variable is set.

- **Web Search**: General web search using Brave Search API
- **Image Search**: Search for images with metadata
- **News Search**: Search for news articles and recent events
- **Local Search**: Search for local businesses and points of interest
- **Video Search**: Search for videos with metadata

### SearXNG

Only made available when the `SEARXNG_BASE_URL` environment variable is set.

- **Web Search**: General web search using SearXNG instance
- **Image Search**: Search for images via SearXNG
- **News Search**: Search for news articles via SearXNG
- **Video Search**: Search for videos via SearXNG

## Configuration

### Brave Search
Set the `BRAVE_API_KEY` environment variable to enable Brave search:

```bash
"dev-tools": {
  "type": "stdio",
  "command": "/Users/samm/git/sammcj/mcp-devtools/bin/mcp-devtools",
  "env": {
    "BRAVE_API_KEY": "your-key-here"
  }
}
```

Get your API key from: https://brave.com/search/api/


#### SearXNG
Set the `SEARXNG_BASE_URL` environment variable to enable SearXNG search.

```bash
"dev-tools": {
  "type": "stdio",
  "command": "/Users/samm/git/sammcj/mcp-devtools/bin/mcp-devtools",
  "env": {
    "SEARXNG_BASE_URL": "https://your-searxng-instance.com"
  }
}
```

## Conditional Registration

Tools only register when their required environment variables are present. This ensures:
- Clean tool lists showing only functional tools
- Clear startup logging about enabled/disabled providers
- No runtime API key errors

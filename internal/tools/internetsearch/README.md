# Internet Search Tools

This package contains tools for searching the internet using various search providers.

## Available Providers

### Brave Search
- **Web Search**: General web search using Brave Search API
- **Image Search**: Search for images with metadata
- **News Search**: Search for news articles and recent events
- **Local Search**: Search for local businesses and points of interest
- **Video Search**: Search for videos with metadata

## Configuration

### Brave Search
Set the `BRAVE_API_KEY` environment variable to enable Brave search tools:

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

## Conditional Registration

Tools only register when their required environment variables are present. This ensures:
- Clean tool lists showing only functional tools
- Clear startup logging about enabled/disabled providers
- No runtime API key errors

## Future Providers

The architecture supports additional search providers that may be added in the future, such as:
- SearXNG (would use `SEARXNG_BASE_URL`)
- DuckDuckGo (if API becomes available)

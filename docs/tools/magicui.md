# Magic UI Tool

Access the Magic UI animated component library with 74+ free components built with React, TypeScript, Tailwind CSS, and Framer Motion.

## Overview

The Magic UI tool provides programmatic access to the Magic UI component registry, allowing you to discover, search, and get detailed information about animated React components. Unlike shadcn/ui which requires web scraping, Magic UI has a structured JSON registry making it faster and more reliable.

## Tool Information

- **Name**: `magic_ui`
- **Availability**: Disabled by default (requires `ENABLE_ADDITIONAL_TOOLS=magic_ui`)
- **Type**: Read-only, non-destructive
- **External Access**: Fetches from GitHub registry

## Actions

### List Components

Get all available Magic UI components.

**Arguments:**
```json
{
  "action": "list"
}
```

**Returns:** Array of all 74+ animated components with names, titles, and descriptions.

**Example:**
```bash
echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "magic_ui", "arguments": {"action": "list"}}}' | ENABLE_ADDITIONAL_TOOLS=magic_ui ./bin/mcp-devtools stdio
```

### Search Components

Search for components by keyword in name, title, or description.

**Arguments:**
```json
{
  "action": "search",
  "query": "text"
}
```

**Returns:** Array of matching components.

**Example:**
```bash
echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "magic_ui", "arguments": {"action": "search", "query": "text"}}}' | ENABLE_ADDITIONAL_TOOLS=magic_ui ./bin/mcp-devtools stdio
```

### Get Component Details

Get detailed information about a specific component including dependencies and file paths.

**Arguments:**
```json
{
  "action": "details",
  "componentName": "marquee"
}
```

**Returns:** Component details with dependencies, files, and documentation URL.

**Example:**
```bash
echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "magic_ui", "arguments": {"action": "details", "componentName": "marquee"}}}' | ENABLE_ADDITIONAL_TOOLS=magic_ui ./bin/mcp-devtools stdio
```

## Component Categories

Magic UI components fall into these categories:

- **Text Animations**: aurora-text, morphing-text, line-shadow-text
- **Background Effects**: warp-background, grid-pattern, dot-pattern
- **Card Components**: magic-card, neon-gradient-card, bento-grid
- **Interactive Elements**: lens, pointer, smooth-cursor
- **Particle Effects**: particles, meteors, flickering-grid
- **UI Components**: shimmer-button, globe, marquee, scroll-progress
- **Media**: hero-video-dialog
- **Utility**: code-comparison, tweet-card

## Installation

Components can be installed using the Magic UI CLI:

```bash
npx magicui-cli add [component-name]
```

## Common Dependencies

Most Magic UI components depend on:
- `motion` (framer-motion) - for animations
- `tw-animate-css` - Tailwind CSS animations

## Caching

- **Registry cache**: 24 hours
- **Component details cache**: 24 hours

The tool caches the component registry and individual component details to minimise network requests.

## Use Cases

1. **Discover animated components** for marketing sites and portfolios
2. **Find micro-interactions** to enhance user experience
3. **Check component dependencies** before installation
4. **Browse component library** without leaving your development environment

## Comparison with shadcn Tool

| Feature | Magic UI | shadcn/ui |
|---------|----------|-----------|
| Data Source | JSON registry | Web scraping |
| Components | 74+ animated | 50+ base UI |
| Focus | Animations, effects | UI primitives |
| Dependencies | Framer Motion | Radix UI |
| Speed | Fast (JSON) | Slower (scraping) |
| Reliability | High | Medium |

## Extended Help

The tool provides extended help with examples, common patterns, and troubleshooting tips:

```json
{
  "name": "get_tool_help",
  "arguments": {
    "tool_name": "magic_ui"
  }
}
```

## Error Handling

- **Component not found**: Returns error if component name doesn't exist
- **Network errors**: Returns error if GitHub registry is unreachable
- **Invalid action**: Returns error for unsupported actions

## Security

The tool uses the security framework to:
- Validate GitHub registry URL
- Scan fetched content for security risks
- Log security warnings when detected

## See Also

- [shadcn Tool](shadcn.md) - shadcn/ui component library
- [Package Documentation](packagedocs.md) - Library documentation lookup
- [Web Fetch](webfetch.md) - General web content fetching

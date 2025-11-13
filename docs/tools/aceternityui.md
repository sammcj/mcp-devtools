# Aceternity UI Tool

Access the Aceternity UI animated component library with 24+ free components built with React, TypeScript, Tailwind CSS, and Framer Motion.

## Overview

The Aceternity UI tool provides programmatic access to Aceternity UI components through a hardcoded component registry. Unlike web scraping approaches, this tool uses curated component data for fast, reliable access to component information, installation commands, and dependencies.

## Tool Information

- **Name**: `aceternity_ui`
- **Availability**: Disabled by default (requires `ENABLE_ADDITIONAL_TOOLS=aceternity_ui`)
- **Type**: Read-only, non-destructive
- **External Access**: No (uses hardcoded component data)

## Actions

### List Components

Get all available Aceternity UI components.

**Arguments:**
```json
{
  "action": "list"
}
```

**Returns:** Array of all 24+ animated components with names, descriptions, categories, and installation commands.

**Example:**
```bash
echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "aceternity_ui", "arguments": {"action": "list"}}}' | ENABLE_ADDITIONAL_TOOLS=aceternity_ui ./bin/mcp-devtools stdio
```

### Search Components

Search for components by keyword in name, description, or tags.

**Arguments:**
```json
{
  "action": "search",
  "query": "background"
}
```

**Returns:** Array of matching components.

**Example:**
```bash
echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "aceternity_ui", "arguments": {"action": "search", "query": "background"}}}' | ENABLE_ADDITIONAL_TOOLS=aceternity_ui ./bin/mcp-devtools stdio
```

### Get Component Details

Get detailed information about a specific component including installation command and dependencies.

**Arguments:**
```json
{
  "action": "details",
  "componentName": "bento-grid"
}
```

**Returns:** Component details with install command, dependencies, tags, and documentation URL.

**Example:**
```bash
echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "aceternity_ui", "arguments": {"action": "details", "componentName": "bento-grid"}}}' | ENABLE_ADDITIONAL_TOOLS=aceternity_ui ./bin/mcp-devtools stdio
```

### List Categories

Get all component categories with their descriptions and component lists.

**Arguments:**
```json
{
  "action": "categories"
}
```

**Returns:** Array of categories with component lists.

**Example:**
```bash
echo '{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "aceternity_ui", "arguments": {"action": "categories"}}}' | ENABLE_ADDITIONAL_TOOLS=aceternity_ui ./bin/mcp-devtools stdio
```

## Component Categories

Aceternity UI components are organised into these categories:

- **Layout**: bento-grid, hero, timeline, feature-section, layout-grid, hero-highlight, animated-tooltip
- **Cards**: expandable-cards, focus-cards
- **Form**: file-upload, signup-form
- **Navigation**: floating-dock, resizable-navbar
- **Background**: background-beams, hero-highlight
- **Text**: text-generate-effect, text-hover-effect, typewriter-effect
- **Container**: container-cover, animated-tabs
- **Utilities**: loader, placeholder-and-vanish-input
- **Sidebar**: sidebar
- **Map**: world-map

## Installation

Components can be installed using shadcn's CLI with Aceternity UI's registry URLs:

```bash
npx shadcn@latest add https://ui.aceternity.com/registry/[component-name].json
```

For example:
```bash
npx shadcn@latest add https://ui.aceternity.com/registry/bento-grid.json
```

## Common Dependencies

Most Aceternity UI components depend on:
- `motion` (framer-motion) - for animations
- `tailwindcss` - styling framework
- `clsx` - class name utility
- `tailwind-merge` - Tailwind class merging
- `@tabler/icons-react` - icons (some components)

## Use Cases

1. **Discover animated components** for modern web applications
2. **Find specific effects** like text animations, background beams, parallax
3. **Check component dependencies** before installation
4. **Browse by category** to find layout, form, or navigation components
5. **Get installation commands** without leaving your development environment

## Comparison with Other UI Tools

| Feature | Aceternity UI | Magic UI | shadcn/ui |
|---------|---------------|----------|-----------|
| Data Source | Hardcoded | JSON registry | Web scraping |
| Components | 24+ animated | 74+ animated | 50+ base UI |
| Focus | Visual effects | Micro-interactions | UI primitives |
| Dependencies | Framer Motion | Framer Motion | Radix UI |
| Speed | Instant | Fast | Slower |
| Updates | Manual | Automatic | Automatic |

## Extended Help

The tool provides extended help with examples, common patterns, and troubleshooting tips:

```json
{
  "name": "get_tool_help",
  "arguments": {
    "tool_name": "aceternity_ui"
  }
}
```

## Error Handling

- **Component not found**: Returns error if component name doesn't exist in the hardcoded list
- **Invalid action**: Returns error for unsupported actions
- **Missing parameters**: Returns error with clear guidance on required parameters

## Limitations

- **Component list is hardcoded**: New components from Aceternity UI won't appear until manually added
- **No live data**: Component information may become outdated if Aceternity UI changes
- **Free components only**: Pro components are not included

To update the component list, see the `internal/tools/aceternityui/components_data.go` file.

## See Also

- [Magic UI Tool](magicui.md) - Magic UI component library
- [shadcn Tool](shadcn.md) - shadcn/ui component library
- [Package Documentation](packagedocs.md) - Library documentation lookup

# MCP DevTools API Documentation

This directory contains auto-generated API documentation for all MCP tools provided by MCP DevTools.

## Overview

The API documentation is automatically generated from the Go source code using markdown templates. This ensures the documentation stays in sync with the actual tool implementations.

## Documentation Structure

**Generated Files (after running `make generate-api-docs`):**
- **tool-registry.md** - Complete list of all available tools
- **parameter-types.md** - Common parameter types and formats
- **Individual tool files** - Detailed reference for each tool (e.g., internet_search.md)

**Template Files:**
- **[templates/tool-registry.md.tmpl](templates/tool-registry.md.tmpl)** - Registry template
- **[templates/tool-reference.md.tmpl](templates/tool-reference.md.tmpl)** - Individual tool template
- **[templates/parameter-types.md.tmpl](templates/parameter-types.md.tmpl)** - Parameter types template

## Tool Categories

### Search & Discovery
- `internet_search` - Multi-provider web search
- `search_packages` - Package version checking
- `resolve_library_id` - Library identification
- `get_library_docs` - Library documentation

### Document Processing
- `process_document` - Advanced document conversion
- `pdf` - PDF text and image extraction

### Web & Network
- `fetch_url` - Web content retrieval

### Intelligence & Memory
- `think` - Structured reasoning
- `memory` - Knowledge graph operations

### UI & Utilities
- `shadcn` - ShadCN UI component information
- `murican_to_english` - Text conversion

## Usage Examples

Each tool includes:
- **Purpose**: What the tool does
- **Parameters**: Required and optional parameters
- **Examples**: Common usage patterns
- **Response**: Expected output format
- **Errors**: Possible error conditions

## Generating Documentation

To regenerate this documentation:

```bash
# Generate all API documentation
make generate-api-docs

# Generate specific tool documentation
make generate-tool-docs TOOL=internet_search
```

## Integration

This API documentation is designed for:
- **MCP Client Developers** - Understanding available tools
- **Tool Contributors** - Reference for implementing new tools
- **Integration Partners** - API contracts and expectations

---

**Note**: This documentation is auto-generated. Do not edit manually. Changes should be made to the source code and templates.
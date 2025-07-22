# MCP DevTools - Tool Reference

MCP DevTools provides a comprehensive suite of developer tools through a single binary. All tools are designed to work across macOS and Linux environments.

## Quick Reference

### Example Workflows

#### Document Analysis Workflow
```
1. Document Processing → Extract text and structure
2. Think → Analyse content and identify key points
3. Memory → Store important entities and relationships
```

#### Package Management Workflow
```
1. Package Search → Find latest versions
2. Package Documentation → Get usage examples
3. Think → Plan integration approach
```

#### Research Workflow
```
1. Internet Search → Find relevant information
2. Web Fetch → Retrieve detailed content
3. Memory → Store findings for later reference
```

## Tool Dependencies

### Required Environment Variables
| Tool                      | Variable           | Purpose                      |
|---------------------------|--------------------|------------------------------|
| Internet Search (Brave)   | `BRAVE_API_KEY`    | Enable Brave search provider |
| Internet Search (SearXNG) | `SEARXNG_BASE_URL` | Enable SearXNG provider      |
| Document Processing       | `DOCLING_*`        | Configure processing options |
| Memory                    | `MEMORY_FILE_PATH` | Set storage location         |

### Optional Dependencies
- **Python 3.10+**: Required for Document Processing tool
- **Docling**: Auto-installed by Document Processing tool
- **Hardware Acceleration**: MPS (macOS), CUDA (NVIDIA), or CPU

## Configuration Quick Start

### Minimal Configuration (Most Tools Available)
```json
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "/path/to/mcp-devtools"
    }
  }
}
```

### Full Configuration (All Tools)
```json
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "/path/to/mcp-devtools",
      "env": {
        "BRAVE_API_KEY": "your-brave-api-key",
        "SEARXNG_BASE_URL": "https://your-searxng-instance.com",
        "MEMORY_FILE_PATH": "~/.mcp-devtools/",
        "DOCLING_CACHE_ENABLED": "true"
      }
    }
  }
}
```

## Tool Selection Guide

### Choose Based on Your Needs

**For Document Work:**
- Single documents → PDF Processing
- Complex documents → Document Processing
- Research papers → Document Processing + Memory

**For Development:**
- Package management → Package Search + Package Documentation
- Code research → Internet Search + Web Fetch
- Architecture planning → Think + Memory
- File operations → Filesystem + Think

**For File Management:**
- File operations → Filesystem
- Project setup → Filesystem + Package Search
- Code analysis → Filesystem + Think

**For Content Creation:**
- Research → Internet Search + Web Fetch + Memory
- Analysis → Think + Document Processing
- UI work → ShadCN UI + Package Search

## Getting Help

- **Tool-specific documentation**: Click tool names above for detailed guides
- **Development**: See [Creating New Tools](../creating-new-tools.md)
- **Issues**: Report problems on the [GitHub repository](https://github.com/sammcj/mcp-devtools/issues)

---

**Tip**: Start with the minimal configuration and add API keys as needed. Most tools work without any setup!

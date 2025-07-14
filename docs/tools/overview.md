# MCP DevTools - Tool Reference

MCP DevTools provides a comprehensive suite of developer tools through a single binary. All tools are designed to work across macOS and Linux environments.

## Tool Categories

### 🔍 Search & Discovery
| Tool | Purpose | Dependencies | Key Features |
|------|---------|--------------|--------------|
| **[Internet Search](internet-search.md)** | Search the web, images, news, videos | Brave API key (optional), SearXNG (optional) | Multi-provider support, fresh results |
| **[Package Search](package-search.md)** | Find packages across ecosystems | None | NPM, Python, Go, Java, Swift, Docker, GitHub Actions |
| **[Package Documentation](package-documentation.md)** | Retrieve library documentation | None | Context7 integration, focused topic search |

### 📄 Document Processing  
| Tool | Purpose | Dependencies | Key Features |
|------|---------|--------------|--------------|
| **[Document Processing](document-processing.md)** | Convert documents to Markdown | Python 3.10+, Docling | PDF, DOCX, images, OCR, diagram analysis |
| **[PDF Processing](pdf-processing.md)** | Extract text and images from PDFs | None | Fast extraction, image export, page ranges |

### 🧠 Intelligence & Memory
| Tool | Purpose | Dependencies | Key Features |
|------|---------|--------------|--------------|
| **[Think](think.md)** | Structured reasoning space | None | Complex problem analysis, decision support |
| **[Memory](memory.md)** | Persistent knowledge graphs | None | Entity-relation storage, fuzzy search |

### 🎨 UI Components
| Tool | Purpose | Dependencies | Key Features |
|------|---------|--------------|--------------|
| **[ShadCN UI](shadcn-ui.md)** | UI component information | None | Component details, usage examples |

### 🌐 Web & Network
| Tool | Purpose | Dependencies | Key Features |
|------|---------|--------------|--------------|
| **[Web Fetch](web-fetch.md)** | Fetch and convert web content | None | Markdown conversion, pagination |

### 🔧 Utilities
| Tool | Purpose | Dependencies | Key Features |
|------|---------|--------------|--------------|
| **[American to English](american-to-english.md)** | Convert US to British spelling | None | File processing, inline conversion |

## Quick Reference

### Most Popular Tools
1. **Document Processing** - Convert any document to structured Markdown
2. **Internet Search** - Search multiple providers from one interface
3. **Package Search** - Check versions across all major package managers
4. **Think** - Structured reasoning for complex problems

### Common Workflows

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
| Tool | Variable | Purpose |
|------|----------|---------|
| Internet Search (Brave) | `BRAVE_API_KEY` | Enable Brave search provider |
| Internet Search (SearXNG) | `SEARXNG_BASE_URL` | Enable SearXNG provider |
| Document Processing | `DOCLING_*` | Configure processing options |
| Memory | `MEMORY_FILE_PATH` | Set storage location |

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

**For Content Creation:**
- Research → Internet Search + Web Fetch + Memory
- Analysis → Think + Document Processing
- UI work → ShadCN UI + Package Search

## Performance Characteristics

### Fast Tools (< 1 second)
- Think, Memory, ShadCN UI, Package Search, Web Fetch

### Moderate Tools (1-10 seconds)  
- Internet Search, Package Documentation, PDF Processing

### Intensive Tools (10+ seconds)
- Document Processing (depends on document size and processing mode)

## Getting Help

- **Tool-specific documentation**: Click tool names above for detailed guides
- **Development**: See [Creating New Tools](../development/creating-new-tools.md)
- **Issues**: Report problems on the [GitHub repository](https://github.com/sammcj/mcp-devtools/issues)

---

**Tip**: Start with the minimal configuration and add API keys as needed. Most tools work without any setup!
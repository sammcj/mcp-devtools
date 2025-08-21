# MCP DevTools - Tool Reference

MCP DevTools provides a comprehensive suite of developer tools through a single binary. All tools are designed to work on both macOS and Linux environments.

Each tool has it's own documentation in this directory, detailing its purpose, actions, parameters, and usage examples.

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

#### Release Management Workflow
```
1. Generate Changelog → Create release documentation
2. SBOM Generation → Analyse dependencies
3. Vulnerability Scan → Check security issues
```

#### Research Workflow
```
1. Internet Search → Find relevant information
2. Web Fetch → Retrieve detailed content
3. Memory → Store findings for later reference
```

#### API Integration Workflow
```
1. Configure APIs in ~/.mcp-devtools/apis.yaml
2. Dynamic API tools automatically available (e.g., github_api, slack_api)
3. Use endpoints via unified parameter interface
4. Security framework validates all requests automatically
```

#### AWS Documentation Research Workflow
```
1. AWS Documentation (search) → Find relevant AWS guides
2. AWS Documentation (fetch) → Get detailed AWS content
3. Package Documentation → Access AWS Strands Agents SDK docs via resolve_library_id + get_library_docs
4. AWS Documentation (recommend) → Discover related AWS services
5. Memory → Store AWS configuration patterns
```

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
        "ENABLE_ADDITIONAL_TOOLS": "aws,web_fetch,internet_search,think,memory,filesystem,shadcn_ui,security,claude-agent,gemini-agent,q-developer-agent,brave_local_search,brave_video_search,pdf,process_document",
        "GOOGLE_CLOUD_PROJECT": "gemini-code-assist-123456",
        "BRAVE_API_KEY": "abc123",
        "SEARXNG_BASE_URL": "https://searxng.your.domain",
        "DOCLING_VLM_MODEL": "qwen2.5vl:7b-q8_0",
        "DOCLING_VLM_API_KEY": "ollama",
        "DOCLING_VLM_API_URL": "https://ollama.your.domain/v1",
        "GITHUB_TOKEN": "github_pat_READ_ONLY_GITHUB_TOKEN"
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

- **Extended tool information**: Use [Get DevTools Tool Info](devtools_help.md) for detailed usage examples and troubleshooting
- **Tool-specific documentation**: Click tool names above for detailed guides
- **Development**: See [Creating New Tools](../creating-new-tools.md)
- **Issues**: Report problems on the [GitHub repository](https://github.com/sammcj/mcp-devtools/issues)

---

**Tip**: Start with the minimal configuration and add API keys as needed. Most tools work without any setup!

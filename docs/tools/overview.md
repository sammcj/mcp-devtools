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
```

#### Terraform Documentation Research Workflow
```
1. Terraform Documentation (search_providers) → Find provider resources
2. Terraform Documentation (get_provider_details) → Get detailed provider documentation
3. Terraform Documentation (search_modules) → Find relevant modules
4. Terraform Documentation (get_module_details) → Get module configuration examples
5. Memory → Store Terraform configuration patterns
```

#### Complex Problem Solving Workflow
```
1. Sequential Thinking → Break down problem systematically
2. Internet Search / Web Fetch → Gather relevant information
3. Sequential Thinking → Process findings and revise approach
4. Think → Deep analysis of solution options
5. Sequential Thinking → Finalise solution with confidence
```

#### Codebase Analysis Workflow
```
1. Code Skim → Strip implementation details from files/directories
2. Think → Analyse code structure and architecture
3. Code Skim (pagination) → Process large files in chunks
4. Memory → Store architectural patterns and key components
```

#### Excel Data Analysis Workflow
```
1. Excel (create_workbook) → Create new workbook
2. Excel (write_data) → Populate with data
3. Excel (format_range) → Apply formatting and conditional formatting
4. Excel (create_chart) → Add visualisations
5. Excel (create_pivot_table) → Generate analysis summaries
6. Excel (create_table) → Convert data to structured tables
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
        "ENABLE_ADDITIONAL_TOOLS": "aws_documentation,fetch_url,internet_search,think,memory,filesystem,shadcn_ui,magic_ui,aceternity_ui,security,claude-agent,codex-agent,copilot-agent,gemini-agent,q-developer-agent,brave_local_search,brave_video_search,pdf,process_document,sequential-thinking,excel,find_long_files,code_skim",
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

**For Data Analysis:**
- Spreadsheet creation → Excel
- Data visualisation → Excel (charts and conditional formatting)
- Data summarisation → Excel (pivot tables)
- Structured data → Excel (tables with formatting)

**For Development:**
- Package management → Package Search + Package Documentation
- Code research → Internet Search + Web Fetch
- Codebase exploration → Code Skim + Think
- Architecture planning → Sequential Thinking + Think + Memory
- Complex debugging → Sequential Thinking + Internet Search
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

- **Extended tool information**: Use [Get DevTools Tool Info](get_tool_help.md) for detailed usage examples and troubleshooting
- **Tool-specific documentation**: Click tool names above for detailed guides
- **Development**: See [Creating New Tools](../creating-new-tools.md)
- **Issues**: Report problems on the [GitHub repository](https://github.com/sammcj/mcp-devtools/issues)

---

**Tip**: Start with the minimal configuration and add API keys as needed. Most tools work without any setup!

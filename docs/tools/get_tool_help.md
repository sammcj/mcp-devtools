# DevTools Help

The DevTools Help tool provides detailed usage information, examples, and troubleshooting guidance for MCP DevTools server tools, helping AI agents understand complex tool usage beyond basic definitions.

## Overview

This tool addresses the challenge of providing comprehensive tool documentation without bloating the core tool definitions. It enables on-demand access to detailed usage information, examples, and troubleshooting tips specifically for tools available in the current MCP DevTools server instance.

## Purpose

While standard MCP tool definitions provide basic parameter information, some tools are complex enough to benefit from additional guidance. The Get DevTools Tool Info tool:

- **Provides context-specific help** when standard definitions are insufficient
- **Includes practical examples** showing real usage patterns
- **Offers troubleshooting guidance** for common issues
- **Explains when and when not** to use specific tools

## Key Features

- **Scoped Information**: Only provides information about MCP DevTools server tools
- **Extended Examples**: Real-world usage patterns with expected outcomes
- **Troubleshooting Guide**: Common problems and solutions
- **Parameter Details**: Additional context beyond basic schema definitions
- **Usage Guidelines**: Clear guidance on appropriate tool usage
- **Dynamic Tool Discovery**: Automatically reflects currently enabled tools

## When to Use This Tool

### ✅ Use When

- **Tool call failures**: Parameter validation errors or unclear error messages
- **Complex parameters**: Need clarification on expected input formats
- **Usage patterns**: Want to see concrete examples of tool usage
- **Troubleshooting**: Encountering issues with specific tools
- **Best practices**: Need guidance on optimal tool usage

### ❌ Don't Use When

- **Routine operations**: Basic tool definitions are sufficient for standard usage
- **Tool discovery**: Use standard tool listing instead
- **Every tool call**: Wastes context and reduces efficiency
- **General queries**: Not for tools outside MCP DevTools server

## Usage Examples

### Basic Tool Information Request

```json
{
  "name": "get_tool_help",
  "arguments": {
    "tool_name": "internet_search"
  }
}
```

**Response includes:**
- Basic tool definition and parameters
- Extended usage examples with different search types (when available)
- Troubleshooting tips for API key issues
- Parameter details for provider-specific options

### Troubleshooting a Complex Tool

```json
{
  "name": "get_tool_help",
  "arguments": {
    "tool_name": "document_processing"
  }
}
```

**Helps with:**
- Understanding different processing profiles
- Configuring hardware acceleration options
- Handling large document processing
- Troubleshooting Python dependency issues

### Quick Parameter Reference

```json
{
  "name": "get_tool_help",
  "arguments": {
    "tool_name": "package_search"
  }
}
```

**Provides:**
- Parameter details and usage examples (when available)
- When to use different ecosystems
- Common troubleshooting scenarios

## Tool Workflow Integration

### Problem-Solving Pattern

```bash
# 1. Encounter tool usage issue
package_search --ecosystem "npm" --query "malformed-query-here"
# Error: Invalid query format

# 2. Get detailed information
get_tool_help --tool_name "package_search"

# 3. Apply learned patterns
package_search --ecosystem "npm" --query "react"
```

### Complex Tool Usage Pattern

```bash
# 1. Check tool capabilities before using
get_tool_help --tool_name "document_processing"

# 2. Use tool with proper configuration
document_processing --source "document.pdf" --profile "llm"

# 3. If issues arise, check troubleshooting
get_tool_help --tool_name "document_processing" --include_examples false
```

## Response Format

The tool returns structured JSON containing:

```json
{
  "tool_name": "example_tool",
  "basic_info": {
    "name": "example_tool",
    "description": "Tool description from definition",
    "input_schema": {...}
  },
  "extended_info": {
    "examples": [
      {
        "description": "Basic usage example",
        "arguments": {"param": "value"},
        "expected_result": "Success response format"
      }
    ],
    "common_patterns": [
      "Pattern 1: Use with specific parameter combinations",
      "Pattern 2: Typical workflow integration"
    ],
    "troubleshooting": [
      {
        "problem": "Tool returns validation error",
        "solution": "Check parameter format and required fields"
      }
    ],
    "parameter_details": {
      "param_name": "Additional context about parameter usage"
    },
    "when_to_use": "Use this tool when you need to...",
    "when_not_to_use": "Avoid using this tool for..."
  },
  "has_extended_info": true
}
```

## Extended Information Coverage

Not all tools provide extended information. The response indicates availability:

### Tools with Extended Info
Tools that implement the `ExtendedHelpProvider` interface provide comprehensive details including examples, troubleshooting, and usage patterns.

### Tools without Extended Info
Tools that only implement the basic `Tool` interface return:
```json
{
  "tool_name": "simple_tool",
  "basic_info": {...},
  "extended_info": null,
  "has_extended_info": false,
  "message": "Tool 'simple_tool' does not provide extended information beyond the standard definition"
}
```

## Available Tools

The tool automatically discovers and validates against currently enabled tools in the MCP DevTools server. The tool description and parameter enum update dynamically based on:

- Tools registered in the server
- Tools not disabled via `DISABLED_TOOLS` environment variable

## Best Practices

### Efficient Usage

**Targeted Queries**: Request specific tools experiencing issues
```json
// ✅ Good - specific tool with clear purpose
{"tool_name": "github"}

// ❌ Avoid - routine information gathering when basic usage is already clear
{"tool_name": "web_fetch"}
```

**Focus on Complex Tools**: Use for tools that need clarification
```json
// ✅ Learning - when exploring complex tools
{"tool_name": "memory"}

// ✅ Troubleshooting - when encountering tool usage issues
{"tool_name": "internet_search"}
```

### Integration Patterns

**Error Recovery**:
```bash
# When tool calls fail
internet_search --query "complex query" --type "invalid_type"
# Error: invalid search type

# Get help and retry
get_tool_help --tool_name "internet_search"
internet_search --query "complex query" --type "web"
```

**Complex Tool Exploration**:
```bash
# Before using unfamiliar complex tools
get_tool_help --tool_name "document_processing"

# Apply learned configuration
document_processing --source "file.pdf" --profile "llm" --hardware_acceleration "auto"
```

## Security Considerations

- **Information Scope**: Only provides information about MCP DevTools server tools
- **No Sensitive Data**: Extended information should not contain API keys or credentials
- **Validation**: Tool names are validated against enabled tools only
- **Resource Protection**: Bounded response sizes prevent resource exhaustion

## Performance Impact

- **Minimal Overhead**: Information is generated on-demand from registered tools
- **No External Calls**: All information comes from local tool registry
- **Efficient Lookup**: Direct registry queries with O(1) tool access
- **Bounded Responses**: Structured format prevents unbounded output

## Error Handling

### Invalid Tool Names
```json
{
  "error": "tool 'nonexistent_tool' not found or disabled. Available tools: think, web_fetch, internet_search, ..."
}
```

### Missing Parameters
```json
{
  "error": "missing or invalid required parameter: tool_name"
}
```

### Disabled Tools
```json
{
  "error": "tool 'disabled_tool' not found or disabled. Available tools: ..."
}
```

## Implementation for Tool Developers

Tools can optionally implement the `ExtendedHelpProvider` interface to provide detailed information:

```go
func (t *YourTool) ProvideExtendedInfo() *tools.ExtendedInfo {
    return &tools.ExtendedInfo{
        Examples: []tools.ToolExample{
            {
                Description: "Basic usage",
                Arguments: map[string]interface{}{"param": "value"},
                ExpectedResult: "Success response",
            },
        },
        WhenToUse: "Use when you need to...",
        WhenNotToUse: "Avoid when...",
        // ... additional fields
    }
}
```

## Configuration

No additional configuration required. The tool automatically:

- Discovers enabled tools from the registry
- Respects `DISABLED_TOOLS` environment variable
- Updates tool lists dynamically
- Validates requests against current tool availability

---

For technical implementation details, see the [tool source code](../../internal/tools/utilities/toolhelp).

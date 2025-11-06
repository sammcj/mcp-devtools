# Creating New Tools

The MCP DevTools server is designed to be easily extensible with new tools. This section provides detailed guidance on how to create and integrate new tools into the server.

- [Creating New Tools](#creating-new-tools)
  - [Tool Interface](#tool-interface)
  - [Tool Structure](#tool-structure)
  - [Step-by-Step Guide](#step-by-step-guide)
    - [1. Create a New Package](#1-create-a-new-package)
    - [2. Implement the Tool Interface](#2-implement-the-tool-interface)
    - [4. Result Schema](#4-result-schema)
    - [5. Caching](#5-caching)
    - [6. Security Integration](#6-security-integration)
    - [7. Register the Tool for Import](#7-register-the-tool-for-import)
  - [Example: Hello World Tool](#example-hello-world-tool)
    - [Testing Your Tool](#testing-your-tool)
  - [Testing](#testing)
  - [Extended Help for Complex Tools](#extended-help-for-complex-tools)
    - [Implementing Extended Help](#implementing-extended-help)
    - [Extended Help Structure](#extended-help-structure)
    - [When to Add Extended Help](#when-to-add-extended-help)
    - [Extended Help Benefits](#extended-help-benefits)
  - [Tool Annotations](#tool-annotations)
    - [Annotation Types](#annotation-types)
    - [Tool Categories](#tool-categories)
  - [Tool Error Logging](#tool-error-logging)
  - [Additional Considerations](#additional-considerations)

## Tool Interface

All tools must implement the `tools.Tool` interface defined in `internal/tools/tools.go`:

```go
type Tool interface {
    // Definition returns the tool's definition for MCP registration
    Definition() mcp.Tool

    // Execute executes the tool's logic
    Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error)
}
```

## Tool Structure

A typical tool implementation follows this structure:

1. **Tool Type**: Define a struct that will implement the Tool interface
2. **Registration**: Register the tool with the registry in an `init()` function
3. **Definition**: Implement the `Definition()` method to define the tool's name, description, and parameters
4. **Execution**: Implement the `Execute()` method to perform the tool's logic

## Step-by-Step Guide

### 1. Create a New Package

Create a new package in the appropriate category under `internal/tools/` or create a new category if needed:

```bash
mkdir -p internal/tools/your-category/your-tool
touch internal/tools/your-category/your-tool/your-tool.go
```

### 2. Implement the Tool Interface

Here's a template for implementing a new tool:

```go
package yourtool

import (
    "context"
    "fmt"
    "sync"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/sammcj/mcp-devtools/internal/registry"
    "github.com/sammcj/mcp-devtools/internal/security"
    "github.com/sirupsen/logrus"
)

// YourTool implements the tools.Tool interface
type YourTool struct {
    // Add any fields your tool needs here
}

// init registers the tool with the registry
func init() {
    registry.Register(&YourTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *YourTool) Definition() mcp.Tool {
    return mcp.NewTool(
        "your_tool_name",
        mcp.WithDescription("Description of your tool"),
        // Define required parameters
        mcp.WithString("param1",
            mcp.Required(),
            mcp.Description("Description of param1"),
        ),
        // Define optional parameters
        mcp.WithNumber("param2",
            mcp.Description("Description of param2"),
            mcp.DefaultNumber(10),
        ),
        // Add more parameters as needed
    )
}

// Execute executes the tool's logic
func (t *YourTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
    // Log the start of execution
    logger.Info("Executing your tool")

    // Parse parameters
    param1, ok := args["param1"].(string)
    if !ok {
        return nil, fmt.Errorf("missing required parameter: param1")
    }

    // Parse optional parameters with defaults
    param2 := float64(10)
    if param2Raw, ok := args["param2"].(float64); ok {
        param2 = param2Raw
    }

    // SECURITY INTEGRATION: Check file access if your tool reads files
    if needsFileAccess {
        if err := security.CheckFileAccess(filePath); err != nil {
            return nil, err
        }
    }

    // SECURITY INTEGRATION: Check domain access if your tool makes HTTP requests
    if needsDomainAccess {
        if err := security.CheckDomainAccess(domain); err != nil {
            return nil, err
        }
    }

    // Implement your tool's logic here
    content := "fetched or processed content"

    // SECURITY INTEGRATION: Analyse content for security risks
    source := security.SourceContext{
        Tool:   "your_tool_name",
        Domain: domain,        // for HTTP content
        Source: filePath,      // for file content
        Type:   "content_type",
    }
    if result, err := security.AnalyseContent(content, source); err == nil {
        switch result.Action {
        case security.ActionBlock:
            return nil, fmt.Errorf("content blocked by security policy: %s", result.Message)
        case security.ActionWarn:
            logger.WithField("security_id", result.ID).Warn(result.Message)
        }
    }

    result := map[string]interface{}{
        "message": fmt.Sprintf("Tool executed with param1=%s, param2=%f", param1, param2),
        "content": content,
        // Add more result fields as needed
    }

    // Return the result
    return mcp.NewCallToolResult(result), nil
}
```

#### 3. Parameter Schema

The MCP framework supports various parameter types:

- **String**: `mcp.WithString("name", ...)`
- **Number**: `mcp.WithNumber("name", ...)`
- **Boolean**: `mcp.WithBoolean("name", ...)`
- **Array**: `mcp.WithArray("name", ...)`
- **Object**: `mcp.WithObject("name", ...)`

For each parameter, you can specify:

- **Required**: `mcp.Required()` - Mark the parameter as required
- **Description**: `mcp.Description("...")` - Provide a description
- **Default Value**: `mcp.DefaultString("...")`, `mcp.DefaultNumber(10)`, `mcp.DefaultBool(false)` - Set a default value
- **Enum**: `mcp.Enum("value1", "value2", ...)` - Restrict to a set of values
- **Properties**: `mcp.Properties(map[string]interface{}{...})` - Define properties for object parameters

### 4. Result Schema

The result of a tool execution should be a `*mcp.CallToolResult` object, which can be created with:

```go
mcp.NewCallToolResult(result)
```

Where `result` is a `map[string]interface{}` containing the tool's output data.

For structured results, you can use:

```go
// Define a result struct
type Result struct {
    Message string `json:"message"`
    Count   int    `json:"count"`
}

// Create a result
result := Result{
    Message: "Tool executed successfully",
    Count:   42,
}

// Convert to JSON
resultJSON, err := json.Marshal(result)
if err != nil {
    return nil, fmt.Errorf("failed to marshal result: %w", err)
}

// Create a CallToolResult
return mcp.NewCallToolResultJSON(resultJSON)
```

### 5. Caching

The `cache` parameter in the `Execute` method is a shared cache that can be used to store and retrieve data across tool executions:

```go
// Store a value in the cache
cache.Store("key", value)

// Retrieve a value from the cache
if cachedValue, ok := cache.Load("key"); ok {
    // Use cachedValue
}
```

### 6. Security Integration

**IMPORTANT**: All tools that access files or make HTTP requests MUST integrate with the security system. This provides protection against malicious content and unauthorized access.

#### Recommended: Security Helper Functions

The preferred approach is to use security helper functions that provide simplified APIs with automatic security integration and content integrity preservation.

**For HTTP operations:**
```go
// Create operations instance for your tool
ops := security.NewOperations("your_tool_name")

// Secure HTTP GET with content integrity preservation
safeResp, err := ops.SafeHTTPGet(urlStr)
if err != nil {
    // Handle security blocks or network errors
    if secErr, ok := err.(*security.SecurityError); ok {
        return nil, security.FormatSecurityBlockError(secErr)
    }
    return nil, err
}

// Content is EXACT bytes from server
content := safeResp.Content

// Check for security warnings (non-blocking)
if safeResp.SecurityResult != nil && safeResp.SecurityResult.Action == security.ActionWarn {
    logger.Warnf("Security warning [ID: %s]: %s", safeResp.SecurityResult.ID, safeResp.SecurityResult.Message)
    // Content is still available despite warning
}

// Process exact content
return processContent(content)
```

**For file operations:**
```go
ops := security.NewOperations("your_tool_name")

// Secure file read with content integrity preservation
safeFile, err := ops.SafeFileRead(filePath)
if err != nil {
    // Handle security blocks or file errors
    if secErr, ok := err.(*security.SecurityError); ok {
        return nil, security.FormatSecurityBlockError(secErr)
    }
    return nil, err
}

// Content is EXACT file bytes
content := safeFile.Content

// Handle security warnings if present
if safeFile.SecurityResult != nil && safeFile.SecurityResult.Action == security.ActionWarn {
    logger.Warnf("Security warning [ID: %s]: %s", safeFile.SecurityResult.ID, safeFile.SecurityResult.Message)
}

return processContent(content)
```

#### Helper Functions Benefits

- **80-90% Boilerplate Reduction**: From 30+ lines to 5-10 lines
- **Content Integrity**: Guaranteed exact byte preservation
- **Security Compliance**: Automatic integration with security framework
- **Error Handling**: Consistent security error patterns
- **Performance**: Same security guarantees with simpler code

#### Alternative: Manual Security Integration

For tools requiring fine-grained control, you can manually integrate with the security system:

**File Access Security:**
```go
// Before any file operation
if err := security.CheckFileAccess(filePath); err != nil {
    return nil, err  // Access denied by security policy
}
```

**Domain Access Security:**
```go
// Before making HTTP requests
if err := security.CheckDomainAccess(domain); err != nil {
    return nil, err  // Domain blocked by security policy
}
```

**Content Analysis Security:**
```go
// After fetching/processing content
source := security.SourceContext{
    Tool:   "your_tool_name",
    Domain: domain,        // for HTTP content
    Source: filePath,      // for file content
    Type:   "content_type", // e.g., "web_content", "file_content", "api_response"
}

if result, err := security.AnalyseContent(content, source); err == nil {
    switch result.Action {
    case security.ActionBlock:
        return nil, fmt.Errorf("content blocked by security policy: %s", result.Message)
    case security.ActionWarn:
        logger.WithField("security_id", result.ID).Warn(result.Message)
        // Continue processing but log the warning
    case security.ActionAllow:
        // Content is safe, continue normally
    }
}
```

#### Security Integration Checklist

**For Helper Functions (Recommended):**
- [ ] Import `"github.com/sammcj/mcp-devtools/internal/security"`
- [ ] Create Operations instance: `ops := security.NewOperations("tool_name")`
- [ ] Use `ops.SafeHTTPGet/Post()` for HTTP operations
- [ ] Use `ops.SafeFileRead/Write()` for file operations
- [ ] Handle `SecurityError` in error responses
- [ ] Log security warnings when present
- [ ] Process exact content from response types

**For Manual Integration:**
- [ ] Import `"github.com/sammcj/mcp-devtools/internal/security"`
- [ ] Call `security.CheckFileAccess()` before file operations
- [ ] Call `security.CheckDomainAccess()` before HTTP requests
- [ ] Call `security.AnalyseContent()` for returned content
- [ ] Handle `ActionBlock` by returning an error
- [ ] Handle `ActionWarn` by logging with security ID
- [ ] Provide appropriate `SourceContext` for content analysis

#### Security System Behaviour

- **Disabled by default**: Security checks are no-ops when security is not enabled
- **Graceful degradation**: Tools work normally when security is disabled
- **Override capability**: Blocked content includes security IDs for potential overrides
- **Audit logging**: All security events are logged for review

### 7. Register the Tool for Import

Add your tool package to the imports registry so it gets automatically loaded. Add the import to `internal/imports/tools.go`:

```go
import (
    // ... existing imports ...
    _ "github.com/sammcj/mcp-devtools/internal/tools/your-category/your-tool"
)
```

**Important**: Do NOT add your tool import directly to `main.go`. Use the imports registry system instead to ensure proper build tag handling and maintainability.

## Example: Hello World Tool

Here's a simple "Hello World" tool example:

```go
package hello

import (
    "context"
    "fmt"
    "sync"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/sammcj/mcp-devtools/internal/registry"
    "github.com/sirupsen/logrus"
)

// HelloTool implements a simple hello world tool
type HelloTool struct{}

// init registers the tool with the registry
func init() {
    registry.Register(&HelloTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *HelloTool) Definition() mcp.Tool {
    return mcp.NewTool(
        "hello_world",
        mcp.WithDescription("A simple hello world tool"),
        mcp.WithString("name",
            mcp.Description("Name to greet"),
            mcp.DefaultString("World"),
        ),
    )
}

// Execute executes the tool's logic
func (t *HelloTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
    // Parse parameters
    name := "World"
    if nameRaw, ok := args["name"].(string); ok && nameRaw != "" {
        name = nameRaw
    }

    // Create result
    result := map[string]interface{}{
        "message": fmt.Sprintf("Hello, %s!", name),
    }

    // Return the result
    return mcp.NewCallToolResult(result), nil
}
```

### Testing Your Tool

To test your tool:

1. Build the server: `make build`
2. Run the server: `make run`
3. Send a request to the server:

```json
{
  "name": "your_tool_name",
  "arguments": {
    "param1": "value1",
    "param2": 42
  }
}
```

## Testing

The project includes unit tests for core functionality. Tests are designed to be lightweight and fast, avoiding external dependencies.

```bash
# Run all tests
make test

# Run only fast tests (no external dependencies)
make test-fast
```

## Extended Help for Complex Tools

For tools with complex parameter structures or usage patterns, you can implement the optional `ExtendedHelpProvider` interface to provide detailed usage information accessible through the `get_tool_help` tool.

Note: The extended help is not automatically visible to agents, they have to explicitly call the `get_tool_help` tool to retrieve it.

### Implementing Extended Help

To add extended help to your tool, implement the `ExtendedHelpProvider` interface:

```go
import "github.com/sammcj/mcp-devtools/internal/tools"

// Add the ProvideExtendedInfo method to your tool
func (t *YourTool) ProvideExtendedInfo() *tools.ExtendedHelp {
    return &tools.ExtendedHelp{
        Examples: []tools.ToolExample{
            {
                Description: "Basic usage example",
                Arguments: map[string]interface{}{
                    "param1": "example_value",
                    "param2": 42,
                },
                ExpectedResult: "Description of what this example returns",
            },
            // Add more examples for different use cases
        },
        CommonPatterns: []string{
            "Start with basic parameters before using advanced options",
            "Use parameter X for Y scenario",
            "Combine with other tools for complete workflows",
        },
        Troubleshooting: []tools.TroubleshootingTip{
            {
                Problem:  "Common error or issue users might encounter",
                Solution: "How to resolve this issue step by step",
            },
        },
        ParameterDetails: map[string]string{
            "param1": "Detailed explanation of param1 with examples and constraints",
            "param2": "Advanced usage information for param2 including edge cases",
        },
        WhenToUse:    "Describe when this tool is the right choice",
        WhenNotToUse: "Describe when other tools would be better alternatives",
    }
}
```

### Extended Help Structure

- **Examples**: Provide 3-5 real-world examples showing different usage patterns with expected results
- **CommonPatterns**: List workflow patterns and best practices for using the tool effectively
- **Troubleshooting**: Address common errors and their solutions
- **ParameterDetails**: Explain complex parameters that need more context than the basic description
- **WhenToUse/WhenNotToUse**: Help AI agents and less capable AI models understand appropriate tool selection

### When to Add Extended Help

Consider adding extended help for tools that have:

- Multiple parameter combinations with different behaviours
- Complex parameter structures (nested objects, arrays with specific formats)
- Integration patterns with other tools
- Common error conditions or edge cases
- Context-sensitive behaviour based on available resources/configurations

### Extended Help Benefits

Tools with extended help:

- Appear in the `get_tool_help` tool for discoverability
- Provide rich context for AI agents to use tools more effectively
- Reduce trial-and-error by providing clear examples and patterns
- Prevent common mistakes through proactive troubleshooting guidance

## Tool Annotations

Annotations help MCP clients understand tool behaviour and make informed decisions about tool usage.

### Annotation Types

- **ReadOnlyHint**: Indicates whether a tool modifies its environment
- **DestructiveHint**: Marks tools that may perform destructive operations
- **IdempotentHint**: Shows if repeated calls with same arguments have additional effects
- **OpenWorldHint**: Indicates whether tools interact with external systems

### Tool Categories

**Read-Only Tools** (safe, no side effects):
- Calculator, Internet Search, Web Fetch, Package Documentation, Think Tool
- Annotations: `readOnly: true, destructive: false, openWorld: varies`

**Non-Destructive Writing Tools** (create content, don't destroy):
- Generate Changelog, Document Processing, PDF Processing, Memory Storage
- Annotations: `readOnly: false, destructive: false/true, openWorld: varies`

**Potentially Destructive Tools** (can modify/delete files or execute commands):
- Filesystem Operations, Security Override, Agent Tools (Claude, Codex, Gemini, Q Developer)
- Annotations: `readOnly: false, destructive: true, openWorld: true`
- **Note**: These tools require `ENABLE_ADDITIONAL_TOOLS` environment variable

## Tool Error Logging

MCP DevTools includes an optional tool error logging feature that captures detailed information about failed tool calls. This helps identify patterns in tool failures and improve tool reliability over time.

When enabled, any tool execution that returns an error will be logged to a dedicated log file at `~/.mcp-devtools/logs/tool-errors.log`.

To enable tool error logging, set the `LOG_TOOL_ERRORS` environment variable to `true`

## Additional Considerations

- You must remember to register tools so that MCP clients can discover them.
- Tool descriptions and parameter annotations are important for the AI agents to understand how to use the tools effectively, but must be concise and clear so they don't overload the context.
  - If you want to create a function to help with debugging to testing a tool but don't want to expose it to MCP clients using the server, you can do so, just make sure you add a comment that it is a function not intended to be exposed to MCP clients. For tool descriptions aim to keep them under 450 characters if possible.
- Tool responses should be limited to only include information that is actually useful, there's no point in returning the information an agent provides to call the tool back to them, or any generic information or null / empty fields - these just waste tokens.
- All tools should work on both macOS and Linux unless otherwise specified (we do not need to support Windows).
- Rather than creating lots of tools for one purpose / provider, instead favour creating a single tool with multiple functions and parameters.
- Tools should have fast, concise unit tests that do not rely on external dependencies or services.
- No tool should ever log to stdout or stderr when the MCP server is running in stdio mode as this breaks the MCP protocol.
- Consider if the tool should be enabled or disabled by default, if unsure - make it disabled by default following existing patterns (Don't forget to enable it in tests).
- **CRITICAL for disabled-by-default tools**: To disable a tool by default, add its name to the `additionalTools` list in the `requiresEnablement()` function in `internal/registry/registry.go`. The registry will automatically prevent the tool from being registered unless explicitly enabled via `ENABLE_ADDITIONAL_TOOLS`. No additional checks are needed in the tool's `Execute()` method - the centralised registration system handles all enablement logic.
- You should update docs/tools/overview.md with adding or changing a tool.
- **SECURITY**: All tools that access files or make HTTP requests MUST integrate with the security system. See [Security Integration](#6-security-integration) above and [Security System Documentation](security.md) for details.
- Follow least privilege security principles.

# Creating New Tools

The MCP DevTools server is designed to be easily extensible with new tools. This section provides detailed guidance on how to create and integrate new tools into the server.

- [Creating New Tools](#creating-new-tools)
  - [MCP SDK and the mcpapi package](#mcp-sdk-and-the-mcpapi-package)
  - [Tool Interface](#tool-interface)
  - [Tool Structure](#tool-structure)
  - [Step-by-Step Guide](#step-by-step-guide)
  - [Example: Hello World Tool](#example-hello-world-tool)
  - [Testing](#testing)
  - [Extended Help for Complex Tools](#extended-help-for-complex-tools)
  - [Tool Annotations](#tool-annotations)
  - [Tool Error Logging](#tool-error-logging)
  - [Additional Considerations](#additional-considerations)

## MCP SDK and the mcpapi package

MCP DevTools uses the official [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) and targets MCP spec revision 2026-07-28.

Tools don't use the SDK directly. They use `internal/mcpapi`, a small package that wraps the SDK with the builder style used throughout this codebase (`mcpapi.NewTool`, `mcpapi.WithString`, `mcpapi.NewToolResultText`, and so on). `mcpapi.Tool` and `mcpapi.CallToolResult` are type aliases for the SDK types, so anything the SDK accepts still works if you need it.

Use `mcpapi` for everything a tool definition needs. Reach for the SDK directly only for server-level features (middleware, resources, prompts), which live in `main.go`.

- https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp (SDK reference)
- https://modelcontextprotocol.io/specification/2026-07-28 (spec)

Input schemas are [JSON Schema draft 2020-12](https://github.com/google/jsonschema-go). `mcpapi.NewTool` always produces an object schema, which the SDK requires.

## Tool Interface

All tools must implement the `tools.Tool` interface defined in `internal/tools/tools.go`:

```go
type Tool interface {
    // Definition returns the tool's definition for MCP registration
    Definition() mcpapi.Tool

    // Execute executes the tool's logic
    Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcpapi.CallToolResult, error)
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

    "github.com/sammcj/mcp-devtools/internal/mcpapi"
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
func (t *YourTool) Definition() mcpapi.Tool {
    return mcpapi.NewTool(
        "your_tool_name",
        mcpapi.WithDescription("Description of your tool"),
        // Define required parameters
        mcpapi.WithString("param1",
            mcpapi.Required(),
            mcpapi.Description("Description of param1"),
        ),
        // Define optional parameters
        mcpapi.WithNumber("param2",
            mcpapi.Description("Description of param2"),
            mcpapi.DefaultNumber(10),
        ),
        // Add more parameters as needed
    )
}

// Execute executes the tool's logic
func (t *YourTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcpapi.CallToolResult, error) {
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

    result := map[string]any{
        "message": fmt.Sprintf("Tool executed with param1=%s, param2=%f", param1, param2),
        "content": content,
        // Add more result fields as needed
    }

    // Return the result
    return mcpapi.NewToolResultJSON(result)
}
```

#### 3. Parameter Schema

The MCP framework supports various parameter types:

- **String**: `mcpapi.WithString("name", ...)`
- **Number**: `mcpapi.WithNumber("name", ...)`
- **Boolean**: `mcpapi.WithBoolean("name", ...)`
- **Array**: `mcpapi.WithArray("name", ...)`
- **Object**: `mcpapi.WithObject("name", ...)`

For each parameter, you can specify:

- **Required**: `mcpapi.Required()` - Mark the parameter as required
- **Description**: `mcpapi.Description("...")` - Provide a description
- **Default Value**: `mcpapi.DefaultString("...")`, `mcpapi.DefaultNumber(10)`, `mcpapi.DefaultBool(false)` - Set a default value
- **Enum**: `mcpapi.Enum("value1", "value2", ...)` - Restrict to a set of values
- **Properties**: `mcpapi.Properties(map[string]any{...})` - Define properties for object parameters

### 4. Result Schema

The result of a tool execution is a `*mcpapi.CallToolResult`. Build it with one of:

- `mcpapi.NewToolResultText(text)` - plain text
- `mcpapi.NewToolResultJSON(value)` - marshals any Go value to JSON text, returns `(*CallToolResult, error)`
- `mcpapi.NewToolResultError(message)` - a tool-level error the model can read and act on
- `mcpapi.NewToolResultErrorFromErr(message, err)` - the same, wrapping a Go error

```go
type Result struct {
    Message string `json:"message"`
    Count   int    `json:"count"`
}

return mcpapi.NewToolResultJSON(Result{
    Message: "Tool executed successfully",
    Count:   42,
})
```

**Errors**: returning a non-nil `error` from `Execute` is fine. The server's tool handler converts it into a tool result with `isError: true` before it reaches the client, so a failing tool never surfaces as a JSON-RPC protocol error. Return an error for genuine failures; use `mcpapi.NewToolResultError` when you want to control the wording the model sees.

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
safeResp, err := ops.SafeHTTPGet(ctx, urlStr)
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

    "github.com/sammcj/mcp-devtools/internal/mcpapi"
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
func (t *HelloTool) Definition() mcpapi.Tool {
    return mcpapi.NewTool(
        "hello_world",
        mcpapi.WithDescription("A simple hello world tool"),
        mcpapi.WithString("name",
            mcpapi.Description("Name to greet"),
            mcpapi.DefaultString("World"),
        ),
    )
}

// Execute executes the tool's logic
func (t *HelloTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcpapi.CallToolResult, error) {
    // Parse parameters
    name := "World"
    if nameRaw, ok := args["name"].(string); ok && nameRaw != "" {
        name = nameRaw
    }

    // Create result
    result := map[string]any{
        "message": fmt.Sprintf("Hello, %s!", name),
    }

    // Return the result
    return mcpapi.NewToolResultJSON(result)
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

### Verifying Token Cost

After implementing your tool, verify its context overhead doesn't exceed the configured threshold:

```bash
# Test your tool's token cost (enable it first if disabled by default)
ENABLE_ADDITIONAL_TOOLS=your_tool_name make benchmark-tokens
```

If your tool exceeds the default 800 token threshold, consider:
- Simplifying parameter descriptions (keep under 200 characters)
- Reducing enum values or moving detailed information to Extended Help
- Using more concise parameter names

The benchmark shows description vs parameter token breakdown to help identify optimisation opportunities.

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
                Arguments: map[string]any{
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

If you want to view the tool descriptions, parameters and annotations as a MCP client would see it, you can optionally run `make list-tools`.

## Additional Considerations

- You must remember to register tools so that MCP clients can discover them.
- Tool descriptions should focus on WHAT the tool does. Tool descriptions should be action-oriented and concise. For example:
  - ✅ Good: "Returns source code structure by stripping function/method bodies whilst preserving signatures, types, and declarations."
  - ❌ Poor: "Transform source code by removing implementation details while preserving structure. Achieves 60-80% token reduction for optimising AI context windows"
  - The first describes what the tool does; the second explains why it's useful (which bloats the context unnecessarily)
- Parameter descriptions should be clear and specific about the expected input format and constraints
- Tool descriptions should aim to be under 200 characters where possible; save detailed usage information for Extended Help
  - If you want to create a function to help with debugging to testing a tool but don't want to expose it to MCP clients using the server, you can do so, just make sure you add a comment that it is a function not intended to be exposed to MCP clients.
- Tool responses should be limited to only include information that is actually useful, there's no point in returning the information an agent provides to call the tool back to them, or any generic information or null / empty fields - these just waste tokens.
- All tools should work on both macOS and Linux unless otherwise specified (we do not need to support Windows).
- Rather than creating lots of tools for one purpose / provider, instead favour creating a single tool with multiple functions and parameters.
- Tools should have fast, concise unit tests that do not rely on external dependencies or services.
- No tool should ever log to stdout or stderr when the MCP server is running in stdio mode as this breaks the MCP protocol.
- Tool enablement decision: By default, ALL new tools should be DISABLED by default unless explicitly approved by Sam, if approved tool gets enabled by being added to the `defaultTools` list in the `enabledByDefault()` function in `registry.go`. Having tools disabled by default follows the secure-by-default principle and helps to prevent context bloat.
- You should update docs/tools/overview.md with adding or changing a tool.
- **Tools must be stateless across calls.** The HTTP transport is stateless: consecutive calls from one client can land on different server processes, so anything stored in a tool struct between calls will be missing or wrong. Cache derived data that can be recomputed, not conversation state. If a tool genuinely cannot work this way, implement the `tools.StdioOnly` marker interface (an empty `StdioOnly()` method) and it will only be registered on the stdio transport. `sequential_thinking` is the only tool that does this.
- **SECURITY**: All tools that access files or make HTTP requests MUST integrate with the security system. See [Security Integration](#6-security-integration) above and [Security System Documentation](security.md) for details.
- Follow least privilege security principles.

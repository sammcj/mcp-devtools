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
    - [6. Import the Tool Package](#6-import-the-tool-package)
  - [Example: Hello World Tool](#example-hello-world-tool)
    - [Testing Your Tool](#testing-your-tool)
  - [Testing](#testing)

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

    // Implement your tool's logic here
    result := map[string]interface{}{
        "message": fmt.Sprintf("Tool executed with param1=%s, param2=%f", param1, param2),
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

### 6. Import the Tool Package

Finally, import your tool package in `main.go` to ensure it's registered:

```go
import _ "github.com/sammcj/mcp-devtools/internal/tools/your-category/your-tool"
```

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

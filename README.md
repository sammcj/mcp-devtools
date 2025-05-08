# MCP DevTools

This is a modular MCP server that provides various developer tools. It started as a package version checker and has been refactored into a modular architecture to support additional tools in the future.

## Features

Currently, the server provides the following tools:

### Package Versions

  - Check latest versions of NPM packages
  - Check latest versions of Python packages (requirements.txt and pyproject.toml)
  - Check latest versions of Java packages (Maven and Gradle)
  - Check latest versions of Go packages (go.mod)
  - Check latest versions of Swift packages
  - Check available tags and image sizes for Docker images (from Docker Hub)
  - Search and list AWS Bedrock models
  - Check latest versions of GitHub Actions

### Shadcn/UI Components

  - List all available shadcn/ui components
  - Search for shadcn/ui components by keyword
  - Get detailed information (description, installation, usage, props) for a specific component
  - Get usage examples for a specific component

## Installation

```bash
go install github.com/sammcj/mcp-devtools@latest
```

You can also install a specific version:

```bash
go install github.com/sammcj/mcp-devtools@v1.0.0
```

Or clone the repository and build it:

```bash
git clone https://github.com/sammcj/mcp-devtools.git
cd mcp-devtools
make
```

### Version Information

You can check the version of the installed binary:

```bash
mcp-devtools version
```

## Usage

The server supports two transport modes: stdio (default) and SSE (Server-Sent Events).

### STDIO Transport (Default)

```bash
mcp-devtools
```

Or if you built it locally:

```bash
./bin/mcp-devtools
```

### SSE Transport

```bash
mcp-devtools --transport sse --port 18080 --base-url http://localhost
```

Or if you built it locally:

```bash
./bin/mcp-devtools --transport sse --port 18080 --base-url http://localhost
```

#### Command-line Options

- `--transport`, `-t`: Transport type (stdio or sse). Default: stdio
- `--port`: Port to use for SSE transport. Default: 18080
- `--base-url`: Base URL for SSE transport. Default: http://localhost
- `--debug`, `-d`: Enable debug logging. Default: false

## Tools

### NPM Packages

Check the latest versions of NPM packages:

```json
{
  "name": "check_npm_versions",
  "arguments": {
    "dependencies": {
      "react": "^17.0.2",
      "react-dom": "^17.0.2",
      "lodash": "4.17.21"
    },
    "constraints": {
      "react": {
        "majorVersion": 17
      }
    }
  }
}
```

### Python Packages (requirements.txt)

Check the latest versions of Python packages from requirements.txt:

```json
{
  "name": "check_python_versions",
  "arguments": {
    "requirements": [
      "requests==2.28.1",
      "flask>=2.0.0",
      "numpy"
    ]
  }
}
```

### Python Packages (pyproject.toml)

Check the latest versions of Python packages from pyproject.toml:

```json
{
  "name": "check_pyproject_versions",
  "arguments": {
    "dependencies": {
      "dependencies": {
        "requests": "^2.28.1",
        "flask": ">=2.0.0"
      },
      "optional-dependencies": {
        "dev": {
          "pytest": "^7.0.0"
        }
      },
      "dev-dependencies": {
        "black": "^22.6.0"
      }
    }
  }
}
```

### Java Packages (Maven)

Check the latest versions of Java packages from Maven:

```json
{
  "name": "check_maven_versions",
  "arguments": {
    "dependencies": [
      {
        "groupId": "org.springframework.boot",
        "artifactId": "spring-boot-starter-web",
        "version": "2.7.0"
      },
      {
        "groupId": "com.google.guava",
        "artifactId": "guava",
        "version": "31.1-jre"
      }
    ]
  }
}
```

### Java Packages (Gradle)

Check the latest versions of Java packages from Gradle:

```json
{
  "name": "check_gradle_versions",
  "arguments": {
    "dependencies": [
      {
        "configuration": "implementation",
        "group": "org.springframework.boot",
        "name": "spring-boot-starter-web",
        "version": "2.7.0"
      },
      {
        "configuration": "testImplementation",
        "group": "junit",
        "name": "junit",
        "version": "4.13.2"
      }
    ]
  }
}
```

### Go Packages

Check the latest versions of Go packages from go.mod:

```json
{
  "name": "check_go_versions",
  "arguments": {
    "dependencies": {
      "module": "github.com/example/mymodule",
      "require": [
        {
          "path": "github.com/gorilla/mux",
          "version": "v1.8.0"
        },
        {
          "path": "github.com/spf13/cobra",
          "version": "v1.5.0"
        }
      ]
    }
  }
}
```

### Docker Images

Check available tags for Docker images:

```json
{
  "name": "check_docker_tags",
  "arguments": {
    "image": "nginx",
    "registry": "dockerhub",
    "limit": 5,
    "filterTags": ["^1\\."],
    "includeDigest": true,
    "includeSize": true
  }
}
```

### AWS Bedrock Models

List all AWS Bedrock models:

```json
{
  "name": "check_bedrock_models",
  "arguments": {
    "action": "list"
  }
}
```

Search for specific AWS Bedrock models:

```json
{
  "name": "check_bedrock_models",
  "arguments": {
    "action": "search",
    "query": "claude",
    "provider": "anthropic"
  }
}
```

Get the latest Claude Sonnet model:

```json
{
  "name": "get_latest_bedrock_model",
  "arguments": {}
}
```

### Swift Packages

Check the latest versions of Swift packages:

```json
{
  "name": "check_swift_versions",
  "arguments": {
    "dependencies": [
      {
        "url": "https://github.com/apple/swift-argument-parser",
        "version": "1.1.4"
      },
      {
        "url": "https://github.com/vapor/vapor",
        "version": "4.65.1"
      }
    ],
    "constraints": {
      "swift-argument-parser": {
        "majorVersion": 1
      }
    }
  }
}
```

### GitHub Actions

Check the latest versions of GitHub Actions:

```json
{
  "name": "check_github_actions",
  "arguments": {
    "actions": [
      {
        "owner": "actions",
        "repo": "checkout",
        "currentVersion": "v3"
      },
      {
        "owner": "actions",
        "repo": "setup-node",
        "currentVersion": "v3"
      }
    ],
    "includeDetails": true
  }
}
```

### Shadcn/UI Components

List all available shadcn/ui components:

```json
{
  "name": "list_shadcn_components",
  "arguments": {}
}
```

Search for shadcn/ui components:

```json
{
  "name": "search_components",
  "arguments": {
    "query": "button"
  }
}
```

Get detailed information for a specific shadcn/ui component:

```json
{
  "name": "get_component_details",
  "arguments": {
    "componentName": "alert-dialog"
  }
}
```

Get usage examples for a specific shadcn/ui component:

```json
{
  "name": "get_component_examples",
  "arguments": {
    "componentName": "accordion"
  }
}
```

## Architecture

The server is built with a modular architecture to make it easy to add new tools in the future. The main components are:

- **Core Tool Interface**: Defines the interface that all tools must implement.
- **Central Tool Registry**: Manages the registration and retrieval of tools.
- **Tool Modules**: Individual tool implementations organized by category.

## Creating New Tools

The MCP DevTools server is designed to be easily extensible with new tools. This section provides detailed guidance on how to create and integrate new tools into the server.

### Tool Interface

All tools must implement the `tools.Tool` interface defined in `internal/tools/tools.go`:

```go
type Tool interface {
    // Definition returns the tool's definition for MCP registration
    Definition() mcp.Tool

    // Execute executes the tool's logic
    Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error)
}
```

### Tool Structure

A typical tool implementation follows this structure:

1. **Tool Type**: Define a struct that will implement the Tool interface
2. **Registration**: Register the tool with the registry in an `init()` function
3. **Definition**: Implement the `Definition()` method to define the tool's name, description, and parameters
4. **Execution**: Implement the `Execute()` method to perform the tool's logic

### Step-by-Step Guide

#### 1. Create a New Package

Create a new package in the appropriate category under `internal/tools/` or create a new category if needed:

```bash
mkdir -p internal/tools/your-category/your-tool
touch internal/tools/your-category/your-tool/your-tool.go
```

#### 2. Implement the Tool Interface

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

#### 4. Result Schema

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

#### 5. Caching

The `cache` parameter in the `Execute` method is a shared cache that can be used to store and retrieve data across tool executions:

```go
// Store a value in the cache
cache.Store("key", value)

// Retrieve a value from the cache
if cachedValue, ok := cache.Load("key"); ok {
    // Use cachedValue
}
```

#### 6. Import the Tool Package

Finally, import your tool package in `main.go` to ensure it's registered:

```go
import _ "github.com/sammcj/mcp-devtools/internal/tools/your-category/your-tool"
```

### Example: Hello World Tool

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

## Releases and CI/CD

This project uses GitHub Actions for continuous integration and deployment. The workflow automatically:

1. Builds and tests the application on every push to the main branch and pull requests
2. Creates a release when a tag with the format `v*` (e.g., `v1.0.0`) is pushed
3. Builds and pushes Docker images to GitHub Container Registry

### Creating a Release

To create a new release:

1. Update the version in your code if necessary
2. Tag the commit with a semantic version:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```
3. The GitHub Actions workflow will automatically:
   - Build and test the application
   - Create a GitHub release with the binary
   - Generate a changelog based on commits since the last release
   - Build and push Docker images with appropriate tags

### Docker Images

Docker images are available from GitHub Container Registry:

```bash
docker pull ghcr.io/sammcj/mcp-devtools:latest
```

Or with a specific version:

```bash
docker pull ghcr.io/sammcj/mcp-devtools:v1.0.0
```

## License

MIT

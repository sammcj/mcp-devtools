# Refactoring `mcp-package-version-go` to `mcp-devtools`

## Objective

Transform the existing single-purpose MCP server (`mcp-package-version-go`) into a modular, multi-tool MCP server named `mcp-devtools`. The initial goal is to refactor the existing package version checking tools into the first module within this new structure, making it easy to add completely new tool modules in the future.

## Plan

1.  **Project Renaming & Module Path Update:**
    *   Rename the project conceptually to `mcp-devtools`.
    *   Update the Go module path in `go.mod` from `github.com/sammcj/mcp-package-version-go` to `github.com/sammcj/mcp-devtools`.
    *   Update all internal import paths accordingly.

2.  **Define Core `Tool` Interface (`internal/tools/tools.go`):**
    *   Create a standard Go interface (`tools.Tool`) that all MCP tool implementations must satisfy.
    *   This interface will require methods like:
        *   `Definition() mcp.ToolDefinition`: Returns the tool's name, description, and input schema for MCP registration.
        *   `Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error)`: Executes the tool's logic using shared resources (logger, cache) and parsed arguments.

3.  **Implement Central Tool Registry (`internal/registry/registry.go`):**
    *   Create a package to manage the registration of available tools.
    *   Provide functions:
        *   `Init(logger *logrus.Logger)`: Initialises the registry and shared resources (logger, cache).
        *   `Register(tool tools.Tool)`: Adds a tool implementation to the registry map (`map[string]tools.Tool`). Called by individual tools.
        *   `GetTool(name string) (tools.Tool, bool)`: Retrieves a tool by name for execution.
        *   `GetTools() map[string]tools.Tool`: Returns all registered tools (for generating server definition).
        *   `GetLogger() *logrus.Logger`, `GetCache() *sync.Map`: Accessors for shared resources.

4.  **Restructure and Refactor Existing Handlers:**
    *   Create the primary module directory: `internal/tools/packageversions/`.
    *   Move existing handler logic (`internal/handlers/*`) into specific sub-packages within `internal/tools/packageversions/`. Examples:
        *   `internal/handlers/npm.go` -> `internal/tools/packageversions/npm/npm.go`
        *   `internal/handlers/go.go` -> `internal/tools/packageversions/go/go.go`
        *   `internal/handlers/docker.go` -> `internal/tools/packageversions/docker/docker.go`
        *   `internal/handlers/bedrock.go` -> `internal/tools/packageversions/bedrock/bedrock.go`
        *   (Repeat for python, github_actions, swift, java)
    *   Refactor each tool's code:
        *   Implement the `tools.Tool` interface.
        *   Define the `mcp.ToolDefinition` struct for each tool, including its specific input schema. **Crucially, write clear, action-oriented descriptions with relevant keywords for each tool's definition to ensure AI agents can effectively discover and utilise them.**
        *   Adapt the core logic function (e.g., `GetLatestVersion`) into the `Execute` method, matching the interface signature.
        *   Add an `init()` function in each tool's package (e.g., `internal/tools/packageversions/npm/npm.go`) that creates an instance of the tool and calls `registry.Register()` with it.
    *   Consolidate or relocate shared types (from `internal/handlers/types.go`) as needed, potentially into `internal/tools/packageversions/types.go` or within the specific tool packages if not shared.

5.  **Update Server Core (`main.go`):**
    *   Initialise a logger (e.g., using `logrus`).
    *   Call `registry.Init(logger)` early in the execution.
    *   Use blank imports (`_`) for all the newly created tool packages (e.g., `_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/npm"`) to ensure their `init()` functions run and register the tools.
    *   Retrieve all registered tool definitions from `registry.GetTools()`.
    *   Instantiate the `mcp.Server` (from `github.com/mark3labs/mcp-go/mcp`).
    *   Provide the server with:
        *   The collected list of `mcp.ToolDefinition`s.
        *   A dispatcher function that, given a tool name and arguments:
            *   Retrieves the corresponding `tools.Tool` using `registry.GetTool()`.
            *   Calls the tool's `Execute` method, passing the context, shared logger (`registry.GetLogger()`), shared cache (`registry.GetCache()`), and arguments.
    *   Update CLI flags, descriptions, and application name to reflect `mcp-devtools`.
    *   Start the server using the chosen transport (stdio/sse).

6.  **Update Build/Deployment Files:**
    *   Review and modify `Makefile` and `Dockerfile` to account for the new structure, module path, and potentially changed build commands or dependencies.

7.  **Update Documentation (`README.md`):**
    *   Revise the README to describe the new multi-tool nature of `mcp-devtools`.
    *   Explain the modular architecture.
    *   Provide clear instructions on how developers can add *new* tool modules to the server in the future.

## Visual Structure

```mermaid
graph TD
    subgraph Project Root (mcp-devtools)
        direction TB
        M(main.go)
        GoMod(go.mod)
        R(internal/registry/)
        T(internal/tools/)
        Makefile(Makefile)
        Dockerfile(Dockerfile)
        README(README.md)
    end

    subgraph internal/tools/
        direction TB
        ToolsInterface(tools.go)
        PV(packageversions)
        FutureToolCategory(...)
    end

    subgraph internal/tools/packageversions/
        direction TB
        NPM(npm/)
        GO(go/)
        PY(python/)
        DK(docker/)
        BD(bedrock/)
        GA(githubactions/)
        SW(swift/)
        JV(java/)
        Types(types.go) -- Optional --
    end

    M --> R;
    M -- Imports tools from --> PV;
    R -- Registers tools from --> PV;
    T --> ToolsInterface;
    T --> PV;
    PV --> NPM;
    PV --> GO;
    PV --> PY;
    PV --> DK;
    PV --> BD;
    PV --> GA;
    PV --> SW;
    PV --> JV;

    classDef default fill:#EAF5EA,stroke:#C6E7C6,color:#77AD77
    class M, R, T, PV, GoMod, Makefile, Dockerfile, README default;
    class ToolsInterface, NPM, GO, PY, DK, BD, GA, SW, JV, Types, FutureToolCategory fill:#EFF3FF,stroke:#9ECAE1,color:#3182BD;
```

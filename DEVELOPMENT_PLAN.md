# MCP-Devtools Development Plan

This document outlines the development tasks for enhancing the `mcp-devtools` MCP server. It includes fixes identified from `mcp-package-version` and porting new functionality from `shadcn-ui-mcp-server`.

This project has three git repos:

1. mcp-devtools (- /Users/samm/git/sammcj/mcp-devtools) - this is my new MCP server that provides multiple tools wrapped up in one server, this is our target for development.
2. mcp-package-version - this is my old mcp server that provides package version information to AI agentic coding agents such as yourself. I want to finish migrating all the features and fixes recently made in this repo to my mcp-devtools repo.
3. shadcn-ui-mcp-server - this is an unmaintained javascript mcp server that aims to help AI agentic coding agents such as yourself with access to up to date information around the shadcn ui components library. - I want to learn from this and add it's functionality to our new mcp-devtools golang MCP server.

The full paths to the other two repos we want to bring in are:

- /Users/samm/git/mcp/shadcn-ui-mcp-server
- /Users/samm/git/sammcj/mcp-package-version

## Phase 1: Package Version Tool Enhancements

### Task 1.1: Enhance Docker Tool Functionality

- **File**: `internal/tools/packageversions/docker/docker.go`
- **Objective**: Ensure the `check_docker_tags` tool provides comprehensive information for Docker Hub images.
- **Details**:
  - [ ] **Populate Image Size**: In the `getDockerHubTags` function, modify the logic to extract and populate the `Size` field in the `packageversions.DockerImageVersion` struct. This information is available in the Docker Hub API response (`result.Images[0].Size`).
    ```go
    // Existing code snippet for context (around line 180-190 in mcp-devtools/internal/tools/packageversions/docker/docker.go):
    // ...
    // Add created date
    // created := result.LastUpdated.Format(time.RFC3339)
    // tag.Created = &created
    // tags = append(tags, tag)

    // Add this logic:
    if includeDigest && len(result.Images) > 0 { // Ensure Images slice is not empty
        // ... (existing digest logic) ...
        // Add Size
        sizeStr := fmt.Sprintf("%d", result.Images[0].Size) // Add this
        tag.Size = &sizeStr                                 // Add this
    }
    // ...
    ```
- **Testing**:
  - [ ] Verify that queries to `check_docker_tags` for Docker Hub images now include the `size` field in the output when `includeDigest` is true (or always, if appropriate).

## Phase 2: Shadcn UI Tools Integration (Porting from JavaScript)

This phase involves creating a new suite of tools for interacting with shadcn/ui documentation.

### Task 2.1: Project Setup and Dependencies

- **File**: `go.mod`
  - [ ] Add the `github.com/PuerkitoBio/goquery` dependency:
        ```bash
        go get github.com/PuerkitoBio/goquery
        ```
- **Directory**: `internal/tools/shadcnui/`
  - [ ] Create this new directory.
- **File**: `internal/tools/shadcnui/types.go`
  - [ ] Create this file.
  - [ ] Define the following Go structs, analogous to those in the JavaScript `shadcn-ui-mcp-server`:
        ```go
        package shadcnui

        // ComponentProp defines the structure for a component's property or variant.
        type ComponentProp struct {
            Type        string `json:"type"` // e.g., "variant", "string", "boolean"
            Description string `json:"description"`
            Required    bool   `json:"required"`
            Default     string `json:"default,omitempty"`
            Example     string `json:"example,omitempty"` // For variants, this could be code
        }

        // ComponentExample defines a usage example for a component.
        type ComponentExample struct {
            Title       string `json:"title"`
            Code        string `json:"code"`
            Description string `json:"description,omitempty"`
        }

        // ComponentInfo holds all details for a shadcn/ui component.
        type ComponentInfo struct {
            Name         string                    `json:"name"`
            Description  string                    `json:"description"`
            URL          string                    `json:"url"` // Link to the docs page
            SourceURL    string                    `json:"sourceUrl,omitempty"` // Link to GitHub source
            APIReference string                    `json:"apiReference,omitempty"` // If available
            Installation string                    `json:"installation,omitempty"` // npx command
            Usage        string                    `json:"usage,omitempty"`        // General usage code block
            Props        map[string]ComponentProp  `json:"props,omitempty"`      // Component props/variants
            Examples     []ComponentExample        `json:"examples,omitempty"`
        }
        ```
- **File**: `internal/tools/shadcnui/client.go` (New file)
  - [ ] Define a shared HTTP client and utility functions for making requests and scraping, if needed, or use existing utilities from `internal/tools/packageversions/utils.go`.
  - [ ] Define constants for Shadcn UI URLs:
        ```go
        const (
            ShadcnDocsURL        = "https://ui.shadcn.com"
            ShadcnDocsComponents = ShadcnDocsURL + "/docs/components"
            ShadcnGitHubURL      = "https://github.com/shadcn-ui/ui"
            ShadcnRawGitHubURL   = "https://raw.githubusercontent.com/shadcn-ui/ui/main"
        )
        ```

### Task 2.2: Implement `list_shadcn_components` Tool

- **File**: `internal/tools/shadcnui/list_components.go` (New file)
- **Objective**: Create a tool to list all available shadcn/ui components.
- **Details**:
  - [ ] Define `ListShadcnComponentsTool` struct.
  - [ ] Implement `Definition()` method:
    - Name: `list_shadcn_components`
    - Description: "Get a list of all available shadcn/ui components"
    - InputSchema: Empty object.
  - [ ] Implement `Execute()` method:
    - Fetch HTML from `ShadcnDocsComponents` URL.
    - Use `goquery` to parse HTML and find all links starting with `/docs/components/`.
    - Extract component names and construct `ComponentInfo` objects (name, URL, basic description if available directly).
    - Implement caching for the component list (e.g., using `sync.Map` with a suitable cache key and TTL).
    - Return the list as a JSON string.
  - [ ] Register the tool in `internal/registry/registry.go`.
- **Testing**:
  - [ ] Call the tool and verify it returns an accurate list of components.

### Task 2.3: Implement `get_component_details` Tool

- **File**: `internal/tools/shadcnui/get_details.go` (New file)
- **Objective**: Create a tool to get detailed information for a specific shadcn/ui component.
- **Details**:
  - [ ] Define `GetComponentDetailsTool` struct.
  - [ ] Implement `Definition()` method:
    - Name: `get_component_details`
    - Description: "Get detailed information about a specific shadcn/ui component"
    - InputSchema: Object with `componentName` (string, required).
  - [ ] Implement `Execute()` method:
    - Take `componentName` as input.
    - Fetch HTML from `${ShadcnDocsComponents}/{componentName}`.
    - Use `goquery` to scrape:
      - Title (component name).
      - Description (paragraph after H1).
      - Installation command (`npx shadcn-ui@latest add ...`).
      - Usage examples (code blocks under "Usage" H2).
      - Props/Variants (from "Examples" H3 sections or dedicated "Props" / "API Reference" sections if they exist). This requires careful selector design.
      - Source URL (construct from `ShadcnGitHubURL`).
    - Populate a `ComponentInfo` struct.
    - Implement caching for individual component details.
    - Return the `ComponentInfo` as a JSON string.
  - [ ] Register the tool.
- **Testing**:
  - [ ] Call the tool with various component names (e.g., "button", "accordion", "date-picker") and verify the accuracy and completeness of the returned details.

### Task 2.4: Implement `get_component_examples` Tool

- **File**: `internal/tools/shadcnui/get_examples.go` (New file)
- **Objective**: Create a tool to get usage examples for a specific shadcn/ui component.
- **Details**:
  - [ ] Define `GetComponentExamplesTool` struct.
  - [ ] Implement `Definition()` method:
    - Name: `get_component_examples`
    - Description: "Get usage examples for a specific shadcn/ui component"
    - InputSchema: Object with `componentName` (string, required).
  - [ ] Implement `Execute()` method:
    - Take `componentName` as input.
    - Scrape the component's docs page for code blocks under relevant headings (e.g., "Usage", "Examples", specific variant examples).
    - Attempt to fetch the demo file from GitHub: `${ShadcnRawGitHubURL}/apps/www/registry/default/example/{componentName}-demo.tsx`.
    - Compile these into a list of `ComponentExample` structs.
    - Return the list as a JSON string.
  - [ ] Register the tool.
- **Testing**:
  - [ ] Call the tool for several components and verify the examples are correctly fetched and formatted.

### Task 2.5: Implement `search_components` Tool

- **File**: `internal/tools/shadcnui/search_components.go` (New file)
- **Objective**: Create a tool to search for shadcn/ui components by keyword.
- **Details**:
  - [ ] Define `SearchShadcnComponentsTool` struct.
  - [ ] Implement `Definition()` method:
    - Name: `search_components`
    - Description: "Search for shadcn/ui components by keyword"
    - InputSchema: Object with `query` (string, required).
  - [ ] Implement `Execute()` method:
    - Take `query` as input.
    - Ensure the full component list (from `list_shadcn_components` logic/cache) is available.
    - Filter the list based on whether the query string appears in the component's name or description (case-insensitive).
    - Return the filtered list of `ComponentInfo` objects as a JSON string.
  - [ ] Register the tool.
- **Testing**:
  - [ ] Call the tool with various queries and verify the search results are relevant.

## Phase 3: (Optional) Extend Version Constraints Feature

### Task 3.1: Evaluate and Plan Constraint Extension

- **Objective**: Determine if version constraints (`excludePackage`, `majorVersion`) should be added to other package checking tools.
- **Details**:
  - [ ] Review user requirements for constraint needs in Python, Go, Java, Swift package tools.
  - [ ] If proceeding, plan implementation for each relevant tool.

### Task 3.2: Implement Constraints for Python Tools

- **Files**:
  - `internal/tools/packageversions/python/python.go` (for `check_python_versions`)
  - `internal/tools/packageversions/python/pyproject.go` (for `check_pyproject_versions`)
- **Details**:
  - [ ] Add optional `constraints` parameter (type `packageversions.VersionConstraints`) to the `Definition()` of both tools.
  - [ ] In the `Execute()` method of each tool:
    - Parse the `constraints` argument.
    - Apply `excludePackage` logic.
    - Apply `majorVersion` logic when determining the `latestVersion` from PyPI, similar to the NPM tool's implementation.
- **Testing**:
  - [ ] Test with various constraints for both `requirements.txt` and `pyproject.toml` inputs.

### Task 3.3: Implement Constraints for Go Tool

- **File**: `internal/tools/packageversions/go/go.go` (for `check_go_versions`)
- **Details**:
  - [ ] Add optional `constraints` parameter to `Definition()`.
  - [ ] Implement constraint logic in `Execute()`.
- **Testing**:
  - [ ] Test with Go module inputs and constraints.

### Task 3.4: Implement Constraints for Java Tools

- **Files**:
  - `internal/tools/packageversions/java/maven.go` (for `check_maven_versions`)
  - `internal/tools/packageversions/java/gradle.go` (for `check_gradle_versions`)
- **Details**:
  - [ ] Add optional `constraints` parameter to `Definition()` of both tools.
  - [ ] Implement constraint logic in `Execute()` for both.
- **Testing**:
  - [ ] Test with Maven and Gradle inputs and constraints.

### Task 3.5: Implement Constraints for Swift Tool

- **File**: `internal/tools/packageversions/swift/swift.go` (for `check_swift_versions`)
- **Details**:
  - [ ] Add optional `constraints` parameter to `Definition()`.
  - [ ] Implement constraint logic in `Execute()`.
- **Testing**:
  - [ ] Test with Swift package inputs and constraints.

## General Considerations

- **Logging**: Ensure consistent and informative logging (using `logrus`) throughout all new and modified code.
- **Error Handling**: Implement robust error handling for API requests, scraping, and argument parsing.
- **Testing**: Write unit tests for key logic, especially scraping and data transformation, where feasible. Perform thorough manual testing of each tool.
- **Code Style**: Adhere to existing Go coding conventions in the `mcp-devtools` project.
- **Documentation**: Update any relevant internal documentation or READMEs if tool behaviors change significantly or new major features are added.

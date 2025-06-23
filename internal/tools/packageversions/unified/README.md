# Unified Package Search Tool

This directory contains the unified `search_packages` tool that consolidates all individual package manager tools into a single, easy-to-use interface.

## Overview

The unified tool replaces the individual package manager tools (npm, go, python, java, swift, github-actions, docker, bedrock) with a single tool that can search across all supported ecosystems.

## Usage

The tool accepts the following parameters:

- **ecosystem** (required): The package ecosystem to search. Supported values:
  - `npm` - Node.js packages
  - `go` - Go modules
  - `python` - PyPI packages
  - `python-pyproject` - pyproject.toml format
  - `java-maven` - Maven dependencies
  - `java-gradle` - Gradle dependencies
  - `swift` - Swift Package Manager
  - `github-actions` - GitHub Actions
  - `docker` - Container images
  - `bedrock` - AWS Bedrock models
- **query** (required): The search query (package name, image name, model search term)
- **data** (optional): Ecosystem-specific data object for batch operations
- **constraints** (optional): Version constraints and exclusions
- **action** (optional): Specific actions for certain ecosystems (bedrock, docker)
- **limit** (optional): Maximum number of results
- **registry** (optional): Specific registry to use (for docker)
- **includeDetails** (optional): Include additional details in results

## Examples

### Search for npm package
```json
{
  "ecosystem": "npm",
  "query": "react"
}
```

### Search for Go module
```json
{
  "ecosystem": "go",
  "query": "github.com/gin-gonic/gin"
}
```

### List AWS Bedrock models
```json
{
  "ecosystem": "bedrock",
  "action": "list"
}
```

### Search Docker images
```json
{
  "ecosystem": "docker",
  "query": "nginx",
  "registry": "dockerhub",
  "limit": 5
}
```

## Benefits

1. **Simplified Discovery**: AI agents only need to discover one tool instead of 9+ separate tools
2. **Consistent Interface**: Same parameter pattern across all ecosystems
3. **Rich Annotations**: Comprehensive tool description with detailed ecosystem explanations
4. **Maintains Functionality**: All existing capabilities preserved
5. **Easy Extension**: New ecosystems can be added easily

## Implementation

The unified tool acts as a dispatcher that:

1. Parses the ecosystem parameter
2. Converts the query/data to the appropriate format for the target ecosystem
3. Creates and calls the appropriate ecosystem-specific tool
4. Returns the result in the original ecosystem's format

This approach maintains backward compatibility with existing response formats while providing a unified interface for AI agents.

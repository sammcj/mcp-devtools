# Package Search Tool

The Package Search tool provides version checking and package information across multiple programming language ecosystems through a single unified interface.

## Overview

Instead of manually checking different package managers, the Package Search tool lets you query NPM, PyPI, Go modules, Maven, Docker Hub, GitHub Actions, and more from one place. Perfect for dependency management, security audits, and keeping projects up-to-date.

## Configuration

### Environment Variables

The Package Search tool supports the following configuration options:

- **`PACKAGES_RATE_LIMIT`**: Maximum HTTP requests per second to package registries
  - **Default**: `10`
  - **Description**: Controls the rate of HTTP requests to prevent overwhelming package registry APIs
  - **Example**: `PACKAGES_RATE_LIMIT=20` allows up to 20 requests per second

### Security Features

- **Rate Limiting**: Configurable request rate limiting protects against overwhelming external package registries
- **Input Validation**: Comprehensive validation of package names and version constraints
- **Error Handling**: Graceful handling of network issues and API failures
- **Trusted Sources**: Only queries well-known, established package registries

## Supported Ecosystems

| Ecosystem          | Package Types         | Features                                    |
|--------------------|-----------------------|---------------------------------------------|
| **NPM**            | Node.js packages      | Version constraints, dependency trees       |
| **Python**         | PyPI packages         | Requirements.txt and pyproject.toml formats |
| **Go**             | Go modules            | Module versions and dependencies            |
| **Java**           | Maven & Gradle        | Group/artifact resolution                   |
| **Swift**          | Swift Package Manager | Package dependencies                        |
| **Docker**         | Container images      | Tag information and registries              |
| **GitHub Actions** | Workflow actions      | Action versions and metadata                |
| **AWS Bedrock**    | AI/ML models          | Model availability and capabilities         |

## Usage Examples

### NPM Packages

**Single Package Query:**
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "npm",
    "query": "lodash"
  }
}
```

**Multiple Packages with Constraints:**
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "npm",
    "data": {
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

### Python Packages

**Single Package:**
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "python",
    "query": "requests"
  }
}
```

**Requirements.txt Format:**
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "python",
    "data": [
      "requests==2.28.1",
      "flask>=2.0.0",
      "numpy"
    ]
  }
}
```

**pyproject.toml Format:**
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "python-pyproject",
    "data": {
      "dependencies": {
        "requests": "^2.28.1",
        "flask": ">=2.0.0"
      }
    }
  }
}
```

### Go Modules
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "go",
    "query": "github.com/gin-gonic/gin"
  }
}
```

### Java Dependencies

**Maven:**
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "java-maven",
    "data": [
      {
        "groupId": "org.springframework.boot",
        "artifactId": "spring-boot-starter-web",
        "version": "2.7.0"
      }
    ]
  }
}
```

**Gradle:**
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "java-gradle",
    "data": [
      {
        "configuration": "implementation",
        "group": "org.springframework.boot",
        "name": "spring-boot-starter-web",
        "version": "2.7.0"
      }
    ]
  }
}
```

### Swift Packages
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "swift",
    "data": [
      {
        "url": "https://github.com/apple/swift-argument-parser",
        "version": "1.1.4"
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

### Docker Images
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "docker",
    "query": "nginx",
    "registry": "dockerhub",
    "limit": 5,
    "includeDetails": true
  }
}
```

### GitHub Actions
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "github-actions",
    "query": "actions/checkout@v3",
    "includeDetails": true
  }
}
```

### AWS Bedrock Models

**List All Models:**
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "bedrock",
    "action": "list"
  }
}
```

**Search for Specific Models:**
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "bedrock",
    "action": "search",
    "query": "claude"
  }
}
```

## Parameters Reference

### Universal Parameters
- **`ecosystem`** (required): Target ecosystem to search
- **`query`** (optional): Package name or search term
- **`data`** (optional): Structured package data for batch operations
- **`constraints`** (optional): Version constraints and filters
- **`limit`** (optional): Maximum results to return
- **`includeDetails`** (optional): Include additional metadata

### Ecosystem-Specific Parameters

#### NPM
- **`data`**: Object with package names as keys, version constraints as values
- **`constraints`**: Version requirements per package

#### Python
- **`data`**: Array of requirement strings or dependency object (pyproject.toml)

#### Docker
- **`registry`**: Target registry (`dockerhub`, `ghcr`, `custom`)
- **`action`**: Operation type (`tags`, `info`)

#### AWS Bedrock
- **`action`**: Operation type (`list`, `search`, `get`)

## Common Use Cases

### Dependency Auditing
Check all dependencies in a project for outdated versions:

```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "npm",
    "data": {
      "express": "^4.18.0",
      "mongoose": "^6.5.0",
      "jsonwebtoken": "^8.5.1"
    }
  }
}
```

### Security Updates
Find latest secure versions for specific packages:

```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "python",
    "data": ["requests>=2.28.0", "django>=4.0.0"],
    "includeDetails": true
  }
}
```

### Migration Planning
Check availability before major version upgrades:

```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "java-maven",
    "query": "org.springframework.boot:spring-boot-starter-web",
    "constraints": {
      "majorVersion": 3
    }
  }
}
```

### Container Image Management
Find latest stable tags for deployment:

```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "docker",
    "query": "postgres",
    "action": "tags",
    "limit": 10
  }
}
```

## Batch Operations

For efficiency, prefer batch operations over individual queries:

**Instead of multiple single queries:**
```json
// Avoid this approach
{"ecosystem": "npm", "query": "react"}
{"ecosystem": "npm", "query": "lodash"}
{"ecosystem": "npm", "query": "express"}
```

**Use batch operations:**
```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "npm",
    "data": {
      "react": "latest",
      "lodash": "^4.0.0",
      "express": "^4.0.0"
    }
  }
}
```

## Response Format

### Single Package Response
```json
{
  "package": "lodash",
  "latest_version": "4.17.21",
  "description": "Lodash modular utilities",
  "homepage": "https://lodash.com/",
  "license": "MIT",
  "published": "2021-02-20T00:00:00Z"
}
```

### Batch Response
```json
{
  "packages": {
    "react": {
      "latest_version": "18.2.0",
      "requested_version": "^17.0.2",
      "status": "outdated"
    },
    "lodash": {
      "latest_version": "4.17.21",
      "requested_version": "4.17.21",
      "status": "up_to_date"
    }
  }
}
```

## Performance Tips

1. **Use batch operations** - More efficient than individual queries
2. **Specify version constraints** - Reduces processing time
3. **Use appropriate limits** - Avoid retrieving unnecessary data
4. **Cache results** - Package versions change infrequently

## Error Handling

Common error scenarios and responses:

- **Package not found**: Clear message with suggested alternatives
- **Invalid version format**: Validation error with correct format examples
- **Network timeouts**: Retry suggestions and alternative approaches
- **Rate limiting**: Information about limits and retry timing

## Integration Examples

While intended to be activated via a prompt to an agent, below are some example JSON tool calls that can also be used directly or in scripts such as CI/CD pipelines.

### CI/CD Pipeline
Use for automated dependency checking:

```bash
# Check for outdated packages
echo '{"ecosystem": "npm", "data": {...}}' | mcp-devtools search_packages
```

### Security Scanning
Identify packages with known vulnerabilities:

```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "python",
    "data": ["django==3.1.0", "requests==2.25.0"],
    "includeDetails": true
  }
}
```

### Dependency Management
Plan upgrades across multiple ecosystems:

```json
{
  "name": "search_packages",
  "arguments": {
    "ecosystem": "java-maven",
    "data": [
      {"groupId": "org.springframework", "artifactId": "spring-core"},
      {"groupId": "junit", "artifactId": "junit"}
    ]
  }
}
```

---

For technical implementation details, see the [Package Search source documentation](../../internal/tools/packageversions/unified/README.md).

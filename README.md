# MCP Package Version Go

This is a Go implementation of the MCP Package Version server, which provides tools for checking the latest versions of packages from various package managers and registries.

## Features

- Check latest versions of NPM packages
- Check latest versions of Python packages (requirements.txt and pyproject.toml)
- Check latest versions of Java packages (Maven and Gradle)
- Check latest versions of Go packages (go.mod)
- Check latest versions of Swift packages
- Check available tags for Docker images
- Search and list AWS Bedrock models
- Check latest versions of GitHub Actions

## Installation

```bash
go install github.com/sammcj/mcp-package-version-go@latest
```

You can also install a specific version:

```bash
go install github.com/sammcj/mcp-package-version-go@v1.0.0
```

Or clone the repository and build it:

```bash
git clone https://github.com/sammcj/mcp-package-version-go.git
cd mcp-package-version-go
make
```

### Version Information

You can check the version of the installed binary:

```bash
mcp-package-version-go version
```

## Usage

The server supports two transport modes: stdio (default) and SSE (Server-Sent Events).

### STDIO Transport (Default)

```bash
mcp-package-version-go
```

Or if you built it locally:

```bash
./bin/mcp-package-version-go
```

### SSE Transport

```bash
mcp-package-version-go --transport sse --port 18080 --base-url http://localhost
```

Or if you built it locally:

```bash
./bin/mcp-package-version-go --transport sse --port 18080 --base-url http://localhost
```

#### Command-line Options

- `--transport`, `-t`: Transport type (stdio or sse). Default: stdio
- `--port`: Port to use for SSE transport. Default: 18080
- `--base-url`: Base URL for SSE transport. Default: http://localhost

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
    "includeDigest": true
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
      "https://github.com/apple/swift-argument-parser": {
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
docker pull ghcr.io/sammcj/mcp-package-version-go:latest
```

Or with a specific version:

```bash
docker pull ghcr.io/sammcj/mcp-package-version-go:v1.0.0
```

## License

MIT

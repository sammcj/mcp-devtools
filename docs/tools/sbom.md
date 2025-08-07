# SBOM Tool

The SBOM (Software Bill of Materials) tool generates comprehensive dependency inventories from source code projects using Anchore Syft, providing essential visibility into software composition for AI-assisted development workflows.

## Overview

Critical for modern software development, this tool creates detailed inventories of all software components, dependencies, and libraries in your projects. Perfect for security analysis preparation, compliance requirements, vulnerability management, and understanding software composition in AI-driven development environments.

## Features

- **Source Code Analysis**: Scan project dependencies from multiple package managers
- **Multiple Output Formats**: Syft JSON, CycloneDX, SPDX, and human-readable table formats
- **Development Dependencies**: Optional inclusion of dev/test dependencies
- **Absolute Path Requirements**: Consistent file path handling for MCP environments
- **Multi-Language Support**: Works with npm, pip, go.mod, Maven, Gradle, Cargo, and more
- **Security-First Design**: Disabled by default, explicitly enabled via environment variable
- **Compliance Ready**: Generates industry-standard SBOM formats

## Prerequisites

This tool is disabled by default. Enable it by including `sbom` in the `ENABLE_ADDITIONAL_TOOLS` environment variable.

## Tool Usage Examples

While intended to be activated via a prompt to an agent, below are some example JSON tool calls.

### Basic Project Analysis
```json
{
  "name": "sbom",
  "arguments": {
    "source": "/Users/developer/my-project"
  }
}
```

### Generate SBOM with File Output
```json
{
  "name": "sbom",
  "arguments": {
    "source": "/Users/developer/web-app",
    "output_file": "/Users/developer/reports/app-sbom.json"
  }
}
```

### Include Development Dependencies
```json
{
  "name": "sbom",
  "arguments": {
    "source": "/Users/developer/api-service",
    "include_dev_dependencies": true,
    "output_format": "cyclonedx-json"
  }
}
```

### Generate Compliance-Ready SBOM
```json
{
  "name": "sbom",
  "arguments": {
    "source": "/Users/developer/production-app",
    "output_format": "spdx-json",
    "output_file": "/Users/developer/compliance/production-sbom.spdx.json"
  }
}
```

## Parameters Reference

### Core Parameters
| Parameter                  | Type    | Default     | Required | Description                              |
|----------------------------|---------|-------------|----------|------------------------------------------|
| `source`                   | string  | -           | Yes      | Absolute path to source directory        |
| `output_format`            | string  | "syft-json" | No       | SBOM output format                       |
| `include_dev_dependencies` | boolean | false       | No       | Include development dependencies         |
| `output_file`              | string  | -           | No       | Absolute path to save SBOM               |

### Output Format Options

| Format         | Description                    | Best For                          | File Extension    |
|----------------|--------------------------------|-----------------------------------|-------------------|
| `syft-json`    | Syft native JSON format        | Vulnerability scanning, analysis  | `.json`           |
| `cyclonedx-json` | CycloneDX standard format    | Industry toolchain integration    | `.cdx.json`       |
| `spdx-json`    | SPDX standard format           | Compliance, legal review          | `.spdx.json`      |
| `syft-table`   | Human-readable table format    | Manual review, documentation      | `.txt`            |

### Supported Package Managers

The tool automatically detects and analyses dependencies from these package managers:

| Language/Platform | Package Manager | Detection Files                   |
|-------------------|-----------------|-----------------------------------|
| Node.js           | npm/yarn        | package.json, package-lock.json   |
| Python            | pip/poetry      | requirements.txt, pyproject.toml  |
| Go                | go modules      | go.mod, go.sum                    |
| Java              | Maven/Gradle    | pom.xml, build.gradle             |
| Rust              | Cargo           | Cargo.toml, Cargo.lock            |
| C#                | NuGet           | packages.config, *.csproj        |
| Ruby              | Bundler         | Gemfile, Gemfile.lock             |
| PHP               | Composer        | composer.json, composer.lock      |

## Response Format

### Syft JSON Response (Default)
```json
{
  "sbom_format": "syft-json",
  "source": "/Users/developer/my-project",
  "packages": [
    {
      "name": "express",
      "version": "4.18.2",
      "type": "npm",
      "foundBy": "javascript-package-cataloger",
      "locations": [
        {
          "path": "/Users/developer/my-project/package.json"
        }
      ],
      "licenses": ["MIT"],
      "purl": "pkg:npm/express@4.18.2"
    }
  ],
  "package_count": 156,
  "timestamp": "2024-01-15T10:30:45Z"
}
```

### CycloneDX JSON Response
```json
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.4",
  "version": 1,
  "metadata": {
    "timestamp": "2024-01-15T10:30:45Z",
    "component": {
      "name": "my-project",
      "type": "application"
    }
  },
  "components": [
    {
      "type": "library",
      "name": "express",
      "version": "4.18.2",
      "purl": "pkg:npm/express@4.18.2",
      "licenses": [{"license": {"name": "MIT"}}]
    }
  ]
}
```

### Table Format Response
```
NAME                    VERSION    TYPE
express                 4.18.2     npm
body-parser            1.20.1     npm
cookie-parser          1.4.6      npm
cors                   2.8.5      npm
helmet                 6.0.1      npm
morgan                 1.10.0     npm
...
```

## Common Use Cases

### Development Workflow Analysis
Understand your project's dependency landscape:
```json
{
  "name": "sbom",
  "arguments": {
    "source": "/Users/developer/new-service",
    "include_dev_dependencies": true
  }
}
```

### Pre-Security Scan Preparation
Generate SBOM for vulnerability analysis:
```json
{
  "name": "sbom",
  "arguments": {
    "source": "/Users/developer/api-gateway",
    "output_file": "/Users/developer/security/gateway-sbom.json"
  }
}
```

### Compliance Documentation
Create industry-standard compliance artifacts:
```json
{
  "name": "sbom",
  "arguments": {
    "source": "/Users/developer/production-app",
    "output_format": "spdx-json",
    "output_file": "/Users/developer/compliance/app-sbom.spdx.json"
  }
}
```

### Dependency Auditing
Review all project dependencies including development tools:
```json
{
  "name": "sbom",
  "arguments": {
    "source": "/Users/developer/frontend-app",
    "output_format": "syft-table",
    "include_dev_dependencies": true,
    "output_file": "/Users/developer/audit/dependency-report.txt"
  }
}
```

## Workflow Integration

### Security Analysis Workflow
```bash
# 1. Generate comprehensive SBOM
sbom --source="/Users/dev/my-app" --output_file="/Users/dev/reports/app-sbom.json"

# 2. Scan SBOM for vulnerabilities
vulnerability_scan --source="sbom:/Users/dev/reports/app-sbom.json" --only_fixed=true

# 3. Store dependency insights
memory create_entities --namespace="dependencies" --data='{"entities": [{"name": "MyApp_Dependencies", "observations": ["156 total packages", "12 dev dependencies", "No critical vulnerabilities"]}]}'

# 4. Generate human-readable report
sbom --source="/Users/dev/my-app" --output_format="syft-table" --output_file="/Users/dev/reports/dependencies.txt"
```

### Compliance Preparation Workflow
```bash
# 1. Generate SPDX-compliant SBOM
sbom --source="/Users/prod/release-candidate" --output_format="spdx-json" --output_file="/Users/prod/compliance/release-sbom.spdx.json"

# 2. Generate CycloneDX for toolchain integration
sbom --source="/Users/prod/release-candidate" --output_format="cyclonedx-json" --output_file="/Users/prod/compliance/release-sbom.cdx.json"

# 3. Create human-readable summary
sbom --source="/Users/prod/release-candidate" --output_format="syft-table" --output_file="/Users/prod/compliance/dependency-summary.txt"
```

### Development Understanding Workflow
```bash
# 1. Analyse new project dependencies
sbom --source="/Users/dev/unfamiliar-project" --include_dev_dependencies=true

# 2. Focus on production dependencies
sbom --source="/Users/dev/unfamiliar-project" --output_format="syft-table"

# 3. Research high-risk packages
package_search --ecosystem="npm" --query="identified-package-name" --includeDetails=true

# 4. Store architectural insights
think "This project uses Express.js with 156 dependencies. Main security concerns are around authentication middleware and data validation libraries."
```

## Advanced Features

### Development vs Production Dependencies

**Production Only (Default)**:
```json
{
  "include_dev_dependencies": false
}
```
- Runtime dependencies only
- Smaller SBOM size
- Focus on deployed components

**Include Development Dependencies**:
```json
{
  "include_dev_dependencies": true
}
```
- Complete project view
- Test frameworks, build tools, linters
- Comprehensive security analysis

### Format Selection Strategy

**Syft JSON**: Best for vulnerability scanning
- Native Anchore format
- Rich metadata and relationships
- Optimised for Grype vulnerability scanning

**CycloneDX**: Best for industry toolchain integration
- OWASP standard format
- Broad tool ecosystem support
- Security-focused metadata

**SPDX**: Best for compliance and legal review
- Linux Foundation standard
- License compliance focus
- Legal department friendly

**Table Format**: Best for human review
- Quick visual assessment
- Easy to understand structure
- Suitable for documentation

### Multi-Project Analysis
For monorepos or multiple related projects:
```bash
# Generate SBOMs for each service
sbom --source="/Users/dev/monorepo/service-a" --output_file="/Users/dev/reports/service-a-sbom.json"
sbom --source="/Users/dev/monorepo/service-b" --output_file="/Users/dev/reports/service-b-sbom.json"
sbom --source="/Users/dev/monorepo/service-c" --output_file="/Users/dev/reports/service-c-sbom.json"

# Analyse each for vulnerabilities
vulnerability_scan --source="sbom:/Users/dev/reports/service-a-sbom.json"
vulnerability_scan --source="sbom:/Users/dev/reports/service-b-sbom.json"
vulnerability_scan --source="sbom:/Users/dev/reports/service-c-sbom.json"
```

## Error Handling

### Path Validation Errors
```json
{
  "error": "source path must be absolute: ./relative/path",
  "suggestion": "Use absolute paths like /Users/username/project"
}
```

### Missing Package Files
```json
{
  "error": "No package manager files found in directory",
  "suggestion": "Ensure directory contains package.json, go.mod, requirements.txt, or similar dependency files"
}
```

### Output File Errors
```json
{
  "error": "output_file path must be absolute: ./reports/sbom.json",
  "suggestion": "Use absolute paths like /Users/username/reports/sbom.json"
}
```

## Performance Considerations

### Scan Time Factors
- **Project size**: Larger projects take longer to analyse
- **Dependency count**: More dependencies = longer scan time
- **Package manager**: Some package managers faster than others
- **Development dependencies**: Including dev deps increases time

### Optimisation Tips
- **Exclude dev dependencies**: For production SBOM generation
- **Use absolute paths**: Prevents path resolution overhead
- **Cache SBOMs**: Generate once, use for multiple vulnerability scans
- **Choose appropriate format**: Table format is fastest for human review

### Expected Performance
- **Small projects** (<50 deps): 10-30 seconds
- **Medium projects** (50-200 deps): 30-90 seconds
- **Large projects** (200+ deps): 1-3 minutes
- **Monorepos**: Process each service separately

## Integration Patterns

### CI/CD Pipeline Integration
```yaml
# Generate SBOM in CI pipeline
- name: Generate SBOM
  run: |
    sbom --source="${GITHUB_WORKSPACE}" \
         --output_file="${GITHUB_WORKSPACE}/sbom.json" \
         --output_format="cyclonedx-json"

- name: Scan for vulnerabilities
  run: |
    vulnerability_scan --source="sbom:${GITHUB_WORKSPACE}/sbom.json" \
                      --only_fixed=true \
                      --output_format="sarif" \
                      --output_file="security.sarif"
```

### Security Workflow Integration
```bash
# Regular security assessment workflow
sbom --source="/Users/prod/api-service" --output_file="/Users/security/sboms/api-$(date +%Y%m%d).json"
vulnerability_scan --source="sbom:/Users/security/sboms/api-$(date +%Y%m%d).json" --only_fixed=true
```

## Security Considerations

- **Absolute Paths**: Required for consistent MCP tool behaviour
- **Disabled by Default**: Explicitly enable via environment variable
- **File System Access**: Only reads package manager files, doesn't execute code
- **Output Security**: Files created with secure permissions
- **Dependency Privacy**: SBOM reveals project dependencies (consider confidentiality)

## Configuration

### Environment Variables
- **`ENABLE_ADDITIONAL_TOOLS`**: Must include `sbom` to enable tool
- **Example**: `ENABLE_ADDITIONAL_TOOLS="sbom,vulnerability_scan"`

### Performance Tuning
- **Exclude dev dependencies**: Faster generation, smaller files
- **Use appropriate formats**: JSON for processing, table for review
- **Generate once, scan multiple**: Create SBOM once, reuse for vulnerability scanning

---

For technical implementation details, see the [SBOM source documentation](../../internal/tools/sbom/).

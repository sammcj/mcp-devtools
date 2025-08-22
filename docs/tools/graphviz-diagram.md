# Graphviz Diagram Tool

**Note:** This tool is disabled by default. To enable it, set the `ENABLE_ADDITIONAL_TOOLS` environment variable to include `graphviz_diagram`.

## Overview

The Graphviz Diagram tool generates architecture diagrams using a native Go implementation with embedded Graphviz. Particularly excellent for AWS/cloud architectures, it's specifically designed for AI coding agents and provides a simple, unified JSON interface for creating, discovering, and learning from diagram patterns.

## Features

- **Native Go Implementation**: No external Python dependencies - uses embedded WASM Graphviz
- **AI-Friendly JSON**: Simple JSON format optimised for AI agent consumption
- **Multi-format Output**: Supports PNG, SVG, and DOT formats (all three by default)
- **Professional Icons**: Official AWS architecture icons embedded in the binary - no external files needed
- **Progressive Examples**: Learn from basic to advanced diagram patterns
- **Icon Discovery**: Browse AWS, GCP, Kubernetes, and generic icons
- **Action-Based API**: Single tool with multiple functions
- **High-Resolution Output**: 300 DPI for crisp, professional diagrams

## Usage

The tool uses an action-based approach with three main operations:

### Get Examples

Retrieve example diagram definitions to learn the syntax:

```json
{
  "action": "examples",
  "diagram_type": "aws"
}
```

**Parameters:**
- `diagram_type` (optional): Type of examples - `"aws"`, `"sequence"`, `"flow"`, `"class"`, `"k8s"`, `"onprem"`, `"custom"`, or `"all"` (default)

### List Available Icons

Discover icons for your diagrams:

```json
{
  "action": "list_icons",
  "provider": "aws",
  "service": "compute"
}
```

**Parameters:**
- `provider` (optional): Filter by provider - `"aws"`, `"gcp"`, `"k8s"`, `"generic"`
- `service` (optional): Filter by service category - `"compute"`, `"database"`, `"network"`, `"storage"`, `"security"`, `"analytics"`

### Generate Diagrams

Create diagrams from definitions:

```json
{
  "action": "generate",
  "definition": "{\"name\": \"My Architecture\", \"nodes\": [{\"id\": \"web\", \"type\": \"aws.ec2\", \"label\": \"Web Server\"}, {\"id\": \"db\", \"type\": \"aws.rds\", \"label\": \"Database\"}], \"connections\": [{\"from\": \"web\", \"to\": \"db\"}]}",
  "output_format": ["png", "svg"],
  "filename": "my-architecture",
  "workspace_dir": "/path/to/output"
}
```

**Parameters:**
- `definition` (required): Diagram definition in JSON format (see structure below)
- `output_format` (optional): Array of formats - `["png"]`, `["svg"]`, `["dot"]` (default: `["png", "svg", "dot"]`)
- `filename` (optional): Custom filename without extension (auto-generated if not provided)
- `workspace_dir` (optional): Output directory (defaults to current working directory)

## JSON Structure

The tool uses a simple JSON format optimised for AI agents:

### Basic Structure

```json
{
  "name": "Diagram Title",
  "direction": "LR",
  "nodes": [
    {"id": "nodeId", "type": "aws.ec2", "label": "Display Name"}
  ],
  "connections": [
    {"from": "nodeId1", "to": "nodeId2"}
  ]
}
```

### Supported Providers

- **AWS**: `aws.ec2`, `aws.rds`, `aws.lambda`, `aws.s3`, `aws.alb`, etc.
- **GCP**: `gcp.compute`, `gcp.storage`, etc.
- **Kubernetes**: `k8s.pod`, `k8s.service`, etc.
- **Generic**: `generic.server`, `generic.database`, `generic.client`, etc.

### Advanced Features

**Clusters/Groups:**
```json
{
  "name": "VPC Architecture",
  "nodes": [
    {"id": "web", "type": "aws.ec2", "label": "Web Server"},
    {"id": "db", "type": "aws.rds", "label": "Database"}
  ],
  "connections": [
    {"from": "web", "to": "db"}
  ],
  "clusters": [
    {"name": "Production VPC", "nodes": ["web", "db"]}
  ]
}
```

**Multiple Connections:**
```json
{
  "name": "Multi-Connection Pattern",
  "nodes": [
    {"id": "web", "type": "aws.ec2", "label": "Web Server"},
    {"id": "db", "type": "aws.rds", "label": "Database"},
    {"id": "cache", "type": "aws.elasticache", "label": "Cache"}
  ],
  "connections": [
    {"from": "web", "to": "db"},
    {"from": "web", "to": "cache"}
  ]
}
```

## Examples

### Simple 3-Tier Architecture

```json
{
  "name": "3-Tier Web Application",
  "direction": "LR",
  "nodes": [
    {"id": "alb", "type": "aws.alb", "label": "Application Load Balancer"},
    {"id": "web", "type": "aws.ec2", "label": "Web Server"},
    {"id": "db", "type": "aws.rds", "label": "Database"}
  ],
  "connections": [
    {"from": "alb", "to": "web"},
    {"from": "web", "to": "db"}
  ]
}
```

### Serverless Architecture

```json
{
  "name": "Serverless Architecture",
  "direction": "TB",
  "nodes": [
    {"id": "user", "type": "generic.client", "label": "User"},
    {"id": "api", "type": "aws.apigateway", "label": "API Gateway"},
    {"id": "lambda", "type": "aws.lambda", "label": "Lambda Function"},
    {"id": "dynamodb", "type": "aws.dynamodb", "label": "DynamoDB"},
    {"id": "s3", "type": "aws.s3", "label": "S3 Bucket"}
  ],
  "connections": [
    {"from": "user", "to": "api"},
    {"from": "api", "to": "lambda"},
    {"from": "lambda", "to": "dynamodb"},
    {"from": "lambda", "to": "s3"}
  ]
}
```

### Microservices with VPC

```
diagram "Microservices Architecture" {
  cluster vpc "Production VPC" {
    cluster ingress "Ingress Layer" {
      node alb = aws.alb "Application LB"
      node apigw = aws.apigateway "API Gateway"
    }

    cluster services "Services Layer" {
      node user_svc = aws.ecs "User Service"
      node order_svc = aws.ecs "Order Service"
      node payment_svc = aws.ecs "Payment Service"
    }

    cluster data "Data Layer" {
      node user_db = aws.rds "User DB"
      node order_db = aws.rds "Order DB"
      node payment_db = aws.rds "Payment DB"
    }
  }

  alb -> apigw
  apigw -> user_svc
  apigw -> order_svc
  apigw -> payment_svc
  user_svc -> user_db
  order_svc -> order_db
  payment_svc -> payment_db
  order_svc -> payment_svc
}
```

## Configuration

Enable the tool by setting:

```bash
ENABLE_ADDITIONAL_TOOLS="aws_diagram"
```

In MCP client configuration:
```json
{
  "env": {
    "ENABLE_ADDITIONAL_TOOLS": "aws_diagram"
  }
}
```

## Output

Generated diagrams are saved to a `generated-diagrams` subdirectory in the specified workspace (or current directory). The tool returns:

- File paths for all generated formats
- DOT source code
- Generation timestamp
- File sizes and metadata

## Learning Path for AI Agents

1. **Start with Examples**: Call `{"action": "examples", "diagram_type": "aws"}` to see patterns
2. **Discover Icons**: Use `{"action": "list_icons", "provider": "aws"}` to find available components
3. **Practice Basic Patterns**: Begin with simple node-connection diagrams
4. **Add Complexity**: Introduce clusters and multiple connections
5. **Iterate**: Refine diagrams based on requirements

## Troubleshooting

**Tool not available:**
- Ensure `ENABLE_ADDITIONAL_TOOLS` includes `aws_diagram`
- Check that the MCP client configuration is properly set

**Parsing errors:**
- Verify all node IDs are defined before use in connections
- Ensure clusters contain nodes or other clusters
- Check that connection syntax uses `->` operator

**Empty output files:**
- This indicates a Graphviz rendering issue
- The DOT content is still generated and can be used with external tools
- Try different output formats (SVG often works better than PNG)

## Implementation Notes

- Uses embedded Graphviz WASM for zero external dependencies
- Implements proper file permissions (0600/0700) for security
- Integrates with the MCP DevTools security system
- Supports workspace-aware output directory handling
- Provides comprehensive error handling and validation

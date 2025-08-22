# AWS Diagram Tool

**Note:** This tool is disabled by default. To enable it, set the `ENABLE_ADDITIONAL_TOOLS` environment variable to include `aws-diagram`.

## Overview

The AWS Diagram tool generates architecture diagrams using a native Go implementation with embedded Graphviz. It's specifically designed for AI coding agents and provides a simple, unified interface for creating, discovering, and learning from diagram patterns.

## Features

- **Native Go Implementation**: No external Python dependencies - uses embedded WASM Graphviz
- **AI-Friendly DSL**: Simple text format optimised for AI agent consumption
- **Multi-format Output**: Supports PNG, SVG, PDF, and DOT formats
- **Progressive Examples**: Learn from basic to advanced diagram patterns
- **Icon Discovery**: Browse AWS, GCP, Kubernetes, and generic icons
- **Action-Based API**: Single tool with multiple functions

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
  "definition": "diagram \"My Architecture\" { node web = aws.ec2 \"Web Server\"; node db = aws.rds \"Database\"; web -> db }",
  "output_format": ["png", "svg"],
  "filename": "my-architecture",
  "workspace_dir": "/path/to/output"
}
```

**Parameters:**
- `definition` (required): Diagram definition in text DSL format
- `output_format` (optional): Array of formats - `["png"]`, `["svg"]`, `["pdf"]`, `["dot"]` (default: `["png"]`)
- `filename` (optional): Custom filename without extension (auto-generated if not provided)
- `workspace_dir` (optional): Output directory (defaults to current working directory)

## Diagram Syntax

The tool uses a simple, AI-friendly DSL:

### Basic Structure

```
diagram "Diagram Title" {
  node id = provider.service "Label"
  node2 = aws.ec2 "Web Server"
  
  id -> node2
}
```

### Supported Providers

- **AWS**: `aws.ec2`, `aws.rds`, `aws.lambda`, `aws.s3`, `aws.alb`, etc.
- **GCP**: `gcp.compute`, `gcp.storage`, etc.
- **Kubernetes**: `k8s.pod`, `k8s.service`, etc.
- **Generic**: `generic.server`, `generic.database`, `generic.client`, etc.

### Advanced Features

**Clusters/Groups:**
```
cluster vpc "Production VPC" {
  node web = aws.ec2 "Web Server"
  node db = aws.rds "Database"
}

web -> db
```

**Multiple Connections:**
```
node web = aws.ec2 "Web Server"
node db = aws.rds "Database"
node cache = aws.elasticache "Cache"

web -> db
web -> cache
```

**Connection Labels:**
```
web -> db [label="SQL"]
```

## Examples

### Simple 3-Tier Architecture

```
diagram "3-Tier Web Application" {
  node alb = aws.alb "Application Load Balancer"
  node web = aws.ec2 "Web Server"
  node db = aws.rds "Database"
  
  alb -> web -> db
}
```

### Serverless Architecture

```
diagram "Serverless Architecture" {
  node user = generic.client "User"
  node api = aws.apigateway "API Gateway"
  node lambda = aws.lambda "Lambda Function"
  node dynamodb = aws.dynamodb "DynamoDB"
  node s3 = aws.s3 "S3 Bucket"
  
  user -> api -> lambda
  lambda -> dynamodb
  lambda -> s3
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
ENABLE_ADDITIONAL_TOOLS="aws-diagram"
```

In MCP client configuration:
```json
{
  "env": {
    "ENABLE_ADDITIONAL_TOOLS": "aws-diagram"
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
- Ensure `ENABLE_ADDITIONAL_TOOLS` includes `aws-diagram`
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
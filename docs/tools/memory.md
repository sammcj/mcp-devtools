# Memory Tool

The Memory tool provides persistent knowledge graph storage for AI agents, allowing them to store, search, and retrieve information across sessions using a structured entity-relation model.

## Overview

The Memory tool creates a persistent knowledge graph that survives across different sessions. Store entities (people, companies, projects), create relationships between them, and add observations (facts) that can be searched and retrieved later.

## Features

- **Entity Storage**: Store named entities with types and facts
- **Relationship Mapping**: Create directed relationships between entities
- **Persistent Storage**: Data survives across sessions
- **Fuzzy Search**: Find entities with partial or approximate names
- **Namespaces**: Separate memory spaces for different projects
- **Concurrent Access**: Safe multi-process access with file locking
- **Structured Queries**: Retrieve specific entities or search broadly

## Core Concepts

### Entities
Named objects with types and observations:
- **Name**: Unique identifier (e.g., "John_Doe", "TechCorp")
- **Type**: Category (e.g., "person", "company", "project")
- **Observations**: List of facts about the entity

### Relations
Directed connections between entities:
- **From/To**: Source and target entities
- **Type**: Relationship category (e.g., "works_at", "created_by")

### Namespaces
Separate memory spaces for different contexts:
- **Default**: General-purpose storage
- **Project-specific**: Isolated memory for different projects

## Configuration

### Environment Variables
```bash
MEMORY_FILE_PATH="~/.mcp-devtools/"           # Storage location
MEMORY_ENABLE_FUZZY_SEARCH="true"             # Enable fuzzy search
```

### Storage Structure
```
~/.mcp-devtools/
├── default/
│   ├── memory.json      # Default namespace
│   └── memory.json.lock # Lock file
├── project_name/
│   ├── memory.json      # Project-specific namespace
│   └── memory.json.lock # Lock file
```

## Usage Examples

While intended to be activated via a prompt to an agent, below are some example JSON tool calls.

### Creating Entities
```json
{
  "name": "memory",
  "arguments": {
    "operation": "create_entities",
    "namespace": "my_project",
    "data": {
      "entities": [
        {
          "name": "John_Doe",
          "entityType": "person",
          "observations": [
            "Software engineer at TechCorp",
            "Specialises in Go and Python",
            "Lives in San Francisco"
          ]
        },
        {
          "name": "TechCorp",
          "entityType": "company",
          "observations": [
            "Technology company founded in 2010",
            "Headquarters in Silicon Valley",
            "Focuses on cloud infrastructure"
          ]
        }
      ]
    }
  }
}
```

### Creating Relationships
```json
{
  "name": "memory",
  "arguments": {
    "operation": "create_relations",
    "namespace": "my_project",
    "data": {
      "relations": [
        {
          "from": "John_Doe",
          "to": "TechCorp",
          "relationType": "works_at"
        },
        {
          "from": "John_Doe",
          "to": "CloudPlatform_Project",
          "relationType": "leads"
        }
      ]
    }
  }
}
```

### Adding Observations
```json
{
  "name": "memory",
  "arguments": {
    "operation": "add_observations",
    "namespace": "my_project",
    "data": {
      "observations": [
        {
          "entityName": "John_Doe",
          "contents": [
            "Recently promoted to senior engineer",
            "Working on microservices architecture",
            "Mentoring junior developers"
          ]
        }
      ]
    }
  }
}
```

### Searching Entities
```json
{
  "name": "memory",
  "arguments": {
    "operation": "search_nodes",
    "namespace": "my_project",
    "data": {
      "query": "engineer"
    }
  }
}
```

### Retrieving Specific Entities
```json
{
  "name": "memory",
  "arguments": {
    "operation": "open_nodes",
    "namespace": "my_project",
    "data": {
      "names": ["John_Doe", "TechCorp"]
    }
  }
}
```

### Reading Complete Graph
```json
{
  "name": "memory",
  "arguments": {
    "operation": "read_graph",
    "namespace": "my_project"
  }
}
```

## Operations Reference

### Core Operations
| Operation | Purpose | Data Required |
|-----------|---------|---------------|
| `create_entities` | Add new entities | `entities` array |
| `create_relations` | Add relationships | `relations` array |
| `add_observations` | Add facts to entities | `observations` array |
| `search_nodes` | Search entities | `query` string |
| `open_nodes` | Get specific entities | `names` array |
| `read_graph` | Get complete graph | None |

### Destructive Operations
| Operation | Purpose | Data Required | ⚠️ Warning |
|-----------|---------|---------------|------------|
| `delete_entities` | Remove entities | `names` array | Cascades to relations |
| `delete_relations` | Remove relationships | `relations` array | Permanent removal |
| `delete_observations` | Remove facts | `observations` array | Cannot be undone |

## Common Workflows

### Project Documentation Workflow
```json
// 1. Create project entities
{
  "operation": "create_entities",
  "data": {
    "entities": [
      {
        "name": "APIGateway_Project",
        "entityType": "project",
        "observations": ["REST API gateway for microservices", "Built with Go and Docker"]
      }
    ]
  }
}

// 2. Add team members
{
  "operation": "create_entities",
  "data": {
    "entities": [
      {
        "name": "Alice_Smith",
        "entityType": "person",
        "observations": ["Lead developer", "API design expert"]
      }
    ]
  }
}

// 3. Create relationships
{
  "operation": "create_relations",
  "data": {
    "relations": [
      {
        "from": "Alice_Smith",
        "to": "APIGateway_Project",
        "relationType": "leads"
      }
    ]
  }
}
```

### Research and Learning Workflow
```json
// 1. Store concepts learned
{
  "operation": "create_entities",
  "data": {
    "entities": [
      {
        "name": "Kubernetes_Ingress",
        "entityType": "concept",
        "observations": [
          "Manages external access to services",
          "Provides load balancing and SSL termination",
          "Uses nginx-ingress controller"
        ]
      }
    ]
  }
}

// 2. Connect related concepts
{
  "operation": "create_relations",
  "data": {
    "relations": [
      {
        "from": "Kubernetes_Ingress",
        "to": "Load_Balancing",
        "relationType": "implements"
      }
    ]
  }
}

// 3. Add new learnings
{
  "operation": "add_observations",
  "data": {
    "observations": [
      {
        "entityName": "Kubernetes_Ingress",
        "contents": ["Supports path-based and host-based routing"]
      }
    ]
  }
}
```

### Meeting Notes Workflow
```json
// 1. Create meeting entity
{
  "operation": "create_entities",
  "data": {
    "entities": [
      {
        "name": "Architecture_Review_2025_01_14",
        "entityType": "meeting",
        "observations": [
          "Discussed microservices migration strategy",
          "Decided on API-first approach",
          "Timeline: 3 months for phase 1"
        ]
      }
    ]
  }
}

// 2. Link attendees
{
  "operation": "create_relations",
  "data": {
    "relations": [
      {
        "from": "John_Doe",
        "to": "Architecture_Review_2025_01_14",
        "relationType": "attended"
      }
    ]
  }
}
```

## Best Practices

### Entity Naming
- **Use unique identifiers**: `John_Doe` not `John`
- **No spaces**: Use underscores for readability
- **Be consistent**: Same naming convention across all entities
- **Descriptive**: `APIGateway_Project` vs `Project1`

### Observation Guidelines
- **One fact per observation**: Keep observations atomic
- **Be specific**: "Promotes code reuse" vs "Good for development"
- **Include context**: "Performance improved by 30% after Redis caching"
- **Update, don't duplicate**: Add new observations instead of repeating similar facts

### Relationship Types
- **Use active voice**: `works_at`, `manages`, `created_by`
- **Be directional**: From subject to object
- **Standard conventions**: `leads`, `reports_to`, `part_of`, `implements`
- **Consistent naming**: Use same relationship types across similar connections

### Namespace Strategy
- **Project separation**: Use project names as namespaces
- **Context isolation**: Separate personal vs work vs learning
- **Default for general**: Use default namespace for broadly useful entities
- **Descriptive names**: `customer_project_2024` vs `cp1`

## Search Capabilities

### Fuzzy Search Examples
The tool finds entities even with partial or approximate matches:

```json
// Query: "engineer"
// Finds: "John_Doe" (has "Software engineer" in observations)

// Query: "tech"
// Finds: "TechCorp", "Technology_Trends", "TechConf_2024"

// Query: "api gateway"
// Finds: "APIGateway_Project", "Gateway_Service"
```

### Search Strategies
- **Broad terms**: Search for categories like "project", "person", "api"
- **Specific names**: Find exact entities with partial names
- **Skill-based**: Search for "python", "kubernetes", "design"
- **Topic-based**: Find "authentication", "performance", "security"

## Performance Considerations

### Storage
- **JSONL format**: Efficient append operations
- **File locking**: Prevents corruption during concurrent access
- **Memory usage**: Entire graph loaded for operations

### Search Performance
- **Fuzzy search**: Enabled by default, can be disabled for performance
- **Large datasets**: Consider splitting across namespaces
- **Indexing**: Built-in indexing for entity names and types

### Concurrent Access
- **Advisory locking**: Prevents data corruption between processes
- **Atomic operations**: Uses temporary files with atomic rename
- **Read/write locks**: Allow concurrent reads, exclusive writes

## Error Handling

### Common Issues
**Entity Not Found**
```json
{"error": "Entity 'Unknown_Person' not found in namespace 'my_project'"}
```

**Relationship Target Missing**
```json
{"error": "Cannot create relation: target entity 'NonExistent_Company' not found"}
```

**Permission Denied**
```json
{"error": "Cannot write to memory file: permission denied"}
```

### Troubleshooting
1. **Check entity names**: Case-sensitive, exact matches required
2. **Verify namespace**: Ensure using correct namespace
3. **File permissions**: Check write access to memory directory
4. **Lock conflicts**: Wait and retry if another process is accessing

## Integration Examples

### Code Analysis Workflow
```bash
# Store findings about codebase
memory create_entities --namespace "codebase_analysis" --data '{
  "entities": [{
    "name": "Authentication_Service",
    "entityType": "service",
    "observations": ["Uses JWT tokens", "Redis for session storage", "Rate limiting implemented"]
  }]
}'

# Link to developers
memory create_relations --data '{
  "relations": [{
    "from": "Alice_Smith",
    "to": "Authentication_Service",
    "relationType": "maintains"
  }]
}'
```

### Learning and Research
```bash
# Store new concepts
memory create_entities --namespace "learning" --data '{
  "entities": [{
    "name": "Event_Sourcing",
    "entityType": "pattern",
    "observations": ["Stores events instead of current state", "Enables audit trails", "Complex to implement"]
  }]
}'

# Connect to related patterns
memory create_relations --data '{
  "relations": [{
    "from": "Event_Sourcing",
    "to": "CQRS_Pattern",
    "relationType": "often_used_with"
  }]
}'
```

---

For technical implementation details, see the [Memory tool source documentation](../../internal/tools/memory/README.md).

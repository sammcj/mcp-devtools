# Memory Tool

The Memory Tool provides persistent knowledge graph storage capabilities for AI coding agents, allowing them to store, search, and retrieve memories across sessions using a structured entity-relation model.

## Overview

The memory tool implements a knowledge graph system that stores:
- **Entities**: Named nodes with types and observations (facts)
- **Relations**: Directed connections between entities
- **Observations**: Discrete facts attached to entities

Data is stored in JSONL (JSON Lines) format for efficient operations and concurrent access safety.

## Features

### Core Operations
- **create_entities**: Create new entities with observations
- **create_relations**: Create directed relationships between entities
- **add_observations**: Add new facts to existing entities
- **delete_entities**: Remove entities (cascades to relations)
- **delete_observations**: Remove specific observations from entities
- **delete_relations**: Remove specific relationships
- **read_graph**: Retrieve the complete knowledge graph
- **search_nodes**: Search entities using text queries with fuzzy matching
- **open_nodes**: Retrieve specific entities by name

### Advanced Features
- **Namespaces**: Separate memory spaces for different projects/contexts
- **Fuzzy Search**: Enhanced search using the `sahilm/fuzzy` library
- **Concurrent Access**: File locking prevents data corruption
- **Atomic Operations**: Temporary files ensure data consistency
- **Configurable Storage**: Environment variable configuration

## Configuration

### Environment Variables

| Variable                     | Description                                    | Default            |
|------------------------------|------------------------------------------------|--------------------|
| `MEMORY_FILE_PATH`           | Base directory or file path for memory storage | `~/.mcp-devtools/` |
| `MEMORY_ENABLE_FUZZY_SEARCH` | Enable fuzzy search capabilities               | `true`             |

### Storage Structure

```
~/.mcp-devtools/
├── default/
│   ├── memory.json      # Default namespace
│   └── memory.json.lock # Lock file
├── project_name/
│   ├── memory.json      # Project-specific namespace
│   └── memory.json.lock # Lock file
└── another_project/
    ├── memory.json
    └── memory.json.lock
```

## Usage Examples

### Creating Entities

```json
{
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
          "Technology company",
          "Founded in 2010",
          "Headquarters in Silicon Valley"
        ]
      }
    ]
  }
}
```

### Creating Relations

```json
{
  "operation": "create_relations",
  "namespace": "my_project",
  "data": {
    "relations": [
      {
        "from": "John_Doe",
        "to": "TechCorp",
        "relationType": "works_at"
      }
    ]
  }
}
```

### Searching Nodes

```json
{
  "operation": "search_nodes",
  "namespace": "my_project",
  "data": {
    "query": "engineer"
  }
}
```

### Adding Observations

```json
{
  "operation": "add_observations",
  "namespace": "my_project",
  "data": {
    "observations": [
      {
        "entityName": "John_Doe",
        "contents": [
          "Recently promoted to senior engineer",
          "Working on microservices architecture"
        ]
      }
    ]
  }
}
```

## Data Model

### Entity Structure
```go
type Entity struct {
    Name         string   `json:"name"`         // Unique identifier
    EntityType   string   `json:"entityType"`   // Category (person, company, etc.)
    Observations []string `json:"observations"` // List of facts
}
```

### Relation Structure
```go
type Relation struct {
    From         string `json:"from"`         // Source entity name
    To           string `json:"to"`           // Target entity name
    RelationType string `json:"relationType"` // Relationship type
}
```

### Knowledge Graph Structure
```go
type KnowledgeGraph struct {
    Entities  []Entity  `json:"entities"`
    Relations []Relation `json:"relations"`
}
```

## Best Practices

### Entity Naming
- Use unique, descriptive names without spaces
- Use underscores for multi-word names: `John_Doe`, `TechCorp_Inc`
- Avoid special characters that might cause parsing issues
- Keep names consistent across operations

### Observations
- Keep observations atomic (one fact per observation)
- Use clear, descriptive language
- Avoid redundant information
- Update rather than duplicate similar facts

### Relations
- Use active voice for relation types: `works_at`, `manages`, `created_by`
- Ensure both entities exist before creating relations
- Use consistent relation type naming across your domain

### Namespaces
- Use descriptive namespace names for different projects/contexts
- Default namespace is suitable for general-purpose storage
- Consider data isolation needs when choosing namespaces

## Error Handling

The memory tool provides comprehensive error handling:

- **Validation Errors**: Invalid entity names, missing required fields
- **Relationship Errors**: Relations referencing non-existent entities
- **File System Errors**: Permission issues, disk space, corruption
- **Concurrency Errors**: Lock acquisition failures, concurrent access

## Performance Considerations

### Search Performance
- Fuzzy search is enabled by default but can be disabled for performance
- Large knowledge graphs may experience slower search times
- Consider splitting large datasets across multiple namespaces

### Storage Performance
- JSONL format provides efficient append operations
- File locking ensures consistency but may impact concurrent performance
- Regular backups recommended for important data

### Memory Usage
- Entire knowledge graph is loaded into memory for operations
- Large graphs may require significant RAM
- Consider splitting large datasets across namespaces

## Concurrency and Safety

### File Locking
- Advisory file locking prevents corruption between processes
- Read locks allow concurrent reads
- Write locks ensure exclusive access during modifications

### Atomic Operations
- All write operations use temporary files with atomic rename
- Ensures data consistency even if process is interrupted
- Lock files prevent concurrent access during operations

## Troubleshooting

### Common Issues

**Permission Denied**
- Ensure write permissions to the memory directory
- Check if another process has exclusive locks

**Entity Not Found**
- Verify entity names match exactly (case-sensitive)
- Check if you're using the correct namespace

**Lock Acquisition Failed**
- Another process may be accessing the same memory file
- Wait and retry, or check for stuck processes

**Corrupted Data**
- Check file permissions and disk space
- Restore from backup if available
- Validate JSON format manually

### Debugging

Enable debug logging by setting the debug flag when running mcp-devtools:

```bash
./bin/mcp-devtools --debug stdio
```

Check memory file contents directly:
```bash
cat ~/.mcp-devtools/default/memory.json
```

## Integration with AI Agents

The memory tool is designed specifically for AI coding agents:

### Tool Descriptions
- Comprehensive parameter descriptions help agents make informed decisions
- Clear operation documentation prevents misuse
- Warning annotations highlight destructive operations

### Batch Operations
- Most operations support batch processing for efficiency
- Prefer batch operations over individual calls when possible
- Reduces file I/O and improves performance

### Search Capabilities
- Fuzzy search helps agents find relevant information with partial queries
- Multiple match types (exact, partial, fuzzy) provide flexibility
- Search results include relevance scores for ranking

## Architecture

### Package Structure
```
internal/tools/memory/
├── memory.go     # MCP tool implementation
├── graph.go      # Knowledge graph operations
├── storage.go    # File I/O and persistence
├── types.go      # Data structures and types
└── README.md     # This documentation
```

### Dependencies
- `github.com/sahilm/fuzzy` - Fuzzy string matching
- `github.com/gofrs/flock` - Cross-platform file locking
- `github.com/sirupsen/logrus` - Structured logging
- `github.com/mark3labs/mcp-go/mcp` - MCP framework

### Design Principles
- **Simplicity**: Clean, straightforward implementation
- **Reliability**: Robust error handling and data consistency
- **Performance**: Efficient operations for typical usage patterns
- **Extensibility**: Modular design allows for future enhancements

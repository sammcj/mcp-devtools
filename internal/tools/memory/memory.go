package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
)

// MemoryTool implements the MCP memory tool for knowledge graph operations
type MemoryTool struct {
	graphManager *GraphManager
}

// init registers the memory tool
func init() {
	registry.Register(&MemoryTool{})
}

// Definition returns the tool's definition for MCP registration
func (m *MemoryTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"memory",
		mcp.WithDescription(`Persistent knowledge graph memory system for AI agents. Stores entities, relations, and observations across sessions.

- **Entities** MUST be created before relations can reference them
- **Destructive operations** (delete_*) permanently remove data - use carefully!
- **Batch operations** are more efficient than individual calls
- **Entity names** should be unique identifiers without spaces or special characters
- **Relations** are directional (from -> to) and should use active voice
- **Observations** are discrete facts - keep them atomic (one fact per observation)`),

		// Subcommand parameter to specify the operation
		mcp.WithString("operation",
			mcp.Required(),
			mcp.Description("Memory operation to perform"),
			mcp.Enum(
				"create_entities",
				"create_relations",
				"add_observations",
				"delete_entities",
				"delete_observations",
				"delete_relations",
				"read_graph",
				"search_nodes",
				"open_nodes",
			),
		),

		// Data parameter for operation-specific input
		mcp.WithObject("data",
			mcp.Description("Operation-specific data (structure varies by operation)"),
		),

		// Namespace parameter for memory separation
		mcp.WithString("namespace",
			mcp.Description("Memory namespace for organising memories into separate projects/contexts (default: 'default')"),
			mcp.DefaultString("default"),
		),
	)
}

// Execute executes the memory tool operations
func (m *MemoryTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse namespace parameter (default: "default")
	namespace := "default"
	if namespaceRaw, exists := args["namespace"]; exists && namespaceRaw != nil {
		if namespaceStr, ok := namespaceRaw.(string); ok && namespaceStr != "" {
			namespace = namespaceStr
		}
	}

	// Initialise graph manager with namespace if not already done
	if m.graphManager == nil {
		gm, err := NewGraphManagerWithNamespace(logger, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to initialise graph manager: %w", err)
		}
		m.graphManager = gm
	} else {
		// Update namespace for existing graph manager
		if err := m.graphManager.SetNamespace(namespace); err != nil {
			return nil, fmt.Errorf("failed to set namespace: %w", err)
		}
	}

	// Parse operation parameter
	operation, ok := args["operation"].(string)
	if !ok || operation == "" {
		return nil, fmt.Errorf("missing or invalid required parameter: operation")
	}

	// Parse data parameter (optional for some operations)
	var data map[string]interface{}
	if dataRaw, exists := args["data"]; exists && dataRaw != nil {
		if dataMap, ok := dataRaw.(map[string]interface{}); ok {
			data = dataMap
		} else {
			return nil, fmt.Errorf("data parameter must be an object")
		}
	}

	// Execute the requested operation
	switch operation {
	case "create_entities":
		return m.handleCreateEntities(data)
	case "create_relations":
		return m.handleCreateRelations(data)
	case "add_observations":
		return m.handleAddObservations(data)
	case "delete_entities":
		return m.handleDeleteEntities(data)
	case "delete_observations":
		return m.handleDeleteObservations(data)
	case "delete_relations":
		return m.handleDeleteRelations(data)
	case "read_graph":
		return m.handleReadGraph()
	case "search_nodes":
		return m.handleSearchNodes(data)
	case "open_nodes":
		return m.handleOpenNodes(data)
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// handleCreateEntities handles entity creation
func (m *MemoryTool) handleCreateEntities(data map[string]interface{}) (*mcp.CallToolResult, error) {
	if data == nil {
		return nil, fmt.Errorf("data parameter is required for create_entities operation")
	}

	entitiesRaw, exists := data["entities"]
	if !exists {
		return nil, fmt.Errorf("entities parameter is required")
	}

	// Parse entities
	var entities []Entity
	entitiesJSON, err := json.Marshal(entitiesRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal entities: %w", err)
	}
	if err := json.Unmarshal(entitiesJSON, &entities); err != nil {
		return nil, fmt.Errorf("failed to parse entities: %w", err)
	}

	// Create entities
	createdEntities, err := m.graphManager.CreateEntities(entities)
	if err != nil {
		return nil, fmt.Errorf("failed to create entities: %w", err)
	}

	response := CreateEntitiesResponse{
		CreatedEntities: createdEntities,
		Timestamp:       time.Now(),
	}

	return m.newToolResultJSON(response)
}

// handleCreateRelations handles relation creation
func (m *MemoryTool) handleCreateRelations(data map[string]interface{}) (*mcp.CallToolResult, error) {
	if data == nil {
		return nil, fmt.Errorf("data parameter is required for create_relations operation")
	}

	relationsRaw, exists := data["relations"]
	if !exists {
		return nil, fmt.Errorf("relations parameter is required")
	}

	// Parse relations
	var relations []Relation
	relationsJSON, err := json.Marshal(relationsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal relations: %w", err)
	}
	if err := json.Unmarshal(relationsJSON, &relations); err != nil {
		return nil, fmt.Errorf("failed to parse relations: %w", err)
	}

	// Create relations
	createdRelations, err := m.graphManager.CreateRelations(relations)
	if err != nil {
		return nil, fmt.Errorf("failed to create relations: %w", err)
	}

	response := CreateRelationsResponse{
		CreatedRelations: createdRelations,
		Timestamp:        time.Now(),
	}

	return m.newToolResultJSON(response)
}

// handleAddObservations handles adding observations
func (m *MemoryTool) handleAddObservations(data map[string]interface{}) (*mcp.CallToolResult, error) {
	if data == nil {
		return nil, fmt.Errorf("data parameter is required for add_observations operation")
	}

	observationsRaw, exists := data["observations"]
	if !exists {
		return nil, fmt.Errorf("observations parameter is required")
	}

	// Parse observations
	var observations []ObservationInput
	observationsJSON, err := json.Marshal(observationsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal observations: %w", err)
	}
	if err := json.Unmarshal(observationsJSON, &observations); err != nil {
		return nil, fmt.Errorf("failed to parse observations: %w", err)
	}

	// Add observations
	results, err := m.graphManager.AddObservations(observations)
	if err != nil {
		return nil, fmt.Errorf("failed to add observations: %w", err)
	}

	response := AddObservationsResponse{
		Results:   results,
		Timestamp: time.Now(),
	}

	return m.newToolResultJSON(response)
}

// handleDeleteEntities handles entity deletion
func (m *MemoryTool) handleDeleteEntities(data map[string]interface{}) (*mcp.CallToolResult, error) {
	if data == nil {
		return nil, fmt.Errorf("data parameter is required for delete_entities operation")
	}

	entityNamesRaw, exists := data["entityNames"]
	if !exists {
		return nil, fmt.Errorf("entityNames parameter is required")
	}

	// Parse entity names
	var entityNames []string
	entityNamesJSON, err := json.Marshal(entityNamesRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal entityNames: %w", err)
	}
	if err := json.Unmarshal(entityNamesJSON, &entityNames); err != nil {
		return nil, fmt.Errorf("failed to parse entityNames: %w", err)
	}

	// Delete entities
	if err := m.graphManager.DeleteEntities(entityNames); err != nil {
		return nil, fmt.Errorf("failed to delete entities: %w", err)
	}

	response := MemoryOperationResponse{
		Message:   fmt.Sprintf("Successfully deleted %d entities and cascaded relations", len(entityNames)),
		Timestamp: time.Now(),
	}

	return m.newToolResultJSON(response)
}

// handleDeleteObservations handles observation deletion
func (m *MemoryTool) handleDeleteObservations(data map[string]interface{}) (*mcp.CallToolResult, error) {
	if data == nil {
		return nil, fmt.Errorf("data parameter is required for delete_observations operation")
	}

	deletionsRaw, exists := data["deletions"]
	if !exists {
		return nil, fmt.Errorf("deletions parameter is required")
	}

	// Parse deletions
	var deletions []ObservationDeletion
	deletionsJSON, err := json.Marshal(deletionsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deletions: %w", err)
	}
	if err := json.Unmarshal(deletionsJSON, &deletions); err != nil {
		return nil, fmt.Errorf("failed to parse deletions: %w", err)
	}

	// Delete observations
	if err := m.graphManager.DeleteObservations(deletions); err != nil {
		return nil, fmt.Errorf("failed to delete observations: %w", err)
	}

	response := MemoryOperationResponse{
		Message:   "Successfully deleted observations from entities",
		Timestamp: time.Now(),
	}

	return m.newToolResultJSON(response)
}

// handleDeleteRelations handles relation deletion
func (m *MemoryTool) handleDeleteRelations(data map[string]interface{}) (*mcp.CallToolResult, error) {
	if data == nil {
		return nil, fmt.Errorf("data parameter is required for delete_relations operation")
	}

	relationsRaw, exists := data["relations"]
	if !exists {
		return nil, fmt.Errorf("relations parameter is required")
	}

	// Parse relations
	var relations []Relation
	relationsJSON, err := json.Marshal(relationsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal relations: %w", err)
	}
	if err := json.Unmarshal(relationsJSON, &relations); err != nil {
		return nil, fmt.Errorf("failed to parse relations: %w", err)
	}

	// Delete relations
	if err := m.graphManager.DeleteRelations(relations); err != nil {
		return nil, fmt.Errorf("failed to delete relations: %w", err)
	}

	response := MemoryOperationResponse{
		Message:   fmt.Sprintf("Successfully deleted %d relations", len(relations)),
		Timestamp: time.Now(),
	}

	return m.newToolResultJSON(response)
}

// handleReadGraph handles reading the complete graph
func (m *MemoryTool) handleReadGraph() (*mcp.CallToolResult, error) {
	graph, err := m.graphManager.ReadGraph()
	if err != nil {
		return nil, fmt.Errorf("failed to read graph: %w", err)
	}

	return m.newToolResultJSON(graph)
}

// handleSearchNodes handles node searching
func (m *MemoryTool) handleSearchNodes(data map[string]interface{}) (*mcp.CallToolResult, error) {
	if data == nil {
		return nil, fmt.Errorf("data parameter is required for search_nodes operation")
	}

	queryRaw, exists := data["query"]
	if !exists {
		return nil, fmt.Errorf("query parameter is required")
	}

	query, ok := queryRaw.(string)
	if !ok {
		return nil, fmt.Errorf("query parameter must be a string")
	}

	// Search nodes
	graph, results, err := m.graphManager.SearchNodes(query)
	if err != nil {
		return nil, fmt.Errorf("failed to search nodes: %w", err)
	}

	response := SearchNodesResponse{
		Graph:     *graph,
		Results:   results,
		Query:     query,
		Timestamp: time.Now(),
	}

	return m.newToolResultJSON(response)
}

// handleOpenNodes handles opening specific nodes
func (m *MemoryTool) handleOpenNodes(data map[string]interface{}) (*mcp.CallToolResult, error) {
	if data == nil {
		return nil, fmt.Errorf("data parameter is required for open_nodes operation")
	}

	namesRaw, exists := data["names"]
	if !exists {
		return nil, fmt.Errorf("names parameter is required")
	}

	// Parse names
	var names []string
	namesJSON, err := json.Marshal(namesRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal names: %w", err)
	}
	if err := json.Unmarshal(namesJSON, &names); err != nil {
		return nil, fmt.Errorf("failed to parse names: %w", err)
	}

	// Open nodes
	graph, err := m.graphManager.OpenNodes(names)
	if err != nil {
		return nil, fmt.Errorf("failed to open nodes: %w", err)
	}

	return m.newToolResultJSON(graph)
}

// newToolResultJSON creates a new tool result with JSON content
func (m *MemoryTool) newToolResultJSON(data interface{}) (*mcp.CallToolResult, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

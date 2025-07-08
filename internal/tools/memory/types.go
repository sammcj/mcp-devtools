package memory

import "time"

// Entity represents a node in the knowledge graph with a name, type, and observations
type Entity struct {
	Name         string   `json:"name"`
	EntityType   string   `json:"entityType"`
	Observations []string `json:"observations"`
}

// Relation represents a directed connection between two entities
type Relation struct {
	From         string `json:"from"`
	To           string `json:"to"`
	RelationType string `json:"relationType"`
}

// KnowledgeGraph represents the complete graph structure
type KnowledgeGraph struct {
	Entities  []Entity   `json:"entities"`
	Relations []Relation `json:"relations"`
}

// StoredEntity represents an entity as stored in JSONL format
type StoredEntity struct {
	Type         string   `json:"type"`
	Name         string   `json:"name"`
	EntityType   string   `json:"entityType"`
	Observations []string `json:"observations"`
}

// StoredRelation represents a relation as stored in JSONL format
type StoredRelation struct {
	Type         string `json:"type"`
	From         string `json:"from"`
	To           string `json:"to"`
	RelationType string `json:"relationType"`
}

// Request types for each operation

// CreateEntitiesRequest represents the input for creating entities
type CreateEntitiesRequest struct {
	Entities []Entity `json:"entities"`
}

// CreateRelationsRequest represents the input for creating relations
type CreateRelationsRequest struct {
	Relations []Relation `json:"relations"`
}

// AddObservationsRequest represents the input for adding observations
type AddObservationsRequest struct {
	Observations []ObservationInput `json:"observations"`
}

// ObservationInput represents observations to add to a specific entity
type ObservationInput struct {
	EntityName string   `json:"entityName"`
	Contents   []string `json:"contents"`
}

// DeleteEntitiesRequest represents the input for deleting entities
type DeleteEntitiesRequest struct {
	EntityNames []string `json:"entityNames"`
}

// DeleteObservationsRequest represents the input for deleting observations
type DeleteObservationsRequest struct {
	Deletions []ObservationDeletion `json:"deletions"`
}

// ObservationDeletion represents observations to delete from a specific entity
type ObservationDeletion struct {
	EntityName   string   `json:"entityName"`
	Observations []string `json:"observations"`
}

// DeleteRelationsRequest represents the input for deleting relations
type DeleteRelationsRequest struct {
	Relations []Relation `json:"relations"`
}

// SearchNodesRequest represents the input for searching nodes
type SearchNodesRequest struct {
	Query string `json:"query"`
}

// OpenNodesRequest represents the input for opening specific nodes
type OpenNodesRequest struct {
	Names []string `json:"names"`
}

// Response types

// CreateEntitiesResponse represents the output of creating entities
type CreateEntitiesResponse struct {
	CreatedEntities []Entity  `json:"createdEntities"`
	Timestamp       time.Time `json:"timestamp"`
}

// CreateRelationsResponse represents the output of creating relations
type CreateRelationsResponse struct {
	CreatedRelations []Relation `json:"createdRelations"`
	Timestamp        time.Time  `json:"timestamp"`
}

// AddObservationsResponse represents the output of adding observations
type AddObservationsResponse struct {
	Results   []ObservationResult `json:"results"`
	Timestamp time.Time           `json:"timestamp"`
}

// ObservationResult represents the result of adding observations to an entity
type ObservationResult struct {
	EntityName        string   `json:"entityName"`
	AddedObservations []string `json:"addedObservations"`
}

// MemoryOperationResponse represents a generic response for operations that don't return specific data
type MemoryOperationResponse struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// SearchResult represents search results with relevance scoring
type SearchResult struct {
	Entity    Entity  `json:"entity"`
	Score     float64 `json:"score,omitempty"`
	MatchType string  `json:"matchType,omitempty"` // "exact", "fuzzy", "partial"
}

// SearchNodesResponse represents the output of searching nodes
type SearchNodesResponse struct {
	Graph     KnowledgeGraph `json:"graph"`
	Results   []SearchResult `json:"results,omitempty"`
	Query     string         `json:"query"`
	Timestamp time.Time      `json:"timestamp"`
}

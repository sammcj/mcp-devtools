package memory

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sahilm/fuzzy"
	"github.com/sirupsen/logrus"
)

// GraphManager handles knowledge graph operations
type GraphManager struct {
	storage      *Storage
	logger       *logrus.Logger
	fuzzyEnabled bool
}

// NewGraphManager creates a new graph manager instance with default namespace
func NewGraphManager(logger *logrus.Logger) (*GraphManager, error) {
	return NewGraphManagerWithNamespace(logger, "default")
}

// NewGraphManagerWithNamespace creates a new graph manager instance with specified namespace
func NewGraphManagerWithNamespace(logger *logrus.Logger, namespace string) (*GraphManager, error) {
	storage, err := NewStorageWithNamespace(logger, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Check if fuzzy search is enabled (default: true)
	fuzzyEnabled := true
	if envVal := os.Getenv("MEMORY_ENABLE_FUZZY_SEARCH"); envVal != "" {
		if parsed, err := strconv.ParseBool(envVal); err == nil {
			fuzzyEnabled = parsed
		}
	}

	return &GraphManager{
		storage:      storage,
		logger:       logger,
		fuzzyEnabled: fuzzyEnabled,
	}, nil
}

// SetNamespace changes the namespace for this graph manager
func (gm *GraphManager) SetNamespace(namespace string) error {
	return gm.storage.SetNamespace(namespace)
}

// CreateEntities creates new entities, ignoring duplicates
func (gm *GraphManager) CreateEntities(entities []Entity) ([]Entity, error) {
	graph, err := gm.storage.LoadGraph()
	if err != nil {
		return nil, fmt.Errorf("failed to load graph: %w", err)
	}

	// Create a map of existing entity names for quick lookup
	existingNames := make(map[string]bool)
	for _, entity := range graph.Entities {
		existingNames[entity.Name] = true
	}

	// Filter out entities that already exist
	var newEntities []Entity
	for _, entity := range entities {
		if err := gm.validateEntityName(entity.Name); err != nil {
			gm.logger.WithError(err).WithField("entity", entity.Name).Warn("Invalid entity name, skipping")
			continue
		}

		if !existingNames[entity.Name] {
			// Ensure observations slice is not nil
			if entity.Observations == nil {
				entity.Observations = []string{}
			}
			newEntities = append(newEntities, entity)
			graph.Entities = append(graph.Entities, entity)
		}
	}

	if len(newEntities) > 0 {
		if err := gm.storage.SaveGraph(graph); err != nil {
			return nil, fmt.Errorf("failed to save graph: %w", err)
		}
		gm.logger.WithField("count", len(newEntities)).Info("Created new entities")
	}

	return newEntities, nil
}

// CreateRelations creates new relations, skipping duplicates
func (gm *GraphManager) CreateRelations(relations []Relation) ([]Relation, error) {
	graph, err := gm.storage.LoadGraph()
	if err != nil {
		return nil, fmt.Errorf("failed to load graph: %w", err)
	}

	// Create a map of existing entity names for validation
	entityNames := make(map[string]bool)
	for _, entity := range graph.Entities {
		entityNames[entity.Name] = true
	}

	// Create a set of existing relations for duplicate detection
	existingRelations := make(map[string]bool)
	for _, relation := range graph.Relations {
		key := fmt.Sprintf("%s->%s:%s", relation.From, relation.To, relation.RelationType)
		existingRelations[key] = true
	}

	// Filter and validate new relations
	var newRelations []Relation
	for _, relation := range relations {
		// Validate that both entities exist
		if !entityNames[relation.From] {
			gm.logger.WithField("entity", relation.From).Warn("Source entity does not exist, skipping relation")
			continue
		}
		if !entityNames[relation.To] {
			gm.logger.WithField("entity", relation.To).Warn("Target entity does not exist, skipping relation")
			continue
		}

		// Check for duplicates
		key := fmt.Sprintf("%s->%s:%s", relation.From, relation.To, relation.RelationType)
		if !existingRelations[key] {
			newRelations = append(newRelations, relation)
			graph.Relations = append(graph.Relations, relation)
			existingRelations[key] = true
		}
	}

	if len(newRelations) > 0 {
		if err := gm.storage.SaveGraph(graph); err != nil {
			return nil, fmt.Errorf("failed to save graph: %w", err)
		}
		gm.logger.WithField("count", len(newRelations)).Info("Created new relations")
	}

	return newRelations, nil
}

// AddObservations adds new observations to existing entities
func (gm *GraphManager) AddObservations(observations []ObservationInput) ([]ObservationResult, error) {
	graph, err := gm.storage.LoadGraph()
	if err != nil {
		return nil, fmt.Errorf("failed to load graph: %w", err)
	}

	// Create a map for quick entity lookup
	entityMap := make(map[string]*Entity)
	for i := range graph.Entities {
		entityMap[graph.Entities[i].Name] = &graph.Entities[i]
	}

	var results []ObservationResult
	modified := false

	for _, obs := range observations {
		entity, exists := entityMap[obs.EntityName]
		if !exists {
			return nil, fmt.Errorf("entity '%s' does not exist", obs.EntityName)
		}

		// Filter out observations that already exist
		var newObservations []string
		for _, content := range obs.Contents {
			if content == "" {
				continue // Skip empty observations
			}

			// Check if observation already exists
			exists := false
			for _, existing := range entity.Observations {
				if existing == content {
					exists = true
					break
				}
			}

			if !exists {
				newObservations = append(newObservations, content)
				entity.Observations = append(entity.Observations, content)
			}
		}

		if len(newObservations) > 0 {
			modified = true
		}

		results = append(results, ObservationResult{
			EntityName:        obs.EntityName,
			AddedObservations: newObservations,
		})
	}

	if modified {
		if err := gm.storage.SaveGraph(graph); err != nil {
			return nil, fmt.Errorf("failed to save graph: %w", err)
		}
		gm.logger.WithField("entities", len(observations)).Info("Added observations to entities")
	}

	return results, nil
}

// DeleteEntities deletes entities and cascades to remove related relations
func (gm *GraphManager) DeleteEntities(entityNames []string) error {
	graph, err := gm.storage.LoadGraph()
	if err != nil {
		return fmt.Errorf("failed to load graph: %w", err)
	}

	// Create a set of names to delete for quick lookup
	toDelete := make(map[string]bool)
	for _, name := range entityNames {
		toDelete[name] = true
	}

	// Filter out entities to delete
	var remainingEntities []Entity
	deletedCount := 0
	for _, entity := range graph.Entities {
		if !toDelete[entity.Name] {
			remainingEntities = append(remainingEntities, entity)
		} else {
			deletedCount++
		}
	}

	// Filter out relations involving deleted entities
	var remainingRelations []Relation
	deletedRelationCount := 0
	for _, relation := range graph.Relations {
		if !toDelete[relation.From] && !toDelete[relation.To] {
			remainingRelations = append(remainingRelations, relation)
		} else {
			deletedRelationCount++
		}
	}

	if deletedCount > 0 || deletedRelationCount > 0 {
		graph.Entities = remainingEntities
		graph.Relations = remainingRelations

		if err := gm.storage.SaveGraph(graph); err != nil {
			return fmt.Errorf("failed to save graph: %w", err)
		}

		gm.logger.WithFields(logrus.Fields{
			"entities":  deletedCount,
			"relations": deletedRelationCount,
		}).Info("Deleted entities and cascaded relations")
	}

	return nil
}

// DeleteObservations deletes specific observations from entities
func (gm *GraphManager) DeleteObservations(deletions []ObservationDeletion) error {
	graph, err := gm.storage.LoadGraph()
	if err != nil {
		return fmt.Errorf("failed to load graph: %w", err)
	}

	// Create a map for quick entity lookup
	entityMap := make(map[string]*Entity)
	for i := range graph.Entities {
		entityMap[graph.Entities[i].Name] = &graph.Entities[i]
	}

	modified := false
	for _, deletion := range deletions {
		entity, exists := entityMap[deletion.EntityName]
		if !exists {
			continue // Silently skip non-existent entities
		}

		// Create a set of observations to delete
		toDelete := make(map[string]bool)
		for _, obs := range deletion.Observations {
			toDelete[obs] = true
		}

		// Filter out observations to delete
		var remainingObservations []string
		for _, obs := range entity.Observations {
			if !toDelete[obs] {
				remainingObservations = append(remainingObservations, obs)
			} else {
				modified = true
			}
		}

		entity.Observations = remainingObservations
	}

	if modified {
		if err := gm.storage.SaveGraph(graph); err != nil {
			return fmt.Errorf("failed to save graph: %w", err)
		}
		gm.logger.WithField("deletions", len(deletions)).Info("Deleted observations from entities")
	}

	return nil
}

// DeleteRelations deletes specific relations
func (gm *GraphManager) DeleteRelations(relations []Relation) error {
	graph, err := gm.storage.LoadGraph()
	if err != nil {
		return fmt.Errorf("failed to load graph: %w", err)
	}

	// Create a set of relations to delete
	toDelete := make(map[string]bool)
	for _, relation := range relations {
		key := fmt.Sprintf("%s->%s:%s", relation.From, relation.To, relation.RelationType)
		toDelete[key] = true
	}

	// Filter out relations to delete
	var remainingRelations []Relation
	deletedCount := 0
	for _, relation := range graph.Relations {
		key := fmt.Sprintf("%s->%s:%s", relation.From, relation.To, relation.RelationType)
		if !toDelete[key] {
			remainingRelations = append(remainingRelations, relation)
		} else {
			deletedCount++
		}
	}

	if deletedCount > 0 {
		graph.Relations = remainingRelations

		if err := gm.storage.SaveGraph(graph); err != nil {
			return fmt.Errorf("failed to save graph: %w", err)
		}

		gm.logger.WithField("count", deletedCount).Info("Deleted relations")
	}

	return nil
}

// ReadGraph returns the complete knowledge graph
func (gm *GraphManager) ReadGraph() (*KnowledgeGraph, error) {
	return gm.storage.LoadGraph()
}

// SearchNodes searches for nodes based on a query string
func (gm *GraphManager) SearchNodes(query string) (*KnowledgeGraph, []SearchResult, error) {
	graph, err := gm.storage.LoadGraph()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load graph: %w", err)
	}

	if query == "" {
		// Return entire graph if no query
		return graph, nil, nil
	}

	var matchedEntities []Entity
	var searchResults []SearchResult
	queryLower := strings.ToLower(query)

	// Prepare data for fuzzy search if enabled
	var fuzzyTargets []string
	var entityIndexMap map[int]int // maps fuzzy result index to entity index

	if gm.fuzzyEnabled {
		entityIndexMap = make(map[int]int)
		fuzzyIndex := 0
		for i, entity := range graph.Entities {
			// Add entity name
			fuzzyTargets = append(fuzzyTargets, entity.Name)
			entityIndexMap[fuzzyIndex] = i
			fuzzyIndex++

			// Add entity type
			fuzzyTargets = append(fuzzyTargets, entity.EntityType)
			entityIndexMap[fuzzyIndex] = i
			fuzzyIndex++

			// Add observations
			for _, obs := range entity.Observations {
				fuzzyTargets = append(fuzzyTargets, obs)
				entityIndexMap[fuzzyIndex] = i
				fuzzyIndex++
			}
		}
	}

	// Track which entities we've already added to avoid duplicates
	addedEntities := make(map[string]bool)

	// Exact and partial matches first
	for _, entity := range graph.Entities {
		score := 0.0
		matchType := ""

		// Check entity name
		if strings.EqualFold(entity.Name, query) {
			score = 1.0
			matchType = "exact"
		} else if strings.Contains(strings.ToLower(entity.Name), queryLower) {
			score = 0.8
			matchType = "partial"
		}

		// Check entity type
		if score == 0.0 {
			if strings.EqualFold(entity.EntityType, query) {
				score = 0.9
				matchType = "exact"
			} else if strings.Contains(strings.ToLower(entity.EntityType), queryLower) {
				score = 0.7
				matchType = "partial"
			}
		}

		// Check observations
		if score == 0.0 {
			for _, obs := range entity.Observations {
				if strings.Contains(strings.ToLower(obs), queryLower) {
					score = 0.6
					matchType = "partial"
					break
				}
			}
		}

		if score > 0.0 {
			matchedEntities = append(matchedEntities, entity)
			searchResults = append(searchResults, SearchResult{
				Entity:    entity,
				Score:     score,
				MatchType: matchType,
			})
			addedEntities[entity.Name] = true
		}
	}

	// Fuzzy search for additional matches
	if gm.fuzzyEnabled && len(fuzzyTargets) > 0 {
		fuzzyResults := fuzzy.Find(query, fuzzyTargets)
		for _, result := range fuzzyResults {
			if entityIndex, exists := entityIndexMap[result.Index]; exists {
				entity := graph.Entities[entityIndex]

				// Skip if already added
				if addedEntities[entity.Name] {
					continue
				}

				// Calculate fuzzy score (Normalise to 0-1 range)
				fuzzyScore := float64(result.Score) / 100.0
				if fuzzyScore < 0.3 { // Skip very low relevance matches
					continue
				}

				matchedEntities = append(matchedEntities, entity)
				searchResults = append(searchResults, SearchResult{
					Entity:    entity,
					Score:     fuzzyScore,
					MatchType: "fuzzy",
				})
				addedEntities[entity.Name] = true
			}
		}
	}

	// Create filtered graph with matched entities and their relations
	filteredGraph := &KnowledgeGraph{
		Entities: matchedEntities,
	}

	// Add relations between matched entities
	entitySet := make(map[string]bool)
	for _, entity := range matchedEntities {
		entitySet[entity.Name] = true
	}

	for _, relation := range graph.Relations {
		if entitySet[relation.From] && entitySet[relation.To] {
			filteredGraph.Relations = append(filteredGraph.Relations, relation)
		}
	}

	return filteredGraph, searchResults, nil
}

// OpenNodes retrieves specific entities by name
func (gm *GraphManager) OpenNodes(names []string) (*KnowledgeGraph, error) {
	graph, err := gm.storage.LoadGraph()
	if err != nil {
		return nil, fmt.Errorf("failed to load graph: %w", err)
	}

	// Create a set of requested names
	requestedNames := make(map[string]bool)
	for _, name := range names {
		requestedNames[name] = true
	}

	// Filter entities
	var filteredEntities []Entity
	for _, entity := range graph.Entities {
		if requestedNames[entity.Name] {
			filteredEntities = append(filteredEntities, entity)
		}
	}

	// Create a set of filtered entity names for relation filtering
	entitySet := make(map[string]bool)
	for _, entity := range filteredEntities {
		entitySet[entity.Name] = true
	}

	// Filter relations to only include those between filtered entities
	var filteredRelations []Relation
	for _, relation := range graph.Relations {
		if entitySet[relation.From] && entitySet[relation.To] {
			filteredRelations = append(filteredRelations, relation)
		}
	}

	return &KnowledgeGraph{
		Entities:  filteredEntities,
		Relations: filteredRelations,
	}, nil
}

// validateEntityName validates entity names
func (gm *GraphManager) validateEntityName(name string) error {
	if name == "" {
		return fmt.Errorf("entity name cannot be empty")
	}

	if strings.TrimSpace(name) != name {
		return fmt.Errorf("entity name cannot have leading or trailing whitespace")
	}

	// Allow most characters but warn about potential issues
	if strings.Contains(name, "\n") || strings.Contains(name, "\r") {
		return fmt.Errorf("entity name cannot contain newline characters")
	}

	return nil
}

// GetStorageInfo returns information about the storage
func (gm *GraphManager) GetStorageInfo() (string, bool, error) {
	filePath := gm.storage.GetFilePath()
	exists := gm.storage.FileExists()

	var err error
	if exists {
		_, err = gm.storage.GetFileInfo()
	}

	return filePath, exists, err
}

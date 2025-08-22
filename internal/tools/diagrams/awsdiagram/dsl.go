package awsdiagram

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// parseDefinition parses both text DSL and JSON formats
func (t *AWSDiagramTool) parseDefinition(definition string) (*DiagramSpec, error) {
	definition = strings.TrimSpace(definition)

	// Try JSON first
	if strings.HasPrefix(definition, "{") {
		return t.parseJSONDefinition(definition)
	}

	// Otherwise parse as text DSL
	return t.parseTextDSL(definition)
}

// parseJSONDefinition parses JSON format diagram definitions
func (t *AWSDiagramTool) parseJSONDefinition(definition string) (*DiagramSpec, error) {
	var jsonDef struct {
		Diagram struct {
			Name      string `json:"name"`
			Direction string `json:"direction"`
		} `json:"diagram"`
		Nodes []struct {
			ID    string            `json:"id"`
			Type  string            `json:"type"`
			Label string            `json:"label"`
			Style map[string]string `json:"style"`
		} `json:"nodes"`
		Connections []struct {
			From  string            `json:"from"`
			To    string            `json:"to"`
			Label string            `json:"label"`
			Style map[string]string `json:"style"`
		} `json:"connections"`
		Clusters []struct {
			Name  string            `json:"name"`
			Nodes []string          `json:"nodes"`
			Style map[string]string `json:"style"`
		} `json:"clusters"`
	}

	if err := json.Unmarshal([]byte(definition), &jsonDef); err != nil {
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	spec := &DiagramSpec{
		Name:      jsonDef.Diagram.Name,
		Direction: jsonDef.Diagram.Direction,
	}

	// Default direction
	if spec.Direction == "" {
		spec.Direction = "LR"
	}

	// Convert nodes
	for _, node := range jsonDef.Nodes {
		spec.Nodes = append(spec.Nodes, NodeSpec{
			ID:    node.ID,
			Type:  node.Type,
			Label: node.Label,
			Style: node.Style,
		})
	}

	// Convert connections
	for _, conn := range jsonDef.Connections {
		spec.Connections = append(spec.Connections, ConnectionSpec{
			From:  conn.From,
			To:    conn.To,
			Label: conn.Label,
			Style: conn.Style,
		})
	}

	// Convert clusters
	for _, cluster := range jsonDef.Clusters {
		spec.Clusters = append(spec.Clusters, ClusterSpec{
			Name:  cluster.Name,
			Nodes: cluster.Nodes,
			Style: cluster.Style,
		})
	}

	return spec, nil
}

// parseTextDSL parses the AI-friendly text DSL format
func (t *AWSDiagramTool) parseTextDSL(definition string) (*DiagramSpec, error) {
	spec := &DiagramSpec{
		Direction: "LR", // default
	}

	lines := strings.Split(definition, "\n")
	var currentCluster *ClusterSpec

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// Parse diagram declaration
		if strings.HasPrefix(line, "diagram ") {
			if err := t.parseDiagramDeclaration(line, spec); err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			continue
		}

		// Parse cluster start
		if strings.Contains(line, "cluster ") && strings.Contains(line, "{") {
			cluster, err := t.parseClusterStart(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			currentCluster = &cluster
			continue
		}

		// Parse cluster end
		if line == "}" && currentCluster != nil {
			spec.Clusters = append(spec.Clusters, *currentCluster)
			currentCluster = nil
			continue
		}

		// Skip opening brace for diagram
		if line == "{" {
			continue
		}

		// Parse node definition
		if strings.Contains(line, "node ") && strings.Contains(line, "=") {
			node, err := t.parseNodeDefinition(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			spec.Nodes = append(spec.Nodes, node)

			// Add to current cluster if inside one
			if currentCluster != nil {
				currentCluster.Nodes = append(currentCluster.Nodes, node.ID)
			}
			continue
		}

		// Parse connection
		if strings.Contains(line, "->") {
			connections, err := t.parseConnection(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			spec.Connections = append(spec.Connections, connections...)
			continue
		}
	}

	// Validate the parsed specification
	if err := t.validateSpec(spec); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return spec, nil
}

// parseDiagramDeclaration parses diagram name and attributes
func (t *AWSDiagramTool) parseDiagramDeclaration(line string, spec *DiagramSpec) error {
	// Pattern: diagram "Name" direction=TB {
	re := regexp.MustCompile(`diagram\s+"([^"]+)"(?:\s+direction=([A-Z]+))?\s*\{?`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 2 {
		return fmt.Errorf("invalid diagram declaration: %s", line)
	}

	spec.Name = matches[1]
	if len(matches) > 2 && matches[2] != "" {
		spec.Direction = matches[2]
	}

	return nil
}

// parseClusterStart parses cluster declarations
func (t *AWSDiagramTool) parseClusterStart(line string) (ClusterSpec, error) {
	// Pattern: cluster name "Label" {
	re := regexp.MustCompile(`cluster\s+(\w+)\s+"([^"]+)"\s*\{`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 3 {
		return ClusterSpec{}, fmt.Errorf("invalid cluster declaration: %s", line)
	}

	return ClusterSpec{
		Name:  matches[2],
		Nodes: []string{},
		Style: map[string]string{},
	}, nil
}

// parseNodeDefinition parses node declarations
func (t *AWSDiagramTool) parseNodeDefinition(line string) (NodeSpec, error) {
	// Pattern: node id = type "Label"
	re := regexp.MustCompile(`node\s+(\w+)\s*=\s*([a-zA-Z0-9.]+)\s+"([^"]+)"`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 4 {
		return NodeSpec{}, fmt.Errorf("invalid node definition: %s", line)
	}

	return NodeSpec{
		ID:    matches[1],
		Type:  matches[2],
		Label: matches[3],
		Style: map[string]string{},
	}, nil
}

// parseConnection parses connection statements
func (t *AWSDiagramTool) parseConnection(line string) ([]ConnectionSpec, error) {
	var connections []ConnectionSpec

	// Handle array notation: [web1, web2] -> rds
	if strings.Contains(line, "[") && strings.Contains(line, "]") {
		return t.parseArrayConnection(line)
	}

	// Handle simple chain: a -> b -> c
	if strings.Count(line, "->") > 1 {
		return t.parseChainConnection(line)
	}

	// Handle simple connection: a -> b
	parts := strings.Split(line, "->")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid connection: %s", line)
	}

	from := strings.TrimSpace(parts[0])
	to := strings.TrimSpace(parts[1])

	connections = append(connections, ConnectionSpec{
		From:  from,
		To:    to,
		Style: map[string]string{},
	})

	return connections, nil
}

// parseArrayConnection handles array-style connections like [web1, web2] -> db
func (t *AWSDiagramTool) parseArrayConnection(line string) ([]ConnectionSpec, error) {
	var connections []ConnectionSpec

	// Extract array and target
	re := regexp.MustCompile(`\[([^\]]+)\]\s*->\s*(\w+)`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 3 {
		return nil, fmt.Errorf("invalid array connection: %s", line)
	}

	// Parse array elements
	arrayElements := strings.Split(matches[1], ",")
	target := strings.TrimSpace(matches[2])

	for _, element := range arrayElements {
		element = strings.TrimSpace(element)
		connections = append(connections, ConnectionSpec{
			From:  element,
			To:    target,
			Style: map[string]string{},
		})
	}

	return connections, nil
}

// parseChainConnection handles chain-style connections like a -> b -> c
func (t *AWSDiagramTool) parseChainConnection(line string) ([]ConnectionSpec, error) {
	var connections []ConnectionSpec

	parts := strings.Split(line, "->")
	for i := 0; i < len(parts)-1; i++ {
		from := strings.TrimSpace(parts[i])
		to := strings.TrimSpace(parts[i+1])

		connections = append(connections, ConnectionSpec{
			From:  from,
			To:    to,
			Style: map[string]string{},
		})
	}

	return connections, nil
}

// validateSpec validates the parsed diagram specification
func (t *AWSDiagramTool) validateSpec(spec *DiagramSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("diagram name is required")
	}

	if len(spec.Nodes) == 0 {
		return fmt.Errorf("at least one node is required")
	}

	// Check that all connection references exist
	nodeIDs := make(map[string]bool)
	for _, node := range spec.Nodes {
		if node.ID == "" {
			return fmt.Errorf("node ID cannot be empty")
		}
		nodeIDs[node.ID] = true
	}

	for _, conn := range spec.Connections {
		if !nodeIDs[conn.From] {
			return fmt.Errorf("connection references unknown node: %s", conn.From)
		}
		if !nodeIDs[conn.To] {
			return fmt.Errorf("connection references unknown node: %s", conn.To)
		}
	}

	// Check cluster node references
	for _, cluster := range spec.Clusters {
		for _, nodeID := range cluster.Nodes {
			if !nodeIDs[nodeID] {
				return fmt.Errorf("cluster '%s' references unknown node: %s", cluster.Name, nodeID)
			}
		}
	}

	return nil
}

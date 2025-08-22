package graphvizdiagram

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseDefinition parses JSON format diagram definitions only
func (t *GraphvizDiagramTool) parseDefinition(definition string) (*DiagramSpec, error) {
	definition = strings.TrimSpace(definition)

	// Only accept JSON format
	if !strings.HasPrefix(definition, "{") {
		return nil, fmt.Errorf("definition must be in JSON format. Expected structure: {\"name\": \"Diagram Title\", \"nodes\": [{\"id\": \"nodeId\", \"type\": \"aws.ec2\", \"label\": \"Display Name\"}], \"connections\": [{\"from\": \"nodeId1\", \"to\": \"nodeId2\"}]}. Use action='examples' for complete examples")
	}

	return t.parseJSONDefinition(definition)
}

// parseJSONDefinition parses JSON format diagram definitions
func (t *GraphvizDiagramTool) parseJSONDefinition(definition string) (*DiagramSpec, error) {
	// Support both nested and flat JSON structures
	var jsonDef struct {
		Name      string `json:"name"`
		Direction string `json:"direction"`
		Diagram   struct {
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
		return nil, fmt.Errorf("invalid JSON format: %w. Expected structure: {\"name\": \"My Diagram\", \"nodes\": [{\"id\": \"web\", \"type\": \"aws.ec2\", \"label\": \"Web Server\"}], \"connections\": [{\"from\": \"web\", \"to\": \"db\"}]}. Use action='examples' for complete examples", err)
	}

	// Support both flat and nested JSON structures
	name := jsonDef.Name
	direction := jsonDef.Direction
	if name == "" && jsonDef.Diagram.Name != "" {
		name = jsonDef.Diagram.Name
	}
	if direction == "" && jsonDef.Diagram.Direction != "" {
		direction = jsonDef.Diagram.Direction
	}

	spec := &DiagramSpec{
		Name:      name,
		Direction: direction,
	}

	if spec.Name == "" {
		return nil, fmt.Errorf("diagram name is required. Expected JSON format: {\"name\": \"My Diagram\", \"nodes\": [...], \"connections\": [...]}")
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

package graphvizdiagram

import (
	"fmt"
	"strings"
)

// generateDOT converts a DiagramSpec to DOT notation
func (t *GraphvizDiagramTool) generateDOT(diagram *DiagramSpec) (string, error) {
	var dot strings.Builder

	// Track rendered nodes to avoid duplicates across clusters
	nodesRendered := make(map[string]bool)

	// Start digraph
	dot.WriteString("digraph {\n")

	// Set graph attributes
	dot.WriteString(fmt.Sprintf("  label=\"%s\";\n", escapeDOTString(diagram.Name)))
	dot.WriteString("  labelloc=\"t\";\n")
	dot.WriteString("  fontsize=\"16\";\n")
	dot.WriteString("  fontname=\"Helvetica\";\n") // Clean, modern font available on most systems

	// High-resolution settings for better PNG quality
	dot.WriteString("  dpi=\"200\";\n")        // 200 DPI for good quality
	dot.WriteString("  resolution=\"200\";\n") // Alternative resolution setting
	dot.WriteString("  size=\"12,8!\";\n")     // Set reasonable size limit

	// Set direction
	switch diagram.Direction {
	case "TB", "TD":
		dot.WriteString("  rankdir=\"TB\";\n")
	case "BT":
		dot.WriteString("  rankdir=\"BT\";\n")
	case "LR":
		dot.WriteString("  rankdir=\"LR\";\n")
	case "RL":
		dot.WriteString("  rankdir=\"RL\";\n")
	default:
		dot.WriteString("  rankdir=\"LR\";\n") // default
	}

	// Default node and edge styles
	dot.WriteString("  node [shape=box, style=\"filled,rounded\", fontname=\"Helvetica\", fontsize=11];\n")
	dot.WriteString("  edge [fontname=\"Helvetica\", fontsize=10, color=\"#4A90E2\", penwidth=2, arrowsize=0.8];\n")
	dot.WriteString("  bgcolor=\"white\";\n")
	dot.WriteString("\n")

	// Generate clusters
	for i, cluster := range diagram.Clusters {
		if err := t.generateCluster(&dot, cluster, i, diagram, nodesRendered); err != nil {
			return "", fmt.Errorf("failed to generate cluster '%s': %w", cluster.Name, err)
		}
	}

	// Generate nodes (only those not in clusters, avoiding duplicates)
	nodesInClusters := make(map[string]bool)
	for _, cluster := range diagram.Clusters {
		for _, nodeID := range cluster.Nodes {
			nodesInClusters[nodeID] = true
		}
	}

	for _, node := range diagram.Nodes {
		if !nodesInClusters[node.ID] {
			if err := t.generateNode(&dot, node); err != nil {
				return "", fmt.Errorf("failed to generate node '%s': %w", node.ID, err)
			}
		}
	}

	// Generate edges
	dot.WriteString("\n")
	for _, conn := range diagram.Connections {
		if err := t.generateConnection(&dot, conn); err != nil {
			return "", fmt.Errorf("failed to generate connection '%s->%s': %w", conn.From, conn.To, err)
		}
	}

	// End digraph
	dot.WriteString("}\n")

	return dot.String(), nil
}

// generateCluster generates DOT for a cluster/subgraph
func (t *GraphvizDiagramTool) generateCluster(dot *strings.Builder, cluster ClusterSpec, index int, diagram *DiagramSpec, nodesRendered map[string]bool) error {
	// Start subgraph
	fmt.Fprintf(dot, "  subgraph cluster_%d {\n", index)
	fmt.Fprintf(dot, "    label=\"%s\";\n", escapeDOTString(cluster.Name))
	dot.WriteString("    style=\"rounded,filled\";\n")
	dot.WriteString("    fillcolor=\"#f0f0f0\";\n")
	dot.WriteString("    color=\"#cccccc\";\n")
	dot.WriteString("    fontsize=\"13\";\n")
	dot.WriteString("    fontname=\"Helvetica-Bold\";\n")
	dot.WriteString("    penwidth=\"2\";\n")

	// Apply custom cluster styles
	for key, value := range cluster.Style {
		fmt.Fprintf(dot, "    %s=\"%s\";\n", key, escapeDOTString(value))
	}

	dot.WriteString("\n")

	// Generate nodes within the cluster (avoid duplicates)
	for _, nodeID := range cluster.Nodes {
		// Skip if already rendered in another cluster
		if nodesRendered[nodeID] {
			continue
		}

		// Find the node spec
		var nodeSpec *NodeSpec
		for i := range diagram.Nodes {
			if diagram.Nodes[i].ID == nodeID {
				nodeSpec = &diagram.Nodes[i]
				break
			}
		}

		if nodeSpec == nil {
			return fmt.Errorf("node '%s' not found in cluster '%s'", nodeID, cluster.Name)
		}

		// Generate node with indentation for cluster
		if err := t.generateNodeWithIndent(dot, *nodeSpec, "    "); err != nil {
			return err
		}

		// Mark as rendered
		nodesRendered[nodeID] = true
	}

	// End subgraph
	dot.WriteString("  }\n\n")

	return nil
}

// generateNode generates DOT for a single node
func (t *GraphvizDiagramTool) generateNode(dot *strings.Builder, node NodeSpec) error {
	return t.generateNodeWithIndent(dot, node, "  ")
}

// generateNodeWithIndent generates DOT for a node with specified indentation
func (t *GraphvizDiagramTool) generateNodeWithIndent(dot *strings.Builder, node NodeSpec, indent string) error {
	// Try icon-based rendering first
	normalizedType := normalizeNodeType(node.Type)
	if iconPath, hasIcon := t.getIconPath(normalizedType); hasIcon {
		// Use icon-based node rendering with better connection points
		return t.generateIconNode(dot, node, iconPath, indent)
	}

	// Fall back to standard shape-based rendering
	// Get node styling based on type
	shape, color, style := t.getNodeStyling(node.Type)

	// Start node definition
	fmt.Fprintf(dot, "%s%s [", indent, node.ID)

	// Add label
	fmt.Fprintf(dot, "label=\"%s\"", escapeDOTString(node.Label))

	// Add shape and styling
	fmt.Fprintf(dot, ", shape=\"%s\"", shape)
	fmt.Fprintf(dot, ", fillcolor=\"%s\"", color)
	fmt.Fprintf(dot, ", style=\"%s\"", style)

	// Add custom styles
	for key, value := range node.Style {
		fmt.Fprintf(dot, ", %s=\"%s\"", key, escapeDOTString(value))
	}

	// End node definition
	dot.WriteString("];\n")

	return nil
}

// generateIconNode generates a node with an icon image and proper connection points
func (t *GraphvizDiagramTool) generateIconNode(dot *strings.Builder, node NodeSpec, iconPath string, indent string) error {
	// Use a rectangle shape with proper connection boundaries
	fmt.Fprintf(dot, "%s%s [", indent, node.ID)

	// Use rectangle shape with proper connection points
	fmt.Fprintf(dot, "shape=\"box\"")
	fmt.Fprintf(dot, ", style=\"filled,rounded\"")
	fmt.Fprintf(dot, ", fillcolor=\"white\"")
	fmt.Fprintf(dot, ", penwidth=\"2\"")
	fmt.Fprintf(dot, ", color=\"#4A90E2\"")
	
	// Create HTML table with proper border for connections
	fmt.Fprintf(dot, ", label=<")
	fmt.Fprintf(dot, "<TABLE BORDER=\"1\" CELLBORDER=\"0\" CELLSPACING=\"0\" CELLPADDING=\"8\" STYLE=\"rounded\">")
	fmt.Fprintf(dot, "<TR><TD><IMG SRC=\"%s\" SCALE=\"FALSE\" WIDTH=\"32\" HEIGHT=\"32\"/></TD></TR>", iconPath)
	fmt.Fprintf(dot, "<TR><TD><FONT POINT-SIZE=\"10\" FACE=\"Helvetica-Bold\">%s</FONT></TD></TR>", escapeDOTString(node.Label))
	fmt.Fprintf(dot, "</TABLE>>")

	// Apply any custom styles
	for key, value := range node.Style {
		fmt.Fprintf(dot, ", %s=\"%s\"", key, escapeDOTString(value))
	}

	dot.WriteString("];\n")
	return nil
}

// generateConnection generates DOT for an edge/connection
func (t *GraphvizDiagramTool) generateConnection(dot *strings.Builder, conn ConnectionSpec) error {
	// Basic connection
	fmt.Fprintf(dot, "  %s -> %s", conn.From, conn.To)

	// Add attributes
	attributes := []string{}
	
	// Default professional styling
	attributes = append(attributes, "penwidth=2")
	attributes = append(attributes, "color=\"#4A90E2\"")
	attributes = append(attributes, "arrowsize=0.8")
	attributes = append(attributes, "arrowhead=\"vee\"")

	if conn.Label != "" {
		attributes = append(attributes, fmt.Sprintf("label=\"%s\"", escapeDOTString(conn.Label)))
		attributes = append(attributes, "fontsize=9")
		attributes = append(attributes, "fontcolor=\"#666666\"")
	}

	// Add custom styles (allow override of defaults)
	for key, value := range conn.Style {
		attributes = append(attributes, fmt.Sprintf("%s=\"%s\"", key, escapeDOTString(value)))
	}

	// Add all attributes
	if len(attributes) > 0 {
		dot.WriteString(" [")
		dot.WriteString(strings.Join(attributes, ", "))
		dot.WriteString("]")
	}

	dot.WriteString(";\n")

	return nil
}

// getNodeStyling returns shape, color, and style based on node type
func (t *GraphvizDiagramTool) getNodeStyling(nodeType string) (shape, color, style string) {
	// Default styling
	shape = "box"
	color = "lightblue"
	style = "filled,rounded"

	// AWS service styling
	switch {
	case strings.Contains(nodeType, "aws.ec2") || strings.Contains(nodeType, "aws.compute"):
		color = "#FF9900" // AWS orange
		shape = "ellipse"
	case strings.Contains(nodeType, "aws.rds") || strings.Contains(nodeType, "aws.database"):
		color = "#3F48CC" // AWS blue
		shape = "cylinder"
	case strings.Contains(nodeType, "aws.s3") || strings.Contains(nodeType, "aws.storage"):
		color = "#3F48CC" // AWS blue
		shape = "folder"
	case strings.Contains(nodeType, "aws.lambda"):
		color = "#FF9900" // AWS orange
		shape = "hexagon"
	case strings.Contains(nodeType, "aws.elb") || strings.Contains(nodeType, "aws.alb"):
		color = "#FF9900" // AWS orange
		shape = "box"
	case strings.Contains(nodeType, "aws.vpc") || strings.Contains(nodeType, "aws.network"):
		color = "#232F3E" // AWS dark
		shape = "box"
		style = "filled,dashed"
	case strings.Contains(nodeType, "aws.iam") || strings.Contains(nodeType, "aws.security"):
		color = "#FF4B4B" // Red for security
		shape = "diamond"

	// GCP service styling
	case strings.Contains(nodeType, "gcp.compute"):
		color = "#4285F4" // Google blue
		shape = "ellipse"
	case strings.Contains(nodeType, "gcp.storage"):
		color = "#34A853" // Google green
		shape = "folder"
	case strings.Contains(nodeType, "gcp.database"):
		color = "#EA4335" // Google red
		shape = "cylinder"

	// Kubernetes styling
	case strings.Contains(nodeType, "k8s.pod"):
		color = "#326CE5" // Kubernetes blue
		shape = "ellipse"
	case strings.Contains(nodeType, "k8s.service"):
		color = "#326CE5"
		shape = "box"
	case strings.Contains(nodeType, "k8s.deployment"):
		color = "#326CE5"
		shape = "hexagon"

	// Generic/programming styling
	case strings.Contains(nodeType, "generic.database"):
		color = "lightgray"
		shape = "cylinder"
	case strings.Contains(nodeType, "generic.server"):
		color = "lightgray"
		shape = "box"
	case strings.Contains(nodeType, "user"):
		color = "lightyellow"
		shape = "ellipse"
	}

	return shape, color, style
}

// escapeDOTString escapes special characters for DOT notation
func escapeDOTString(s string) string {
	// Escape quotes and backslashes
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")

	// Replace newlines with \n
	s = strings.ReplaceAll(s, "\n", "\\n")

	return s
}

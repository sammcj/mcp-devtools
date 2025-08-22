package graphvizdiagram

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// GraphvizDiagramTool implements diagram generation using Graphviz with excellent AWS support
type GraphvizDiagramTool struct{}

// init registers the tool with the registry
func init() {
	registry.Register(&GraphvizDiagramTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *GraphvizDiagramTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"graphviz_diagram",
		mcp.WithDescription("Generate diagrams using Graphviz with excellent AWS/cloud architecture support. Supports multiple actions: generate diagrams, get examples, or list available icons."),

		// Required action parameter
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform: 'generate' (create diagram), 'examples' (get example definitions), or 'list_icons' (discover available icons)"),
			mcp.Enum("generate", "examples", "list_icons"),
		),

		// Definition parameter (required for generate action)
		mcp.WithString("definition",
			mcp.Description("Diagram definition in JSON format (required for 'generate' action). Structure: {\"name\": \"Diagram Title\", \"direction\": \"TB|LR|BT|RL\", \"nodes\": [{\"id\": \"nodeId\", \"type\": \"aws.ec2|generic.server|...\", \"label\": \"Display Name\"}], \"connections\": [{\"from\": \"nodeId1\", \"to\": \"nodeId2\"}], \"clusters\": [{\"name\": \"Group Name\", \"nodes\": [\"nodeId1\"]}]}"),
		),

		// Output format parameter (optional for generate action)
		mcp.WithArray("output_format",
			mcp.Description("Output formats for generated diagrams. Options: 'png', 'svg', 'dot'. Default: ['png', 'svg', 'dot']"),
		),

		// Diagram type parameter (optional for examples action)
		mcp.WithString("diagram_type",
			mcp.Description("Type of diagram examples to retrieve (for 'examples' action). Options: 'aws', 'sequence', 'flow', 'class', 'k8s', 'onprem', 'custom', 'all'"),
			mcp.Enum("aws", "sequence", "flow", "class", "k8s", "onprem", "custom", "all"),
			mcp.DefaultString("all"),
		),

		// Provider parameter (optional for list_icons action)
		mcp.WithString("provider",
			mcp.Description("Filter icons by provider (for 'list_icons' action). Options: 'aws', 'gcp', 'k8s', 'generic'"),
			mcp.Enum("aws", "gcp", "k8s", "generic"),
		),

		// Service parameter (optional for list_icons action)
		mcp.WithString("service",
			mcp.Description("Filter icons by service category (for 'list_icons' action). Options: 'compute', 'database', 'network', 'storage', 'security', 'analytics'"),
			mcp.Enum("compute", "database", "network", "storage", "security", "analytics"),
		),

		// Filename parameter (optional for generate action)
		mcp.WithString("filename",
			mcp.Description("Custom filename for output (without extension). If not provided, a name will be generated from diagram title."),
		),

		// Workspace directory parameter (optional for generate action)
		mcp.WithString("workspace_dir",
			mcp.Description("Target directory for generated diagrams. Defaults to current working directory. Diagrams are saved to 'generated-diagrams' subdirectory."),
		),
	)
}

// Execute executes the tool's logic based on the action parameter
func (t *GraphvizDiagramTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Check if graphviz_diagram tool is enabled (disabled by default)
	if !tools.IsToolEnabled("graphviz_diagram") {
		return nil, fmt.Errorf("graphviz_diagram tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'graphviz_diagram'")
	}

	logger.Info("Executing Graphviz diagram tool")

	// Parse required action parameter
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return nil, fmt.Errorf("missing required parameter: action")
	}

	// Route to appropriate handler based on action
	switch action {
	case "generate":
		return t.handleGenerate(ctx, logger, cache, args)
	case "examples":
		return t.handleExamples(ctx, logger, cache, args)
	case "list_icons":
		return t.handleListIcons(ctx, logger, cache, args)
	default:
		return nil, fmt.Errorf("invalid action: %s. Must be 'generate', 'examples', or 'list_icons'", action)
	}
}

// handleGenerate processes diagram generation requests
func (t *GraphvizDiagramTool) handleGenerate(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse required definition parameter
	definition, ok := args["definition"].(string)
	if !ok || definition == "" {
		return nil, fmt.Errorf("missing required parameter 'definition' for generate action. Expected JSON format: {\"name\": \"My Diagram\", \"nodes\": [{\"id\": \"web\", \"type\": \"aws.ec2\", \"label\": \"Web Server\"}], \"connections\": [{\"from\": \"web\", \"to\": \"db\"}]}. Use action='examples' to see full examples")
	}

	// Parse optional parameters
	outputFormats := []string{"png", "svg", "dot"} // default to all three formats
	if formats, ok := args["output_format"].([]interface{}); ok {
		outputFormats = make([]string, len(formats))
		for i, format := range formats {
			if formatStr, ok := format.(string); ok {
				outputFormats[i] = formatStr
			}
		}
	}

	filename := ""
	if f, ok := args["filename"].(string); ok {
		filename = f
	}

	workspaceDir := ""
	if wd, ok := args["workspace_dir"].(string); ok {
		workspaceDir = wd
	}

	logger.WithFields(logrus.Fields{
		"definition_length": len(definition),
		"output_formats":    outputFormats,
		"filename":          filename,
		"workspace_dir":     workspaceDir,
	}).Info("Processing diagram generation request")

	// Parse the diagram definition
	diagram, err := t.parseDefinition(definition)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diagram definition: %w", err)
	}

	// Generate DOT notation
	dotContent, err := t.generateDOT(diagram)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DOT notation: %w", err)
	}

	// Render diagrams in requested formats
	result, err := t.renderDiagram(ctx, logger, dotContent, diagram, outputFormats, filename, workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to render diagram: %w", err)
	}

	// Create clean success message
	outputFiles, _ := result["output_files"].(map[string]string)
	outputDir, _ := result["output_directory"].(string)

	var fileList []string
	for format, path := range outputFiles {
		// Use relative path from output directory for cleaner display
		filename := filepath.Base(path)
		fileList = append(fileList, fmt.Sprintf("%s (%s)", filename, format))
	}

	successMessage := fmt.Sprintf("Diagram successfully created: %s\nFiles: %s",
		outputDir, strings.Join(fileList, ", "))

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(successMessage)},
		IsError: false,
	}, nil
}

// handleExamples returns example diagram definitions
func (t *GraphvizDiagramTool) handleExamples(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse optional diagram_type parameter
	diagramType := "all"
	if dt, ok := args["diagram_type"].(string); ok && dt != "" {
		diagramType = dt
	}

	logger.WithField("diagram_type", diagramType).Info("Retrieving diagram examples")

	examples, err := t.getExamples(diagramType)
	if err != nil {
		return nil, fmt.Errorf("failed to get examples: %w", err)
	}

	result := map[string]interface{}{
		"action":       "examples",
		"diagram_type": diagramType,
		"examples":     examples,
		"description":  "Example diagram definitions for AI agents to learn from. Start with basic examples and add complexity incrementally.",
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("%+v", result))},
		IsError: false,
	}, nil
}

// handleListIcons returns available icons filtered by provider/service
func (t *GraphvizDiagramTool) handleListIcons(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse optional filter parameters
	provider := ""
	if p, ok := args["provider"].(string); ok {
		provider = p
	}

	service := ""
	if s, ok := args["service"].(string); ok {
		service = s
	}

	logger.WithFields(logrus.Fields{
		"provider": provider,
		"service":  service,
	}).Info("Listing available icons")

	icons, err := t.listIcons(provider, service)
	if err != nil {
		return nil, fmt.Errorf("failed to list icons: %w", err)
	}

	result := map[string]interface{}{
		"action":      "list_icons",
		"provider":    provider,
		"service":     service,
		"icons":       icons,
		"description": "Available icons for diagram generation. Use these exact names in your diagram definitions.",
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("%+v", result))},
		IsError: false,
	}, nil
}

// DiagramSpec represents a parsed diagram definition
type DiagramSpec struct {
	Name        string
	Direction   string
	Nodes       []NodeSpec
	Connections []ConnectionSpec
	Clusters    []ClusterSpec
}

// NodeSpec represents a node in the diagram
type NodeSpec struct {
	ID    string
	Type  string
	Label string
	Style map[string]string
}

// ConnectionSpec represents a connection between nodes
type ConnectionSpec struct {
	From  string
	To    string
	Label string
	Style map[string]string
}

// ClusterSpec represents a cluster/group of nodes
type ClusterSpec struct {
	Name  string
	Nodes []string
	Style map[string]string
}

package awsdiagram

import (
	"context"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// AWSDiagramTool implements diagram generation for AWS architecture diagrams
type AWSDiagramTool struct{}

// init registers the tool with the registry
func init() {
	registry.Register(&AWSDiagramTool{})
}

// Definition returns the tool's definition for MCP registration
func (t *AWSDiagramTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"aws_diagram",
		mcp.WithDescription("Generate AWS architecture diagrams using native Go implementation. Supports multiple actions: generate diagrams, get examples, or list available icons."),

		// Required action parameter
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform: 'generate' (create diagram), 'examples' (get example definitions), or 'list_icons' (discover available icons)"),
			mcp.Enum("generate", "examples", "list_icons"),
		),

		// Definition parameter (required for generate action)
		mcp.WithString("definition",
			mcp.Description("Diagram definition in AI-friendly text format (required for 'generate' action). Supports both simple text DSL and JSON format."),
		),

		// Output format parameter (optional for generate action)
		mcp.WithArray("output_format",
			mcp.Description("Output formats for generated diagrams. Options: 'png', 'svg', 'pdf', 'dot'. Default: ['png']"),
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
func (t *AWSDiagramTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Check if aws-diagram tool is enabled (disabled by default)
	if !tools.IsToolEnabled("aws-diagram") {
		return nil, fmt.Errorf("aws-diagram tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'aws-diagram'")
	}

	logger.Info("Executing AWS diagram tool")

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
func (t *AWSDiagramTool) handleGenerate(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Parse required definition parameter
	definition, ok := args["definition"].(string)
	if !ok || definition == "" {
		return nil, fmt.Errorf("missing required parameter 'definition' for generate action")
	}

	// Parse optional parameters
	outputFormats := []string{"png"} // default
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

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("%+v", result))},
		IsError: false,
	}, nil
}

// handleExamples returns example diagram definitions
func (t *AWSDiagramTool) handleExamples(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
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
func (t *AWSDiagramTool) handleListIcons(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
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

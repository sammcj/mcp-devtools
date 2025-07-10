package docprocessing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// Environment variable constants for LLM integration
const (
	EnvOpenAIAPIBase  = "DOCLING_VLM_API_URL"     // e.g., "https://api.openai.com/v1"
	EnvOpenAIModel    = "DOCLING_VLM_MODEL"       // e.g., "gpt-4-vision-preview"
	EnvOpenAIAPIKey   = "DOCLING_VLM_API_KEY"     // API key for the provider (consistent with VLM naming)
	EnvLLMMaxTokens   = "DOCLING_LLM_MAX_TOKENS"  // Maximum tokens for LLM response (default: 16384)
	EnvLLMTemperature = "DOCLING_LLM_TEMPERATURE" // Temperature for LLM inference (default: 0.1)
	EnvLLMTimeout     = "DOCLING_LLM_TIMEOUT"     // Timeout for LLM requests in seconds (default: 240)

	// Prompt configuration environment variables
	EnvPromptBase         = "DOCLING_LLM_PROMPT_BASE"         // Base prompt for diagram analysis
	EnvPromptFlowchart    = "DOCLING_LLM_PROMPT_FLOWCHART"    // Flowchart-specific prompt
	EnvPromptArchitecture = "DOCLING_LLM_PROMPT_ARCHITECTURE" // Architecture diagram prompt
	EnvPromptChart        = "DOCLING_LLM_PROMPT_CHART"        // Chart analysis prompt
	EnvPromptGeneric      = "DOCLING_LLM_PROMPT_GENERIC"      // Generic diagram prompt
)

// Default LLM configuration values
const (
	DefaultMaxTokens   = 16384
	DefaultTemperature = 0.1
	DefaultTimeout     = 240
)

// Default prompts
const (
	DefaultBasePrompt = "You are an expert at analysing diagrams and converting them to Mermaid syntax. " +
		"Analyse the following diagram and provide a detailed response.\n\n"

	DefaultFlowchartPrompt = `This appears to be a flowchart. Please:
1. Identify the start and end points
2. Identify decision points (diamond shapes) and process steps (rectangles)
3. Trace the flow connections between elements
4. Create Mermaid flowchart syntax using:
   - flowchart TD (top-down) or LR (left-right)
   - Rectangle nodes: A[Process Step]
   - Diamond nodes: B{Decision?}
   - Connections: A --> B
   - Labels on connections: B -->|Yes| C

Always use British English spelling.
Focus on accuracy of the data, logical flow and clear node relationships.`

	DefaultArchitecturePrompt = `This appears to be an architecture diagram. Please:
1. Identify system components, services, and databases
2. Identify data flow and connections between components
3. Classify components by type (compute, storage, networking, etc.)
4. Create Mermaid graph syntax using:
   - graph TD for top-down layout
   - Rectangle nodes: A[Component]
   - Rounded rectangles: B(Service)
   - Cylinders: C[(Database)]
   - Connections: A --> B

Always use British English spelling.
Include AWS-style colour coding:
- classDef compute fill:#FF9900,color:#fff
- classDef storage fill:#569A31,color:#fff
- classDef database fill:#205081,color:#fff
- classDef networking fill:#8C4FFF,color:#fff`

	DefaultChartPrompt = `This appears to be a chart or graph. Please:
1. Identify the chart type (bar, line, pie, scatter, etc.)
2. Extract data points, labels, and values if visible
3. Identify axes labels and scales
4. Create a simple Mermaid representation or describe the data structure
5. If the chart is too complex for Mermaid, provide a structured description

Always use British English spelling.
For simple charts, use Mermaid graph syntax to represent data relationships.
For complex charts, focus on describing the data structure and trends.`

	DefaultGenericPrompt = `Analyse this diagram and:
1. Determine the most likely diagram type
2. Identify key components and their relationships
3. Create appropriate Mermaid syntax based on the diagram structure
4. If uncertain about the type, provide the best possible representation
5. Always use British English spelling.

Choose the most suitable Mermaid diagram type:
- flowchart: for process flows
- graph: for system architectures
- sequenceDiagram: for interactions
- classDiagram: for object relationships`
)

// DiagramLLMClient handles LLM-based diagram analysis using OpenAI API
type DiagramLLMClient struct {
	client      *openai.Client
	model       string
	maxTokens   int
	temperature float64
	timeout     time.Duration
}

// LLMConfig contains configuration for the LLM client
type LLMConfig struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
}

// DiagramAnalysis represents the result of LLM-based diagram analysis
type DiagramAnalysis struct {
	Description    string                 `json:"description"`
	DiagramType    string                 `json:"diagram_type"`
	MermaidCode    string                 `json:"mermaid_code"`
	Elements       []DiagramElement       `json:"elements"`
	Confidence     float64                `json:"confidence"`
	Properties     map[string]interface{} `json:"properties"`
	ProcessingTime time.Duration          `json:"processing_time"`
	TokenUsage     *TokenUsage            `json:"token_usage,omitempty"` // Token usage from LLM provider (if available)
}

// NewDiagramLLMClient creates a new LLM client for diagram analysis using OpenAI API
func NewDiagramLLMClient() (*DiagramLLMClient, error) {
	// Check environment variables
	baseURL := os.Getenv(EnvOpenAIAPIBase)
	model := os.Getenv(EnvOpenAIModel)
	apiKey := os.Getenv(EnvOpenAIAPIKey)

	if baseURL == "" || model == "" || apiKey == "" {
		return nil, fmt.Errorf("LLM environment variables not configured: required %s, %s, %s",
			EnvOpenAIAPIBase, EnvOpenAIModel, EnvOpenAIAPIKey)
	}

	// Get configurable LLM settings with defaults
	maxTokens := getEnvInt(EnvLLMMaxTokens, DefaultMaxTokens)
	temperature := getEnvFloat(EnvLLMTemperature, DefaultTemperature)
	timeout := time.Duration(getEnvInt(EnvLLMTimeout, DefaultTimeout)) * time.Second

	// Create OpenAI client with custom base URL
	var opts []option.RequestOption
	opts = append(opts, option.WithAPIKey(apiKey))
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := openai.NewClient(opts...)

	return &DiagramLLMClient{
		client:      &client,
		model:       model,
		maxTokens:   maxTokens,
		temperature: temperature,
		timeout:     timeout,
	}, nil
}

// IsLLMConfigured checks if the required environment variables are set
func IsLLMConfigured() bool {
	baseURL := os.Getenv(EnvOpenAIAPIBase)
	model := os.Getenv(EnvOpenAIModel)
	apiKey := os.Getenv(EnvOpenAIAPIKey)

	return baseURL != "" && model != "" && apiKey != ""
}

// AnalyseDiagram performs LLM-based analysis of a diagram
func (c *DiagramLLMClient) AnalyseDiagram(diagram *ExtractedDiagram) (*DiagramAnalysis, error) {
	startTime := time.Now()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Create prompt based on diagram type and extracted data
	promptText := c.buildDiagramPrompt(diagram)

	// Prepare messages for the chat completion
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(promptText),
	}

	// TODO: Add proper vision support for image analysis when diagram.Base64Data is available

	// Call OpenAI API
	response, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:       c.model, // Use the configured model (string is compatible with shared.ChatModel)
		Messages:    messages,
		MaxTokens:   openai.Int(int64(c.maxTokens)),
		Temperature: openai.Float(c.temperature),
	})
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	// Extract response content
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned from LLM")
	}

	responseText := response.Choices[0].Message.Content

	// Parse response and extract analysis
	analysis, err := c.parseAnalysisResponse(responseText, diagram)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Extract token usage if available
	if usage := response.Usage; usage.PromptTokens > 0 || usage.CompletionTokens > 0 || usage.TotalTokens > 0 {
		analysis.TokenUsage = &TokenUsage{
			PromptTokens:     int(usage.PromptTokens),
			CompletionTokens: int(usage.CompletionTokens),
			TotalTokens:      int(usage.TotalTokens),
		}
	}

	// Validate generated Mermaid code if present
	if analysis.MermaidCode != "" {
		if !validateMermaidSyntax(analysis.MermaidCode) {
			// Don't fail completely, just log the issue
			if analysis.Properties == nil {
				analysis.Properties = make(map[string]interface{})
			}
			analysis.Properties["mermaid_validation_failed"] = true
		}
	}

	analysis.ProcessingTime = time.Since(startTime)
	return analysis, nil
}

// buildDiagramPrompt creates a prompt for diagram analysis based on diagram type
func (c *DiagramLLMClient) buildDiagramPrompt(diagram *ExtractedDiagram) string {
	var prompt strings.Builder

	// Base prompt (configurable)
	basePrompt := getEnvString(EnvPromptBase, DefaultBasePrompt)
	prompt.WriteString(basePrompt)

	// Add diagram information
	prompt.WriteString(fmt.Sprintf("Diagram ID: %s\n", diagram.ID))
	prompt.WriteString(fmt.Sprintf("Detected Type: %s\n", diagram.DiagramType))

	if diagram.Caption != "" {
		prompt.WriteString(fmt.Sprintf("Caption: %s\n", diagram.Caption))
	}

	if diagram.Description != "" {
		prompt.WriteString(fmt.Sprintf("Initial Description: %s\n", diagram.Description))
	}

	// Add extracted elements
	if len(diagram.Elements) > 0 {
		prompt.WriteString("\nExtracted Text Elements:\n")
		for i, element := range diagram.Elements {
			prompt.WriteString(fmt.Sprintf("%d. %s (%s)\n", i+1, element.Content, element.Type))
		}
	}

	// Add type-specific instructions
	prompt.WriteString("\n")
	switch strings.ToLower(diagram.DiagramType) {
	case "flowchart":
		prompt.WriteString(c.getFlowchartPrompt())
	case "architecture":
		prompt.WriteString(c.getArchitecturePrompt())
	case "chart":
		prompt.WriteString(c.getChartPrompt())
	default:
		prompt.WriteString(c.getGenericPrompt())
	}

	// Response format
	prompt.WriteString("\n\nProvide your response in the following JSON format:\n")
	prompt.WriteString(`{
  "description": "Detailed description of the diagram",
  "diagram_type": "flowchart|architecture|chart|sequence|other",
  "mermaid_code": "Valid Mermaid syntax for the diagram",
  "elements": [
    {
      "type": "text|shape|connector",
      "content": "Element content",
      "position": "Position description"
    }
  ],
  "confidence": 0.95,
  "properties": {
    "additional_metadata": "value"
  }
}`)

	return prompt.String()
}

// getFlowchartPrompt returns specific instructions for flowchart analysis
func (c *DiagramLLMClient) getFlowchartPrompt() string {
	return getEnvString(EnvPromptFlowchart, DefaultFlowchartPrompt)
}

// getArchitecturePrompt returns specific instructions for architecture diagram analysis
func (c *DiagramLLMClient) getArchitecturePrompt() string {
	return getEnvString(EnvPromptArchitecture, DefaultArchitecturePrompt)
}

// getChartPrompt returns specific instructions for chart analysis
func (c *DiagramLLMClient) getChartPrompt() string {
	return getEnvString(EnvPromptChart, DefaultChartPrompt)
}

// getGenericPrompt returns instructions for unknown diagram types
func (c *DiagramLLMClient) getGenericPrompt() string {
	return getEnvString(EnvPromptGeneric, DefaultGenericPrompt)
}

// parseAnalysisResponse parses the LLM response and extracts diagram analysis
func (c *DiagramLLMClient) parseAnalysisResponse(response string, originalDiagram *ExtractedDiagram) (*DiagramAnalysis, error) {
	// Try to extract JSON from the response
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		// Fallback: create analysis from text response
		return c.createFallbackAnalysis(response, originalDiagram), nil
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var analysis DiagramAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &analysis); err != nil {
		// Fallback: create analysis from text response
		return c.createFallbackAnalysis(response, originalDiagram), nil
	}

	// Ensure confidence is reasonable
	if analysis.Confidence == 0 {
		analysis.Confidence = 0.7 // Default confidence
	}
	if analysis.Confidence > 1.0 {
		analysis.Confidence = 1.0
	}

	// Ensure we have a description
	if analysis.Description == "" {
		analysis.Description = "LLM-enhanced diagram analysis"
	}

	return &analysis, nil
}

// createFallbackAnalysis creates a basic analysis when JSON parsing fails
func (c *DiagramLLMClient) createFallbackAnalysis(response string, originalDiagram *ExtractedDiagram) *DiagramAnalysis {
	analysis := &DiagramAnalysis{
		Description: response,
		DiagramType: originalDiagram.DiagramType,
		Confidence:  0.5, // Lower confidence for fallback
		Properties:  make(map[string]interface{}),
	}

	// Try to extract Mermaid code from response
	if mermaidCode := extractMermaidFromText(response); mermaidCode != "" {
		analysis.MermaidCode = mermaidCode
	}

	analysis.Properties["fallback_parsing"] = true
	return analysis
}

// extractMermaidFromText attempts to extract Mermaid code from text response
func extractMermaidFromText(text string) string {
	// Look for code blocks
	lines := strings.Split(text, "\n")
	var mermaidLines []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Start of code block
		if strings.HasPrefix(trimmed, "```") {
			if strings.Contains(trimmed, "mermaid") || inCodeBlock {
				inCodeBlock = !inCodeBlock
				continue
			}
		}

		// Inside code block
		if inCodeBlock {
			mermaidLines = append(mermaidLines, line)
		}

		// Look for Mermaid diagram types at start of lines
		if strings.HasPrefix(trimmed, "flowchart") ||
			strings.HasPrefix(trimmed, "graph") ||
			strings.HasPrefix(trimmed, "sequenceDiagram") ||
			strings.HasPrefix(trimmed, "classDiagram") {
			mermaidLines = append(mermaidLines, line)
			inCodeBlock = true
		}
	}

	if len(mermaidLines) > 0 {
		return strings.Join(mermaidLines, "\n")
	}

	return ""
}

// validateMermaidSyntax performs basic validation of Mermaid syntax
func validateMermaidSyntax(mermaidCode string) bool {
	if mermaidCode == "" {
		return false
	}

	lines := strings.Split(strings.TrimSpace(mermaidCode), "\n")
	if len(lines) == 0 {
		return false
	}

	// Check for valid diagram type declaration
	firstLine := strings.TrimSpace(lines[0])
	validTypes := []string{"graph", "flowchart", "sequenceDiagram", "classDiagram", "stateDiagram", "erDiagram"}

	hasValidType := false
	for _, diagType := range validTypes {
		if strings.HasPrefix(strings.ToLower(firstLine), strings.ToLower(diagType)) {
			hasValidType = true
			break
		}
	}

	if !hasValidType {
		return false
	}

	// Check for balanced brackets and parentheses
	bracketCount := strings.Count(mermaidCode, "[") - strings.Count(mermaidCode, "]")
	parenCount := strings.Count(mermaidCode, "(") - strings.Count(mermaidCode, ")")
	braceCount := strings.Count(mermaidCode, "{") - strings.Count(mermaidCode, "}")

	if bracketCount != 0 || parenCount != 0 || braceCount != 0 {
		return false
	}

	// Check for at least one node definition or connection
	hasContent := false
	for _, line := range lines[1:] { // Skip first line (diagram type)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "classDef") || strings.HasPrefix(trimmed, "class ") {
			continue
		}

		// Look for node definitions or connections
		if strings.Contains(trimmed, "[") || strings.Contains(trimmed, "(") ||
			strings.Contains(trimmed, "{") || strings.Contains(trimmed, "-->") ||
			strings.Contains(trimmed, "---") {
			hasContent = true
			break
		}
	}

	return hasContent
}

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(envVar string, defaultValue int) int {
	if value := os.Getenv(envVar); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvFloat gets a float environment variable with a default value
func getEnvFloat(envVar string, defaultValue float64) float64 {
	if value := os.Getenv(envVar); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// getEnvString gets a string environment variable with a default value
func getEnvString(envVar string, defaultValue string) string {
	if value := os.Getenv(envVar); value != "" {
		return value
	}
	return defaultValue
}

// Package main generates API documentation from MCP tool definitions
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"

	// Import all tools to register them
	_ "github.com/sammcj/mcp-devtools/internal/tools/docprocessing"
	_ "github.com/sammcj/mcp-devtools/internal/tools/internetsearch/unified"
	_ "github.com/sammcj/mcp-devtools/internal/tools/m2e"
	_ "github.com/sammcj/mcp-devtools/internal/tools/memory"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packagedocs"
	_ "github.com/sammcj/mcp-devtools/internal/tools/packageversions/unified"
	_ "github.com/sammcj/mcp-devtools/internal/tools/pdf"
	_ "github.com/sammcj/mcp-devtools/internal/tools/shadcnui"
	_ "github.com/sammcj/mcp-devtools/internal/tools/think"
	_ "github.com/sammcj/mcp-devtools/internal/tools/webfetch"
)

type ToolInfo struct {
	ToolName             string              `json:"tool_name"`
	FunctionName         string              `json:"function_name"`
	Category             string              `json:"category"`
	Description          string              `json:"description"`
	ShortDescription     string              `json:"short_description"`
	Parameters           []ParameterInfo     `json:"parameters"`
	ParameterCount       int                 `json:"parameter_count"`
	RequiredParameterCount int              `json:"required_parameter_count"`
	Examples             []ExampleInfo       `json:"examples"`
	ResponseExample      string              `json:"response_example"`
	ResponseFields       []ResponseFieldInfo `json:"response_fields"`
	ErrorCodes           []ErrorCodeInfo     `json:"error_codes"`
	Dependencies         []string            `json:"dependencies"`
	SourceFile           string              `json:"source_file"`
	RegistrationMethod   string              `json:"registration_method"`
	Notes                string              `json:"notes"`
	ReferenceLink        string              `json:"reference_link"`
}

type ParameterInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Description string   `json:"description"`
	EnumValues  []string `json:"enum_values,omitempty"`
	Example     string   `json:"example"`
}

type ExampleInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Code        string `json:"code"`
	Notes       string `json:"notes,omitempty"`
}

type ResponseFieldInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type ErrorCodeInfo struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

type CategoryInfo struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Tools       []ToolInfo `json:"tools"`
}

type RegistryData struct {
	TotalTools            int                    `json:"total_tools"`
	GeneratedAt           string                 `json:"generated_at"`
	Tools                 []ToolInfo             `json:"tools"`
	Categories            []CategoryInfo         `json:"categories"`
	CommonTypes           []TypeInfo             `json:"common_types"`
	CustomTypes           []TypeInfo             `json:"custom_types"`
	EnvironmentVariables  []EnvVarInfo           `json:"environment_variables"`
	StandardErrors        []StandardErrorInfo    `json:"standard_errors"`
}

type TypeInfo struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Example     string         `json:"example"`
	Properties  []PropertyInfo `json:"properties,omitempty"`
}

type PropertyInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type EnvVarInfo struct {
	Name        string   `json:"name"`
	RequiredFor []string `json:"required_for"`
	OptionalFor []string `json:"optional_for"`
	Description string   `json:"description"`
	Example     string   `json:"example"`
}

type StandardErrorInfo struct {
	Type    string `json:"type"`
	When    string `json:"when"`
	Format  string `json:"format"`
	Example string `json:"example"`
}

func main() {
	var (
		toolName   = flag.String("tool", "", "Generate docs for specific tool only")
		outputDir  = flag.String("output", "docs/api", "Output directory")
		templateDir = flag.String("templates", "docs/api/templates", "Template directory")
	)
	flag.Parse()

	fmt.Println("Generating MCP DevTools API Documentation...")

	// Get all registered tools
	toolList := registry.GetRegisteredTools()
	
	var tools []ToolInfo
	categories := make(map[string][]ToolInfo)

	for _, tool := range toolList {
		toolInfo := extractToolInfo(tool)
		
		// Filter by tool name if specified
		if *toolName != "" && toolInfo.FunctionName != *toolName {
			continue
		}
		
		tools = append(tools, toolInfo)
		
		// Group by category
		if categories[toolInfo.Category] == nil {
			categories[toolInfo.Category] = []ToolInfo{}
		}
		categories[toolInfo.Category] = append(categories[toolInfo.Category], toolInfo)
	}

	// Sort tools by name
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].FunctionName < tools[j].FunctionName
	})

	// Create category list
	var categoryList []CategoryInfo
	for catName, catTools := range categories {
		sort.Slice(catTools, func(i, j int) bool {
			return catTools[i].FunctionName < catTools[j].FunctionName
		})
		
		categoryList = append(categoryList, CategoryInfo{
			Name:        catName,
			Description: getCategoryDescription(catName),
			Tools:       catTools,
		})
	}
	
	sort.Slice(categoryList, func(i, j int) bool {
		return categoryList[i].Name < categoryList[j].Name
	})

	// Prepare registry data
	registryData := RegistryData{
		TotalTools:           len(tools),
		GeneratedAt:          time.Now().Format("2006-01-02 15:04:05 UTC"),
		Tools:                tools,
		Categories:           categoryList,
		CommonTypes:          getCommonTypes(),
		CustomTypes:          getCustomTypes(),
		EnvironmentVariables: getEnvironmentVariables(),
		StandardErrors:       getStandardErrors(),
	}

	// Generate documentation
	if *toolName != "" {
		// Generate single tool documentation
		if len(tools) > 0 {
			generateToolDoc(tools[0], *templateDir, *outputDir)
		} else {
			fmt.Printf("Tool '%s' not found\n", *toolName)
			os.Exit(1)
		}
	} else {
		// Generate all documentation
		generateRegistryDoc(registryData, *templateDir, *outputDir)
		
		for _, tool := range tools {
			generateToolDoc(tool, *templateDir, *outputDir)
		}
		
		generateParameterTypesDoc(registryData, *templateDir, *outputDir)
	}

	fmt.Printf("Generated documentation for %d tools in %s\n", len(tools), *outputDir)
}

func extractToolInfo(tool tools.Tool) ToolInfo {
	definition := tool.Definition()
	
	// Extract basic info
	toolInfo := ToolInfo{
		ToolName:         strings.Title(strings.ReplaceAll(definition.Name, "_", " ")),
		FunctionName:     definition.Name,
		Category:         inferCategory(definition.Name),
		Description:      definition.Description,
		ShortDescription: truncateDescription(definition.Description, 100),
		Dependencies:     extractDependencies(definition.Name),
		SourceFile:       findSourceFile(definition.Name),
		RegistrationMethod: "registry.Register()",
		ReferenceLink:    fmt.Sprintf("%s.md", definition.Name),
	}

	// Extract parameters
	if definition.InputSchema != nil && definition.InputSchema.Properties != nil {
		for paramName, paramSchema := range definition.InputSchema.Properties {
			param := ParameterInfo{
				Name:        paramName,
				Type:        getParameterType(paramSchema),
				Required:    isRequired(paramName, definition.InputSchema.Required),
				Description: getParameterDescription(paramSchema),
				Example:     getParameterExample(paramName, paramSchema),
			}
			
			if enum := getEnumValues(paramSchema); len(enum) > 0 {
				param.EnumValues = enum
			}
			
			if defaultVal := getDefaultValue(paramSchema); defaultVal != "" {
				param.Default = defaultVal
			}
			
			toolInfo.Parameters = append(toolInfo.Parameters, param)
			toolInfo.ParameterCount++
			
			if param.Required {
				toolInfo.RequiredParameterCount++
			}
		}
	}

	// Sort parameters by name
	sort.Slice(toolInfo.Parameters, func(i, j int) bool {
		return toolInfo.Parameters[i].Name < toolInfo.Parameters[j].Name
	})

	// Add examples and response info
	toolInfo.Examples = getToolExamples(definition.Name)
	toolInfo.ResponseExample = getResponseExample(definition.Name)
	toolInfo.ResponseFields = getResponseFields(definition.Name)
	toolInfo.ErrorCodes = getErrorCodes(definition.Name)

	return toolInfo
}

func generateToolDoc(tool ToolInfo, templateDir, outputDir string) error {
	templatePath := filepath.Join(templateDir, "tool-reference.md.tmpl")
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.md", tool.FunctionName))

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer file.Close()

	// Add generation timestamp
	tool.GeneratedAt = time.Now().Format("2006-01-02 15:04:05 UTC")

	return tmpl.Execute(file, tool)
}

func generateRegistryDoc(data RegistryData, templateDir, outputDir string) error {
	templatePath := filepath.Join(templateDir, "tool-registry.md.tmpl")
	outputPath := filepath.Join(outputDir, "tool-registry.md")

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}

func generateParameterTypesDoc(data RegistryData, templateDir, outputDir string) error {
	templatePath := filepath.Join(templateDir, "parameter-types.md.tmpl")
	outputPath := filepath.Join(outputDir, "parameter-types.md")

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}

// Helper functions

func inferCategory(toolName string) string {
	categories := map[string]string{
		"internet_search":    "Search & Discovery",
		"search_packages":    "Search & Discovery",
		"resolve_library_id": "Search & Discovery",
		"get_library_docs":   "Search & Discovery",
		"process_document":   "Document Processing",
		"pdf":                "Document Processing",
		"fetch_url":          "Web & Network",
		"think":              "Intelligence & Memory",
		"memory":             "Intelligence & Memory",
		"shadcn":             "UI & Utilities",
		"murican_to_english": "UI & Utilities",
	}
	
	if category, exists := categories[toolName]; exists {
		return category
	}
	return "Other"
}

func getCategoryDescription(category string) string {
	descriptions := map[string]string{
		"Search & Discovery":   "Tools for finding and discovering information across various sources",
		"Document Processing":  "Tools for converting and extracting content from documents",
		"Web & Network":        "Tools for fetching and processing web content",
		"Intelligence & Memory": "Tools for reasoning and persistent storage",
		"UI & Utilities":       "User interface components and text processing utilities",
		"Other":                "Miscellaneous tools and utilities",
	}
	return descriptions[category]
}

func truncateDescription(desc string, maxLen int) string {
	if len(desc) <= maxLen {
		return desc
	}
	return desc[:maxLen-3] + "..."
}

func extractDependencies(toolName string) []string {
	deps := map[string][]string{
		"internet_search":    {"BRAVE_API_KEY (optional)", "SEARXNG_BASE_URL (optional)"},
		"process_document":   {"Python 3.10+", "Docling"},
		"get_library_docs":   {"Context7 API"},
		"resolve_library_id": {"Context7 API"},
	}
	return deps[toolName]
}

func findSourceFile(toolName string) string {
	// This would need to scan the filesystem or use go/ast to find actual source files
	pathMap := map[string]string{
		"internet_search":    "internal/tools/internetsearch/unified/internet_search.go",
		"search_packages":    "internal/tools/packageversions/unified/search_packages.go",
		"process_document":   "internal/tools/docprocessing/document_processor.go",
		"pdf":                "internal/tools/pdf/pdf.go",
		"fetch_url":          "internal/tools/webfetch/fetch_url.go",
		"think":              "internal/tools/think/think.go",
		"memory":             "internal/tools/memory/memory.go",
		"shadcn":             "internal/tools/shadcnui/unified_shadcn.go",
		"murican_to_english": "internal/tools/m2e/m2e.go",
		"resolve_library_id": "internal/tools/packagedocs/resolve_library_id.go",
		"get_library_docs":   "internal/tools/packagedocs/get_library_docs.go",
	}
	return pathMap[toolName]
}

func getParameterType(schema interface{}) string {
	// This would need to examine the actual schema structure
	// For now, return a default
	return "string"
}

func isRequired(paramName string, required []string) bool {
	for _, req := range required {
		if req == paramName {
			return true
		}
	}
	return false
}

func getParameterDescription(schema interface{}) string {
	// Extract description from schema
	return "Parameter description"
}

func getParameterExample(paramName string, schema interface{}) string {
	examples := map[string]string{
		"query":     "golang best practices",
		"url":       "https://example.com/docs",
		"ecosystem": "npm",
		"source":    "/path/to/document.pdf",
		"thought":   "I need to analyze this before proceeding...",
	}
	if example, exists := examples[paramName]; exists {
		return example
	}
	return "example value"
}

func getEnumValues(schema interface{}) []string {
	// Extract enum values from schema
	return nil
}

func getDefaultValue(schema interface{}) string {
	// Extract default value from schema
	return ""
}

func getToolExamples(toolName string) []ExampleInfo {
	// Return predefined examples for each tool
	examples := map[string][]ExampleInfo{
		"internet_search": {
			{
				Title:       "Basic Web Search",
				Description: "Search for general information",
				Code:        `{"name": "internet_search", "arguments": {"type": "web", "query": "golang best practices", "count": 10}}`,
			},
		},
	}
	return examples[toolName]
}

func getResponseExample(toolName string) string {
	return `{"result": "example response"}`
}

func getResponseFields(toolName string) []ResponseFieldInfo {
	return []ResponseFieldInfo{
		{Name: "result", Type: "string", Description: "The operation result"},
	}
}

func getErrorCodes(toolName string) []ErrorCodeInfo {
	return []ErrorCodeInfo{
		{
			Code:        "400",
			Title:       "Bad Request",
			Description: "Invalid parameters provided",
			Example:     `{"error": "missing required parameter: query"}`,
		},
	}
}

func getCommonTypes() []TypeInfo {
	return []TypeInfo{
		{
			Name:        "file_path",
			Type:        "string",
			Description: "Absolute path to a file",
			Example:     "/Users/john/documents/file.pdf",
		},
		{
			Name:        "url",
			Type:        "string", 
			Description: "HTTP or HTTPS URL",
			Example:     "https://example.com/api/docs",
		},
	}
}

func getCustomTypes() []TypeInfo {
	return []TypeInfo{}
}

func getEnvironmentVariables() []EnvVarInfo {
	return []EnvVarInfo{
		{
			Name:        "BRAVE_API_KEY",
			RequiredFor: []string{"internet_search (Brave provider)"},
			Description: "API key for Brave Search",
			Example:     "BSA1234567890abcdef",
		},
	}
}

func getStandardErrors() []StandardErrorInfo {
	return []StandardErrorInfo{
		{
			Type:    "Parameter Error",
			When:    "Required parameter is missing or invalid",
			Format:  `{"error": "description"}`,
			Example: `{"error": "missing required parameter: query"}`,
		},
	}
}
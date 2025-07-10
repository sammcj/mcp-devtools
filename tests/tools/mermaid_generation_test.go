package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools/docprocessing"
	"github.com/sirupsen/logrus"
)

func TestMermaidGeneration(t *testing.T) {
	// Load .env file if it exists
	loadDotEnv()

	// Skip if no VLM configuration is available
	if !isVLMConfigured() {
		t.Skip("VLM configuration not available - skipping Mermaid generation tests")
	}

	t.Run("VLM_Configuration_Check", func(t *testing.T) {
		// Check that VLM environment variables are set
		apiURL := os.Getenv("DOCLING_VLM_API_URL")
		model := os.Getenv("DOCLING_VLM_MODEL")
		apiKey := os.Getenv("DOCLING_VLM_API_KEY")

		if apiURL == "" {
			t.Error("DOCLING_VLM_API_URL not set")
		}
		if model == "" {
			t.Error("DOCLING_VLM_MODEL not set")
		}
		if apiKey == "" {
			t.Error("DOCLING_VLM_API_KEY not set")
		}

		t.Logf("VLM Configuration:")
		t.Logf("  API URL: %s", apiURL)
		t.Logf("  Model: %s", model)
		t.Logf("  API Key: %s", maskAPIKeyMermaid(apiKey))
	})

	t.Run("Document_Processing_With_VLM", func(t *testing.T) {
		// Test document processing with VLM external profile
		logger := logrus.New()
		logger.SetLevel(logrus.InfoLevel)

		cache := &sync.Map{}

		// Get the document processor tool
		registry.Init(logger)
		tool, exists := registry.GetTool("process_document")
		if !exists {
			t.Fatal("process_document tool not found in registry")
		}

		// Test with a document that should contain diagrams
		testPDFPath := "/Users/samm/Downloads/ocrtest/my-pdf.pdf"
		if _, err := os.Stat(testPDFPath); os.IsNotExist(err) {
			t.Skip("Test PDF not available - skipping VLM processing test")
		}

		args := map[string]interface{}{
			"source":                      testPDFPath,
			"profile":                     "llm-external",
			"inline":                      true,
			"extract_images":              true,
			"clear_file_cache":            true,
			"convert_diagrams_to_mermaid": true,
		}

		ctx := context.Background()
		result, err := tool.Execute(ctx, logger, cache, args)
		if err != nil {
			t.Fatalf("Document processing failed: %v", err)
		}

		// Parse the result
		if result == nil || len(result.Content) == 0 {
			t.Fatal("No result content returned")
		}

		// Extract the JSON response
		content := result.Content[0]
		textContent, ok := mcp.AsTextContent(content)
		if !ok {
			t.Fatal("Expected TextContent, got different type")
		}

		var response map[string]interface{}
		if err := json.Unmarshal([]byte(textContent.Text), &response); err != nil {
			t.Fatalf("Failed to parse response JSON: %v", err)
		}

		t.Logf("Processing response keys: %v", getMapKeys(response))

		// Check if processing was successful
		if success, ok := response["success"].(bool); ok && !success {
			if errorMsg, ok := response["error"].(string); ok {
				t.Fatalf("Processing failed: %s", errorMsg)
			}
		}

		// Check processing method
		if processingInfo, ok := response["processing_info"].(map[string]interface{}); ok {
			if method, ok := processingInfo["processing_method"].(string); ok {
				t.Logf("Processing method: %s", method)

				// Verify that VLM processing was used
				if !strings.Contains(method, "diagrams") && !strings.Contains(method, "charts") {
					t.Logf("Warning: Processing method doesn't indicate VLM processing was used")
				}
			}
		}

		// Check for diagrams in the response
		if diagrams, ok := response["diagrams"].([]interface{}); ok {
			t.Logf("Found %d diagrams in response", len(diagrams))

			for i, diagramInterface := range diagrams {
				if diagram, ok := diagramInterface.(map[string]interface{}); ok {
					t.Logf("Diagram %d:", i+1)
					if id, ok := diagram["id"].(string); ok {
						t.Logf("  ID: %s", id)
					}
					if diagramType, ok := diagram["type"].(string); ok {
						t.Logf("  Type: %s", diagramType)
					}
					if description, ok := diagram["description"].(string); ok {
						t.Logf("  Description: %s", truncateString(description, 100))
					}
					if mermaidCode, ok := diagram["mermaid_code"].(string); ok && mermaidCode != "" {
						t.Logf("  Mermaid Code: %s", truncateString(mermaidCode, 200))

						// Validate Mermaid syntax
						if !isValidMermaidSyntax(mermaidCode) {
							t.Errorf("Invalid Mermaid syntax in diagram %d", i+1)
						}
					} else {
						t.Logf("  No Mermaid code generated for diagram %d", i+1)
					}
				}
			}
		} else {
			t.Log("No diagrams found in response")
		}

		// Check the content for Mermaid code blocks
		if content, ok := response["content"].(string); ok {
			mermaidBlocks := extractMermaidBlocks(content)
			t.Logf("Found %d Mermaid code blocks in content", len(mermaidBlocks))

			for i, block := range mermaidBlocks {
				t.Logf("Mermaid block %d: %s", i+1, truncateString(block, 200))

				if !isValidMermaidSyntax(block) {
					t.Errorf("Invalid Mermaid syntax in content block %d", i+1)
				}
			}

			if len(mermaidBlocks) == 0 {
				t.Log("No Mermaid code blocks found in markdown content")
			}
		}
	})

	t.Run("Embedded_Scripts_With_VLM", func(t *testing.T) {
		// Test that embedded scripts work with VLM configuration
		scriptPath, err := docprocessing.GetEmbeddedScriptPath()
		if err != nil {
			t.Fatalf("Failed to get embedded script path: %v", err)
		}

		// Verify the script exists
		if _, err := os.Stat(scriptPath); err != nil {
			t.Errorf("Embedded script does not exist: %s", scriptPath)
		}

		t.Logf("Embedded script path: %s", scriptPath)
	})
}

func TestMermaidValidation(t *testing.T) {
	t.Run("Valid_Mermaid_Syntax", func(t *testing.T) {
		validMermaid := `flowchart TD
    A[Start] --> B{Decision?}
    B -->|Yes| C[Process]
    B -->|No| D[End]`

		if !isValidMermaidSyntax(validMermaid) {
			t.Error("Valid Mermaid syntax was rejected")
		}
	})

	t.Run("Invalid_Mermaid_Syntax", func(t *testing.T) {
		invalidMermaid := `flowchart TD
    A[Start --> B{Decision?
    B -->|Yes| C[Process`

		if isValidMermaidSyntax(invalidMermaid) {
			t.Error("Invalid Mermaid syntax was accepted")
		}
	})

	t.Run("Empty_Mermaid", func(t *testing.T) {
		if isValidMermaidSyntax("") {
			t.Error("Empty Mermaid syntax was accepted")
		}
	})
}

// Helper functions

func isVLMConfigured() bool {
	apiURL := os.Getenv("DOCLING_VLM_API_URL")
	model := os.Getenv("DOCLING_VLM_MODEL")
	apiKey := os.Getenv("DOCLING_VLM_API_KEY")

	return apiURL != "" && model != "" && apiKey != ""
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func extractMermaidBlocks(content string) []string {
	var blocks []string

	// Look for ```mermaid code blocks
	lines := strings.Split(content, "\n")
	var currentBlock []string
	inMermaidBlock := false

	for _, line := range lines {
		if strings.TrimSpace(line) == "```mermaid" {
			inMermaidBlock = true
			currentBlock = []string{}
		} else if strings.TrimSpace(line) == "```" && inMermaidBlock {
			inMermaidBlock = false
			if len(currentBlock) > 0 {
				blocks = append(blocks, strings.Join(currentBlock, "\n"))
			}
		} else if inMermaidBlock {
			currentBlock = append(currentBlock, line)
		}
	}

	return blocks
}

func isValidMermaidSyntax(mermaidCode string) bool {
	if strings.TrimSpace(mermaidCode) == "" {
		return false
	}

	lines := strings.Split(strings.TrimSpace(mermaidCode), "\n")
	if len(lines) == 0 {
		return false
	}

	// Check for valid diagram type declaration
	firstLine := strings.TrimSpace(strings.ToLower(lines[0]))
	validTypes := []string{"graph", "flowchart", "sequencediagram", "classdiagram", "statediagram", "erdiagram"}

	hasValidType := false
	for _, validType := range validTypes {
		if strings.HasPrefix(firstLine, validType) {
			hasValidType = true
			break
		}
	}

	if !hasValidType {
		return false
	}

	// Check for balanced brackets
	bracketCount := strings.Count(mermaidCode, "[") - strings.Count(mermaidCode, "]")
	parenCount := strings.Count(mermaidCode, "(") - strings.Count(mermaidCode, ")")
	braceCount := strings.Count(mermaidCode, "{") - strings.Count(mermaidCode, "}")

	return bracketCount == 0 && parenCount == 0 && braceCount == 0
}

// loadDotEnv loads environment variables from .env file if it exists
func loadDotEnv() {
	// Try to load .env file from current directory
	_ = godotenv.Load(".env")
}

// maskAPIKeyMermaid masks an API key for logging
func maskAPIKeyMermaid(apiKey string) string {
	if apiKey == "" {
		return "(not set)"
	}
	if len(apiKey) <= 8 {
		return "***"
	}
	return apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
}

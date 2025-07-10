package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	"github.com/sammcj/mcp-devtools/internal/tools/docprocessing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLLMClient_Connectivity(t *testing.T) {
	// Load .env file from project root
	projectRoot, err := findProjectRoot()
	require.NoError(t, err, "Failed to find project root")

	envPath := filepath.Join(projectRoot, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Skip("Skipping LLM connectivity test: .env file not found. Create .env file with LLM configuration to run this test.")
	}

	err = godotenv.Load(envPath)
	require.NoError(t, err, "Failed to load .env file")

	// Check if LLM is configured
	if !docprocessing.IsLLMConfigured() {
		t.Skip("Skipping LLM connectivity test: LLM environment variables not configured")
	}

	// Get configuration details for logging
	apiBase := os.Getenv("DOCLING_VLM_API_URL")
	modelName := os.Getenv("DOCLING_VLM_MODEL")
	apiKey := os.Getenv("DOCLING_VLM_API_KEY")

	t.Logf("Testing LLM connectivity with:")
	t.Logf("  API Base: %s", apiBase)
	t.Logf("  Model: %s", modelName)
	t.Logf("  API Key: %s", maskAPIKey(apiKey))

	// Create LLM client
	client, err := docprocessing.NewDiagramLLMClient()
	require.NoError(t, err, "Failed to create LLM client")
	require.NotNil(t, client, "LLM client should not be nil")

	t.Run("Basic_Connectivity", func(t *testing.T) {
		// Create a simple test diagram with minimal content for faster testing
		testDiagram := &docprocessing.ExtractedDiagram{
			ID:          "test_diagram_1",
			Type:        "diagram",
			Caption:     "Test connectivity diagram",
			Description: "A simple flowchart with Start -> Process -> End",
			DiagramType: "flowchart",
			Elements: []docprocessing.DiagramElement{
				{Type: "process", Content: "Start"},
				{Type: "process", Content: "Process"},
				{Type: "process", Content: "End"},
			},
		}

		// Analyze the diagram - this will make an actual API call
		analysis, err := client.AnalyseDiagram(testDiagram)

		if err != nil {
			// Log the specific error for debugging
			t.Logf("LLM API call failed: %v", err)

			// Check if it's a connectivity issue vs configuration issue
			if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "unauthorized") {
				t.Skip("Skipping test: API key authentication failed (this is expected for test keys)")
			} else if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "timeout") {
				t.Skip("Skipping test: Network connectivity issue")
			} else {
				// For other errors, we want to know about them
				t.Errorf("Unexpected LLM API error: %v", err)
			}
			return
		}

		// If we get here, the API call succeeded
		require.NotNil(t, analysis, "Analysis result should not be nil")

		t.Logf("✅ LLM API call successful!")
		t.Logf("  Description: %s", analysis.Description)
		t.Logf("  Diagram Type: %s", analysis.DiagramType)
		t.Logf("  Confidence: %.2f", analysis.Confidence)
		t.Logf("  Processing Time: %v", analysis.ProcessingTime)

		// Basic validation of response
		assert.NotEmpty(t, analysis.Description, "Analysis should include a description")
		assert.True(t, analysis.Confidence >= 0.0, "Confidence should be non-negative")
		assert.True(t, analysis.ProcessingTime > 0, "Processing time should be positive")
	})

	t.Run("Mermaid_Generation", func(t *testing.T) {
		// Create a flowchart test diagram with clear structure
		testDiagram := &docprocessing.ExtractedDiagram{
			ID:          "test_flowchart",
			Type:        "diagram",
			Caption:     "Simple flowchart",
			Description: "A flowchart showing: Start -> Decision -> Process -> End",
			DiagramType: "flowchart",
			Elements: []docprocessing.DiagramElement{
				{Type: "process", Content: "Start"},
				{Type: "decision", Content: "Is valid?"},
				{Type: "process", Content: "Process data"},
				{Type: "process", Content: "End"},
			},
		}

		// Analyze the diagram
		analysis, err := client.AnalyseDiagram(testDiagram)

		if err != nil {
			t.Logf("LLM API call failed: %v", err)
			if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "unauthorized") {
				t.Skip("Skipping test: API key authentication failed")
			}
			return
		}

		require.NotNil(t, analysis, "Analysis result should not be nil")

		t.Logf("Mermaid Generation Test Results:")
		t.Logf("  Description: %s", analysis.Description)
		t.Logf("  Generated Mermaid Code: %s", analysis.MermaidCode)

		// Verify we got a meaningful response
		assert.NotEmpty(t, analysis.Description, "Should have a description")

		// Check if Mermaid code was generated
		if analysis.MermaidCode != "" {
			t.Logf("✅ Mermaid code generation successful!")

			// Basic validation of Mermaid syntax
			mermaidLower := strings.ToLower(analysis.MermaidCode)
			hasValidStart := strings.Contains(mermaidLower, "flowchart") ||
				strings.Contains(mermaidLower, "graph") ||
				strings.Contains(mermaidLower, "sequencediagram")

			assert.True(t, hasValidStart, "Mermaid code should start with a valid diagram type")

			// Should contain some kind of node or connection syntax
			hasNodes := strings.Contains(analysis.MermaidCode, "[") ||
				strings.Contains(analysis.MermaidCode, "(") ||
				strings.Contains(analysis.MermaidCode, "-->") ||
				strings.Contains(analysis.MermaidCode, "---")

			assert.True(t, hasNodes, "Mermaid code should contain nodes or connections")
		} else {
			t.Logf("⚠️  No Mermaid code generated (model may not support this feature)")
		}
	})
}

func TestLLMClient_Configuration(t *testing.T) {
	// Load .env file from project root
	projectRoot, err := findProjectRoot()
	require.NoError(t, err, "Failed to find project root")

	envPath := filepath.Join(projectRoot, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Skip("Skipping LLM configuration test: .env file not found")
	}

	err = godotenv.Load(envPath)
	require.NoError(t, err, "Failed to load .env file")

	t.Run("Environment_Variables", func(t *testing.T) {
		// Check required environment variables
		apiBase := os.Getenv("DOCLING_VLM_API_URL")
		modelName := os.Getenv("DOCLING_VLM_MODEL")
		apiKey := os.Getenv("DOCLING_VLM_API_KEY")

		t.Logf("LLM Configuration:")
		t.Logf("  API Base: %s", apiBase)
		t.Logf("  Model Name: %s", modelName)
		t.Logf("  API Key: %s", maskAPIKey(apiKey))

		if apiBase == "" || modelName == "" || apiKey == "" {
			t.Skip("LLM environment variables not fully configured")
		}

		assert.NotEmpty(t, apiBase, "DOCLING_VLM_API_URL should be set")
		assert.NotEmpty(t, modelName, "DOCLING_VLM_MODEL should be set")
		assert.NotEmpty(t, apiKey, "DOCLING_VLM_API_KEY should be set")
		assert.NotEqual(t, "your-api-key-here", apiKey, "API key should be replaced with actual key")
	})

	t.Run("IsLLMConfigured", func(t *testing.T) {
		configured := docprocessing.IsLLMConfigured()
		t.Logf("IsLLMConfigured: %v", configured)

		if configured {
			// If configured, we should be able to create a client
			client, err := docprocessing.NewDiagramLLMClient()
			assert.NoError(t, err, "Should be able to create LLM client when configured")
			assert.NotNil(t, client, "LLM client should not be nil")
		}
	})
}

func TestLLMClient_NoConfiguration(t *testing.T) {
	// Test behaviour when no LLM configuration is available (CI environment)
	// Save current environment variables
	originalAPIBase := os.Getenv("DOCLING_VLM_API_URL")
	originalModel := os.Getenv("DOCLING_VLM_MODEL")
	originalAPIKey := os.Getenv("DOCLING_VLM_API_KEY")

	// Clear LLM environment variables
	_ = os.Unsetenv("DOCLING_VLM_API_URL")
	_ = os.Unsetenv("DOCLING_VLM_MODEL")
	_ = os.Unsetenv("DOCLING_VLM_API_KEY")

	// Restore environment variables after test
	defer func() {
		if originalAPIBase != "" {
			_ = os.Setenv("DOCLING_VLM_API_URL", originalAPIBase)
		}
		if originalModel != "" {
			_ = os.Setenv("DOCLING_VLM_MODEL", originalModel)
		}
		if originalAPIKey != "" {
			_ = os.Setenv("DOCLING_VLM_API_KEY", originalAPIKey)
		}
	}()

	t.Run("IsLLMConfigured_False", func(t *testing.T) {
		configured := docprocessing.IsLLMConfigured()
		assert.False(t, configured, "IsLLMConfigured should return false when no environment variables are set")
	})

	t.Run("NewDiagramLLMClient_Error", func(t *testing.T) {
		client, err := docprocessing.NewDiagramLLMClient()
		assert.Error(t, err, "NewDiagramLLMClient should return error when not configured")
		assert.Nil(t, client, "Client should be nil when configuration fails")
		assert.Contains(t, err.Error(), "LLM environment variables not configured", "Error should mention missing configuration")
	})
}

// Helper function to find the project root directory
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached root
		}
		dir = parent
	}

	return "", os.ErrNotExist
}

// Helper function to mask API key for logging
func maskAPIKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

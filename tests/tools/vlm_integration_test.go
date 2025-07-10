package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

func TestVLMPipeline_ActualIntegration(t *testing.T) {
	// Load .env file from project root
	projectRoot, err := findProjectRootIntegration()
	require.NoError(t, err, "Failed to find project root")

	envPath := filepath.Join(projectRoot, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Skip("Skipping VLM Pipeline integration test: .env file not found")
	}

	err = godotenv.Load(envPath)
	require.NoError(t, err, "Failed to load .env file")

	// Check if VLM is configured
	vlmAPIURL := os.Getenv("DOCLING_VLM_API_URL")
	vlmModel := os.Getenv("DOCLING_VLM_MODEL")

	if vlmAPIURL == "" || vlmModel == "" {
		t.Skip("Skipping VLM Pipeline integration test: VLM environment variables not configured")
	}

	t.Run("VLM_Pipeline_Implementation_Status", func(t *testing.T) {
		t.Logf("VLM Pipeline Configuration:")
		t.Logf("  API URL: %s", vlmAPIURL)
		t.Logf("  Model: %s", vlmModel)
		t.Logf("  API Key: %s", maskAPIKeyIntegration(os.Getenv("DOCLING_VLM_API_KEY")))

		t.Logf("\n🔍 ANALYSIS: The VLM Pipeline integration is currently incomplete.")
		t.Logf("The Python processor has placeholder functions that don't actually call your Ollama server.")
		t.Logf("\nFunctions that need implementation:")
		t.Logf("  - analyze_with_vlm_pipeline() - Currently returns mock data")
		t.Logf("  - generate_vlm_description() - Currently returns None")
		t.Logf("  - VlmPipeline class integration - Not implemented")
		t.Logf("\nThis explains why your Ollama server doesn't receive any requests.")
		t.Logf("The 'unknown error' occurs because the VLM Pipeline functions fail silently.")

		// This test documents the current state rather than testing functionality
		t.Logf("\n✅ Environment variables are correctly configured")
		t.Logf("❌ VLM Pipeline implementation is incomplete")
		t.Logf("📋 Next step: Implement actual VLM Pipeline API calls in Python processor")
	})
}

// Helper function to find the project root directory
func findProjectRootIntegration() (string, error) {
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
func maskAPIKeyIntegration(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

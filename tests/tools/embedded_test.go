package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/docprocessing"
)

func TestEmbeddedScripts(t *testing.T) {
	t.Run("IsEmbeddedScriptsAvailable", func(t *testing.T) {
		available := docprocessing.IsEmbeddedScriptsAvailable()
		if !available {
			t.Error("Embedded scripts should be available")
		}
	})

	t.Run("GetEmbeddedScriptPath", func(t *testing.T) {
		scriptPath, err := docprocessing.GetEmbeddedScriptPath()
		if err != nil {
			t.Fatalf("Failed to get embedded script path: %v", err)
		}

		// Verify the script path exists
		if _, err := os.Stat(scriptPath); err != nil {
			t.Errorf("Embedded script path does not exist: %s", scriptPath)
		}

		// Verify it's in a temporary directory
		if !strings.Contains(scriptPath, "mcp-devtools-python-") {
			t.Errorf("Script path should be in a temporary directory with mcp-devtools-python- prefix, got: %s", scriptPath)
		}

		// Verify it's the main script
		if !strings.HasSuffix(scriptPath, "docling_processor.py") {
			t.Errorf("Script path should end with docling_processor.py, got: %s", scriptPath)
		}

		// Verify the script is executable/readable
		content, err := os.ReadFile(scriptPath)
		if err != nil {
			t.Errorf("Failed to read embedded script: %v", err)
		}

		// Verify it's a Python script
		contentStr := string(content)
		if !strings.Contains(contentStr, "#!/usr/bin/env python") && !strings.Contains(contentStr, "import") {
			t.Error("Embedded script doesn't appear to be a valid Python script")
		}

		// Verify it contains expected functions
		expectedFunctions := []string{
			"def process_document",
			"def main",
		}

		for _, expectedFunc := range expectedFunctions {
			if !strings.Contains(contentStr, expectedFunc) {
				t.Errorf("Embedded script missing expected function: %s", expectedFunc)
			}
		}
	})

	t.Run("MultipleCallsReturnSamePath", func(t *testing.T) {
		// Test that multiple calls return the same path (singleton behavior)
		path1, err1 := docprocessing.GetEmbeddedScriptPath()
		path2, err2 := docprocessing.GetEmbeddedScriptPath()

		if err1 != nil || err2 != nil {
			t.Fatalf("Failed to get embedded script paths: %v, %v", err1, err2)
		}

		if path1 != path2 {
			t.Errorf("Multiple calls should return the same path, got: %s and %s", path1, path2)
		}
	})

	t.Run("AllPythonFilesExtracted", func(t *testing.T) {
		scriptPath, err := docprocessing.GetEmbeddedScriptPath()
		if err != nil {
			t.Fatalf("Failed to get embedded script path: %v", err)
		}

		// Get the directory containing the extracted scripts
		scriptDir := filepath.Dir(scriptPath)

		// Check that all expected Python files are present
		expectedFiles := []string{
			"docling_processor.py",
			"image_processing.py",
			"table_processing.py",
		}

		for _, expectedFile := range expectedFiles {
			filePath := filepath.Join(scriptDir, expectedFile)
			if _, err := os.Stat(filePath); err != nil {
				t.Errorf("Expected embedded file not found: %s", expectedFile)
			}
		}
	})

	t.Run("CleanupEmbeddedScripts", func(t *testing.T) {
		// Get the script path first to ensure extraction
		scriptPath, err := docprocessing.GetEmbeddedScriptPath()
		if err != nil {
			t.Fatalf("Failed to get embedded script path: %v", err)
		}

		scriptDir := filepath.Dir(scriptPath)

		// Verify the directory exists before cleanup
		if _, err := os.Stat(scriptDir); err != nil {
			t.Fatalf("Script directory should exist before cleanup: %v", err)
		}

		// Test cleanup
		err = docprocessing.CleanupEmbeddedScripts()
		if err != nil {
			t.Errorf("Failed to cleanup embedded scripts: %v", err)
		}

		// Note: We can't verify the directory is gone because the OS will clean it up
		// and the cleanup function is designed to be safe to call multiple times
	})
}

func TestEmbeddedScriptsIntegration(t *testing.T) {
	t.Run("ConfigUsesEmbeddedScripts", func(t *testing.T) {
		// Create a config and test that it can find the embedded scripts
		config := docprocessing.DefaultConfig()

		// Get the script path - this should use embedded scripts as fallback
		scriptPath := config.GetScriptPath()

		// If embedded scripts are available, the path should be valid
		if docprocessing.IsEmbeddedScriptsAvailable() {
			// The path should either be a filesystem path or an embedded path
			// We can't guarantee which one without manipulating the filesystem,
			// but we can verify that GetScriptPath() returns something reasonable
			if scriptPath == "" {
				t.Error("GetScriptPath() should return a valid path when embedded scripts are available")
			}
		}
	})
}

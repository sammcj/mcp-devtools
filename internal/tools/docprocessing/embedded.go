package docprocessing

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

//go:embed python/*.py
var embeddedPythonFiles embed.FS

var (
	extractedScriptPath string
	extractOnce         sync.Once
	extractError        error
)

// GetEmbeddedScriptPath extracts the embedded Python scripts to a temporary directory
// and returns the path to the main docling_processor.py script.
// This is thread-safe and only extracts once per process.
func GetEmbeddedScriptPath() (string, error) {
	extractOnce.Do(func() {
		extractedScriptPath, extractError = extractEmbeddedScripts()
	})
	return extractedScriptPath, extractError
}

// extractEmbeddedScripts extracts all embedded Python files to a temporary directory
func extractEmbeddedScripts() (string, error) {
	// Create a temporary directory for the Python scripts
	tempDir, err := os.MkdirTemp("", "mcp-devtools-python-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	// Read the embedded files and extract them
	entries, err := embeddedPythonFiles.ReadDir("python")
	if err != nil {
		return "", fmt.Errorf("failed to read embedded python directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip directories
		}

		// Only extract .py files
		if filepath.Ext(entry.Name()) != ".py" {
			continue
		}

		// Read the embedded file
		embeddedPath := filepath.Join("python", entry.Name())
		content, err := embeddedPythonFiles.ReadFile(embeddedPath)
		if err != nil {
			return "", fmt.Errorf("failed to read embedded file %s: %w", embeddedPath, err)
		}

		// Write to temporary directory
		extractedPath := filepath.Join(tempDir, entry.Name())
		if err := os.WriteFile(extractedPath, content, 0700); err != nil {
			return "", fmt.Errorf("failed to write extracted file %s: %w", extractedPath, err)
		}
	}

	// Return the path to the main script
	mainScriptPath := filepath.Join(tempDir, "docling_processor.py")

	// Verify the main script was extracted
	if _, err := os.Stat(mainScriptPath); err != nil {
		return "", fmt.Errorf("main script not found after extraction: %w", err)
	}

	return mainScriptPath, nil
}

// CleanupEmbeddedScripts removes the temporary directory containing extracted scripts
// This should be called during graceful shutdown, but the OS will clean up temp files anyway
func CleanupEmbeddedScripts() error {
	if extractedScriptPath == "" {
		return nil // Nothing to clean up
	}

	tempDir := filepath.Dir(extractedScriptPath)
	return os.RemoveAll(tempDir)
}

// IsEmbeddedScriptsAvailable checks if the embedded Python scripts are available
func IsEmbeddedScriptsAvailable() bool {
	// Check if we can read the main script from embedded files
	_, err := embeddedPythonFiles.ReadFile("python/docling_processor.py")
	return err == nil
}

// ReadEmbeddedFile reads an embedded file and returns its content
func ReadEmbeddedFile(path string) ([]byte, error) {
	return embeddedPythonFiles.ReadFile(path)
}

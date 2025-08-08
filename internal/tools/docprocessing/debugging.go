package docprocessing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// rotateDebugLogs manages debug log rotation, keeping logs from the past 48 hours
func rotateDebugLogs() {
	debugLogDir := filepath.Join(os.Getenv("HOME"), ".mcp-devtools")
	debugLogPath := filepath.Join(debugLogDir, "debug.log")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(debugLogDir, 0700); err != nil {
		return // Silently fail to avoid MCP protocol interference
	}

	// Check if current log file exists and is older than 24 hours
	if info, err := os.Stat(debugLogPath); err == nil {
		// If the current log is older than 24 hours, rotate it
		if time.Since(info.ModTime()) > 24*time.Hour {
			// Create timestamped backup filename
			timestamp := info.ModTime().Format("2006-01-02_15-04-05")
			backupPath := filepath.Join(debugLogDir, fmt.Sprintf("debug_%s.log", timestamp))

			// Move current log to backup
			_ = os.Rename(debugLogPath, backupPath)
		}
	}

	// Clean up old log files (older than 48 hours)
	cleanupOldLogs(debugLogDir, 48*time.Hour)
}

// cleanupOldLogs removes log files older than the specified duration
func cleanupOldLogs(logDir string, maxAge time.Duration) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return // Silently fail
	}

	cutoffTime := time.Now().Add(-maxAge)

	for _, entry := range entries {
		// Only process debug log files
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "debug_") || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}

		filePath := filepath.Join(logDir, entry.Name())
		if info, err := os.Stat(filePath); err == nil {
			if info.ModTime().Before(cutoffTime) {
				_ = os.Remove(filePath) // Silently remove old files
			}
		}
	}
}

// getDebugInfo returns debug information including environment variables (with secrets masked)
func (t *DocumentProcessorTool) getDebugInfo() map[string]interface{} {
	debugInfo := map[string]interface{}{
		"debug_mode":  true,
		"timestamp":   time.Now().Format(time.RFC3339),
		"system_info": t.config.GetSystemInfo(),
	}

	// Environment variables related to document processing
	envVars := map[string]interface{}{
		// LLM Configuration
		"DOCLING_VLM_API_URL":     maskSecret(os.Getenv("DOCLING_VLM_API_URL")),
		"DOCLING_VLM_MODEL":       os.Getenv("DOCLING_VLM_MODEL"),
		"DOCLING_VLM_API_KEY":     maskSecret(os.Getenv("DOCLING_VLM_API_KEY")),
		"DOCLING_LLM_MAX_TOKENS":  os.Getenv("DOCLING_LLM_MAX_TOKENS"),
		"DOCLING_LLM_TEMPERATURE": os.Getenv("DOCLING_LLM_TEMPERATURE"),
		"DOCLING_LLM_TIMEOUT":     os.Getenv("DOCLING_LLM_TIMEOUT"),

		// Cache Configuration
		"DOCLING_CACHE_MAX_AGE_HOURS": os.Getenv("DOCLING_CACHE_MAX_AGE_HOURS"),
		"DOCLING_CACHE_ENABLED":       os.Getenv("DOCLING_CACHE_ENABLED"),

		// Processing Configuration
		"DOCLING_TIMEOUT":     os.Getenv("DOCLING_TIMEOUT"),
		"DOCLING_MAX_FILE_MB": os.Getenv("DOCLING_MAX_FILE_MB"),

		// Certificate Configuration
		"SSL_CERT_FILE": maskSecret(os.Getenv("SSL_CERT_FILE")),
		"SSL_CERT_DIR":  os.Getenv("SSL_CERT_DIR"),
	}

	debugInfo["environment_variables"] = envVars

	// LLM Configuration Status
	llmStatus := map[string]interface{}{
		"configured":   IsLLMConfigured(),
		"api_base_set": os.Getenv("DOCLING_VLM_API_URL") != "",
		"model_set":    os.Getenv("DOCLING_VLM_MODEL") != "",
		"api_key_set":  os.Getenv("DOCLING_VLM_API_KEY") != "",
	}

	if IsLLMConfigured() {
		// Test LLM client creation (but don't make API calls)
		if _, err := NewDiagramLLMClient(); err != nil {
			llmStatus["client_creation_error"] = err.Error()
		} else {
			llmStatus["client_creation"] = "success"
		}
	}

	debugInfo["llm_status"] = llmStatus

	// Configuration details
	configInfo := map[string]interface{}{
		"python_path":       t.config.PythonPath,
		"script_path":       t.config.GetScriptPath(),
		"cache_enabled":     t.config.CacheEnabled,
		"cache_directory":   "cache", // Default cache directory
		"timeout":           t.config.Timeout,
		"max_file_size":     t.config.MaxFileSize,
		"docling_available": t.config.isDoclingAvailable(),
	}

	debugInfo["configuration"] = configInfo

	return debugInfo
}

// maskSecret masks sensitive information in environment variables
func maskSecret(value string) string {
	if value == "" {
		return "(not set)"
	}
	if len(value) <= 8 {
		return "***"
	}
	// Show first 4 and last 4 characters, mask the middle
	return value[:4] + "..." + value[len(value)-4:]
}

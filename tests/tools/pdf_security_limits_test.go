package tools_test

import (
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/pdf"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestPDFTool_FileSize_DefaultLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("PDF_MAX_FILE_SIZE")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PDF_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("PDF_MAX_FILE_SIZE", originalValue)
		}
	}()

	// Clear environment variable to test default behaviour
	_ = os.Unsetenv("PDF_MAX_FILE_SIZE")

	tool := &pdf.PDFTool{}

	// Test with file size under limit (should pass)
	smallFileSize := int64(100 * 1024 * 1024) // 100MB
	err := tool.ValidateFileSize(smallFileSize)
	testutils.AssertNoError(t, err)

	// Test with file size over limit (should fail)
	largeFileSize := int64(300 * 1024 * 1024) // 300MB (over 200MB default limit)
	err = tool.ValidateFileSize(largeFileSize)
	testutils.AssertError(t, err)
	testutils.AssertTrue(t, contains(err.Error(), "exceeds maximum allowed size"))
	testutils.AssertTrue(t, contains(err.Error(), "200.0MB"))
}

func TestPDFTool_FileSize_CustomLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("PDF_MAX_FILE_SIZE")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PDF_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("PDF_MAX_FILE_SIZE", originalValue)
		}
	}()

	// Set custom limit (100MB = 104857600 bytes)
	err := os.Setenv("PDF_MAX_FILE_SIZE", "104857600")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &pdf.PDFTool{}

	// Test with file size under custom limit (should pass)
	smallFileSize := int64(50 * 1024 * 1024) // 50MB
	err = tool.ValidateFileSize(smallFileSize)
	testutils.AssertNoError(t, err)

	// Test with file size over custom limit (should fail)
	largeFileSize := int64(150 * 1024 * 1024) // 150MB (over 100MB custom limit)
	err = tool.ValidateFileSize(largeFileSize)
	testutils.AssertError(t, err)
	testutils.AssertTrue(t, contains(err.Error(), "exceeds maximum allowed size"))
	testutils.AssertTrue(t, contains(err.Error(), "100.0MB"))
}

func TestPDFTool_FileSize_InvalidEnvironmentVariable(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("PDF_MAX_FILE_SIZE")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PDF_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("PDF_MAX_FILE_SIZE", originalValue)
		}
	}()

	// Set invalid environment variable value
	err := os.Setenv("PDF_MAX_FILE_SIZE", "invalid")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &pdf.PDFTool{}

	// Should fall back to default when invalid value is provided
	maxSize := tool.GetMaxFileSize()
	testutils.AssertEqual(t, pdf.DefaultMaxFileSize, maxSize)
}

func TestPDFTool_FileSize_ZeroEnvironmentVariable(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("PDF_MAX_FILE_SIZE")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PDF_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("PDF_MAX_FILE_SIZE", originalValue)
		}
	}()

	// Set zero environment variable value
	err := os.Setenv("PDF_MAX_FILE_SIZE", "0")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &pdf.PDFTool{}

	// Should fall back to default when zero value is provided
	maxSize := tool.GetMaxFileSize()
	testutils.AssertEqual(t, pdf.DefaultMaxFileSize, maxSize)
}

func TestPDFTool_MemoryLimit_DefaultLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("PDF_MAX_MEMORY_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PDF_MAX_MEMORY_LIMIT")
		} else {
			_ = os.Setenv("PDF_MAX_MEMORY_LIMIT", originalValue)
		}
	}()

	// Clear environment variable to test default behaviour
	_ = os.Unsetenv("PDF_MAX_MEMORY_LIMIT")

	tool := &pdf.PDFTool{}

	// Test default memory limit
	maxMemory := tool.GetMaxMemoryLimit()
	testutils.AssertEqual(t, pdf.DefaultMaxMemoryLimit, maxMemory)
}

func TestPDFTool_MemoryLimit_CustomLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("PDF_MAX_MEMORY_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PDF_MAX_MEMORY_LIMIT")
		} else {
			_ = os.Setenv("PDF_MAX_MEMORY_LIMIT", originalValue)
		}
	}()

	// Set custom limit (2GB = 2147483648 bytes)
	err := os.Setenv("PDF_MAX_MEMORY_LIMIT", "2147483648")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &pdf.PDFTool{}

	// Test custom memory limit
	maxMemory := tool.GetMaxMemoryLimit()
	testutils.AssertEqual(t, int64(2147483648), maxMemory)
}

func TestPDFTool_MemoryLimit_InvalidEnvironmentVariable(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("PDF_MAX_MEMORY_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("PDF_MAX_MEMORY_LIMIT")
		} else {
			_ = os.Setenv("PDF_MAX_MEMORY_LIMIT", originalValue)
		}
	}()

	// Set invalid environment variable value
	err := os.Setenv("PDF_MAX_MEMORY_LIMIT", "not-a-number")
	if err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	tool := &pdf.PDFTool{}

	// Should fall back to default when invalid value is provided
	maxMemory := tool.GetMaxMemoryLimit()
	testutils.AssertEqual(t, pdf.DefaultMaxMemoryLimit, maxMemory)
}

func TestPDFConstants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, "PDF_MAX_FILE_SIZE", pdf.PDFMaxFileSizeEnvVar)
	testutils.AssertEqual(t, "PDF_MAX_MEMORY_LIMIT", pdf.PDFMaxMemoryLimitEnvVar)
	testutils.AssertEqual(t, int64(200*1024*1024), pdf.DefaultMaxFileSize)
	testutils.AssertEqual(t, int64(5*1024*1024*1024), pdf.DefaultMaxMemoryLimit)
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		containsMiddle(s, substr))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

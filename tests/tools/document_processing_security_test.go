package tools_test

import (
	"os"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/docprocessing"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

func TestDocumentProcessing_DefaultLimits(t *testing.T) {
	// Save original environment variables
	originalMemoryLimit := os.Getenv("DOCLING_MAX_MEMORY_LIMIT")
	originalFileSize := os.Getenv("DOCLING_MAX_FILE_SIZE")
	defer func() {
		if originalMemoryLimit == "" {
			_ = os.Unsetenv("DOCLING_MAX_MEMORY_LIMIT")
		} else {
			_ = os.Setenv("DOCLING_MAX_MEMORY_LIMIT", originalMemoryLimit)
		}
		if originalFileSize == "" {
			_ = os.Unsetenv("DOCLING_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("DOCLING_MAX_FILE_SIZE", originalFileSize)
		}
	}()

	// Clear environment variables to test defaults
	_ = os.Unsetenv("DOCLING_MAX_MEMORY_LIMIT")
	_ = os.Unsetenv("DOCLING_MAX_FILE_SIZE")

	config := docprocessing.DefaultConfig()

	// Test default memory limit
	testutils.AssertEqual(t, docprocessing.DefaultMaxMemoryLimit, config.GetMaxMemoryLimit())

	// Test default file size limit
	testutils.AssertEqual(t, docprocessing.DefaultMaxFileSizeMB, config.MaxFileSize)

	// Test memory limit validation
	err := config.ValidateMemoryLimit()
	testutils.AssertNoError(t, err)
}

func TestDocumentProcessing_CustomMemoryLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("DOCLING_MAX_MEMORY_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("DOCLING_MAX_MEMORY_LIMIT")
		} else {
			_ = os.Setenv("DOCLING_MAX_MEMORY_LIMIT", originalValue)
		}
	}()

	// Set custom memory limit (2GB = 2147483648 bytes)
	err := os.Setenv("DOCLING_MAX_MEMORY_LIMIT", "2147483648")
	testutils.AssertNoError(t, err)

	config := docprocessing.LoadConfig()

	// Test custom memory limit
	testutils.AssertEqual(t, int64(2147483648), config.GetMaxMemoryLimit())
}

func TestDocumentProcessing_InvalidMemoryLimit(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("DOCLING_MAX_MEMORY_LIMIT")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("DOCLING_MAX_MEMORY_LIMIT")
		} else {
			_ = os.Setenv("DOCLING_MAX_MEMORY_LIMIT", originalValue)
		}
	}()

	// Set invalid memory limit
	err := os.Setenv("DOCLING_MAX_MEMORY_LIMIT", "invalid")
	testutils.AssertNoError(t, err)

	config := docprocessing.LoadConfig()

	// Should fall back to default when invalid value is provided
	testutils.AssertEqual(t, docprocessing.DefaultMaxMemoryLimit, config.GetMaxMemoryLimit())
}

func TestDocumentProcessing_FileTypeValidation(t *testing.T) {
	config := docprocessing.DefaultConfig()

	// Test supported file types
	supportedFiles := []string{
		"document.pdf",
		"document.docx",
		"document.doc",
		"document.xlsx",
		"document.xls",
		"document.pptx",
		"document.ppt",
		"document.txt",
		"document.md",
		"document.rtf",
		"document.html",
		"document.htm",
		"document.csv",
		"image.png",
		"image.jpg",
		"image.jpeg",
		"image.gif",
		"image.bmp",
		"image.tiff",
		"image.tif",
	}

	for _, file := range supportedFiles {
		err := config.ValidateFileType(file)
		testutils.AssertNoError(t, err)
	}

	// Test unsupported file types
	unsupportedFiles := []string{
		"archive.zip",
		"script.py",
		"audio.mp3",
		"video.mp4",
		"application.exe",
		"data.bin",
		"config.ini",
		"style.css",
		"script.js",
	}

	for _, file := range unsupportedFiles {
		err := config.ValidateFileType(file)
		if err == nil {
			t.Errorf("Expected error for unsupported file type '%s', but got none", file)
		} else {
			t.Logf("File '%s' produced expected error: %s", file, err.Error())
		}
		testutils.AssertError(t, err)
		testutils.AssertTrue(t, contains(err.Error(), "unsupported file type"))
	}

	// Test file with no extension
	err := config.ValidateFileType("file_without_extension")
	testutils.AssertError(t, err)
	testutils.AssertTrue(t, contains(err.Error(), "no extension"))
}

func TestDocumentProcessing_FileSizeValidation(t *testing.T) {
	config := docprocessing.DefaultConfig()

	// Test file size under limit (should pass)
	smallFileSize := int64(50 * 1024 * 1024) // 50MB
	err := config.ValidateFileSize(smallFileSize)
	testutils.AssertNoError(t, err)

	// Test file size over limit (should fail)
	largeFileSize := int64(200 * 1024 * 1024) // 200MB (over 100MB default limit)
	err = config.ValidateFileSize(largeFileSize)
	testutils.AssertError(t, err)
	testutils.AssertTrue(t, contains(err.Error(), "exceeds maximum allowed size"))
	testutils.AssertTrue(t, contains(err.Error(), "100.0MB"))
}

func TestDocumentProcessing_CustomFileSize(t *testing.T) {
	// Save original environment variable
	originalValue := os.Getenv("DOCLING_MAX_FILE_SIZE")
	defer func() {
		if originalValue == "" {
			_ = os.Unsetenv("DOCLING_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("DOCLING_MAX_FILE_SIZE", originalValue)
		}
	}()

	// Set custom file size limit (50MB)
	err := os.Setenv("DOCLING_MAX_FILE_SIZE", "50")
	testutils.AssertNoError(t, err)

	config := docprocessing.LoadConfig()

	// Test file size under custom limit (should pass)
	smallFileSize := int64(25 * 1024 * 1024) // 25MB
	err = config.ValidateFileSize(smallFileSize)
	testutils.AssertNoError(t, err)

	// Test file size over custom limit (should fail)
	largeFileSize := int64(75 * 1024 * 1024) // 75MB (over 50MB custom limit)
	err = config.ValidateFileSize(largeFileSize)
	testutils.AssertError(t, err)
	testutils.AssertTrue(t, contains(err.Error(), "exceeds maximum allowed size"))
	testutils.AssertTrue(t, contains(err.Error(), "50.0MB"))
}

func TestDocumentProcessing_Constants(t *testing.T) {
	// Test that constants are exported and have expected values
	testutils.AssertEqual(t, "DOCLING_MAX_MEMORY_LIMIT", docprocessing.DocProcessingMaxMemoryLimitEnvVar)
	testutils.AssertEqual(t, "DOCLING_MAX_FILE_SIZE", docprocessing.DocProcessingMaxFileSizeEnvVar)
	testutils.AssertEqual(t, int64(5*1024*1024*1024), docprocessing.DefaultMaxMemoryLimit)
	testutils.AssertEqual(t, 100, docprocessing.DefaultMaxFileSizeMB)
}

func TestDocumentProcessing_ZeroValues(t *testing.T) {
	// Save original environment variables
	originalMemoryLimit := os.Getenv("DOCLING_MAX_MEMORY_LIMIT")
	originalFileSize := os.Getenv("DOCLING_MAX_FILE_SIZE")
	defer func() {
		if originalMemoryLimit == "" {
			_ = os.Unsetenv("DOCLING_MAX_MEMORY_LIMIT")
		} else {
			_ = os.Setenv("DOCLING_MAX_MEMORY_LIMIT", originalMemoryLimit)
		}
		if originalFileSize == "" {
			_ = os.Unsetenv("DOCLING_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("DOCLING_MAX_FILE_SIZE", originalFileSize)
		}
	}()

	// Test zero memory limit
	err := os.Setenv("DOCLING_MAX_MEMORY_LIMIT", "0")
	testutils.AssertNoError(t, err)

	config := docprocessing.LoadConfig()
	// Should fall back to default when zero value is provided
	testutils.AssertEqual(t, docprocessing.DefaultMaxMemoryLimit, config.GetMaxMemoryLimit())

	// Test zero file size
	err = os.Setenv("DOCLING_MAX_FILE_SIZE", "0")
	testutils.AssertNoError(t, err)

	config = docprocessing.LoadConfig()
	// Should fall back to default when zero value is provided
	testutils.AssertEqual(t, docprocessing.DefaultMaxFileSizeMB, config.MaxFileSize)
}

func TestDocumentProcessing_NegativeValues(t *testing.T) {
	// Save original environment variables
	originalMemoryLimit := os.Getenv("DOCLING_MAX_MEMORY_LIMIT")
	originalFileSize := os.Getenv("DOCLING_MAX_FILE_SIZE")
	defer func() {
		if originalMemoryLimit == "" {
			_ = os.Unsetenv("DOCLING_MAX_MEMORY_LIMIT")
		} else {
			_ = os.Setenv("DOCLING_MAX_MEMORY_LIMIT", originalMemoryLimit)
		}
		if originalFileSize == "" {
			_ = os.Unsetenv("DOCLING_MAX_FILE_SIZE")
		} else {
			_ = os.Setenv("DOCLING_MAX_FILE_SIZE", originalFileSize)
		}
	}()

	// Test negative memory limit
	err := os.Setenv("DOCLING_MAX_MEMORY_LIMIT", "-1000")
	testutils.AssertNoError(t, err)

	config := docprocessing.LoadConfig()
	// Should fall back to default when negative value is provided
	testutils.AssertEqual(t, docprocessing.DefaultMaxMemoryLimit, config.GetMaxMemoryLimit())

	// Test negative file size
	err = os.Setenv("DOCLING_MAX_FILE_SIZE", "-50")
	testutils.AssertNoError(t, err)

	config = docprocessing.LoadConfig()
	// Should fall back to default when negative value is provided
	testutils.AssertEqual(t, docprocessing.DefaultMaxFileSizeMB, config.MaxFileSize)
}

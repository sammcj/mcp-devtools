package unit

import (
	"os"
	"strings"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/security"
)

func TestNewOperations(t *testing.T) {
	ops := security.NewOperations("test-tool")
	if ops == nil {
		t.Fatal("NewOperations returned nil")
	}
}

func TestSafeFileOperations(t *testing.T) {
	ops := security.NewOperations("test-tool")

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "helper_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			t.Logf("Failed to remove temp file: %v", removeErr)
		}
	}()
	if closeErr := tmpFile.Close(); closeErr != nil {
		t.Fatalf("Failed to close temp file: %v", closeErr)
	}

	testContent := []byte("Hello, World!\nThis is test content.")

	// Test SafeFileWrite
	err = ops.SafeFileWrite(tmpFile.Name(), testContent)
	if err != nil {
		t.Fatalf("SafeFileWrite failed: %v", err)
	}

	// Test SafeFileRead
	safeFile, err := ops.SafeFileRead(tmpFile.Name())
	if err != nil {
		t.Fatalf("SafeFileRead failed: %v", err)
	}

	// Verify content integrity
	if string(safeFile.Content) != string(testContent) {
		t.Errorf("Content integrity violated. Expected %q, got %q", testContent, safeFile.Content)
	}

	// Verify metadata
	if safeFile.Path != tmpFile.Name() {
		t.Errorf("Path mismatch. Expected %q, got %q", tmpFile.Name(), safeFile.Path)
	}

	if safeFile.Info == nil {
		t.Error("File info should not be nil")
	}
}

func TestContentTypeDetection(t *testing.T) {
	ops := security.NewOperations("test-tool")

	tests := []struct {
		name        string
		content     []byte
		contentType string
		shouldSkip  bool
	}{
		{
			name:        "text content",
			content:     []byte("Hello, World!"),
			contentType: "text/plain",
			shouldSkip:  false,
		},
		{
			name:        "binary content - image",
			content:     []byte{0xFF, 0xD8, 0xFF, 0xE0}, // JPEG header
			contentType: "image/jpeg",
			shouldSkip:  true,
		},
		{
			name:        "binary content - PDF header",
			content:     []byte("%PDF-1.4"),
			contentType: "",    // No content type for file operations
			shouldSkip:  false, // PDF header is valid UTF-8 text, so it gets analyzed
		},
		{
			name:        "binary content - null bytes",
			content:     []byte{0x00, 0x01, 0x02},
			contentType: "",
			shouldSkip:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use reflection to access private method for testing
			// In a real implementation, you might want to make this method public for testing
			// or create a test helper that exposes the logic

			// For now, we'll test indirectly through the file operations
			tmpFile, err := os.CreateTemp("", "content_test_*.bin")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer func() {
				if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
					t.Logf("Failed to remove temp file: %v", removeErr)
				}
			}()
			if closeErr := tmpFile.Close(); closeErr != nil {
				t.Fatalf("Failed to close temp file: %v", closeErr)
			}

			err = ops.SafeFileWrite(tmpFile.Name(), tt.content)
			if err != nil {
				t.Fatalf("SafeFileWrite failed: %v", err)
			}

			safeFile, err := ops.SafeFileRead(tmpFile.Name())
			if err != nil {
				t.Fatalf("SafeFileRead failed: %v", err)
			}

			// Verify content integrity regardless of type
			if string(safeFile.Content) != string(tt.content) {
				t.Errorf("Content integrity violated. Expected %q, got %q", tt.content, safeFile.Content)
			}

			// For binary content, security result should be nil (no analysis performed)
			if tt.shouldSkip && safeFile.SecurityResult != nil {
				t.Errorf("Binary content should not have security analysis, but got: %v", safeFile.SecurityResult)
			}
		})
	}
}

func TestSecurityErrorHandling(t *testing.T) {
	ops := security.NewOperations("test-tool")

	// Test with a path that should be denied (if security is enabled)
	// This test might pass if security is disabled or path is allowed
	restrictedPath := "/etc/passwd"

	_, err := ops.SafeFileRead(restrictedPath)
	if err != nil {
		// If it's a security error, verify it has the right type
		if secErr, ok := err.(*security.SecurityError); ok {
			if secErr.GetSecurityID() == "" {
				t.Error("SecurityError should have a non-empty ID")
			}
			if secErr.Error() == "" {
				t.Error("SecurityError should have a non-empty message")
			}
		}
		// If it's not a security error, it might be a permission error, which is also fine
	}
}

func TestFilePermissions(t *testing.T) {
	ops := security.NewOperations("test-tool")

	tmpFile, err := os.CreateTemp("", "perm_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			t.Logf("Failed to remove temp file: %v", removeErr)
		}
	}()
	if closeErr := tmpFile.Close(); closeErr != nil {
		t.Fatalf("Failed to close temp file: %v", closeErr)
	}

	testContent := []byte("permission test")
	err = ops.SafeFileWrite(tmpFile.Name(), testContent)
	if err != nil {
		t.Fatalf("SafeFileWrite failed: %v", err)
	}

	// Check file permissions (0600)
	info, err := os.Stat(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	mode := info.Mode().Perm()
	expected := os.FileMode(0600)
	if mode != expected {
		t.Errorf("File permissions incorrect. Expected %o, got %o", expected, mode)
	}
}

func TestIsTextContent(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "empty content",
			content:  []byte{},
			expected: false,
		},
		{
			name:     "plain text",
			content:  []byte("Hello, World!"),
			expected: true,
		},
		{
			name:     "text with newlines",
			content:  []byte("Line 1\nLine 2\nLine 3"),
			expected: true,
		},
		{
			name:     "JSON content",
			content:  []byte(`{"key": "value", "number": 42}`),
			expected: true,
		},
		{
			name:     "binary with null bytes",
			content:  []byte{0x00, 0x01, 0x02, 0x03},
			expected: false,
		},
		{
			name:     "mixed content with null",
			content:  []byte("Hello\x00World"),
			expected: false,
		},
		{
			name:     "unicode text",
			content:  []byte("Hello 世界! 🌍"),
			expected: true,
		},
		{
			name:     "large text content",
			content:  []byte(strings.Repeat("A", 2000)),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test indirectly through file operations since isTextContent is private
			ops := security.NewOperations("test-tool")

			tmpFile, err := os.CreateTemp("", "text_test_*.bin")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer func() {
				if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
					t.Logf("Failed to remove temp file: %v", removeErr)
				}
			}()
			if closeErr := tmpFile.Close(); closeErr != nil {
				t.Fatalf("Failed to close temp file: %v", closeErr)
			}

			err = ops.SafeFileWrite(tmpFile.Name(), tt.content)
			if err != nil {
				t.Fatalf("SafeFileWrite failed: %v", err)
			}

			safeFile, err := ops.SafeFileRead(tmpFile.Name())
			if err != nil {
				t.Fatalf("SafeFileRead failed: %v", err)
			}

			// Content should always be preserved exactly
			if string(safeFile.Content) != string(tt.content) {
				t.Errorf("Content integrity violated")
			}

			// For binary content, security result should be nil
			if !tt.expected && safeFile.SecurityResult != nil {
				t.Errorf("Binary content should not have security analysis")
			}
		})
	}
}

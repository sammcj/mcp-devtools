package tools

import (
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/pdf"
)

func TestParsePageSelection(t *testing.T) {
	tool := &pdf.PDFTool{}

	tests := []struct {
		name     string
		pages    string
		maxPage  int
		expected []int
		hasError bool
	}{
		{
			name:     "all pages",
			pages:    "all",
			maxPage:  5,
			expected: []int{1, 2, 3, 4, 5},
			hasError: false,
		},
		{
			name:     "empty string defaults to all",
			pages:    "",
			maxPage:  3,
			expected: []int{1, 2, 3},
			hasError: false,
		},
		{
			name:     "single page",
			pages:    "3",
			maxPage:  5,
			expected: []int{3},
			hasError: false,
		},
		{
			name:     "page range",
			pages:    "2-4",
			maxPage:  5,
			expected: []int{2, 3, 4},
			hasError: false,
		},
		{
			name:     "multiple pages",
			pages:    "1,3,5",
			maxPage:  5,
			expected: []int{1, 3, 5},
			hasError: false,
		},
		{
			name:     "mixed range and single",
			pages:    "1,3-4,7",
			maxPage:  10,
			expected: []int{1, 3, 4, 7},
			hasError: false,
		},
		{
			name:     "page out of range",
			pages:    "10",
			maxPage:  5,
			expected: nil,
			hasError: true,
		},
		{
			name:     "invalid range",
			pages:    "5-3",
			maxPage:  5,
			expected: nil,
			hasError: true,
		},
		{
			name:     "invalid format",
			pages:    "abc",
			maxPage:  5,
			expected: nil,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.ParsePageSelection(tt.pages, tt.maxPage)

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d pages, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("expected page %d at index %d, got %d", expected, i, result[i])
				}
			}
		})
	}
}

func TestParseRequest(t *testing.T) {
	tool := &pdf.PDFTool{}

	tests := []struct {
		name     string
		args     map[string]any
		hasError bool
	}{
		{
			name: "valid minimal request",
			args: map[string]any{
				"file_path": "/absolute/path/to/file.pdf",
			},
			hasError: false,
		},
		{
			name: "valid full request",
			args: map[string]any{
				"file_path":      "/absolute/path/to/file.pdf",
				"output_dir":     "/absolute/output/dir",
				"extract_images": true,
				"pages":          "1-3",
			},
			hasError: false,
		},
		{
			name: "missing file_path",
			args: map[string]any{
				"output_dir": "/some/dir",
			},
			hasError: true,
		},
		{
			name: "relative file_path",
			args: map[string]any{
				"file_path": "relative/path/file.pdf",
			},
			hasError: true,
		},
		{
			name: "non-pdf file",
			args: map[string]any{
				"file_path": "/absolute/path/to/file.txt",
			},
			hasError: true,
		},
		{
			name: "relative output_dir",
			args: map[string]any{
				"file_path":  "/absolute/path/to/file.pdf",
				"output_dir": "relative/dir",
			},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.ParseRequest(tt.args)

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("expected result but got nil")
			}
		})
	}
}

func TestExtractTextFromPDFOperation(t *testing.T) {
	tool := &pdf.PDFTool{}

	tests := []struct {
		name      string
		operation string
		expected  []string
	}{
		{
			name:      "simple text operation",
			operation: "(Hello World) Tj",
			expected:  []string{"Hello World"},
		},
		{
			name:      "text with escaped characters",
			operation: "(Hello \\(World\\)) Tj",
			expected:  []string{"Hello (World)"},
		},
		{
			name:      "empty text",
			operation: "() Tj",
			expected:  []string{},
		},
		{
			name:      "no parentheses",
			operation: "some other operation",
			expected:  []string{},
		},
		{
			name:      "multiple parentheses pairs",
			operation: "(First) Tj (Second) Tj",
			expected:  []string{"First", "Second"},
		},
		{
			name:      "text with newlines and tabs",
			operation: "(Hello\\nWorld\\tTest) Tj",
			expected:  []string{"Hello\nWorld\tTest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.ExtractTextFromPDFOperation(tt.operation)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d results, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("expected %q at index %d, got %q", expected, i, result[i])
				}
			}
		})
	}
}

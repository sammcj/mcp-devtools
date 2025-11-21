package kagi

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestKagiProvider_GetName(t *testing.T) {
	provider := &KagiProvider{
		client: NewKagiClient("test-key"),
	}

	if provider.GetName() != "kagi" {
		t.Errorf("Expected provider name 'kagi', got '%s'", provider.GetName())
	}
}

func TestKagiProvider_GetSupportedTypes(t *testing.T) {
	provider := &KagiProvider{
		client: NewKagiClient("test-key"),
	}

	supportedTypes := provider.GetSupportedTypes()
	if len(supportedTypes) != 1 {
		t.Errorf("Expected 1 supported type, got %d", len(supportedTypes))
	}
	if supportedTypes[0] != "web" {
		t.Errorf("Expected 'web' as supported type, got '%s'", supportedTypes[0])
	}
}

func TestKagiProvider_IsAvailable_WithoutAPIKey(t *testing.T) {
	// Save original value
	originalKey := ""
	if val, exists := os.LookupEnv("KAGI_API_KEY"); exists {
		originalKey = val
		os.Unsetenv("KAGI_API_KEY")
		defer os.Setenv("KAGI_API_KEY", originalKey)
	}

	provider := NewKagiProvider()
	if provider != nil {
		t.Error("Expected nil provider when KAGI_API_KEY is not set")
	}
}

func TestKagiProvider_UnsupportedSearchType(t *testing.T) {
	provider := &KagiProvider{
		client: NewKagiClient("test-key"),
	}

	ctx := context.Background()
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	args := map[string]any{
		"query": "test query",
	}

	// Try an unsupported search type
	_, err := provider.Search(ctx, logger, "image", args)
	if err == nil {
		t.Error("Expected error for unsupported search type 'image'")
	}
	if err != nil && err.Error() != "unsupported search type for Kagi: image" {
		t.Errorf("Expected specific error message, got: %s", err.Error())
	}
}

func TestKagiProvider_InvalidCountParameter(t *testing.T) {
	provider := &KagiProvider{
		client: NewKagiClient("test-key"),
	}

	ctx := context.Background()
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	// Test with count too low
	args := map[string]any{
		"query": "test query",
		"count": float64(0),
	}

	_, err := provider.executeWebSearch(ctx, logger, args)
	if err == nil {
		t.Error("Expected error for count < 1")
	}

	// Test with count too high
	args["count"] = float64(30)
	_, err = provider.executeWebSearch(ctx, logger, args)
	if err == nil {
		t.Error("Expected error for count > 25")
	}
}

func TestDecodeHTMLEntities(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Test &amp; Example",
			expected: "Test & Example",
		},
		{
			input:    "Test &lt;tag&gt;",
			expected: "Test",
		},
		{
			input:    "Multiple  spaces",
			expected: "Multiple spaces",
		},
		{
			input:    "  Leading and trailing  ",
			expected: "Leading and trailing",
		},
	}

	for _, tt := range tests {
		result := decodeHTMLEntities(tt.input)
		if result != tt.expected {
			t.Errorf("decodeHTMLEntities(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

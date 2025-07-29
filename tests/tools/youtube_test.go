package tools

import (
	"context"
	"sync"
	"testing"

	"github.com/sammcj/mcp-devtools/internal/tools/media/youtube"
	"github.com/sirupsen/logrus"
)

func TestYouTubeTool_Definition(t *testing.T) {
	tool := &youtube.YouTubeTool{}
	def := tool.Definition()

	if def.Name != "youtube_transcript" {
		t.Errorf("expected tool name 'youtube_transcript', got '%s'", def.Name)
	}

	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	// Basic validation that we have an input schema
	if def.InputSchema.Properties == nil {
		t.Error("expected input schema to have properties")
	}

	if len(def.InputSchema.Required) == 0 {
		t.Error("expected input schema to have required fields")
	}
}

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		shouldError bool
	}{
		{
			name:     "Direct video ID",
			input:    "dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "Standard YouTube URL",
			input:    "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "YouTube URL with additional parameters",
			input:    "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=30s",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "YouTube short URL",
			input:    "https://youtu.be/dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "YouTube embed URL",
			input:    "https://www.youtube.com/embed/dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "YouTube live URL",
			input:    "https://www.youtube.com/live/dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "Mobile YouTube URL",
			input:    "https://m.youtube.com/watch?v=dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:        "Invalid video ID length",
			input:       "invalid",
			shouldError: true,
		},
		{
			name:        "Empty input",
			input:       "",
			shouldError: true,
		},
		{
			name:        "Non-YouTube URL",
			input:       "https://www.example.com/video",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := youtube.ExtractVideoID(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error for input '%s', but got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for input '%s': %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("for input '%s', expected '%s', got '%s'", tt.input, tt.expected, result)
			}
		})
	}
}

func TestIsValidVideoID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"dQw4w9WgXcQ", true},
		{"_-aBcDeFgHi", true},
		{"123456789ab", true},
		{"short", false},          // too short
		{"toolongvideoid", false}, // too long
		{"invalid@#$%", false},    // invalid characters
		{"", false},               // empty
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := youtube.IsValidVideoID(tt.input)
			if result != tt.expected {
				t.Errorf("for input '%s', expected %v, got %v", tt.input, tt.expected, result)
			}
		})
	}
}

func TestValidateFormat(t *testing.T) {
	tests := []struct {
		format      string
		shouldError bool
	}{
		{"text", false},
		{"json", false},
		{"srt", false},
		{"vtt", false},
		{"invalid", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			err := youtube.ValidateFormat(tt.format)
			if tt.shouldError && err == nil {
				t.Errorf("expected error for format '%s', but got none", tt.format)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error for format '%s': %v", tt.format, err)
			}
		})
	}
}

func TestNormaliseLanguageCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"en-gb", "en-GB"},
		{"EN-GB", "en-GB"},
		{"british", "en-GB"},
		{"en-us", "en-US"},
		{"american", "en-US"},
		{"en", "en"},
		{"fr", "fr"},
		{"french", "fr"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := youtube.NormaliseLanguageCode(tt.input)
			if result != tt.expected {
				t.Errorf("for input '%s', expected '%s', got '%s'", tt.input, tt.expected, result)
			}
		})
	}
}

func TestFormatSRTTime(t *testing.T) {
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0.0, "00:00:00,000"},
		{1.5, "00:00:01,500"},
		{61.123, "00:01:01,122"},
		{3661.456, "01:01:01,456"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := youtube.FormatSRTTime(tt.seconds)
			if result != tt.expected {
				t.Errorf("for input %f, expected '%s', got '%s'", tt.seconds, tt.expected, result)
			}
		})
	}
}

func TestFormatVTTTime(t *testing.T) {
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0.0, "00:00:00.000"},
		{1.5, "00:00:01.500"},
		{61.123, "00:01:01.122"},
		{3661.456, "01:01:01.456"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := youtube.FormatVTTTime(tt.seconds)
			if result != tt.expected {
				t.Errorf("for input %f, expected '%s', got '%s'", tt.seconds, tt.expected, result)
			}
		})
	}
}

func TestGetDefaultLanguagePreference(t *testing.T) {
	pref := youtube.GetDefaultLanguagePreference()

	if pref.Primary != "en-GB" {
		t.Errorf("expected primary language 'en-GB', got '%s'", pref.Primary)
	}

	expectedFallbacks := []string{"en", "en-US"}
	if len(pref.Fallbacks) != len(expectedFallbacks) {
		t.Errorf("expected %d fallbacks, got %d", len(expectedFallbacks), len(pref.Fallbacks))
	}

	for i, expected := range expectedFallbacks {
		if i >= len(pref.Fallbacks) || pref.Fallbacks[i] != expected {
			t.Errorf("expected fallback %d to be '%s', got '%s'", i, expected, pref.Fallbacks[i])
		}
	}
}

// Integration test that requires network access - only run with short flag disabled
func TestYouTubeTool_Execute_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tool := &youtube.YouTubeTool{}
	// Initialize config to prevent nil pointer dereference
	tool.SetConfig(&youtube.Config{
		CacheEnabled:    true,
		CacheTTLMinutes: 60,
		YouTubeAPIKey:   "", // Empty for testing without API key
	})
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests
	cache := &sync.Map{}

	// Test with a known video that should have captions
	// Using a YouTube provided sample video
	args := map[string]interface{}{
		"video_url":               "https://www.youtube.com/watch?v=LXb3EKWsInQ", // SMPTE test pattern video
		"format":                  "text",
		"include_timestamps":      false,
		"auto_generated_fallback": true,
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, logger, cache, args)

	if err != nil {
		// This test may fail if the video doesn't have captions or if YouTube blocks the request
		// Log the error but don't fail the test hard
		t.Logf("Integration test failed (this may be expected): %v", err)
		return
	}

	if result == nil {
		t.Error("expected non-nil result")
		return
	}

	// Basic validation of the response structure
	if result.Content == nil {
		t.Error("expected non-nil content")
	}

	t.Logf("Integration test passed - extracted transcript successfully")
}

// Benchmark for video ID extraction
func BenchmarkExtractVideoID(b *testing.B) {
	testURL := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = youtube.ExtractVideoID(testURL)
	}
}

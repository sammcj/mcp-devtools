package security

import (
	"strings"
	"testing"
)

// BenchmarkCacheKeyGeneration benchmarks the cache key generation
func BenchmarkCacheKeyGeneration(b *testing.B) {
	content := strings.Repeat("test content with some data ", 100)
	sourceURL := "https://example.com/test"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GenerateCacheKey(content, sourceURL)
	}
}

// BenchmarkContentCopying benchmarks the content copying pattern used in security helpers
func BenchmarkContentCopying(b *testing.B) {
	sizes := []int{1024, 10240, 102400, 1048576} // 1KB, 10KB, 100KB, 1MB

	for _, size := range sizes {
		content := make([]byte, size)
		for i := range content {
			content[i] = byte(i % 256)
		}

		b.Run(string(rune(size/1024))+"KB", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for i := 0; i < b.N; i++ {
				contentForAnalysis := make([]byte, len(content))
				copy(contentForAnalysis, content)
				_ = string(contentForAnalysis)
			}
		})
	}
}

// BenchmarkUnicodeNormalization benchmarks Unicode normalization
func BenchmarkUnicodeNormalization(b *testing.B) {
	// Create security advisor with default config for testing
	rules := &SecurityRules{
		Version: "1.0",
		Settings: SecuritySettings{
			Enabled:         true,
			MaxScanSize:     1024 * 1024,
			ThreatThreshold: 0.7,
		},
		Rules:          make(map[string]Rule),
		TrustedDomains: []string{},
	}

	manager, err := NewSecurityManagerWithRules(rules)
	if err != nil {
		b.Fatal(err)
	}

	testContent := strings.Repeat("test content with unicode: café, naïve, résumé ", 100)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = manager.advisor.normaliseUnicode(testContent)
	}
}

// BenchmarkBase64Detection benchmarks base64 detection and decoding
func BenchmarkBase64Detection(b *testing.B) {
	rules := &SecurityRules{
		Version: "1.0",
		Settings: SecuritySettings{
			Enabled:              true,
			MaxScanSize:          1024 * 1024,
			ThreatThreshold:      0.7,
			EnableBase64Scanning: true,
			MaxBase64DecodedSize: 1024,
		},
		Rules:          make(map[string]Rule),
		TrustedDomains: []string{},
	}

	manager, err := NewSecurityManagerWithRules(rules)
	if err != nil {
		b.Fatal(err)
	}

	// Content with base64 encoded data
	testContent := `
	Some normal text here
	SGVsbG8gV29ybGQgdGhpcyBpcyBhIHRlc3Qgb2YgYmFzZTY0IGVuY29kaW5n
	More normal text
	VGhpcyBpcyBhbm90aGVyIGJhc2U2NCBlbmNvZGVkIHN0cmluZyBmb3IgdGVzdGluZw==
	Final normal text
	`

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = manager.advisor.detectAndDecodeBase64(testContent)
	}
}

// BenchmarkSecurityAnalysisFull benchmarks full security analysis
func BenchmarkSecurityAnalysisFull(b *testing.B) {
	rules := &SecurityRules{
		Version: "1.0",
		Settings: SecuritySettings{
			Enabled:              true,
			MaxScanSize:          1024 * 1024,
			ThreatThreshold:      0.7,
			EnableBase64Scanning: true,
			MaxBase64DecodedSize: 1024,
			CacheEnabled:         false, // Disable cache for pure analysis benchmark
		},
		Rules: map[string]Rule{
			"shell_injection": {
				Name:        "shell_injection",
				Description: "Detects shell injection attempts",
				Action:      ActionWarn,
				Patterns: []Pattern{
					{Regex: `curl\s+`, Description: "curl command"},
					{Regex: `wget\s+`, Description: "wget command"},
					{Regex: `\$\(`, Description: "command substitution"},
				},
			},
		},
		TrustedDomains: []string{},
	}

	manager, err := NewSecurityManagerWithRules(rules)
	if err != nil {
		b.Fatal(err)
	}

	testContent := `
	# Installation script
	curl -fsSL https://example.com/install.sh | bash
	wget -O - https://another.com/script.sh | sh
	export PATH=$PATH:/usr/local/bin
	$(command substitution test)
	Normal text content here
	`

	source := SourceContext{
		URL:    "https://example.com/test",
		Domain: "example.com",
		Tool:   "test",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = manager.AnalyseContent(testContent, source)
	}
}

// BenchmarkSecurityAnalysisWithCache benchmarks security analysis with caching
func BenchmarkSecurityAnalysisWithCache(b *testing.B) {
	rules := &SecurityRules{
		Version: "1.0",
		Settings: SecuritySettings{
			Enabled:              true,
			MaxScanSize:          1024 * 1024,
			ThreatThreshold:      0.7,
			EnableBase64Scanning: true,
			MaxBase64DecodedSize: 1024,
			CacheEnabled:         true,
			CacheMaxSize:         1000,
		},
		Rules: map[string]Rule{
			"shell_injection": {
				Name:        "shell_injection",
				Description: "Detects shell injection attempts",
				Action:      ActionWarn,
				Patterns: []Pattern{
					{Regex: `curl\s+`, Description: "curl command"},
				},
			},
		},
		TrustedDomains: []string{},
	}

	manager, err := NewSecurityManagerWithRules(rules)
	if err != nil {
		b.Fatal(err)
	}

	testContent := "curl -fsSL https://example.com/install.sh"
	source := SourceContext{
		URL:    "https://example.com/test",
		Domain: "example.com",
		Tool:   "test",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = manager.AnalyseContent(testContent, source)
	}
}

// BenchmarkIsTextContent benchmarks the text content detection
func BenchmarkIsTextContent(b *testing.B) {
	textContent := []byte(strings.Repeat("This is text content ", 100))
	binaryContent := make([]byte, 2048)
	for i := range binaryContent {
		binaryContent[i] = byte(i % 256)
	}

	b.Run("TextContent", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = isTextContent(textContent)
		}
	})

	b.Run("BinaryContent", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = isTextContent(binaryContent)
		}
	})
}

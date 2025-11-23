package registry

import (
	"strings"
	"testing"
)

// BenchmarkToolNameNormalization benchmarks tool name normalization
func BenchmarkToolNameNormalization(b *testing.B) {
	toolNames := []string{
		"calculator",
		"fetch_url",
		"internet_search",
		"get_library_documentation",
		"sequential_thinking",
		"think",
	}

	b.Run("Current", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, name := range toolNames {
				_ = strings.ToLower(strings.ReplaceAll(name, "_", "-"))
			}
		}
	})

	// Benchmark pre-normalized approach
	normalizedMap := make(map[string]string)
	for _, name := range toolNames {
		normalizedMap[name] = strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	}

	b.Run("PreNormalized", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, name := range toolNames {
				_ = normalizedMap[name]
			}
		}
	})
}

// BenchmarkShouldRegisterTool benchmarks the tool registration check
func BenchmarkShouldRegisterTool(b *testing.B) {
	// Initialize registry for testing
	Init(nil)

	toolNames := []string{
		"calculator",
		"fetch_url",
		"internet_search",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, name := range toolNames {
			_ = ShouldRegisterTool(name)
		}
	}
}

// BenchmarkParseDisabledTools benchmarks the parsing of disabled tools
func BenchmarkParseDisabledTools(b *testing.B) {
	disabledToolsEnv := "tool1,tool2,tool3,tool4,tool5,tool6,tool7,tool8,tool9,tool10"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tools := strings.SplitSeq(disabledToolsEnv, ",")
		count := 0
		for tool := range tools {
			_ = strings.TrimSpace(tool)
			count++
		}
	}
}

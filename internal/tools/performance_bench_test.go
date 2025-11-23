package tools

import (
	"context"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
)

// BenchmarkToolExecution benchmarks common tool execution patterns
// This helps identify overhead in the tool execution pipeline

// Mock tool for benchmarking
type mockTool struct{}

func (m *mockTool) Definition() interface{} {
	return nil
}

func (m *mockTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (interface{}, error) {
	// Simulate minimal work
	return map[string]interface{}{"result": "success"}, nil
}

// BenchmarkToolExecutionOverhead measures the overhead of tool execution
func BenchmarkToolExecutionOverhead(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}
	ctx := context.Background()
	tool := &mockTool{}
	args := map[string]interface{}{
		"param1": "value1",
		"param2": 42,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = tool.Execute(ctx, logger, cache, args)
	}
}

// BenchmarkCacheOperations benchmarks sync.Map operations
func BenchmarkCacheOperations(b *testing.B) {
	cache := &sync.Map{}

	b.Run("Store", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			cache.Store("key", "value")
		}
	})

	b.Run("Load", func(b *testing.B) {
		cache.Store("key", "value")
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = cache.Load("key")
		}
	})

	b.Run("LoadOrStore", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			cache.LoadOrStore("key", "value")
		}
	})
}

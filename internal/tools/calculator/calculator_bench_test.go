package calculator

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
)

// BenchmarkCalculatorExecution benchmarks calculator tool execution
func BenchmarkCalculatorExecution(b *testing.B) {
	calc := &Calculator{}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cache := &sync.Map{}
	ctx := context.Background()

	b.Run("SingleExpression", func(b *testing.B) {
		args := map[string]interface{}{
			"expression": "2 + 3 * 4",
		}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = calc.Execute(ctx, logger, cache, args)
		}
	})

	b.Run("ComplexExpression", func(b *testing.B) {
		args := map[string]interface{}{
			"expression": "(100 + 50) * 0.15 + 25 / 5 - 10",
		}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = calc.Execute(ctx, logger, cache, args)
		}
	})

	b.Run("BatchExpressions", func(b *testing.B) {
		args := map[string]interface{}{
			"expressions": []interface{}{
				"2 + 3",
				"10 * 5",
				"(15 - 3) / 4",
				"2^8",
			},
		}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = calc.Execute(ctx, logger, cache, args)
		}
	})
}

// BenchmarkJSONMarshal benchmarks JSON marshaling approaches
func BenchmarkJSONMarshal(b *testing.B) {
	data := map[string]interface{}{
		"expression": "2 + 3 * 4",
		"result":     14,
	}

	b.Run("MarshalIndent", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = json.MarshalIndent(data, "", "  ")
		}
	})

	b.Run("Marshal", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(data)
		}
	})
}

// BenchmarkExpressionParsing benchmarks expression parsing
func BenchmarkExpressionParsing(b *testing.B) {
	calc := &Calculator{}

	expressions := []string{
		"2 + 3",
		"2 + 3 * 4",
		"(10 + 5) / 3",
		"2^8 - 1",
		"((100 - 20) * 1.5) + (50 / 2)",
	}

	for _, expr := range expressions {
		b.Run(expr, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = calc.evaluateExpression(expr)
			}
		})
	}
}

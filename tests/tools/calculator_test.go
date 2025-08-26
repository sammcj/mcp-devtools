package tools_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/tools/calculator"
	"github.com/sammcj/mcp-devtools/tests/testutils"
)

// extractCalculatorResult extracts the result from calculator tool response
func extractCalculatorResult(result *mcp.CallToolResult) (any, error) {
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("no content in result")
	}

	textContent, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		return nil, fmt.Errorf("expected text content")
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return response["result"], nil
}

func TestCalculator_Definition(t *testing.T) {
	tool := &calculator.Calculator{}
	definition := tool.Definition()

	// Test basic definition properties
	testutils.AssertEqual(t, "calculator", definition.Name)
	testutils.AssertNotNil(t, definition.Description)

	// Test that description contains key phrases
	desc := definition.Description
	if !testutils.Contains(desc, "arithmetic") {
		t.Errorf("Expected description to contain 'arithmetic', got: %s", desc)
	}

	// Test input schema exists
	testutils.AssertNotNil(t, definition.InputSchema)
}

func TestCalculator_Execute_BasicArithmetic(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tests := []struct {
		expression string
		expected   any
	}{
		{"2 + 3", float64(5)},
		{"10 - 4", float64(6)},
		{"6 * 7", float64(42)},
		{"15 / 3", float64(5)},
		{"17 % 5", float64(2)},
		{"2.5 + 1.5", 4.0},
		{"10.0 / 4.0", 2.5},
	}

	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			args := map[string]any{
				"expression": test.expression,
			}

			result, err := tool.Execute(ctx, logger, cache, args)

			testutils.AssertNoError(t, err)
			testutils.AssertNotNil(t, result)

			// Extract result from JSON response
			actualResult, err := extractCalculatorResult(result)
			testutils.AssertNoError(t, err)
			testutils.AssertEqual(t, test.expected, actualResult)
		})
	}
}

func TestCalculator_Execute_OrderOfOperations(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tests := []struct {
		expression string
		expected   any
	}{
		{"2 + 3 * 4", float64(14)},     // Should be 2 + 12 = 14, not (2 + 3) * 4 = 20
		{"20 - 2 * 5", float64(10)},    // Should be 20 - 10 = 10, not (20 - 2) * 5 = 90
		{"2 * 3 + 4", float64(10)},     // Should be 6 + 4 = 10
		{"15 / 3 + 2", float64(7)},     // Should be 5 + 2 = 7
		{"2 + 3 * 4 - 1", float64(13)}, // Should be 2 + 12 - 1 = 13
		{"8 / 2 / 2", float64(2)},      // Should be (8 / 2) / 2 = 2 (left-to-right)
		{"2 * 3 * 4", float64(24)},     // Should be ((2 * 3) * 4) = 24
	}

	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			args := map[string]any{
				"expression": test.expression,
			}

			result, err := tool.Execute(ctx, logger, cache, args)

			testutils.AssertNoError(t, err)
			testutils.AssertNotNil(t, result)

			// Extract result from JSON response
			actualResult, err := extractCalculatorResult(result)
			testutils.AssertNoError(t, err)
			testutils.AssertEqual(t, test.expected, actualResult)
		})
	}
}

func TestCalculator_Execute_Parentheses(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tests := []struct {
		expression string
		expected   any
	}{
		{"(2 + 3) * 4", float64(20)},       // Should be 5 * 4 = 20
		{"2 * (3 + 4)", float64(14)},       // Should be 2 * 7 = 14
		{"(10 + 5) / 3", float64(5)},       // Should be 15 / 3 = 5
		{"((2 + 3) * 4)", float64(20)},     // Nested parentheses
		{"2 * (3 + 4) - 1", float64(13)},   // Should be 2 * 7 - 1 = 13
		{"(2 + 3) * (4 - 1)", float64(15)}, // Should be 5 * 3 = 15
	}

	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			args := map[string]any{
				"expression": test.expression,
			}

			result, err := tool.Execute(ctx, logger, cache, args)

			testutils.AssertNoError(t, err)
			testutils.AssertNotNil(t, result)

			// Extract result from JSON response
			actualResult, err := extractCalculatorResult(result)
			testutils.AssertNoError(t, err)
			testutils.AssertEqual(t, test.expected, actualResult)
		})
	}
}

func TestCalculator_Execute_UnaryOperators(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tests := []struct {
		expression string
		expected   any
	}{
		{"-5", float64(-5)},
		{"+5", float64(5)},
		{"-5 + 3", float64(-2)},
		{"-(2 + 3)", float64(-5)},
		{"-(2 * 3) + 10", float64(4)},
		{"5 + (-3)", float64(2)},
	}

	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			args := map[string]any{
				"expression": test.expression,
			}

			result, err := tool.Execute(ctx, logger, cache, args)

			testutils.AssertNoError(t, err)
			testutils.AssertNotNil(t, result)

			// Extract result from JSON response
			actualResult, err := extractCalculatorResult(result)
			testutils.AssertNoError(t, err)
			testutils.AssertEqual(t, test.expected, actualResult)
		})
	}
}

func TestCalculator_Execute_ErrorHandling(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tests := []struct {
		expression  string
		expectedErr string
	}{
		{"", "expression cannot be empty"},
		{"5 / 0", "division by zero"},
		{"5 % 0", "modulo by zero"},
		{"2 +", "unexpected character"},
		{"2 + * 3", "unexpected character"},
		{"(2 + 3", "expected closing parenthesis"},
		{"2 + 3)", "unexpected characters after expression"},
		{"2 ** 3", "unexpected character '*'"},
		{"2 + abc", "unexpected character 'a'"},
		{"2.5.5", "unexpected characters after expression"},
	}

	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			args := map[string]any{
				"expression": test.expression,
			}

			_, err := tool.Execute(ctx, logger, cache, args)

			testutils.AssertError(t, err)
			testutils.AssertErrorContains(t, err, test.expectedErr)
		})
	}
}

func TestCalculator_Execute_WhitespaceHandling(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tests := []struct {
		expression string
		expected   any
	}{
		{" 2 + 3 ", float64(5)},
		{"2+3", float64(5)},
		{" ( 2 + 3 ) * 4 ", float64(20)},
		{"2  *  3", float64(6)},
		{"\t2\n+\r3\t", float64(5)},
	}

	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			args := map[string]any{
				"expression": test.expression,
			}

			result, err := tool.Execute(ctx, logger, cache, args)

			testutils.AssertNoError(t, err)
			testutils.AssertNotNil(t, result)

			// Extract result from JSON response
			actualResult, err := extractCalculatorResult(result)
			testutils.AssertNoError(t, err)
			testutils.AssertEqual(t, test.expected, actualResult)
		})
	}
}

func TestCalculator_Execute_MissingParameter(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "either 'expression' or 'expressions' parameter is required")
}

func TestCalculator_Execute_InvalidParameterType(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"expression": 123, // Invalid type
	}

	_, err := tool.Execute(ctx, logger, cache, args)

	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "expression must be a string")
}

func TestCalculator_Execute_ArrayMode(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	// Test array of expressions
	args := map[string]any{
		"expressions": []any{
			"2 + 3",
			"10 * 5",
			"(15 - 3) / 4",
			"2.5 + 1.5",
		},
	}

	result, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertNoError(t, err)
	testutils.AssertNotNil(t, result)

	// Extract results from JSON response
	textContent, ok := mcp.AsTextContent(result.Content[0])
	testutils.AssertNotNil(t, textContent)
	if !ok {
		t.Fatal("Expected text content")
	}

	var response map[string]any
	err = json.Unmarshal([]byte(textContent.Text), &response)
	testutils.AssertNoError(t, err)

	results, ok := response["results"].([]any)
	if !ok {
		t.Fatal("Expected results array")
	}

	testutils.AssertEqual(t, 4, len(results))

	// Check each result
	expectedResults := []float64{5, 50, 3, 4}
	for i, expectedResult := range expectedResults {
		resultItem, ok := results[i].(map[string]any)
		if !ok {
			t.Fatalf("Expected result %d to be a map", i)
		}

		actualResult := resultItem["result"]
		testutils.AssertEqual(t, expectedResult, actualResult)
	}
}

func TestCalculator_Execute_ArrayMode_EmptyArray(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"expressions": []any{},
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "expressions array cannot be empty")
}

func TestCalculator_Execute_ArrayMode_InvalidType(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"expressions": []any{
			"2 + 3",
			123, // Invalid type
		},
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "expression at index 1 must be a string")
}

func TestCalculator_Execute_ArrayMode_WithError(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{
		"expressions": []any{
			"2 + 3",
			"5 / 0", // This will cause an error
		},
	}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "error in expression 1")
	testutils.AssertErrorContains(t, err, "division by zero")
}

func TestCalculator_Execute_NoParameters(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	args := map[string]any{}

	_, err := tool.Execute(ctx, logger, cache, args)
	testutils.AssertError(t, err)
	testutils.AssertErrorContains(t, err, "either 'expression' or 'expressions' parameter is required")
}

func TestCalculator_Execute_ComplexExpressions(t *testing.T) {
	tool := &calculator.Calculator{}
	logger := testutils.CreateTestLogger()
	cache := testutils.CreateTestCache()
	ctx := testutils.CreateTestContext()

	tests := []struct {
		expression string
		expected   any
	}{
		{"((2 + 3) * 4 - 1) / (5 + 2)", float64(19.0 / 7.0)}, // (20 - 1) / 7 = 19/7 ≈ 2.714...
		{"2 + 3 * 4 - 5 / 2 + 1", float64(12.5)},             // 2 + 12 - 2.5 + 1 = 12.5
		{"-((2 + 3) * 4)", float64(-20)},                     // -(5 * 4) = -20
		{"5 * -3 + 10", float64(-5)},                         // -15 + 10 = -5
	}

	for _, test := range tests {
		t.Run(test.expression, func(t *testing.T) {
			args := map[string]any{
				"expression": test.expression,
			}

			result, err := tool.Execute(ctx, logger, cache, args)

			testutils.AssertNoError(t, err)
			testutils.AssertNotNil(t, result)

			// Extract result from JSON response
			actualResult, err := extractCalculatorResult(result)
			testutils.AssertNoError(t, err)
			testutils.AssertEqual(t, test.expected, actualResult)
		})
	}
}

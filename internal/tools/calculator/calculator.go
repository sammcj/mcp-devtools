package calculator

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sammcj/mcp-devtools/internal/tools"
	"github.com/sirupsen/logrus"
)

// Calculator implements the tools.Tool interface for basic arithmetic calculations
type Calculator struct{}

// init registers the tool with the registry
func init() {
	registry.Register(&Calculator{})
}

// Definition returns the tool's definition for MCP registration
func (c *Calculator) Definition() mcp.Tool {
	return mcp.NewTool(
		"calculator",
		mcp.WithDescription("Calculator. Use when performing arithmetic to ensure accuracy. Supports +, -, *, /, %, parentheses, and decimal numbers."),
		mcp.WithString("expression",
			mcp.Description("Single mathematical expression to evaluate (e.g., '2 + 3 * 4', '(10 + 5) / 3', '12.5 * 2')"),
		),
		mcp.WithArray("expressions",
			mcp.Description("Array of mathematical expressions to evaluate"),
			mcp.Items(map[string]any{
				"type": "string",
			}),
		),
	)
}

// Execute executes the calculator's logic
func (c *Calculator) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
	logger.Info("Executing calculator")

	// Check if we have a single expression or an array of expressions
	if expressionRaw, ok := args["expression"]; ok {
		// Single expression mode
		expression, ok := expressionRaw.(string)
		if !ok {
			return nil, fmt.Errorf("expression must be a string")
		}

		result, err := c.evaluateExpression(expression)
		if err != nil {
			return nil, err
		}

		response := map[string]any{
			"expression": expression,
			"result":     result,
		}

		return c.newToolResultJSON(response)

	} else if expressionsRaw, ok := args["expressions"]; ok {
		// Array mode
		expressions, ok := expressionsRaw.([]any)
		if !ok {
			return nil, fmt.Errorf("expressions must be an array")
		}

		if len(expressions) == 0 {
			return nil, fmt.Errorf("expressions array cannot be empty")
		}

		results := make([]map[string]any, 0, len(expressions))

		for i, exprRaw := range expressions {
			expression, ok := exprRaw.(string)
			if !ok {
				return nil, fmt.Errorf("expression at index %d must be a string", i)
			}

			result, err := c.evaluateExpression(expression)
			if err != nil {
				return nil, fmt.Errorf("error in expression %d: %w", i, err)
			}

			results = append(results, map[string]any{
				"expression": expression,
				"result":     result,
			})
		}

		response := map[string]any{
			"results": results,
		}

		return c.newToolResultJSON(response)

	} else {
		return nil, fmt.Errorf("either 'expression' or 'expressions' parameter is required")
	}
}

// evaluateExpression evaluates a single mathematical expression
func (c *Calculator) evaluateExpression(expression string) (any, error) {
	// Clean the expression
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, fmt.Errorf("expression cannot be empty")
	}

	// Parse and evaluate the expression
	parser := newParser(expression)
	result, err := parser.parseExpression()
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate expression '%s': %w", expression, err)
	}

	// Check if we consumed all tokens
	if !parser.isAtEnd() {
		return nil, fmt.Errorf("unexpected characters after expression: %s", parser.remaining())
	}

	// Format result appropriately
	if result == float64(int64(result)) {
		// If it's a whole number, return as int
		return int64(result), nil
	} else {
		return result, nil
	}
}

// Parser represents an expression parser
type parser struct {
	expression string
	position   int
	current    rune
}

// newParser creates a new expression parser
func newParser(expression string) *parser {
	p := &parser{
		expression: expression,
		position:   0,
	}
	p.advance() // Initialize current character
	return p
}

// advance moves to the next character
func (p *parser) advance() {
	if p.position >= len(p.expression) {
		p.current = 0 // EOF
		return
	}
	p.current = rune(p.expression[p.position])
	p.position++
}

// isAtEnd checks if we're at the end of the expression
func (p *parser) isAtEnd() bool {
	return p.current == 0
}

// remaining returns the remaining unparsed part of the expression
func (p *parser) remaining() string {
	if p.position-1 >= len(p.expression) {
		return ""
	}
	return p.expression[p.position-1:]
}

// skipWhitespace skips whitespace characters
func (p *parser) skipWhitespace() {
	for unicode.IsSpace(p.current) {
		p.advance()
	}
}

// parseNumber parses a number (integer or float)
func (p *parser) parseNumber() (float64, error) {
	start := p.position - 1

	// Parse digits before decimal point
	for unicode.IsDigit(p.current) {
		p.advance()
	}

	// Handle decimal point
	if p.current == '.' {
		p.advance()
		if !unicode.IsDigit(p.current) {
			return 0, fmt.Errorf("expected digit after decimal point")
		}
		for unicode.IsDigit(p.current) {
			p.advance()
		}
	}

	end := p.position - 1
	if p.current == 0 {
		end = len(p.expression)
	}

	numStr := p.expression[start:end]
	return strconv.ParseFloat(numStr, 64)
}

// parseExpression parses an expression (handles + and -)
func (p *parser) parseExpression() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}

	for {
		p.skipWhitespace()
		switch p.current {
		case '+':
			p.advance()
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			left += right
		case '-':
			p.advance()
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			left -= right
		default:
			return left, nil
		}
	}
}

// parseTerm parses a term (handles *, /, %)
func (p *parser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}

	for {
		p.skipWhitespace()
		switch p.current {
		case '*':
			p.advance()
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			left *= right
		case '/':
			p.advance()
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		case '%':
			p.advance()
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			// Go's modulo operator behaviour for floats
			left = float64(int64(left) % int64(right))
		default:
			return left, nil
		}
	}
}

// parseFactor parses a factor (handles numbers and parentheses)
func (p *parser) parseFactor() (float64, error) {
	p.skipWhitespace()

	// Handle parentheses
	if p.current == '(' {
		p.advance()
		result, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipWhitespace()
		if p.current != ')' {
			return 0, fmt.Errorf("expected closing parenthesis")
		}
		p.advance()
		return result, nil
	}

	// Handle unary minus
	if p.current == '-' {
		p.advance()
		factor, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		return -factor, nil
	}

	// Handle unary plus
	if p.current == '+' {
		p.advance()
		return p.parseFactor()
	}

	// Handle numbers
	if unicode.IsDigit(p.current) || p.current == '.' {
		return p.parseNumber()
	}

	return 0, fmt.Errorf("unexpected character '%c'", p.current)
}

// newToolResultJSON creates a new tool result with JSON content
func (c *Calculator) newToolResultJSON(data any) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// ProvideExtendedInfo implements the ExtendedHelpProvider interface for the Calculator tool
func (c *Calculator) ProvideExtendedInfo() *tools.ExtendedHelp {
	return &tools.ExtendedHelp{
		WhenToUse:    "Use when performing arithmetic calculations to ensure accuracy, when complex expressions need proper order of operations, when handling multiple related calculations in batch, or when decimal precision is important for financial calculations.",
		WhenNotToUse: "Don't use for scientific functions (sqrt, sin, cos, log, etc.), bitwise operations or boolean logic, statistical calculations or data analysis, or advanced mathematical operations like matrices or calculus.",
		CommonPatterns: []string{
			"Basic arithmetic: \"2 + 3 * 4\" (follows order of operations)",
			"Parentheses for grouping: \"(10 + 5) * 2\"",
			"Decimal calculations: \"12.50 * 1.08\" (tax calculations)",
			"Array mode for batch: {\"expressions\": [\"100 * 0.15\", \"200 * 0.15\"]}",
			"Unary operators: \"-5 + 10\" or \"-(2 + 3)\"",
			"Use parentheses to make complex expressions clear",
			"Test complex calculations by breaking them into smaller parts",
			"Be aware of precision limitations with decimal numbers",
			"Handle errors gracefully when expressions might be invalid",
			"Use array mode for batch calculations to improve efficiency",
			"Group related calculations in arrays when performing multiple operations",
		},
		ParameterDetails: map[string]string{
			"expression":  "Single mathematical expression to evaluate (string). Supports +, -, *, /, % operators with standard precedence, parentheses for grouping, and unary +/- operators.",
			"expressions": "Array of mathematical expressions for batch processing (array of strings). Each expression follows same rules as single expression parameter.",
		},
		Examples: []tools.ToolExample{
			{
				Description:    "Basic percentage calculation",
				Arguments:      map[string]any{"expression": "150 * 0.20"},
				ExpectedResult: `{"expression": "150 * 0.20", "result": 30}`,
			},
			{
				Description:    "Complex business calculation with proper order",
				Arguments:      map[string]any{"expression": "(1200 - 200) * 0.15 + 50"},
				ExpectedResult: `{"expression": "(1200 - 200) * 0.15 + 50", "result": 200}`,
			},
			{
				Description: "Batch calculations for efficiency",
				Arguments: map[string]any{
					"expressions": []string{"2 + 3", "10 * 5", "(15 - 3) / 4"},
				},
				ExpectedResult: `{"results": [{"expression": "2 + 3", "result": 5}, {"expression": "10 * 5", "result": 50}, {"expression": "(15 - 3) / 4", "result": 3}]}`,
			},
		},
		Troubleshooting: []tools.TroubleshootingTip{
			{
				Problem:  "Getting 'division by zero' error",
				Solution: "Check expressions for division or modulo by zero (e.g., '5/0' or '5%0')",
			},
			{
				Problem:  "Unexpected results with order of operations",
				Solution: "Use parentheses to make precedence explicit: '2 + 3 * 4' = 14, but '(2 + 3) * 4' = 20",
			},
			{
				Problem:  "Syntax errors with unexpected characters",
				Solution: "Ensure only supported operators (+, -, *, /, %, parentheses) and valid decimal numbers",
			},
			{
				Problem:  "Array mode returns error for single calculation",
				Solution: "Use 'expression' parameter for single calculations, 'expressions' array for multiple",
			},
		},
	}
}

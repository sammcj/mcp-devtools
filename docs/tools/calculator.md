# Calculator Tool

The Calculator tool provides basic arithmetic computation capabilities for AI agents that need to perform mathematical calculations accurately.

## Purpose

AI agents can use this tool when they need to:
- Perform basic arithmetic operations
- Calculate complex expressions with proper order of operations
- Ensure mathematical accuracy in their computations
- Handle both integer and decimal number calculations

## Usage

### Basic Operations

```json
{
  "name": "calculator",
  "arguments": {
    "expression": "2 + 3"
  }
}
```

**Response:**
```json
{
  "expression": "2 + 3",
  "result": 5
}
```

### Supported Operations

- **Addition**: `+`
- **Subtraction**: `-`
- **Multiplication**: `*`
- **Division**: `/`
- **Modulo**: `%`
- **Exponentiation**: `^`
- **Parentheses**: `()` for grouping

### Order of Operations

The calculator follows standard mathematical order of operations (PEMDAS/BODMAS):

1. **P**arentheses/Brackets first
2. **E**xponents (right to left)
3. **M**ultiplication and **D**ivision (left to right)
4. **A**ddition and **S**ubtraction (left to right)

```json
{
  "name": "calculator",
  "arguments": {
    "expression": "2 + 3 * 4"
  }
}
```

Returns `14` (not `20`) because multiplication is performed before addition.

### Complex Expressions

```json
{
  "name": "calculator",
  "arguments": {
    "expression": "((10 + 5) * 2 - 3) / 9"
  }
}
```

**Response:**
```json
{
  "expression": "((10 + 5) * 2 - 3) / 9",
  "result": 3
}
```

### Decimal Numbers

```json
{
  "name": "calculator",
  "arguments": {
    "expression": "3.14 * 2.5"
  }
}
```

**Response:**
```json
{
  "expression": "3.14 * 2.5",
  "result": 7.85
}
```

### Unary Operators

```json
{
  "name": "calculator",
  "arguments": {
    "expression": "-5 + 10"
  }
}
```

**Response:**
```json
{
  "expression": "-5 + 10",
  "result": 5
}
```

### Exponentiation

```json
{
  "name": "calculator",
  "arguments": {
    "expression": "2^53 - 1"
  }
}
```

**Response:**
```json
{
  "expression": "2^53 - 1",
  "result": 9007199254740991
}
```

Note: Exponentiation is right-associative, so `2^3^2` equals `2^(3^2)` = 512, not `(2^3)^2` = 64.

## Parameters

You must provide **either** `expression` or `expressions` parameter (not both).

### expression (optional)
- **Type**: string
- **Description**: Single mathematical expression to evaluate
- **Example**: `"2 + 3 * 4"`

### expressions (optional)
- **Type**: array of strings
- **Description**: Multiple mathematical expressions to evaluate in one call
- **Example**: `["2 + 3", "10 * 5", "(15 - 3) / 4"]`

Each expression can contain:
- Numbers (integers and decimals)
- Basic arithmetic operators (`+`, `-`, `*`, `/`, `%`, `^`)
- Parentheses for grouping
- Unary operators (`+`, `-`)
- Whitespace (ignored)

## Examples

### Simple Arithmetic
```json
{
  "name": "calculator",
  "arguments": {
    "expression": "25 / 5"
  }
}
```

### Percentage Calculations
```json
{
  "name": "calculator",
  "arguments": {
    "expression": "150 * 0.20"
  }
}
```

### Complex Business Calculations
```json
{
  "name": "calculator",
  "arguments": {
    "expression": "(1200 - 200) * 0.15 + 50"
  }
}
```

### Multiple Expressions (Array Mode)
```json
{
  "name": "calculator",
  "arguments": {
    "expressions": [
      "2 + 3",
      "10 * 5",
      "(15 - 3) / 4",
      "2.5 + 1.5"
    ]
  }
}
```

**Response:**
```json
{
  "results": [
    {
      "expression": "2 + 3",
      "result": 5
    },
    {
      "expression": "10 * 5",
      "result": 50
    },
    {
      "expression": "(15 - 3) / 4",
      "result": 3
    },
    {
      "expression": "2.5 + 1.5",
      "result": 4
    }
  ]
}
```

### Batch Financial Calculations
```json
{
  "name": "calculator",
  "arguments": {
    "expressions": [
      "1000 * 0.08",
      "1500 * 0.15",
      "2000 * 0.12"
    ]
  }
}
```

## Error Handling

The calculator provides clear error messages for invalid expressions:

### Division by Zero
```json
{
  "name": "calculator",
  "arguments": {
    "expression": "10 / 0"
  }
}
```
**Error**: "division by zero"

### Invalid Syntax
```json
{
  "name": "calculator",
  "arguments": {
    "expression": "2 + * 3"
  }
}
```
**Error**: "unexpected character '*'"

### Unmatched Parentheses
```json
{
  "name": "calculator",
  "arguments": {
    "expression": "(2 + 3"
  }
}
```
**Error**: "expected closing parenthesis"

## Limitations

- No support for scientific notation (e.g., `1e6`)
- No support for functions (e.g., `sqrt`, `sin`, `cos`, `log`)
- No support for variables or constants (e.g., `π`, `e`)
- No support for bitwise operations
- Decimal precision limited by floating-point representation

package pythonexec

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPythonExecute(t *testing.T) {
	logger := logrus.New()
logger.SetLevel(logrus.DebugLevel) // Enable debug logging for the test

// Get the tool instance directly
toolInstance, ok := registry.GetTool("execute_python_sandbox")
require.True(t, ok, "PythonExecTool should be registered and found")
require.NotNil(t, toolInstance, "PythonExecTool instance should not be nil")

pythonTool, ok := toolInstance.(*PythonExecTool)
	require.True(t, ok, "Registered tool is not of type *PythonExecTool")
	require.NotNil(t, pythonTool, "PythonExecTool instance is nil")

	// Test case 1: Simple print statement
	t.Run("SimplePrint", func(t *testing.T) {
		args := map[string]interface{}{
			"code":            "print(\"hello test\")",
			"timeout_seconds": float64(5),
		}

result, err := pythonTool.Execute(context.Background(), logger, nil, args)
require.NoError(t, err, "Execute should not return an error for a simple print")
require.NotNil(t, result, "Result should not be nil")
require.NotNil(t, result.Content, "Result.Content should not be nil")
require.Len(t, result.Content, 1, "Expected one content item in result.Content")

// Assume the first content item is mcp.TextContent and holds our JSON string in its Text field
contentItem := result.Content[0]
// We need to import mcp package to use mcp.TextContent
// import "github.com/mark3labs/mcp-go/mcp"
// This is a guess for the type name and field.
// Use mcp.TextContent as confirmed from mcp-go source.
textContent, ok := contentItem.(mcp.TextContent)
require.True(t, ok, "Result content item is not of expected type mcp.TextContent")

var resultMap map[string]interface{}
unmarshalErr := json.Unmarshal([]byte(textContent.Text), &resultMap)
require.NoError(t, unmarshalErr, "Failed to unmarshal result text JSON")

t.Logf("Stdout: %s", resultMap["stdout"])
t.Logf("Stderr: %s", resultMap["stderr"])
t.Logf("Error: %v", resultMap["error"])

assert.Equal(t, "hello test\n", resultMap["stdout"], "Stdout should match")
assert.Equal(t, "", resultMap["stderr"], "Stderr should be empty for a simple print")
assert.Nil(t, resultMap["error"], "Error field should be nil for a successful execution")
})

// Test case 2: Code with an error
t.Run("PythonError", func(t *testing.T) {
args := map[string]interface{}{
"code":            "print(1/0)",
"timeout_seconds": float64(5),
}

result, err := pythonTool.Execute(context.Background(), logger, nil, args)
require.NoError(t, err, "Execute itself should not error, Python error goes to stderr")
require.NotNil(t, result, "Result should not be nil")
require.NotNil(t, result.Content, "Result.Content should not be nil")
require.Len(t, result.Content, 1, "Expected one content item in result.Content")

contentItem := result.Content[0]
textContent, ok := contentItem.(mcp.TextContent)
require.True(t, ok, "Result content item is not of expected type mcp.TextContent")

var resultMap map[string]interface{}
unmarshalErr := json.Unmarshal([]byte(textContent.Text), &resultMap)
require.NoError(t, unmarshalErr, "Failed to unmarshal result text JSON")

t.Logf("Stdout: %s", resultMap["stdout"])
t.Logf("Stderr: %s", resultMap["stderr"])
t.Logf("Error: %v", resultMap["error"])

assert.Contains(t, resultMap["stderr"], "ZeroDivisionError", "Stderr should contain ZeroDivisionError")
assert.Equal(t, "PythonException", resultMap["error"], "Error field should indicate PythonException")
})

// Test case 3: Timeout
t.Run("Timeout", func(t *testing.T) {
args := map[string]interface{}{
"code":            "import time\ntime.sleep(2)",
"timeout_seconds": float64(1), // Timeout is 1 second, code sleeps for 2
}

result, err := pythonTool.Execute(context.Background(), logger, nil, args)
require.NoError(t, err, "Execute itself should not error on timeout, timeout is reported in result")
require.NotNil(t, result, "Result should not be nil")
require.NotNil(t, result.Content, "Result.Content should not be nil")
require.Len(t, result.Content, 1, "Expected one content item in result.Content")

contentItem := result.Content[0]
// In case of NewToolResultError, the structure might be different,
// but our pythonexec.go uses NewToolResultText for timeout JSON payload too.
textContent, ok := contentItem.(mcp.TextContent)
require.True(t, ok, "Result content item is not of expected type mcp.TextContent for timeout")

var resultMap map[string]interface{}
unmarshalErr := json.Unmarshal([]byte(textContent.Text), &resultMap)
require.NoError(t, unmarshalErr, "Failed to unmarshal result text JSON for timeout")

t.Logf("Stdout: %s", resultMap["stdout"])
t.Logf("Stderr: %s", resultMap["stderr"])
t.Logf("Error: %v", resultMap["error"])

assert.Contains(t, resultMap["stderr"], "execution timed out", "Stderr should indicate timeout")
assert.Equal(t, "TimeoutError", resultMap["error"], "Error field should be TimeoutError")
})

// Test case 4: Empty code
t.Run("EmptyCode", func(t *testing.T) {
args := map[string]interface{}{
"code":            "",
"timeout_seconds": float64(5),
}

result, err := pythonTool.Execute(context.Background(), logger, nil, args)
require.NoError(t, err, "Execute should not return an error for empty code")
require.NotNil(t, result, "Result should not be nil")
require.NotNil(t, result.Content, "Result.Content should not be nil")
require.Len(t, result.Content, 1, "Expected one content item in result.Content")

contentItem := result.Content[0]
textContent, ok := contentItem.(mcp.TextContent)
require.True(t, ok, "Result content item is not of expected type mcp.TextContent")

var resultMap map[string]interface{}
unmarshalErr := json.Unmarshal([]byte(textContent.Text), &resultMap)
require.NoError(t, unmarshalErr, "Failed to unmarshal result text JSON")

t.Logf("Stdout: %s", resultMap["stdout"])
t.Logf("Stderr: %s", resultMap["stderr"])
t.Logf("Error: %v", resultMap["error"])

assert.Equal(t, "", resultMap["stdout"], "Stdout should be empty for empty code")
assert.Equal(t, "", resultMap["stderr"], "Stderr should be empty for empty code")
assert.Nil(t, resultMap["error"], "Error field should be nil for empty code")
})
}

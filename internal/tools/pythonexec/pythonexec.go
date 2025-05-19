package pythonexec

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/mcp-devtools/internal/registry"
	"github.com/sirupsen/logrus"
	"github.com/tetratelabs/wazero"
	wazeroapi "github.com/tetratelabs/wazero/api" // Add aliased import

	// "github.com/tetratelabs/wazero/api" // Commented line can remain or be removed by formatter
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed micropython.wasm
var micropythonWasm []byte

// PythonExecTool implements the tools.Tool interface for executing Python code.
type PythonExecTool struct {
	runtime       wazero.Runtime
	compiledWasm  wazero.CompiledModule
	moduleConfig  wazero.ModuleConfig
}

// init registers the tool with the registry.
// It also pre-compiles the WASM module.
func init() {
logrus.Infof("Size of embedded micropythonWasm: %d bytes", len(micropythonWasm))
if len(micropythonWasm) == 0 {
logrus.Fatal("Embedded micropython.wasm is empty. Check embedding process and file.")
// No point continuing if WASM is empty
return
}

tool := &PythonExecTool{}

// Create a new Wazero runtime.
	// This is a long-lived value, so we can reuse it.
	ctx := context.Background()
	tool.runtime = wazero.NewRuntime(ctx)

// Instantiate WASI, which implements system calls.
wasi_snapshot_preview1.MustInstantiate(ctx, tool.runtime)

// Attempt to instantiate an "env" module with required functions.
// This is not in the original plan and is a speculative fix based on runtime errors.
envBuilder := tool.runtime.NewHostModuleBuilder("env")

// Add invoke_ii: Assuming signature (i32, i32) -> i32 based on common Emscripten patterns.
// Parameters p1 and p2 are likely function pointers or indices.
envBuilder = envBuilder.NewFunctionBuilder().
WithFunc(func(ctx context.Context, mod wazeroapi.Module, p1 uint32, p2 uint32) uint32 {
logrus.Warnf("env.invoke_ii called with p1=%d, p2=%d. Returning 0 (dummy).", p1, p2)
return 0
}).Export("invoke_ii")

// Add invoke_viii: void-return function taking four integers (one for func ptr + three params).
// Common Emscripten pattern: p0 is a function pointer/table index, p1-p3 are actual parameters.
envBuilder = envBuilder.NewFunctionBuilder().
WithFunc(func(ctx context.Context, mod wazeroapi.Module, p0 uint32, p1 uint32, p2 uint32, p3 uint32) { // Added p0
logrus.Warnf("env.invoke_viii called with p0=%d (func ptr?), p1=%d, p2=%d, p3=%d.", p0, p1, p2, p3)
}).Export("invoke_viii")

// Add invoke_iiiii: i32-return function taking five integers (one for func ptr + four params).
// Common Emscripten pattern used for dynamic calls with more parameters.
envBuilder = envBuilder.NewFunctionBuilder().
WithFunc(func(ctx context.Context, mod wazeroapi.Module, p0 uint32, p1 uint32, p2 uint32, p3 uint32, p4 uint32) uint32 {
logrus.Warnf("env.invoke_iiiii called with p0=%d (func ptr?), p1=%d, p2=%d, p3=%d, p4=%d. Returning 0.", p0, p1, p2, p3, p4)
return 0 // Dummy return like invoke_ii
}).Export("invoke_iiiii")

// TODO: Other "invoke_..." functions might be needed, e.g., invoke_v, invoke_vi, invoke_iii, etc.
// Also potentially "memory" or "table" imports, or globals like "STACKTOP".

_, errEnv := envBuilder.Instantiate(ctx)
if errEnv != nil {
logrus.Fatalf("Failed to instantiate 'env' host module with invoke_ii: %v", errEnv)
return // Stop if "env" module setup fails
}
logrus.Info("Successfully instantiated 'env' host module with invoke_ii stub.")

// Compile the WebAssembly module.
var err error
	tool.compiledWasm, err = tool.runtime.CompileModule(ctx, micropythonWasm)
	if err != nil {
		// Log the error and prevent registration if WASM compilation fails.
		// A real application might panic or handle this more gracefully.
		logrus.Fatalf("Failed to compile MicroPython WASM module: %v", err)
		return
	}
// Note: compiledWasm.Close(ctx) should be called when the tool is no longer needed,
// typically when the application shuts down. For a global tool instance, this might be in a main cleanup.

if tool.compiledWasm != nil {
logrus.Infof("Inspecting imports for compiled MicroPython WASM module:")
for _, imp := range tool.compiledWasm.ImportedFunctions() {
logrus.Infof("WASM Function Import: ModuleName='%s', Name='%s', ParamTypes=%v, ResultTypes=%v", imp.ModuleName(), imp.Name(), imp.ParamTypes(), imp.ResultTypes())
}
// Commenting out memory and global import logging due to persistent compiler issues with API.
// The function import logs should still provide some insight.
/*
for _, imp := range tool.compiledWasm.ImportedMemories() {
	maxVal, hasMax := imp.Max()
	maxStr := "none"
	if hasMax {
		maxStr = fmt.Sprintf("%d", maxVal)
	}
	logrus.Infof("WASM Memory Import: ModuleName='%s', Name='%s', MinPages=%d, MaxPages=%s", imp.ModuleName(), imp.Name(), imp.Min(), maxStr)
}
for _, imp := range tool.compiledWasm.ImportedGlobals() {
	logrus.Infof("WASM Global Import: ModuleName='%s', Name='%s', Type=%s", imp.ModuleName(), imp.Name(), imp.Type().String())
}
*/
} else {
logrus.Error("compiledWasm is nil, cannot inspect imports.")
}

registry.Register(tool)
logrus.Info("PythonExecTool registered and MicroPython WASM compiled")
}

// Definition returns the tool's definition for MCP registration.
func (t *PythonExecTool) Definition() mcp.Tool {
	return mcp.NewTool(
		"execute_python_sandbox",
		mcp.WithDescription("Executes a Python code snippet in a secure WebAssembly (WASM) sandbox. Ideal for quick logic validation, simple calculations, algorithm testing, or verifying small Python expressions. Captures stdout, stderr, and indicates success, timeout, or errors."),
		mcp.WithString("code",
			mcp.Required(),
			mcp.Description("The Python code string to be executed in the sandbox. Should be a complete, executable snippet."),
		),
		mcp.WithNumber("timeout_seconds",
			mcp.Description("Maximum execution time in seconds. Protects against accidental infinite loops or overly long computations in snippets. Defaults to 5 seconds."),
			mcp.DefaultNumber(5),
		),
	)
}

// Execute executes the Python code using the MicroPython WASM module.
func (t *PythonExecTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logger.Info("Executing PythonExecTool")

	pythonCode, ok := args["code"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid required parameter: code (must be string)")
	}

	timeoutSeconds := float64(5) // Default timeout
	if ts, ok := args["timeout_seconds"].(float64); ok {
		timeoutSeconds = ts
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()

	var stdoutBuf, stderrBuf bytes.Buffer

	// Configure the module instance with stdio pipes.
	// Each execution needs its own stdio buffers.
	config := wazero.NewModuleConfig().
		WithStdout(&stdoutBuf).
		WithStderr(&stderrBuf).
		WithStdin(bytes.NewReader([]byte(pythonCode))) // Pass Python code via stdin

	logger.Debugf("Instantiating MicroPython module with code:\n%s", pythonCode)

	// Instantiate the module. This is cheap because the module is already compiled.
	mod, err := t.runtime.InstantiateModule(execCtx, t.compiledWasm, config)
	if err != nil {
		// Check if the error is due to context deadline exceeded (timeout)
		if execCtx.Err() == context.DeadlineExceeded {
			logger.Warnf("WASM execution timed out: %v", err)
// Ensure stderr reflects the timeout for the client
stderrBuf.WriteString(fmt.Sprintf("\nError: execution timed out after %v seconds", timeoutSeconds))
resultMap := map[string]interface{}{
"stdout": stdoutBuf.String(),
"stderr": stderrBuf.String(),
"error":  "TimeoutError",
}
jsonBytes, marshalErr := json.Marshal(resultMap) // Using Marshal for compactness
if marshalErr != nil {
logger.Errorf("Failed to marshal timeout result to JSON: %v", marshalErr)
// Fallback to a simpler error message if JSON marshalling fails
return mcp.NewToolResultError(fmt.Sprintf("TimeoutError (JSON marshal failed: %v)", marshalErr)), nil
}
return mcp.NewToolResultText(string(jsonBytes)), nil
}
logger.Errorf("Failed to instantiate MicroPython module: %v", err)
		return nil, fmt.Errorf("failed to instantiate MicroPython module: %w", err)
	}
	defer mod.Close(execCtx) // Ensure module is closed after execution.

	// The MicroPython WASM built for WASI is expected to read from stdin and exit.
	// The result of its execution (stdout/stderr) is captured in stdoutBuf/stderrBuf.
	// If instantiation succeeded but the context was cancelled (e.g. timeout during execution),
	// the buffers will contain whatever output was generated before the timeout.

	// Default error type is nil (success)
	var errorType interface{} = nil

	// Check if context was cancelled (timeout or other cancellation)
	if execCtx.Err() == context.DeadlineExceeded {
		logger.Warn("Python execution timed out.")
		stderrBuf.WriteString(fmt.Sprintf("\nError: execution timed out after %v seconds", timeoutSeconds))
		errorType = "TimeoutError"
	} else if execCtx.Err() != nil {
		logger.Warnf("Python execution context error: %v", execCtx.Err())
		stderrBuf.WriteString(fmt.Sprintf("\nError: execution context error: %v", execCtx.Err()))
		errorType = "WasmExecutionError" // Or a more specific error if identifiable
	}


	// Heuristic: If stderr contains "Error:" or "Exception:", it's likely a Python exception.
	// This is a simple check; a more robust solution might involve parsing stderr.
	// MicroPython specific error messages for common issues:
	// - "MemoryError"
	// - "SyntaxError"
	// - "NameError"
	// - "TypeError"
	// - "ValueError"
	// - "IndexError"
	// - "KeyError"
	// - "ImportError"
	// - "ZeroDivisionError"
	// - "OverflowError"
	// - "AssertionError"
	// - "IndentationError"
	// - "SystemExit" (though less common for snippets)
	// - "KeyboardInterrupt" (less likely in this non-interactive setup)
	// - "Exception" (generic base)
	// We'll set a generic "PythonException" if stderr is non-empty and no other error type (like Timeout) was set.
	if stderrBuf.Len() > 0 && errorType == nil {
		// Check for specific Python error keywords if needed, for now, generic
		// This is a basic heuristic. A more robust way would be if the WASM module
		// could signal error types more directly (e.g., exit codes), but WASI
		// stdio model makes this tricky without custom extensions.
		errorType = "PythonException"
	}


logger.Debugf("Python execution finished. Stdout: %s, Stderr: %s", stdoutBuf.String(), stderrBuf.String())

resultMap := map[string]interface{}{
"stdout": stdoutBuf.String(),
"stderr": stderrBuf.String(),
"error":  errorType,
}
jsonBytes, marshalErr := json.Marshal(resultMap) // Using Marshal for compactness
if marshalErr != nil {
logger.Errorf("Failed to marshal execution result to JSON: %v", marshalErr)
// Fallback to a simpler error message if JSON marshalling fails for the main result
return mcp.NewToolResultError(fmt.Sprintf("ExecutionResultMarshalError (JSON marshal failed: %v)", marshalErr)), nil
}
return mcp.NewToolResultText(string(jsonBytes)), nil
}


# Python Exec Tool Plan

## Goal

Implement a Go tool (`PythonExecTool`) that uses Wazero to execute Python code in a MicroPython WASM environment. The WASM module should be a standalone WASI application, taking Python code via stdin and producing output via stdout/stderr.

## Current Status & Summary (as of 2025-05-08)

We have a candidate `micropython.wasm` (1.1MB) located at `internal/tools/pythonexec/micropython.wasm`. This was built by modifying MicroPython's `ports/webassembly/Makefile` to target WASI using Emscripten.

The primary challenge has been testing this WASM with the `pythonexec.go` tool due to:

1. Persistent issues with the `replace_in_file` tool not reliably updating `main.go` for simplified test runs.
2. The full `main.go` application hangs after tool registration logs, preventing the `pythonexec` test code from being verifiably reached and executed.

The `pythonexec.go` code itself includes `go:embed` for the WASM, Wazero runtime setup, WASI preview instantiation, and piping for stdin/stdout/stderr. Detailed logging has been added around Wazero calls.

### What has been tried & worked:

- __Initial `pythonexec.go` structure:__ Core Go file setup with Wazero basics.

- __WASM Module Investigation:__ Identified initial NPM-sourced WASM as JS-interop, not WASI.

- __Building MicroPython `ports/webassembly` (as-is):__ Successfully built the JS-interop version, confirming Emscripten build environment.

- __Building MicroPython `ports/webassembly` (modified for WASI):__

  - Modified `ports/webassembly/Makefile`:

    - `JSFLAGS` set to `-s WASI=1 -s STANDALONE_WASM=1 -s ERROR_ON_UNDEFINED_SYMBOLS=0 -s SUPPORT_LONGJMP=emscripten`.
    - Removed JS-specific C files from `SRC_C`.
    - Removed JS library linking.
    - Targeted `.wasm` output.

  - This build produced the current `internal/tools/pythonexec/micropython.wasm`.

### What hasn't worked (Debugging Blockers):

- __`replace_in_file` tool reliability:__ Failed to update `main.go` for focused testing.
- __Program Hang with Full `main.go`:__ The application hangs after tool registration, before test code in `main.go`'s `Action` function is reached. The exact cause of this hang (CLI framework, other tools, or early interaction with `pythonexec`) is unclear due to the inability to simplify `main.go` via tooling.

### Current `micropython.wasm` Status:

- Located at `internal/tools/pythonexec/micropython.wasm` (1.1MB).
- Built from `ports/webassembly` with Makefile modifications for WASI.
- __Untested and unverified__ as a fully compliant WASI module that correctly handles stdio. The `main.c` used in its compilation (from `ports/webassembly`) might still have JS environment assumptions.

## Next Steps & Recommendations for a New Agent

The immediate goal is to determine if the current `micropython.wasm` can run Python code via WASI and if the `pythonexec.go` Wazero logic is correct.

1. __RECOMMENDED FIRST STEP: Test `pythonexec.Execute` in Isolation via a Go Test File:__

   - __Why:__ Bypasses the problematic `main.go` and `urfave/cli` framework, allowing direct testing of the core `pythonexec.go` logic with the compiled `micropython.wasm`.

   - __How:__

     - Create `internal/tools/pythonexec/pythonexec_test.go`.

     - Write a `func TestPythonExecute(t *testing.T)`:

       - Instantiate `PythonExecTool`.
       - Prepare simple Python code (e.g., `print("hello test")`).
       - Call `tool.Execute()` with necessary context, logger, and parameters.
       - Use `t.Logf` for verbose output from the test.
       - Assert expected `stdout` (e.g., "hello test\n") and empty `stderr`.
       - Run with `go test -v ./internal/tools/pythonexec/...`.

     - The existing debug logs in `pythonexec.go` will provide insight into Wazero's operations.

2. __If Go Test Fails/Hangs: Verify `micropython.wasm` with a Standalone WASI Runtime:__

   - __Why:__ To determine if the issue lies with the `micropython.wasm` module itself (e.g., it's not a valid WASI module, doesn't handle stdio correctly, or has unresolved JS imports).

   - __How:__

     - Use a command-line WASI runtime like `wasmtime` or `wasmer`.
     - Example: `wasmtime run internal/tools/pythonexec/micropython.wasm`
     - Try piping a simple script: `echo "print('hello from wasmtime')" | wasmtime run internal/tools/pythonexec/micropython.wasm`
     - Observe output, errors, or hangs.

   - __Expected behavior for a working WASI MicroPython:__ It should start a REPL or execute the script from stdin.

3. __If `micropython.wasm` is Faulty: Revisit WASM Build Process:__

   - __Why:__ The current WASM build might be incomplete or incorrect for pure WASI.

   - __Key areas to investigate in `ports/webassembly/Makefile` and C sources:__

     - __`main.c`:__ The `main.c` in `ports/webassembly` (currently `micropython/ports/webassembly/main.c`) is critical. It *must* initialize MicroPython and then enter a loop that reads Python code from WASI stdin (e.g., using `fgets` or `read` on file descriptor 0) and executes it, printing results to WASI stdout/stderr (FD 1 & 2). If it's still geared towards JS interop for I/O or REPL management, it won't work for WASI. Consider adapting `main.c` from `ports/unix/main.c` or writing a minimal one specifically for WASI.
     - __Linker Flags & Dependencies:__ Ensure no Emscripten JS libraries (`library.js`, etc.) or JS-interfacing C files are accidentally linked. Setting `ERROR_ON_UNDEFINED_SYMBOLS=1` in `JSFLAGS` (temporarily) during linking can help identify if the WASM still expects JS-provided functions.
     - __Emscripten `mínima_runtime`:__ For a pure WASI build without any JS, one might typically use `clang` with the WASI SDK directly, or ensure `emcc` is used with flags that minimize JS glue (e.g. `-s STANDALONE_WASM=1` is good, but ensure no other flags pull in JS).
     - __Consider `ports/unix` again:__ If `ports/webassembly` proves too difficult to convert to pure WASI, attempting to compile `ports/unix` C files with `emcc` and `-s WASI=1` using a custom minimal Makefile (or direct `emcc` command line listing all sources) might be an alternative, as its C code is already POSIX-oriented.

Once a working `micropython.wasm` is confirmed and `pythonexec_test.go` passes, the original `main.go` can be restored and debugged separately if its CLI/server functionality is still required.


---

Original plan below:


# Plan: Add Python Execution Tool via Go & Wazero (WebAssembly)

## 1. Project Overview

The goal is to add a new MCP tool to the `mcp-devtools` project that allows AI coding agents to execute arbitrary Python code snippets in a secure, sandboxed environment. This will be achieved by embedding a MicroPython interpreter (compiled to WebAssembly) into the existing Go application and using the `wazero` library as the WASM runtime.

## 2. Tool Definition (for MCP Server)

*   **Tool Name:** `execute_python_sandbox`
*   **Description:**
    > "Executes a Python code snippet in a secure WebAssembly (WASM) sandbox. Ideal for quick logic validation, simple calculations, algorithm testing, or verifying small Python expressions. Captures stdout, stderr, and indicates success, timeout, or errors."
*   **Parameters:**
    *   `code` (string, required):
        > "The Python code string to be executed in the sandbox. Should be a complete, executable snippet."
    *   `timeout_seconds` (number, optional, default: 5):
        > "Maximum execution time in seconds. Protects against accidental infinite loops or overly long computations in snippets. Defaults to 5 seconds."
*   **Expected Output (JSON object):**
    *   `stdout` (string): Content written to standard output by the Python script.
    *   `stderr` (string): Content written to standard error by the Python script. Includes timeout messages or internal WASM execution errors.
    *   `error` (string, nullable): An error type string (e.g., "TimeoutError", "WasmExecutionError", "PythonException") if a significant error occurred, otherwise `null`. Python's own exceptions will typically appear in `stderr`.

## 3. Architecture

The tool will integrate a WebAssembly (WASM) module containing the MicroPython interpreter. The Go host application (`mcp-devtools`) will use the `wazero` runtime to load, execute, and manage this WASM module.

```mermaid
graph TD
    subgraph AI Coding Agent
        A[Agent sends MCP Request] --> B{execute_python_sandbox};
    end

    subgraph mcp-devtools (Go Application)
        B --> C[PythonExecTool (Go struct)];
        C --> D{wazero Runtime};
        subgraph D
            E[Load micropython.wasm] --> F[Compile Module];
            F --> G[Instantiate Module with WASI];
            G --> H[Set up stdin/stdout/stderr pipes];
        end
        C -- Python code, timeout ---> I{Invoke WASM function to run code};
        subgraph MicroPython WASM Module (Guest)
            I --> J[MicroPython Interpreter executes code];
            J -- Output/Errors ---> K[Writes to WASM stdout/stderr];
        end
        H --- Captured stdout/stderr ---> C;
        C -- Formatted Result (stdout, stderr, error) ---> L[MCP Response];
    end

    L --> M[AI Coding Agent receives result];

    classDef agent fill:#D6EAF8,stroke:#2E86C1,color:#2E86C1;
    classDef mcp_server fill:#E8F8F5,stroke:#1ABC9C,color:#1ABC9C;
    classDef wasm_runtime fill:#FEF9E7,stroke:#F1C40F,color:#F1C40F;
    classDef tool_internal fill:#FDEDEC,stroke:#E74C3C,color:#E74C3C;
    classDef wasm_guest fill:#EAECEE,stroke:#839192,color:#839192;


    class A,B,M agent;
    class C,L mcp_server;
    class D,E,F,G,H,I tool_internal;
    class J,K wasm_guest;
```

**Components:**

*   **`PythonExecTool` (Go):** The MCP tool implementation in Go. It will manage the `wazero` runtime and the MicroPython WASM lifecycle.
*   **`wazero` Runtime (Go library):** Responsible for loading, compiling, instantiating, and running the WASM module. It provides the sandboxed environment.
*   **`micropython.wasm` (WASM binary):** The MicroPython interpreter compiled to WebAssembly. This will be embedded in the Go application.
*   **WASI (WebAssembly System Interface):** `wazero` will provide WASI support, which the `micropython.wasm` module will likely require for basic functionalities like I/O.

## 4. Core Implementation Steps (Go & `wazero`)

The implementation will reside in a new package, e.g., `internal/tools/pythonexec/pythonexec.go`.

1.  **Acquire and Embed `micropython.wasm`:**
    *   Obtain a suitable `micropython.wasm` binary. A good starting point is the `firmware.wasm` found in projects like `rafi16jan/micropython-wasm` or by investigating the official MicroPython `ports/javascript` build process.
    *   Embed this `.wasm` file into the Go binary using `//go:embed`.

2.  **Initialize `wazero` Runtime and Compile Module (Tool `init`):**
    *   Create a `wazero.Runtime` instance.
    *   Instantiate `wasi_snapshot_preview1` for basic POSIX-like functionality needed by MicroPython.
    *   Compile the embedded `micropython.wasm` bytes into a `wazero.CompiledModule`. This is done once for efficiency.
    *   Store the runtime and compiled module in the `PythonExecTool` struct.

3.  **Implement `PythonExecTool.Execute()`:**
    *   **Parse Arguments:** Get `code` (string) and `timeout_seconds` (number).
    *   **Set up Execution Context:** Create a `context.Context` with the specified timeout.
    *   **Configure Module Instance:**
        *   Create `bytes.Buffer` for stdout and stderr.
        *   Use `wazero.NewModuleConfig().WithStdout(&stdoutBuf).WithStderr(&stderrBuf)` to redirect I/O.
    *   **Instantiate Module:** `runtime.InstantiateModule(ctx, compiledMod, moduleConfig)`.
    *   **Interface with MicroPython WASM (CRITICAL & NEEDS INVESTIGATION):**
        *   This is the most complex part and depends on the specific C API exported by the `micropython.wasm` module.
        *   **Goal:** Pass the Python `code` string to the MicroPython interpreter running inside WASM and trigger its execution.
        *   **Possible Mechanisms:**
            1.  **Exported `eval_string` function:** The WASM module might export a C function like `int mp_js_do_str(const char *code)` or similar.
                *   Go code would need to:
                    *   Allocate memory within the WASM instance's memory using `module.Memory().Allocate()`.
                    *   Write the Python code string into this allocated WASM memory using `module.Memory().Write()`.
                    *   Call the exported WASM function (e.g., `run_code_func.Call(ctx, pointer_to_code_in_wasm, length_of_code)`).
                    *   Free the allocated WASM memory.
            2.  **Standard Input:** If the MicroPython WASM is set up to run a script from stdin or a "main" entry point that reads code, Go might need to write the code to the WASM's stdin pipe (configured via `ModuleConfig`). This is less direct for simple string evaluation.
            3.  **Virtual Filesystem:** `wazero` supports virtual filesystems. Go could write the code to a virtual file, and the WASM module could be invoked to run that file.
        *   The exact function names and calling conventions must be determined by inspecting the MicroPython JavaScript port's C source or the `micropython.js` glue code from projects like `rafi16jan/micropython-wasm`.
    *   **Capture Output:** After execution (or timeout), read from `stdoutBuf` and `stderrBuf`.
    *   **Handle Errors & Timeout:**
        *   Check `execCtx.Err()` for `context.DeadlineExceeded`.
        *   Handle errors from `InstantiateModule` or function calls.
    *   **Format and Return Result:** Construct the JSON output with `stdout`, `stderr`, and `error` fields.

## 5. Key Considerations & Challenges

*   **MicroPython WASM Module API:**
    *   **The primary challenge.** Determining the precise C functions exported by `micropython.wasm` for initializing, passing code, and executing it is crucial. This may require examining Emscripten glue code or the MicroPython port's C source.
    *   How Python exceptions are propagated (e.g., written to stderr, specific return codes).
*   **Standard Library Limitations:**
    *   MicroPython has a subset of the CPython standard library. Complex scripts relying on unavailable modules will fail. This should be documented for users of the tool.
    *   Modules requiring OS features like networking or extensive filesystem access will likely be non-functional or heavily restricted within the WASM sandbox.
*   **Security:**
    *   WASM provides good sandboxing by default (memory isolation, controlled function calls).
    *   The `micropython.wasm` binary itself should be from a trusted source or built carefully.
    *   Resource limits (memory, CPU via timeout) are important. `wazero` allows setting memory page limits during module instantiation.
*   **Error Handling:**
    *   Distinguish between:
        *   Go-level errors (e.g., failed to load WASM).
        *   WASM execution errors (e.g., trap, timeout).
        *   Python script errors (exceptions printed to stderr).
*   **Performance:**
    *   WASM startup and Python interpretation overhead. For small snippets, this should be acceptable.
    *   The one-time compilation of the WASM module in `init()` helps.
*   **Resource Management:** Ensure WASM instances and associated resources are properly closed/released (e.g., `defer mod.Close(ctx)`).

## 6. Potential Benefits for AI Coding Agents

*   **Quick Code Validation:** Verify syntax, simple logic, and expressions.
*   **Simple Calculations:** Perform mathematical operations.
*   **Algorithm Sketching:** Test small algorithmic components.
*   **Data Manipulation Snippets:** Experiment with list/dict operations, string processing.
*   **Reduced Hallucination:** Allows agents to confirm Python behavior instead of relying solely on learned knowledge.
*   **Interactive Learning/Debugging:** Agents can run code incrementally.

This plan provides a high-level roadmap. The most significant next step before implementation would be to acquire a `micropython.wasm` binary and investigate its exported C API to confirm the exact interfacing mechanism.

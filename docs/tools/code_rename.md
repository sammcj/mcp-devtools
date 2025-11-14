

# code_rename

Rename symbols (variables, functions, types, methods) across codebases using Language Server Protocol (LSP) servers.

## Overview

The `code_rename` tool uses LSP servers to identify and rename symbols across files. It handles imports, comments, and cross-file references according to the LSP server's rename implementation.

## Supported Languages

When enabled at the point where the MCP-DevTools server starts the tool detects which language servers are available and will provide rename functionality for. If it cannot find the appropriate server for a given language it will simply not make the rename functionality for that language available to clients.

- **Go** - via `gopls`
- **TypeScript/JavaScript** - via `typescript-language-server`
- **Python** - via `pyright-langserver`
- **Rust** - via `rust-analyzer`
- **Bash/Shell** - via `bash-language-server`
- **HTML** - via `vscode-html-language-server`
- **CSS/SCSS/LESS** - via `vscode-css-language-server`
- **JSON** - via `vscode-json-language-server`
- **YAML** - via `yaml-language-server`
- **C/C++** - via `clangd`
- **Java** - via `jdtls`
- **Swift** - via `sourcekit-lsp`

## Requirements

The tool requires the appropriate LSP server to be installed for the language you're working with:

```bash
# Go
go install golang.org/x/tools/gopls@latest

# TypeScript/JavaScript
pnpm install -g typescript-language-server

# Python
pip install pyright

# Rust
rustup component add rust-analyzer

# Bash
pnpm install -g bash-language-server

# HTML, CSS, SCSS, LESS, JSON
pnpm install -g vscode-langservers-extracted

# YAML
pnpm install -g yaml-language-server

# C/C++
brew install llvm  # macOS
apt install clangd # Linux

# Java
brew install jdtls # macOS
# Or download from Eclipse JDT Language Server

# Swift
# Included with Xcode or Swift toolchain
```

## Parameters

| Parameter   | Type    | Required | Description                                               |
|-------------|---------|----------|-----------------------------------------------------------|
| `file_path` | string  | Yes      | Absolute path to file containing the symbol               |
| `old_name`  | string  | Yes      | Current name of the symbol to rename                      |
| `new_name`  | string  | Yes      | New name for the symbol                                   |
| `preview`   | boolean | No       | If true, returns preview without applying (default: true) |

## Response Format

```json
{
  "files_modified": 5,
  "total_replacements": 23,
  "affected_files": [
    "/path/to/main.go (12 changes)",
    "/path/to/utils.go (7 changes)",
    "/path/to/types.go (4 changes)"
  ],
  "applied": true
}
```

Error responses include an `error` field:

```json
{
  "error": "failed to find symbol 'oldName' in file"
}
```

## Usage Examples

### Preview Rename (Default)

```json
{
  "file_path": "/Users/dev/project/main.go",
  "old_name": "handleData",
  "new_name": "processData",
  "preview": true
}
```

Returns a preview of all changes without modifying files. The tool automatically finds the symbol position.

### Apply Rename

```json
{
  "file_path": "/Users/dev/project/main.go",
  "old_name": "handleData",
  "new_name": "processData",
  "preview": false
}
```

Applies the rename operation to all affected files.

### Rename Python Variable

```json
{
  "file_path": "/Users/dev/app/handlers.py",
  "old_name": "userId",
  "new_name": "user_id"
}
```

## LSP Server Features

The tool relies on LSP server capabilities for rename operations:

- **Scope analysis**: Renames symbols within correct scope (LSP server dependent)
- **Type checking**: Validates rename constraints (LSP server dependent)
- **Reference tracking**: Finds references including imports (LSP server dependent)
- **Shadowing detection**: Identifies shadowing issues (LSP server dependent)
- **Preview mode**: Default mode shows changes without applying them

## Common Errors

### No LSP Server Available

**Error**: `no LSP server available for python`

**Solution**: Install the required LSP server for your language (see Requirements section).

### Unsupported File Type

**Error**: `unsupported file type: .txt`

**Solution**: The tool only supports file types with LSP server support. Use it with supported languages.

### Symbol Not Found

**Error**: `failed to find symbol 'oldName' in file`

**Solution**: Ensure the `old_name` parameter exactly matches the symbol name in the file (case-sensitive). The tool searches for the first occurrence of the symbol name. Check for typos.

## Performance Notes

- **Server detection**: LSP server availability is cached for 5 minutes to avoid repeated checks
- **Client caching**: LSP server connections are cached for 5 minutes to improve performance for batch operations
- **Startup time**: First rename in a workspace may take 1-2 seconds whilst the LSP server initialises
- **Batch operations**: Subsequent renames in the same workspace are 10-100x faster due to connection reuse
- **Large projects**: Rename operations scale with project size; preview mode is recommended for large codebases

## Improvements (v0.48.3+)

### LSP-Validated Symbol Finding
The tool now uses LSP's `PrepareRename` to validate symbol positions, ensuring correct detection even when symbols appear in:
- Block comments (`/* */`, `"""`, `'''`)
- String literals
- Multiple locations in the same file

This eliminates false positives where the tool might previously have targeted comments or strings instead of actual code symbols.

### Connection Pooling
LSP server connections are now cached and reused within a 5-minute window, dramatically improving performance for batch rename operations:
- **Previous**: 2 seconds per rename (N renames = 2N seconds)
- **Now**: 2 seconds first rename + 0.1-0.2 seconds per subsequent rename

## Limitations

- **LSP server required**: Each language needs its LSP server installed
- **No rollback**: If applying changes fails partway through multiple files, some files may be modified whilst others aren't, potentially leaving the codebase in an inconsistent state. Always commit before using `preview: false`
- **Language-specific**: Not all LSP servers support all rename scenarios
- **Single language**: Cross-language renames are not supported
- **Timeout constraints**: Operations have timeouts (10s init, 5s prepare, 30s rename)

## Configuration

The tool is **disabled by default**. Enable it by adding to `ENABLE_ADDITIONAL_TOOLS`:

```bash
export ENABLE_ADDITIONAL_TOOLS=code_rename
```

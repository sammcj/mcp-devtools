# Project Actions Tool

Execute project development tasks (tests, linters, formatters) through a project's Makefile, along with (very) limited git operations.

## Overview

The project-actions tool provides a security-aware interface for running common development workflows without requiring knowledge of project-specific tooling. It acts as a thin wrapper around `make` and `git` commands, enabling AI agents to perform development tasks in a controlled manner.

## Security Framework Dependency

**Important:** This tool requires the `security` tool to be enabled in `ENABLE_ADDITIONAL_TOOLS`.

The tool integrates with the security framework to:
- Use `security.Operations.SafeFileRead()` for reading Makefiles
- Validate working directory ownership and permissions
- Prevent path traversal attacks
- Block operations in system directories

## Operations

### Make Targets

Execute any `.PHONY` target defined in the project's Makefile.  Targets must conform to a very limited regex, consisting of only alphanumeric characters, hyphen, and underscore.

**Parameters:**
- `operation`: Target name from Makefile (e.g., "test", "lint", "build")
- `working_directory`: Optional. Project directory (default: current directory)
- `dry_run`: Optional. Preview command without execution

**Example:**
```json
{
  "operation": "test",
  "working_directory": "/path/to/project"
}
```

### Git Add

Stage files for commit in a single batch operation.

**Parameters:**
- `operation`: "add"
- `paths`: Array of relative file paths
- `working_directory`: Optional. Project directory
- `dry_run`: Optional. Preview command without execution

**Example:**
```json
{
  "operation": "add",
  "paths": ["src/main.go", "src/utils.go"],
  "working_directory": "/path/to/project"
}
```

### Git Commit

Create a commit with a custom message passed via stdin.

**Parameters:**
- `operation`: "commit"
- `message`: Commit message (max 16KB by default)
- `working_directory`: Optional. Project directory
- `dry_run`: Optional. Preview command without execution

**Example:**
```json
{
  "operation": "commit",
  "message": "Fix authentication bug\n\nUpdated token validation logic"
}
```

### Generate Makefile

Auto-generate a language-specific Makefile with common development targets.

**Parameters:**
- `operation`: "generate"
- `language`: One of: "python", "rust", "go", "nodejs"
- `working_directory`: Optional. Project directory

**Example:**
```json
{
  "operation": "generate",
  "language": "python",
  "working_directory": "/path/to/project"
}
```

## Language Templates

### Python

Generated targets:
- `default`: Alias for test
- `test`: Run pytest
- `lint`: Run ruff check
- `format`: Run black formatter
- `fix`: Run ruff check --fix

**Example Makefile:**
```makefile
.PHONY: default test lint format fix

default: test

test:
	pytest

lint:
	ruff check

format:
	black .

fix:
	ruff check --fix
```

### Rust

Generated targets:
- `default`: Alias for build
- `build`: Run cargo build
- `test`: Run cargo test
- `lint`: Run cargo clippy
- `format`: Run cargo fmt
- `fix`: Run cargo clippy --fix

**Example Makefile:**
```makefile
.PHONY: default build test lint format fix

default: build

build:
	cargo build

test:
	cargo test

lint:
	cargo clippy

format:
	cargo fmt

fix:
	cargo clippy --fix
```

### Go

Generated targets:
- `default`: Alias for build
- `build`: Run go build
- `test`: Run go test ./...
- `lint`: Run golangci-lint run
- `format`: Run go fmt
- `fix`: Run go fmt

**Example Makefile:**
```makefile
.PHONY: default build test lint format fix

default: build

build:
	go build

test:
	go test ./...

lint:
	golangci-lint run

format:
	go fmt

fix:
	go fmt
```

### Node.js

Generated targets:
- `default`: Alias for test
- `test`: Run npm test
- `lint`: Run npm run lint
- `format`: Run npm run format
- `fix`: Run npm run lint:fix

**Example Makefile:**
```makefile
.PHONY: default test lint format fix

default: test

test:
	npm test

lint:
	npm run lint

format:
	npm run format

fix:
	npm run lint:fix
```

## Configuration

### Environment Variables

- `PROJECT_ACTIONS_MAX_COMMIT_SIZE`: Maximum commit message size in bytes (default: 16384)

**Example:**
```bash
export PROJECT_ACTIONS_MAX_COMMIT_SIZE=32768
```

## Security Features

### Working Directory Validation

- Blocks system directories: `/`, `/bin`, `/lib`, `/usr`, `/etc`, `/var`, `/sys`, `/proc`, `/dev`, `/boot`, `/sbin`
- Verifies directory is owned by current user
- Checks owner write bit is set

### Path Validation

- All file paths must be relative to working directory
- Path traversal (e.g., `../file`) is blocked
- Paths are resolved using `filepath.Clean()` and `filepath.Abs()`

### Command Execution

- No shell interpretation - direct command execution
- Commit messages passed via stdin to prevent injection
- Target names validated (alphanumeric, hyphen, underscore only)

## Error Handling

The tool returns command output and exit codes without modification. Common errors:

### Tool Not Found
```
make not found on PATH - install make to use this tool
git not found on PATH - install git to use git operations
```

**Solution:** Install required tools using your package manager.

### Invalid Target
```
target 'X' not found in Makefile .PHONY targets
```

**Solution:** Check Makefile for available targets or use `generate` operation.

### Git Not Initialized
```
git command failed - you may need to run 'git init' or specify another directory
```

**Solution:** Initialize git repository or specify correct working directory.

### Path Escape
```
path escapes working directory: ../file
```

**Solution:** Use relative paths within the project directory.

### Commit Too Large
```
commit message exceeds 16 KB limit
```

**Solution:** Reduce message size or increase `PROJECT_ACTIONS_MAX_COMMIT_SIZE`.

## Limitations

- Only executes `.PHONY` targets from Makefile
- Does not validate or scan Makefile commands
- Requires `make` and `git` on system PATH
- Single Makefile per project (in working directory root)

## Best Practices

1. **Use dry-run mode** to preview commands before execution
2. **Generate Makefile first** if project doesn't have one
3. **Batch git operations** - add multiple files in single operation
4. **Validate working directory** - ensure it's the correct project
5. **Check exit codes** - non-zero indicates command failure

## Examples

### Complete Workflow

```json
// 1. Generate Makefile for Python project
{
  "operation": "generate",
  "language": "python"
}

// 2. Run tests
{
  "operation": "test"
}

// 3. Stage modified files
{
  "operation": "add",
  "paths": ["src/main.py", "tests/test_main.py"]
}

// 4. Create commit
{
  "operation": "commit",
  "message": "Add new feature"
}
```

### Dry-Run Preview

```json
{
  "operation": "test",
  "dry_run": true
}
```

Returns command preview without execution.

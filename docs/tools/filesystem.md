# Filesystem Tool

The filesystem tool provides secure file and directory operations with strict access controls. **This tool is disabled by default for security reasons** and must be explicitly enabled.

## Security Notice

⚠️ **IMPORTANT**: This tool provides direct filesystem access and is **disabled by default**. Only enable it if you understand the security implications and trust the AI agent with filesystem operations.

## Enabling the Tool

To enable the filesystem tool, set the environment variable:
```bash
export FILESYSTEM_TOOL_ENABLE=true
```

## Configuration

### Environment Variables

- **`FILESYSTEM_TOOL_ENABLE`** (required): Set to `"true"` to enable the tool (disabled by default)
- **`FILESYSTEM_TOOL_ALLOWED_DIRS`** (optional): Colon-separated (Unix) list of allowed directory paths

### Custom Allowed Directories

By default, the tool allows access to:
- Current working directory
- User home directory

To restrict access to specific directories only:

**Unix/Linux/macOS:**
```bash
export FILESYSTEM_TOOL_ALLOWED_DIRS="/home/user/projects:/tmp:/home/user/documents"
```

### MCP Configuration Example

```json
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "/path/to/mcp-devtools",
      "env": {
        "FILESYSTEM_TOOL_ENABLE": "true",
        "FILESYSTEM_TOOL_ALLOWED_DIRS": "/home/user/projects:/tmp"
      }
    }
  }
}
```

## Features

- **File Operations**: Read, write, and edit files with atomic operations
- **Directory Operations**: Create, list, and navigate directories
- **File Search**: Recursively search for files with pattern matching
- **File Metadata**: Get detailed file information including size, permissions, and timestamps
- **Security**: Strict directory access control prevents operations outside allowed directories
- **Advanced Features**: Head/tail file reading, directory trees, file moving
- **Configurable Access**: Customisable allowed directories via environment variables

## Functions

### File Operations

#### `read_file`
Read complete contents of a file or specific lines.

**Parameters:**
- `path` (required): File path to read
- `head` (optional): Read only first N lines
- `tail` (optional): Read only last N lines

**Example:**
```json
{
  "function": "read_file",
  "options": {
    "path": "/path/to/file.txt",
    "head": 10
  }
}
```

#### `read_multiple_files`
Read multiple files simultaneously for efficient batch operations.

**Parameters:**
- `paths` (required): Array of file paths

**Example:**
```json
{
  "function": "read_multiple_files",
  "options": {
    "paths": ["/path/to/file1.txt", "/path/to/file2.txt"]
  }
}
```

#### `write_file`
Create new file or overwrite existing file with content.

**Parameters:**
- `path` (required): File path to write
- `content` (required): Content to write

**Example:**
```json
{
  "function": "write_file",
  "options": {
    "path": "/path/to/file.txt",
    "content": "Hello, World!"
  }
}
```

#### `edit_file`
Make selective edits to files with diff preview.

**Parameters:**
- `path` (required): File path to edit
- `edits` (required): Array of edit operations with `oldText` and `newText`
- `dryRun` (optional): Preview changes without applying (default: false)

**Example:**
```json
{
  "function": "edit_file",
  "options": {
    "path": "/path/to/file.txt",
    "edits": [
      {
        "oldText": "old content",
        "newText": "new content"
      }
    ],
    "dryRun": true
  }
}
```

### Directory Operations

#### `create_directory`
Create directory with parent directories as needed.

**Parameters:**
- `path` (required): Directory path to create

**Example:**
```json
{
  "function": "create_directory",
  "options": {
    "path": "/path/to/new/directory"
  }
}
```

#### `list_directory`
List directory contents with file/directory indicators.

**Parameters:**
- `path` (required): Directory path to list

**Example:**
```json
{
  "function": "list_directory",
  "options": {
    "path": "/path/to/directory"
  }
}
```

#### `list_directory_with_sizes`
List directory contents with file sizes and sorting options.

**Parameters:**
- `path` (required): Directory path to list
- `sortBy` (optional): Sort by "name" or "size" (default: "name")

**Example:**
```json
{
  "function": "list_directory_with_sizes",
  "options": {
    "path": "/path/to/directory",
    "sortBy": "size"
  }
}
```

#### `directory_tree`
Get recursive tree view of directory structure as JSON.

**Parameters:**
- `path` (required): Root directory path

**Example:**
```json
{
  "function": "directory_tree",
  "options": {
    "path": "/path/to/directory"
  }
}
```

### File Management

#### `move_file`
Move or rename files and directories.

**Parameters:**
- `source` (required): Source path
- `destination` (required): Destination path

**Example:**
```json
{
  "function": "move_file",
  "options": {
    "source": "/path/to/old/location",
    "destination": "/path/to/new/location"
  }
}
```

#### `search_files`
Recursively search for files matching a pattern.

**Parameters:**
- `path` (required): Starting directory path
- `pattern` (required): Search pattern (case-insensitive)
- `excludePatterns` (optional): Array of patterns to exclude

**Example:**
```json
{
  "function": "search_files",
  "options": {
    "path": "/path/to/search",
    "pattern": "*.txt",
    "excludePatterns": ["*.tmp", "node_modules"]
  }
}
```

#### `get_file_info`
Get detailed metadata about a file or directory.

**Parameters:**
- `path` (required): File or directory path

**Example:**
```json
{
  "function": "get_file_info",
  "options": {
    "path": "/path/to/file.txt"
  }
}
```

### Security

#### `list_allowed_directories`
List all directories the tool is allowed to access.

**Parameters:** None

**Example:**
```json
{
  "function": "list_allowed_directories",
  "options": {}
}
```

## Security Features

### Directory Access Control
- All operations are restricted to allowed directories
- Symlink validation prevents directory traversal attacks
- Atomic file operations prevent race conditions
- Path normalisation prevents bypass attempts

### Default Allowed Directories
- Current working directory
- User home directory
- Can be configured via MCP Roots protocol

### Safe Operations
- Atomic writes using temporary files
- Symlink resolution and validation
- Parent directory validation for new files
- Comprehensive error handling

## Usage Examples

### Basic File Operations
```json
// Write a configuration file
{
  "function": "write_file",
  "options": {
    "path": "./config.json",
    "content": "{\"debug\": true}"
  }
}

// Read the file back
{
  "function": "read_file",
  "options": {
    "path": "./config.json"
  }
}
```

### Directory Management
```json
// Create project structure
{
  "function": "create_directory",
  "options": {
    "path": "./src/components"
  }
}

// List contents with sizes
{
  "function": "list_directory_with_sizes",
  "options": {
    "path": "./src",
    "sortBy": "size"
  }
}
```

### File Search and Analysis
```json
// Find all TypeScript files
{
  "function": "search_files",
  "options": {
    "path": "./src",
    "pattern": ".ts",
    "excludePatterns": ["node_modules", "*.d.ts"]
  }
}

// Get file information
{
  "function": "get_file_info",
  "options": {
    "path": "./package.json"
  }
}
```

### Advanced File Reading
```json
// Read last 50 lines of log file
{
  "function": "read_file",
  "options": {
    "path": "./app.log",
    "tail": 50
  }
}

// Preview file edits
{
  "function": "edit_file",
  "options": {
    "path": "./config.js",
    "edits": [
      {
        "oldText": "debug: false",
        "newText": "debug: true"
      }
    ],
    "dryRun": true
  }
}
```

## Error Handling

The tool provides comprehensive error handling for:
- **Access Denied**: Operations outside allowed directories
- **File Not Found**: Missing files or directories
- **Permission Errors**: Insufficient file system permissions
- **Invalid Parameters**: Missing or malformed parameters
- **Symlink Security**: Symlinks pointing outside allowed directories

## Best Practices

1. **Use `dryRun` for edits**: Always preview file edits before applying
2. **Validate paths**: Check allowed directories before operations
3. **Handle errors gracefully**: All operations can fail, handle errors appropriately
4. **Use atomic operations**: The tool uses atomic writes for safety
5. **Respect security boundaries**: Don't attempt to bypass directory restrictions

## Dependencies

- **None**: Pure Go implementation with standard library
- **Cross-platform**: Works on macOS and Linux
- **Secure by default**: Directory access control enabled by default

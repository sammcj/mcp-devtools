# Replace in All Files Tool

The `replace_in_all_files` tool provides AI coding agents with efficient and safe text replacement across file trees. It enables exact string replacements across multiple files while respecting .gitignore patterns and handling binary files safely.

## Features

- **Exact String Matching**: Only replaces exact matches (no regex) for safety
- **Multiple Replacements**: Support for one or more replacement pairs in a single operation
- **Dry Run Mode**: Preview changes without modifying files
- **Parallel Processing**: Efficient processing using worker pools
- **File Filtering**: Respects .gitignore and excludes binary files automatically
- **Permission Checking**: Only operates on files with write permissions
- **Clear Reporting**: Detailed output showing what was changed

## Parameters

| Parameter             | Type    | Required | Description                                                       |
|-----------------------|---------|----------|-------------------------------------------------------------------|
| `path`                | string  | Yes      | Absolute path to directory or file to operate on                  |
| `replacement_pairs`   | array   | Yes      | Array of replacement pairs with `source` and `target` strings     |
| `dry_run`             | boolean | No       | If true, preview changes without modifying files (default: false) |
| `additional_excludes` | array   | No       | Additional glob patterns to exclude beyond default exclusions     |

### Replacement Pairs Format

Each replacement pair must contain:
- `source`: The exact text to find and replace
- `target`: The text to replace the source with

```json
{
  "source": "oldText",
  "target": "newText"
}
```

## Examples

### Basic Text Replacement

Replace all occurrences of "oldFunction" with "newFunction":

```json
{
  "path": "/Users/username/project",
  "replacement_pairs": [
    {
      "source": "oldFunction",
      "target": "newFunction"
    }
  ]
}
```

### Multiple Replacements

Perform multiple replacements in one operation:

```json
{
  "path": "/Users/username/project",
  "replacement_pairs": [
    {
      "source": "console.log",
      "target": "logger.info"
    },
    {
      "source": "var ",
      "target": "const "
    },
    {
      "source": "function ",
      "target": "const "
    }
  ]
}
```

### Dry Run Preview

Preview changes without modifying files:

```json
{
  "path": "/Users/username/project",
  "replacement_pairs": [
    {
      "source": "deprecated_api",
      "target": "new_api"
    }
  ],
  "dry_run": true
}
```

### Single File Replacement

Replace text in a specific file:

```json
{
  "path": "/Users/username/project/src/main.js",
  "replacement_pairs": [
    {
      "source": "localhost:3000",
      "target": "api.example.com"
    }
  ]
}
```

### Special Characters and Symbols

The tool safely handles special characters, quotes, and symbols:

```json
{
  "path": "/Users/username/project",
  "replacement_pairs": [
    {
      "source": "process.env['API_KEY']",
      "target": "process.env.API_KEY"
    },
    {
      "source": "\"use strict\";",
      "target": "'use strict';"
    },
    {
      "source": "${OLD_VAR}",
      "target": "${NEW_VAR}"
    }
  ]
}
```

## Response Format

The tool returns a structured response with:

```json
{
  "files_processed": [
    {
      "path": "./src/main.js",
      "replacement_count": {
        "oldFunction": 3,
        "oldVar": 1
      },
      "modified": true
    }
  ],
  "total_files": 15,
  "modified_files": 3,
  "total_scanned": 150,
  "skipped_files": ["./dist/bundle.js", "./node_modules/package/file.js"],
  "execution_time": "1.2s",
  "dry_run": false,
  "summary": "Successfully modified 3 files with 4 total replacements"
}
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REPLACE_FILES_MAX_WORKERS` | 4 | Maximum number of concurrent file processors |
| `REPLACE_FILES_MAX_SIZE_KB` | 2048 | Maximum file size to process (in KB) |

## File Exclusions

The tool automatically excludes:

### Binary Files
- Images (PNG, JPG, GIF, etc.)
- Videos and audio files
- Archives (ZIP, TAR, etc.)
- Executables and libraries
- Database files
- PDF documents

### Common Directories
- `node_modules/`
- `.git/`
- `vendor/`
- `build/`, `dist/`, `target/`
- Cache directories

### Gitignore Patterns
Files and directories listed in `.gitignore` are automatically excluded.

## Safety Features

1. **Exact Matching Only**: No regex interpretation - only literal string replacement
2. **Binary File Protection**: Automatically skips binary files
3. **Permission Checking**: Only operates on files with write permissions
4. **Dry Run Mode**: Test changes before applying them
5. **No Destructive Operations**: Preserves file permissions and structure

## Performance

- **Parallel Processing**: Processes multiple files concurrently
- **Memory Efficient**: Streams file content rather than loading everything into memory
- **Large File Support**: Handles files up to 2MB by default (configurable)
- **Fast Binary Detection**: Quick binary file identification to avoid processing

## Best Practices

1. **Always Test First**: Use dry run mode to preview changes
2. **Commit Before Running**: Ensure your changes are committed to version control
3. **Use Specific Patterns**: Be as specific as possible with source strings
4. **Check Results**: Review the output to ensure expected replacements
5. **Handle Special Cases**: Consider escaping special characters if needed

## Common Use Cases

- **API Migration**: Replace old API endpoints with new ones
- **Variable Renaming**: Rename variables across a codebase
- **Library Updates**: Update import statements or function calls
- **Configuration Changes**: Update configuration values across files
- **Code Modernisation**: Replace deprecated patterns with modern equivalents

## Limitations

- Performs literal string replacement (no regex support)
- No support for multi-line replacements spanning line breaks

## Error Handling

The tool gracefully handles:
- Permission errors (files are skipped)
- Binary files (automatically excluded)
- Large files (skipped if over size limit)
- Invalid paths (clear error messages)
- Concurrent access issues (worker pool coordination)

Files that cannot be processed are listed in the `skipped_files` array with the reason for skipping.

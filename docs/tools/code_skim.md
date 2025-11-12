# code_skim

Transform source code by removing implementation details whilst preserving structure. Achieves 60-80% token reduction for optimising AI context windows.

## Status

🔒 **Disabled by default** - Enable with `ENABLE_ADDITIONAL_TOOLS=code_skim`

## Overview

The `code_skim` tool uses tree-sitter to parse source code and strip function/method bodies whilst preserving signatures, types, and overall structure. Language is automatically detected from file extensions. Results are paginated to prevent overwhelming context windows.

**Supported languages:**
- Python (`.py`)
- Go (`.go`)
- JavaScript (`.js`, `.jsx`)
- TypeScript (`.ts`, `.tsx`)
- Rust (`.rs`)
- C (`.c`, `.h`)
- C++ (`.cpp`, `.cc`, `.cxx`, `.hpp`, `.hxx`, `.hh`)
- Bash (`.sh`, `.bash`)
- HTML (`.html`, `.htm`)
- CSS (`.css`)
- Swift (`.swift`)
- Java (`.java`)
- YAML (`.yml`, `.yaml`)
- HCL/Terraform (`.hcl`, `.tf`)

## Why Use code_skim?

When working with large codebases, you often don't need implementation details to understand architecture, APIs, or structure. The `code_skim` tool addresses the context attention problem:

- Large contexts degrade model performance (attention dilution)
- 80% of the time, you don't need implementation details
- Focus on *what* code does, not *how* it does it

**Token reduction example:**
- Original: 63,000 tokens
- Structure mode: 25,000 tokens (60% reduction)
- Fits more code in limited context windows

## Parameters

### Required

- `source` (array): Array of file paths, directory paths, or glob patterns
  - Single file: `["/path/to/file.py"]`
  - Directory: `["/path/to/directory"]` (recursively finds supported files)
  - Glob pattern: `["/path/to/**/*.py"]` (matches using glob syntax)
  - Multiple: `["/path/to/file1.py", "/path/to/file2.go", "/path/**/*.ts"]`
  - Multiple sources are automatically deduplicated

### Optional

- `clear_cache` (boolean): Clear cache entry before processing
  - Default: `false`
- `starting_line` (number): Line number to start from (1-based) for pagination
  - Use when previous response was truncated
  - Specified in `next_starting_line` field of truncated responses
- `filter` (array): Array of glob patterns to filter function/method/class names
  - Single pattern: `["handle_*"]`, `["test_*"]`, `["*Controller"]`
  - Multiple patterns: `["handle_*", "process_*", "get*"]`
  - Inverse filter (exclusion): Prefix with `!` (e.g., `["!temp_*"]`, `["!test_*"]`)
  - Combined: `["handle_*", "!handle_temp*"]` (include handle_* but exclude handle_temp*)
  - Exclusions take priority over inclusions
  - Returns `matched_items`, `total_items`, `filtered_items` counts in response

## How It Works

The tool removes function/method bodies whilst preserving:
- Function and method signatures
- Class declarations
- Type definitions
- Overall code structure

**Token reduction: 60-80%**

**Example:**
```python
# Before
def process_user(user):
    validated = validate_user(user)
    if not validated:
        raise ValueError("Invalid user")
    normalized = normalize_data(user)
    return save_to_database(normalized)

# After transformation
def process_user(user): { /* ... */ }
```

## Line Limiting

By default, results are limited to 10,000 lines per file to prevent overwhelming context windows. When results exceed this limit:

- Response includes `truncated: true`
- `total_lines` shows the full file line count
- `returned_lines` shows how many lines were returned
- `next_starting_line` specifies where to continue from

Configure the limit with the `CODE_SKIM_MAX_LINES` environment variable.

## Examples

### Transform a single file

```json
{
  "source": ["/path/to/src/api.py"]
}
```

### Transform all Python files in a directory

```json
{
  "source": ["/path/to/src"]
}
```

### Transform files matching a glob pattern

```json
{
  "source": ["/path/to/src/**/*.ts"]
}
```

### Clear cache and re-process

```json
{
  "source": ["/path/to/app.js"],
  "clear_cache": true
}
```

### Paginate through a large file

```json
{
  "source": ["/path/to/large_file.py"],
  "starting_line": 10001
}
```

### Filter by function name pattern

```json
{
  "source": ["/path/to/api.py"],
  "filter": ["handle_*"]
}
```

### Show only test functions

```json
{
  "source": ["/path/to/tests.py"],
  "filter": ["test_*"]
}
```

### Multiple source files

```json
{
  "source": [
    "/path/to/api.py",
    "/path/to/handlers.py",
    "/path/to/models.py"
  ]
}
```

### Multiple filter patterns

```json
{
  "source": ["/path/to/api.py"],
  "filter": ["handle_*", "process_*", "validate_*"]
}
```

### Exclude specific patterns (inverse filter)

```json
{
  "source": ["/path/to/api.py"],
  "filter": ["handle_*", "!handle_temp*"]
}
```

### Show everything except test functions

```json
{
  "source": ["/path/to/src"],
  "filter": ["!test_*"]
}
```

## Response Format

### Single File

```json
{
  "files": [
    {
      "path": "/path/to/api.py",
      "transformed": "def hello(name): { /* ... */ }",
      "language": "python",
      "from_cache": false,
      "truncated": false,
      "total_lines": 8,
      "returned_lines": 8,
      "reduction_percentage": 65
    }
  ],
  "total_files": 1,
  "processed_files": 1,
  "failed_files": 0,
  "processing_time_ms": 15
}
```

### With Filtering

```json
{
  "files": [
    {
      "path": "/path/to/api.py",
      "transformed": "def handle_request(): { /* ... */ }\ndef handle_response(): { /* ... */ }",
      "language": "python",
      "from_cache": false,
      "truncated": false,
      "total_lines": 4,
      "returned_lines": 4,
      "reduction_percentage": 75,
      "matched_items": 2,
      "total_items": 10,
      "filtered_items": 8
    }
  ],
  "total_files": 1,
  "processed_files": 1,
  "failed_files": 0,
  "processing_time_ms": 18
}
```

### Truncated Response (Pagination)

```json
{
  "files": [
    {
      "path": "/path/to/large_file.py",
      "transformed": "...first 10,000 lines...",
      "language": "python",
      "from_cache": false,
      "truncated": true,
      "total_lines": 25000,
      "returned_lines": 10000,
      "next_starting_line": 10001
    }
  ],
  "total_files": 1,
  "processed_files": 1,
  "failed_files": 0
}
```

**Response Fields:**
- `files`: Array of file results
  - `path`: Absolute file path
  - `transformed`: Transformed source code
  - `language`: Detected language
  - `from_cache`: Whether result came from cache
  - `truncated`: Whether output was truncated due to line limit
  - `total_lines`: Total line count of transformed output
  - `returned_lines`: Number of lines returned in this response
  - `next_starting_line`: Line number to use for next request (if truncated)
  - `reduction_percentage`: Percentage of token/character reduction from original (0-100)
  - `matched_items`: Number of functions/methods/classes that matched filter (only when filtering)
  - `total_items`: Total number of functions/methods/classes found (only when filtering)
  - `filtered_items`: Number of functions/methods/classes excluded by filter (only when filtering)
  - `error`: Error message (if file processing failed)
- `total_files`: Total number of files found
- `processed_files`: Number of successfully processed files
- `failed_files`: Number of files that failed processing
- `processing_time_ms`: Total processing time in milliseconds

## Caching

Results are cached using a key based on:
- File path
- Language
- Filter patterns (if applied)
- Source code hash (SHA256)

**Cache behaviour:**
- First call: Processes and caches result (`from_cache: false`)
- Subsequent calls: Returns cached result if file content unchanged (`from_cache: true`)
- Clear cache: Set `clear_cache: true` to force re-processing
- Each file in batch operations is cached independently
- Pagination: Cached transformed output is reused for different line ranges
- Different filter patterns create separate cache entries

## Use Cases

### 1. Codebase Overview
Quickly understand code structure without implementation noise:
```json
{
  "source": "/path/to/src"
}
```

### 2. API Documentation
Extract function signatures for documentation:
```json
{
  "source": "/path/to/api.py"
}
```

### 3. Architecture Analysis
Analyse entire packages or modules:
```json
{
  "source": "/path/to/project/**/*.go"
}
```

### 4. Context Window Optimisation
Fit more code into limited AI context windows by removing implementation noise.

## When to Use

✅ **Use when:**
- Analysing code structure without implementation details
- Fitting large codebases into limited AI context windows
- Providing architectural overviews
- Examining API surfaces and function signatures
- Understanding "what" code does without the "how" details

❌ **Don't use when:**
- Debugging implementation logic
- Examining algorithm details
- Reviewing line-by-line code quality
- Actual implementation is required for the task
- Working with unsupported languages

## Troubleshooting

### File not found or access denied
**Problem:** Error about file not found or access denied

**Solution:** Ensure the file path is absolute and exists. Check that the security configuration allows access to the file location.

### No files match glob pattern
**Problem:** Error when using glob patterns

**Solution:** Verify the glob pattern is correct and matches existing files. Use `**/*.py` for recursive matching.

### Language detection failed
**Problem:** Error about unsupported file extension or language

**Solution:** Ensure files have supported extensions. See the full list of supported languages and extensions in the Overview section.

### Transformation failed with parse error
**Problem:** Tree-sitter parser error

**Solution:** Ensure source code is syntactically valid for the specified language. Tree-sitter requires valid syntax to parse.

### Cache returning stale results
**Problem:** Getting old transformation when source has changed

**Solution:** Set `clear_cache: true` to force re-processing. Cache uses file content hash, so changes are automatically detected.

### Token reduction lower than expected
**Problem:** Reduction percentage is much lower than 60-80%

**Solution:** Structure mode targets 60-80% reduction. Low reduction may indicate minimal function bodies in source code (e.g., mostly declarations or empty functions).

### File too large error
**Problem:** Individual file exceeds 500KB size limit

**Solution:** The tool limits individual file sizes to 500KB to prevent memory exhaustion. Consider splitting large files, or if the file is genuinely needed, process it in smaller chunks or use alternative tools.

### Memory limit exceeded error
**Problem:** Total memory usage would exceed 4GB limit

**Solution:** The tool limits total memory to 4GB across all files being processed. Process fewer files at once, use more specific glob patterns to target subsets, or process files in batches sequentially.

## Memory and Resource Limits

To ensure safe operation and prevent resource exhaustion:

- **Maximum file size**: 500KB per individual file
- **Maximum total memory**: 4GB across all files being processed
- **Maximum AST depth**: 500 levels (prevents stack overflow)
- **Maximum AST nodes**: 100,000 per file (prevents memory exhaustion)
- **Parallel workers**: Up to 10 concurrent file processors

Files exceeding these limits are skipped with detailed error messages in the response.

## Implementation Details

- Built on [go-tree-sitter](https://github.com/smacker/go-tree-sitter)
- Uses tree-sitter parsers for accurate AST analysis
- Parallel processing with worker pool (up to 10 workers)
- In-memory caching with SHA256 hashing for performance
- File access controlled by security integration
- Batch processing for directories and glob patterns using [doublestar](https://github.com/bmatcuk/doublestar)
- Memory-safe with configurable limits

## Related Tools

- `find_long_files`: Identify large files that may benefit from skimming
- `get_library_documentation`: Get focused library documentation
- `fetch_url`: Fetch web content (can be combined with skimming)

## Extended Help

Use the `get_tool_help` tool to access detailed usage information:

```json
{
  "tool_name": "code_skim"
}
```

This provides:
- Detailed examples for all languages
- Common usage patterns
- Troubleshooting tips
- Parameter explanations
- When to use / when not to use guidance

# Find Long Files Tool

The Find Long Files tool efficiently identifies code files that exceed a specified line count threshold, helping AI coding agents to identify files that may need refactoring for effective reading, editing and understanding.

## Overview

This tool scans a project directory and returns a formatted checklist of files that are longer than a specified threshold (default: 700 lines). It's designed to help maintain code quality by identifying files that may have grown too large and need to be split into smaller, more manageable components.

## Features

- **Fast Line Counting**: Uses optimised buffer-based counting for superior performance
- **Comprehensive Exclusions**: Automatically excludes binary files, build artifacts, and common non-code files
- **Gitignore Respect**: Automatically respects `.gitignore` patterns in the project
- **Permission Handling**: Gracefully handles permission errors without failing
- **Environment Variables**: Configurable via environment variables for automation
- **Formatted Output**: Returns a clean checklist with line counts, file sizes, and execution timing

## Usage Examples

While intended to be activated via a prompt to an agent, below are some example JSON tool calls.

### Basic Project Scan
```json
{
  "name": "find_long_files",
  "arguments": {
    "path": "/Users/username/git/my-project"
  }
}
```

### Custom Line Threshold
```json
{
  "name": "find_long_files",
  "arguments": {
    "path": "/Users/username/git/my-project",
    "line_threshold": 500
  }
}
```

### Additional File Exclusions
```json
{
  "name": "find_long_files",
  "arguments": {
    "path": "/Users/username/git/my-project",
    "line_threshold": 800,
    "additional_excludes": ["**/*.generated.js", "**/migrations/**"]
  }
}
```

## Parameters

| Parameter             | Type   | Required | Default | Description                                                             |
|-----------------------|--------|----------|---------|-------------------------------------------------------------------------|
| `path`                | string | Yes      | -       | Absolute directory path to search (e.g., '/Users/username/git/project') |
| `line_threshold`      | number | No       | 700     | Minimum number of lines to consider a file 'long'                       |
| `additional_excludes` | array  | No       | []      | Additional glob patterns to exclude beyond .gitignore                   |

## Environment Variables

| Variable                              | Description                                          | Example                     |
|---------------------------------------|------------------------------------------------------|-----------------------------|
| `LONG_FILES_DEFAULT_LENGTH`           | Override default line threshold                      | `1000`                      |
| `LONG_FILES_ADDITIONAL_EXCLUDES`      | Comma-separated additional exclusion patterns        | `**/*.test.js,**/*.spec.js` |
| `LONG_FILES_RETURN_PROMPT`            | Custom message returned with the checklist (set to empty string `""` to disable) | `Custom instructions...`    |
| `LONG_FILES_SORT_BY_DIRECTORY_TOTALS` | Sort by directory totals instead of individual files | `true`                      |

## Default Exclusions

The tool automatically excludes many file types and directories:

### Binary and Document Files
- Office documents: `**/*.docx`, `**/*.pdf`, `**/*.pptx`, etc.
- Images: `**/*.png`, `**/*.jpg`, `**/*.gif`, etc.
- Archives: `**/*.zip`, `**/*.tar`, `**/*.gz`, etc.
- Executables: `**/*.exe`, `**/*.dll`, `**/*.bin`, etc.

### Development Artifacts
- Build directories: `**/build/**`, `**/dist/**`, `**/target/**`
- Dependencies: `**/node_modules/**`, `**/vendor/**`
- Caches: `**/.cache/**`, `**/__pycache__/**`
- Version control: `**/.git/**`, `**/.svn/**`

### Temporary and System Files
- Logs: `**/*.log`, `**/*.out.*`
- Temporary: `**/*.tmp`, `**/*.bak`, `**/*.swp`
- System: `**/.DS_Store`, `**/Thumbs.db`

## Output Format

The tool returns a formatted checklist with:

```markdown
# Checklist of files over 700 lines

Last checked: 2025-07-29 09:29:46
Calculated in: 2.4s

- [ ] `./backend/main.py`: 1500 Lines, 108KB
- [ ] `./frontend/components/AgentNetwork.tsx`: 1492 Lines, 95KB
- [ ] `./backend/agents/orchestrator_agent.py`: 1248 Lines, 87KB

Next Steps (Unless the user has instructed you otherwise):

1. Please take this checklist and save it to a temporary location.
2. Perform a quick review of the identified files and under each checklist item adds a concise (1-2 sentences) summary of what should be done to reduce the file length - for example the best solution might be to split the file up into multiple files and if so what sort of pattern or logic should be used to decide what goes where ensuring your strategy values concise, clean, efficient code and operations.
3. Then stop and ask the user to review.

## Performance

The tool uses optimised algorithms for fast execution:
- Buffer-based line counting (4.5x faster than traditional scanning)
- 32KB read buffers for optimal I/O performance
- Efficient binary file detection
- Permission-aware directory traversal

Typical performance: 1-4s for medium-sized projects.

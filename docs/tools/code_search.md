# code_search

Semantic code search using local embeddings. Find functions, classes, and methods by describing what they do in natural language.

## Overview

`code_search` enables semantic search over codebases by:
1. Indexing code signatures (functions, classes, methods) using the `code_skim` AST parser
2. Generating embeddings locally using the all-MiniLM-L6-v2 model via hugot
3. Storing vectors in chromem-go with persistence
4. Searching by natural language query

## First-Time Setup

On first use, the tool downloads:
- **Embedding model** (~90MB) from Hugging Face to `~/.mcp-devtools/models/`

The binary size increase is minimal (~6MB) as heavy dependencies are downloaded on demand.

## Actions

### index

Index a codebase for semantic search.

```json
{
  "action": "index",
  "source": ["/path/to/project"]
}
```

**Response:**
```json
{
  "indexed_files": 298,
  "indexed_items": 1217
}
```

### search

Find code by natural language description.

```json
{
  "action": "search",
  "query": "function that handles HTTP requests",
  "limit": 5
}
```

**Response:**
```json
{
  "results": [
    {
      "path": "/project/internal/oauth/validation/validator.go",
      "name": "isLocalhostRequest",
      "type": "function",
      "signature": "func isLocalhostRequest(r *http.Request) bool",
      "similarity": 0.47,
      "line": 272
    },
    {
      "path": "/project/internal/oauth/server/server.go",
      "name": "RequireScope",
      "type": "function",
      "signature": "func RequireScope(scope string) func(http.Handler) http.Handler",
      "similarity": 0.46,
      "line": 232
    },
    {
      "path": "/project/internal/tools/packageversions/utils.go",
      "name": "MakeRequest",
      "type": "function",
      "signature": "func MakeRequest(client HTTPClient, method, url string, headers map[string]string) ([]byte, error)",
      "similarity": 0.44,
      "line": 99
    }
  ],
  "total_indexed": 1217
}
```

### status

Check index status.

```json
{
  "action": "status"
}
```

**Response:**
```json
{
  "indexed": true,
  "total_files": 251,
  "total_items": 1217,
  "model_loaded": true,
  "runtime_loaded": true,
  "runtime_version": "gomlx"
}
```

### clear

Clear the index (optionally for specific paths).

```json
{
  "action": "clear",
  "source": ["/path/to/project"]
}
```

## Parameters

| Parameter   | Type   | Required   | Default | Description                                  |
|-------------|--------|------------|---------|----------------------------------------------|
| `action`    | string | Yes        | -       | One of: `search`, `index`, `status`, `clear` |
| `source`    | array  | For index  | -       | Paths to index or filter                     |
| `query`     | string | For search | -       | Natural language search query                |
| `limit`     | number | No         | 10      | Maximum results to return                    |
| `threshold` | number | No         | 0.3     | Minimum similarity score (0-1)               |

## Example Queries

The tool matches natural language descriptions to function signatures semantically:

| Query                                   | Typical matches                                      |
|-----------------------------------------|------------------------------------------------------|
| `"function that handles HTTP requests"` | `MakeRequest`, `isLocalhostRequest`, HTTP middleware |
| `"parse JSON configuration"`            | Config parsers, JSON unmarshalers                    |
| `"validate user input"`                 | Input validators, sanitisers                         |
| `"retry with backoff"`                  | Retry helpers, exponential backoff implementations   |
| `"create database connection"`          | DB initialisers, connection pool factories           |
| `"hash password"`                       | Password hashers, bcrypt wrappers                    |
| `"send email notification"`             | Email senders, notification handlers                 |

**Tips for effective queries:**
- Describe *what the code does*, not what it's called
- Include domain terms (e.g., "HTTP", "database", "authentication")
- Be specific about the operation (e.g., "validate" vs "process")

## Supported Languages

Indexes the same languages as `code_skim`:
- Go, Python, JavaScript/TypeScript, Rust, Java, Swift, C/C++

## Storage

- **Index location:** `~/.mcp-devtools/embeddings/`
- **Model location:** `~/.mcp-devtools/models/`
- **File tracking:** `~/.mcp-devtools/embeddings/file_tracker.json`
- Incremental indexing: skips already-indexed files

## Automatic Stale File Detection

By default, the index is not automatically updated when files change. To enable automatic reindexing of modified files before each search, set the `CODE_SEARCH_STALE_THRESHOLD` environment variable:

```bash
CODE_SEARCH_STALE_THRESHOLD=30s  # Reindex files modified more than 30 seconds ago
CODE_SEARCH_STALE_THRESHOLD=1m   # Reindex files modified more than 1 minute ago
CODE_SEARCH_STALE_THRESHOLD=5m   # Reindex files modified more than 5 minutes ago
```

**How it works:**
1. Before each search, checks if any indexed files have been modified since indexing
2. If a file was modified AND the modification is older than the threshold, it's reindexed
3. The threshold prevents reindexing files during active editing sessions
4. Reindexing happens transparently - the search response is unchanged

**When disabled (default):** No automatic reindexing occurs. Use `clear` + `index` to manually refresh the index.

## Example Workflow

```bash
# 1. Index your project
{"action": "index", "source": ["/path/to/myproject"]}

# 2. Search for relevant code
{"action": "search", "query": "parse JSON configuration file"}

# 3. Check what's indexed
{"action": "status"}

# 4. Re-index after changes (clears and rebuilds)
{"action": "clear"}
{"action": "index", "source": ["/path/to/myproject"]}
```

## When to Use

- Discovering unfamiliar codebases
- Finding code when you know *what* it does but not *what it's called*
- Exploring implementations of concepts (e.g., "retry with exponential backoff")

## When Not to Use

- Exact name lookups: use `grep` or `glob` instead
- Finding specific syntax: use regex search
- Small codebases where manual exploration is faster

## Build Requirements

Requires CGO and is available on:
- macOS (darwin)
- Linux (amd64)

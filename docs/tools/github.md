# GitHub Tool

Access GitHub repositories and data with comprehensive read-only operations including searching, retrieving file contents, cloning repositories, and monitoring GitHub Actions workflows.

This was added as the official Github MCP server fills the context with many tools, tool parameters and returns information that's not overly useful.

## Features

- **Repository Search**: Find GitHub repositories by name, description, or other criteria
- **Issue Management**: Search and retrieve issue details with optional comments
- **Pull Request Access**: Search and get PR information including comments
- **File Content Retrieval**: Access file contents from repositories with branch/tag support
- **Repository Cloning**: Clone repositories locally with authentication support
- **GitHub Actions**: Monitor workflow runs and retrieve logs
- **Flexible Authentication**: Support for token-based and SSH key authentication

## Authentication

The GitHub tool supports multiple authentication methods:

### 1. GitHub Personal Access Token (Recommended)
Set the `GITHUB_TOKEN` environment variable with your personal access token:

```bash
export GITHUB_TOKEN="ghp_your_token_here"
```

**Token Permissions**: For read-only operations, create a token with minimal permissions:
- `public_repo` (for public repositories)
- `repo` (for private repositories you have access to)

### 2. SSH Key Authentication
Set `GITHUB_AUTH_METHOD=ssh` to use SSH keys for git operations:

```bash
export GITHUB_AUTH_METHOD=ssh
```

**SSH Key Detection**: Automatically detects SSH keys in this order:
1. `GITHUB_SSH_PRIVATE_KEY_PATH` environment variable
2. `~/.ssh/id_ed25519` (preferred)
3. `~/.ssh/id_rsa` (fallback)

### 3. No Authentication
Works with public repositories without any authentication setup.

## Functions

### search_repositories
Search for GitHub repositories by name, description, or other criteria.

**Parameters**:
- `function`: `"search_repositories"`
- `options.query`: Search query string (required)
- `options.limit`: Maximum results (default: 30, max: 100)

**Example**:
```json
{
  "function": "search_repositories",
  "options": {
    "query": "machine learning python",
    "limit": 10
  }
}
```

### search_issues
Search for issues within a specific repository.

**Parameters**:
- `function`: `"search_issues"`
- `repository`: Repository identifier (owner/repo or GitHub URL)
- `options.query`: Search query (optional)
- `options.limit`: Maximum results (default: 30, max: 100)

**Example**:
```json
{
  "function": "search_issues",
  "repository": "microsoft/vscode",
  "options": {
    "query": "bug label:bug",
    "limit": 20
  }
}
```

### search_pull_requests
Search for pull requests within a specific repository.

**Parameters**:
- `function`: `"search_pull_requests"`
- `repository`: Repository identifier (owner/repo or GitHub URL)
- `options.query`: Search query (optional)
- `options.limit`: Maximum results (default: 30, max: 100)

**Example**:
```json
{
  "function": "search_pull_requests",
  "repository": "microsoft/vscode",
  "options": {
    "query": "is:open",
    "limit": 15
  }
}
```

### get_issue
Retrieve detailed information about a specific issue.

**Parameters**:
- `function`: `"get_issue"`
- `repository`: Repository identifier OR full issue URL
- `options.number`: Issue number (required if not in URL)
- `options.include_comments`: Include comments (default: false)

**Examples**:
```json
{
  "function": "get_issue",
  "repository": "https://github.com/microsoft/vscode/issues/123",
  "options": {
    "include_comments": true
  }
}
```

```json
{
  "function": "get_issue",
  "repository": "microsoft/vscode",
  "options": {
    "number": 123,
    "include_comments": true
  }
}
```

### get_pull_request
Retrieve detailed information about a specific pull request.

**Parameters**:
- `function`: `"get_pull_request"`
- `repository`: Repository identifier OR full PR URL
- `options.number`: PR number (required if not in URL)
- `options.include_comments`: Include comments (default: false)

**Examples**:
```json
{
  "function": "get_pull_request",
  "repository": "https://github.com/microsoft/vscode/pull/456",
  "options": {
    "include_comments": true
  }
}
```

### get_file_contents
Retrieve the contents of one or more files from a repository.

**Parameters**:
- `function`: `"get_file_contents"`
- `repository`: Repository identifier (owner/repo or GitHub URL)
- `options.paths`: Array of file paths (required)
- `options.ref`: Git reference - branch, tag, or commit (optional)

**Example**:
```json
{
  "function": "get_file_contents",
  "repository": "microsoft/vscode",
  "options": {
    "paths": ["package.json", "README.md"],
    "ref": "main"
  }
}
```

### clone_repository
Clone a GitHub repository to a local directory.

**Parameters**:
- `function`: `"clone_repository"`
- `repository`: Repository identifier (owner/repo or GitHub URL)
- `options.local_path`: Local directory path (optional, defaults to `~/github-repos/{repo}`)

**Example**:
```json
{
  "function": "clone_repository",
  "repository": "microsoft/vscode",
  "options": {
    "local_path": "/Users/username/projects/vscode"
  }
}
```

### get_workflow_run
Retrieve GitHub Actions workflow run status and optionally logs.

**Parameters**:
- `function`: `"get_workflow_run"`
- `repository`: Repository identifier OR full workflow run URL
- `options.run_id`: Workflow run ID (required if not in URL)
- `options.include_logs`: Include workflow logs (default: false)

**Examples**:
```json
{
  "function": "get_workflow_run",
  "repository": "https://github.com/microsoft/vscode/actions/runs/123456789",
  "options": {
    "include_logs": true
  }
}
```

## Repository Identifier Formats

The `repository` parameter accepts multiple formats:

- **owner/repo**: `microsoft/vscode`
- **GitHub URL**: `https://github.com/microsoft/vscode`
- **GitHub URL with .git**: `https://github.com/microsoft/vscode.git`
- **Issue URL**: `https://github.com/microsoft/vscode/issues/123`
- **PR URL**: `https://github.com/microsoft/vscode/pull/456`
- **Workflow Run URL**: `https://github.com/microsoft/vscode/actions/runs/123456789`

## Security Considerations

- **Read-Only Operations**: All functions are read-only and cannot modify repositories
- **Token Scoping**: Use minimal required permissions for GitHub tokens
- **SSH Key Security**: Ensure SSH keys are properly secured and use strong key types (ed25519 preferred)
- **Input Validation**: All repository identifiers and parameters are validated before processing
- **Content Filtering**: Large files and binary content are automatically filtered to prevent memory issues

## Rate Limiting

GitHub API has rate limits:
- **Authenticated requests**: 5,000 requests per hour
- **Unauthenticated requests**: 60 requests per hour

The tool respects these limits and provides helpful error messages when limits are exceeded.

## Error Handling

Common error scenarios and solutions:

- **Repository not found**: Verify repository name and access permissions
- **Authentication failed**: Check token validity and permissions
- **Rate limit exceeded**: Wait for rate limit reset or use authentication
- **File not found**: Verify file path and repository reference
- **SSH key not found**: Ensure SSH keys exist and are properly configured

## Examples

### Basic Repository Search
```json
{
  "function": "search_repositories",
  "options": {
    "query": "language:go topic:cli",
    "limit": 10
  }
}
```

### Get Issue with Comments
```json
{
  "function": "get_issue",
  "repository": "https://github.com/sammcj/mcp-devtools/issues/1",
  "options": {
    "include_comments": true
  }
}
```

### Retrieve Multiple Files
```json
{
  "function": "get_file_contents",
  "repository": "sammcj/mcp-devtools",
  "options": {
    "paths": ["go.mod", "main.go", "README.md"],
    "ref": "main"
  }
}
```

### Clone with SSH
```bash
export GITHUB_AUTH_METHOD=ssh
```
```json
{
  "function": "clone_repository",
  "repository": "sammcj/mcp-devtools",
  "options": {
    "local_path": "/tmp/mcp-devtools"
  }
}
```

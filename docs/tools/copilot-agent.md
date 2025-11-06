# GitHub Copilot Agent Tool

The GitHub Copilot Agent tool provides integration with GitHub Copilot CLI through the MCP (Model Context Protocol) server. This tool enables AI agents to leverage Copilot's capabilities for code analysis, generation, and assistance.

## Overview

GitHub Copilot is an AI-powered code assistant that provides code completions, explanations, and solutions. The Copilot Agent tool allows MCP clients to interact with Copilot CLI in non-interactive mode, enabling automated code assistance workflows.

## Configuration

### Requirements

- GitHub Copilot CLI must be installed and available in PATH
- GitHub CLI (`gh`) must be authenticated
- Active GitHub Copilot subscription

### Environment Variables

- `ENABLE_ADDITIONAL_TOOLS`: Must include `copilot-agent` to enable the tool
- `AGENT_TIMEOUT`: Timeout for Copilot operations in seconds (default: 180)
- `AGENT_MAX_RESPONSE_SIZE`: Maximum response size in bytes (default: 2MB)
- `AGENT_PERMISSIONS_MODE`: Controls yolo mode behaviour. Options: `default` (agent can control via parameter), `enabled`/`true`/`yolo` (force on, hide parameter), `disabled`/`false` (force off, hide parameter). Defaults to `default`

### Security

The tool is disabled by default for security reasons. It must be explicitly enabled by setting:

```bash
export ENABLE_ADDITIONAL_TOOLS="copilot-agent"
```

### Authentication Setup

Before using the Copilot agent tool, authenticate with GitHub:

```bash
gh auth login
```

Follow the prompts to authenticate. Ensure you have an active GitHub Copilot subscription.

## Usage

### Basic Parameters

- **prompt** (required): The instruction or question for Copilot
- **override-model** (optional): Specify model to use (passed directly to Copilot)
- **resume** (optional): Continue the most recent session
- **session-id** (optional): Resume a specific session by ID (takes priority over resume)
- **yolo-mode** (optional): Trust all tools without confirmation
- **allow-tool** (optional): Array of specific tool permission patterns to allow
- **deny-tool** (optional): Array of specific tool permission patterns to deny
- **include-directories** (optional): Array of additional directories to grant access to
- **disable-mcp-server** (optional): Array of MCP server names to disable

### Examples

#### Basic Code Analysis
```json
{
  "name": "copilot-agent",
  "arguments": {
    "prompt": "Review this function for security best practices and suggest improvements"
  }
}
```

#### Continue Previous Conversation
```json
{
  "name": "copilot-agent",
  "arguments": {
    "prompt": "Now implement the caching mechanism we discussed",
    "resume": true
  }
}
```

#### Resume Specific Session
```json
{
  "name": "copilot-agent",
  "arguments": {
    "prompt": "Continue working on the authentication module",
    "session-id": "abc123def456"
  }
}
```

#### Specify Model and Enable Tools
```json
{
  "name": "copilot-agent",
  "arguments": {
    "prompt": "Help me troubleshoot this API error",
    "override-model": "gpt-5",
    "yolo-mode": true
  }
}
```

#### Grant Specific Tool Permissions
```json
{
  "name": "copilot-agent",
  "arguments": {
    "prompt": "Analyse the codebase and run tests to verify functionality",
    "allow-tool": ["shell(npm test)", "shell(go test)"]
  }
}
```

#### Include Additional Directories
```json
{
  "name": "copilot-agent",
  "arguments": {
    "prompt": "Review the shared utilities and frontend components",
    "include-directories": ["/path/to/shared", "/path/to/frontend"]
  }
}
```

## Features

### Session Management
- Continue most recent conversation using `resume` parameter
- Resume specific sessions by ID using `session-id` parameter
- Session-id takes priority when both are provided

### Model Selection
- Support for multiple AI models
- Default uses Copilot's configured default model
- Override with specific model versions using `override-model`

### Permission Management
- Automatic tool approval with `yolo-mode`
- Selective tool trust with `allow-tool` array
- Tool denial with `deny-tool` array
- Permission patterns passed directly to Copilot (e.g., `shell(git:*)`)

### Directory Access Control
- Grant access to additional directories outside the project
- No path validation - intentionally allows access beyond normal boundaries
- Useful for cross-project analysis and multi-repository work

### MCP Server Configuration
- Disable specific MCP servers to avoid conflicts
- Useful when certain tools should not be available to Copilot

### Response Management
- Configurable response size limits
- Intelligent truncation at line boundaries
- Timeout handling with partial output preservation
- Automatic filtering of progress indicators and usage statistics

## Common Use Cases

### Code Review and Analysis
- Security vulnerability identification
- Performance optimisation suggestions
- Best practices guidance
- Code quality improvements

### Code Generation
- Function and class implementation
- API handler generation
- Test case creation
- Documentation generation

### Debugging and Troubleshooting
- Error analysis and diagnosis
- Stack trace interpretation
- Configuration issue debugging
- Dependency conflict resolution

### Refactoring
- Code structure improvements
- Pattern application
- Legacy code modernisation
- Architecture enhancements

## Error Handling

### Common Error Scenarios

#### Tool Not Enabled
```
Error: copilot agent tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'copilot-agent'
```

#### CLI Not Found
```
Error: copilot CLI not found. Please install Copilot CLI and ensure it's available in your PATH
```

#### Authentication Issues
```
Error: copilot authentication failed. Please ensure you are authenticated. Error: [details]
```

#### Session Not Found
```
Error: session not found
Solution: Verify the session ID is correct or use 'resume: true' for most recent session
```

## Performance Considerations

### Timeouts
- Default timeout: 180 seconds (3 minutes)
- Configurable via `AGENT_TIMEOUT` environment variable
- Operations exceeding timeout return partial output with notification

### Response Size Limits
- Default limit: 2MB
- Configurable via `AGENT_MAX_RESPONSE_SIZE` environment variable
- Large responses are truncated at line boundaries with size information

### Output Filtering
- Progress indicators (●, ✓, ✗, ↪) are automatically filtered
- Command execution traces (lines starting with $) are removed
- Usage statistics sections are excluded
- Multiple consecutive empty lines are collapsed

## Security Considerations

### Access Control
- Explicit enablement required via environment variable
- No pre-flight CLI installation verification
- User responsible for GitHub authentication
- File system permissions are still enforced by the OS

### Permission Management
- `yolo-mode` grants permission to execute all tools without confirmation
- Use `allow-tool` and `deny-tool` for granular control
- Permission patterns passed directly to Copilot without validation
- Security implications should be carefully considered

### Directory Access
- `include-directories` allows access outside normal project boundaries
- No path validation or boundary restrictions
- Intentional design to enable cross-project work
- User responsible for preventing access to sensitive directories

### Command Safety
- Uses `exec.CommandContext` with separate arguments (no shell interpretation)
- Input validation prevents command injection
- Always includes `--no-color` flag for clean output
- Timeout handling prevents resource locks

## Troubleshooting

### Installation Issues
1. Verify Copilot CLI installation: `which copilot`
2. Check GitHub CLI authentication: `gh auth status`
3. Ensure active Copilot subscription

### Permission Problems
1. Ensure `ENABLE_ADDITIONAL_TOOLS` includes `copilot-agent`
2. Check GitHub authentication status
3. Verify file permissions for session storage

### Performance Issues
1. Adjust `AGENT_TIMEOUT` for longer operations
2. Increase `AGENT_MAX_RESPONSE_SIZE` for large outputs
3. Use `allow-tool` or `yolo-mode` to avoid approval prompts
4. Break down complex requests into smaller tasks

### Session Management Issues
1. Use `resume: true` for most recent session
2. Verify session-id format and validity
3. Check session storage directory permissions
4. Sessions may expire after some time

## Comparison with Other Agents

| Feature | Copilot Agent | Q Developer | Claude Agent | Gemini Agent |
|---------|---------------|-------------|--------------|--------------|
| Focus | General Purpose | AWS/Cloud | General Purpose | General Purpose |
| File Context | No @ syntax | No @ syntax | @ syntax supported | @ syntax supported |
| Models | Various | Claude only | Claude models | Gemini models |
| Session Management | Session ID based | Directory-based | Session ID based | Include files |
| Tool Trust | Granular control | Granular control | All or nothing | All or nothing |
| Directory Access | Flexible | No | Flexible | No |
| MCP Server Control | Yes | No | No | No |

## Related Documentation

- [GitHub Copilot Documentation](https://docs.github.com/en/copilot)
- [GitHub CLI Authentication](https://cli.github.com/manual/gh_auth_login)
- [Copilot CLI Reference](https://docs.github.com/en/copilot/using-github-copilot/using-github-copilot-in-the-command-line)

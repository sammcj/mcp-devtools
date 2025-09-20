# Codex Agent Tool

The `codex-agent` tool provides integration with the Codex CLI through MCP. It enables AI agents to leverage Codex's capabilities for code analysis, generation, and assistance. The tool exclusively uses the `codex exec` command for non-interactive execution.

Requires the [Codex CLI](https://codex.ai) to be installed and authenticated.

## Parameters

- `prompt` (string, required): A clear, concise prompt to send to Codex CLI to instruct the AI Agent to perform a specific task. Supports @file, @directory/ syntax for file references.
- `override-model` (string, optional): Model to override the default. No default model is provided, allowing Codex to use user's configured default.
- `sandbox` (string, optional): Sandbox policy for execution security. Options: read-only, workspace-write, danger-full-access. WARNING: Controls execution security context.
- `full-auto` (boolean, optional): Enable low-friction sandboxed automatic execution. In exec mode, implies --sandbox workspace-write. Defaults to `false`.
- `yolo-mode` (boolean, optional): DANGER: Bypass all approvals and sandbox restrictions (maps to --dangerously-bypass-approvals-and-sandbox). Use with extreme caution. Defaults to `false`.
- `resume` (boolean, optional): Continue the most recent session using --last flag. Defaults to `false`.
- `session-id` (string, optional): Specify a session identifier for resuming specific sessions.
- `profile` (string, optional): Configuration profile to use from config.toml.
- `config` (array of strings, optional): Configuration overrides in key=value format. Supports dotted path notation for nested values with JSON parsing.
- `images` (array of strings, optional): Image files to attach to the prompt. Multiple files supported.
- `cd` (string, optional): Working directory for Codex execution. Directory must exist.
- `skip-git-repo-check` (boolean, optional): Skip git repository validation checks. Defaults to `false`.

## Environment Variables

- `ENABLE_ADDITIONAL_TOOLS`: A comma-separated list of additional tools to enable. Must include `codex-agent` to use this tool.
- `AGENT_TIMEOUT`: The timeout in seconds for the `codex` command. Defaults to `180`.
- `AGENT_MAX_RESPONSE_SIZE`: Maximum response size in bytes. Defaults to `2097152` (2MB).

## Security Features

- **Response Size Limits**: Configurable maximum response size prevents excessive memory usage and potential DoS conditions
- **Input Validation**: Comprehensive parameter validation and type checking
- **Process Isolation**: Agent execution runs in isolated subprocess with proper timeout controls
- **Timeout Controls**: Configurable timeout limits prevent runaway processes
- **Error Handling**: Secure error handling that doesn't expose sensitive system information
- **Sandbox Support**: Full support for Codex's sandbox policies for execution security
- **Directory Validation**: Validates working directory exists before execution

## Examples

### Basic Code Analysis
```json
{
  "prompt": "Please review this function for efficiency and suggest optimizations: @myfile.py"
}
```

### Secure Code Generation
```json
{
  "prompt": "Generate a Python function to validate email addresses",
  "sandbox": "read-only"
}
```

### Session Continuation
```json
{
  "prompt": "Thanks for the previous suggestions. Can you now help me implement the caching layer you mentioned?",
  "resume": true
}
```

### Full Auto Mode
```json
{
  "prompt": "Create a REST API endpoint for user authentication with proper error handling",
  "full-auto": true
}
```

### Custom Model and Directory
```json
{
  "prompt": "Analyse the project structure and suggest architectural improvements",
  "override-model": "claude-3.5-sonnet",
  "cd": "/path/to/project"
}
```

## Common Patterns

- Use `resume: true` to continue conversations in the same context
- Set `sandbox: "read-only"` for safe code analysis without execution
- Use `full-auto: true` for streamlined development with automatic permissions
- Specify `override-model` to use different Claude models for specific tasks
- Use `@file` syntax in prompts to reference specific files
- Set `cd` parameter to change working directory for project analysis

## Troubleshooting

- **Codex CLI not found error**: Install Codex CLI and ensure it's available in your PATH
- **Authentication errors**: Ensure you are authenticated and Codex is properly configured
- **Tool is not enabled**: Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'codex-agent'
- **Permission denied errors**: Check your sandbox settings. Use 'sandbox: workspace-write' or 'full-auto: true' for write operations
- **Session not found errors**: Verify the session-id exists or use 'resume: true' without session-id to resume the last session
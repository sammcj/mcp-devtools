# Claude Agent Tool

The `claude-agent` tool provides access to Anthropic's Claude large language models via the `claude` command-line tool. It allows you to use Claude as a sub-agent for various tasks, such as code generation, analysis, and troubleshooting.

Requires the [Claude Code CLI](https://www.anthropic.com/claude-code) to be installed and authenticated.

## Parameters

- `prompt` (string, required): A clear, concise prompt to send to the Claude CLI.
- `override-model` (string, optional): Specify a different Claude model to use (e.g., `opus`). Defaults to `sonnet`.
- `yolo-mode` (boolean, optional): Bypass all permission checks. Defaults to `false`.
- `continue-last-conversation` (boolean, optional): Continue the most recent conversation. Defaults to `false`.
- `resume-specific-session` (string, optional): Resume a conversation with a specific session ID.
- `include-directories` (array of strings, optional): A list of additional directories to allow the tool to access.

## Environment Variables

- `AGENT_TIMEOUT`: (optional) The timeout in seconds for the `claude` command. Defaults to `300`.
- `AGENT_MAX_RESPONSE_SIZE`: (optional) Maximum response size in bytes. Defaults to `2097152` (2MB).
- `AGENT_PERMISSIONS_MODE`: (optional) Controls whether yolo-mode parameter is exposed and its default behaviour. Options: `yolo` (force yolo-mode on, hide parameter), `disabled`/`false` (force yolo-mode off, hide parameter). If unset, agent can control yolo-mode via parameter. This controls the `--dangerously-skip-permissions` flag.
- `CLAUDE_SYSTEM_PROMPT`: (optional) A string to append to the default system prompt.
- `CLAUDE_PERMISSION_MODE`: (optional) Claude CLI's native permission mode setting (passed as `--permission-mode` to Claude CLI). This controls how Claude CLI prompts for permissions and is separate from AGENT_PERMISSIONS_MODE which controls `--dangerously-skip-permissions`.

## Security Features

- **Response Size Limits**: Configurable maximum response size prevents excessive memory usage and potential DoS conditions
- **Input Validation**: Comprehensive parameter validation and type checking
- **Process Isolation**: Agent execution runs in isolated subprocess with proper timeout controls
- **Timeout Controls**: Configurable timeout limits prevent runaway processes
- **Error Handling**: Secure error handling that doesn't expose sensitive system information

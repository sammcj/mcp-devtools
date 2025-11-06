# Gemini Agent Tool

The `gemini-agent` tool provides access to Google's Gemini large language models via the `gemini` command-line tool. It allows you to use Gemini as a sub-agent for various tasks, such as code generation, analysis, and troubleshooting.

Requires the [Gemini CLI](https://github.com/google-gemini/gemini-cli) to be installed and  and authenticated.

## Parameters

- `prompt` (string, required): A clear, concise prompt to send to the Gemini CLI.
- `override-model` (string, optional): Specify a different Gemini model to use (e.g., `gemini-2.5-flash`). Defaults to `gemini-2.5-pro`.
- `sandbox` (boolean, optional): Run the command in the Gemini sandbox. Defaults to `false`.
- `yolo-mode` (boolean, optional): Allow Gemini to make changes and run commands without confirmation. Defaults to `false`.
- `include-all-files` (boolean, optional): Recursively include all files in the current directory as context. Defaults to `false`.

## Environment Variables

- `ENABLE_AGENTS`: A comma-separated list of agents to enable (e.g., `claude,gemini`).
- `AGENT_TIMEOUT`: The timeout in seconds for the `gemini` command. Defaults to `180`.
- `AGENT_MAX_RESPONSE_SIZE`: Maximum response size in bytes. Defaults to `2097152` (2MB).
- `AGENT_PERMISSIONS_MODE`: Controls yolo mode behaviour. Options: `default` (agent can control via parameter), `enabled`/`true`/`yolo` (force on, hide parameter), `disabled`/`false` (force off, hide parameter). Defaults to `default`.

## Security Features

- **Response Size Limits**: Configurable maximum response size prevents excessive memory usage and potential DoS conditions
- **Input Validation**: Comprehensive parameter validation and type checking
- **Process Isolation**: Agent execution runs in isolated subprocess with proper timeout controls
- **Timeout Controls**: Configurable timeout limits prevent runaway processes
- **Error Handling**: Secure error handling that doesn't expose sensitive system information
- **Model Fallback**: Automatic fallback to flash model on quota exhaustion provides resilience

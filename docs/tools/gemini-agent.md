# Gemini Agent Tool

The `gemini-agent` tool provides access to Google's Gemini large language models via the `gemini` command-line tool. It allows you to use Gemini as a sub-agent for various tasks, such as code generation, analysis, and troubleshooting.

Requires the [Gemini CLI](https://github.com/google-gemini/gemini-cli) to be installed and  and authenticated.

## Parameters

- `prompt` (string, required): A clear, concise prompt to send to the Gemini CLI.
- `model` (string, optional): Specify a different Gemini model to use (e.g., `gemini-2.5-flash`). Defaults to `gemini-2.5-pro`.
- `sandbox` (boolean, optional): Run the command in the Gemini sandbox. Defaults to `false`.
- `yolo-mode` (boolean, optional): Allow Gemini to make changes and run commands without confirmation. Defaults to `false`.
- `include-all-files` (boolean, optional): Recursively include all files in the current directory as context. Defaults to `false`.

## Environment Variables

- `ENABLE_AGENTS`: A comma-separated list of agents to enable (e.g., `claude,gemini`).
- `GEMINI_TIMEOUT`: The timeout in seconds for the `gemini` command. Defaults to `180`.

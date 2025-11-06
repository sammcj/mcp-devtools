# Q Developer Agent Tool

The Q Developer Agent tool provides integration with AWS Q Developer CLI through the MCP (Model Context Protocol) server. This tool enables AI agents to leverage Q Developer's capabilities for AWS-focused code analysis, generation, and assistance.

## Overview

AWS Q Developer is Amazon's AI coding assistant that provides code completions, explanations, and solutions with deep knowledge of AWS services and best practices. The Q Developer Agent tool allows MCP clients to interact with Q Developer CLI commands programmatically, enabling automated code assistance workflows.

## Configuration

### Requirements

- AWS Q Developer CLI must be installed and available in PATH
- AWS credentials must be configured
- Q Developer must be authenticated (`q auth`)

### Environment Variables

- `ENABLE_ADDITIONAL_TOOLS`: Must include `q-developer-agent` to enable the tool
- `AGENT_TIMEOUT`: (optional) Timeout for Q Developer operations in seconds (default: 300)
- `AGENT_MAX_RESPONSE_SIZE`: (optional) Maximum response size in bytes (default: 2MB)
- `AGENT_PERMISSIONS_MODE`: (optional) Controls whether yolo-mode parameter is exposed and its default behaviour. Options: `yolo` (force yolo-mode on, hide parameter), `disabled`/`false` (force yolo-mode off, hide parameter). If unset, agent can control yolo-mode via parameter. This controls the `--trust-all-tools` flag.

### Security

The tool is disabled by default for security reasons. It must be explicitly enabled by setting:

```bash
export ENABLE_ADDITIONAL_TOOLS="q-developer-agent"
```

## Usage

### Basic Parameters

- **prompt** (required): The instruction or question for Q Developer
- **resume** (optional): Continue previous conversation from current directory
- **agent** (optional): Context profile to use for the conversation
- **override-model** (optional): Specify model (claude-3.5-sonnet, claude-3.7-sonnet, claude-sonnet-4)
- **yolo-mode** (optional): Trust all tools without confirmation
- **trust-tools** (optional): Comma-separated list of specific tools to trust
- **verbose** (optional): Enable detailed logging

### Examples

#### Basic Code Analysis
```json
{
  "name": "q-developer-agent",
  "arguments": {
    "prompt": "Review this Lambda function for security best practices and suggest improvements"
  }
}
```

#### Continue Previous Conversation
```json
{
  "name": "q-developer-agent",
  "arguments": {
    "prompt": "Now implement the caching mechanism we discussed",
    "resume": true
  }
}
```

#### Specify Model and Enable Tools
```json
{
  "name": "q-developer-agent",
  "arguments": {
    "prompt": "Help me troubleshoot this CloudFormation template",
    "override-model": "claude-sonnet-4",
    "yolo-mode": true,
    "verbose": true
  }
}
```

## Features

### Session Management
- Directory-based conversation history
- Resume conversations using the `resume` parameter
- Context preserved per working directory

### Model Selection
- Support for multiple Claude models
- Default uses Q Developer's current default model
- Override with specific model versions

### Tool Integration
- Automatic tool approval with `yolo-mode`
- Selective tool trust with `trust-tools`
- Enhanced security with explicit enablement requirement

### Response Management
- Configurable response size limits
- Intelligent truncation at line boundaries
- Timeout handling with partial output preservation

## Common Use Cases

### AWS Development
- CloudFormation template analysis and debugging
- Lambda function optimization and security review
- AWS CLI command generation and troubleshooting
- Infrastructure as Code best practices guidance

### Code Review and Analysis
- Security vulnerability identification
- Performance optimization suggestions
- AWS service integration recommendations
- Code quality improvements

### Debugging and Troubleshooting
- AWS service configuration issues
- Permission and IAM policy problems
- Network connectivity debugging
- Resource quota and limit issues

## Error Handling

### Common Error Scenarios

#### Tool Not Enabled
```
Error: Q Developer agent tool is not enabled. Set ENABLE_ADDITIONAL_TOOLS environment variable to include 'q-developer-agent'
```

#### CLI Not Found
```
Error: Q Developer CLI not found. Please install Q Developer CLI and ensure it's available in your PATH
```

#### Authentication Issues
```
Error: Q Developer authentication failed. Please ensure you are authenticated with AWS and Q Developer is configured properly
```

#### Tool Approval Required
```
Error: Tool approval required but --no-interactive was specified. Use yolo-mode parameter to automatically approve tools
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

### Resource Usage
- Stateless operation - no persistent connections
- Temporary files cleaned up automatically
- Memory usage scales with response size

## Security Considerations

### Access Control
- Explicit enablement required via environment variable
- No pre-flight CLI installation verification
- User responsible for AWS credential management

### Command Safety
- Uses `exec.CommandContext` with separate arguments (no shell interpretation)
- Input validation prevents command injection
- Always includes `--no-interactive` flag to prevent hanging

### Data Handling
- No sensitive data logging in stdio mode
- Response size limits prevent memory exhaustion
- Timeout handling prevents resource locks

## Troubleshooting

### Installation Issues
1. Verify Q Developer CLI installation: `which q`
2. Check AWS CLI configuration: `aws configure list`
3. Test Q Developer authentication: `q auth`

### Permission Problems
1. Ensure `ENABLE_ADDITIONAL_TOOLS` includes `q-developer-agent`
2. Check working directory permissions for `.q` session files
3. Verify AWS credentials and permissions

### Performance Issues
1. Adjust `AGENT_TIMEOUT` for longer operations
2. Increase `AGENT_MAX_RESPONSE_SIZE` for large outputs
3. Use `trust-tools` or `yolo-mode` to avoid approval prompts

### Tool Integration Problems
1. Enable verbose mode for detailed logging
2. Check tool compatibility with Q Developer version
3. Use explicit tool trust configuration

## Comparison with Other Agents

| Feature | Q Developer | Claude Agent | Gemini Agent |
|---------|-------------|--------------|--------------|
| Focus | AWS/Cloud | General Purpose | General Purpose |
| File Context | No @ syntax | @ syntax supported | @ syntax supported |
| Models | Claude only | Claude models | Gemini models |
| Session Management | Directory-based | Session ID based | Include files |
| Tool Trust | Granular control | All or nothing | All or nothing |

## Related Documentation

- [AWS Q Developer Documentation](https://docs.aws.amazon.com/q/)
- [Q Developer CLI Reference](https://docs.aws.amazon.com/q/latest/userguide/q-cli.html)
- [AWS CLI Configuration](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html)

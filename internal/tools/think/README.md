# Think Tool

A simple MCP tool that provides a structured thinking space for AI agents during complex workflows.

## Overview

The `think` tool allows AI agents to pause and reason about complex problems, analyze tool outputs, and plan sequential actions. It simply returns the provided thought string, creating a dedicated space for structured reasoning without any side effects.

## Based on Research

This tool is inspired by [Anthropic's research](https://www.anthropic.com/engineering/claude-think-tool) which showed significant performance improvements in complex, multi-step scenarios when AI agents have access to a dedicated thinking space.

## Usage

```json
{
  "name": "think",
  "arguments": {
    "thought": "I need to analyse the API response before deciding which action to take next..."
  }
}
```

## When to Use

The think tool is particularly valuable for:

- **Analysing tool outputs** before taking action
- **Breaking down complex multi-step problems** into manageable parts
- **Reasoning through policy decisions or constraints**
- **Planning sequential actions** where mistakes are costly
- **Processing and reflecting** on information gathered from previous tool calls

## Tool Definition

- **Name**: `think`
- **Description**: Use the tool to think about something. It will not obtain new information or change the database, but just append the thought to the log. Use it when complex reasoning or some cache memory is needed.
- **Parameters**:
  - `thought` (required, string): A thought to think about

## Implementation

The tool is implemented as a simple function that:
1. Takes a thought string as input
2. Returns the thought string as output
3. Has no side effects (no logging, no database changes, no network calls)

## Performance Benefits

Based on Anthropic's τ-bench evaluation:
- 54% relative improvement in complex airline domain scenarios
- Improved consistency across multiple trials
- Better handling of edge cases and unusual scenarios
- Significant benefits in policy-heavy environments

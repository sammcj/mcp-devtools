# Sequential Thinking Tool

The Sequential Thinking tool provides a structured approach to dynamic problem-solving through auto-managed thought processes that adapt and evolve as understanding deepens.

## Overview

The Sequential Thinking tool allows AI agents to work through complex problems with automatically managed state. Focus on your thinking content while the tool handles numbering, tracking, and branching mechanics.

## Features

- **Auto-Managed State**: Automatic thought numbering and progress tracking
- **Smart Revision Detection**: Simply reference previous thoughts to revise
- **Simple Branching**: Use explore labels for alternative approaches  
- **Dynamic Adaptation**: Tool adjusts to your thinking flow automatically
- **Context Preservation**: Complete thought history maintained
- **Flexible Process**: Add, revise, or explore thoughts naturally
- **Formatted Logging**: Visual output shows thinking structure

## Parameters

### Required Parameters

- **`action`** (string): Action to perform
  - **Options**: `"think"` (default) or `"get_usage"`
  - **Description**: Specifies whether to process a thought or get detailed usage instructions
- **`thought`** (string): Your current thinking step content
  - **Description**: The actual content of your thinking - analysis, insights, questions, conclusions
  - **Examples**: "Let me break down this API design problem into authentication, rate limiting, and error handling"
- **`continue`** (boolean): Whether more thinking is needed after this step
  - **Description**: `true` if you need more thoughts, `false` when done
  - **Usage**: Set to `false` only when satisfied with final answer

### Optional Parameters

- **`revise`** (string): Brief text snippet from previous thought to revise
  - **Description**: Reference text from a previous thought you want to reconsider
  - **Examples**: "API design problem", "authentication approach"
  - **Effect**: Automatically marks this as a revision and shows what you're revising
- **`explore`** (string): Label for exploring alternative approach
  - **Description**: Identifier for branching into alternative reasoning path
  - **Examples**: "graphql-alternative", "security-first", "performance-focused"
  - **Effect**: Creates a named branch for this line of thinking

## When to Use Sequential Thinking

### Complex Problem Breakdown
When facing multi-faceted challenges requiring systematic analysis:
```json
{
  "name": "sequential_thinking",
  "arguments": {
    "action": "think",
    "thought": "This API integration requires authentication, rate limiting, error handling, and data transformation. Let me work through each component systematically.",
    "continue": true
  }
}
```

### Revising Previous Analysis
When new information contradicts earlier thinking:
```json
{
  "name": "sequential_thinking", 
  "arguments": {
    "action": "think",
    "thought": "Actually, my earlier assumption was wrong. The logs show connection pool exhaustion, not individual connection failures.",
    "continue": true,
    "revise": "database connections"
  }
}
```

### Exploring Alternative Approaches
When considering different paths:
```json
{
  "name": "sequential_thinking",
  "arguments": {
    "action": "think", 
    "thought": "What if we used GraphQL instead of REST? This could simplify the client interface significantly.",
    "continue": true,
    "explore": "graphql-alternative"
  }
}
```

### Final Decision
When reaching conclusion:
```json
{
  "name": "sequential_thinking",
  "arguments": {
    "action": "think",
    "thought": "After comparing both approaches, REST with proper OpenAPI documentation offers better tooling support. Final design: REST API with OAuth2, rate limiting, and comprehensive validation.",
    "continue": false
  }
}
```

## Best Practices

1. **Focus on Content**: Concentrate on what you're thinking, not mechanics
2. **Natural Revision**: Use `revise` when reconsidering previous thoughts  
3. **Explore Alternatives**: Use `explore` for different approaches
4. **Express Uncertainty**: Natural uncertainty is valuable in thinking process
5. **Clear Conclusions**: Set `continue: false` only with satisfactory answer
6. **Concise Thoughts**: Keep individual thoughts focused and clear

## Advanced Usage

### Getting Detailed Instructions
For comprehensive usage guide with examples:
```json
{
  "name": "sequential_thinking",
  "arguments": {
    "action": "get_usage"
  }
}
```

### Environment Variables
- **`DISABLE_THOUGHT_LOGGING`**: Set to "true" to disable formatted console output
- **`ENABLE_ADDITIONAL_TOOLS`**: Must include "sequential-thinking" to enable this tool

## Key Advantages

- **Reduced Cognitive Load**: No manual numbering or tracking
- **Natural Flow**: Think naturally, tool handles structure
- **Smart Automation**: Automatic state management and progress tracking  
- **Flexible Process**: Adapt thinking process dynamically
- **Clear Output**: Formatted display shows thinking progression

The Sequential Thinking tool transforms complex problem-solving by removing mechanical overhead and letting you focus on the actual thinking process.
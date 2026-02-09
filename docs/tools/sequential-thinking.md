# Sequential Thinking Tool

Multi-step reasoning tool for problems that need more than a quick thought. Each step should be a focused, concise observation or decision. Spread analysis across multiple steps rather than writing one long step.

## Overview

Use `sequential_thinking` instead of `think` when:
- The problem needs more than 2-4 sentences of reasoning
- You need to revise or branch your analysis
- The full scope is unclear and you may need course correction

The tool handles numbering, tracking, and branching automatically -- focus on your thinking content.

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
- **`thought`** (string): A single focused reasoning step
  - **Description**: 1-3 sentences covering one observation, decision, or question. Use multiple steps for longer analysis rather than one long step
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

1. **Keep Steps Short**: Aim for 1-3 sentences (~50-100 words) per step. Split longer reasoning into multiple steps
2. **Focus on Content**: Concentrate on what you're thinking, not mechanics
3. **Natural Revision**: Use `revise` when reconsidering previous thoughts
4. **Explore Alternatives**: Use `explore` for different approaches
5. **Express Uncertainty**: Natural uncertainty is valuable in thinking process
6. **Clear Conclusions**: Set `continue: false` only with satisfactory answer

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
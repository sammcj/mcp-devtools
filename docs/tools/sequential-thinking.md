# Sequential Thinking Tool

The Sequential Thinking tool provides a structured approach to dynamic and reflective problem-solving through organised thought processes that can adapt and evolve as understanding deepens.

## Overview

Based on sequential reasoning methodologies, the Sequential Thinking tool allows AI agents to work through complex problems step-by-step, with the flexibility to revise earlier thoughts, branch into alternative approaches, and dynamically adjust the thinking process as insights emerge.

## Features

- **Dynamic Thought Progression**: Adjust the total number of thoughts as understanding evolves
- **Revision Support**: Question and revise previous thoughts when new insights emerge
- **Branching Logic**: Explore alternative approaches from any point in the thinking process
- **Flexible Planning**: Add more thoughts even after reaching what seemed like the end
- **Context Preservation**: Maintain continuity across multiple thinking steps
- **Hypothesis Generation**: Develop and verify solution hypotheses systematically
- **Uncertainty Expression**: Acknowledge and work with incomplete information

## Parameters

### Required Parameters

- **`thought`** (string): Your current thinking step
  - **Description**: The content of the current thought, which can include analysis, revision, branching, or hypothesis development
  - **Examples**: Regular analysis, questions about previous decisions, realisations about needing more analysis
- **`nextThoughtNeeded`** (boolean): Whether another thought step is needed
  - **Description**: Indicates if the thinking process should continue
  - **Usage**: Set to `false` only when truly satisfied with the solution
- **`thoughtNumber`** (integer): Current thought number in sequence
  - **Description**: The position of this thought in the sequence
  - **Minimum**: 1
  - **Note**: Can exceed initial `totalThoughts` estimate if more thinking is needed
- **`totalThoughts`** (integer): Current estimate of total thoughts needed
  - **Description**: Your current estimate of how many thoughts the problem requires
  - **Minimum**: 1
  - **Flexibility**: Can be adjusted up or down as you progress

### Optional Parameters

- **`isRevision`** (boolean): Whether this thought revises previous thinking
  - **Description**: Marks thoughts that reconsider or correct earlier analysis
  - **Default**: `false`
- **`revisesThought`** (integer): Which thought number is being reconsidered
  - **Description**: References the specific thought being revised
  - **Usage**: Required when `isRevision` is `true`
- **`branchFromThought`** (integer): Branching point thought number
  - **Description**: The thought number where this alternative path begins
  - **Usage**: Used with `branchId` to create alternative reasoning paths
- **`branchId`** (string): Branch identifier for alternative reasoning paths
  - **Description**: Unique identifier for this branch of thinking
  - **Examples**: "alternative-approach", "security-focused", "performance-optimised"
- **`needsMoreThoughts`** (boolean): If more thoughts are needed beyond current estimate
  - **Description**: Indicates realisation that additional thinking is required
  - **Usage**: Helps adjust `totalThoughts` dynamically

## When to Use Sequential Thinking

### Complex Problem Breakdown
When facing multi-faceted challenges requiring systematic analysis:
```json
{
  "name": "sequential_thinking",
  "arguments": {
    "thought": "This API integration requires authentication, rate limiting, error handling, and data transformation. Let me work through each component systematically.",
    "nextThoughtNeeded": true,
    "thoughtNumber": 1,
    "totalThoughts": 5
  }
}
```

### Planning with Uncertainty
When the full scope isn't clear initially:
```json
{
  "name": "sequential_thinking",
  "arguments": {
    "thought": "The user wants to 'improve performance' but I need to understand what specific performance issues they're experiencing before I can plan an approach.",
    "nextThoughtNeeded": true,
    "thoughtNumber": 1,
    "totalThoughts": 3,
    "needsMoreThoughts": true
  }
}
```

### Revising Previous Analysis
When new information contradicts earlier thinking:
```json
{
  "name": "sequential_thinking",
  "arguments": {
    "thought": "Actually, my earlier assumption about database connections was wrong. The logs show connection pool exhaustion, not individual connection failures.",
    "nextThoughtNeeded": true,
    "thoughtNumber": 4,
    "totalThoughts": 6,
    "isRevision": true,
    "revisesThought": 2
  }
}
```

### Exploring Alternative Approaches
When branching into different solution paths:
```json
{
  "name": "sequential_thinking",
  "arguments": {
    "thought": "Instead of microservices, what if we used a modular monolith? This could reduce complexity while maintaining separation of concerns.",
    "nextThoughtNeeded": true,
    "thoughtNumber": 5,
    "totalThoughts": 8,
    "branchFromThought": 3,
    "branchId": "monolith-approach"
  }
}
```

## Usage Patterns

### Progressive Problem Solving
```json
// 1. Initial assessment
{
  "thought": "The user reports slow database queries. I need to investigate query patterns, indexing, and connection pooling.",
  "nextThoughtNeeded": true,
  "thoughtNumber": 1,
  "totalThoughts": 4
}

// 2. Gathering information
{
  "thought": "Looking at the query logs, I see repeated SELECT statements without proper indexes. The explain plans show full table scans.",
  "nextThoughtNeeded": true,
  "thoughtNumber": 2,
  "totalThoughts": 4
}

// 3. Solution development
{
  "thought": "I should recommend adding composite indexes on the frequently queried columns: user_id, created_at, and status.",
  "nextThoughtNeeded": true,
  "thoughtNumber": 3,
  "totalThoughts": 4
}

// 4. Verification and completion
{
  "thought": "The index recommendations should resolve the performance issues. I'll also suggest query optimisation best practices.",
  "nextThoughtNeeded": false,
  "thoughtNumber": 4,
  "totalThoughts": 4
}
```

### Adaptive Thinking Process
```json
// 1. Initial plan
{
  "thought": "This seems like a straightforward configuration change.",
  "nextThoughtNeeded": true,
  "thoughtNumber": 1,
  "totalThoughts": 2
}

// 2. Realisation of complexity
{
  "thought": "Wait, this configuration affects multiple services and requires coordination. I need to think through the dependencies.",
  "nextThoughtNeeded": true,
  "thoughtNumber": 2,
  "totalThoughts": 5,
  "needsMoreThoughts": true
}

// 3. Expanded analysis
{
  "thought": "The configuration change impacts: authentication service, API gateway, database connections, and monitoring. Each needs individual consideration.",
  "nextThoughtNeeded": true,
  "thoughtNumber": 3,
  "totalThoughts": 5
}
```

### Hypothesis Testing
```json
// 1. Problem identification
{
  "thought": "The application crashes intermittently. I need to form hypotheses about potential causes.",
  "nextThoughtNeeded": true,
  "thoughtNumber": 1,
  "totalThoughts": 6
}

// 2. Hypothesis formation
{
  "thought": "Hypothesis 1: Memory leak in the background job processor. Evidence needed: memory usage patterns over time.",
  "nextThoughtNeeded": true,
  "thoughtNumber": 2,
  "totalThoughts": 6
}

// 3. Alternative hypothesis
{
  "thought": "Hypothesis 2: Race condition in concurrent request handling. Evidence needed: crash logs showing threading issues.",
  "nextThoughtNeeded": true,
  "thoughtNumber": 3,
  "totalThoughts": 6,
  "branchFromThought": 2,
  "branchId": "concurrency-hypothesis"
}

// 4. Hypothesis verification
{
  "thought": "The memory monitoring shows steady increases during job processing, supporting hypothesis 1. I should investigate the job processor code.",
  "nextThoughtNeeded": true,
  "thoughtNumber": 4,
  "totalThoughts": 6,
  "revisesThought": 3
}
```

## Best Practices

### Effective Sequential Thinking

**Start with Reasonable Estimates**: Begin with a thoughtful estimate but be ready to adjust
```json
{
  "thought": "This database migration seems complex. Starting with 5 thoughts but may need more.",
  "thoughtNumber": 1,
  "totalThoughts": 5
}
```

**Don't Hesitate to Revise**: Question your earlier thinking when new insights emerge
```json
{
  "thought": "My initial approach in thought 2 overlooked security implications. Let me reconsider.",
  "isRevision": true,
  "revisesThought": 2
}
```

**Use Branching for Alternatives**: Explore different paths without losing the main thread
```json
{
  "thought": "Alternative approach: What if we used event sourcing instead of traditional CRUD?",
  "branchFromThought": 4,
  "branchId": "event-sourcing"
}
```

**Express Uncertainty**: Acknowledge when you need more information
```json
{
  "thought": "I'm not certain about the scalability requirements. This affects whether to recommend horizontal or vertical scaling.",
  "needsMoreThoughts": true
}
```

### Integration Strategies

**Problem Analysis Flow**:
1. Use sequential thinking to break down the problem systematically
2. Gather information using other tools between thoughts
3. Return to sequential thinking to process and integrate new information
4. Continue until reaching a well-reasoned solution

**Decision Making Process**:
1. Identify decision points and criteria
2. Explore alternatives through branching
3. Evaluate trade-offs systematically
4. Converge on the optimal solution

## Advanced Features

### Thought History Tracking
The tool maintains a complete history of your thinking process, enabling:
- Reference to earlier thoughts for consistency
- Understanding of how conclusions were reached
- Ability to trace reasoning paths

### Branch Management
Multiple reasoning branches can be maintained simultaneously:
- Each branch has a unique identifier
- Branches can diverge and potentially converge
- Helps explore alternative solutions systematically

### Dynamic Adjustment
The thinking process adapts to emerging complexity:
- Total thought estimates can increase or decrease
- New insights can redirect the entire approach
- Flexibility prevents premature closure of analysis

## Response Format

The Sequential Thinking tool returns structured information about the thinking process:

```json
{
  "thoughtNumber": 3,
  "totalThoughts": 5,
  "nextThoughtNeeded": true,
  "branches": ["alternative-approach", "security-focused"],
  "thoughtHistoryLength": 7
}
```

The tool also provides formatted output to stderr (when enabled) showing:
- 💭 Regular thoughts with progress indicators
- 🔄 Revision thoughts with reference to revised thinking
- 🌿 Branch thoughts with branching information

## Configuration

### Environment Variables

- **`ENABLE_ADDITIONAL_TOOLS`**: Enable the sequential thinking tool
  - **Required value**: Must include `"sequential-thinking"`
  - **Example**: `ENABLE_ADDITIONAL_TOOLS="sequential-thinking,claude-agent,filesystem"`
  - **Note**: The tool is disabled by default for security reasons

- **`DISABLE_THOUGHT_LOGGING`**: Disable thought logging to stderr
  - **Values**: `"true"` to disable, any other value or unset to enable
  - **Default**: Enabled (logging active)
  - **Usage**: Set to `"true"` in production environments where stderr output should be minimised

## Integration Examples

### With Research Tools
```bash
# 1. Start sequential thinking
sequential_thinking "I need to research the best approach for implementing real-time features"

# 2. Gather information
internet_search "websockets vs server-sent events performance comparison"

# 3. Continue thinking with new information
sequential_thinking "Based on the research, WebSockets provide bidirectional communication but require more infrastructure..."

# 4. Deep dive into specific areas
package_documentation "socket.io" --topic "scaling"

# 5. Finalise approach
sequential_thinking "Considering the requirements and research, I recommend Server-Sent Events for this use case"
```

### With Development Tools
```bash
# 1. Plan the implementation
sequential_thinking "Breaking down this feature into components: API endpoints, database schema, frontend components"

# 2. Work through each component
sequential_thinking "Starting with the database schema - I need tables for users, posts, and relationships"

# 3. Revise based on new considerations
sequential_thinking "Actually, I should consider NoSQL for the social features - more flexible for relationship data"

# 4. Branch into alternative design
sequential_thinking "Alternative: hybrid approach with SQL for users and NoSQL for social graph" --branch-from 3 --branch-id "hybrid-db"
```

## Common Use Cases

### Architecture Planning
- Breaking down system components systematically
- Evaluating different architectural patterns
- Planning migration strategies
- Assessing scalability requirements

### Debugging Complex Issues
- Systematic hypothesis formation and testing
- Tracing through multiple potential causes
- Revising understanding based on new evidence
- Correlating symptoms across system components

### Code Review and Design
- Analysing code changes for broader impact
- Evaluating design patterns and trade-offs
- Planning refactoring approaches
- Considering security and performance implications

### Project Planning
- Breaking down project phases and dependencies
- Risk assessment and mitigation planning
- Resource allocation and timeline estimation
- Stakeholder impact analysis

---

For technical implementation details, see the [Sequential Thinking tool source code](../../internal/tools/sequentialthinking/sequential_thinking.go).

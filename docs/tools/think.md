# Think Tool

A concise scratchpad for reasoning through a single question or decision. Does not retrieve information or modify anything -- just records the thought.

## Overview

Based on Anthropic's research, the Think tool allows AI agents to pause and reason through a problem before taking action. Keep thoughts brief and focused: 2-4 sentences (~50-150 words). For multi-step reasoning, revision, or branching analysis, use `sequential_thinking` instead.

## When to Use Think vs Sequential Thinking

| Scenario                                          | Tool                  |
|---------------------------------------------------|-----------------------|
| Quick reasoning about a single decision           | `think`               |
| Evaluating one option or reflecting on a result   | `think`               |
| Multi-step analysis across several aspects        | `sequential_thinking` |
| Revising or branching previous reasoning          | `sequential_thinking` |
| Problem scope unclear, may need course correction | `sequential_thinking` |

## Parameters

### Required Parameters

- **`thought`** (string): A brief reasoning note -- 2-4 sentences covering what you're considering and your conclusion
  - **Maximum length**: Configurable via `THINK_MAX_LENGTH` environment variable (default: 2000 characters, ~300 words)
  - **Note**: The tool includes a 500-character safety buffer above the configured limit to accommodate AI agents' imprecise character counting, whilst still encouraging concise thoughts
  - **Guidance**: State what you need to reason about, your conclusion or next step, and why. Do NOT include multi-step analyses, inline code blocks, or exhaustive breakdowns

### Optional Parameters

- **`how_hard`** (string): Intensity level for thinking about the problem
  - **Options**: `"hard"` (default), `"harder"`, `"ultra"`
  - **Description**: Indicates the complexity level of the thinking required
  - **Default**: `"hard"` if not specified

## Examples

### Analysing a Tool Output
```json
{
  "name": "think",
  "arguments": {
    "thought": "The API response contains both a 429 and a 503. The 429 is endpoint-specific rate limiting, not a global outage. I should retry with backoff on just the /users endpoint.",
    "how_hard": "hard"
  }
}
```

### Making a Quick Decision
```json
{
  "name": "think",
  "arguments": {
    "thought": "The user wants to modify production config. This needs approval and a rollback plan first -- I should ask before proceeding."
  }
}
```

### Reflecting on Gathered Data
```json
{
  "name": "think",
  "arguments": {
    "thought": "CPU spikes at 14:30 correlate with the slow query log. This points to connection pool exhaustion under peak load, not a query optimisation issue."
  }
}
```

> **Note:** For multi-step breakdowns (numbered checklists, systematic analysis across several aspects), use `sequential_thinking` instead. The think tool is for a single focused observation or decision.

### Thinking Intensities

#### Standard (`how_hard: "hard"`)
```json
{
  "name": "think",
  "arguments": {
    "thought": "The user wants a new API endpoint. REST with JSON is the right fit here given the existing patterns in the codebase.",
    "how_hard": "hard"
  }
}
```

#### Deeper (`how_hard: "harder"`)
```json
{
  "name": "think",
  "arguments": {
    "thought": "This auth change affects both the gateway and downstream services. The token format change is backward-incompatible, so I need a migration path before proceeding.",
    "how_hard": "harder"
  }
}
```

#### Maximum (`how_hard: "ultra"`)
```json
{
  "name": "think",
  "arguments": {
    "thought": "Cascading failures across regions -- replication lag plus CDN invalidation plus third-party degradation. The replication lag is the root cause; fixing that unblocks the CDN, and the third-party issue is independent.",
    "how_hard": "ultra"
  }
}
```

## Usage Patterns

### Single Decision After Research
```json
// 1. Gather information
{"name": "internet_search", "arguments": {"query": "kubernetes pod restart loop causes"}}

// 2. Quick conclusion
{"name": "think", "arguments": {
  "thought": "The search results point to resource limits as the most likely cause given the OOMKilled status. I should check pod resource usage next."
}}

// 3. Take targeted action
{"name": "fetch_url", "arguments": {"url": "kubernetes.io/docs/troubleshooting"}}
```

> **Note:** If you need multiple think calls in sequence (e.g., consider options, then evaluate trade-offs, then decide), use `sequential_thinking` instead -- that's exactly what it's designed for.

## Best Practices

### Keep It Brief
Each thought should be 2-4 sentences. State the problem, your conclusion, and why.

**Good** -- specific and concise:
```json
{"thought": "The 429 errors are only on /users after 100 req/hr. Other endpoints are fine. This is endpoint-specific rate limiting, not a global issue."}
```

**Bad** -- multi-step breakdown (use `sequential_thinking` for this):
```json
{"thought": "From a security perspective, this approach exposes user data. From a performance perspective, it reduces database queries. From a maintenance perspective, it adds complexity. I need to weigh these trade-offs based on the project priorities."}
```

### Integration with Other Tools

```json
{"name": "think", "arguments": {"thought": "The error says permission denied but auth succeeded. The problem is at the resource ACL level, not authentication. I should check the IAM policy next."}}
```

## Common Use Cases

### Quick Code Review Observation
```json
{
  "name": "think",
  "arguments": {
    "thought": "This middleware change alters the token claims structure. Existing tokens will fail validation, so this needs a migration or dual-parsing approach."
  }
}
```

### Architecture Decision
```json
{
  "name": "think",
  "arguments": {
    "thought": "For real-time features, SSE is the simplest fit here -- the data flow is server-to-client only and the existing infrastructure already supports HTTP streaming."
  }
}
```

### Debugging Observation
```json
{
  "name": "think",
  "arguments": {
    "thought": "Works locally but fails in prod. The error traces to a missing env var for the database connection string -- the deployment config is missing DATABASE_URL."
  }
}
```

## Integration Examples

### Research Workflow
```bash
# 1. Gather information
internet_search "kubernetes ingress nginx configuration"

# 2. Quick conclusion before next step
think "The nginx ingress controller with cert-manager is the standard approach for their AWS setup. I should pull the official docs next."

# 3. Get detailed documentation
package_documentation "nginx-ingress" --topic "configuration"
```

> For workflows requiring multiple think steps (analyse, then correlate, then decide), use `sequential_thinking` to keep reasoning structured across steps.

## Configuration

### Environment Variables

The Think tool supports the following configuration options:

- **`THINK_MAX_LENGTH`**: Maximum length for thought input in characters
  - **Default**: `2000`
  - **Description**: Controls the advertised maximum length of thoughts to prevent resource exhaustion. The actual enforcement includes a 500-character safety buffer (e.g., 2000 advertised = 2500 actual maximum) to accommodate AI agents' imprecise character counting
  - **Example**: `THINK_MAX_LENGTH=5000` advertises 5000 characters to agents but accepts up to 5500 characters

### Security Features

- **Input Length Validation**: Prevents excessively long thoughts that could impact performance
- **Resource Protection**: Configurable limits help maintain system stability
- **Error Handling**: Clear feedback when thoughts exceed configured limits

## Performance Impact

The Think tool has minimal performance overhead:
- **Processing time**: < 100ms typically
- **Memory usage**: Negligible
- **Network**: No external calls
- **Storage**: Thoughts are logged but not persisted
- **Input limits**: Configurable to balance functionality with resource protection

## Response Format

The Think tool returns the thought with a prefix indicating the thinking intensity level:

### Example Responses

**Default (`how_hard: "hard"`):**
```
I should use the think hard tool on this problem: The user wants to add a new API endpoint. I need to consider the request/response format, validation rules, and database queries required.
```

**Complex (`how_hard: "harder"`):**
```
I should use the think harder tool on this problem: This microservices architecture change affects authentication, data consistency, service discovery, and deployment pipelines. I need to map out all the interdependencies and potential failure points.
```

**Extremely Complex (`how_hard: "ultra"`):**
```
I should use the ultrathink tool on this problem: The system is experiencing cascading failures across multiple regions with database replication lag, CDN cache invalidation issues, and third-party service degradation all occurring simultaneously.
```

The prefix helps indicate the cognitive effort level applied to the problem, while the value comes from the structured thinking process itself.

---

For technical implementation details, see the [Think tool source documentation](../../internal/tools/think/README.md).

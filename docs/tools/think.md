# Think Tool

The Think tool provides a structured thinking space for AI agents during complex workflows, enabling better reasoning and decision-making through explicit thought processes.

## Overview

Based on Anthropic's research, the Think tool allows AI agents to pause and reason through complex problems before taking action. This leads to more accurate solutions and better handling of edge cases, particularly in complex scenarios.

## Features

- **Structured Reasoning**: Dedicated space for analysis and planning
- **Complex Problem Solving**: Break down multi-step problems systematically
- **Decision Support**: Evaluate options before choosing actions
- **Edge Case Handling**: Consider unusual scenarios and potential issues
- **Workflow Integration**: Seamlessly fits into existing tool chains

## Research Background

According to Anthropic's research, the Think tool provides:
- **54% relative improvement** in complex domain scenarios
- **Better consistency** across multiple trials
- **Enhanced handling** of edge cases and unusual scenarios

## When to Use the Think Tool

### Analysing Tool Outputs
Before acting on complex tool results:
```json
{
  "name": "think",
  "arguments": {
    "thought": "I need to analyse this API response before deciding which action to take next. The response contains multiple error codes and I should determine which is the primary issue to address first."
  }
}
```

### Breaking Down Complex Problems
For multi-step challenges:
```json
{
  "name": "think",
  "arguments": {
    "thought": "This deployment issue involves network configuration, database migrations, and service dependencies. Let me work through each component systematically to identify the root cause."
  }
}
```

### Reasoning Through Policy Decisions
When rules or constraints apply:
```json
{
  "name": "think",
  "arguments": {
    "thought": "The user is asking me to modify production configuration, but I should consider the security implications and whether this requires additional approvals or safeguards first."
  }
}
```

### Planning Sequential Actions
When mistakes could be costly:
```json
{
  "name": "think",
  "arguments": {
    "thought": "Before running this database migration, I should verify: 1) backup exists, 2) migration is reversible, 3) application can handle the schema changes, 4) timing won't impact users."
  }
}
```

### Processing Information from Previous Tools
After gathering data:
```json
{
  "name": "think",
  "arguments": {
    "thought": "I've collected performance metrics from three different monitoring tools. Now I need to correlate the data to identify patterns: the CPU spikes at 14:30 correspond with the database slow queries, suggesting a connection pool issue."
  }
}
```

## Usage Patterns

### Problem Analysis Pattern
```json
// 1. Gather information
{"name": "internet_search", "arguments": {"query": "kubernetes pod restart loop causes"}}

// 2. Think through the findings
{"name": "think", "arguments": {
  "thought": "The search results show several potential causes: resource limits, liveness probe failures, and image pull issues. I should check the pod logs and resource usage to narrow down the specific cause in this case."
}}

// 3. Take targeted action
{"name": "web_fetch", "arguments": {"url": "kubernetes.io/docs/troubleshooting"}}
```

### Decision Making Pattern
```json
// 1. Consider options
{"name": "think", "arguments": {
  "thought": "The user wants to implement authentication. I can see three approaches: JWT tokens, session-based auth, or OAuth integration. I need to consider their requirements: scalability, security level, and integration complexity."
}}

// 2. Evaluate trade-offs
{"name": "think", "arguments": {
  "thought": "Given they mentioned microservices and need stateless authentication, JWT tokens seem most appropriate. However, I should confirm they have secure token storage and refresh mechanisms in place."
}}

// 3. Implement solution
{"name": "package_search", "arguments": {"ecosystem": "npm", "query": "jsonwebtoken"}}
```

### Validation Pattern
```json
// 1. Implement solution
{"name": "document_processing", "arguments": {"source": "/path/to/requirements.pdf"}}

// 2. Validate approach
{"name": "think", "arguments": {
  "thought": "The document processing extracted the requirements, but I notice some technical details are in diagrams that might not have been captured fully. I should check if the extracted content covers all the functional requirements mentioned in the original request."
}}

// 3. Fill gaps if needed
{"name": "document_processing", "arguments": {"source": "/path/to/requirements.pdf", "profile": "llm-external"}}
```

## Best Practices

### Effective Thinking

**Be Specific**: Include concrete details and constraints
```json
{"thought": "The API returns 429 rate limit errors specifically for the /users endpoint after 100 requests per hour, but other endpoints work fine. This suggests endpoint-specific rate limiting rather than global limits."}
```

**Consider Multiple Angles**: Explore different perspectives
```json
{"thought": "From a security perspective, this approach exposes user data. From a performance perspective, it reduces database queries. From a maintenance perspective, it adds complexity. I need to weigh these trade-offs based on the project priorities."}
```

**Plan Ahead**: Think through consequences
```json
{"thought": "If I update this configuration, it will affect all microservices. I should: 1) test in staging first, 2) plan a rollback strategy, 3) coordinate with the team for the deployment window."}
```

### Integration with Other Tools

**Before Complex Operations**:
```json
{"name": "think", "arguments": {"thought": "Before running this package search across multiple ecosystems, I should consider which package managers are most relevant for this project and prioritise those results."}}
```

**After Information Gathering**:
```json
{"name": "think", "arguments": {"thought": "I've gathered documentation from three different sources. Let me identify the common patterns and any conflicting information that needs clarification."}}
```

**During Problem Solving**:
```json
{"name": "think", "arguments": {"thought": "The error message suggests a permission issue, but the logs show successful authentication. This contradiction indicates the problem might be at the resource level rather than authentication level."}}
```

## Common Use Cases

### Code Review and Analysis
```json
{
  "name": "think",
  "arguments": {
    "thought": "This code change modifies the authentication middleware. I should consider: 1) backward compatibility with existing tokens, 2) performance impact of additional validation, 3) security implications of the new claims structure."
  }
}
```

### Architecture Decisions
```json
{
  "name": "think",
  "arguments": {
    "thought": "The user wants to add real-time features. I need to evaluate: WebSockets vs Server-Sent Events vs polling. Factors: browser support, scaling requirements, infrastructure constraints, and development complexity."
  }
}
```

### Debugging Complex Issues
```json
{
  "name": "think",
  "arguments": {
    "thought": "The application works locally but fails in production. Key differences: environment variables, database connections, network configuration, and load balancing. I should systematically check each difference."
  }
}
```

### Risk Assessment
```json
{
  "name": "think",
  "arguments": {
    "thought": "This deployment involves database schema changes during business hours. Risks: downtime, data corruption, rollback complexity. Mitigations: feature flags, blue-green deployment, comprehensive testing."
  }
}
```

## Integration Examples

### Research and Analysis Workflow
```bash
# 1. Gather information
internet_search "kubernetes ingress nginx configuration"

# 2. Think through findings
think "The search results show multiple configuration approaches. I need to consider which matches their cloud provider and security requirements."

# 3. Get detailed documentation
package_documentation "nginx-ingress" --topic "configuration"

# 4. Plan implementation
think "Based on the documentation, I'll need to: 1) configure TLS certificates, 2) set up rate limiting, 3) configure health checks. Let me verify their current setup first."
```

### Problem Solving Workflow
```bash
# 1. Analyse the problem
think "The user reports intermittent 500 errors. This could be: resource exhaustion, database connection issues, external service failures, or application bugs. I need to gather more specific information."

# 2. Gather diagnostic information
web_fetch "https://their-monitoring-dashboard.com/metrics"

# 3. Correlate findings
think "The metrics show memory usage spikes correlating with the errors. This suggests a memory leak or inefficient memory usage during peak load periods."

# 4. Research solutions
internet_search "node.js memory leak detection production"
```

## Configuration

### Environment Variables

The Think tool supports the following configuration options:

- **`THINK_MAX_LENGTH`**: Maximum length for thought input in characters
  - **Default**: `2000`
  - **Description**: Controls the maximum length of thoughts to prevent resource exhaustion
  - **Example**: `THINK_MAX_LENGTH=5000` allows thoughts up to 5000 characters

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

The Think tool returns a simple confirmation:
```json
{
  "thought_recorded": true,
  "content": "Your thought has been recorded for reference",
  "timestamp": "2025-01-14T10:30:45Z"
}
```

The value comes from the cognitive process, not the response data.

---

For technical implementation details, see the [Think tool source documentation](../../internal/tools/think/README.md).

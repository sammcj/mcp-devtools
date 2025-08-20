# AWS Documentation Tool

The AWS documentation tool provides unified access to AWS official documentation through three action modes: search, fetch, and recommend.

- AWS documentation tool: "What is this service and how does it work?"
- Context7 tools (`resolve_library_id` & `get_library_docs`): "How do I code against this service?"

## Tool Overview

**Tool Name:** `aws_documentation`
**Actions:** `search`, `fetch`, `recommend`
**Enablement:** Requires `ENABLE_ADDITIONAL_TOOLS=aws`

## Parameters

### Required
- `action` (string): Action to perform - one of "search", "fetch", or "recommend"

### Action-Specific Parameters

For `search` action:
- `search_phrase` (required): Search terms for finding AWS documentation
- `limit` (optional): Maximum results to return (1-50, default: 5)

For `fetch` action:
- `url` (required): AWS documentation URL (must be from docs.aws.amazon.com and end with .html)
- `max_length` (optional): Maximum characters to return (default: 5000)
- `start_index` (optional): Starting character index for pagination (default: 0)

For `recommend` action:
- `url` (required): AWS documentation URL to get recommendations for

## Usage Examples

### Search for Documentation
```json
{
  "name": "aws_documentation",
  "arguments": {
    "action": "search",
    "search_phrase": "S3 bucket versioning",
    "limit": 5
  }
}
```

**Returns:**
- `action`: "search"
- `search_phrase`: Original search query
- `results_count`: Number of results found
- `results`: Array of search results with rank_order, url, title, and context

### Fetch Documentation Content
```json
{
  "name": "aws_documentation",
  "arguments": {
    "action": "fetch",
    "url": "https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html"
  }
}
```

**Returns:**
- `action`: "fetch"
- `url`: Original documentation URL
- `content`: Converted markdown content
- `total_length`: Total content length in characters
- `start_index`: Starting character index used
- `end_index`: Ending character index
- `has_more_content`: Boolean indicating if pagination is needed
- `next_start_index`: Next starting index for continuation (if applicable)
- `pagination_hint`: Instructions for continuing pagination

### Get Content Recommendations
```json
{
  "name": "aws_documentation",
  "arguments": {
    "action": "recommend",
    "url": "https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-express-one-zone.html"
  }
}
```

**Returns:**
- `action`: "recommend"
- `url`: Original documentation URL
- `recommendations`: Array of recommendation results with url, title, and context
- `recommendations_count`: Number of recommendations found

## Configuration

The AWS tools are **disabled by default** for security purposes. Enable them by adding to your MCP configuration:

```json
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "/path/to/mcp-devtools",
      "env": {
        "ENABLE_ADDITIONAL_TOOLS": "aws"
      }
    }
  }
}
```

**No API keys required** - these tools use AWS's public documentation APIs.

## Common Workflows

### AWS Service Research
1. Use `search` action to find relevant AWS service pages
2. Use `fetch` action with URLs from search results to read detailed content
3. Use `recommend` action to discover related AWS services and features
4. Use pagination for large documents with `start_index` and `max_length`

### AWS Strands Agents SDK Learning
1. Use `resolve_library_id` with 'strands agents' to find available library IDs
2. Use `get_library_docs` with the appropriate library ID (e.g., '/strands-agents/sdk-python')
3. Available library IDs include:
   - `/strands-agents/docs` - General documentation
   - `/strands-agents/sdk-python` - Python SDK
   - `/strands-agents/samples` - Code samples
   - `/strands-agents/tools` - Tools documentation

### Complete AWS Documentation Research
1. Search for broad topic (e.g., "Lambda security")
2. Read most relevant results using fetch action
3. Get recommendations from key pages
4. For Strands Agents documentation, use the package documentation tools

## Best Practices

### Search Optimisation
- Include AWS service names for targeted results
- Use specific technical terms rather than general phrases
- Combine multiple keywords for precision
- Try alternative terms if initial search is insufficient

### Pagination Management
- Start with default max_length (5000 characters)
- Check `has_more_content` field before continuing
- Use `next_start_index` from response for continuation
- Increase max_length for fewer pagination requests

### Content Discovery
- Use recommendations after reading important pages
- Check "New" recommendations for latest AWS features
- Follow "Highly Rated" recommendations for popular topics
- Use "Journey" recommendations for complete workflows

### Strands SDK Usage
- Use `resolve_library_id` tool to find appropriate Strands library IDs
- Use `get_library_docs` with specific topics for focused documentation
- Library IDs are in format '/strands-agents/[component]'

## URL Requirements

All AWS documentation URLs must:
- Use the `docs.aws.amazon.com` domain
- Use HTTPS protocol
- End with `.html` file extension

**Valid Examples:**
- `https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html`
- `https://docs.aws.amazon.com/lambda/latest/dg/lambda-invocation.html`
- `https://docs.aws.amazon.com/ec2/latest/userguide/concepts.html`

## Integration Examples

### Complete Research Workflow
```javascript
// 1. Search for AWS service documentation
{
  "action": "search",
  "search_phrase": "Lambda environment variables"
}

// 2. Read specific result
{
  "action": "fetch",
  "url": "https://docs.aws.amazon.com/lambda/latest/dg/configuration-envvars.html"
}

// 3. Get related recommendations
{
  "action": "recommend",
  "url": "https://docs.aws.amazon.com/lambda/latest/dg/configuration-envvars.html"
}
```

This tool provides a consistent interface for AWS documentation needs.

## AWS Strands Agents SDK Documentation

The AWS documentation tool focuses on official AWS service documentation. For AWS Strands Agents SDK documentation, use these complementary tools:

1. **resolve_library_id**: Find the correct Context7 library ID for Strands components
2. **get_library_docs**: Retrieve comprehensive documentation for specific Strands libraries

Available Strands Library IDs:

- `/strands-agents/docs` - General Strands Agents SDK documentation
- `/strands-agents/sdk-python` - Python SDK specific documentation
- `/strands-agents/samples` - Sample code and examples
- `/strands-agents/tools` - Tools documentation

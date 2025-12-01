# AWS Documentation Tool

The AWS documentation tool provides unified access to AWS official documentation and pricing information through five action modes: search, fetch, recommend, list_pricing_services, and get_service_pricing.

- AWS documentation tool: "What is this service and how does it work?"
- AWS pricing tool: "How much does this service cost?"
- Context7 tools (`resolve_library_id` & `get_library_documentation`): "How do I code against this service?"

## Tool Overview

**Tool Name:** `aws_documentation`
**Actions:** `search`, `fetch`, `recommend`, `list_pricing_services`, `get_service_pricing`
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

For `list_pricing_services` action:
- No additional parameters required

For `get_service_pricing` action:
- `service_code` (required): AWS service code (e.g., "AmazonEC2", "AmazonS3")
- `max_results` (optional): Maximum number of products to return (default: 10)
- `filters` (optional): Array of filter objects, each containing:
  - `field` (required): Attribute name to filter on (e.g., "instanceType", "location", "operatingSystem")
  - `value` (required): Value to match (e.g., "t2.micro", "US East (N. Virginia)")
  - `type` (optional): Comparison type - "TERM_MATCH" (default)

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

### List All AWS Services with Pricing
```json
{
  "name": "aws_documentation",
  "arguments": {
    "action": "list_pricing_services"
  }
}
```

**Returns:**
- `action`: "list_pricing_services"
- `services_count`: Number of services with pricing data
- `services`: Array of AWS service codes (e.g., ["AmazonEC2", "AmazonS3", "AmazonRDS", ...])

### Get Service Pricing
```json
{
  "name": "aws_documentation",
  "arguments": {
    "action": "get_service_pricing",
    "service_code": "AmazonEC2",
    "max_results": 5
  }
}
```

**Returns:**
- `action`: "get_service_pricing"
- `service_code`: AWS service code
- `product_count`: Number of products returned
- `price_list`: Array of pricing data (JSON strings containing product details and pricing terms)

### Get Filtered Pricing
```json
{
  "name": "aws_documentation",
  "arguments": {
    "action": "get_service_pricing",
    "service_code": "AmazonEC2",
    "filters": [
      {"field": "instanceType", "value": "t2.micro"},
      {"field": "location", "value": "US East (N. Virginia)"},
      {"field": "operatingSystem", "value": "Linux"}
    ],
    "max_results": 3
  }
}
```

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

### AWS Credentials for Pricing

**Documentation actions** (`search`, `fetch`, `recommend`) use AWS's public APIs and **require no credentials**.

**Pricing actions** (`list_pricing_services`, `get_service_pricing`) use the AWS Pricing API and **require AWS credentials**. The tool automatically detects credentials from:

1. **Environment variables**:
   - `AWS_ACCESS_KEY_ID`
   - `AWS_SECRET_ACCESS_KEY`
   - `AWS_SESSION_TOKEN` (for temporary credentials/SSO)

2. **AWS SSO**: If logged in via `aws sso login`, credentials are automatically used

3. **Shared credentials file**: `~/.aws/credentials` and `~/.aws/config`

4. **IAM instance profile**: When running on EC2

If pricing actions are called without credentials, a clear error message will be returned. Documentation actions continue to work without credentials.

## Common Workflows

### AWS Service Research
1. Use `search` action to find relevant AWS service pages
2. Use `fetch` action with URLs from search results to read detailed content
3. Use `recommend` action to discover related AWS services and features
4. Use pagination for large documents with `start_index` and `max_length`

### AWS Pricing Research
1. Ensure you have AWS credentials configured (see Configuration section)
2. Use `list_pricing_services` to discover available AWS services with pricing
3. Use `get_service_pricing` with service code to get pricing information
4. Apply filters to narrow down pricing results (instance types, locations, operating systems, etc.)
5. Pricing data is fetched on-demand via AWS Pricing API - no caching required

### AWS Strands Agents SDK Learning
1. Use `resolve_library_id` with 'strands agents' to find available library IDs
2. Use `get_library_documentation` with the appropriate library ID (e.g., '/strands-agents/sdk-python')
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
- Use `get_library_documentation` with specific topics for focused documentation
- Library IDs are in format '/strands-agents/[component]'

### Pricing Filters
- Use `field` and `value` pairs to filter pricing results
- Common EC2 filters: `instanceType`, `location`, `operatingSystem`, `tenancy`, `preInstalledSw`
- Common S3 filters: `storageClass`, `location`, `volumeType`
- Location values use AWS's descriptive names (e.g., "US East (N. Virginia)", "EU (Ireland)")
- Combine multiple filters in the array to narrow results effectively
- Filters use exact matching by default (TERM_MATCH type)

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
2. **get_library_documentation**: Retrieve comprehensive documentation for specific Strands libraries

Available Strands Library IDs:

- `/strands-agents/docs` - General Strands Agents SDK documentation
- `/strands-agents/sdk-python` - Python SDK specific documentation
- `/strands-agents/samples` - Sample code and examples
- `/strands-agents/tools` - Tools documentation

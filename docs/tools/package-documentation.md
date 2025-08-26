# Package Documentation Tool

The Package Documentation tool provides access to comprehensive library documentation through the Context7 API, giving you detailed documentation for popular libraries and frameworks.

## Overview

Instead of hunting through multiple documentation sites, this tool retrieves focused, up-to-date documentation for libraries across different programming languages. It's particularly useful for understanding APIs, getting usage examples, and exploring library capabilities.

## Features

- **Comprehensive Coverage**: Documentation for popular libraries across languages
- **Topic-Focused Search**: Get documentation specific to topics like "hooks", "routing", "authentication"
- **Trust Scoring**: Results prioritised by documentation quality and completeness
- **Code Examples**: Real-world usage examples and snippets
- **Version-Specific**: Access documentation for specific library versions

## Two-Step Process

The tool uses a two-step process for optimal results:

1. **`resolve_library_id`**: Find the correct library identifier
2. **`get_library_docs`**: Retrieve detailed documentation

## Usage Examples

While intended to be activated via a prompt to an agent, below are some example JSON tool calls.

### Step 1: Resolve Library ID

Find the correct library identifier:

```json
{
  "name": "resolve_library_id",
  "arguments": {
    "libraryName": "react"
  }
}
```

**Response includes:**
- Exact library identifier (e.g., `/facebook/react`)
- Alternative matches if multiple libraries found
- Trust scores and documentation coverage metrics
- Rationale for the selected match

### Step 2: Get Documentation

Use the resolved ID to fetch documentation:

```json
{
  "name": "get_library_docs",
  "arguments": {
    "context7CompatibleLibraryID": "/facebook/react",
    "topic": "hooks",
    "tokens": 15000
  }
}
```

## Complete Examples

### React Hooks Documentation
```json
// Step 1: Resolve
{
  "name": "resolve_library_id",
  "arguments": {
    "libraryName": "react"
  }
}

// Step 2: Get docs focused on hooks
{
  "name": "get_library_docs",
  "arguments": {
    "context7CompatibleLibraryID": "/facebook/react",
    "topic": "hooks",
    "tokens": 15000
  }
}
```

### Express.js Routing Documentation
```json
// Step 1: Resolve
{
  "name": "resolve_library_id",
  "arguments": {
    "libraryName": "express"
  }
}

// Step 2: Get routing-specific docs
{
  "name": "get_library_docs",
  "arguments": {
    "context7CompatibleLibraryID": "/expressjs/express",
    "topic": "routing",
    "tokens": 10000
  }
}
```

### TensorFlow Machine Learning Documentation
```json
// Step 1: Resolve
{
  "name": "resolve_library_id",
  "arguments": {
    "libraryName": "tensorflow"
  }
}

// Step 2: Get ML-specific docs
{
  "name": "get_library_docs",
  "arguments": {
    "context7CompatibleLibraryID": "/tensorflow/tensorflow",
    "topic": "machine learning",
    "tokens": 20000
  }
}
```

### Django Authentication Documentation
```json
// Step 1: Resolve
{
  "name": "resolve_library_id",
  "arguments": {
    "libraryName": "django"
  }
}

// Step 2: Get authentication docs
{
  "name": "get_library_docs",
  "arguments": {
    "context7CompatibleLibraryID": "/django/django",
    "topic": "authentication",
    "tokens": 12000
  }
}
```

## Parameters Reference

### `resolve_library_id`

| Parameter | Required | Description |
|-----------|----------|-------------|
| `libraryName` | ✅ | Library name to search for (e.g., "react", "tensorflow", "express") |

### `get_library_docs`

| Parameter | Required | Description |
|-----------|----------|-------------|
| `context7CompatibleLibraryID` | ✅ | Exact library ID from `resolve_library_id` |
| `topic` | ❌ | Focus on specific topics (e.g., "hooks", "routing", "authentication") |
| `tokens` | ❌ | Maximum tokens to retrieve (default: 10,000, max: 100,000) |

## Topic Examples

### Frontend Development
- **"hooks"**: React hooks, Vue composition API
- **"components"**: Component patterns and best practices
- **"routing"**: Client-side routing, navigation
- **"state management"**: Redux, Vuex, state patterns
- **"styling"**: CSS-in-JS, styled components

### Backend Development
- **"authentication"**: User auth, JWT, sessions
- **"database"**: ORM usage, queries, migrations
- **"api"**: REST API design, GraphQL
- **"middleware"**: Request/response processing
- **"testing"**: Unit tests, integration tests

### Data Science & ML
- **"machine learning"**: Model training, algorithms
- **"data processing"**: Data manipulation, cleaning
- **"visualization"**: Charts, graphs, plotting
- **"neural networks"**: Deep learning, layers
- **"preprocessing"**: Feature engineering, scaling

### Mobile Development
- **"navigation"**: Screen navigation, routing
- **"animations"**: UI animations, transitions
- **"storage"**: Local storage, databases
- **"networking"**: HTTP requests, API calls
- **"performance"**: Optimization, profiling

## Response Format

### Library Resolution Response
```json
{
  "selected_library": {
    "id": "/facebook/react",
    "name": "React",
    "description": "A JavaScript library for building user interfaces",
    "trust_score": 9.5,
    "code_snippets": 1247,
    "documentation_coverage": "comprehensive"
  },
  "alternatives": [
    {
      "id": "/reactjs/react-router",
      "name": "React Router",
      "trust_score": 8.8,
      "relevance": "related"
    }
  ],
  "selection_rationale": "Exact name match with highest trust score and comprehensive documentation coverage"
}
```

### Documentation Response
```json
{
  "library_id": "/facebook/react",
  "topic": "hooks",
  "content": "# React Hooks Documentation\n\nHooks are functions that let you use state and other React features...",
  "metadata": {
    "tokens_used": 12450,
    "tokens_requested": 15000,
    "content_sections": ["useState", "useEffect", "useContext", "Custom Hooks"],
    "code_examples": 15,
    "last_updated": "2024-01-15"
  }
}
```

## Common Workflows

### Learning New Library
1. Resolve library ID to ensure correct documentation
2. Get general overview without topic filter
3. Focus on specific topics as you dive deeper

### Solving Specific Problem
1. Resolve library ID for the relevant library
2. Use topic-focused search for your specific need
3. Increase token limit for comprehensive coverage

### Comparing Libraries
1. Resolve multiple library IDs for alternatives
2. Get documentation for same topics across libraries
3. Compare approaches and features

## Configuration

### Environment Variables

The Package Documentation tool supports the following configuration options:

- **`CONTEXT7_API_KEY`**: API key for Context7 authentication
  - **Default**: Not set (anonymous requests)
  - **Description**: Provides higher rate limits and authentication with Context7 API. Get your API key at [context7.com/console](https://context7.com/console)
  - **Example**: `CONTEXT7_API_KEY=your_api_key_here`

- **`PACKAGE_DOCS_RATE_LIMIT`**: Maximum HTTP requests per second to the Context7 API
  - **Default**: `10`
  - **Description**: Controls the rate of HTTP requests to prevent overwhelming the Context7 API service
  - **Example**: `PACKAGE_DOCS_RATE_LIMIT=15` allows up to 15 requests per second
  
### Security Features

- **Rate Limiting**: Configurable request rate limiting protects against overwhelming external documentation APIs
- **Input Validation**: Comprehensive validation of library IDs and search parameters
- **Error Handling**: Graceful handling of network issues and API failures
- **Trusted Sources**: Only queries the established Context7 documentation service

## Best Practices

### Effective Topic Selection
- **Be specific**: "authentication" vs "auth"
- **Use common terms**: "routing" vs "navigation patterns"
- **Match library terminology**: React "hooks" vs "composition API"
- **Combine related topics**: "hooks useState useEffect"

### Token Management
- **Start small**: Use default 10,000 tokens initially
- **Scale up**: Increase to 20,000+ for comprehensive topics
- **Consider limits**: Maximum 100,000 tokens per request

### Library ID Usage
- **Always resolve first**: Don't guess library IDs
- **Check alternatives**: Review alternative matches for better fits
- **Use exact IDs**: Copy the exact ID from resolution response

## Integration Examples

### Research Workflow
```json
// 1. Find the library
{"name": "resolve_library_id", "arguments": {"libraryName": "vue"}}

// 2. Get overview
{"name": "get_library_docs", "arguments": {"context7CompatibleLibraryID": "/vuejs/vue"}}

// 3. Focus on specific needs
{"name": "get_library_docs", "arguments": {"context7CompatibleLibraryID": "/vuejs/vue", "topic": "composition api"}}
```

### Problem-Solving Workflow
```json
// 1. Identify library
{"name": "resolve_library_id", "arguments": {"libraryName": "django"}}

// 2. Get targeted help
{"name": "get_library_docs", "arguments": {"context7CompatibleLibraryID": "/django/django", "topic": "forms validation", "tokens": 15000}}
```

### Learning Workflow
```json
// 1. Start with fundamentals
{"name": "get_library_docs", "arguments": {"context7CompatibleLibraryID": "/facebook/react", "topic": "components"}}

// 2. Progress to advanced topics
{"name": "get_library_docs", "arguments": {"context7CompatibleLibraryID": "/facebook/react", "topic": "performance optimization"}}
```

## Error Handling

### Common Issues
- **Library not found**: Try alternative names or check spelling
- **No topic match**: Use broader terms or remove topic filter
- **Token limit exceeded**: Reduce token request or use more specific topics
- **Documentation unavailable**: Library may not be in Context7 database

### Fallback Strategies
1. **Try alternative library names**: "reactjs" vs "react"
2. **Use broader topics**: "auth" instead of "jwt authentication"
3. **Remove topic filter**: Get general documentation first
4. **Check library alternatives**: Use suggested alternatives from resolution

---

For technical implementation details, see the [Package Documentation source documentation](../../internal/tools/packagedocs/README.md).

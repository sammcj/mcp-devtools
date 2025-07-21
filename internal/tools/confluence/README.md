# Confluence Search Tool

The Confluence search tool enables searching your Confluence instance and retrieving content converted to clean Markdown format. It fetches the top matching results and converts them from Confluence's storage format (XHTML) to readable Markdown, removing navigation elements and other non-content components.

## Features

- **Full-text search** using Confluence CQL (Confluence Query Language)
- **Content retrieval** with automatic conversion to Markdown
- **Space filtering** to limit searches to specific Confluence spaces
- **Content type filtering** (pages, blog posts, comments, attachments)
- **Clean conversion** from Confluence storage format to Markdown
- **Configurable results** (1-10 results, default 3)
- **Caching** for improved performance on repeated searches

## Configuration

The tool supports two authentication methods: **OAuth 2.0** (recommended) and **Basic Authentication**.

### OAuth 2.0 Authentication (Recommended)

For Confluence instances that require OAuth 2.0 authentication:

```bash
CONFLUENCE_URL="https://your-company.atlassian.net/wiki"
CONFLUENCE_OAUTH_CLIENT_ID="your-oauth-client-id"
CONFLUENCE_OAUTH_CLIENT_SECRET="your-oauth-client-secret"
CONFLUENCE_OAUTH_ISSUER_URL="https://your-oauth-provider.com"
CONFLUENCE_OAUTH_SCOPE="read:confluence-content.all"  # Optional
CONFLUENCE_OAUTH_TOKEN_FILE="/path/to/token/cache.json"  # Optional
```

The OAuth implementation:
- Uses the **Authorization Code flow with PKCE** for security
- **Caches tokens** automatically to avoid repeated authentication
- **Opens a browser** for interactive authentication when needed
- Supports **token refresh** (if provided by the OAuth server)

#### For Regular Users (No Admin Access Required)

**If your organisation uses SAML/SSO for Confluence**, you have several authentication options:

### Option 1: Automatic Browser Cookie Extraction (Easiest for SAML)

The tool can automatically extract session cookies from your browser after you've logged in through SAML:

1. **Log into Confluence** through your browser (complete the SAML flow)
2. **Configure automatic extraction**:
   ```bash
   CONFLUENCE_URL="https://your-company.atlassian.net/wiki"
   CONFLUENCE_BROWSER_TYPE="chrome"  # or firefox, brave, edge, chromium, firefox-nightly
   ```

**Supported Browsers**:
- **Chrome** (`chrome`)
- **Firefox** (`firefox`)
- **Brave** (`brave`)
- **Microsoft Edge** (`edge`)
- **Chromium** (`chromium`)
- **Firefox Nightly** (`firefox-nightly`)
- **Safari** (`safari`) - macOS only, limited support

**Note**: The tool will automatically find and extract the necessary cookies from your browser's database. Session cookies expire (usually after a few hours or days), so you may need to re-authenticate in your browser periodically.

### Option 2: Manual Session Cookie Authentication

If automatic extraction doesn't work, you can manually extract cookies:

1. **Log into Confluence** through your browser (complete the SAML flow)
2. **Extract session cookies** from your browser:
   - **Chrome/Edge**: Press F12 → Application tab → Cookies → Select your Confluence domain
   - **Firefox**: Press F12 → Storage tab → Cookies → Select your Confluence domain
   - **Safari**: Develop menu → Web Inspector → Storage tab → Cookies
3. **Copy the cookie values** - look for cookies like `JSESSIONID`, `seraph.rememberme.cookie`, `atlassian.xsrf.token`
4. **Format as a cookie string**: `JSESSIONID=ABC123; seraph.rememberme.cookie=XYZ789; atlassian.xsrf.token=DEF456`
5. **Configure environment variable**:
   ```bash
   CONFLUENCE_URL="https://your-company.atlassian.net/wiki"
   CONFLUENCE_SESSION_COOKIES="JSESSIONID=ABC123; seraph.rememberme.cookie=XYZ789; atlassian.xsrf.token=DEF456"
   ```

### Option 2: Check for Existing OAuth App

1. **In Okta**, go to your Confluence app (the one with URL like `https://yourcompany.okta.com/app/atlassian/your_id/sso/saml/confluence`)
2. **Look for an "OAuth" or "API" section** in the app settings
3. **If you see a Client ID**, that's what you need for OAuth configuration

### Option 3: Try API Token Authentication (May Not Work with SAML)

1. **Generate API Token**:
   - Go to [Atlassian Account Settings](https://id.atlassian.com/manage-profile/security/api-tokens)
   - Click **Create API token**
   - Give it a label like "MCP DevTools"
2. **Configure**:
   ```bash
   CONFLUENCE_URL="https://your-company.atlassian.net/wiki"
   CONFLUENCE_USERNAME="your-email@company.com"
   CONFLUENCE_TOKEN="your-api-token-from-step-1"
   ```

#### For Organisations with Existing OAuth Apps

If your IT department has already set up an OAuth application for Confluence access, they can provide you with:

1. **Client ID** (public, safe to share)
2. **Issuer URL** (your Okta/OAuth provider URL)
3. **Scope** (what permissions the app has)

Then configure:
```bash
CONFLUENCE_URL="https://your-company.atlassian.net/wiki"
CONFLUENCE_OAUTH_CLIENT_ID="client-id-from-it-department"
CONFLUENCE_OAUTH_CLIENT_SECRET=""  # Usually empty for public clients
CONFLUENCE_OAUTH_ISSUER_URL="https://your-org.okta.com"
CONFLUENCE_OAUTH_SCOPE="openid profile email"
```

### Basic Authentication (Legacy)

For traditional username/password or API token authentication:

```bash
CONFLUENCE_URL="https://your-company.atlassian.net/wiki"
CONFLUENCE_USERNAME="your-email@company.com"
CONFLUENCE_TOKEN="your-api-token-or-password"
```

#### Basic Auth Options

1. **API Token** (recommended for Atlassian Cloud):
   - Generate an API token at https://id.atlassian.com/manage-profile/security/api-tokens
   - Use your email as `CONFLUENCE_USERNAME` and the token as `CONFLUENCE_TOKEN`

2. **Basic Auth** (for Server/Data Center):
   - Use your username and password
   - Set `CONFLUENCE_USERNAME` to your username and `CONFLUENCE_TOKEN` to your password

## Usage Examples

### Basic Search
```json
{
  "query": "API documentation"
}
```

### Search in Specific Space
```json
{
  "query": "deployment guide",
  "space_key": "DEV"
}
```

### Limit Results and Content Types
```json
{
  "query": "security best practices",
  "max_results": 5,
  "content_types": ["page", "blogpost"]
}
```

## Parameters

| Parameter       | Type   | Required | Description                                                  |
|-----------------|--------|----------|--------------------------------------------------------------|
| `query`         | string | Yes      | Search query string to find content                          |
| `space_key`     | string | No       | Confluence space key to limit search scope                   |
| `max_results`   | number | No       | Maximum results to fetch (1-10, default: 3)                  |
| `content_types` | array  | No       | Filter by content types: page, blogpost, comment, attachment |

## Response Format

```json
{
  "query": "search query",
  "results": [
    {
      "id": "123456",
      "type": "page",
      "title": "Page Title",
      "space": {
        "key": "DEV",
        "name": "Development",
        "type": "global"
      },
      "url": "https://confluence.example.com/rest/api/content/123456",
      "web_url": "https://confluence.example.com/display/DEV/Page+Title",
      "last_modified": "2024-01-15T10:30:00Z",
      "author": {
        "account_id": "abc123",
        "display_name": "John Doe",
        "email": "john.doe@company.com"
      },
      "content": "# Page Title\n\nThis is the converted markdown content...",
      "content_preview": "Page Title - This is the converted markdown content...",
      "metadata": {
        "version": 3,
        "status": "current",
        "content_type": "page"
      }
    }
  ],
  "total_count": 15
}
```

## Content Conversion

The tool converts Confluence's storage format (XHTML) to clean Markdown:

### Supported Elements
- **Headers** (h1-h6) → Markdown headers (#, ##, etc.)
- **Paragraphs** → Standard paragraphs with line breaks
- **Lists** (ordered/unordered) → Markdown lists (-, 1., etc.)
- **Tables** → Markdown tables
- **Links** → `[text](url)` format
- **Images** → `![alt](src)` format
- **Code blocks** → Fenced code blocks with language detection
- **Inline code** → Backtick-wrapped code
- **Bold/italic** → `**bold**` and `*italic*`
- **Blockquotes** → `>` prefixed lines

### Confluence-Specific Elements
- **Code macro** → Fenced code blocks with language
- **Info/Note/Warning panels** → Blockquotes with titles
- **Quote macro** → Blockquotes
- **Table of Contents** → Text placeholder

## Error Handling

The tool provides structured error responses:

```json
{
  "error": "Authentication failed: Invalid credentials",
  "query": "search query",
  "success": false
}
```

Common errors:
- **Configuration errors**: Missing environment variables
- **Authentication errors**: Invalid credentials or expired tokens
- **API errors**: Network issues, rate limiting, or service unavailability
- **Content errors**: Malformed content or conversion failures

## Performance

- **Caching**: Search results are cached for 5 minutes to improve performance
- **Rate limiting**: Respects Confluence API rate limits
- **Concurrent requests**: Handles multiple content fetches efficiently
- **Content size**: Large pages are handled gracefully with proper error handling

## Permissions

The tool respects Confluence permissions and will only return content that the authenticated user has access to view. Ensure the configured user has appropriate read permissions for the spaces and content you want to search.

## Troubleshooting

### Tool Not Available
If the Confluence tool doesn't appear in the MCP server:
- Verify all required environment variables are set
- Check that the `CONFLUENCE_URL` is accessible
- Ensure credentials are valid

### Authentication Issues
- For Atlassian Cloud, use API tokens instead of passwords
- Verify the API token hasn't expired
- Check that the user has permission to access the Confluence instance

### Search Returns No Results
- Verify the search query syntax
- Check that the user has permission to view the content
- Try searching without space or content type filters
- Confirm the content exists and is published

### Content Conversion Issues
- Large or complex pages may take longer to process
- Some custom Confluence macros may not convert perfectly
- HTML content is converted to Markdown on a best-effort basis

# Streaming Progress Notifications for project_actions Tool

## Problem
The `project_actions` tool's "test" operation doesn't stream output to the caller as it arrives. Currently, all output is buffered and returned only after the command completes.

## MCP Progress Notification Protocol

According to the [MCP specification](https://modelcontextprotocol.io/specification/2025-03-26/basic/utilities/progress):

### How It Works
1. **Client includes progressToken in request metadata**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "_meta": {
      "progressToken": "abc123"
    },
    "name": "project_actions",
    "arguments": {"operation": "test"}
  }
}
```

2. **Server sends progress notifications** during execution:
```json
{
  "jsonrpc": "2.0",
  "method": "notifications/progress",
  "params": {
    "progressToken": "abc123",
    "progress": 50,
    "total": 100,
    "message": "Running tests..."
  }
}
```

3. **Server returns final result** when complete.

### Key Requirements
- Progress notifications **MUST** only reference tokens from active requests
- The `progress` value **MUST** increase with each notification
- The `progress` and `total` values **MAY** be floating point
- The `message` field **SHOULD** provide human-readable progress information
- Servers **MAY** choose not to send any progress notifications
- Progress notifications **MUST** stop after completion

## mark3labs/mcp-go Library Support

The mcp-go library has **excellent built-in support** for progress notifications:

### Built-in Types

```go
// ProgressToken type
type ProgressToken any

// Meta struct with ProgressToken field
type Meta struct {
    ProgressToken    ProgressToken
    AdditionalFields map[string]any
}

// ProgressNotification for sending notifications
type ProgressNotification struct {
    Notification
    Params ProgressNotificationParams `json:"params"`
}

// ProgressNotificationParams with all required fields
type ProgressNotificationParams struct {
    ProgressToken ProgressToken `json:"progressToken"`
    Progress      float64       `json:"progress"`
    Total         float64       `json:"total,omitempty"`
    Message       string        `json:"message,omitempty"`
}
```

### Usage

The library automatically:
- Extracts `progressToken` from request `_meta` field
- Provides proper JSON marshaling/unmarshaling
- Offers type-safe structures for progress notifications

Use `server.SendNotificationToClient(ctx, "notifications/progress", params)` with the params structure already defined.

## Solution for project_actions

### 1. Extract progressToken from Request

In `project_actions.go`, modify the `Execute` method to extract the progress token:

```go
func (t *ProjectActionsTool) Execute(ctx context.Context, logger *logrus.Logger, cache *sync.Map, args map[string]any) (*mcp.CallToolResult, error) {
    // Extract progress token from _meta if present
    var progressToken string
    if meta, ok := args["_meta"].(map[string]any); ok {
        if token, ok := meta["progressToken"].(string); ok {
            progressToken = token
        }
    }

    // ... rest of existing code ...
}
```

### 2. Pass progressToken to executeCommand

Modify the command execution to accept and use the progress token:

```go
func (t *ProjectActionsTool) executeCommand(ctx context.Context, cmd *exec.Cmd, progressToken string) (*CommandResult, error) {
    // ... existing pipe setup ...

    // Start command
    start := time.Now()
    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("failed to start command: %w", err)
    }

    // Get server from context to send progress notifications
    server := registry.ServerFromContext(ctx)
    session := server.ClientSessionFromContext(ctx)

    // Stream stdout with progress notifications
    wg.Add(1)
    go func() {
        defer wg.Done()
        scanner := bufio.NewScanner(stdoutPipe)
        lineCount := 0
        for scanner.Scan() {
            line := scanner.Text()
            stdoutBuf.WriteString(line + "\n")
            lineCount++

            // Send progress notification if token provided
            if progressToken != "" && server != nil && session != nil {
                server.SendNotificationToClient(ctx, "notifications/progress", map[string]any{
                    "progressToken": progressToken,
                    "progress":      lineCount,
                    "message":       line,
                })
            }
        }
    }()

    // Similar for stderr...

    // ... rest of existing code ...
}
```

### 3. Update Call Sites

Update all calls to `executeCommand` to pass the progress token:

```go
result, err = t.executeMakeTarget(ctx, operation, dryRun, progressToken)
result, err = t.executeGitAdd(ctx, pathStrs, dryRun, progressToken)
result, err = t.executeGitCommit(ctx, message, dryRun, progressToken)
```

### 4. Access Server Instance

The tool needs access to the MCP server instance. Two options:

**Option A: Store server reference in tool**
```go
type ProjectActionsTool struct {
    // ... existing fields ...
    server *server.MCPServer
}

// In init() or registration
tool.server = serverInstance
```

**Option B: Use context (cleaner)**
```go
// In registry or server setup, add server to context
ctx = context.WithValue(ctx, serverKey{}, server)

// In tool, retrieve from context
if srv := registry.ServerFromContext(ctx); srv != nil {
    srv.SendNotificationToClient(ctx, "notifications/progress", params)
}
```

## Implementation Steps

1. **Add progressToken extraction** in `Execute` method
2. **Modify executeCommand** to stream output with progress notifications
3. **Update executeMakeTarget, executeGitAdd, executeGitCommit** signatures
4. **Add server context access** mechanism
5. **Test with MCP client** that supports progress tokens

## Benefits

- Real-time feedback during long-running operations
- Better user experience for test execution
- Follows MCP specification correctly
- No breaking changes (progress is optional)

## Notes

- Progress notifications are **optional** - if no progressToken is provided, behavior remains unchanged
- The client must support progress notifications to see streaming output
- Amazon Q and other MCP clients may or may not display progress notifications depending on their implementation
- Consider rate-limiting progress notifications to avoid flooding (e.g., batch lines or use time-based throttling)

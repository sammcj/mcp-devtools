# Collaboration Tool (`collab` / `collab_wait`)

Cross-agent communication tool that enables two AI coding agents to exchange structured messages through a shared filesystem mailbox. Useful when agents working on related projects need to coordinate -- e.g. Project A needs API changes from Project B's library.

**Enable**: `ENABLE_ADDITIONAL_TOOLS=collab` (also enables `collab_wait`)

## How It Works

Agents communicate through **collaboration sessions** identified by UUIDs:

1. Agent A calls `create_session` -- gets a session UUID
2. The human copies the UUID to Agent B's chat
3. Agent B calls `join_session` with the UUID -- sees the topic and any existing messages
4. Both agents exchange messages via `post` and `check` (or `collab_wait` to block until a reply arrives)
5. Either agent calls `close` when done

Sessions are stored on the local filesystem. Both agents must have access to the same machine (or shared filesystem).

## Actions

### `create_session`

Start a new collaboration session.

| Parameter | Required | Description                                                |
| --------- | -------- | ---------------------------------------------------------- |
| `topic`   | Yes      | What the collaboration is about (max 500 chars)            |
| `name`    | No       | Participant name (auto-detected from MCP Roots if omitted) |

```json
{
  "action": "create_session",
  "topic": "Add streaming support to library-b",
  "name": "project-a"
}
```

Response:

```json
{
  "session_id": "f39be8d5-416c-442a-ba14-a492b1d249da",
  "topic": "Add streaming support to library-b",
  "participant": "project-a",
  "status": "active",
  "hints": "Share this session_id with the other agent..."
}
```

### `join_session`

Join an existing session by UUID. Returns the topic and all existing messages so the joining agent has full context.

| Parameter    | Required | Description      |
| ------------ | -------- | ---------------- |
| `session_id` | Yes      | Session UUID     |
| `name`       | No       | Participant name |

```json
{
  "action": "join_session",
  "session_id": "f39be8d5-416c-442a-ba14-a492b1d249da",
  "name": "library-b"
}
```

Response:

```json
{
  "session_id": "f39be8d5-416c-442a-ba14-a492b1d249da",
  "topic": "Add streaming support to library-b",
  "status": "active",
  "participant": "library-b",
  "participants": ["project-a", "library-b"],
  "message_count": 1,
  "messages": [
    {
      "id": 1,
      "from": "project-a",
      "type": "feature_request",
      "content": "We need a streaming API for the data pipeline...",
      "created_at": "2026-02-21T10:00:00Z"
    }
  ],
  "hints": "Use collab post to send messages..."
}
```

### `post`

Send a message to a session. The sender must have joined the session first.

| Parameter    | Required | Description                        |
| ------------ | -------- | ---------------------------------- |
| `session_id` | Yes      | Session UUID                       |
| `content`    | Yes      | Message content (max 50,000 chars) |
| `type`       | No       | Message type (default: `general`)  |
| `name`       | No       | Sender name                        |

Message types: `feature_request`, `implementation_summary`, `question`, `feedback`, `bug_report`, `api_change`, `general`

```json
{
  "action": "post",
  "session_id": "f39be8d5-416c-442a-ba14-a492b1d249da",
  "content": "Here's the proposed streaming API:\n\n```go\nfunc (c *Client) Stream(ctx context.Context) (<-chan Event, error)\n```\n\nIt returns a channel of events that closes when the context is cancelled.",
  "type": "implementation_summary",
  "name": "library-b"
}
```

Response:

```json
{
  "message_id": 2,
  "session_id": "f39be8d5-416c-442a-ba14-a492b1d249da"
}
```

### `check`

Check for new messages since the participant's last read position. Automatically advances the read marker.

| Parameter    | Required | Description      |
| ------------ | -------- | ---------------- |
| `session_id` | Yes      | Session UUID     |
| `name`       | No       | Participant name |

```json
{
  "action": "check",
  "session_id": "f39be8d5-416c-442a-ba14-a492b1d249da",
  "name": "project-a"
}
```

Response:

```json
{
  "session_id": "f39be8d5-416c-442a-ba14-a492b1d249da",
  "new_messages": [
    {
      "id": 2,
      "from": "library-b",
      "type": "implementation_summary",
      "content": "Here's the proposed streaming API...",
      "created_at": "2026-02-21T10:05:00Z"
    }
  ],
  "has_new": true
}
```

### `read`

Read all messages in a session. Does not update read markers -- useful for reviewing full history.

| Parameter    | Required | Description  |
| ------------ | -------- | ------------ |
| `session_id` | Yes      | Session UUID |

### `list_sessions`

List all sessions on this machine.

| Parameter | Required | Description                    |
| --------- | -------- | ------------------------------ |
| `status`  | No       | Filter by `active` or `closed` |

### `close`

Mark a session as resolved.

| Parameter    | Required | Description                                    |
| ------------ | -------- | ---------------------------------------------- |
| `session_id` | Yes      | Session UUID                                   |
| `summary`    | No       | Summary of what was resolved (max 2,000 chars) |

```json
{
  "action": "close",
  "session_id": "f39be8d5-416c-442a-ba14-a492b1d249da",
  "summary": "Agreed on streaming API design using channel-based event model"
}
```

## Waiting for Replies (`collab_wait`)

`collab_wait` is a separate tool that blocks until new messages arrive in a session. It polls the filesystem at a configurable interval and returns when new messages are detected or the timeout is reached.

| Parameter               | Required | Description                                        |
| ----------------------- | -------- | -------------------------------------------------- |
| `session_id`            | Yes      | Session UUID to watch                              |
| `timeout_seconds`       | No       | Max wait time (default: 600, max: 3600)            |
| `poll_interval_seconds` | No       | How often to check (default: 60, min: 5, max: 300) |

```json
{
  "session_id": "f39be8d5-416c-442a-ba14-a492b1d249da",
  "timeout_seconds": 300,
  "poll_interval_seconds": 10
}
```

Response when messages arrive:

```json
{
  "session_id": "f39be8d5-416c-442a-ba14-a492b1d249da",
  "status": "new_messages",
  "new_count": 1,
  "message": "1 new message(s) detected. Use collab check to read them."
}
```

Response on timeout:

```json
{
  "session_id": "f39be8d5-416c-442a-ba14-a492b1d249da",
  "status": "timeout",
  "new_count": 0,
  "message": "No new messages after 300 seconds."
}
```

After `collab_wait` returns `"new_messages"`, call `collab` with `action: "check"` to read the actual content.

Poll interval priority: tool parameter > `COLLAB_POLL_INTERVAL` env var > 60s default.

## Configuration

| Variable               | Default                   | Description                                                           |
| ---------------------- | ------------------------- | --------------------------------------------------------------------- |
| `COLLAB_DIR`           | `~/.mcp-devtools/collab/` | Storage directory for sessions                                        |
| `COLLAB_POLL_INTERVAL` | `60`                      | Default poll interval in seconds for `collab_wait` (min: 5, max: 300) |

## Example: API Design Coordination

Two agents working on related projects -- `project-a` consumes an API from `library-b`. Project A needs streaming support added.

**Agent A** (working in project-a):
```
collab create_session topic="Add streaming support to library-b"
  -> session_id: "f39be8d5-..."

collab post session_id="f39be8d5-..." type="feature_request"
  content="We need a streaming API for the data pipeline.
  Requirements: channel-based, context-cancellable, backpressure support."
```

*Human copies the UUID to Agent B's session.*

**Agent B** (working in library-b):
```
collab join_session session_id="f39be8d5-..."
  -> sees topic and the feature request

collab post session_id="f39be8d5-..." type="implementation_summary"
  content="Proposed API: func (c *Client) Stream(ctx context.Context) (<-chan Event, error)
  Backpressure handled via channel buffer size parameter."

collab_wait session_id="f39be8d5-..." poll_interval_seconds=10
  -> blocks until Agent A responds
```

**Agent A**:
```
collab check session_id="f39be8d5-..."
  -> reads the implementation summary

collab post session_id="f39be8d5-..." type="feedback"
  content="Looks good. Can you add a WithBufferSize option?"
```

**Agent B** (collab_wait returns):
```
collab check session_id="f39be8d5-..."
  -> reads the feedback, implements the change

collab post session_id="f39be8d5-..." type="api_change"
  content="Added StreamOption type with WithBufferSize(n int).
  Default buffer: 100. Updated in v0.5.0."
```

**Agent A**:
```
collab close session_id="f39be8d5-..."
  summary="Agreed on streaming API with configurable buffer size"
```

## Example: Bug Report Across Projects

**Agent A** discovers a bug in a dependency:
```
collab create_session topic="Bug: nil pointer in library-b's Parse function"
  -> session_id: "a1b2c3d4-..."

collab post session_id="a1b2c3d4-..." type="bug_report"
  content="Parse() panics when input is empty string.
  Stack trace: parse.go:42 -> validate.go:18 -> nil deref on config.Rules"
```

**Agent B** (after human relays UUID):
```
collab join_session session_id="a1b2c3d4-..."
  -> reads the bug report, investigates, fixes

collab post session_id="a1b2c3d4-..." type="implementation_summary"
  content="Fixed: added nil check in validate.go:18.
  Also added test case for empty input. Committed to main."
```

## Participant Name Resolution

If `name` is omitted, the tool auto-detects from MCP Roots (the project directory name). If detection fails, it falls back to `"agent"`.

Names must be lowercase alphanumeric with hyphens, underscores, and dots (max 128 chars).

## Filesystem Layout

```
~/.mcp-devtools/collab/
  sessions/
    <uuid>/
      session.json        # Session metadata and participants
      session.json.lock   # Advisory file lock (flock)
      msg-001.json        # Individual messages (append-only)
      msg-002.json
      ...
```

## Security

- Directories: 0700, files: 0600
- Session IDs validated as UUID format (prevents path traversal)
- All string inputs validated and length-limited
- Local filesystem only -- no network access
- Advisory file locking prevents concurrent write corruption

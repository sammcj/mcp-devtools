# Changelog

## Unreleased

Migration to the official [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) and MCP spec revision **2026-07-28** (the stateless release), replacing `mark3labs/mcp-go` and spec revision 2025-06-18.

### Breaking

- **SSE transport removed.** `--transport sse` now exits with an error pointing at `--transport http`. SSE was removed from the MCP spec in 2026-07-28.
- **HTTP transport is stateless.** There is no `initialize` handshake and no `Mcp-Session-Id`. Every request carries its own protocol version, client capabilities and client info in `_meta`. `GET` and `DELETE` on the endpoint return `405`. Any replica can serve any request, so round-robin load balancing, scale-to-zero and rolling restarts work without sticky sessions.
- **`--session-timeout` removed.** There are no sessions to time out.
- **`sequential_thinking` is stdio-only.** It keeps thought history in the server process, so it is no longer registered when running with `--transport http`. Tools can opt into this with the new `tools.StdioOnly` marker interface.
- **Proxy speaks streamable HTTP only.** `"transport": "sse"` and `"http-first"` in `PROXY_UPSTREAMS` still parse, but the value is ignored; an `sse` value logs a warning. The proxy sends the 2026-07-28 framing and falls back to the older form for the rest of the connection if an upstream rejects it.
- **Roots, Sampling and Logging are no longer used.** The `codesearch` indexing progress notification now goes to the log and an OTEL span event instead of `notifications/message`.

### Security

- HTTP bearer token authentication now rejects unauthenticated requests. The previous middleware read the token but never enforced it. Comparison is constant-time.
- OAuth `state` is now generated, sent and validated on both the server-side browser flow and the proxy's upstream flow. Neither previously checked it, so anything able to reach the loopback callback could deliver an authorisation code of its choosing.
- RFC 9207 issuer identification on the OAuth client side: the issuer is captured from discovery and bound to stored credentials, `iss` is validated when present, and both a promised-but-missing `iss` and an `iss` with no known issuer to compare against are rejected. The binding covers cached access tokens as well as refresh tokens and client registrations; a token whose issuer no longer matches is dropped from memory and disk rather than reused.
- OAuth callbacks validate `state` before anything else and drop a failed request without notifying the waiter, so anything able to reach the loopback callback port can no longer abort an in-flight flow. A `state` can only be redeemed once, and validation and redemption happen under one lock.
- A failed metadata discovery no longer leaves the rejected authorisation and token endpoints in live config.
- PKCE `plain` is no longer advertised or accepted; the accepting branch was removed and `S256` is the only method.
- Cross-origin protection now uses `http.NewCrossOriginProtection` rather than a hand-rolled origin check.
- Request bodies on the HTTP transport are capped at 8MB. There is no server-wide `WriteTimeout` (it would have capped tool runtime); a per-request write deadline bounds a client that stops reading.
- The OAuth authorisation server metadata no longer advertises `/oauth/authorize`, `/oauth/token` or a local `jwks_uri`. mcp-devtools is a resource server and serves none of them; `jwks_uri` now reports the configured JWKS URL, and clients find the real authorisation server through the protected resource metadata.
- A JWT whose header carries no `kid`, or a non-string one, is rejected rather than panicking.
- Proxy metadata discovery validates the issuer against the URL the document was actually served from, so an off-host redirect cannot deliver an accepted document.

### Added

- `application_type` on dynamic client registration, defaulting to `web`.
- `internal/mcpapi`, a thin wrapper over the SDK that keeps the existing tool builder API (`mcpapi.NewTool`, `mcpapi.WithString`, and so on). Tool code did not need to change.

### Dependencies

All direct dependencies updated to their latest stable releases. Three needed code changes:

- `go.lsp.dev/protocol`, `go.lsp.dev/jsonrpc2` and `go.lsp.dev/uri` v0.x to v1.0.1. The v1 API is a full regeneration, so `code_rename`'s LSP client was ported to it.
- `github.com/xuri/excelize/v2` 2.10.1 to 2.11.0. Chart and axis titles moved from `[]RichTextRun` to a `ChartTitle` struct, and `ChartLine` became `LineOptions`.
- `github.com/openai/openai-go/v3` 3.39 to 3.50, `github.com/pdfcpu/pdfcpu` 0.12.1 to 0.14.0, `github.com/urfave/cli/v3` 3.9.1 to 3.10.1, all OpenTelemetry packages 1.44 to 1.45, and the AWS SDK.

### Deprecated

- Dynamic client registration (RFC 7591). It still works and logs a deprecation warning on use. Client ID Metadata Documents supersede it: mcp-devtools is a resource server, not an authorisation server, so there is nothing for it to implement server-side. As a client, pass the https URL of your metadata document as the configured client ID (`PROXY_<UPSTREAM>_CLIENT_ID`) and no registration call is made.

### Known limitations

- `tools/list` returns `ttlMs: 0`, so clients will not cache it. The SDK has no option to set a TTL; doing so needs receiving middleware that rewrites `ListToolsResult`.
- Tools that persist state under `~/.mcp-devtools` (`code_search`, `memory`, `security_override`) need a single replica or a shared volume when served over HTTP. They keep no state in the process, but two replicas with separate disks will disagree.

# Migration to MCP spec 2026-07-28

Plan for moving mcp-devtools from `mark3labs/mcp-go` to the official `modelcontextprotocol/go-sdk` and onto the stateless 2026-07-28 spec.

Status: all phases done, including the phase 6 review and its fixes. Branch: `mcp_2026-07-28`.

Verified working on the branch: `make lint && make test && make build` all pass; stdio serves a
stateless `tools/call` with no `initialize`; a legacy `initialize` client on `2025-06-18` still works;
HTTP answers a single self-contained POST with no session and returns 405 for GET; `server/discover`
advertises all five supported versions and only the `tools` capability.

## Context

MCP 2026-07-28 makes the protocol stateless. What matters for this project:

- The `initialize` handshake is gone (SEP-2575). Protocol version, capabilities and `clientInfo` ride in `_meta` on every request, and a mandatory `server/discover` RPC advertises identity and supported versions.
- `Mcp-Session-Id` and protocol-level sessions are gone (SEP-2567). Cross-call state becomes explicit handles a tool mints and accepts back as ordinary arguments.
- `Mcp-Method` and `Mcp-Name` request headers are now required, so gateways can route without parsing bodies.
- `ttlMs` and `cacheScope` are required on `tools/list` and the other list results (SEP-2549). Tools should be returned in deterministic order for prompt-cache hits.
- Roots, Sampling and Logging are deprecated (SEP-2577); `ping`, `logging/setLevel` and `notifications/roots/list_changed` are removed. Push notifications collapse into one `subscriptions/listen` stream. The HTTP+SSE transport is deprecated and SSE resumability is gone.
- Server-initiated requests become Multi Round-Trip Requests (SEP-2322). Not used by any tool here.
- OAuth hardens: validate `iss` (RFC 9207), bind credentials to the issuing server, send `application_type` at registration, and prefer Client ID Metadata Documents over Dynamic Client Registration.

Deprecations get a minimum twelve months, so nothing breaks immediately, but new clients will expect the new behaviour.

Spec: https://modelcontextprotocol.io/specification/2026-07-28

## Decision

Migrate to `github.com/modelcontextprotocol/go-sdk` v1.7.0 or newer. Do not rewrite the project.

Rationale:

- `mark3labs/mcp-go` v0.57.0 (latest, 23 Jul 2026) declares `LATEST_PROTOCOL_VERSION = "2025-11-25"` and has no 2026-07-28 support. Two open tracking issues, no PR: [#948](https://github.com/mark3labs/mcp-go/issues/948), [#928](https://github.com/mark3labs/mcp-go/issues/928). Still maintained, so support may land, on an unknown timeline.
- The official SDK shipped 2026-07-28 support the day the spec published (v1.7.0, 28 Jul 2026), covering SEP-2575, SEP-2322, SEP-2243 and SEP-2549, and stayed on the v1 module path.
- SDK coupling here is thin: 97 files import `mcp-go`, but the surface is about ten builder functions plus `mcp.Tool` and `mcp.CallToolResult`. The 84k lines of tool logic are SDK-agnostic, and `internal/security`, `internal/registry` and `internal/handlers` carry no SDK types beyond the result.

Release strategy:

- Cut this as **mcp-devtools v2**, leaving v1 tagged for anyone needing the old transports.
- **Drop SSE entirely** rather than carry a deprecated code path.
- **Streamable HTTP becomes stateless-only.** The SDK only accepts 2026-07-28 when `StreamableHTTPOptions.Stateless` is true; supporting both modes means two configurations of the same server for no benefit.
- **stdio keeps backwards compatibility for free** through SDK version negotiation.

## Current state

Registered tools: 31, across roughly 50 packages under `internal/tools/`.

SDK choke points, all of which need touching:

- `internal/tools/tools.go` - the `Tool` interface: `Definition() mcp.Tool`, `Execute(...) (*mcp.CallToolResult, error)`.
- `main.go` `newToolHandler` - the single wrapper adapting MCP calls to `Tool.Execute`, including argument extraction, metrics and telemetry.
- `main.go` server construction, tool registration loop, transport switch, and `startStreamableHTTPServer`.
- `tests/testutils/mocks.go` and `helpers.go` - use the same builder API, so they follow the shim.

Deprecated or removed features currently in use:

- SSE transport: `mcpserver.NewSSEServer`, plus the `--transport sse` flag.
- Sessions: `TimeoutSessionManager` and `WithSessionIdManager`, driven by `--session-timeout`; `WithHeartbeatInterval` derived from it.
- Protocol version allowlist in `main.go` lists only `2025-06-18` and `2024-11-05`, defaulting to `2025-06-18` when the header is absent.
- Logging: `internal/tools/codesearch/codesearch.go` sends `notifications/message`.
- `tools/list_changed`: `internal/tools/proxy/register.go` relies on mcp-go emitting it after async upstream registration.
- Dynamic Client Registration: `internal/oauth/registration/registrar.go` and `--oauth-dynamic-registration`.

`internal/tools/proxy/upstream/` is a hand-rolled JSON-RPC client that never performs `initialize` and holds no session ID, so it is already close to stateless. It needs the new headers and `_meta`, and `sse.go` should go.

Process-local state:

- `internal/tools/sequentialthinking` holds thought history in a mutex-guarded in-process map, which breaks behind a load balancer. **Decision: stdio-only.** Do not attempt a handle-based redesign.
- Every other `sync.Map` is cache-only. Caches are an optimisation, not affinity, so they are safe as-is.

## Considerations

- **Read Appendix A before writing code.** It records the verified v1.7.0 API. The SDK's own `docs/protocol.md` has at least one example that does not compile.
- **Error semantics are inverted from mcp-go.** A non-nil `error` returned from an SDK `ToolHandler` is a JSON-RPC protocol error, not an `isError: true` result. Any tool doing `return nil, err` to signal a tool-level failure changes meaning. Highest-risk part of the migration.
- **Default capabilities are not empty.** A nil `ServerCapabilities` makes the SDK advertise the deprecated `logging` capability. Set it explicitly.
- **`MCPGODEBUG` escape hatches are temporary** and removed in v1.9.0. `allowsessionsinstateless=1` in particular restores session handling. Do not build on any of them.
- **Stdio must never write to stdout or stderr.** The existing constraint holds through the Logging removal.
- **Keep the `Tool` interface shape.** `Execute(ctx, logger, cache, args)` maps closely onto the SDK's untyped handler. Changing it would touch all 31 tools for no gain.
- **Default enablement is unchanged.** No tool gains or loses default-enabled status here.
- **`requestState` is untrusted input** if MRTR is ever adopted. It round-trips through the client, so sign or validate it.

## Out of scope

- Adopting the Tasks extension or MCP Apps. Note candidates for later; do not implement.
- Implementing MRTR / `input_required` in any tool. No current tool needs server-initiated input.
- Redesigning `sequentialthinking` for statelessness.
- Adding new tools, or changing tool behaviour or output formats.
- Rewriting `internal/security`, `internal/registry` or `internal/telemetry` beyond what compilation requires.

## Tasks

### Phase 1: shim and mechanical swap

- [x] Add `github.com/modelcontextprotocol/go-sdk` v1.7.0 and `github.com/google/jsonschema-go` to `go.mod`.
- [x] Create `internal/mcpapi/` reproducing the builder API currently in use, over go-sdk types (Appendix A has the targets):

```
builders:    NewTool, WithDescription, WithString, WithNumber, WithBoolean,
             WithArray, WithObject, WithStringItems
properties:  Required, Description, Enum, Items, Properties,
             DefaultString, DefaultNumber, DefaultBool
annotations: WithReadOnlyHintAnnotation, WithDestructiveHintAnnotation,
             WithIdempotentHintAnnotation, WithOpenWorldHintAnnotation
results:     NewToolResultText, NewToolResultJSON, NewToolResultError, AsTextContent
aliases:     Tool, CallToolResult, TextContent, ToolOption, PropertyOption
```

- [x] Reconcile `Definition()` returning `mcp.Tool` by value against the SDK wanting `*mcp.Tool`. Resolved by taking the address in the registration loop; `mcpapi.Tool` stays an alias of `mcp.Tool`.
- [x] Write unit tests for the shim (`tests/unit/mcpapi_test.go`), including a cross-check against mcp-go output for a tool exercising every builder. The two schemas differed only in key order.
- [x] Audit every tool for `return nil, err` in `Execute`. **Resolved centrally:** `newToolHandler` in `main.go` already converts any error returned by `Execute` into an `IsError: true` result and never propagates it, so no tool needs changing. The one gap is the inline handler in `internal/tools/proxy/register.go`, which returns errors directly; it must reuse the shared handler (Phase 2).
- [x] Change `internal/tools/tools.go` to import the shim instead of `mark3labs/mcp-go/mcp`.
- [x] Swap the import in the remaining files (96 in total, scripted). Only five call sites needed hand edits, all of them consequences of the SDK modelling content as pointers or `InputSchema` as `any`:
  - `Content` implementations are pointers, so `x.(mcp.TextContent)` became `x.(*mcpapi.TextContent)` (three sites).
  - `TextContent` has no `Type` field; two test assertions on it were redundant after `AsTextContent` and were dropped.
  - `Tool.InputSchema` is `any`, so `def.InputSchema.Properties` became `mcpapi.InputSchemaOf(def).Properties` (`toolhelp.go` plus several tests).
  - `Tool.GetName()` does not exist; use `def.Name`.
- [x] Update `tests/testutils/mocks.go` and `tests/testutils/helpers.go`.
- [x] Confirm `go build ./...` and the test suite pass with the shim in place.

### Phase 2: server and transports

- [x] Replace `mcpserver.NewMCPServer` with `mcp.NewServer(&mcp.Implementation{...}, opts)` in `main.go`.
- [x] Set `ServerOptions.Capabilities` explicitly to avoid advertising the deprecated `logging` capability that the SDK enables by default. Confirmed: `server/discover` now reports only `tools`.
- [x] Rewrite `newToolHandler` against `mcp.ToolHandler`, unmarshalling `req.Params.Arguments` from `json.RawMessage`. Metrics, telemetry spans and error logging preserved.
- [x] Sort tools by name in the registration loop so `tools/list` output is deterministic.
- [x] Confirm every tool's `InputSchema` is non-nil and of type `object`. `mcpapi.NewTool` always populates one, so a no-parameter tool is covered; the SDK panics at `AddTool` otherwise.
- [x] Replace `ServeStdio` with the SDK stdio transport (`srv.Run(ctx, &mcp.StdioTransport{})`).
- [x] Delete the `sse` transport case and the `sse` option from the `--transport` flag. Passing `--transport sse` now fails with a message pointing at `http`.
- [x] Rewrite `startStreamableHTTPServer` on `NewStreamableHTTPHandler` with `Stateless: true`. Verified GET returns 405, so a health check cannot use the MCP endpoint.
- [x] Set `PropagateRequestCancellation: true`.
- [x] Set `MaxRequestBodyBytes` to 8MB and wrap the mux in `http.NewCrossOriginProtection().Handler`. The SDK's localhost/DNS-rebinding protection is left on.
- [x] Delete `TimeoutSessionManager` and the `--session-timeout` flag.
- [x] Delete the heartbeat interval configuration.
- [x] Delete the local protocol version allowlist and the `2025-06-18` default, along with `isValidOrigin`, which duplicated protection the SDK and `CrossOriginProtection` now provide.
- [x] Port the OAuth and legacy token middleware off `WithHTTPContextFunc` to `http.Handler` wrapping. **Security fix along the way:** both old middlewares only annotated the context and never rejected anything, and nothing downstream read `OAuthAuthFailedKey`, so unauthenticated requests were served. OAuth now uses the existing `OAuth2Server.CreateMiddleware()`, which returns 401 with `WWW-Authenticate`; the legacy token path uses a new `requireBearerToken` with a constant-time comparison.
- [x] Rework anything reading request state out of the `WithHTTPContextFunc` context. Only trace context did; it moved to a `withTraceContext` handler wrapper.
- [x] Keep the custom mux, OAuth well-known registration, HTTP timeouts and graceful shutdown. The OAuth and non-OAuth branches were duplicated; they are now one path, and graceful shutdown applies to both (previously only OAuth mode had it).
- [x] Verify `server/discover` responds correctly. The SDK answers it; no work needed.
- [x] `cacheScope` comes back as `public`, but `ttlMs` is always 0, meaning "immediately stale". The SDK has no option for it and the raw `AddTool` path does not touch list results, so setting a real TTL needs `Server.AddReceivingMiddleware` to rewrite `ListToolsResult`. **Decided against for now**: the proxy registers upstream tools asynchronously, so a cached list could be served before those tools exist. Recorded under Known limitations in the CHANGELOG.
- [x] Test stdio end to end from the command line with a `tools/call` JSON-RPC request. Note for whoever repeats this: the shell must hold stdin open (`{ echo ...; sleep 2; }`) or the server exits before flushing, and a stateless request needs `_meta` carrying `io.modelcontextprotocol/protocolVersion`, `clientCapabilities` and `clientInfo`.
- [x] Test HTTP end to end with a single self-contained POST carrying `MCP-Protocol-Version`, `Mcp-Method` and `Mcp-Name` headers and no session.
- [x] Confirm an old client still works: `initialize` on `2025-06-18` followed by `tools/call` succeeds and the response carries no 2026-07-28 fields.

### Phase 3: deprecated features

- [x] Mark `sequentialthinking` stdio-only. Done with a `tools.StdioOnly` marker interface rather than a special case in `main.go`, so the registration loop skips any tool that declares it. Verified: `tools/list` over HTTP omits it. Tool docs still to update in Phase 5.
- [x] Replace the `notifications/message` call in `internal/tools/codesearch/codesearch.go`. It now logs through logrus (file-backed, so stdio stays clean) and adds an `indexing.started` span event.
- [x] Move `tools/list_changed` in `internal/tools/proxy/register.go`. The inline handler there duplicated `newToolHandler` and returned Go errors instead of `isError` results; `RegisterUpstreamToolsAsync` now takes a handler factory and reuses the shared one, so proxied tools get the same metrics, telemetry and error semantics. The stale mcp-go comment is replaced.
- [x] Decide whether to keep the hand-rolled JSON-RPC client in `internal/tools/proxy/upstream/`. **Kept.** It is only ~900 lines after the SSE removal, already stateless, and carries OAuth device-flow handling that would have to be rebuilt against the SDK client. Swapping it is a candidate for later, not part of this migration.
- [x] Add `Mcp-Method` and `Mcp-Name` headers and the `_meta` protocol version to outbound requests. `newRequest` in `jsonrpc.go` stamps `_meta` (version, capabilities, client info) on every request; the HTTP transport sets the headers from `Message.Method` and a new non-serialised `Message.Name`.
- [x] Do not forward client routing headers verbatim to an upstream. Configured custom headers are applied first, then the routing headers overwrite them, so a stale `Mcp-Method` in config cannot cause a `-32020 CodeHeaderMismatch`.
- [x] Delete `internal/tools/proxy/upstream/sse.go` and the config that selects it. `Strategy` collapses to `http`/`sse`; an `sse*` value still parses, logs a warning, and falls back to HTTP rather than failing a previously working config.
- [x] Audit the rest of the codebase for Roots, Sampling, Logging, `ping` and `logging/setLevel`. Nothing left after the codesearch change.

### Phase 4: OAuth hardening

- [x] Validate the `iss` parameter per RFC 9207 before redeeming an authorisation code. **Both OAuth clients also had no `state` validation at all**, and the proxy's client never sent a `state` parameter, so any page able to reach the loopback callback while a browser flow was open could inject its own authorisation code. Both callback servers now require the state they issued and validate `iss`, and refuse any callback that arrives before the caller has declared its expectations. `internal/oauth/client` additionally rejects an authorisation server whose metadata declares an issuer other than the configured one.
- [x] Bind stored credentials to the issuing authorisation server. `Tokens` and `ClientInfo` record the issuer; a cached client registration or refresh token is refused if the server later reports a different one, so a client secret or refresh token cannot be posted to a new party's token endpoint. Credentials cached before this change have no issuer recorded and are still accepted, so nobody is forced to reauthenticate on upgrade.
- [x] Set `application_type` during client registration. The proxy registers as `native`; the server-side registrar validates the value, echoes it, and defaults an absent one to `web` per OpenID Connect.
- [x] Add Client ID Metadata Document support and make it the default. New `internal/oauth/cimd` resolves an https client ID by fetching the document, and `OAuth2Server.ResolveClient` prefers it, falling back to the DCR registry only when one is configured. The fetch is the one place an unauthenticated caller can make this server issue a request, so it is https-only, refuses non-public addresses, follows no redirects, caps the body at 64KB, requires the document to declare the client ID it was fetched from, and discards any client secret the document tries to confer.
- [x] Deprecate the Dynamic Client Registration path. It stays behind `--oauth-dynamic-registration`, now logs a deprecation warning at startup, and the `registration_endpoint` is only advertised when the flag is set.
- [x] Also dropped `plain` from the advertised `code_challenge_methods_supported`. OAuth 2.1 forbids it; the validator still accepts it for existing clients mid-flight.
- [x] Update `docs/oauth/` for the new flow, including `docs/oauth/authentik-setup.md`. Done in phase 5.

### Phase 5: docs, tests, release

- [x] Rewrite the `## mcp-go` section of `docs/creating-new-tools.md` for the new SDK and shim, and check the rest of that document for stale API examples. Renamed to `## MCP SDK and the mcpapi package`. Also fixed pre-existing wrong examples: the doc called `mcp.NewCallToolResult` and `mcp.NewCallToolResultJSON`, neither of which ever existed in mcp-go. Documented the result constructors that do exist and the error semantics (a returned Go error is converted to `isError: true` by `newToolHandler`, so it never reaches the client as a protocol error).
- [x] Remove SSE from `README.md`, `docs/security.md`, `docs/oauth/`, and any tool docs that mention it. `docs/security.md` had no SSE references. `docs/tools/proxy.md` needed the most work: transport section rewritten, SSE lifecycle implementation note deleted, example URLs moved off `/v1/sse`.
- [x] Document the stateless HTTP deployment model in `README.md`, including that scale-to-zero and round-robin load balancing now work.
- [x] Note the `sequentialthinking` stdio-only restriction in `docs/tools/`, the README tool table, and the `Additional Considerations` list in `docs/creating-new-tools.md` (with the `tools.StdioOnly` escape hatch).
- [x] Update the Makefile `run-http` target if its flags changed. No change needed; `--transport http --port --base-url` all still exist.
- [x] Update `docs/observability.md` if session span handling changed with the loss of protocol sessions. Session spans and `mcp.session.*` metrics are now stdio-only; HTTP relies on inbound `traceparent` and each call is otherwise its own root trace.
- [x] Add a CHANGELOG entry describing the v2 breaking changes: SSE removed, HTTP is stateless-only, `--session-timeout` removed, DCR deprecated. Created `CHANGELOG.md` (none existed) with Breaking / Security / Added / Deprecated sections.
- [x] Also updated spec version references from 2025-06-18 to 2026-07-28 across `docs/oauth/`, and replaced the mcp-go pointer in `CLAUDE.md` and `.github/copilot-instructions.md`.
- [x] Run `make lint && make test && make build` and fix everything. All pass: lint 0 issues, all test packages ok, build clean.

### Phase 6: review

- [x] Have a read-only sub-agent on the fable model review the whole migration diff, then action the findings that hold up and re-run `make lint && make test && make build`.

Findings actioned:

- **Proxy sent `MCP-Protocol-Version: 2026-07-28` unconditionally.** The old client sent no version header at all, and servers on older revisions validate it against an allowlist and answer 400. The `_meta` block moved out of `newRequest` into `Message.encode(stateless bool)`, and `HTTPTransport` now retries once without the version header, routing headers and `_meta` when a 400 mentions any of them, then stays in that mode for the connection's lifetime.
- **`WriteTimeout: 30s` capped every tool call over HTTP.** The old non-OAuth path had no timeouts; unifying the two paths silently applied the OAuth path's write timeout to everything, killing any tool that runs longer than 30 seconds. Removed; `ReadHeaderTimeout` and `IdleTimeout` still bound slow clients.
- **Client ID Metadata Documents were unreachable.** mcp-devtools is a resource server: it never serves `/authorize` or `/token`, so it never resolves a client ID. `OAuth2Server.ResolveClient` had no caller and `client_id_metadata_document_supported` was an unbacked claim. `internal/oauth/cimd` deleted along with the advertisement (which also removes the resolver's DNS-rebinding TOCTOU and unbounded cache). As a client, a CIMD URL already works through the existing static client ID config; documented in `docs/oauth/README.md`.
- **`authorization_response_iss_parameter_supported` was advertised on the same false premise** and was removed with it. The client-side `iss` validation, which is real, stays.
- **`iss` validation was a no-op when no issuer was known.** Both callback servers now reject an `iss` they have nothing to compare against, rather than waving it through.
- **Callbacks handled `error=` before validating `state`,** so any local page could abort an in-flight flow. Validation now runs first in both servers, the proxy's error send no longer blocks on a full channel, and a `state` is single-use.
- **Failed discovery left attacker-supplied endpoints in live config.** `DiscoverEndpoints` now decodes into locals and assigns only after the issuer check passes.
- **Proxy metadata discovery** used `http.DefaultClient` with no size cap and never checked the issuer against the URL it was fetched from. Now a bounded client, a 256KB `io.LimitReader`, and an RFC 8414 scheme/host check.
- **PKCE `plain` was still accepted** by `ValidateChallenge` and `GenerateChallenge` even though it was no longer advertised. Both branches removed.
- **Docs**: `CLAUDE.md` still described SSE as a live transport and pointed new tools at `main.go` instead of `internal/imports/tools.go`; `docs/creating-new-tools.md` had a `SafeHTTPGet` call missing its `ctx`; the CHANGELOG overstated the `http-first` warning and the PKCE change.

A second review round (three reviewers, on the transport, OAuth and docs slices) found more, all actioned:

- **Rejected callbacks still aborted the flow.** Validation failure pushed an error onto the channel the caller waits on, so anything able to reach the loopback port could still cancel an authorisation. Both handlers now log, answer 400, and stay quiet.
- **`state` validation and redemption were not atomic.** Folded into one write-locked method in both callback servers.
- **The server-wide `WriteTimeout` removal left responses unbounded.** A per-request write deadline replaces it, bounding a client that stops reading without capping tool runtime.
- **The framing fallback missed empty 400 bodies** (gateways that strip them) and silently kept the new form on an unmatched 400. Empty bodies now trigger the fallback, and an unmatched 400 is logged.
- **Legacy mode deleted routing headers the user set in config,** which defeats the point of reproducing the old wire form.
- **Metadata discovery validated the issuer against the pre-redirect URL,** so an off-host redirect could deliver an accepted document.
- **The OAuth authorisation server metadata advertised `/oauth/authorize`, `/oauth/token` and a local `jwks_uri` that nothing serves** - the same false-claim class as the CIMD advertisement. Removed; `jwks_uri` now reports the configured URL.
- **A JWT header with no `kid`, or a non-string one, panicked** inside the JWKS keyfunc on untrusted input.
- **A client registration stored before issuer binding existed never got stamped,** so the upgrade grace applied forever rather than once.
- **`ValidatePKCEChallenge` in `internal/oauth/client/pkce.go` still accepted `plain`.** It had no callers; deleted.
- Smaller: `encode` panicked on JSON `null` params, `legacy` was read twice in `SendReceive`, `ErrClosed` was dead, two doc comments were wrong, and `sequential-thinking` was documented as needing `ENABLE_ADDITIONAL_TOOLS` despite being default-enabled.

Findings noted but deliberately not actioned:

- **`code_search`, `memory` and `security_override` keep state on disk under `~/.mcp-devtools`.** These are not in-process state, so they survive restarts and work fine on a single HTTP replica; `code_search` also indexes a local path, which only makes sense when the server shares the filesystem with the code. Marking them `StdioOnly` would break working single-replica HTTP deployments, so the requirement is documented in `README.md` and `docs/tools/code_search.md` instead.
- **Tool annotations always serialise `readOnlyHint`/`idempotentHint`** because the SDK makes those value fields rather than pointers. The emitted values match the spec defaults, so this is `tools/list` noise, not a behaviour change.
- **`ttlMs: 0`** remains, as recorded above.

## Done when

- `make lint && make test && make build` all pass.
- A stateless HTTP client can call any enabled tool in one self-contained POST, with no `initialize` and no session header.
- An existing stdio client on an older protocol version still works through SDK version negotiation.
- No reference to `mark3labs/mcp-go` remains in `go.mod` or any Go file.
- No SSE transport code or documentation remains.
- `tools/list` returns tools in a stable order across restarts.

## References

- Spec: https://modelcontextprotocol.io/specification/2026-07-28
- Changelog: https://modelcontextprotocol.io/specification/2026-07-28/changelog
- Release post: https://blog.modelcontextprotocol.io/posts/2026-07-28/
- Official Go SDK: https://github.com/modelcontextprotocol/go-sdk
- SDK protocol notes: https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/protocol.md
- SDK rough edges: https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/rough_edges.md
- `MCPGODEBUG` flags: https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/mcpgodebug.md
- Local analysis of the release: `~/Library/Mobile Documents/com~apple~CloudDocs/Documents/Wisdom/2026-08-10-Mcp-Stateless-Release/`
- `mcp-explorer` for probing a stateless server during development: https://github.com/simonw/mcp-explorer

## Appendix A: verified go-sdk v1.7.0 API

Read from source at tag v1.7.0 (commit `bc72835f62eb94d0fb484439f886b6885b075f36`). Trust this over `docs/protocol.md`, which shows an `mcp.Connect` helper that does not exist.

### Server and raw tool handler

```go
func NewServer(impl *Implementation, options *ServerOptions) *Server
func (s *Server) AddTool(t *Tool, h ToolHandler)   // raw / untyped
func AddTool[In, Out any](s *Server, t *Tool, h ToolHandlerFor[In, Out])  // generic, top-level

type ToolHandler func(context.Context, *CallToolRequest) (*CallToolResult, error)
```

`Implementation{Name, Title, Description, Version, WebsiteURL, Icons}`.

On the raw path the caller owns everything: unmarshalling arguments, schema validation, and setting `Content` / `StructuredContent` / `IsError`. A returned `error` is a protocol error.

### Tool

```go
type Tool struct {
    Meta         `json:"_meta,omitempty"`
    Annotations  *ToolAnnotations `json:"annotations,omitempty"`
    Description  string           `json:"description,omitempty"`
    InputSchema  any              `json:"inputSchema"`   // must be non-nil, type "object"
    Name         string           `json:"name"`
    OutputSchema any              `json:"outputSchema,omitempty"`
    Title        string           `json:"title,omitempty"`
    Icons        []Icon           `json:"icons,omitempty"`
}
```

`InputSchema` is `any`, so `*jsonschema.Schema`, `json.RawMessage` or `map[string]any` all work. For a no-parameter tool: `json.RawMessage("{\"type\":\"object\"}")`. Schemas use `github.com/google/jsonschema-go/jsonschema` v0.4.3, draft 2020-12 only.

```go
&jsonschema.Schema{
    Type: "object",
    Properties: map[string]*jsonschema.Schema{
        "name":    {Type: "string", Description: "...", MaxLength: jsonschema.Ptr(10)},
        "count":   {Type: "number", Minimum: jsonschema.Ptr(0.0)},
        "verbose": {Type: "boolean", Default: json.RawMessage(`false`)},
        "tags":    {Type: "array", Items: &jsonschema.Schema{Type: "string"}},
        "mode":    {Type: "string", Enum: []any{"fast", "slow"}, Default: json.RawMessage(`"fast"`)},
    },
    Required: []string{"name"},
}
```

Traps for the shim:

- `Default` is `json.RawMessage`, not `any`.
- `Minimum` / `Maximum` / `MultipleOf` / `ExclusiveMin` / `ExclusiveMax` are `*float64`; `MinLength` / `MaxLength` / `MinItems` / `MaxItems` are `*int`. Use `jsonschema.Ptr`.
- `Items` is singular `*Schema`; `ItemsArray []*Schema` is a different field.
- `Type string` and `Types []string` both exist. Use one, never both.

### Reading arguments

```go
type CallToolRequest = ServerRequest[*CallToolParamsRaw]

type CallToolParamsRaw struct {
    Meta           `json:"_meta,omitempty"`
    Name           string           `json:"name"`
    Arguments      json.RawMessage  `json:"arguments,omitempty"`
    InputResponses InputResponseMap `json:"inputResponses,omitempty"`
    RequestState   string           `json:"requestState,omitempty"`
}
```

`req.Extra` is `*RequestExtra{TokenInfo *auth.TokenInfo; Header http.Header; CloseSSEStream func(...)}`.

### Results

No `NewToolResult*` constructors exist. Build literals:

```go
// text
&mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}

// JSON
b, _ := json.Marshal(v)
&mcp.CallToolResult{
    Content:           []mcp.Content{&mcp.TextContent{Text: string(b)}},
    StructuredContent: v,   // only when Tool.OutputSchema is set
}

// error
res := &mcp.CallToolResult{}
res.SetError(err)   // sets IsError and fills Content if empty
```

`Content` implementations are pointers (`*TextContent`, `*ImageContent`, `*AudioContent`, `*ResourceLink`, `*EmbeddedResource`) with no JSON tags; they marshal through custom `MarshalJSON`.

### Annotations

```go
type ToolAnnotations struct {
    DestructiveHint *bool  `json:"destructiveHint,omitempty"`  // nil means true
    IdempotentHint  bool   `json:"idempotentHint"`             // always serialised
    OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`    // nil means true
    ReadOnlyHint    bool   `json:"readOnlyHint"`               // always serialised
    Title           string `json:"title,omitempty"`
}
```

The mixed pointer and value fields are deliberate, and listed in the SDK's `rough_edges.md` as a v2 fix. The shim must take an address for the two hints that default to true.

### Streamable HTTP

```go
func NewStreamableHTTPHandler(getServer func(*http.Request) *Server, opts *StreamableHTTPOptions) *StreamableHTTPHandler
```

`StreamableHTTPOptions` fields: `Stateless`, `JSONResponse`, `Logger *slog.Logger`, `EventStore`, `SessionTimeout`, `DisableLocalhostProtection`, `CrossOriginProtection` (deprecated), `MaxRequestBodyBytes`, `PropagateRequestCancellation`.

Under `Stateless: true` no session ID is read or set, a temporary session is created per request, server-to-client requests are rejected, and GET and DELETE return 405.

Auth is handler wrapping, not an option:

```go
h := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return srv },
    &mcp.StreamableHTTPOptions{Stateless: true, PropagateRequestCancellation: true})
h = auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{})(h)
```

`auth.TokenVerifier` is `func(ctx context.Context, token string, req *http.Request) (*TokenInfo, error)`.

### Stdio

```go
srv := mcp.NewServer(&mcp.Implementation{Name: "mcp-devtools", Version: Version}, opts)
err := srv.Run(ctx, &mcp.StdioTransport{})   // zero value is usable, no constructor
```

### Client, for the proxy

```go
transport := &mcp.StreamableClientTransport{
    Endpoint:             url,
    HTTPClient:           httpClient,
    MaxRetries:           5,
    DisableStandaloneSSE: true,
}
cli := mcp.NewClient(&mcp.Implementation{Name: "mcp-devtools-proxy", Version: Version}, nil)
cs, err := cli.Connect(ctx, transport, nil)
defer cs.Close()

for tool, err := range cs.Tools(ctx, nil) { ... }   // iter.Seq2, auto-paginates
res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: n, Arguments: v})
```

Note the asymmetry: `CallToolParams.Arguments` is `any` on the client side, `json.RawMessage` on the server side.

### Silent failures to watch for

- An invalid tool name is logged, not returned as an error. The tool just does not appear.
- A tool whose input schema carries an invalid `x-mcp-header` annotation is silently dropped from `tools/list`.
- `ClientCapabilities.Roots` is broken (SDK issue #607); `RootsV2` is the working field.

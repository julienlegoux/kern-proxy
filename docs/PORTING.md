# Porting map: @earendil-works/pi-ai → kern-proxy (Go)

kern-proxy is a full-parity Go rebuild of
[`@earendil-works/pi-ai`](https://github.com/earendil-works/pi/tree/main/packages/ai).
The pinned upstream revision lives in `upstream/UPSTREAM.lock`; use
`upstream/sync.sh` to diff upstream changes since the pin.

Every ported Go file carries a `// Ports: packages/ai/src/...` header comment
naming its upstream source.

## File mapping

| Upstream (packages/ai) | Go | Status |
|---|---|---|
| `src/types.ts` | `ai/types.go`, `ai/model.go`, `ai/options.go`, `ai/events.go` | ported |
| `src/utils/event-stream.ts` | `ai/stream.go` | ported |
| `src/models.ts` (calculateCost, thinking helpers) | `ai/cost.go`, `ai/model.go` | ported |
| `src/models.ts` (Provider/Models/createProvider) | `ai/provider.go`, `ai/lazy.go` | ported |
| `src/utils/retry.ts` | `ai/retry.go` | ported |
| `src/utils/overflow.ts` | `ai/overflow.go` | ported |
| `src/utils/estimate.ts` | `ai/estimate.go` | ported |
| `src/utils/hash.ts` | `ai/hash.go` | ported (hash-compatible) |
| `src/utils/sanitize-unicode.ts` | `ai/sanitize.go` | ported |
| `src/utils/headers.ts` | `ai/headers.go` | ported |
| `src/utils/diagnostics.ts` | `ai/diagnostics.go` | ported |
| `src/utils/json-parse.ts` + npm `partial-json` | `ai/internal/partialjson` | ported |
| SSE parsing (from `src/api/anthropic-messages.ts`) | `ai/internal/sse` | ported |
| `src/providers/faux.ts` | `ai/providers/faux` | ported |
| `src/api/lazy.ts` | `ai/lazy.go` | ported (module lazy-loading is a TS concern; only lazyStream semantics carry over) |
| `src/api/simple-options.ts` | `ai/apis/simpleopts.go` | ported |
| `src/auth/types.ts`, `src/auth/resolve.ts`, `src/auth/credential-store.ts`, `src/auth/context.ts` | `ai/auth.go`, `ai/resolve.go`, `ai/credentialstore.go`, `ai/authcontext.go` (in package `ai`: the Provider interface references ProviderAuth, so a separate package would cycle) | ported |
| persistent auth.json store (from `packages/coding-agent/src/core/auth-storage.ts`) | `ai/auth/filestore.go` | ported |
| `src/auth/helpers.ts` | `ai/auth` | phase 11 (envApiKeyAuth backs the provider bindings; lazyOAuth is a TS bundler concern) |
| `src/utils/oauth/*`, `src/oauth.ts` | `ai/auth/oauth` | phase 12 |
| `src/api/transform-messages.ts` | `ai/apis/transform.go` | ported |
| `src/utils/validation.ts`, `src/utils/typebox-helpers.ts` | `ai/internal/jsonschema` | ported (typebox-helpers' StringEnum is a TypeBox schema-authoring convenience; plain JSON Schema documents need no equivalent) |
| `src/api/anthropic-messages.ts` | `ai/apis/anthropic` | ported (phase 5, issues 01–04: core, adaptive thinking, cache_control, OAuth Claude Code impersonation, and retry/overflow classifier + HTTP error-body wiring) |
| `src/api/openai-completions.ts`, `openai-prompt-cache.ts` | `ai/apis/openaicompletions` | ported (phase 6, issues 01-04. Issue 01: core adapter — request/message building, dual-map tool-call correlation, partial-JSON re-parsing, prompt/cache usage math. Issue 02: `detectCompat`/`getCompat` vendor auto-detection matrix (~18 flags) and `max_tokens`/`max_completion_tokens` selection, wired into request/message building for the flags already read (store, developer role, strict mode, usage-in-streaming, tool-result name, synthetic post-tool-result assistant message). Issue 03: the 10-value `thinkingFormat` enum's per-format request encoding (`reasoning_effort`/`thinking`/`enable_thinking`/`chat_template_kwargs`/`reasoning`/`tool_stream`), same-model thinking-block replay (plain text under `requiresThinkingAsText`, else the signature-named field plus `requiresReasoningContentOnAssistantMessages`), and Google-style `reasoning_details` encode/decode on tool calls. Issue 04 (epic close-out): `prompt_cache_key`/`prompt_cache_retention` (`openai-prompt-cache.ts`'s clamp, ported to `promptcache.go`), Anthropic-style `cache_control` replication for `compat.cacheControlFormat === "anthropic"` vendors, session-affinity headers (`session_id`/`x-client-request-id`/`x-session-affinity`), and the env-gated `OPENAI_API_KEY` live smoke) |
| `src/api/openai-responses.ts`, `openai-responses-shared.ts` | `ai/apis/openairesponses` | ported (phase 7, issue 01: request building — input items, tools, reasoning config — and SSE stream decode into unified events; `ConvertMessages`/`ConvertTools`/`DecodeStream` exported for reuse by the Azure and Codex variants below) |
| `src/api/azure-openai-responses*.ts` | `ai/apis/azure` | ported (phase 7, issue 02: endpoint/deployment-name resolution — `AZURE_OPENAI_BASE_URL`/`AZURE_OPENAI_RESOURCE_NAME`/`AZURE_OPENAI_API_VERSION`/`AZURE_OPENAI_DEPLOYMENT_NAME_MAP` env fallbacks, Azure-host base-URL normalization to `/openai/v1` — and its own buildParams over the shared `ConvertMessages`/`ConvertTools`/`DecodeStream` from `ai/apis/openairesponses`; the provider binding/catalog entry is deferred to epic 11) |
| `src/api/openai-codex-responses*.ts` | `ai/apis/codex` | ported (phase 7, issue 03: request/auth over the shared `ai/apis/openairesponses` core on the plain HTTP/SSE path -- JWT `chatgpt_account_id` extraction, Codex's own request shaping (unconditional `store`/`stream`/`instructions`/`text.verbosity`/`include`/`tool_choice`/`parallel_tool_calls`, no `max_output_tokens`, no automatic default-off reasoning fallback), session-affinity headers, prompt-cache-key clamping, per-request HTTP retry with exponential backoff and `retry-after(-ms)` handling, and the Codex-specific service-tier resolution (`resolveCodexServiceTier`, now a `ResolveServiceTier` hook on the shared `openairesponses.ServiceTierOptions`). Issue 04: the WebSocket transport (`github.com/coder/websocket`) tried first for any `Transport` other than `"sse"`, with its connection-limit-reached retry-once behavior and per-session fallback memory (`ai/apis/codex/websocket.go`); the SSE path (both the direct `"sse"` transport and the WebSocket-failure fallback) now always zstd-compresses its request body (`github.com/klauspost/compress/zstd`, `ai/apis/codex/zstd.go`), matching the Codex backend's own compressed traffic. **Not ported**: upstream's WebSocket connection *cache* (reusing an open connection across separate requests in the same session, with its busy/idle/age-limit bookkeeping) and the resulting `"websocket-cached"` delta/`previous_response_id` continuation mode -- every request here opens and closes its own connection, so `"auto"`/`"websocket-cached"` always send the full request body, same as `"websocket"`; see `ai/apis/codex/websocket.go`'s package doc for the reasoning) |
| `src/api/google-generative-ai.ts`, `google-shared.ts` | `ai/apis/google` | ported (phase 8, issue 01: google-shared converters (contents, tools, generation config) and thinking-signature handling, exported as `ConvertMessages`/`ConvertTools`/`DecodeStream` for the Vertex variant (issue 02) to reuse; streaming decode into unified events and API-key auth, over raw `net/http` + `ai/internal/sse` against the Generative Language REST API directly (no Go equivalent of the `@google/genai` SDK -- see "Vendor SDKs" below)) |
| `src/api/google-vertex.ts` | `ai/apis/google/vertex` | ported (phase 8, issue 02: Vertex endpoint shaping (project/location-scoped REST path for ADC, a project-less publisher path for an explicit API key) over `ai/apis/google`'s shared `ConvertMessages`/`ConvertTools`/`DecodeStream`, plus the thinking-level/budget helpers and `wireRequest` shape duplicated from that package -- matching upstream's own duplication between `google-generative-ai.ts` and `google-vertex.ts`, since only `google-shared.ts` is actually shared upstream. Application Default Credentials via `golang.org/x/oauth2/google` (`google.DefaultTokenSource`), reached through an injectable package-level `adcTokenFunc` (mirroring `ai/resolve.go`'s `authClock` stubbing pattern) so tests cover ADC resolution with a fake token source, no live GCP credentials needed. The `gcp-vertex-credentials` sentinel and `<placeholder>`-shaped API keys both fall back to ADC, matching `resolveApiKey`/`isPlaceholderApiKey`. **Deviation**: the exact REST path the `@google/genai` SDK builds when a custom `httpOptions.baseUrl` is combined with project/location is an internal SDK detail with no vendored source to introspect; this port instead defines and tests its own contract -- a genuine `baseUrl` override replaces the default host outright, and a catalog `baseUrl` template still containing `{location}` is ignored in favor of the default host) |
| `src/api/mistral-conversations.ts` | `ai/apis/mistral` | ported (phase 9: request building — messages, tools, tool choice, prompt_mode/reasoning_effort reasoning controls, prompt_cache_key/x-affinity session caching — and SSE stream decode into unified events over raw `net/http` + `ai/internal/sse`, since there is no Go equivalent of the `@mistralai/mistralai` SDK (see "Vendor SDKs" below); the provider binding/catalog entry is deferred to epic 11) |
| `src/api/bedrock-converse-stream.ts`, `src/bedrock-provider.ts` | `ai/apis/bedrock` | phase 10 |
| `src/providers/*.ts` (bindings), `src/providers/all.ts` | `ai/providers` | phase 11 |
| `src/providers/*.models.ts`, `src/models.generated.ts` | `ai/catalog/data/*.json` (via `tools/export-catalog`) | phase 11 |
| `src/env-api-keys.ts`, `src/utils/provider-env.ts` | `ai/auth/env.go` | phase 3 |
| `src/images*.ts`, `src/api/openrouter-images.ts` | `ai/images` | phase 13 |
| `src/cli.ts` | `cmd/pi-ai` | phase 14 |
| `src/utils/error-body.ts` | `ai/internal/httpx` | phase 5 (partial: `ai/apis/anthropic` composes Anthropic's own status+body error text inline, matching the SDK contract this file documents, since Anthropic's error shape already "carries the body" and needs none of the other providers' SDK-field probing; the shared multi-SDK `normalizeProviderError`/`formatProviderError` moves to `ai/internal/httpx` whichever later phase (6+) first needs Mistral/openai/Bedrock-shaped probing) |
| `src/utils/node-http-proxy.ts` | Go `net/http` ProxyFromEnvironment + adapter checks | phase 5 |
| `src/session-resources.ts` | `ai/sessionresources.go` | ported |

## Intentional deviations

- **`src/compat.ts` and `src/legacy-api-aliases.ts` are not ported.** Both are
  deprecated back-compat shims for pre-`createModels()` JS callers; the Go port
  targets the `Models`/`createProvider` design directly.
- **Cancellation**: TS `AbortSignal` → `context.Context` passed as the first
  argument of `Stream`/`Complete`. Context cancellation produces the in-band
  `error` event with StopReason `"aborted"`, same as an aborted signal.
- **Durations**: TS `timeoutMs`/`maxRetryDelayMs` numbers → `time.Duration`
  fields (`Timeout`, `MaxRetryDelay`). Options are not part of the persisted
  wire format, so this does not affect interop.
- **Compat struct**: the three per-api TS compat interfaces are merged into one
  flat Go `ai.Compat` struct (the wire JSON is a flat object in both ports);
  adapters read only their own fields.
- **Per-adapter options struct**: TS's `AnthropicOptions extends StreamOptions`
  (and sibling per-api options interfaces) rely on function overloading that
  Go's single `StreamFunc` signature can't express. Anthropic-specific request
  knobs (`ThinkingEnabled`, `ThinkingBudgetTokens`, `Effort`,
  `ThinkingDisplay`) are merged directly into `ai.StreamOptions` instead,
  mirroring the Compat merge above; other adapters ignore fields they don't
  read.
- **Vendor SDKs**: the TS package delegates transport to @anthropic-ai/sdk,
  openai, @google/genai, @mistralai/mistralai. The Go port uses raw `net/http`
  plus `ai/internal/sse` for those protocols, and `aws-sdk-go-v2` for Bedrock
  (SigV4 + event-stream framing). Vertex's Application Default Credentials
  path is the one exception needing a real Google-authored dependency rather
  than raw `net/http`: `golang.org/x/oauth2/google` performs the actual ADC
  discovery/token-refresh algorithm (well-known credentials file, metadata
  server, ...), which would be impractical to hand-roll faithfully.
- **`src/utils/provider-env.ts` Bun sandbox fallback** (/proc/self/environ) is
  not ported; Go reads the process environment directly.
- **`src/utils/abort-signals.ts`** (combineAbortSignals) is not ported;
  compose `context.Context` instead.
- **Codex tool `strict` flag**: upstream's Codex adapter calls
  `convertResponsesTools(tools, { strict: null })`, sending a literal
  `"strict": null` on every tool. `ai/apis/codex` reuses the shared
  `openairesponses.ConvertTools` with its default `false` instead, sending
  `"strict": false`. No upstream or ported test asserts on this field's exact
  wire value; revisit if a future golden needs the literal `null`.
- **Mistral cached-prompt-token probing**: upstream's
  `getMistralCachedPromptTokens` defensively probes six differently-cased
  usage field variants (`promptTokensDetails`/`prompt_tokens_details`/
  `numCachedTokens`/...) across SDK versions. `ai/apis/mistral` probes only
  the documented `prompt_tokens_details.cached_tokens` wire field, since this
  port talks one fixed REST shape directly rather than an SDK whose JS
  bindings evolved across versions.
- **Mistral auth check has no header escape valve**: unlike the sibling
  anthropic/google adapters' `assertRequestAuth` (which accept a
  caller-supplied auth header in lieu of `apiKey`), `ai/apis/mistral` requires
  a non-empty `apiKey` unconditionally, matching upstream's own unconditional
  `if (!apiKey) throw ...` in `mistral-conversations.ts`'s `stream` function.

## Upstream sync procedure

1. `upstream/sync.sh` — fetches upstream and diffs `packages/ai` from the
   pinned SHA, bucketing changed files by area.
2. Catalog-only changes (`src/providers/*.models.ts`): re-run
   `tools/export-catalog` and the catalog tests; no Go code changes.
3. For each semantic change: port the change and its new tests into the mapped
   Go package (table above).
4. Update `upstream/UPSTREAM.lock` with the new SHA/version.
5. `go test ./... -race`, then the env-gated live smokes.

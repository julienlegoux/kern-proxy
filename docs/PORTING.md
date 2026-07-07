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
| `src/api/transform-messages.ts` | `ai/apis/transform.go` | phase 4 |
| `src/utils/validation.ts`, `src/utils/typebox-helpers.ts` | `ai/internal/jsonschema` | ported (typebox-helpers' StringEnum is a TypeBox schema-authoring convenience; plain JSON Schema documents need no equivalent) |
| `src/api/anthropic-messages.ts` | `ai/apis/anthropic` | phase 5 |
| `src/api/openai-completions.ts`, `openai-prompt-cache.ts` | `ai/apis/openaicompletions` | phase 6 |
| `src/api/openai-responses*.ts`, `azure-*`, `openai-codex-*` | `ai/apis/openairesponses` (+`azure`,`codex`) | phase 7 |
| `src/api/google-*.ts` | `ai/apis/google` (+`vertex`) | phase 8 |
| `src/api/mistral-conversations.ts` | `ai/apis/mistral` | phase 9 |
| `src/api/bedrock-converse-stream.ts`, `src/bedrock-provider.ts` | `ai/apis/bedrock` | phase 10 |
| `src/providers/*.ts` (bindings), `src/providers/all.ts` | `ai/providers` | phase 11 |
| `src/providers/*.models.ts`, `src/models.generated.ts` | `ai/catalog/data/*.json` (via `tools/export-catalog`) | phase 11 |
| `src/env-api-keys.ts`, `src/utils/provider-env.ts` | `ai/auth/env.go` | phase 3 |
| `src/images*.ts`, `src/api/openrouter-images.ts` | `ai/images` | phase 13 |
| `src/cli.ts` | `cmd/pi-ai` | phase 14 |
| `src/utils/error-body.ts` | `ai/internal/httpx` | phase 5 (shape depends on Go HTTP errors) |
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
- **Vendor SDKs**: the TS package delegates transport to @anthropic-ai/sdk,
  openai, @google/genai, @mistralai/mistralai. The Go port uses raw `net/http`
  plus `ai/internal/sse` for those protocols, and `aws-sdk-go-v2` for Bedrock
  (SigV4 + event-stream framing).
- **`src/utils/provider-env.ts` Bun sandbox fallback** (/proc/self/environ) is
  not ported; Go reads the process environment directly.
- **`src/utils/abort-signals.ts`** (combineAbortSignals) is not ported;
  compose `context.Context` instead.

## Upstream sync procedure

1. `upstream/sync.sh` — fetches upstream and diffs `packages/ai` from the
   pinned SHA, bucketing changed files by area.
2. Catalog-only changes (`src/providers/*.models.ts`): re-run
   `tools/export-catalog` and the catalog tests; no Go code changes.
3. For each semantic change: port the change and its new tests into the mapped
   Go package (table above).
4. Update `upstream/UPSTREAM.lock` with the new SHA/version.
5. `go test ./... -race`, then the env-gated live smokes.

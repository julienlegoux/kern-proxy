---
type: Reference
title: Porting map
description: Upstream-to-Go file mapping for the pi-ai port, intentional deviations, and the upstream sync procedure.
tags: [porting, upstream, parity]
timestamp: "2026-07-09"
---

# Porting map: @earendil-works/pi-ai → kern-proxy (Go)

kern-proxy is a full-parity Go rebuild of
[`@earendil-works/pi-ai`](https://github.com/earendil-works/pi/tree/main/packages/ai).
The pinned upstream revision lives in `upstream/UPSTREAM.lock`; use
`upstream/sync.sh` to diff upstream changes since the pin.

Every ported Go file carries a `// Ports: packages/ai/src/...` header comment
naming its upstream source.

# File mapping

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
| `src/api/lazy.ts` | `ai/lazy.go` | ported (only lazyStream semantics; TS module lazy-loading is a bundler concern) |
| `src/api/*.lazy.ts` (one per adapter) | n/a | not ported (bundler code-splitting shims; every adapter package is imported directly) |
| `src/api/simple-options.ts` | `ai/apis/simpleopts.go` | ported |
| `src/auth/types.ts`, `resolve.ts`, `credential-store.ts`, `context.ts` | `ai/auth.go`, `ai/resolve.go`, `ai/credentialstore.go`, `ai/authcontext.go` | ported (in package `ai`: a separate package would cycle with `Provider`) |
| persistent auth.json store (`packages/coding-agent`'s auth-storage) | `ai/auth/filestore.go` | ported |
| `src/auth/helpers.ts` | `ai/auth/helpers.go` | ported (`EnvAPIKeyAuth`; `lazyOAuth` is a bundler concern — Go providers construct `ai.OAuthAuth` directly) |
| `src/utils/oauth/*`, `src/oauth.ts` | `ai/auth/oauth` | ported (PKCE + callback-server scaffolding; Anthropic PKCE flow, Copilot device-code flow with RFC 8628 `slow_down` back-off, Codex dual PKCE/device-code flow; Codex stores `expires` without the 5-minute margin, matching upstream) |
| `src/api/transform-messages.ts` | `ai/apis/transform.go` | ported |
| `src/api/cloudflare.ts` (base-URL templates) | catalog `baseUrl` templates + `ai/providers/cloudflare_auth.go` | ported (resolved at runtime by `resolveCloudflareBaseURL`) |
| `src/api/github-copilot-headers.ts` | none | **not ported** (dynamic `X-Initiator`/`Copilot-Vision-Request` headers; open follow-up) |
| `src/utils/validation.ts`, `typebox-helpers.ts` | `ai/internal/jsonschema` | ported (TypeBox's `StringEnum` is an authoring convenience with no Go equivalent needed) |
| `src/api/anthropic-messages.ts` | `ai/apis/anthropic` | ported (incl. adaptive thinking, `cache_control`, OAuth Claude Code impersonation, retry/overflow classifiers) |
| `src/api/openai-completions.ts`, `openai-prompt-cache.ts` | `ai/apis/openaicompletions` | ported (vendor compat matrix, 10-format thinking encoding, prompt-cache and session-affinity headers) |
| `src/api/openai-responses.ts`, `openai-responses-shared.ts` | `ai/apis/openairesponses` | ported (converters exported for the Azure and Codex variants) |
| `src/api/azure-openai-responses*.ts` | `ai/apis/azure` | ported (env-based endpoint/deployment resolution over the shared responses core) |
| `src/api/openai-codex-responses*.ts` | `ai/apis/codex` | ported (WebSocket transport with SSE fallback and zstd request bodies; upstream's cross-request connection cache is not — each request opens its own connection) |
| `src/api/google-generative-ai.ts`, `google-shared.ts` | `ai/apis/google` | ported (raw `net/http` + SSE; no Go `@google/genai`) |
| `src/api/google-vertex.ts` | `ai/apis/google/vertex` | ported (ADC via `golang.org/x/oauth2/google`; defines its own tested contract for `baseUrl` overrides) |
| `src/api/mistral-conversations.ts` | `ai/apis/mistral` | ported (raw REST; no Go Mistral SDK) |
| `src/api/bedrock-converse-stream.ts`, `src/bedrock-provider.ts` | `ai/apis/bedrock` | ported (`aws-sdk-go-v2` bedrockruntime; full AWS auth matrix incl. SDK-native bearer-token auth) |
| `src/providers/*.ts` (bindings), `src/providers/all.ts` | `ai/providers` | ported (all ~35 bindings incl. OAuth wiring; native `RefreshModels` for OpenRouter/Vercel AI Gateway/NVIDIA/Copilot is a deviation — upstream refreshes its catalog at build time) |
| `src/providers/*.models.ts`, `src/models.generated.ts`, `src/image-models.generated.ts` | `ai/catalog/data/**/*.json` | ported (serialized by `tools/export-catalog`, embedded via `go:embed`) |
| `src/env-api-keys.ts` | `ai/providers/*.go` via `auth.EnvAPIKeyAuth` | ported (distributed: each binding declares its own env-var precedence list) |
| `src/images*.ts`, `src/api/openrouter-images.ts` | `ai/images` | ported (OpenRouter images adapter, API registry, embedded image catalog) |
| `src/cli.ts` | `cmd/pi-ai` | ported (`login`/`list`/`help` over the three OAuth flows and the shared file credential store) |
| `src/utils/error-body.ts` | inline per adapter | ported (distributed: each adapter formats its own status+body error text) |
| `src/utils/node-http-proxy.ts` | Go `net/http` | ported (built-in `ProxyFromEnvironment` covers the same env-var precedence) |
| `src/session-resources.ts` | `ai/sessionresources.go` | ported |

# Intentional deviations

- **`src/compat.ts` / `src/legacy-api-aliases.ts` not ported** — deprecated
  back-compat shims for pre-`createModels()` JS callers.
- **Cancellation**: TS `AbortSignal` → `context.Context` as the first argument
  of `Stream`/`Complete`; cancellation yields the in-band `error` event with
  StopReason `"aborted"`.
- **`Stream.Events(ctx)`**: upstream's `EventStream` is an async iterator that
  a JS consumer abandons with `break`, letting the generator be garbage
  collected. Go's equivalent is a goroutine feeding an unbuffered channel, and
  abandoning it strands that goroutine forever. `Events` therefore takes a
  `context.Context` where upstream takes nothing, and its pump exits on
  cancellation. A consumer that stops ranging early must cancel; pass
  `context.Background()` only when the loop always runs to completion.
- **Durations**: TS `timeoutMs`-style numbers → `time.Duration` fields.
- **Compat struct**: the three per-api TS compat interfaces merge into one flat
  `ai.Compat` struct; adapters read only their own fields.
- **Per-adapter options**: Anthropic-specific knobs (`ThinkingEnabled`,
  `ThinkingBudgetTokens`, `Effort`, `ThinkingDisplay`) merge into
  `ai.StreamOptions` since Go can't overload the single `StreamFunc` signature.
- **Vendor SDKs**: upstream delegates transport to the Anthropic/OpenAI/Google/
  Mistral JS SDKs; the Go port speaks raw `net/http` + `ai/internal/sse`,
  except Bedrock (`aws-sdk-go-v2` for SigV4 + event-stream framing) and Vertex
  ADC (`golang.org/x/oauth2/google` for credential discovery).
- **Codex tool `strict` flag**: sends `"strict": false` instead of upstream's
  literal `"strict": null`; no test on either side asserts the exact value.
- **Mistral cached-token probing**: reads only the documented
  `prompt_tokens_details.cached_tokens` field rather than upstream's six
  SDK-version-dependent casings.
- **Bedrock**: error formatting reads `smithy.APIError` directly instead of
  shape-probing; unrecognized image MIME types pass through to Bedrock's own
  validation instead of throwing; the Node-only HTTP-proxy-agent branch has no
  Go equivalent (Go's `net/http` handles proxies natively); Go-side tests add
  coverage for explicit-credentials/bearer-token/skip-auth cells upstream
  never tested.
- **Unicode-closed content interfaces**: upstream tests feeding
  `{ type: "unknown" }` content blocks aren't portable — Go's content-part
  interfaces structurally can't hold unknown block types.

# Upstream sync procedure

A weekly job (`.github/workflows/upstream-sync.yml`, Monday 06:00 UTC, plus
manual `workflow_dispatch`) runs `upstream/sync.sh` and opens an
`upstream-sync`-labeled GitHub issue when the diff is non-empty and no such
issue is already open. `upstream/sync_test.sh` (run on every push) offline-tests
the diff-detection logic.

On a non-empty diff:

1. `upstream/sync.sh` — fetches upstream and diffs `packages/ai` from the
   pinned SHA, bucketing changed files by area.
2. Catalog-only changes (`src/providers/*.models.ts`): re-run
   `tools/export-catalog` and the catalog tests; no Go code changes.
3. For each semantic change: port the change and its new tests into the mapped
   Go package (table above).
4. Update `upstream/UPSTREAM.lock` with the new SHA/version. Go tags mirror
   upstream releases (`v0.80.3-go.N`).
5. `go test ./... -race`, then the env-gated live smokes.

See [architecture](/architecture.md) for how the ported packages fit together.

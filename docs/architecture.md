---
type: Reference
title: Architecture
description: Package layering of the kern-link library — domain core, wire adapters, provider registry, auth, embedded catalog — and how a streaming request flows through them.
tags: [architecture, packages, providers]
timestamp: "2026-07-09"
---

# Architecture

kern-link is a library, not a server. Everything builds on the domain core in
package `ai`: a unified message model and streaming event protocol that every
provider adapter translates to and from.

```text
your code
   │  ai.Context / []ai.Message
   ▼
ai (domain core: Models registry, unified messages & events, auth resolution)
   │  model.Api selects the wire adapter
   ▼
ai/apis/* (wire adapters: anthropic, openaicompletions, openairesponses,
   │       azure, codex, google, google/vertex, mistral, bedrock)
   ▼
net/http + ai/internal/sse   (bedrock: aws-sdk-go-v2; codex: WebSocket)
```

# Package map

| Package | Purpose |
|---|---|
| `ai` | Domain core: `Message`/`Context` model, `Model` descriptor, `Stream` + event protocol, `StreamOptions`, cost (`CalculateCost`) & token estimation, retry/overflow classifiers, auth types + `ResolveProviderAuth`, `Models` registry (`CreateModels`/`CreateProvider`), JSON codecs for all unions |
| `ai/internal/sse` | Hand-rolled Server-Sent Events reader |
| `ai/internal/partialjson` | Partial-JSON parsing for streaming tool-argument deltas |
| `ai/internal/jsonschema` | Tool-argument validation/coercion against JSON Schema |
| `ai/apis` | Adapter-shared layer: `TransformMessages` cross-model normalization, simple-options resolution |
| `ai/apis/{anthropic,openaicompletions,openairesponses,azure,codex,google,google/vertex,mistral,bedrock}` | One package per wire protocol; each exposes `Stream`/`StreamSimple` matching `ai.StreamFunc` |
| `ai/providers` | 35 built-in provider bindings + `providers.Models()` aggregation; `RefreshModels` for the dynamic ones |
| `ai/providers/faux` | In-process scripted provider — the executable specification of the event contract |
| `ai/auth` | Persistent file credential store (cross-process locked), `EnvAPIKeyAuth` strategy builder |
| `ai/auth/oauth` | OAuth login flows: Anthropic PKCE, Copilot device-code, Codex dual, shared PKCE/callback/device-code scaffolding |
| `ai/catalog` | Embedded model catalog (`go:embed` JSON exported from upstream by `tools/export-catalog`) |
| `ai/images` | Image-generation stack (registry, OpenRouter adapter, image catalog) |
| `cmd/pi-ai` | CLI: `login` (OAuth), `list` (catalog), `help`; runnable examples |
| `upstream/` | Upstream pin (`UPSTREAM.lock`) + `sync.sh` diff tooling — see [PORTING](/PORTING.md) |

# The unified model

**Messages** (`ai/types.go`, JSON `role` discriminator): `UserMessage`,
`AssistantMessage`, `ToolResultMessage`. Assistant content is the closed union
`TextContent | ThinkingContent | ToolCall`; user/tool-result content is
`TextContent | ImageContent`. Everything JSON-round-trips, which is what makes
session persistence and cross-provider hand-off trivial.

**Events** (`ai/events.go`, JSON `type` discriminator): one `StartEvent`, then
`Text*`/`Thinking*`/`ToolCall*` start/delta/end triples (each non-terminal
event carries a `Partial *AssistantMessage` snapshot), terminated by exactly
one `DoneEvent` or `ErrorEvent`. Provider failures are in-band — they end the
stream with a `StopReason` of `error`/`aborted`, never a Go panic or a lost
error.

# Request flow

One streaming call, hop by hop:

1. **Lookup** — `providers.Models(opts)` builds a `MutableModels` registry
   with all 35 bindings; `models.GetModel(provider, id)` returns the
   `*ai.Model` descriptor from the embedded catalog.
2. **Stream entry** — `models.StreamSimple(...)` wraps setup in
   `ai.LazyStream`, so configuration errors surface as in-band stream events.
3. **Provider binding** — the registry finds the `ai.Provider` for
   `model.Provider`.
4. **Auth** — `ai.ResolveProviderAuth` (`ai/resolve.go`) resolves credentials
   with the precedence explicit request key → stored credential (OAuth
   refresh under a double-checked lock) → ambient env/AWS/ADC. The result is
   merged into cloned request copies of the model and options.
5. **Adapter selection** — `model.Api` (the wire-protocol string, e.g.
   `anthropic-messages`, `openai-completions`) picks the adapter: a binding
   either fronts a single protocol (`Api`) or dispatches per model
   (`ApiByProtocol`), e.g. GitHub Copilot speaks three protocols.
6. **Wire** — the adapter normalizes messages (`apis.TransformMessages`),
   builds the vendor request, and talks raw `net/http` + SSE (Bedrock:
   `aws-sdk-go-v2` with SigV4; Codex: WebSocket with SSE fallback).
7. **Events** — the adapter pushes unified `ai.Event`s onto the returned
   `*ai.Stream`; `Stream.Result(ctx)` resolves on the terminal event.

# Providers

35 registered bindings (`ai/providers/all.go`):

`amazon-bedrock`, `ant-ling`, `anthropic`, `azure-openai-responses`,
`cerebras`, `cloudflare-ai-gateway`, `cloudflare-workers-ai`, `deepseek`,
`fireworks`, `github-copilot`, `google`, `google-vertex`, `groq`,
`huggingface`, `kimi-coding`, `minimax`, `minimax-cn`, `mistral`,
`moonshotai`, `moonshotai-cn`, `nvidia`, `openai`, `openai-codex`,
`opencode`, `opencode-go`, `openrouter`, `together`, `vercel-ai-gateway`,
`xai`, `xiaomi`, `xiaomi-token-plan-ams`, `xiaomi-token-plan-cn`,
`xiaomi-token-plan-sgp`, `zai`, `zai-coding-cn`.

Most compat vendors reuse the `openaicompletions` adapter (with a per-vendor
compat matrix); several reuse `anthropic`; four are dynamic and fetch their
live model list via `RefreshModels` (openrouter, vercel-ai-gateway, nvidia,
github-copilot).

# Catalog

Model metadata (context windows, costs, thinking levels, compat flags) is not
hand-maintained: `tools/export-catalog` serializes the pinned upstream
checkout's generated catalog to JSON, which `ai/catalog` embeds via
`go:embed` and validates at load. Providers bind their models with
`catalog.BuiltinModels("<id>")`.

# Testing strategy

`go test ./... -race` runs everything offline: the `faux` provider exercises
the full event/tool/abort/prompt-cache contract in-process, adapters are
tested against recorded wire shapes, and OAuth flows against injectable
clocks and local callback servers. Live provider smokes are env-gated behind
real API keys, mirroring upstream.

For the upstream file-by-file mapping and deviations, see
[PORTING](/PORTING.md); for consumer-facing walkthroughs, see
[usage](/usage.md) and [auth](/auth.md).

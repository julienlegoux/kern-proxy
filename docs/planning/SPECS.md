---
type: Technical Specification
title: "kern-link — Technical Specs"
description: "A Go library exposing one streaming LLM interface across 35 providers — a domain core in package ai, nine wire adapters over net/http, an embedded model catalog, and file-backed credential resolution."
tags: [planning, specs]
timestamp: 2026-08-08T23:07:30Z
status: final
mapped_commit: b70cba3293f998348d60080a05954bbb2eb27785
mapped_at: 2026-07-29T04:36:27Z
---

# kern-link — Technical Specs

kern-link is a **library, not a service**. It is consumed with `go get` and
linked into the caller's process; there is no server, no database, and no
deployed artifact. It is a full-parity Go port of the TypeScript
`@earendil-works/pi-ai` package that tracks upstream over time — a fact that
shapes nearly every decision recorded below.

## Stack

Single Go module, `github.com/kern-ia/kern-link`, at Go **1.26**
(`go.mod`). CI pins the toolchain from `go-version-file: go.mod`, so the
declared version is the only version.

Direct dependencies, all narrow and purposeful:

| Dependency | Version | Why |
|---|---|---|
| `aws/aws-sdk-go-v2/config`, `credentials`, `service/bedrockruntime` | 1.32.28 / 1.19.27 / 1.55.0 | Bedrock adapter: SigV4 signing and the ambient AWS credential chain |
| `aws/smithy-go` | 1.27.3 | Bedrock event-stream and error plumbing |
| `coder/websocket` | 1.8.15 | Codex adapter's WebSocket transport |
| `gofrs/flock` | 0.13.0 | Cross-process lock on the credential store (`flock` on Unix, `LockFileEx` on Windows) |
| `klauspost/compress` | 1.19.0 | zstd for the Codex wire protocol |
| `santhosh-tekuri/jsonschema/v6` | 6.0.2 | Tool-argument validation and coercion |
| `golang.org/x/oauth2` | 0.36.0 | OAuth token plumbing |
| `golang.org/x/text` | 0.14.0 | Text normalization |

What is deliberately **absent** is as much of the spec as what is present: no
web framework, no ORM, no test framework (`testify` appears zero times), no
logging library, no DI container, no code generator. HTTP is raw `net/http`;
Server-Sent Events are a hand-rolled reader in `ai/internal/sse`. The only
non-Go tooling is `tools/export-catalog/export-catalog.ts`, a TypeScript script
run by hand against a pinned upstream checkout to regenerate the embedded
catalog.

## Architecture

Everything builds on the domain core in package `ai`: one unified message model
and one streaming event protocol that every provider adapter translates to and
from.

```text
consumer code
   │  ai.Context / []ai.Message
   ▼
ai                     domain core — messages, events, Models registry,
   │                   auth resolution, cost, retry/overflow classifiers
   │  model.Api selects the wire adapter
   ▼
ai/apis/*              nine wire adapters
   ▼
net/http + ai/internal/sse      (bedrock: aws-sdk-go-v2; codex: WebSocket)
```

**Package map:**

| Package | Responsibility |
|---|---|
| `ai` | `Message`/`Context` model, `Model` descriptor, `Stream` + event protocol, cost & token estimation, retry/overflow classifiers, `ResolveProviderAuth`, the `Models` registry, JSON codecs for every union |
| `ai/apis` | Adapter-shared layer: `TransformMessages` cross-model normalization, simple-options resolution |
| `ai/apis/{anthropic,openaicompletions,openairesponses,azure,codex,google,google/vertex,mistral,bedrock}` | One package per wire protocol; each exposes `Stream`/`StreamSimple` matching `ai.StreamFunc` |
| `ai/apis/internal/httpretry` | Shared HTTP retry helper, extracted from the Codex adapter |
| `ai/internal/sse` | Hand-rolled SSE reader |
| `ai/internal/partialjson` | Partial-JSON parsing and repair for streaming tool-argument deltas |
| `ai/internal/jsonschema` | Tool-argument validation/coercion against JSON Schema |
| `ai/providers` | 35 built-in provider bindings + `providers.Models()` aggregation; `RefreshModels` for the dynamic ones |
| `ai/providers/faux` | In-process scripted provider — the executable specification of the event contract |
| `ai/auth` | File credential store (cross-process locked), `EnvAPIKeyAuth` strategy builder |
| `ai/auth/oauth` | OAuth flows: Anthropic PKCE, Copilot device-code, Codex dual, shared PKCE/callback/device-code scaffolding |
| `ai/catalog` | Embedded model catalog (`go:embed data`), validated at load |
| `ai/images` | Image-generation stack: API registry, OpenRouter adapter, image catalog |
| `cmd/pi-ai` | CLI: `login`, `list`, `help`, plus runnable examples |
| `upstream/` | Upstream pin (`UPSTREAM.lock`) + `sync.sh` diff tooling |

**Two independent public surfaces.** `ai` + `ai/providers` is the text/streaming
API. `ai/images` is a *parallel* surface with its own registry — nothing in `ai`
or `ai/providers` imports it, and adapters self-register from `init()`. A
consumer wanting image generation imports `ai/images` directly. This mirrors
upstream's separate `images*.ts` module set and is intentional, not an oversight.

**Request flow**, one streaming call hop by hop:

1. **Lookup** — `providers.Models(opts)` builds a `MutableModels` registry with
   all 35 bindings; `models.GetModel(provider, id)` returns the `*ai.Model`
   descriptor from the embedded catalog.
2. **Stream entry** — `models.StreamSimple(...)` wraps setup in `ai.LazyStream`,
   so configuration errors surface as in-band stream events rather than as a
   returned error.
3. **Provider binding** — the registry finds the `ai.Provider` for
   `model.Provider`.
4. **Auth** — `ai.ResolveProviderAuth` (`ai/resolve.go`) resolves credentials,
   merging the result into cloned request copies of the model and options.
5. **Adapter selection** — `model.Api` (the wire-protocol string, e.g.
   `anthropic-messages`, `openai-completions`) picks the adapter. A binding
   either fronts a single protocol (`Api`) or dispatches per model
   (`ApiByProtocol`) — GitHub Copilot speaks three.
6. **Wire** — the adapter normalizes messages via `apis.TransformMessages`,
   builds the vendor request, and talks raw `net/http` + SSE. Bedrock uses
   `aws-sdk-go-v2` with SigV4; Codex uses WebSocket with an SSE fallback.
7. **Events** — the adapter pushes unified `ai.Event`s onto the returned
   `*ai.Stream`; `Stream.Result(ctx)` resolves on the terminal event.

**Concurrency model.** No goroutine pools or schedulers. Each `Stream` owns one
pump goroutine, cancellable via the context passed to `Stream.Events`. Dynamic
providers share one in-flight `RefreshModels` fetch under a mutex; on failure the
model list stays at its last-known state and a later call retries. OAuth refresh
runs under a double-checked lock. The credential store adds an OS-level lock so
concurrent *processes* cannot corrupt a read-modify-write.

## Data model & storage

There is **no database**. State is a conversation the caller holds, plus one
credential file.

**The unified message model** (`ai/types.go`, JSON `role` discriminator) is a
closed union: `UserMessage`, `AssistantMessage`, `ToolResultMessage`. Assistant
content is `TextContent | ThinkingContent | ToolCall`; user and tool-result
content is `TextContent | ImageContent`. `UserContent` models TypeScript's
`string | (TextContent | ImageContent)[]` with a Plain/Blocks pair where exactly
one is meaningful, so a plain string round-trips as a JSON string and blocks as
an array.

Every type round-trips through `encoding/json` — which is the whole persistence
story. A session is `[]ai.Message`; saving it and resuming it, on the same model
or a different provider, needs no extra machinery. Message implementations use
**pointer receivers** (unified in `564a3fe`, a deliberate breaking change) and
`MarshalJSON`/`UnmarshalJSON` live in `ai/json.go`, separate from the type
declarations.

**The event protocol** (`ai/events.go`, JSON `type` discriminator): one
`StartEvent`, then `Text*`/`Thinking*`/`ToolCall*` start/delta/end triples where
each non-terminal event carries a `Partial *AssistantMessage` snapshot,
terminated by exactly one `DoneEvent` or `ErrorEvent`. Events use **value
receivers**, in contrast to messages. Provider failures are in-band: the stream
ends with a `StopReason` of `error`/`aborted`, never a panic and never a lost
error.

**The model catalog** is not hand-maintained. `tools/export-catalog` serializes a
pinned upstream checkout's generated catalog into
`ai/catalog/data/models/*.json` (35 files) and `data/images/openrouter.json`;
`ai/catalog` embeds the tree with `go:embed data` and validates it at load.
Providers bind their models with `catalog.BuiltinModels("<id>")`. Catalog entries
carry context windows, per-million-token pricing, thinking levels, and
per-vendor compat flags.

**The credential store** (`ai/auth/filestore.go`) is a single JSON file, by
default `~/.pi/agent/auth.json`, one object keyed by provider ID, written with
mode `600` (asserted by test). Read-modify-write is serialized across processes
by a `gofrs/flock` lock on a sidecar `auth.json.lock`, so locking never touches
the store file itself.

## Auth

Credentials resolve automatically per provider in `ai/resolve.go`. Consumers
never set an HTTP header themselves. Precedence:

1. **Explicit per-request key** — `StreamOptions.APIKey` plus optional
   `StreamOptions.Env` overrides, built into a synthetic `api_key` credential.
   Bypasses everything below.
2. **Stored credential** — from the `CredentialStore` handed to
   `providers.Models`: an OAuth credential refreshed under a double-checked lock
   when expired, or a stored API key. A stored credential *owns* the provider —
   there is deliberately no silent env fallback behind it.
3. **Ambient sources** — env vars, the AWS credential chain, Google ADC — only
   when nothing is stored.

`models.GetAuth(ctx, model)` reports the outcome without sending a request:
`(nil, nil)` means unconfigured, and `AuthResult.Source` is a display label (the
env var name, `"stored credential"`, or `"OAuth"`). Failures are a typed
`*ModelsError` — code `oauth` when a refresh fails (the stored credential is
preserved so re-login fixes it), code `auth` when api-key resolution or the store
itself fails.

Each provider declares its env vars in precedence order; the full table lives in
[`docs/auth.md`](../auth.md). Notable non-uniform cases: `anthropic` checks
`ANTHROPIC_OAUTH_TOKEN` before `ANTHROPIC_API_KEY`; `amazon-bedrock` walks stored
bearer token → `AWS_BEARER_TOKEN_BEDROCK` → `AWS_PROFILE` → access-key pair →
container/web-identity, then SigV4-signs; `google-vertex` needs ADC *plus*
`GOOGLE_CLOUD_PROJECT` *plus* `GOOGLE_CLOUD_LOCATION`; `openai-codex` is OAuth
only, with no API-key path.

**Three OAuth flows** in `ai/auth/oauth`, over shared PKCE, local-callback, and
device-code scaffolding: Anthropic PKCE (Claude Pro/Max), GitHub Copilot device
code, and Codex dual-flow (ChatGPT Plus/Pro). `cmd/pi-ai login` drives them
interactively and persists to the default store.

**Terms-of-service split, documented deliberately.** API keys are billed per
token under a developer agreement written for programmatic access and carry no
ToS risk. Subscription OAuth yields a *first-party client's* credential, and
kern-link then presents itself as that client (`user-agent: claude-cli/…`,
`Editor-Version: vscode/…`). This is inherited upstream behavior, fine for
personal use, and a real risk to ship in a product — providers can revoke the
account. `docs/auth.md` and the README both say so in those terms; treat that
framing as load-bearing, not boilerplate.

## Interfaces & integrations

**The Go API is the interface.** Five interfaces in `ai/provider.go` carry the
contract: `Provider` (id/name/base metadata, auth methods, model listing, stream
behavior), `Models` (provider lookup, model lookup, `Refresh`, `GetAuth`, and the
`Stream`/`Complete`/`StreamSimple`/`CompleteSimple` quartet), and `MutableModels`
(adds registration). Doc comments on these methods specify failure behavior
precisely — `GetModels` "must not fail", `RefreshModels` keeps the last-known
list on error — and are part of the contract.

**35 provider bindings** (`ai/providers/all.go`): `amazon-bedrock`, `ant-ling`,
`anthropic`, `azure-openai-responses`, `cerebras`, `cloudflare-ai-gateway`,
`cloudflare-workers-ai`, `deepseek`, `fireworks`, `github-copilot`, `google`,
`google-vertex`, `groq`, `huggingface`, `kimi-coding`, `minimax`, `minimax-cn`,
`mistral`, `moonshotai`, `moonshotai-cn`, `nvidia`, `openai`, `openai-codex`,
`opencode`, `opencode-go`, `openrouter`, `together`, `vercel-ai-gateway`, `xai`,
`xiaomi`, `xiaomi-token-plan-ams`, `xiaomi-token-plan-cn`,
`xiaomi-token-plan-sgp`, `zai`, `zai-coding-cn`.

Most compat vendors reuse the `openaicompletions` adapter behind a per-vendor
compat matrix; several reuse `anthropic`. Four are **dynamic** and fetch their
live model list through `RefreshModels`: `openrouter`, `vercel-ai-gateway`,
`nvidia`, `github-copilot` — itself a deviation from upstream, which refreshes
its catalog at build time.

**`cmd/pi-ai`** is a three-command CLI (`login`, `list`, `help`) whose `main` is
a thin `os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))` — `run` takes its
writers as parameters and returns an exit code, which is what makes it testable.
`list` and `help` are pure functions of the embedded catalog and touch neither
network nor filesystem.

**`testbed/`** is a local web UI (`go run ./testbed`, binds `127.0.0.1:8787`
only, unauthenticated by design) for exercising the library live: any built-in
provider/model, streaming output, thinking blocks, tool-call round trips, usage
and cost in real time. It is **gitignored** — deliberately throwaway, not part of
the module, and nothing else references it.

## Deployment & operations

**No deployment.** kern-link is distributed as a Go module; consumers run
`go get github.com/kern-ia/kern-link`. There is no Dockerfile, no IaC, no
platform config, no release workflow — releases are a git tag plus a `CHANGELOG.md`
entry, prepared by hand (`d654012`, "chore(release): prepare v0.1.1 changelog").
Current version is **v0.1.1**. `CHANGELOG.md` follows Keep a Changelog 1.1.0 and
the project claims SemVer.

**No observability wiring, by design.** The library imports no logging package
whatsoever — `log`, `log/slog`, and third-party loggers appear zero times outside
the gitignored testbed. A library does not choose its consumer's logger. Instead,
non-fatal problems are surfaced as data: `AssistantMessageDiagnostic` attachments
on the returned message, each holding a redacted, serializable
`DiagnosticErrorInfo` (`name`, `message`, `stack`, `code`) produced by
`ExtractDiagnosticError`, which lifts a `Code() string` method off the error when
one exists. Fatal problems are typed errors or in-band error events. No metrics,
no tracing, no error-reporting SDK.

**Two GitHub Actions workflows.** `test.yml` runs on every push and pull request
with two jobs: `test` (`go test ./... -race -v`, then `bash upstream/sync_test.sh`)
and `lint` (`golangci-lint-action@v9` pinned to `v2.12.2`). `upstream-sync.yml`
runs Mondays at 06:00 UTC (plus `workflow_dispatch`), diffs upstream HEAD against
the pin via `upstream/sync.sh`, and opens a labeled GitHub issue when the diff is
non-empty — creating the `upstream-sync` label if needed and skipping when such
an issue is already open.

**Upstream parity is an operational concern, not just a historical one.** The
pinned revision lives in `upstream/UPSTREAM.lock`; `upstream/sync.sh` produces
the diff bucketed by area, and `upstream/sync_test.sh` tests that diff-detection
logic offline by building a throwaway git repo, so the weekly job stays honest
between real syncs. `docs/PORTING.md` holds the file-by-file upstream→Go mapping
and the intentional deviations.

## Testing infrastructure

**102 test files** against 126 non-test source files, co-located with the code
they test. Standard library `testing` only. `go test ./...` runs the entire suite
**offline** — no network, no fixtures directory, no container.

Three mechanisms carry that:

- **The `faux` provider** (`ai/providers/faux`) is an in-process provider that
  scripts canned responses and replays the full event protocol. It is described
  in its own docs as "the executable specification of the event contract", and
  the package examples run against it.
- **`net/http/httptest`** in 32 test files stands in for vendor endpoints;
  adapters are tested against recorded wire shapes.
- **Injectable clocks and local callback servers** for OAuth — `ai/resolve.go`
  exposes `var authClock = func() int64 { … }` specifically so tests can control
  expiry.

**Live provider tests are gated at runtime, not by build tag.** There are no
`//go:build` constraints anywhere in the repo. Each live test calls
`os.Getenv` and `t.Skip`s with a message naming the missing variable — e.g.
`t.Skip("GEMINI_API_KEY not set; skipping live Gemini smoke test")`. CI never
sets those secrets, so they always skip there. Eight adapters plus
`ai/images/openrouter_test.go` carry such a smoke test.

**Cross-process behavior is tested for real.** `ai/auth/filestore_test.go`
re-executes the test binary as a child process (guarded by its own `t.Skip`
sentinel) to prove that concurrent modification across processes is safe.

**Test commands** ([decision](/mapping/02-canonical-test-command.md)):

- **CI gate, authoritative** — `go test ./... -race -v`. A green race run is
  required before merge; `test.yml` carries a comment recording that
  `TestResolveExpiredOAuthRefreshesOnceUnderLock` visibly executing under the
  detector was an explicit acceptance criterion.
- **Local** — `go test ./...` with `GOTMPDIR` set to the repo's gitignored
  `.gotmp/`. `-race` is CI-only: it needs a C toolchain that is not installed
  locally, and Application Control blocks execution of test binaries built into
  the system temp directory.

No coverage configuration, threshold, or reporting exists (`coverage.out` is
merely gitignored).

## Cross-cutting concerns

**Error text is load-bearing.** `ai/retry.go` and `ai/overflow.go` classify
transient failures and context overflow by regex-matching composed error
*strings* (e.g. `"stream ended before message_stop"`, `"insufficient_quota"`,
`"Monthly usage limit reached"`). Those strings are copied verbatim from
upstream, where they are user-facing and sentence-cased. Rewriting one to Go
house style is a behavior change, not a style fix — this is why `staticcheck`'s
ST1005 is disabled with a long justification. `retry.go` further warns that
status-code patterns (`429`, `500`, …) are text patterns valid only against
strings *known to start with* a status code, since a 400 rejecting
`max_tokens: 15000` contains the digits "500".

**Upstream provenance is tracked in the source.** 157 files carry a
`// Ports: <upstream path>` header naming the upstream source, placed after the
`package` clause. The only two non-test files without one are genuinely new code:
`ai/apis/internal/httpretry/httpretry.go` (extracted from the Codex adapter) and
`ai/doc.go`.

**Known gap, recorded upstream-side.** `src/api/github-copilot-headers.ts` is not
ported — the dynamic `X-Initiator` and `Copilot-Vision-Request` headers — logged
as an open follow-up in `docs/PORTING.md:46`. Other intentional deviations
(`compat.ts`/`legacy-api-aliases.ts` skipped as deprecated, `*.lazy.ts` shims
irrelevant without a bundler) are enumerated in the same file.

**Cost and token accounting** is first-class: `AssistantMessage.Usage` carries
unified counts including cache-read/write splits, `ai.CalculateCost` prices a
`Usage` against the catalog's per-million-token sheet, and `ai/estimate.go`
estimates tokens ahead of a request — skipping failed assistant turns.

**Portable capability clamping.** Thinking levels, tool calling, and
retry/overflow behavior are expressed once in `ai` and clamped down to what each
model's catalog entry declares support for, rather than each adapter inventing
its own fallbacks.

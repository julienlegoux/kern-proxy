---
type: Issue
title: "Introduce ProviderRequestOptions as the shared request base, with fetch injection and samplingParams"
description: "Refactor ai/options.go so transport, auth, and lifecycle knobs live in one reusable base that StreamOptions and the deferred options extend, add the fetch and samplingParams knobs, and record the telemetry deviation."
tags: [epic-2]
timestamp: 2026-08-11T13:10:00Z
epic: 2
issue: 05
slug: provider-request-options
size: L
status: open
gh_issue: 133
resource: https://github.com/kern-ia/kern-link/issues/133
depends_on: []
---

# Introduce ProviderRequestOptions as the shared request base, with fetch injection and samplingParams

## Summary

Upstream split `StreamOptions` in two. The transport/auth/lifecycle half became
`ProviderRequestOptions<TModel>`, and `StreamOptions`, `ImagesOptions`,
`DeferredFetchOptions` and `DeferredCancelOptions` all extend it:

```ts
export interface ProviderRequestOptions<TModel = Model<Api>> {
	signal?; telemetryContext?; apiKey?; fetch?; env?;
	onPayload?(payload, model: TModel); onResponse?(response, model: TModel);
	headers?; timeoutMs?; maxRetries?; maxRetryDelayMs?;
}
export interface StreamOptions extends ProviderRequestOptions<Model<Api>> { … }
export interface DeferredFetchOptions extends ProviderRequestOptions<Model<Api>> { wait?: number }
export type DeferredCancelOptions = ProviderRequestOptions<Model<Api>>;
```

This is the structural change [decision 04](../../../planning/scope/04-core-types-migration.md)
put at the head of the program: the deferred-response entry points in
[issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md)
need the transport knobs *without* the streaming ones, and hand-duplicating ten
fields across four option types is exactly the drift the next sync pays for.

Two request-level knobs arrive with the split and belong in the same PR because
they thread through the same struct: `fetch` (an injectable HTTP client) and
`samplingParams` (arbitrary sampling parameters, on both `Model` and
`StreamOptions`).

## Scope

- `ai/options.go` — introduce `ProviderRequestOptions` carrying `APIKey`,
  `Env`, `Headers`, `Timeout`, `MaxRetries`, `MaxRetryDelay`, `OnPayload`,
  `OnResponse`, and the new `Fetch`; embed it in `StreamOptions`, which keeps
  the streaming-only fields (`Temperature`, `MaxTokens`, `Transport`,
  `CacheRetention`, `SessionID`, `WebsocketConnectTimeout`, `Metadata`) plus the
  per-adapter flat-merge fields already documented there.
  - Cancellation stays on `context.Context`, not a `Signal` field — the
    established deviation (`docs/PORTING.md`, "Cancellation").
  - Upstream's `TModel` generic parameter exists so `ImagesOptions` can type its
    callbacks against `ImagesModel`. Go's `OnPayload`/`OnResponse` already take
    `*Model`; whether to generify, duplicate, or leave images alone is an
    implementer call — **record it in `docs/PORTING.md`'s deviations either way.**
- **Rewrite the existing keyed composite literals.** Go promotes embedded fields
  for **selectors** (`opts.APIKey` keeps compiling after the split) but not for
  **composite-literal keys**, so every `StreamOptions{...}` literal that keys
  `APIKey`, `Env`, `Headers`, `Timeout`, `MaxRetries`, `MaxRetryDelay`,
  `OnPayload` or `OnResponse` directly stops compiling the moment those fields
  move onto the embedded `ProviderRequestOptions`. `grep -rn "StreamOptions{"
  --include=*.go .` finds 328 opening braces today; checking each literal's full
  body (not just its opening line — a same-line-only filter undercounts to 218,
  because 36 literals key a moved field on a later line) against those field
  names gives **248** that need the mechanical rewrite. Re-derive both numbers
  at the PR's base commit. Rewrite each to
  `ai.StreamOptions{ProviderRequestOptions: ai.ProviderRequestOptions{APIKey: "k", ...}}`,
  keeping any streaming-only field (e.g. `Temperature`) as a sibling top-level
  key in the same literal.
- **The two OpenAI-family flat-merge fields Epic 4 waits on.** Declare both on
  `StreamOptions`, in the existing
  `--- openai-completions / openai-responses ---` block beside `ReasoningEffort`
  (`ai/options.go:142-154`). Both are declarations only; the wire work is
  [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md)'s issues 04, 05, 09 and 11.
  - `OpenAIToolChoice any` — upstream types `toolChoice` per adapter as an open
    union: `OpenAI.Chat.Completions.ChatCompletionToolChoiceOption`
    (`packages/ai/src/api/openai-completions.ts:143` at `936aff00`),
    `ResponseCreateParamsStreaming["tool_choice"]`
    (`openai-responses.ts:96`), and `"auto" | "none" | "required"`
    (`openai-codex-responses.ts:91`). One `any` carries all three and marshals
    through unchanged; nil means unset. This is the one place the
    `<Vendor>ToolChoice string` + `<Vendor>ToolChoiceFunction string` pattern
    (`GoogleToolChoice` `ai/options.go:202`, `MistralToolChoice` `:227`,
    `BedrockToolChoice` `:265`) deliberately does **not** apply: those vendors'
    unions are closed and enumerable, OpenAI's is not, and
    [Epic 4 issue 09](/epic-4-openai-family-adapters/issues/09-openai-responses-compat-and-wiring.md)
    requires an arbitrary tool-choice value to round-trip into the body
    unchanged. Record that reasoning in the field's doc comment.
  - `OpenAIThinkingBudgets *ThinkingBudgets` — upstream's
    `OpenAICompletionsOptions.thinkingBudgets` (`openai-completions.ts:146`),
    mirroring the existing `BedrockThinkingBudgets *ThinkingBudgets`
    (`ai/options.go:276-278`). `SimpleStreamOptions.ThinkingBudgets` (`:325-326`)
    is **not** this field and cannot stand in for it:
    `buildParams(model, chat, opts *ai.StreamOptions)`
    (`ai/apis/openaicompletions/openaicompletions.go:340`) only ever sees a
    `*StreamOptions`, which is exactly why bedrock already needs its own copy
    (`ai/apis/bedrock/bedrock.go:93-127` copies `opts.ThinkingBudgets` across on
    every branch).
  - No struct tag on either: `StreamOptions` carries no JSON tags at all
    (`grep -c 'json:"' ai/options.go` returns `2`, both on `ProviderResponse`
    at `:43-44`), and these two keep that.
- **`FetchFunction`** — upstream's `typeof globalThis.fetch`. In Go the
  equivalent is an injectable HTTP doer. Add `Fetch` to
  `ProviderRequestOptions` as a narrow interface or func type (e.g.
  `type FetchFunction func(*http.Request) (*http.Response, error)`), defaulting
  to the adapter's current client when nil, and document that it does not affect
  WebSocket transports (upstream says so explicitly).
- **`samplingParams`** — `map[string]any` on both `Model`
  (`json:"samplingParams,omitempty"`) and `StreamOptions`; per-request keys
  override per-model keys. Upstream merges them in `simple-options.ts`; the
  merge itself lands in
  [issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md).
- **`DeferredFetchOptions` / `DeferredCancelOptions`** — declare both here as
  thin extensions of the new base (`Wait time.Duration` on the fetch variant,
  zero meaning "one status check"). Their dispatch is issue 10.
- **The telemetry deviation.** Upstream's `ProviderRequestOptions` carries
  `telemetryContext?: TelemetryContext` from `@earendil-works/pi-telemetry`.
  kern-link takes **no new direct dependencies** and imports no logger at all
  ([SPECS.md](../../../planning/SPECS.md), "Deployment & operations";
  [CONVENTIONS.md](../../../planning/CONVENTIONS.md), "Error handling"), so the
  field is **not ported**. Add it to `docs/PORTING.md`'s "Intentional
  deviations" list with that reasoning, so the next sync does not rediscover it
  as an oversight.

## Out of scope

- **Adapter behavior.** Nine adapter packages read `StreamOptions` fields.
  Embedded-field promotion keeps their **selector** reads (`opts.APIKey`)
  compiling unchanged; only the keyed composite literals in `## Scope` need the
  mechanical rewrite. This PR changes no adapter behavior. Honoring `Fetch` and
  `SamplingParams` on the wire is [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md)
  and [Epic 5](/epic-5-remaining-adapters/EPIC_5.md).
- **`ai/images`.** Upstream reshaped `ImagesOptions` onto the same base
  (`packages/ai/src/types.ts:293-299` at `936aff00`), but this PR does **not**
  touch `ai/images` — unconditionally. The earlier "touch it only if the
  refactor breaks compilation" reading could never fire: `images.Options`
  (`ai/images/types.go:81-106`) is a standalone struct that hand-duplicates the
  transport fields rather than embedding `ai.StreamOptions`, so splitting
  `StreamOptions` leaves it compiling untouched, and the reshape would simply
  never happen. It is owned by
  [Epic 3 issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  (#218), which is where
  [decision 14](../../../planning/scope/14-images-surface.md)'s Verdict puts the
  images surface.
- `ModelsRequestTransforms` and the `Models`-level option aliases — 
  [issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md).
- Idiomatic-Go cleanups of the flat per-adapter option fields. Upstream's shape
  outranks Go idiom in ported files.

## Acceptance criteria / Definition of done

- [ ] `ai.ProviderRequestOptions` exists and is embedded by `ai.StreamOptions`,
      `ai.DeferredFetchOptions`, and `ai.DeferredCancelOptions`; no transport
      field is declared twice
      (`grep -c "MaxRetryDelay \*time.Duration" ai/options.go` returns `1`).
- [ ] Field *access* is unchanged by promotion: a caller reading `opts.APIKey`
      still compiles after the split, proved by a test.
- [ ] Every keyed `StreamOptions` composite literal identified in `## Scope`
      (**248** lines, re-derived at the PR's base commit) is mechanically
      rewritten to key the embedded struct explicitly:
      `ai.StreamOptions{ProviderRequestOptions: ai.ProviderRequestOptions{...}}`.
      No acceptance criterion in this epic claims the literals survive the
      split unchanged.
- [ ] `TestFetchFunctionOverridesTransport` — an adapter-agnostic test in `ai`
      proving a non-nil `Fetch` is what a request goes through, and that nil
      falls back to the default client. If no seam in `ai` can prove it without
      adapter work, the criterion is instead a compile-time assertion plus an
      explicit note in the PR body that behavior lands in epic 4 — do not
      silently drop it.
- [ ] `TestModelMarshalsSamplingParams` — a `Model` with `SamplingParams` set
      round-trips through `json.Marshal`/`Unmarshal`, and the key is omitted
      entirely when unset. Merge precedence itself is
      [issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md)'s
      `TestBuildBaseOptionsMergesSamplingParams`, the sole test asserting it.
- [ ] `ai.StreamOptions` declares `OpenAIToolChoice any` and
      `OpenAIThinkingBudgets *ThinkingBudgets`
      (`grep -n 'OpenAIToolChoice\|OpenAIThinkingBudgets' ai/options.go` returns
      both), each with the doc comment described in `## Scope`. No adapter reads
      them yet.
- [ ] `docs/PORTING.md` records **two** entries: the `telemetryContext`
      non-port, and the `TModel` generic decision.
- [ ] `ModelsStoreEntry`'s deep copy (`ai/modelsstore.go`, from
      [issue 07](/epic-2-core-types-and-models-contracts/issues/07-models-store.md))
      and `TestInMemoryModelsStoreReadReturnsCopy` are extended to clone
      `Model.SamplingParams`, the map this PR adds — mutating a returned model's
      sampling params must not change stored state.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] `gofmt -l .` prints nothing; `ai/options.go` keeps its `// Ports:` header,
      updated if the ported symbol list changed.
- [ ] Conventional Commit, e.g.
      `refactor(ai)!: split StreamOptions over a ProviderRequestOptions base`.

## Relevant files / areas

- `ai/options.go` (343 lines) — `StreamOptions` and every per-adapter field
  merged into it; `:75-290` is the struct itself, `:303-317` its helper methods
  (`EffectiveCacheRetention`, `EffectiveMaxRetryDelay`), `:319-343`
  `SimpleStreamOptions`, `StreamFunc`, `ProviderStreams`.
- `ai/model.go:54-71` — `Model`, which gains `SamplingParams`.
- `ai/modelsstore.go` — from
  [issue 07](/epic-2-core-types-and-models-contracts/issues/07-models-store.md);
  its `ModelsStoreEntry` deep copy must clone the new `SamplingParams` map.
- `ai/apis/simpleopts.go:31` `BuildBaseOptions` — constructs a `StreamOptions`
  field by field; it will need the new fields wired in
  ([issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md)),
  but must compile after this PR.
- `ai/provider.go:72-76, 299-344` — the `Models` quartet passing options
  through; signatures are unchanged here.
- `ai/apis/internal/httpretry/httpretry.go` — where a `Fetch` override would
  plausibly be honored; new code, no `// Ports:` header, do not add one.
- `docs/PORTING.md` — mapping table and "Intentional deviations".
- Upstream: `src/types.ts` at `936aff00`.

## Dependencies

- **Blocked by**: None, but in practice rebase on issues 01–04, which all touch
  `ai/types.go` / `ai/model.go`.
- **Blocks**: [Issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md),
  [Issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md),
  [Issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md),
  [Issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md).
  Cross-epic, through the two fields declared above (`depends_on` carries
  intra-epic numbers only, so the edge lives here in prose):
  [Epic 4 issue 02](/epic-4-openai-family-adapters/issues/02-fetch-and-sampling-params.md)
  (`Fetch`, `SamplingParams`),
  [Epic 4 issue 04](/epic-4-openai-family-adapters/issues/04-completions-thinking-formats.md)
  (`OpenAIThinkingBudgets`), and
  [Epic 4 issues 05](/epic-4-openai-family-adapters/issues/05-completions-deferred-tools-and-finish-reason.md),
  [09](/epic-4-openai-family-adapters/issues/09-openai-responses-compat-and-wiring.md)
  and [11](/epic-4-openai-family-adapters/issues/11-codex-request-body-and-stop-reasons.md)
  (`OpenAIToolChoice`), and
  [Epic 3 issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  (#218), which puts `images.Options` on the base declared here and consumes
  `FetchFunction`.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. **Sized L deliberately**: the struct split, the two new knobs, and the
compile-fix ripple across `ai/apis/*` cannot land separately without leaving the
tree uncompilable between PRs. If the ripple turns out to be larger than
mechanical — an adapter needing real behavior changes to build — stop, land the
struct split alone, and open a follow-up rather than growing this PR past ~1000
lines.

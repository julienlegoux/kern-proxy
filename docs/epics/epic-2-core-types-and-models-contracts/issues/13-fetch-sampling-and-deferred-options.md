---
type: Issue
title: "Add Fetch and SamplingParams to the provider request base, and declare the deferred option variants"
description: "Extend ProviderRequestOptions with fetch injection, add samplingParams to Model and StreamOptions, declare DeferredFetchOptions/DeferredCancelOptions and the two OpenAI-family StreamOptions fields Epic 4 waits on, and record the telemetry and TModel-generic PORTING decisions."
tags: [epic-2]
timestamp: 2026-08-11T16:20:00Z
epic: 2
issue: 13
slug: fetch-sampling-and-deferred-options
size: M
status: open
gh_issue: 225
resource: https://github.com/kern-ia/kern-link/issues/225
depends_on: [5]
---

# Add Fetch and SamplingParams to the provider request base, and declare the deferred option variants

## Summary

Numbered 13 rather than inserted between the existing 05 and 06: this issue is
one half of a pre-split
([Epic 0 issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md))
of what was originally Epic 2 issue 05. The struct split and its 248-site
compile-fix ripple landed in
[issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md);
this issue adds the two request-level knobs that arrive in the same upstream
diff — `fetch` (an injectable HTTP client) and `samplingParams` (arbitrary
sampling parameters, on both `Model` and `StreamOptions`) — plus the deferred
option variants, the two OpenAI-family fields three Epic 4 issues wait on, and
the `docs/PORTING.md` entries the `Fetch`/images chain forces. Renumbering the
epic's existing 06–12 to make room was rejected as unnecessary churn with a
real chance of a mistake; a backward-pointing dependency by number is legal
and cheaper.

## Scope

- `ai/options.go` — add `Fetch` to `ProviderRequestOptions` (the struct
  [issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
  creates). Upstream's `TModel` generic parameter exists so `ImagesOptions` can
  type its callbacks against `ImagesModel`. Go's `OnPayload`/`OnResponse`
  already take `*Model`; whether to generify, duplicate, or leave images alone
  is an implementer call — **record it in `docs/PORTING.md`'s deviations either
  way.**
- **`FetchFunction`** — upstream's `typeof globalThis.fetch`. In Go the
  equivalent is an injectable HTTP doer. Add `Fetch` as a narrow interface or
  func type (e.g. `type FetchFunction func(*http.Request) (*http.Response, error)`),
  defaulting to the adapter's current client when nil, and document that it does
  not affect WebSocket transports (upstream says so explicitly).
- **`samplingParams`** — `map[string]any` on both `Model`
  (`json:"samplingParams,omitempty"`) and `StreamOptions`; per-request keys
  override per-model keys. Upstream merges them in `simple-options.ts`; the
  merge itself lands in
  [issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md).
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
- **`DeferredFetchOptions` / `DeferredCancelOptions`** — declare both here as
  thin extensions of `ProviderRequestOptions` (`Wait time.Duration` on the
  fetch variant, zero meaning "one status check"). Their dispatch is
  [issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md).
- **The telemetry deviation.** Upstream's `ProviderRequestOptions` carries
  `telemetryContext?: TelemetryContext` from `@earendil-works/pi-telemetry`.
  kern-link takes **no new direct dependencies** and imports no logger at all
  ([SPECS.md](../../../planning/SPECS.md), "Deployment & operations";
  [CONVENTIONS.md](../../../planning/CONVENTIONS.md), "Error handling"), so the
  field is **not ported**. Add it to `docs/PORTING.md`'s "Intentional
  deviations" list with that reasoning, so the next sync does not rediscover it
  as an oversight. Recorded here rather than in issue 05 because it is
  discovered alongside the `TModel`-generic decision above, and both feed the
  same `Fetch`-carrying images reshape
  ([Epic 3 issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)).

## Out of scope

- **The `ProviderRequestOptions` struct itself, its embedding into
  `StreamOptions`, and the 248-site literal rewrite that follows.** That is
  [issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md),
  which this issue depends on and extends.
- **Adapter behavior.** Honoring `Fetch` and `SamplingParams` on the wire is
  [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md) and
  [Epic 5](/epic-5-remaining-adapters/EPIC_5.md). No adapter reads
  `OpenAIToolChoice` / `OpenAIThinkingBudgets` yet.
- **`ai/images`.** [Epic 3 issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  (#218) consumes `FetchFunction` and the `TModel` decision this issue records;
  it is not touched here.
- `ModelsRequestTransforms` and the `Models`-level option aliases —
  [issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md).
- `SamplingParams` merge precedence — [issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md)'s
  `TestBuildBaseOptionsMergesSamplingParams`, the sole test asserting it.

## Acceptance criteria / Definition of done

- [ ] `ai.ProviderRequestOptions` declares `Fetch`
      (`grep -n "Fetch " ai/options.go` shows it inside the struct issue 05
      created).
- [ ] `TestFetchFunctionOverridesTransport` — an adapter-agnostic test in `ai`
      proving a non-nil `Fetch` is what a request goes through, and that nil
      falls back to the default client. If no seam in `ai` can prove it without
      adapter work, the criterion is instead a compile-time assertion plus an
      explicit note in the PR body that behavior lands in epic 4 — do not
      silently drop it.
- [ ] `TestModelMarshalsSamplingParams` — a `Model` with `SamplingParams` set
      round-trips through `json.Marshal`/`Unmarshal`, and the key is omitted
      entirely when unset.
- [ ] `ai.StreamOptions` declares `OpenAIToolChoice any` and
      `OpenAIThinkingBudgets *ThinkingBudgets`
      (`grep -n 'OpenAIToolChoice\|OpenAIThinkingBudgets' ai/options.go` returns
      both), each with the doc comment described in `## Scope`. No adapter reads
      them yet.
- [ ] `ai.DeferredFetchOptions` and `ai.DeferredCancelOptions` exist, both
      embedding `ai.ProviderRequestOptions`.
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
- [ ] `gofmt -l .` prints nothing; `ai/options.go` keeps its `// Ports:` header.
- [ ] Conventional Commit, e.g.
      `feat(ai): add Fetch and SamplingParams to the provider request base`.

## Relevant files / areas

- `ai/options.go` — `ProviderRequestOptions` (from
  [issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)),
  `:142-154` the `openai-completions / openai-responses` flat-merge block,
  `:202`, `:227`, `:265`, `:276-278` the other vendor tool-choice / thinking
  fields this issue's additions sit beside.
- `ai/model.go:54-71` — `Model`, which gains `SamplingParams`.
- `ai/modelsstore.go` — from
  [issue 07](/epic-2-core-types-and-models-contracts/issues/07-models-store.md);
  its `ModelsStoreEntry` deep copy must clone the new `SamplingParams` map.
- `ai/apis/internal/httpretry/httpretry.go` — where a `Fetch` override would
  plausibly be honored; new code, no `// Ports:` header, do not add one.
- `docs/PORTING.md` — mapping table and "Intentional deviations".
- Upstream: `src/types.ts` at `936aff00`.

## Dependencies

- **Blocked by**: [Issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
  — extends the struct it creates.
- **Blocks**: [Issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md)
  (`Fetch`, `SamplingParams`),
  [Issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md)
  (`DeferredFetchOptions` / `DeferredCancelOptions`).
  Cross-epic, through the fields declared above (`depends_on` carries
  intra-epic numbers only, so the edge lives here in prose):
  [Epic 4 issue 02](/epic-4-openai-family-adapters/issues/02-fetch-and-sampling-params.md)
  (`Fetch`, `SamplingParams`),
  [Epic 4 issue 04](/epic-4-openai-family-adapters/issues/04-completions-thinking-formats.md)
  (`OpenAIThinkingBudgets`),
  [Epic 4 issues 05](/epic-4-openai-family-adapters/issues/05-completions-deferred-tools-and-finish-reason.md),
  [09](/epic-4-openai-family-adapters/issues/09-openai-responses-compat-and-wiring.md)
  and [11](/epic-4-openai-family-adapters/issues/11-codex-request-body-and-stop-reasons.md)
  (`OpenAIToolChoice`),
  [Epic 5 issue 02](/epic-5-remaining-adapters/issues/02-anthropic-stream-lifecycle.md)
  and [Epic 5 issue 06](/epic-5-remaining-adapters/issues/06-google-and-vertex-stream-and-params.md)
  (`Fetch`), and
  [Epic 3 issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  (#218), which puts `images.Options` on the base issue 05 declares and
  consumes `FetchFunction` from this issue.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. Expected well under the ceiling: one struct field, a `Model` field plus its
deep-copy extension, two thin option types, two `StreamOptions` field
declarations with doc comments, and two `docs/PORTING.md` rows — none of it
touches the 248-site literal rewrite issue 05 already landed.

---
type: Issue
title: "Introduce ProviderRequestOptions as the shared request base, with fetch injection and samplingParams"
description: "Refactor ai/options.go so transport, auth, and lifecycle knobs live in one reusable base that StreamOptions and the deferred options extend, add the fetch and samplingParams knobs, and record the telemetry deviation."
tags: [epic-2]
timestamp: 2026-08-09T04:31:17Z
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

- **Adapter behavior.** Nine adapter packages read `StreamOptions` fields. This
  PR makes whatever mechanical edits keep them compiling — field promotion
  through an embedded struct usually means none — and changes no adapter
  behavior. Honoring `Fetch` and `SamplingParams` on the wire is
  [Epic 4](/epic-4-openai-family-adapters/EPIC_4.md) and
  [Epic 5](/epic-5-remaining-adapters/EPIC_5.md).
- **`ai/images`.** Upstream reshaped `ImagesOptions` onto the same base;
  [Epic 3](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md) owns the images
  surface ([decision 14](../../../planning/scope/14-images-surface.md)). Touch it
  only if the refactor breaks compilation, and say so in the PR body.
- `ModelsRequestTransforms` and the `Models`-level option aliases — 
  [issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md).
- Idiomatic-Go cleanups of the flat per-adapter option fields. Upstream's shape
  outranks Go idiom in ported files.

## Acceptance criteria / Definition of done

- [ ] `ai.ProviderRequestOptions` exists and is embedded by `ai.StreamOptions`,
      `ai.DeferredFetchOptions`, and `ai.DeferredCancelOptions`; no transport
      field is declared twice
      (`grep -c "MaxRetryDelay \*time.Duration" ai/options.go` returns `1`).
- [ ] Existing call sites still compile without churn: a caller writing
      `&ai.StreamOptions{APIKey: "k", Timeout: time.Second}` keeps working
      (embedded-field promotion), and a test asserts it.
- [ ] `TestFetchFunctionOverridesTransport` — an adapter-agnostic test in `ai`
      proving a non-nil `Fetch` is what a request goes through, and that nil
      falls back to the default client. If no seam in `ai` can prove it without
      adapter work, the criterion is instead a compile-time assertion plus an
      explicit note in the PR body that behavior lands in epic 4 — do not
      silently drop it.
- [ ] `TestSamplingParamsPerRequestOverridesModel` — merge precedence is
      asserted at whichever layer this PR places it.
- [ ] `docs/PORTING.md` records **two** entries: the `telemetryContext`
      non-port, and the `TModel` generic decision.
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

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. **Sized L deliberately**: the struct split, the two new knobs, and the
compile-fix ripple across `ai/apis/*` cannot land separately without leaving the
tree uncompilable between PRs. If the ripple turns out to be larger than
mechanical — an adapter needing real behavior changes to build — stop, land the
struct split alone, and open a follow-up rather than growing this PR past ~1000
lines.

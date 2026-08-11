---
type: Issue
title: "Introduce ProviderRequestOptions as the shared request base, and rewrite the affected StreamOptions literals"
description: "Refactor ai/options.go so transport, auth, and lifecycle knobs live in one reusable base that StreamOptions embeds, and mechanically rewrite every keyed composite literal the split breaks."
tags: [epic-2]
timestamp: 2026-08-11T18:15:00Z
epic: 2
issue: 05
slug: provider-request-options
size: L
status: open
gh_issue: 133
resource: https://github.com/kern-ia/kern-link/issues/133
depends_on: []
---

# Introduce ProviderRequestOptions as the shared request base, and rewrite the affected StreamOptions literals

## Summary

Upstream split `StreamOptions` in two. The transport/auth/lifecycle half became
`ProviderRequestOptions<TModel>`, and `StreamOptions` extends it:

```ts
export interface ProviderRequestOptions<TModel = Model<Api>> {
	signal?; telemetryContext?; apiKey?; env?;
	onPayload?(payload, model: TModel); onResponse?(response, model: TModel);
	headers?; timeoutMs?; maxRetries?; maxRetryDelayMs?;
}
export interface StreamOptions extends ProviderRequestOptions<Model<Api>> { … }
```

This is the structural change [decision 04](../../../planning/scope/04-core-types-migration.md)
put at the head of the program: the deferred-response entry points in
[issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md)
need the transport knobs *without* the streaming ones, and hand-duplicating fields
across the option types that will extend this base is exactly the drift the next
sync pays for.

This issue is one half of a pre-split
([Epic 0 issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md)):
it lands the struct itself and the compile-fix ripple that follows from it — the
mechanical part, reviewable as a diffstat. The two request-level knobs that arrive in
the same upstream diff (`fetch`, an injectable HTTP client, and `samplingParams`,
arbitrary sampling parameters) plus the deferred-option variants, the `Model` field,
and the two new-field `docs/PORTING.md` entries are
[issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md),
which depends on this one. The numbering is deliberate: 13 lands after this epic's
existing 06–12 rather than renumbering them, so it is a backward-pointing dependency
by number, not by build order.

## Scope

- `ai/options.go` — introduce `ProviderRequestOptions` carrying `APIKey`,
  `Env`, `Headers`, `Timeout`, `MaxRetries`, `MaxRetryDelay`, `OnPayload`, and
  `OnResponse`; embed it in `StreamOptions`, which keeps the streaming-only
  fields (`Temperature`, `MaxTokens`, `Transport`, `CacheRetention`,
  `SessionID`, `WebsocketConnectTimeout`, `Metadata`) plus the per-adapter
  flat-merge fields already documented there.
  - Cancellation stays on `context.Context`, not a `Signal` field — the
    established deviation (`docs/PORTING.md`, "Cancellation").
  - `Fetch` is deliberately **not** added to `ProviderRequestOptions` in this
    issue — it lands in [issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md)
    as a pure field addition to the struct this issue creates. Do not declare
    it here even as a placeholder.
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

## Out of scope

- **Adapter behavior.** Nine adapter packages read `StreamOptions` fields.
  Embedded-field promotion keeps their **selector** reads (`opts.APIKey`)
  compiling unchanged; only the keyed composite literals in `## Scope` need the
  mechanical rewrite. This PR changes no adapter behavior.
- **`Fetch`, `FetchFunction`, `SamplingParams`, `DeferredFetchOptions`,
  `DeferredCancelOptions`, the `Model.SamplingParams` field, the
  `OpenAIToolChoice` / `OpenAIThinkingBudgets` `StreamOptions` fields, and the
  `telemetryContext` / `TModel`-generic `docs/PORTING.md` entries.** All of it is
  [issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md),
  which this issue blocks but does not itself touch.
- **`ai/images`.** Upstream reshaped `ImagesOptions` onto the same base
  (`packages/ai/src/types.ts:293-299` at `936aff00`), but this PR does **not**
  touch `ai/images` — unconditionally. `images.Options`
  (`ai/images/types.go:81-106`) is a standalone struct that hand-duplicates the
  transport fields rather than embedding `ai.StreamOptions`, so splitting
  `StreamOptions` leaves it compiling untouched, and the reshape would simply
  never happen on its own. It is owned by
  [Epic 3 issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  (#218), which is where
  [decision 14](../../../planning/scope/14-images-surface.md)'s Verdict puts the
  images surface. **Note for whoever lands Epic 3 issue 07: after this split, its
  cross-epic line "declares `ProviderRequestOptions` and `FetchFunction`" needs
  to name this issue for the base and issue 13 for `FetchFunction`** — out of
  scope for this PR to edit (Epic 3 is not this issue's epic), flagged here so
  it is not lost.
- `ModelsRequestTransforms` and the `Models`-level option aliases —
  [issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md).
- Idiomatic-Go cleanups of the flat per-adapter option fields. Upstream's shape
  outranks Go idiom in ported files.

## Acceptance criteria / Definition of done

- [ ] `ai.ProviderRequestOptions` exists and is embedded by `ai.StreamOptions`;
      no transport field is declared twice
      (`grep -c "MaxRetryDelay \*time.Duration" ai/options.go` returns `1`).
- [ ] Field *access* is unchanged by promotion: a caller reading `opts.APIKey`
      still compiles after the split, proved by a test.
- [ ] Every keyed `StreamOptions` composite literal identified in `## Scope`
      (**248** lines, re-derived at the PR's base commit) is mechanically
      rewritten to key the embedded struct explicitly:
      `ai.StreamOptions{ProviderRequestOptions: ai.ProviderRequestOptions{...}}`.
      No acceptance criterion in this epic claims the literals survive the
      split unchanged.
- [ ] `grep -n "Fetch\|SamplingParams" ai/options.go` returns nothing inside
      `ProviderRequestOptions` or `StreamOptions` — both are issue 13's, not
      this issue's.
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
- `ai/model.go:54-71` — `Model`. It does **not** gain `SamplingParams` here —
  that is issue 13.
- `ai/apis/simpleopts.go:31` `BuildBaseOptions` — constructs a `StreamOptions`
  field by field; must compile after this PR (it does not yet read `Fetch` or
  `SamplingParams` — that wiring is
  [issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md),
  which depends on issue 13 for those fields to exist).
- `ai/provider.go:72-76, 299-344` — the `Models` quartet passing options
  through; signatures are unchanged here.
- `docs/PORTING.md` — mapping table; this issue does not add rows to it (the
  `telemetryContext` non-port and the `TModel` generic decision are issue 13's,
  since both are driven by the `FetchFunction`/images-reshape chain it owns).
- Upstream: `src/types.ts` at `936aff00`.

## Dependencies

- **Blocked by**: None, but in practice rebase on issues 01–04, which all touch
  `ai/types.go` / `ai/model.go`.
- **Blocks**: [Issue 06](/epic-2-core-types-and-models-contracts/issues/06-simple-options-and-lazy.md),
  [Issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md),
  [Issue 09](/epic-2-core-types-and-models-contracts/issues/09-models-request-transforms.md),
  [Issue 10](/epic-2-core-types-and-models-contracts/issues/10-deferred-response-dispatch.md),
  [Issue 13](/epic-2-core-types-and-models-contracts/issues/13-fetch-sampling-and-deferred-options.md)
  — the struct this issue creates is what issue 13 extends with `Fetch`.

## PR size note

`L` — ~700 changed lines, almost all of it the **248**-site keyed
`StreamOptions` literal rewrite (re-derive at the PR's base commit); the struct
split itself is small. `L` is the ceiling, not the target: a partial rewrite
leaves the tree uncompilable between PRs, so this cannot land in pieces, and it
is sized on line count rather than review difficulty — one shape repeated 248
times, reviewable as a diffstat. If it trends past ~1000 the honest cut is a
stacked pair on one branch — struct plus the non-test literals first, the
`_test.go` literals second — merged together, since neither half is green alone.

---
type: Issue
title: "Port tiered model pricing: ModelCostRates, ModelCostTier, and tier selection in CalculateCost"
description: "Split ModelCost into rates plus optional request-wide tiers and make CalculateCost pick the highest matching input-token tier, without breaking the currently embedded catalog."
tags: [epic-2]
timestamp: 2026-08-11T13:10:00Z
epic: 2
issue: 03
slug: tiered-model-cost
size: M
status: open
gh_issue: 131
resource: https://github.com/kern-ia/kern-link/issues/131
depends_on: []
---

# Port tiered model pricing: ModelCostRates, ModelCostTier, and tier selection in CalculateCost

## Summary

Upstream split the model price sheet into a rates struct plus optional
request-wide tiers, so long-context requests can be billed at a different rate:

```ts
export interface ModelCostRates { input: number; output: number; cacheRead: number; cacheWrite: number; }
export interface ModelCostTier extends ModelCostRates { inputTokensAbove: number; }
export interface ModelCost extends ModelCostRates { tiers?: ModelCostTier[]; }
```

and `calculateCost` now selects a tier before pricing:

```ts
const inputTokens = usage.input + usage.cacheRead + usage.cacheWrite;
let rates: ModelCostRates = model.cost;
let matchedThreshold = -1;
for (const tier of model.cost.tiers ?? []) {
	if (inputTokens > tier.inputTokensAbove && tier.inputTokensAbove > matchedThreshold) {
		rates = tier; matchedThreshold = tier.inputTokensAbove;
	}
}
```

Cost accounting is first-class in kern-link ([SPECS.md](../../../planning/SPECS.md),
"Cross-cutting concerns"), and this is the only pricing behavior change in the
sync, so it ships as its own PR with its own arithmetic tests.

## Scope

- `ai/model.go:36-42` — split `ModelCost` into `ModelCostRates` (the four
  `$/million` fields) embedded by `ModelCost`, which gains
  `Tiers []ModelCostTier` (`json:"tiers,omitempty"`); add `ModelCostTier`
  embedding `ModelCostRates` plus `InputTokensAbove int`
  (`json:"inputTokensAbove"`).
- Keep the wire shape flat: `{"input":…,"output":…,"cacheRead":…,"cacheWrite":…,"tiers":[…]}`.
  Go struct embedding produces exactly that; assert it rather than assume it.
- `ai/cost.go:10-22` `CalculateCost` — compute
  `inputTokens = usage.Input + usage.CacheRead + usage.CacheWrite`, select the
  **highest** matching `InputTokensAbove` strictly below `inputTokens`, and price
  every component (including the Anthropic 1h cache-write rule, which uses
  `rates.Input * 2`) off the selected rates rather than off `model.Cost`
  directly.
- Preserve the existing 1h-cache-write behavior exactly: it is asserted by
  `ai/cost_test.go` and by upstream's `anthropic-cache-write-1h-cost.test.ts`.
- **Rewrite the existing keyed `ModelCost` composite literals.** The same Go
  rule as `StreamOptions` applies: embedding `ModelCostRates` into `ModelCost`
  breaks any composite literal that keys `Input`, `Output`, `CacheRead` or
  `CacheWrite` directly — Go promotes embedded fields for selectors, not for
  composite-literal keys. `grep -rn "ModelCost{" --include=*.go .` returns
  **12** occurrences across **twelve** files today (re-derive at the PR's base
  commit): `ai/apis/anthropic/anthropic_test.go:944`,
  `ai/apis/bedrock/stream_test.go:186`, `ai/apis/codex/stream_test.go:335`,
  `ai/apis/openairesponses/stream_test.go:281`,
  `ai/catalog/fireworks_test.go:46`, `ai/catalog/together_test.go:48`,
  `ai/cost_test.go:16`, `ai/example_stream_test.go:116`,
  `ai/images/openrouter_test.go:43`, `ai/providers/faux/faux.go:242`,
  `ai/providers/openrouter.go:113`, `ai/providers/vercel_ai_gateway.go:117`.
  Eleven of the twelve are keyed and need the mechanical rewrite to
  `ai.ModelCost{ModelCostRates: ai.ModelCostRates{Input: …}}`;
  `ai/providers/faux/faux.go:242` is the zero-value literal `ai.ModelCost{}`,
  which compiles unchanged either way.

## Out of scope

- **Regenerating the catalog** so any model actually carries `tiers` — that is
  [Epic 3](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md). This PR ships
  the Go structs that *read* tiers; the currently embedded
  `ai/catalog/data/models/*.json` has no `tiers` key and must keep loading
  unchanged.
- `ai/catalog` validation rules for tiers (threshold ordering, non-negative
  rates) — epic 3 owns catalog validation.
- `Model.samplingParams`, also new on `Model` at this revision — it belongs with
  the options work in
  [issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md).

## Acceptance criteria / Definition of done

- [ ] `TestCalculateCostUsesHighestMatchingTier` — a model with tiers at
      128_000 and 512_000 prices a request whose `input + cacheRead + cacheWrite`
      exceeds both at the 512_000 rates.
- [ ] `TestCalculateCostIgnoresTierAtExactThreshold` — the comparison is
      strictly greater than `InputTokensAbove`, so `inputTokens ==
      tier.InputTokensAbove` uses the base rates (upstream uses `>`, not `>=`).
- [ ] `TestCalculateCostWithoutTiersMatchesBaseRates` — a model with no tiers
      produces byte-identical results to before this change.
- [ ] `TestCalculateCostTierAppliesToCacheWrite1h` — the 2× base-input rule for
      1h cache writes uses the *selected tier's* input rate.
- [ ] `TestModelCostMarshalsFlat` — `json.Marshal` of a `ModelCost` with no
      tiers emits exactly the four rate keys and no `tiers` key.
- [ ] Every keyed `ai.ModelCost{...}` composite literal listed in `## Scope` is
      rewritten to key the embedded `ModelCostRates` explicitly;
      `ai/providers/faux/faux.go:242`'s zero-value literal is left as-is.
- [ ] `ModelsStoreEntry`'s deep copy (`ai/modelsstore.go`, from
      [issue 07](/epic-2-core-types-and-models-contracts/issues/07-models-store.md))
      and `TestInMemoryModelsStoreReadReturnsCopy` are extended to clone
      `Model.Cost.Tiers`, the slice this PR adds — mutating a tier on a model
      returned by `Read` must not change stored state.
- [ ] Every existing catalog test stays green with the embedded data unchanged:
      `GOTMPDIR=$PWD/.gotmp go test ./ai/catalog/...`.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] `gofmt -l .` prints nothing; `ai/model.go` and `ai/cost.go` keep their
      `// Ports:` headers.
- [ ] Conventional Commit, e.g. `feat(ai): price requests against model cost tiers`.

## Relevant files / areas

- `ai/model.go:36-42` — `ModelCost` today; `:54-71` — `Model`, whose `Cost`
  field is a value (not a pointer), so the embedding must not change
  `Model`'s zero value semantics.
- `ai/cost.go:6-22` — `CalculateCost`.
- `ai/cost_test.go` — existing cost assertions, including cache-write splits.
- `ai/catalog/catalog.go` and `ai/catalog/data/models/*.json` (35 files) — the
  embedded catalog that must keep decoding; `go:embed data` validates at load.
- `ai/estimate.go` — reads usage, not cost; check it does not need a tier-aware
  path before assuming it doesn't.
- `ai/modelsstore.go` — from
  [issue 07](/epic-2-core-types-and-models-contracts/issues/07-models-store.md);
  its `ModelsStoreEntry` deep copy must clone the new `Tiers` slice.
- The eight `_test.go` files plus `ai/providers/faux/faux.go`,
  `ai/providers/openrouter.go` and `ai/providers/vercel_ai_gateway.go` carrying
  `ai.ModelCost{...}` literals, listed in `## Scope`.
- Upstream: `src/types.ts` (`ModelCost*`) and `src/models.ts` (`calculateCost`)
  at `936aff00`.

## Dependencies

- **Blocked by**: None. Independent of issues 01 and 02; it can land in
  parallel, though it also edits `ai/model.go`, so expect to rebase on whichever
  lands first.
- **Blocks**: Nothing inside this epic.
  [Epic 3](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md) needs it before
  a regenerated catalog carrying `tiers` can load.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.

---
type: Issue
title: "Bring ai/catalog validation up to the 0.84.1 schema, and make unknown catalog keys fail loudly"
description: "Extend the per-api compat field map to the 0.84.1 interfaces, add invariants for tiers and the widened thinking levels, and turn silently-dropped catalog keys into a test failure."
tags: [epic-3]
timestamp: 2026-08-09T04:52:00Z
epic: 3
issue: 03
slug: catalog-validation-0-84-1
size: M
status: open
gh_issue: 143
resource: https://github.com/kern-ia/kern-link/issues/143
depends_on: [1]
---

# Bring ai/catalog validation up to the 0.84.1 schema, and make unknown catalog keys fail loudly

## Summary

`ai/catalog`'s "validation at load" is two things: `json.Unmarshal` into
`[]*ai.Model` (`ai/catalog/catalog.go:60-64`), and `TestCatalogCompatMatchesApi`
(`ai/catalog/compat_test.go:20-28`), which sweeps every embedded entry through
`validateCompatForApi`. That sweep is the runtime stand-in for the compile-time
discriminated union upstream gets from `Model<TApi>`.

Both are behind 0.84.1, and the compat one **fails closed**: `compatFieldsByApi`
(`ai/catalog/compat.go:20-38`) lists 18 / 3 / 7 fields for
`openai-completions` / `openai-responses` / `anthropic-messages`, where upstream
at `936aff00` declares 24 / 8 / 9 plus a fourth interface, `BedrockCompat`. Drop
the regenerated catalog in before this lands and every entry carrying a new flag
turns into a test error.

The `json.Unmarshal` half fails **open**, which is worse. A regenerated catalog
key that no Go struct field claims is silently discarded: the capability
disappears, every test stays green, and nothing says so. That is risk 2's
failure mode ([decision 21](../../../planning/scope/21-risks-and-assumptions.md))
applied to data instead of error strings. This PR closes it with a
`DisallowUnknownFields` sweep over the embedded tree.

**Ordering note.** [Decision 05](../../../planning/scope/05-catalog-schema-and-tooling.md)
writes the epic's order as "update `tools/export-catalog` → regenerate → update
`ai/catalog` validation". This issue is deliberately sequenced *before*
regeneration instead, because the load-bearing principle behind that order is
**code before data** — "the code must precede the data or the regenerated tree
fails validation at load" — and this validation code is exactly such code.
Landing it after the data would mean merging a red PR. Same principle, one
position earlier.

## Scope

- `ai/catalog/compat.go` — bring `compatFieldsByApi` to the `936aff00`
  interfaces, using the Go field names epic 2
  [issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  ships on `ai.Compat`:
  - `openai-completions` adds `SupportsFinishReason`, `ChatTemplateArgs`,
    `SupportsThinkingTokenBudget`, `SupportsOpenAIGrammarTools`,
    `DeferredToolsMode`, `SessionAffinityFormat`.
  - `openai-responses` adds `SupportsStrictMode`, `SupportsOpenAIGrammarTools`,
    `SupportsAdditionalTools`, `SupportsToolSearch`,
    `SupportsExplicitPromptCacheMode`, and `SessionAffinityFormat` in place of
    the retired `SendSessionIDHeader`.
  - `anthropic-messages` adds `SupportsStrictTools`, `SupportsToolReferences`.
  - Add `ai.ApiBedrockConverseStream` → `{SupportsStrictMode}` (upstream's new
    `BedrockCompat`). It currently maps to "accepts no compat block at all",
    so a regenerated Bedrock entry with `compat` would be rejected.
  - Add `ai.ApiAzureOpenAIResponses` and `ai.ApiOpenAICodexResponses`, both
    sharing the `openai-responses` field set — upstream's `Model<TApi>` at
    `936aff00` widened the responses compat arm to cover all three
    (`src/types.ts:814-818`). Share one set rather than copying it three times.
- Add a strict-decode sweep: for every `data/models/*.json` and
  `data/images/*.json` file, decode with `json.Decoder.DisallowUnknownFields`
  and fail naming the file, the model id, and the unclaimed key. This is a test,
  not a change to `load()` — `load()` stays lenient so a future catalog cannot
  panic a consumer's process at init.
- Add catalog invariants for the new schema, as tests over the embedded tree:
  - `ModelCost.Tiers`, when present, has strictly increasing distinct
    `InputTokensAbove` values, all `> 0`, and non-negative rates
    (structs from epic 2
    [issue 03](/epic-2-core-types-and-models-contracts/issues/03-tiered-model-cost.md)).
  - Every `ThinkingLevelMap` key is a known `ai.ModelThinkingLevel`, including
    the new `"max"` (epic 2
    [issue 01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md)).
  - Every entry's `Api` is one of the nine `ai.Api` constants — an unknown api
    string today decodes fine and then fails much later, at adapter dispatch.
  - Every entry's `Provider` matches the filename it came from, and its `ID` is
    unique within that file.
- Extend the same sweep to the image catalog via `catalog.ImagesData` (raw
  bytes, so `ai/catalog` still never imports `ai/images` — see
  `ai/catalog/images.go:1-8`).

## Out of scope

- Adding the `ai.Compat` fields themselves — epic 2
  [issue 04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  ships them; this PR only teaches the catalog which api owns which.
- `ModelCostRates`/`ModelCostTier` and tier-aware pricing — epic 2
  [issue 03](/epic-2-core-types-and-models-contracts/issues/03-tiered-model-cost.md).
- **Regenerating any catalog data.** `ai/catalog/data/**` is untouched here; the
  new invariants must pass against the *currently embedded* tree, which is what
  makes this issue independently mergeable.
- Adapter behavior for any new compat flag — epics 4 and 5.
- Making `load()` strict or making a bad catalog a load-time error for
  consumers. The panic-on-malformed-embed contract in
  `ai/catalog/catalog.go:37-40` stays as it is.

## Acceptance criteria / Definition of done

- [ ] `TestCatalogCompatMatchesApi` still passes against the unchanged embedded
      catalog.
- [ ] `TestValidateCompatForApi_AcceptsBedrockStrictMode` — an
      `ai.ApiBedrockConverseStream` entry with only `SupportsStrictMode` set is
      accepted; one with, say, `SupportsTemperature` set is rejected.
- [ ] `TestValidateCompatForApi_ResponsesArmCoversAzureAndCodex` — the same
      responses-only flag is accepted on all three of `openai-responses`,
      `azure-openai-responses`, `openai-codex-responses`.
- [ ] `TestValidateCompatForApi_RejectsFieldsNotOwnedByApi` grows a case for at
      least one newly added flag per api arm.
- [ ] `TestCatalogRejectsUnknownKeys` — every embedded `data/**/*.json` decodes
      with `DisallowUnknownFields`; the failure message names file, model id and
      key. Verify it actually fails by adding a bogus key to a temp copy of one
      file in the test, not by trusting the happy path.
- [ ] `TestCatalogTierThresholdsStrictlyIncrease` — passes vacuously against
      today's tier-free catalog, and is proven to fire against a hand-built
      `ai.ModelCost` with out-of-order thresholds.
- [ ] `TestCatalogThinkingLevelKeysKnown` accepts `"max"` and rejects an
      invented level.
- [ ] `TestCatalogApiIsKnown` and `TestCatalogProviderMatchesFilename` pass over
      all 35 currently embedded files.
- [ ] `git diff --stat ai/catalog/data` is empty in this PR.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./ai/catalog/...` passes; then
      `GOTMPDIR=$PWD/.gotmp go test ./...`; CI green (`go test ./... -race -v`,
      `bash upstream/sync_test.sh`, `golangci-lint` v2.12.2).
- [ ] `gofmt -l .` prints nothing; `ai/catalog/compat.go` keeps its `// Ports:`
      header, updated if the ported symbol list changed. New test files carry a
      `// Ports:` header only where they port an upstream test — the invariant
      sweeps are original code and take none
      ([CONVENTIONS.md](../../../planning/CONVENTIONS.md), "Every ported file
      carries a provenance header").
- [ ] Discrete named test functions, standard library only, no `t.Parallel()`,
      `got`/`want` phrasing — matching the repo's testing conventions.
- [ ] Conventional Commit, e.g.
      `feat(catalog): validate the 0.84.1 catalog schema and reject unknown keys`.

## Relevant files / areas

- `ai/catalog/compat.go:20-38` — `compatFieldsByApi`; `:48-66`
  `validateCompatForApi` and its reflect-based field walk.
- `ai/catalog/compat_test.go:1-12` — the doc comment explaining why this runtime
  check exists at all; `:20-28` the catalog-wide sweep; `:30+` the table-driven
  case list (one of the repo's 19 sanctioned table tests).
- `ai/catalog/catalog.go:41-73` `load()`, `:29-30` the `go:embed data` root.
- `ai/catalog/images.go:22-49` `loadImages()`, `:61-67` `ImagesData`.
- `ai/catalog/catalog_test.go`, `images_test.go` — existing shape of the
  catalog-level assertions.
- `ai/model.go:36-42` `ModelCost`, `:53-70` `Model`, `:95-150` `ThinkingFormat`
  and `Compat` — all of which epic 2 changes underneath this issue.
- `ai/types.go:9-19` — the nine `ai.Api` constants.
- Upstream at `936aff00`: `src/types.ts` — `OpenAICompletionsCompat`,
  `OpenAIResponsesCompat`, `AnthropicMessagesCompat`, `BedrockCompat`, and
  `Model<TApi>`'s `compat` conditional type at `:813-820`.

## Dependencies

- **Blocked by**: [Issue 01](/epic-3-catalog-schema-and-export-tooling/issues/01-export-catalog-spike.md)
  only for sequencing; there is no code dependency on it. The real prerequisite
  is outside this epic: epic 2 issues
  [01](/epic-2-core-types-and-models-contracts/issues/01-widen-stopreason-and-thinkinglevel.md),
  [03](/epic-2-core-types-and-models-contracts/issues/03-tiered-model-cost.md)
  and [04](/epic-2-core-types-and-models-contracts/issues/04-compat-flags-and-bedrock-compat.md)
  must be merged, since this PR references fields they add. Epic 2 is already
  a declared dependency of this epic.
- **Blocks**: [Issue 04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md)
  — the regenerated tree carries the new compat keys and cannot go green before
  this lands.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.

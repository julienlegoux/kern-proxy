---
type: Epic
title: "Catalog schema and export tooling"
description: "Price and update tools/export-catalog against the frozen upstream, regenerate the model and image catalogs, and re-validate them — including the images surface that rides on the same tooling."
tags: [epic]
timestamp: 2026-08-11T20:10:00Z
epic: 3
slug: catalog-schema-and-export-tooling
status: open
gh_issue: 120
milestone: 20
resource: https://github.com/kern-ia/kern-link/issues/120
source: docs/planning/SCOPE.md#milestone-3-catalog-schema-and-export-tooling
---

# Epic 3: Catalog schema and export tooling

## Goal

`sync.sh` buckets these files as "re-run `tools/export-catalog` only". That
label is wrong for this sync, and treating it as right is the main trap of the
whole program: five upstream generator scripts changed
(`generate-models.ts`, `check-model-data.ts`, `generate-image-models.ts`,
`model-data.ts`, `models-dev-reasoning-options.ts`), so the exporter may not run
against `936aff00` at all.

## Scope

- **First issue is a tooling spike**: confirm `tools/export-catalog` runs green
  against upstream at `936aff00`. Price this before planning the rest of the
  epic.
- Then, in this order: update `tools/export-catalog` → update `ai/catalog`
  validation → regenerate all catalog JSON → run the catalog tests. Code before
  data — the Go structs that read the catalog ship in epic 2, the validation that
  accepts the 0.84.1 schema ships here, and the regenerated tree fails validation
  at load otherwise. This swaps the middle two steps of
  [decision 05](../../planning/scope/05-catalog-schema-and-tooling.md) and
  `SCOPE.md:175-178`, which write regeneration before validation: the same "code
  before data" sentence is what puts the validation ahead of the data it
  validates, and landing it afterwards would mean merging a red PR. Amended to
  follow
  [issue 03](/epic-3-catalog-schema-and-export-tooling/issues/03-catalog-validation-0-84-1.md),
  which sequenced it that way and said why.
- **Unknown catalog keys fail loudly.** `json.Unmarshal` discards a regenerated
  key that no Go struct field claims — the capability disappears and every test
  stays green, which is this epic's headline risk applied to data rather than to
  error strings. A **strict-decode guard** over the embedded tree
  (`json.Decoder.DisallowUnknownFields`, as a test; `load()` stays lenient) is
  authorized here by name and owned exactly once, by
  [issue 03](/epic-3-catalog-schema-and-export-tooling/issues/03-catalog-validation-0-84-1.md),
  for both `data/models/**` and `data/images/**`. Any *other* catalog invariant
  is optional hardening: it does not belong on the path that gates regeneration.
- 39 provider files under `ai/catalog/data/models/` after regeneration, up from
  35 today, plus the regenerated image catalog. The **41** `*.models.ts` that
  decision 05 and `SCOPE.md:179` count is a different number, and the two are
  reconciled rather than averaged: 41 is how many
  `packages/ai/src/providers/*.models.ts` files the sync diff *touches*
  (`git -C <upstream-clone> diff --name-only 244f1dea..936aff00 -- 'packages/ai/src/providers/*.models.ts' | wc -l`),
  while 39 is how many provider keys `packages/ai/src/models.generated.ts`
  exports in `MODELS` *at* `936aff00` — and it is the second that
  `ai/catalog/data/models/` mirrors. Today's 35 is
  `ls ai/catalog/data/models/*.json | wc -l`. Both upstream numbers need the
  `936aff00` checkout the spike produces; neither is derivable from this tree.
- **Images surface rides along**: `images-models.ts`, the OpenRouter images
  adapter, and the regenerated image catalog. Same tooling, so a separate epic
  would touch it twice. It also carries the two `ai/images` consequences upstream
  forces and no other epic accepts — `ImagesOptions` moving onto
  `ProviderRequestOptions` (with its `fetch` override) and the images
  `getAuth(providerId | model, overrides)` overload — owned by
  [issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md).
- **The two images issues sit outside the catalog chain**, so the order above
  does not govern them.
  [Issue 06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md)
  touches no catalog data and no export tooling and can land at any point.
  [Issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  depends on issue 06 and takes cross-epic edges on Epic 6 for the shared auth
  shapes, so it is this epic's tail and lands **after Epic 6** — recorded here
  rather than hidden, because the alternative was leaving the images half of the
  auth restructure with no owner at all.

## Out of scope

- The core Go structs that read the catalog — epic 2 ships those.
- Provider bindings and OAuth flows whose catalog entries other epics add
  (epics 6, 7, 8 each carry their own entries).
- Coverage tooling or thresholds.

## Acceptance criteria

1. The spike's finding is recorded before the rest of the epic is planned:
   whether `tools/export-catalog` runs against `936aff00` as-is, and what it
   costs if not.
2. `tools/export-catalog` runs green against upstream at `936aff00`.
3. All catalog JSON is regenerated from that revision, and `ai/catalog`
   validation accepts it at load.
4. The catalog tests pass, including the image catalog.
5. **Any change to the OpenRouter images adapter gets an offline `httptest`
   case.** CI never sets `OPENROUTER_API_KEY`, so a live-only test proves nothing
   there. The *justification* this criterion was written on has gone stale and is
   corrected here rather than re-argued per issue:
   [decision 14](../../planning/scope/14-images-surface.md) says `ai/images` has
   "exactly one live smoke test", but `ai/images/openrouter_test.go` today
   carries six offline tests — four of them driving an `httptest` server
   (`:18`, `:80`, `:122`, `:142`) and two needing no server (`:108`, `:165`) —
   beside `TestGenerateImagesOpenRouter_LiveSmoke` (`:171`). The criterion stands;
   only its premise moved.
   [Issue 06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md)
   records the stale decision text as a drift record rather than editing
   `docs/planning/` inside an implementation PR.
6. CI green: `go test ./... -race -v`, `bash upstream/sync_test.sh`,
   `golangci-lint` v2.12.2.

## Dependencies

- [Epic 2: Core types and Models contracts](/epic-2-core-types-and-models-contracts/EPIC_2.md)
  — the catalog-reading structs must exist before the regenerated tree lands.
- [Epic 6: Auth core and env API key bindings](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md)
  — **tail issue only**. Issues 01–06 are independent of it;
  [issue 07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  consumes the auth shapes Epic 6 issues 03 and 06 settle, so it lands after
  Epic 6 while the rest of this epic runs in its own window.

## Context

- [Technical specs](../../planning/SPECS.md)
- [Conventions](../../planning/CONVENTIONS.md)
- [Upstream sync scope](../../planning/SCOPE.md)
- [Decision 05 — catalog schema and tooling](../../planning/scope/05-catalog-schema-and-tooling.md)
- [Decision 14 — images surface](../../planning/scope/14-images-surface.md)
- [Decision 21 — risks and assumptions](../../planning/scope/21-risks-and-assumptions.md)

## Notes

- Risk owned here, and the reason the spike is issue one: **`tools/export-catalog`
  may not run against `936aff00` at all.** The spike prices it before the rest
  is planned.
- Project-wide, not this epic's own boundary: no new direct dependencies, tests
  offline and stdlib-only, `// Ports:` headers on ported files, `develop` as the
  integration trunk, and the race detector's verdict only from CI.
- The upstream target is frozen at `936aff00`; regeneration is against that
  revision and no other.
- **`ai/apis/internal/httpretry` is not relocated in this program.**
  [Epic 0](/epic-0-plan-remediation/EPIC_0.md) decided it: moving a shared
  internal package while epics 4 and 5 rewrite its four consumers buys nothing
  that waiting does not, and
  [CONVENTIONS.md](../../planning/CONVENTIONS.md) (`:22-26`) names its current
  home as part of the decided package layout.
  [Issue 06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md)
  reaches the loop from `ai/images` through a thin exported shim in the existing
  `ai/apis` package instead, leaving the four text adapters and their retry tests
  untouched. Whether upstream's `provider-retry.ts` supersedes the helper at all
  stays with [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md), which
  already owns that question.
- **Generated catalog output does not count toward the PR size bands.**
  [Epic 0](/epic-0-plan-remediation/EPIC_0.md) decided it: the hand-written rule
  is the one that measures reviewability. So
  [issue 04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md)
  and
  [issue 05](/epic-3-catalog-schema-and-export-tooling/issues/05-regenerate-image-catalog.md)
  are banded on their hand-written lines alone — one test plus a doc paragraph,
  and one test file — while the regenerated JSON under `ai/catalog/data/**` is
  reviewed through the shape checks in their acceptance criteria rather than
  counted.
- Nothing in this epic makes an edit to `docs/planning/` a merge-blocking
  acceptance criterion. Where an implementation here proves a planning document
  wrong — decision 14's smoke-test claim, decision 05's ordering and its byte-
  stability promise — the issue writes a drift record under
  `docs/epics/epic-3-catalog-schema-and-export-tooling/drift/`, which `close-epic`
  promotes to `docs/planning/DRIFT.md` for the user to disposition.
- **Branch, base-branch and status-commit conventions**
  (`docs/planning/CONVENTIONS.md:213-224`). Feature branches are named
  `issue-<NN>-<slug>` (`:216-218`). PRs target `develop` and merge with a merge
  commit — no rebase, no squash (`:213-219`). Each status transition gets its own
  `docs(epics): …` commit, separate from the implementation commit (`:221-224`).
  `Closes #N` will not auto-close the issue, because PRs merge into `develop`
  rather than the repo's default branch — the explicit close at reconcile is the
  normal route, not a fallback.

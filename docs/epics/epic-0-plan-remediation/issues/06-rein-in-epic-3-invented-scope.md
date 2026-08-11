---
type: Issue
title: "Rein in Epic 3's invented scope, route its SPECS.md edits through drift, and reconcile EPIC_3.md with its issues"
description: "Scope Epic 3 issue 06 to the images adapter instead of relocating a shared internal package, drop the two invented tool surfaces, convert two un-triaged planning-doc edits to drift records, and fix the epic's stated order."
tags: [epic-0]
timestamp: 2026-08-11T20:00:00Z
epic: 0
issue: 06
slug: rein-in-epic-3-invented-scope
size: M
status: done
gh_issue: 204
gh_pr: 223
resource: https://github.com/kern-ia/kern-link/issues/204
depends_on: ["02"]
---

# Rein in Epic 3's invented scope, route its SPECS.md edits through drift, and reconcile EPIC_3.md with its issues

## Summary

Epic 3 is a catalog epic whose issues quietly took on a cross-cutting refactor of the
retry layer, two new configuration surfaces on a tool the epic only asked to keep
working, a six-sweep invariant battery on the critical path, and a definition of done
that includes rewriting four sibling issues. Two of its issues also make an un-triaged
`docs/planning/SPECS.md` edit a merge-blocking acceptance criterion — while a third
issue in the same epic does the same thing correctly, as a drift record.

The `httpretry` relocation is the one with teeth: it rewrites call sites in
`ai/apis/anthropic`, `ai/apis/azure`, `ai/apis/bedrock` and `ai/apis/codex` — the four
packages Epics 4 and 5 rewrite wholesale — while contradicting
`docs/planning/CONVENTIONS.md:22-26`, which names the package's decided home.

Findings repaired: REPORT_3 P1 (httpretry), P2 (SPECS edits, the 04/05 generator-run
guard), P3 ×5 (`MODEL_CATALOG_DIR`, the invariant battery, issue 01 rewriting its
successors, the duplicated strict-decode test, the epic's "strictly in this order", the
41-vs-39 count). Epic 0 scope items 4, 5, 9, 11 and 14; acceptance criteria 4, 5, 9, 11
and 13.

## Scope

**1. `ai/apis/internal/httpretry` stays where it is.** Epic 0 has already decided this
(`EPIC_0.md`, `## Notes`): the package is not relocated in this program, and the
reconciliation with upstream's `provider-retry.ts` stays with
[Epic 9 issue 02](/epic-9-classifier-audit-and-release/issues/02-provider-retry-vs-httpretry.md),
which already owns it. Rewrite `epic-3 issue 06:61-69` to scope the change to the
OpenRouter images adapter — expose the retry loop through a thin exported shim rather
than moving the package — and delete `:129-132` (the four text adapters' retry tests)
and `:135-138` (the `SPECS.md` edit) with it. Record the decision and its reason in one
line so the next reader does not re-open it.

**2. The two invented tool surfaces (REPORT_3 P3 ×2).**
`epic-3 issue 02:56-58` adds a `MODEL_CATALOG_DIR` env var **and** a positional
argument to `tools/export-catalog`, where the existing `UPSTREAM_CLONE_DIR` seam
(`tools/export-catalog/export-catalog.ts:23`) already derives the default. Drop both;
keep the derived default and the actionable error message at `:63-65`.
`epic-3 issue 03:71-90` adds six invariant sweeps and seven new tests beyond the
compat-map work `EPIC_3.md:54-56` asks for, and issue 03 blocks issue 04 — optional
hardening gating the epic's required regeneration. Keep the strict-decode guard (it
directly serves the "silent capability drop" risk `EPIC_3.md`'s Notes are about) by
adding one authorizing line to `EPIC_3.md`'s `## Scope`; move the remaining five sweeps
into a follow-up bullet that does not gate issue 04, or drop them.

**3. Issue 01 stops rewriting its successors (REPORT_3 P3).**
`epic-3 issue 01:82-83` and its criterion at `:118-119` make editing issues 02–05 (and
their GitHub bodies) part of the spike's own definition of done, unbounded — no
criterion covers refreshed `timestamp`s, index reconciliation, or what happens if the
finding invalidates an issue outright. Narrow it to: record every correction in the
drift record and post it on #120, and open a follow-up to re-plan issues 02–05 if the
finding invalidates them.

**4. Route the two planning-doc edits through drift (REPORT_3 P2).**
`epic-3 issue 02:78-81` and `epic-3 issue 06:135-138` make un-triaged
`docs/planning/SPECS.md` edits merge-blocking acceptance criteria.
`epic-3 issue 01:75-81` already does this correctly — a drift record under
`docs/epics/epic-3-catalog-schema-and-export-tooling/drift/`, promoted to
`docs/planning/DRIFT.md` by `close-epic`, dispositioned by the user. Convert both to
that pattern. (Item 1 deletes issue 06's edit outright; if any part of it survives the
rescoping, it goes through drift too.)

**5. Reconcile `EPIC_3.md` with its issues (REPORT_3 P3 ×2).** `EPIC_3.md:32-35` says
"strictly in this order: update `tools/export-catalog` → regenerate all catalog JSON →
update `ai/catalog` validation → run the catalog tests"; the issues sequence validation
*before* regeneration, correctly — the epic's own "code before data" sentence at `:34-35`
demands it, and landing validation after the data means merging a red PR. Amend the
epic to the order its issues use, keeping the justifying sentence. Separately, the epic
counts **41** `*.models.ts` (`:36`) where the issues consistently count **39**
(`02:25`, `04:28-30`, `:76-79`); `ai/catalog/data/models/` holds 35 files today, so the
issues' 35 → 39 arithmetic is right. Add the clause that reconciles them (files touched
in the upstream diff vs providers in `MODELS` at `936aff00`), re-derived, not guessed.

**6. De-duplicate the strict-decode test and guard the shared generator run
(REPORT_3 P2 + P3).** `epic-3 issue 03:88-90`, `:119-122` and `epic-3 issue 05:90-93`
each promise a `DisallowUnknownFields` sweep over
`ai/catalog/data/images/openrouter.json` under two different test names. Leave the
raw-bytes strict decode in issue 03 and reword issue 05's criterion to "reuses issue
03's helper; adds no second sweep". And since one tool invocation writes both catalog
trees (`05:44-48`) while neither issue declares an order, add to issue 04 the criterion
`git diff --stat ai/catalog/data/images` is empty in this PR — the discipline issue 02
already demonstrates at `:110` — and state in both that the pair share one generator run
whose date and SHA are recorded identically in both PR bodies.

## Out of scope

- **Implementing Epic 3.** No Go code, no catalog regeneration, no tool change.
- **The `ImagesOptions` reshape and the `getAuth` orphan** —
  [issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md) owns
  both, and creates the new Epic 3 issue 07 they land in. This issue must be written
  against a seven-issue Epic 3.
- **Editing `docs/planning/SPECS.md` or `CONVENTIONS.md`.** Item 4 removes planning
  edits from Epic 3's PRs; it does not perform them. `CONVENTIONS.md:22-26` keeps naming
  `ai/apis/internal/httpretry` because, after item 1, that is still where the package
  lives.
- **The size-rule question** (`epic-3 issue 04` L vs `issue 05` S, and the "generated
  catalog output does not count toward the band" rule Epic 0 decided) —
  [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md) states the
  rule in `EPIC_3.md`'s `## Notes` and re-labels both issues in the same pass.
- **Epic 9 issue 02's `provider-retry.ts` reconciliation.** Item 1 points at it; it does
  not change it. [Issue 12](/epic-0-plan-remediation/issues/12-repair-the-epic-9-issue-set.md)
  owns Epic 9's own repairs.

## Acceptance criteria / Definition of done

- [ ] `git grep -n 'httpretry' docs/epics/epic-3-catalog-schema-and-export-tooling/`
      shows no relocation: the package's decided home in
      `docs/planning/CONVENTIONS.md:22-26` is unchanged, and Epic 3 issue 06 touches the
      OpenRouter images adapter only.
- [ ] `git grep -n 'MODEL_CATALOG_DIR' docs/epics/` returns nothing.
- [ ] `EPIC_3.md`'s `## Scope` authorizes the strict-decode guard by name; every
      remaining invariant sweep in `epic-3 issue 03` either has that authorization or
      has moved off issue 04's critical path.
- [ ] No Epic 3 issue makes an edit to `docs/planning/` a merge-blocking acceptance
      criterion: `git grep -n 'docs/planning/SPECS.md' docs/epics/epic-3-*/issues/`
      returns only drift-record wording or read-only citations.
- [ ] `epic-3 issue 01`'s definition of done no longer includes editing sibling issue
      files or their GitHub bodies.
- [ ] `EPIC_3.md:32-35` states the order the issues implement, and the 41-vs-39
      discrepancy is reconciled with the command that produced each number.
- [ ] Exactly one Epic 3 issue owns the raw-bytes strict decode over
      `ai/catalog/data/images/openrouter.json`, and issue 04 carries the
      `git diff --stat ai/catalog/data/images` guard.
- [ ] `depends_on` and the `## Dependencies` prose agree in both directions across all
      seven Epic 3 issues, including the new issue 07.
- [ ] `timestamp` refreshed on every edited file; GitHub bodies of #141–#146 (and #120
      where the epic changed) updated to match.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-rein-in-epic-3-invented-scope`, PR targets `develop`, merge
      commit. Commit e.g. `docs(epics): scope Epic 3 to what EPIC_3.md authorizes`.

## Relevant files / areas

- `docs/epics/epic-3-catalog-schema-and-export-tooling/EPIC_3.md:32-35`, `:36`,
  `:37-39`, `:54-59`.
- `.../issues/01-export-catalog-spike.md:75-81`, `:82-83`, `:118-119`.
- `.../issues/02-export-catalog-json-input.md:25`, `:56-58`, `:63-65`, `:78-81`, `:110`.
- `.../issues/03-catalog-validation-0-84-1.md:52-70`, `:71-90`, `:109-129`.
- `.../issues/04-regenerate-model-catalog.md:28-30`, `:64-65`, `:76-103`.
- `.../issues/05-regenerate-image-catalog.md:44-48`, `:90-93`.
- `.../issues/06-images-adapter-retry.md:61-69`, `:83-92`, `:129-132`, `:135-138`.
- Read-only: `docs/planning/CONVENTIONS.md:22-26`, `docs/planning/SPECS.md:73`,
  `tools/export-catalog/export-catalog.ts:23`, `ai/catalog/compat.go:20-38`,
  `ai/catalog/data/models/` (35 files), `ai/catalog/data/images/openrouter.json`
  (712 lines), `ai/images/openrouter.go:239`.

## Dependencies

- **Blocked by**: [Issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md)
  — it adds Epic 3 issue 07 and edits issues 05 and 06, which this issue also rewrites.
- **Blocks**: [Issue 12](/epic-0-plan-remediation/issues/12-repair-the-epic-9-issue-set.md)
  — item 1 hands the `provider-retry.ts` reconciliation to Epic 9 issue 02, and issue 12
  must be written against that hand-off; and
  [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).

## PR size note

`M` — ~250–450 changed lines across eight files, concentrated in Epic 3 issues 02, 03
and 06. Split past ~500. The clean cut: items 1–3 (the invented scope) first, items 4–6
(drift routing, the epic amendment, the de-duplication) second.

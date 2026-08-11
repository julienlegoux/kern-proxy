---
type: Issue
title: "Regenerate the embedded model catalog from upstream 936aff00 (35 to 39 provider files)"
description: "Run the reworked export tool against the frozen upstream revision, replace ai/catalog/data/models wholesale, and prove the regenerated tree loads and validates."
tags: [epic-3]
timestamp: 2026-08-11T18:15:00Z
epic: 3
issue: 04
slug: regenerate-model-catalog
size: S
status: open
gh_issue: 144
resource: https://github.com/kern-ia/kern-link/issues/144
depends_on: [2, 3]
---

# Regenerate the embedded model catalog from upstream 936aff00 (35 to 39 provider files)

## Summary

The data half of the epic. With the export tool fixed
([issue 02](/epic-3-catalog-schema-and-export-tooling/issues/02-export-catalog-json-input.md))
and the validation ahead of it
([issue 03](/epic-3-catalog-schema-and-export-tooling/issues/03-catalog-validation-0-84-1.md)),
this PR replaces `ai/catalog/data/models/**` with the catalog upstream declares
at `936aff00`.

Four provider files are new — `baseten`, `qwen-token-plan`,
`qwen-token-plan-cn`, `qwen-token-plan-individual` (35 → 39, none removed;
verified against `models.generated.ts`'s `MODELS` keys at both revisions). Their
**bindings** are not this issue's business:
[decision 10](../../../planning/scope/10-new-provider-bindings.md) puts those in
the auth epic and says in as many words that "their catalog files come along
free with [decision 05]'s regeneration". So the catalog gains four providers
that `ai/providers` does not yet serve, on purpose, for one epic.

Everything else in the diff — per-model additions, removals, price and
context-window movement, new `thinkingLevelMap` entries, new compat flags — is
generated. Review it as a shape check, not line by line.

## Scope

- Regenerate `ai/catalog/data/models/*.json` against upstream `936aff00`, using
  the procedure issue 02 documents in `docs/PORTING.md`. No hand-editing of
  generated JSON, ever — a needed correction is a tooling change in issue 02, not
  a patch here.
- Commit the resulting 39 files (the tool's `clearDir` removes any provider that
  disappeared; none is expected).
- Record the exact inputs in the PR body so the run is reproducible-ish despite
  the network dependency: upstream SHA, the date of the generator run, and the
  upstream generator command used. **This PR and
  [issue 05](/epic-3-catalog-schema-and-export-tooling/issues/05-regenerate-image-catalog.md)
  share one generator run** — the tool writes both trees in a single invocation —
  so that SHA, date and command are recorded *identically* in both PR bodies. Two
  different dates in the two bodies means the trees came from two fetches of live
  third-party data and the pair is inconsistent. Neither PR waits on the other —
  each commits only its own half of the run, and whichever lands second must not
  need a second invocation to do it.
- Add a test asserting the catalog's provider set matches the expected 39, so a
  future regeneration that silently drops a provider fails instead of shrinking
  quietly.
- Note the four catalog-only providers in `docs/PORTING.md`'s deviations
  section, with a pointer to the auth epic that binds them — a catalog entry with
  no binding is otherwise indistinguishable from an oversight.

## Out of scope

- **Provider bindings** for `baseten` and the three `qwen-token-plan` variants —
  [Epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md), per
  [decision 10](../../../planning/scope/10-new-provider-bindings.md).
- The image catalog —
  [issue 05](/epic-3-catalog-schema-and-export-tooling/issues/05-regenerate-image-catalog.md).
- Any change to `tools/export-catalog` (issue 02) or to `ai/catalog`'s Go code
  (issue 03). If the regenerated tree needs either to change, that is a signal
  the earlier issue was incomplete — fix it there and rebase, rather than
  widening this PR.
- Adapter behavior for any capability the new entries advertise — epics 4 and 5.
- `upstream/UPSTREAM.lock`, which keeps reading `244f1dea` until
  [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md).

## Acceptance criteria / Definition of done

- [ ] `ai/catalog/data/models/` contains exactly 39 files: the 35 present today
      plus `baseten.json`, `qwen-token-plan.json`, `qwen-token-plan-cn.json`,
      `qwen-token-plan-individual.json`.
- [ ] `TestCatalogProviderSetIsExpected` — `catalog.Providers()` equals that
      39-entry list exactly, failing with the set difference (missing / extra)
      rather than a length mismatch.
- [ ] `TestCatalogRejectsUnknownKeys` — issue 03's strict-decode sweep — passes
      against the **regenerated** tree, not just the old one. That is the whole
      validation surface issue 03 ships; the five further invariants it once
      carried are deferred to a follow-up (`issue 03`'s `## Out of scope`) and
      are not gates on this PR.
- [ ] `git diff --stat ai/catalog/data/images` is empty in this PR. One tool
      invocation writes both catalog trees, so the images half must be split off
      and left to
      [issue 05](/epic-3-catalog-schema-and-export-tooling/issues/05-regenerate-image-catalog.md)
      — the same discipline issue 02 applies to `ai/catalog/data` as a whole.
- [ ] `TestCatalogCompatMatchesApi` passes over all 39 files.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./ai/catalog/... ./ai/providers/...` passes —
      `ai/providers` in particular, since `catalog.BuiltinModels("<id>")` feeds
      all 35 existing bindings and a renamed or dropped model id there is a
      runtime break, not a catalog cosmetic.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] `cmd/pi-ai list` runs and prints the catalog without error — it is a pure
      function of the embedded data and is the cheapest end-to-end proof the
      tree loads.
- [ ] The PR body records the upstream SHA, the generator command, and the date
      of the run.
- [ ] Every model id referenced by name in Go code or tests still exists in the
      regenerated catalog — grep for hard-coded ids before merging.
      `ai/catalog/catalog_test.go:47-58` pins `anthropic/claude-opus-4-5` and
      its display name, and it will not be the only one.
- [ ] Conventional Commit, e.g.
      `feat(catalog): regenerate the model catalog from upstream 936aff00`.

## Relevant files / areas

- `ai/catalog/data/models/*.json` — the 35 files replaced, 4 added. This is the
  bulk of the diff.
- `ai/catalog/catalog.go:41-73` — `load()`, which decodes each file into
  `[]*ai.Model`; `:84-89` `BuiltinModels`, whose doc promises "in the order
  upstream's generated catalog defines them" (upstream's JSON output is sorted
  by model id — confirm the promise still describes reality, and reword it if
  not).
- `ai/catalog/catalog_test.go:47-58` — the pinned `claude-opus-4-5` assertion.
- `ai/providers/*.go` (35 bindings) — every one calls
  `catalog.BuiltinModels("<id>")`; a provider whose file changed shape shows up
  here first.
- `cmd/pi-ai` — `list` renders the catalog.
- `docs/PORTING.md` — deviations section, for the four unbound providers.
- Upstream at `936aff00`: `packages/ai/src/models.generated.ts` (39 `MODELS`
  keys).

## Dependencies

- **Blocked by**: [Issue 02](/epic-3-catalog-schema-and-export-tooling/issues/02-export-catalog-json-input.md)
  (the tool must be able to run at all) and
  [Issue 03](/epic-3-catalog-schema-and-export-tooling/issues/03-catalog-validation-0-84-1.md)
  (the new compat keys must be accepted before the data carrying them lands).
- **Blocks**: Nothing inside this epic.
  [Epic 6](/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md)'s four new
  bindings need the catalog entries this PR ships.

## PR size note

`S` — ~60 hand-written lines: one test (`TestCatalogProviderSetIsExpected`) and
one `docs/PORTING.md` paragraph for the four catalog-only providers.
**Re-labelled from `L`**: generated catalog output does not count toward the
bands ([Epic 0](/epic-0-plan-remediation/EPIC_0.md), `## Notes`, restated in
[EPIC_3.md](/epic-3-catalog-schema-and-export-tooling/EPIC_3.md)), and the
regenerated `ai/catalog/data/models/**` — thousands of generated JSON lines over
39 files — is reviewed as a shape check. Split past ~200 hand-written lines.

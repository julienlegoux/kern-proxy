---
type: Issue
title: "Rework tools/export-catalog to consume upstream's generated JSON catalog"
description: "Replace the TypeScript-import input path with upstream's JSON catalog output, and document the regeneration procedure honestly now that it is no longer reproducible from the pinned SHA alone."
tags: [epic-3]
timestamp: 2026-08-11T14:20:00Z
epic: 3
issue: 02
slug: export-catalog-json-input
size: M
status: open
gh_issue: 142
resource: https://github.com/kern-ia/kern-link/issues/142
depends_on: [1]
---

# Rework tools/export-catalog to consume upstream's generated JSON catalog

## Summary

`tools/export-catalog` gets its models by importing upstream's
`src/models.generated.ts`. At `936aff00` that file no longer contains model
data — it re-exports 39 `providers/<id>.models.ts` wrappers, each of which
imports `./data/<id>.json`, a directory upstream **gitignores**. The import
resolves to nothing on a fresh checkout.

This PR moves the tool onto an input that exists: the JSON catalog upstream's
own generator writes with `--json-only --json-output <dir>`
(`npm run generate-model-catalog`), which emits `<dir>/providers/<id>.json` as a
flat `{modelId: Model}` object, sorted by model id, plus `models.json` and
`providers.json`. `Object.values()` of one provider file is exactly the array
shape `ai/catalog/data/models/<id>.json` already holds.

The images half stays as it is: `src/image-models.generated.ts` is still
committed at `936aff00` with its models inline and only type imports, so it
imports cleanly.

The second half of this PR is honesty about reproducibility. The tool's doc
comment claims byte-stability against the pinned commit. Upstream's generator
now fetches live data from `models.dev`, OpenRouter, NVIDIA and the Vercel AI
Gateway, so the same SHA no longer implies the same output. Say so, in the
script and in `docs/PORTING.md`, rather than leaving a false invariant in place
for the next sync to trip over.

Confirm the shape of the finding against
[issue 01](/epic-3-catalog-schema-and-export-tooling/issues/01-export-catalog-spike.md)'s
drift record before writing code; where the record contradicts this issue, the
record wins.

## Scope

- `tools/export-catalog/export-catalog.ts` — replace the models input:
  - Read provider JSON from a catalog directory instead of
    `await import(models.generated.ts)`. Default to
    `${UPSTREAM_CLONE_DIR}/.artifacts/model-catalog` (upstream's
    `generate-model-catalog` destination is `../../.artifacts/model-catalog`,
    i.e. the upstream repo root). **Derive it from `UPSTREAM_CLONE_DIR` and stop
    there** — no second env var and no positional argument. The existing
    `UPSTREAM_CLONE_DIR` seam (`tools/export-catalog/export-catalog.ts:23`)
    already points the tool at any checkout, so a second knob would be a new
    configuration surface the epic never asked for.
  - Write `ai/catalog/data/models/<provider>.json` from `Object.values()` of
    each provider file, preserving the existing serialization exactly:
    tab-indented, trailing newline, `clearDir` first so a removed provider
    disappears instead of lingering.
  - Fail loudly with an actionable message when the catalog directory is
    missing — name the upstream command that produces it
    (`npm run generate-model-catalog`) rather than throwing a bare `ENOENT`.
- Keep the images path unchanged in behavior: still
  `await import(join(srcDir, "image-models.generated.ts"))`, still
  `data/images/<provider>.json`.
- Rewrite the script's doc comment: drop the byte-stability claim, state that
  the models input is produced by a **network-dependent** upstream generator and
  that `ai/catalog/data` is therefore a snapshot, not a pure function of the
  pinned SHA.
- `docs/PORTING.md` — update `:58` (the catalog mapping row) and the sync
  procedure at `:120-124`, whose step 2 currently says catalog-only changes are
  "re-run `tools/export-catalog` and the catalog tests; no Go code changes".
  Spell out the real procedure: generate upstream's JSON catalog first, then run
  the export tool.
- Where the finding contradicts `docs/planning/SPECS.md` — its one-sentence
  description of `tools/export-catalog` under "Stack", or the "The model catalog
  is not hand-maintained" paragraph under "Data model & storage" — **do not edit
  SPECS.md in this PR**. Extend
  [issue 01](/epic-3-catalog-schema-and-export-tooling/issues/01-export-catalog-spike.md)'s
  drift record, or add a second one under
  `docs/epics/epic-3-catalog-schema-and-export-tooling/drift/`, naming the exact
  sentence and what is true instead. `close-epic` promotes it to
  `docs/planning/DRIFT.md`, where the user dispositions it; a planning-doc edge
  triaged inside an implementation PR is the thing that mechanism exists to
  prevent.

## Out of scope

- **Regenerating any catalog data.** `ai/catalog/data/**` must be byte-identical
  before and after this PR — it lands in
  [issue 04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md)
  and [issue 05](/epic-3-catalog-schema-and-export-tooling/issues/05-regenerate-image-catalog.md).
  A diff under `ai/catalog/data/` in this PR is a defect.
- `ai/catalog` validation changes —
  [issue 03](/epic-3-catalog-schema-and-export-tooling/issues/03-catalog-validation-0-84-1.md).
- Porting upstream's generator (`scripts/generate-models.ts`, ~2100 lines) into
  this repo. `docs/PORTING.md` already records it as deliberately not ported,
  and nothing here changes that.
- Vendoring or committing upstream's `src/providers/data/` into kern-link.
- Bumping `upstream/UPSTREAM.lock` — [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md).
- Automating the regeneration in CI. It needs network and third-party data; it
  stays a by-hand step.

## Acceptance criteria / Definition of done

- [ ] Running the tool against a `936aff00` checkout whose JSON catalog has been
      generated writes 39 files under `ai/catalog/data/models/` and 1 under
      `ai/catalog/data/images/`, and prints the provider counts.
- [ ] Running it with no catalog directory present exits non-zero with a message
      naming both the missing path and the upstream command that produces it.
- [ ] The catalog directory is derived from `UPSTREAM_CLONE_DIR` alone:
      `git grep -n 'MODEL_CATALOG_DIR'` returns nothing, and the tool takes no
      positional argument.
- [ ] `git diff --name-only` lists nothing under `docs/planning/`; any
      contradiction with SPECS.md is recorded under the epic's `drift/` folder
      instead.
- [ ] Re-running the tool against the **current** pin (`244f1dea`, with its
      catalog generated) leaves `git status --porcelain ai/catalog/data` empty —
      the output format is unchanged, so this PR is provably data-neutral.
- [ ] `git diff --stat ai/catalog/data` is empty in this PR.
- [ ] The script's doc comment no longer claims byte-stability and names the
      network dependency explicitly.
- [ ] `docs/PORTING.md`'s sync procedure step for catalog changes describes the
      two-step regeneration; the `:58` mapping row still resolves.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] Conventional Commit, e.g.
      `chore(tools): read the model catalog from upstream's JSON output`.

## Relevant files / areas

- `tools/export-catalog/export-catalog.ts` — `:1-14` doc comment, `:20-24`
  path resolution incl. `UPSTREAM_CLONE_DIR`, `:26-36` `writeJSON`/`clearDir`,
  `:38-56` `main()` with the two `await import(...)` calls.
- `ai/catalog/data/models/*.json` (35 files today) and
  `ai/catalog/data/images/openrouter.json` — output targets, untouched here.
- `docs/PORTING.md:58`, `:110-128`.
- Read-only: `docs/planning/SPECS.md` — "Stack" and "Data model & storage". Not
  edited here; a contradiction becomes a drift record.
- Upstream at `936aff00`: `packages/ai/package.json` `scripts`
  (`generate-model-catalog` → `--strict --json-only --json-output
  ../../.artifacts/model-catalog`), `packages/ai/scripts/generate-models.ts`
  (`:2909-2919` the JSON-output writer; `:2769-2775` the sorted `jsonProviders`
  shape), `packages/ai/src/model-catalog.ts`,
  `packages/ai/src/image-models.generated.ts`.

## Dependencies

- **Blocked by**: [Issue 01](/epic-3-catalog-schema-and-export-tooling/issues/01-export-catalog-spike.md)
  — the input path chosen here is only correct if the spike's finding holds.
- **Blocks**: [Issue 04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md),
  [Issue 05](/epic-3-catalog-schema-and-export-tooling/issues/05-regenerate-image-catalog.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR.

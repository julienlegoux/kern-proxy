---
type: Issue
title: "Spike: price tools/export-catalog against upstream 936aff00 and record the finding"
description: "Verify whether tools/export-catalog can run against the frozen upstream revision at all, and write the finding down before the rest of the epic is planned against it."
tags: [epic-3]
timestamp: 2026-08-11T14:20:00Z
epic: 3
issue: 01
slug: export-catalog-spike
size: S
status: open
gh_issue: 141
resource: https://github.com/kern-ia/kern-link/issues/141
depends_on: []
---

# Spike: price tools/export-catalog against upstream 936aff00 and record the finding

## Summary

This is risk 1 of the whole program
([decision 21](../../../planning/scope/21-risks-and-assumptions.md)) and the
reason this epic starts here: `upstream/sync.sh` buckets 41 `*.models.ts` files
as "re-run `tools/export-catalog` only", but five upstream generator scripts
changed in range, so the export tool may not run against `936aff00` at all.

No code ships in this PR. The deliverable is a **verified, written finding**
that issues 02–05 are planned against.

**Expected finding — verify it, do not assume it.** Reading the upstream tree at
`936aff00` while drafting this epic surfaced the following. Reproduce each claim
against a real checkout and correct the record where it turns out to be wrong:

1. `packages/ai/src/providers/data/` is **gitignored upstream** (root
   `.gitignore` at `936aff00` line `packages/ai/src/providers/data/`). A fresh
   checkout of `936aff00` does not contain it.
2. Every `packages/ai/src/providers/<id>.models.ts` at `936aff00` is now a
   four-line wrapper — `import values from "./data/<id>.json" with { type: "json" }`
   plus `flattenModelCatalog(...)` from the new `src/model-catalog.ts` — where
   `0.80.3` inlined the model objects directly in TypeScript.
3. Therefore `tools/export-catalog/export-catalog.ts`'s
   `await import(join(srcDir, "models.generated.ts"))` resolves through
   `providers/*.models.ts` to files that do not exist, and the **models half of
   the tool cannot run**.
4. The **images half still works**: `src/image-models.generated.ts` is committed
   at `936aff00` (639 lines, models inline) and imports only types, so
   `await import(...)` of it succeeds independently of the models half.
5. Materializing `providers/data/` means running upstream's
   `npm run generate-models` (`node scripts/generate-models.ts --strict`), which
   **fetches over the network** — `https://models.dev/api.json`,
   `https://openrouter.ai/api/v1/models`, the NVIDIA `/models` endpoint and the
   Vercel AI Gateway `/models` endpoint (`scripts/generate-models.ts:990, 1012,
   1074, 1306`).
6. That kills the byte-stability property `tools/export-catalog`'s own doc
   comment claims ("re-running against the same `upstream/UPSTREAM.lock` commit
   reproduces identical output"). The output now depends on live third-party
   data, not on the pinned SHA.
7. Upstream also offers `npm run generate-model-catalog`
   (`--strict --json-only --json-output <dir>`), which writes
   `<dir>/providers/<id>.json` as a flat `{modelId: Model}` object sorted by
   model id, plus `models.json` and `providers.json`. It needs the same network
   fetch, but it is a plain-JSON input that needs no TypeScript import at all.

## Scope

- Clone/refresh the upstream checkout at `936aff00` (the existing
  `upstream/.upstream-clone`, or a fresh clone via `upstream/sync.sh`; the
  export tool honors `UPSTREAM_CLONE_DIR`).
- Run `tools/export-catalog` against it and capture the exact failure or success,
  verbatim.
- Establish what it costs to get a usable catalog input: Node/npm version needed,
  whether `npm install` in the upstream repo succeeds, whether
  `npm run generate-models` and `npm run generate-model-catalog` complete, and
  what network egress each requires.
- Record the finding as a **drift record** at
  `docs/epics/epic-3-catalog-schema-and-export-tooling/drift/01-export-catalog-not-byte-stable.md`
  (format in the pipeline interfaces): what was decided, what is actually true,
  the verified blocker with its command and error text, and a revisit trigger.
- Post the finding as a comment on the epic's tracking issue
  [#120](https://github.com/kern-ia/kern-link/issues/120), and update
  `EPIC_3.md`'s `## Notes` with the one-line outcome.
- Where the finding contradicts issues 02–05 as drafted, **record the
  contradiction in the drift record** — the claim, what is true instead, and
  which issue it invalidates — and say the same in the #120 comment. Editing
  those issue files is *not* part of this PR: a spike whose definition of done
  includes rewriting four successors has no bound, and issue 02 already carries
  "where the record contradicts this issue, the record wins", so the finding
  reaches its readers without the edit. If the finding invalidates an issue
  outright rather than adjusting it, open a follow-up issue to re-plan issues
  02–05 and link it from the record.

## Out of scope

- Any change to `tools/export-catalog/export-catalog.ts` — that is
  [issue 02](/epic-3-catalog-schema-and-export-tooling/issues/02-export-catalog-json-input.md).
- Any change to `ai/catalog/data/**` — that is
  [issue 04](/epic-3-catalog-schema-and-export-tooling/issues/04-regenerate-model-catalog.md)
  and [issue 05](/epic-3-catalog-schema-and-export-tooling/issues/05-regenerate-image-catalog.md).
- Any Go code. This PR touches `docs/` only.
- Bumping `upstream/UPSTREAM.lock` to `936aff00` — that is
  [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md), and the lock must
  keep reading `244f1dea` until the port is complete.
- Deciding whether upstream's `utils/provider-retry.ts` supersedes
  `ai/apis/internal/httpretry` — [Epic 9](/epic-9-classifier-audit-and-release/EPIC_9.md)
  owns that question.

## Acceptance criteria / Definition of done

- [ ] The exact command run against the `936aff00` checkout and its **verbatim
      output** (success or error text) are recorded in the drift record — not
      paraphrased.
- [ ] The drift record states, as a yes/no with evidence: does
      `tools/export-catalog` run green against `936aff00` as-is?
- [ ] If it does not, the record names the cheapest path to a usable catalog
      input and what that path costs (tooling prerequisites, network calls,
      whether the result is reproducible from the pinned SHA alone).
- [ ] The record's **Disposition** and **Revisit when** fields are filled with a
      concrete trigger, not "later".
- [ ] `docs/epics/epic-3-catalog-schema-and-export-tooling/drift/01-export-catalog-not-byte-stable.md`
      exists, has valid frontmatter, and is committed.
- [ ] The finding is posted as a comment on
      [#120](https://github.com/kern-ia/kern-link/issues/120).
- [ ] `EPIC_3.md`'s `## Notes` carries the one-line outcome and a refreshed
      `timestamp`.
- [ ] Every claim in the "Expected finding" list above is marked confirmed or
      corrected in the record, and each correction names the issue among 02–05 it
      bears on. No sibling issue file and no GitHub issue body other than #120 is
      touched by this PR: `git diff --name-only` lists nothing under
      `docs/epics/epic-3-catalog-schema-and-export-tooling/issues/`.
- [ ] If any correction invalidates an issue rather than adjusting it, a
      follow-up issue to re-plan issues 02–05 exists and is linked from the drift
      record.
- [ ] CI green: `go test ./... -race -v`, `bash upstream/sync_test.sh`,
      `golangci-lint` v2.12.2. (This PR changes no Go code, so this is a
      regression check, not new coverage.)
- [ ] Conventional Commit, e.g.
      `docs(epics): record the export-catalog finding for epic 3`.

## Relevant files / areas

- `tools/export-catalog/export-catalog.ts` (58 lines) — `:39-40` the two
  `await import(...)` calls that are the subject of this spike; `:1-14` the doc
  comment whose byte-stability claim is under test; `:23` honors
  `UPSTREAM_CLONE_DIR`.
- `upstream/sync.sh:25-27` — the clone/reuse logic and the
  `UPSTREAM_CLONE_DIR` default (`upstream/.upstream-clone`).
- `upstream/UPSTREAM.lock` — currently `commit=244f1dea…` / `version=0.80.3`;
  read-only for this issue.
- `docs/PORTING.md:58` (catalog mapping row) and `:120-124` (the sync procedure
  step that says catalog changes are "re-run `tools/export-catalog`…; no Go code
  changes") — both become wrong if the expected finding holds; note that here,
  fix them in issue 02.
- `docs/planning/SPECS.md` — "The model catalog is not hand-maintained" and the
  `tools/export-catalog` description under "Stack".
- Upstream at `936aff00`: `.gitignore`, `packages/ai/package.json` (`scripts`
  block), `packages/ai/scripts/generate-models.ts`,
  `packages/ai/scripts/model-data.ts`, `packages/ai/src/model-catalog.ts`,
  `packages/ai/src/providers/*.models.ts`.

## Dependencies

- **Blocked by**: None. This is the first issue of the epic by design.
- **Blocks**: [Issue 02](/epic-3-catalog-schema-and-export-tooling/issues/02-export-catalog-json-input.md),
  and — through issue 02, and through its own `depends_on: [1]` sequencing edge —
  issues 03, 04 and 05. The two images issues,
  [06](/epic-3-catalog-schema-and-export-tooling/issues/06-images-adapter-retry.md)
  and [07](/epic-3-catalog-schema-and-export-tooling/issues/07-images-options-base-and-auth-overrides.md)
  (#218), are independent of it: they touch no catalog data and no export
  tooling.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the
PR. This one should land well under that — it is a documentation PR.

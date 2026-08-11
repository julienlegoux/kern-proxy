---
type: Issue
title: "Disposition every in-range upstream file in docs/PORTING.md and drop the vX.Y.Z-go.N convention"
description: "Close the porting map over all 232 in-range upstream files until the disposition checker exits zero, reconcile the deviations list with what the program actually shipped, and remove the unused Go tagging convention."
tags: [epic-9]
timestamp: 2026-08-11T18:15:00Z
epic: 9
issue: 04
slug: disposition-sweep
size: L
status: open
gh_issue: 195
resource: https://github.com/kern-ia/kern-link/issues/195
depends_on: [1, 2, 3]
---

# Disposition every in-range upstream file in docs/PORTING.md and drop the vX.Y.Z-go.N convention

## Summary

Epic 9's acceptance criteria 3 and 5, and the second of the program's three
completion gates. `docs/PORTING.md` is the artifact that makes "full-parity Go
port" a checkable claim rather than a slogan: parity here has always meant
**behavioral** parity with a written exception list
([decision 02](../../../planning/scope/02-parity-bar.md)), and the exception
list is only worth anything if it is closed over the whole range.

Today the table is written in globs and generalizations that were accurate at
0.80.3. This PR closes it over the 232 files in range at `936aff00`, using the
checker from
[issue 03](/epic-9-classifier-audit-and-release/issues/03-disposition-checker.md)
as the arbiter — the criterion is the checker exiting 0, not a reviewer's
impression of completeness.

It also removes the `vX.Y.Z-go.N` tagging convention
([decision 03](../../../planning/scope/03-release-and-breaking-strategy.md)):
never used, and contradicting the SemVer line the CHANGELOG maintains.

## Scope

- Run `bash upstream/disposition_check.sh 936aff00918de1187f085f123c2812d8f2d67745`
  and work the `UNACCOUNTED` list to empty. For each file:
  - **ported** — add or widen a row naming the Go target, in the table's
    existing one-line-status style.
  - **not ported** — a row stating the reason inline (bundler shim, generator
    script, TS-only concern, superseded by a Go stdlib feature), and where the
    reason is substantive rather than mechanical, a matching bullet in
    **Intentional deviations**.
  Prefer widening an existing glob row over adding a near-duplicate; the table
  is read by humans, and 232 individual rows would be worse than the current
  shape, not better.
- Work the `MISSING` list to empty too: every Go path the table claims exists.
  Epics 1–8 renamed and moved real files (the module path move, the auth
  restructure, the new adapter packages), and stale targets are the failure mode
  this sweep exists to catch.
- Reconcile the rows and deviations the program invalidated:
  - `src/api/github-copilot-headers.ts` (`:46`) is recorded as **not ported**
    with an open follow-up. Update it to whatever epic 6 actually shipped
    against [decision 16](../../../planning/scope/16-copilot-headers-gap.md) —
    do not leave a gap recorded as open if it closed, or claim it closed if it
    did not.
  - The `src/providers/*.ts` row's "~35 bindings" count (`:57`) and any other
    count the program moved. Take the counts as they stand when this PR opens.
  - The auth rows (`:40-43`) after epic 6's restructure and epic 7's four new
    OAuth flows — those issues added their own rows; extend rather than
    duplicate.
  - Anything epic 8 issue 05 already added for pi-messages and radius, and
    anything [issue 02](/epic-9-classifier-audit-and-release/issues/02-provider-retry-vs-httpretry.md)
    added for `provider-retry.ts`. Both are already dispositioned; this PR does
    not restate them.
- Remove the `vX.Y.Z-go.N` convention from the sync procedure (`:128`), replacing
  it with kern-link's own SemVer line: the upstream version is recorded in
  `upstream/UPSTREAM.lock` and in the CHANGELOG entry, not in the Go tag.
- Refresh `docs/PORTING.md`'s frontmatter `timestamp`, and add a `docs/log.md`
  entry recording the sweep.

## Out of scope

- Porting anything. If the sweep finds an upstream file that *should* have been
  ported and was not, it does not get ported here — record it as a row with the
  gap stated, and file a follow-up issue. A sweep PR that also writes adapter
  code is unreviewable, and the parity bar explicitly allows a written
  exception — **but not an indefinite one**: `SCOPE.md:65-78` rules out
  postponement as a deviation ground by name ("a deviation is not a
  postponement"; admissible grounds are structural non-portability only, never
  "large" or "hard to test" or "no consumer asked for it"). A
  should-have-been-ported row therefore cannot carry an open follow-up past
  this program's release —
  [issue 06](/epic-9-classifier-audit-and-release/issues/06-cut-v0-2-0-release.md)
  gates the tag on it. If the follow-up cannot land before the tag, the row's
  status is `not ported` for an admissible reason instead, not a promise.
- `upstream/UPSTREAM.lock` and `CHANGELOG.md` —
  [issue 05](/epic-9-classifier-audit-and-release/issues/05-upstream-lock-and-changelog.md).
- Upstream files outside the range (commits after `936aff00`). The target is
  frozen; the weekly job will pick those up.
- `docs/planning/SPECS.md` — epic 8 issue 05 owns the package map; touch it only
  if this sweep proves it wrong, and then only the wrong lines.
- Changing the checker. If the parser cannot handle a row, that is a bug fixed
  in issue 03's script — but a row shape invented here purely to satisfy the
  parser is the tail wagging the dog: keep the table human-first.

## Acceptance criteria / Definition of done

- [ ] `bash upstream/disposition_check.sh 936aff00918de1187f085f123c2812d8f2d67745`
      exits **0**, with zero `UNACCOUNTED` and zero `MISSING` lines, and its
      summary line reports the full in-range file count.
- [ ] The summary's file count equals the epic's **232**, re-derived at this
      PR's base commit via
      `git -C upstream/.upstream-clone diff --name-status 244f1deaf1ae0fc1a242d9df5cddf457cf3d36a7..936aff00918de1187f085f123c2812d8f2d67745 -- packages/ai | wc -l`
      and stated in the PR description. This is no longer a hedge: since
      [issue 03](/epic-9-classifier-audit-and-release/issues/03-disposition-checker.md)'s
      checker enumerates the same in-range diff instead of the whole upstream
      tree, the checker's summary count and this command's count are the same
      measurement and must agree exactly.
- [ ] Every `not ported` row states a reason inline or links a bullet in
      **Intentional deviations**; no row has an empty or hand-wavy status. Most
      reasons are an admissible ground under `SCOPE.md:65-78` — a construct
      with no meaning in Go. A row whose only honest reason is "should be
      ported, not yet done" is still recorded that way (the checker only
      verifies coverage, not admissibility) and names the follow-up issue —
      but that combination is exactly what
      [issue 06](/epic-9-classifier-audit-and-release/issues/06-cut-v0-2-0-release.md)'s
      new criterion blocks the tag on; it does not ship silently.
- [ ] `grep -n 'go\.N' docs/PORTING.md` returns nothing, and the sync procedure
      instead records that Go tags follow kern-link's own SemVer with the
      upstream version living in `UPSTREAM.lock` and the changelog.
- [ ] The `src/api/github-copilot-headers.ts` row matches what epic 6 shipped —
      verified against the code, not against the old note.
- [ ] Every count in the table (bindings, adapters) matches the tree at the time
      the PR opens.
- [ ] `docs/PORTING.md` frontmatter `timestamp` refreshed; `docs/log.md` has an
      entry for the sweep.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2) — a docs PR still has to be green.
- [ ] Conventional Commit, e.g.
      `docs: disposition every in-range upstream file in the porting map`.

## Relevant files / areas

- `docs/PORTING.md` — the mapping table (`:19-64`), **Intentional deviations**
  (`:66-109`), the sync procedure and its `go.N` line (`:111-131`), frontmatter
  (`:1-7`).
- `upstream/disposition_check.sh` from
  [issue 03](/epic-9-classifier-audit-and-release/issues/03-disposition-checker.md)
  — the arbiter.
- `docs/log.md`, `docs/index.md`.
- The epic bundles whose rows already exist and must not be duplicated:
  [epic 7 issue 06](/epic-7-four-new-oauth-flows/issues/06-auth-docs-and-porting.md),
  [epic 8 issue 05](/epic-8-pi-messages-and-radius/issues/05-porting-and-docs.md).
- Upstream at `936aff00`: the whole of `packages/ai`.

## Dependencies

- **Blocked by**: [Issue 01](/epic-9-classifier-audit-and-release/issues/01-classifier-parity-audit.md)
  and [issue 02](/epic-9-classifier-audit-and-release/issues/02-provider-retry-vs-httpretry.md)
  (their rows must exist before the table is declared closed);
  [issue 03](/epic-9-classifier-audit-and-release/issues/03-disposition-checker.md)
  (the checker is the acceptance criterion). Also depends on every other epic
  having merged — a sweep run against a half-merged tree measures nothing.
- **Blocks**: [Issue 05](/epic-9-classifier-audit-and-release/issues/05-upstream-lock-and-changelog.md)
  — the lock may not claim `0.84.1` while files remain unaccounted.

## PR size note

`L` — ~600 changed lines of `docs/PORTING.md`: 232 in-range files closed out
across the mapping table and **Intentional deviations**, plus the `go.N`
convention removed from the sync procedure and a `docs/log.md` entry. `L` is the
ceiling, not the target: the table must be closed in one pass because the
checker's exit-0 criterion is only meaningful once, and splitting it would mean
two branches editing the same rows. If it trends past ~1000, that is a signal
the rows are too granular — widen globs before splitting the PR.

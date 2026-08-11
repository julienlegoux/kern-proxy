---
type: Issue
title: "Bump upstream/UPSTREAM.lock to 936aff00 / 0.84.1 and write the v0.2.0 changelog"
description: "Advance the pin once, at the end, and enumerate every break of the nine-epic program in a Keep a Changelog 0.2.0 entry."
tags: [epic-9]
timestamp: 2026-08-11T18:15:00Z
epic: 9
issue: 05
slug: upstream-lock-and-changelog
size: M
status: open
gh_issue: 196
resource: https://github.com/kern-ia/kern-link/issues/196
depends_on: [4]
---

# Bump upstream/UPSTREAM.lock to 936aff00 / 0.84.1 and write the v0.2.0 changelog

## Summary

Epic 9's acceptance criteria 4 and the first half of 6. `UPSTREAM.lock` is not a
progress marker — it is a **claim about what the port implements**, which is why
[decision 17](../../../planning/scope/17-upstream-lock-and-weekly-job.md)
advances it exactly once, at the end. A tree claiming `0.84.1` while files remain
unaccounted would mislead the next sync into skipping real work, so this issue
sits behind the disposition sweep.

The changelog entry is the other half of the same claim, aimed at humans.
v0.2.0 is deliberately breaking with no shims and no deprecation window
([decision 03](../../../planning/scope/03-release-and-breaking-strategy.md)):
the package has no known consumers and is not ready to be depended on. What that
buys is an obligation to enumerate the breaks honestly — this entry is the only
place a reader sees them all at once.

## Scope

- `upstream/UPSTREAM.lock`: `commit=936aff00918de1187f085f123c2812d8f2d67745`,
  `version=0.84.1`. Leave `repo=` and `package=` alone, and leave the file's
  two header comment lines intact — `upstream/sync.sh` and `sync_test.sh` parse
  this file by prefix.
- `CHANGELOG.md`: a `## [0.2.0] - <release date>` section under `## [Unreleased]`,
  in the existing Keep a Changelog 1.1.0 shape (`### Added` / `### Changed` /
  `### Removed`), enumerating every break of the program. Derive the list from
  what actually merged, not from memory:
  `gh pr list --state merged --base develop --limit 300 --json number,title,mergedAt`
  plus each epic's `docs/epics/epic-*/issues/index.md`.
  At minimum it must cover:
  - the module path move to `github.com/kern-ia/kern-link` and the Go directive
    bump (epic 1) — the break a consumer hits first, so it leads;
  - `StopReason` gaining `pending`/`deferred` and `ThinkingLevel` gaining `max`
    (non-exhaustive `switch` in consumer code);
  - tiered `ModelCost` / `CalculateCost` and the catalog entries that change
    shape with it;
  - `StreamOptions` refactored onto `ProviderRequestOptions`;
  - the `Models` surface: `AuthModel` dropped, `ModelsStore`, the refresh
    contract, `ModelsRequestTransforms`, deferred-response dispatch;
  - the catalog schema and `tools/export-catalog` rework (epic 3);
  - adapter behavior changes across the OpenAI family and the remaining adapters
    (epics 4, 5);
  - the auth restructure and the four new OAuth flows (epics 6, 7);
  - the pi-messages adapter and the radius provider (epic 8);
  - the classifier resync (issue 01) and the httpretry outcome (issue 02).
- The section states the upstream version this release syncs to — `0.84.1` at
  `936aff00` — because the `vX.Y.Z-go.N` tag convention that used to carry that
  information is gone
  ([issue 04](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md)).
- Move anything sitting under `## [Unreleased]` that ships in 0.2.0 into the new
  section, and keep the link-reference style the file already uses at its foot
  if it has one — it does (`CHANGELOG.md:63-65`). Add
  `[0.2.0]: https://github.com/kern-ia/kern-link/compare/v0.1.1...v0.2.0` in
  the same form as `[0.1.1]`. **Do not** also refresh `[Unreleased]` and
  `[0.1.1]` off the old `julienlegoux` org here —
  [Epic 1 issue 02](/epic-1-repository-hygiene/issues/02-move-module-path-to-kern-ia.md)
  already moves those exact two lines (`CHANGELOG.md:63-65`, first bullet
  under `## Scope`), and Epic 9 depends transitively on every epic in the
  program (`EPIC_9.md`, `## Dependencies`), so by the time this issue runs the
  footer already reads `kern-ia/kern-link` — re-touching it here would either
  no-op or, worse, conflict with what Epic 1 already shipped.

## Out of scope

- Tagging, the `develop → main` merge, the GitHub release, and closing #113 —
  [issue 06](/epic-9-classifier-audit-and-release/issues/06-cut-v0-2-0-release.md).
- Any code change. If writing the entry reveals an undocumented break, the entry
  documents it and a follow-up issue fixes the code — a release-prep PR that
  also changes behavior invalidates the CI run the release is cut from.
- `docs/PORTING.md` — [issue 04](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md).
- Touching `.github/workflows/upstream-sync.yml`. The weekly job stays running;
  its "skip if an `upstream-sync` issue is already open" guard keeps it quiet
  while #113 is open, and it is *meant* to fire once #113 closes.

## Acceptance criteria / Definition of done

- [ ] `grep -c '^commit=936aff00918de1187f085f123c2812d8f2d67745$' upstream/UPSTREAM.lock`
      returns 1, and `grep -c '^version=0.84.1$'` returns 1.
- [ ] `bash upstream/sync_test.sh` passes — it exercises the lock format, so a
      malformed bump fails here rather than in the next weekly run.
- [ ] `bash upstream/sync.sh 936aff00918de1187f085f123c2812d8f2d67745` prints
      "No upstream changes since the pinned revision." — the machine-checkable
      form of the parity claim.
- [ ] `CHANGELOG.md` has a `## [0.2.0]` section; every epic 1–8 is represented
      by at least one bullet, and each of the breaks listed under Scope appears.
- [ ] The section names the upstream sync target (`0.84.1`, `936aff00`).
- [ ] Nothing that ships in 0.2.0 is still parked under `## [Unreleased]`.
- [ ] `CHANGELOG.md`'s link-reference footer carries a `[0.2.0]` compare link
      in the same form as `[0.1.1]`, and `[Unreleased]` / `[0.1.1]` already
      read `kern-ia/kern-link` — Epic 1 issue 02's move, verified rather than
      redone.
- [ ] Every bullet describes the *effect on a consumer* (what breaks, what to
      change), not the edit that produced it.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] Conventional Commit, following the repo's release precedent
      (`chore(release): prepare v0.1.1 changelog`):
      `chore(release): prepare v0.2.0 changelog and bump the upstream lock`.

## Relevant files / areas

- `upstream/UPSTREAM.lock` — six lines; `commit=` and `version=` are the two
  that move.
- `upstream/sync.sh:22-24` — how the lock is parsed (`grep '^commit=' | cut -d= -f2`),
  which is why the format must not drift.
- `CHANGELOG.md:1-30` — the Keep a Changelog header, `## [Unreleased]`, and the
  `## [0.1.1]` section that models the shape.
- Every epic's `docs/epics/epic-*/issues/index.md` — the merged-issue inventory
  the entry is derived from.
- `.github/workflows/upstream-sync.yml` — read-only context: the job whose
  behavior changes when #113 closes.

## Dependencies

- **Blocked by**: [Issue 04](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md)
  — the lock may not claim `0.84.1` while any in-range file is unaccounted.
- **Blocks**: [Issue 06](/epic-9-classifier-audit-and-release/issues/06-cut-v0-2-0-release.md)
  — the tag is cut from a `develop` that already carries this entry.

## PR size note

`M` — ~200 changed lines: two lines in `upstream/UPSTREAM.lock` and one
link-reference line, plus a `CHANGELOG.md` `## [0.2.0]` section that enumerates
every break of a nine-epic program — one bullet per consumer-visible effect
across `### Added` / `### Changed` / `### Removed`, from the eleven listed under
`## Scope` upward. Split past ~500, though there is nothing here to split: the
changelog section is one reviewable unit and the lock bump is two lines.

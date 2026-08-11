---
type: Issue
title: "Normalize the in-bundle link form and the depends_on representation, and record the branch conventions once per epic"
description: "Rewrite the nine issues/index.md files onto the leading-slash link form, express depends_on and issue in one identifier form across the bundle, and put the branch, base-branch and status-commit conventions in each epic's Notes."
tags: [epic-0]
timestamp: 2026-08-11T21:00:00Z
epic: 0
issue: 14
slug: normalize-links-depends-on-and-branch-notes
size: S
status: done
gh_issue: 212
gh_pr: 230
resource: https://github.com/kern-ia/kern-link/issues/212
depends_on: ["13"]
---

# Normalize the in-bundle link form and the depends_on representation, and record the branch conventions once per epic

## Summary

Three bundle-wide inconsistencies that no single epic owns, each raised by several
reports independently. All nine `issues/index.md` files link as `./NN-slug.md` where the
bundle rule mandates the leading-slash form that every issue *body* already uses.
`depends_on: [1]` and `issue: 01` express the same identifier two ways, so a consumer
comparing them without normalisation resolves the dependency to nothing. And
`docs/planning/CONVENTIONS.md:213-225` — branch naming, the `develop` base, and the
separate `docs(epics): …` status commit — is reflected densely everywhere else in the
bodies and nowhere for those three rules, which reads as "not required" rather than
"assumed".

Epic 0 has already settled the ambiguity behind the first one (`EPIC_0.md`, `## Notes`):
the link form normalizes to the **leading slash**, not recorded as drift — the skill
contract's link table mandates it, its bullet template shows a bare filename, and every
issue body in the bundle already uses the leading-slash form, so the indexes are the
outlier.

Findings repaired: REPORT_1 P3 ×3, REPORT_2 P3 ×2, REPORT_4 P3, REPORT_5 P3, REPORT_7 P3,
REPORT_9 P3. Epic 0 scope items 15 and 16; acceptance criterion 14.

## Scope

**1. The index link form.** Rewrite every bullet in all nine
`docs/epics/epic-*/issues/index.md` from `](./NN-slug.md)` to
`](/epic-<n>-<slug>/issues/NN-slug.md)`. Keep the bullets mechanical — title, size,
status, GitHub link — and rewrite a section wholesale rather than patching it line by
line if repeated edits have left it disordered.

**2. The `depends_on` representation.** Pick one form and apply it to all nine epics, not
just the ones that noticed: `depends_on: ["01"]` matching the zero-padded `issue: 01`, or
`issue: 1` matching the plain integers. Recommended: keep `issue: <nn>` zero-padded (the
schema requires it) and quote `depends_on` entries to match — the schema calls
`depends_on` "other issue numbers in this epic" without fixing a form, so the one that
matches the field it references wins. Whichever is chosen, apply it uniformly across every
issue file in the bundle, Epic 0's own included, and state the choice once in
`docs/epics/index.md` so the next issue written to this bundle follows it.

**3. The branch, base-branch and status-commit conventions.** Add one line to each of the
ten epic files' `## Notes` — Epic 0's included — covering all of an epic's issues at once
rather than repeating it in 73 bodies: feature branches are named `issue-<NN>-<slug>`
(`docs/planning/CONVENTIONS.md:216-218`), PRs target `develop` and merge with a merge
commit (no rebase, no squash, `:213-219`), and each status transition gets its own
`docs(epics): …` commit separate from the implementation commit (`:224-227`). Note also
that `Closes #N` will not auto-close an issue, since PRs target `develop` rather than the
default branch — Epic 1 is the first epic to exercise that path, and the explicit close
is the normal route, not a fallback.

## Out of scope

- **Rewriting in-bundle links inside GitHub issue bodies.** A `](/epic-N-…)` link copied
  verbatim into a GitHub body renders as `https://github.com/epic-N-…` and 404s — a real
  problem, and [issue 15](/epic-0-plan-remediation/issues/15-reconcile-github-state-with-the-files.md)'s,
  because the fix is a GitHub-side rewrite (to full `https://github.com/kern-ia/kern-link/blob/develop/docs/…`
  URLs), not a change to the files. The files stay on the bundle-absolute form.
- **Any content edit.** Same rule as
  [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md): if this PR
  finds a defect, it goes in the PR body, not the diff.
- **`docs/planning/CONVENTIONS.md`.** Item 3 quotes it; it does not edit it.
- **`docs/epics/log.md` and `docs/epics/index.md` link forms** — both already use the
  mandated leading-slash form; leave them.

## Acceptance criteria / Definition of done

- [ ] `grep -rn '](\./' docs/epics/*/issues/index.md` returns nothing.
- [ ] Every link in the nine `issues/index.md` files resolves to an existing file when
      read as a bundle-relative absolute path — checked by resolving each target on disk,
      not by eye.
- [ ] All ten epics express `depends_on` in one form, that form matches the `issue:`
      field it references, and the choice is stated once in `docs/epics/index.md`.
- [ ] Each of the ten epic files' `## Notes` records the branch name pattern, the
      `develop` base with merge commits, the separate `docs(epics): …` status commit, and
      the `Closes #N` caveat.
- [ ] Every `issues/index.md` still agrees with its files' `title`, `size`, `status` and
      `gh_issue` after the rewrite — normalising the link form must not silently drop a
      bullet or reorder the list.
- [ ] `timestamp` refreshed on every edited epic and issue file.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-normalize-links-depends-on-and-branch-notes`, PR targets
      `develop`, merge commit. Commit e.g.
      `docs(epics): normalize the bundle's link form and depends_on representation`.

## Relevant files / areas

- All ten `docs/epics/epic-*/issues/index.md` (nine today plus Epic 0's).
- All ten `docs/epics/epic-*/EPIC_*.md` — the `## Notes` sections.
- Every issue file's `depends_on:` line — 28 of the 71 read `[]` and need no change; the
  rest carry between one and five entries. Re-count at the PR's base commit, since
  [issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md) and
  [issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md) add
  three files and several edges.
- `docs/epics/index.md` — where the chosen `depends_on` form is stated.
- Read-only: `docs/planning/CONVENTIONS.md:213-227`.

## Dependencies

- **Blocked by**: [Issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md)
  — it rewrites every `issues/index.md` bullet's size field, and two sweeps over the same
  nine index files in flight together is the one conflict this ordering exists to avoid.
  Transitively blocked by issues 03–12.
- **Blocks**: [Issue 15](/epic-0-plan-remediation/issues/15-reconcile-github-state-with-the-files.md),
  which reconciles GitHub last, once every local title, link and bullet has settled.

## PR size note

`S` — ~150–200 changed lines: 73 one-line `depends_on` edits at most, ~73 index bullets,
and ten `## Notes` paragraphs. Purely mechanical, and reviewable as a diffstat. Split past
~200, by taking item 3 (the ten `## Notes` lines) as its own PR — it is the only part that
adds prose.

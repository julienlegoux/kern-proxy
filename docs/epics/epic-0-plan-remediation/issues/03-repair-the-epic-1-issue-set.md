---
type: Issue
title: "Repair the Epic 1 issue set: the two acceptance criteria its issues fail, and the off-convention commit type"
description: "Make EPIC_1.md's AC 2 and AC 4 satisfiable by the two issues that implement them, give the SPECS.md rewrite a criterion, and drop the build: commit type the repo does not use."
tags: [epic-0]
timestamp: 2026-08-10T04:00:00Z
epic: 0
issue: 03
slug: repair-the-epic-1-issue-set
size: S
status: in-progress
gh_issue: 201
resource: https://github.com/kern-ia/kern-link/issues/201
depends_on: []
---

# Repair the Epic 1 issue set: the two acceptance criteria its issues fail, and the off-convention commit type

## Summary

Epic 1 opens the first branch of the nine-epic program, and three of its defects are
the kind that propagate: an acceptance criterion no implementation can satisfy, a
mandated commit type the repo does not use, and a `docs/planning/SPECS.md` rewrite that
sits in a `## Scope` with no criterion checking it — inside the one directory the
issue's own grep excludes.

Small file count, high leverage: whatever lands in the first PR of the program is what
everyone copies.

Findings repaired: REPORT_1 P2 ×3 and REPORT_1 P3 ×2 (the two `file:line`/count
errors). Epic 0 scope items 5, 12 and 14; acceptance criteria 5, 13 and (in part) 17.

## Scope

**1. Epic AC 2's exception (REPORT_1 P2).** `EPIC_1.md:49-50` grants exactly one
exception — historical CHANGELOG entries — while `epic-1 issue 02:96` excludes
`':!docs/planning' ':!docs/epics'` as well, leaving 23 old-path occurrences alive that
the criterion forbids (12 in `docs/planning/`, 11 in `docs/epics/`, measured at
`59e9c5b`). `EPIC_1.md:28` itself carries the old path, so the criterion is
self-contradictory before any implementation runs. Extend `EPIC_1.md:49-50`'s exception
to cover the two bundles, with the reason issue 02 already gives at `:79-82` (those
records exist to state the old path), and name issue 02 as what forced the amendment.

**2. Epic AC 4's single PR (REPORT_1 P2).** `EPIC_1.md:53-54` asks for one
`refactor!:`-scoped PR and `EPIC_1.md:34` restates it; the issues deliberately ship two
PRs, well argued at `epic-1 issue 01:26-31`. Amend `EPIC_1.md:53-54` to say the epic
ships as two PRs with the `refactor!:` requirement applying to the rename, naming issue
01's reasoning. Then give epic AC 1's *combined* end state one checkable owner: add
`grep -c '^go 1\.26$' go.mod` returns `1` to `epic-1 issue 02`'s acceptance criteria,
so nothing has to trust that the toolchain bump survived the 170-file rename.

**3. The `build:` commit type (REPORT_1 P2).** `epic-1 issue 01:72-74` mandates
"Conventional Commits **with a scope**, e.g. `build: bump the go directive to 1.26`" —
`build` is not in the repo's observed type set (`docs/planning/CONVENTIONS.md:205`:
`feat`, `fix`, `refactor`, `docs`, `ci`, `chore`) and the example carries no scope,
contradicting its own sentence. Replace with a type the repo uses and a real scope,
e.g. `chore(go): bump the go directive to 1.26`.

**4. The un-criteria'd SPECS.md rewrite (REPORT_1 P2).** `epic-1 issue 02:59-62` puts
two `docs/planning/SPECS.md` edits in `## Scope` (`:22`, both the module path and
Go **1.25.0** → **1.26**; and `:247`), and every criterion the issue defines exempts
`docs/planning/`. Issue 01 defers its own SPECS line there too (`01:48-50`). Add the
criterion: `git grep -n 'julienlegoux/kern-link' -- docs/planning/SPECS.md` prints
nothing, and `docs/planning/SPECS.md:22` reads `github.com/kern-ia/kern-link` at
Go **1.26**.

**5. The counts and the off-by-one (REPORT_1 P3 ×2).** `epic-1 issue 02:119-121` cites
`git grep -o 'julienlegoux/kern-link' | wc -l` → **370** at `318731c`; the command
returns **369** (366 for the `github.com/`-prefixed form the rename targets).
`ai/providers` carries the path in **44** files, not the 42 stated at `:126`.
`README.md`'s "7 refs" (`:44-45`) is a `grep -c` line count next to a `grep -o`
occurrence headline — two units in one section. Re-derive with the exclusions the issue
itself applies and **state the producing command**, since the number drifts as `docs/`
grows. Separately, `:54` cites root `CONVENTIONS.md:72-77` for a section whose heading
is at `:71`; correct to `:71-77`.

## Out of scope

- **Implementing Epic 1.** No `go.mod` change, no module rename, no `.go` file touched.
- **Rewriting the two issues' PR size notes or their `size:` values** —
  [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md) sweeps all
  71 issues at once, after every content repair has landed.
- **`issues/index.md`'s `./` link form and `depends_on: [1]` vs `issue: 01`** —
  [issue 14](/epic-0-plan-remediation/issues/14-normalize-links-depends-on-and-branch-notes.md)
  normalizes all nine epics in one pass.
- **Recording the branch, base-branch and status-commit conventions** in the bodies
  (REPORT_1 P3) — also issue 14, which writes them once per epic's `## Notes` rather
  than 71 times.
- **`docs/planning/SPECS.md` itself.** This issue adds a *criterion* about SPECS.md to
  an Epic 1 issue; it does not edit SPECS.md. Epic 1 will, when it runs.

## Acceptance criteria / Definition of done

- [ ] `EPIC_1.md` contains no acceptance criterion its own two issues contradict:
      AC 2's exception covers `CHANGELOG.md`, `docs/planning/` and `docs/epics/`, and
      AC 4 describes two PRs. Each amendment names the issue that forced it.
- [ ] `git grep -n 'build:' docs/epics/epic-1-repository-hygiene/` returns nothing, and
      the replacement example uses a type from `docs/planning/CONVENTIONS.md:205` **with
      a scope**.
- [ ] `epic-1 issue 02` carries both new criteria: the `go 1.26` survival grep and the
      `docs/planning/SPECS.md` check.
- [ ] Every count in `epic-1 issue 02` reproduces from the command printed beside it,
      run at the commit the issue names; `ai/providers` reads 44; the `grep -o` and
      `grep -c` figures are labelled as different units or unified.
- [ ] `epic-1 issue 02:54` cites `CONVENTIONS.md:71-77`.
- [ ] `timestamp` refreshed on `EPIC_1.md` and both issue files; GitHub bodies of #127
      and #128 updated to match; #118's body left alone.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-repair-the-epic-1-issue-set`, PR targets `develop`, merge
      commit. Commit e.g. `docs(epics): make EPIC_1's criteria satisfiable by its issues`.

## Relevant files / areas

- `docs/epics/epic-1-repository-hygiene/EPIC_1.md:28`, `:34`, `:49-50`, `:53-54`.
- `.../issues/01-bump-go-directive-to-1-26.md:26-31`, `:48-50`, `:72-74`.
- `.../issues/02-move-module-path-to-kern-ia.md:44-45`, `:54`, `:59-62`, `:79-82`,
  `:94`, `:96`, `:112-113`, `:119-121`, `:126`.
- Read-only, to re-derive: the tree at `318731c` and at the PR base, `go.mod:1`, `:3`,
  `CONVENTIONS.md:71-77` (repo root), `docs/planning/CONVENTIONS.md:205`,
  `docs/planning/SPECS.md:22`, `:247`.

## Dependencies

- **Blocked by**: None. Epic 1 shares no file with any other epic's repairs.
- **Blocks**: [Issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md)
  — the size sweep rewrites both Epic 1 issue files and must not race this PR.

## PR size note

`S` — ~80–150 changed lines across three files. Split past ~200; there is no natural
seam below that, and the five repairs are all in the same two issue bodies.

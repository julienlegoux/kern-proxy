---
type: Drift
title: "docs/REPORT_1.md and docs/REPORT_9.md keep the old module path"
description: "Issue 02's exclusion list covers CHANGELOG.md, docs/planning/ and docs/epics/ but not the review reports, which quote those records verbatim; rewriting them would turn quotations into misquotations."
tags: [epic-1, drift]
timestamp: 2026-08-17T16:00:00Z
epic: 1
issue: 02
gh_issue: 128
---

# docs/REPORT_1.md and docs/REPORT_9.md keep the old module path

## Decided

[Issue 02](../issues/02-move-module-path-to-kern-ia.md) acceptance criterion 3:

> No unintended reference survives:
> `git grep -l 'julienlegoux/kern-link' -- ':!CHANGELOG.md' ':!docs/planning' ':!docs/epics'`
> prints nothing.

[EPIC_1.md](../EPIC_1.md) acceptance criterion 2 names the same three exclusions:
historical CHANGELOG entries, `docs/planning/` and `docs/epics/`.

## Actual

Two more files keep the old path, so that grep prints `docs/REPORT_1.md` and
`docs/REPORT_9.md` rather than nothing:

- `docs/REPORT_1.md:15` quotes `EPIC_1.md:49-50` verbatim; `:42`, `:46`, `:49`
  quote the `git grep` commands whose whole point is to match the old string;
  `:95` states that `EPIC_1.md:28` itself contains the old path.
- `docs/REPORT_9.md:243` quotes the pre-rename `CHANGELOG.md` link-reference line;
  `:309` records a rejected option ("that move is epic 1 issue 02").

## Because

These are review reports: audit records whose sentences are quotations of other
records. Every one of the seven occurrences is inside a quotation, a `git grep`
command, or a statement about what another file contains. Rewriting them does not
update a reference to the module — it makes a report misquote its own source,
which is the exact harm the epic's `docs/planning/` and `docs/epics/` exclusions
exist to prevent. `docs/REPORT_*.md` are the same genus of record as the bundles
they review; the exclusion list simply did not enumerate them.

Verified: `git grep -n 'julienlegoux/kern-link' -- docs/REPORT_1.md docs/REPORT_9.md`
returns 7 lines, all of the above. No occurrence is an import, a `go get`, a badge
URL, or any other live reference to the module.

## Alternatives tried

- **Rewrite them anyway to satisfy the criterion literally.** Rejected: it would
  make `REPORT_1.md:15` quote `EPIC_1.md` as saying `kern-ia` when `EPIC_1.md`
  says `julienlegoux`, and would rewrite `git grep` invocations into commands that
  no longer find what they were written to find.
- **Delete the stale reports.** Rejected: out of scope, and they are the audit
  trail that produced this issue's own exclusions.

## Disposition

Proposed `accepted` — the user's call at `close-epic`. If accepted, issue 02's
criterion 3 and `EPIC_1.md`'s criterion 2 should both gain `':!docs/REPORT_*.md'`
so the standard stops contradicting the tree.

## Revisit when

The `docs/REPORT_*.md` files are archived, regenerated, or moved under
`docs/planning/`, at which point the extra exclusion becomes unnecessary.

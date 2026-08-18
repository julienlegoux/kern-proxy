---
type: Drift
title: "kern-link — Drift"
description: "Standards this codebase has drifted from, and why"
tags: [planning, drift]
timestamp: 2026-08-18T18:10:00Z
---

# Drift

Standards `docs/planning/` already decided that the implementation contradicts,
promoted from the per-epic drift records at each epic's close. Append-only,
newest epic first: an entry that stops being true becomes
`resolved (<date>)` rather than disappearing.

Read this alongside [SPECS.md](SPECS.md) and [CONVENTIONS.md](CONVENTIONS.md) —
a decided standard plus its live drift is what the code actually looks like.

## Epic 1: Repository hygiene

### 01 — The module-path grep excluded the bundles but not the review reports

- **Decided**: [Issue 02](../epics/epic-1-repository-hygiene/issues/02-move-module-path-to-kern-ia.md)
  acceptance criterion 3 and [EPIC_1.md](../epics/epic-1-repository-hygiene/EPIC_1.md)
  acceptance criterion 2 — no reference to `github.com/julienlegoux/kern-link`
  survives outside `CHANGELOG.md`, `docs/planning/` and `docs/epics/`.
- **Actual**: `docs/REPORT_1.md` (5 occurrences) and `docs/REPORT_9.md` (2) keep
  the old path, so the criterion's `git grep` prints those two files instead of
  nothing.
- **Because**: all 7 occurrences sit inside a verbatim quotation of `EPIC_1.md`
  or `CHANGELOG.md`, inside a `git grep` command written to match the old
  string, or inside a statement about what another file contains. None is an
  import, a `go get`, or a badge URL. Rewriting them would make each report
  misquote its own source — the exact harm the `docs/planning/` and
  `docs/epics/` exclusions exist to prevent. The review reports are the same
  genus of record as the bundles they review; the exclusion list simply did not
  enumerate them.
- **Disposition**: accepted — both criteria were amended at epic close to add
  `':!docs/REPORT_*.md'`, so the standard stops contradicting the tree.
- **Revisit when**: the `docs/REPORT_*.md` files are archived, regenerated, or
  moved under `docs/planning/`, at which point the extra exclusion becomes
  unnecessary.
- **Evidence**: [drift record 01](../epics/epic-1-repository-hygiene/drift/01-report-files-keep-the-old-module-path.md),
  PR #233.

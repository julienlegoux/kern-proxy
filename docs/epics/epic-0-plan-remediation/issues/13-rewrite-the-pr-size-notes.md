---
type: Issue
title: "Rewrite the PR size note in every issue against its own band, and correct the mis-declared sizes"
description: "Replace the one identical sentence copied into 70 of 71 issue bodies with a per-issue note that matches its size field, re-label the issues whose size contradicts their scope, and state the generated-catalog rule once."
tags: [epic-0]
timestamp: 2026-08-11T19:30:00Z
epic: 0
issue: 13
slug: rewrite-the-pr-size-notes
size: M
status: pr-open
gh_issue: 211
gh_pr: 229
resource: https://github.com/kern-ia/kern-link/issues/211
depends_on: [3, 4, 5, 6, 7, 8, 9, 10, 11, 12]
---

# Rewrite the PR size note in every issue against its own band, and correct the mis-declared sizes

## Summary

Seventy of the bundle's seventy-one issue files close with the same sentence:

> Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.

That describes an `L`. It is quoted verbatim at `S` issues, where it names a target
**2.5×** the band's ceiling, and at `M` issues, where it licenses growth to the top of
`L`. All nine review reports raised it independently — it is the single most-repeated
finding in the bundle — because it is the field a developer feels first and the one place
an implementer could have learned where a split would be clean.

Every report also found `size:` values that contradict their own scope, in both
directions. This issue fixes the note and the field together, because fixing either alone
leaves the file self-contradictory.

Findings repaired: REPORT_2 P2 + P3, REPORT_3 P2 ×2 + P3, REPORT_4 P2 + P3, REPORT_5 P2
×2, REPORT_6 P2 ×2, REPORT_7 P2, REPORT_8 P2, REPORT_9 P3. Epic 0 scope item 13;
acceptance criterion 12.

## Scope

**1. Rewrite every `## PR size note`** against its file's own `size:` band
(`S` ≈ under 200 changed lines, `M` ≈ 200–500, `L` ≈ 500–1000, ceiling not target):

- `S` — "~N changed lines; split past ~200."
- `M` — "~N changed lines; split past ~500."
- `L` — keep the shape Epic 5 issue 04, Epic 9 issue 04 and Epic 9 issue 06 already use:
  state *why* the issue is whole, and name the honest cut if it overruns.

Keep every per-issue second sentence that already exists (Epic 3 issues 01/04/05/06,
Epic 4 issue 08, Epic 5 issue 04, Epic 7 issue 04, Epic 9 issues 04/06 each have one),
and add one wherever the report named a natural seam: Epic 4 issues 05, 07, 09 and 11,
Epic 6 issue 08, Epic 8 issue 02.

**2. Correct the mis-declared `size:` values**, each with the report's reasoning:

| Issue | From | To | Why |
|---|---|---|---|
| Epic 2 issue 07 | `S` | `M` | three-method interface, concurrent store, deep copy, six tests, two `PORTING.md` rows — ~200–250 lines (REPORT_2 P3) |
| Epic 3 issues 04 / 05 | `L` / `S` | per item 3 | the two apply opposite rules to one situation (REPORT_3 P2) |
| Epic 4 issues 09 / 11 | `M` | `L` | +472 and +778 upstream test lines respectively (REPORT_4 P2) |
| Epic 4 issue 10 | `S` | `M` | its own note targets ~500, 2.5× its band (REPORT_4 P2) |
| Epic 5 issue 01 | `S` | `M` | new package, three adapter packages wired, three unit tests plus `httptest` in three packages (REPORT_5 P2) |
| Epic 6 issue 08 | `M` | `L` | two flows rewritten plus 1153 lines of tests reworked (REPORT_6 P2) |
| Epic 7 issues 01–03 | `M` | `L` | 574 / 580 / 633 upstream lines plus a bespoke callback server in 03 (REPORT_7 P2) |
| Epic 8 issues 01 / 02 | `M` | `L` | 433 TS lines over two issues plus a 248-line test port plus 12–13 named Go tests (REPORT_8 P2) |

Re-check, and re-label only if the reading holds: Epic 5 issues 09 and 10, Epic 6 issue
02, Epic 9 issue 01. Where a re-label is declined, say so in the size note rather than
leaving the report's reasoning unanswered.

**3. State the generated-catalog rule once.** Epic 3 issues 04 and 05 apply opposite
rules to one situation — 04 declares `L` because the generated JSON is huge and then
argues at `:138-139` that "the `~500` target applies to hand-written lines only", while 05
makes the identical argument and declares `S`. Epic 0 has decided
(`EPIC_0.md`, `## Notes`): **generated catalog output does not count toward the size
bands** — the hand-written rule is the one that measures reviewability. Write that once
into `EPIC_3.md`'s `## Notes` and re-label both issues against it.

**4. Update every `issues/index.md` bullet** whose `size` changed, in all nine epics, and
every GitHub issue body whose `## PR size note` or `size:` changed.

## Out of scope

- **Splitting any issue.** Epic 2 issues 05 and 08 are pre-split by
  [issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md),
  which lands first; this issue re-labels and re-notes the halves it produced. No other
  issue is split — an `L` label with an honest note is the correct outcome for the rest,
  and splitting them would re-plan work Epic 0 is not scoped to re-plan.
- **Any content edit.** Scope, acceptance criteria, dependencies and citations are all
  owned by [issues 03–12](/epic-0-plan-remediation/issues/index.md), which all land
  first. If this PR finds a content defect, record it in the PR body — do not fix it
  here, or the size sweep stops being reviewable as a diffstat.
- **Epic 4 issue 09's escape hatch** at `:145-148` ("land the highest-value cases and
  open a follow-up for the remainder"), which negotiates away `EPIC_4.md:54-55` — delete
  it as part of re-declaring the issue `L`, since with an honest band there is nothing
  to negotiate. That is the one content edit this issue does make, and it is a
  consequence of the re-label.
- **`docs/planning/`.** No planning doc changes.

## Acceptance criteria / Definition of done

- [ ] `grep -rn 'Target ~500 changed lines' docs/epics/` returns only issues whose `size`
      is `L` — and only where 500 is genuinely that issue's target, not a copied
      sentence. (`EPIC_0.md` quotes the sentence once, in its own `## Scope`; that
      quotation stays.)
- [ ] Every issue's `size:` value, its `## PR size note` and its `issues/index.md` bullet
      agree, in all nine epics — verified file by file, not by spot check.
- [ ] Every note names a per-issue figure or reason; no two notes in the bundle are byte
      identical.
- [ ] `EPIC_3.md`'s `## Notes` states the generated-catalog rule once, and Epic 3 issues
      04 and 05 are labelled against it.
- [ ] Every re-label in the table above is applied or explicitly declined in the issue's
      own note with a reason.
- [ ] Epic 4 issue 09 no longer contains the "open a follow-up for the remainder" clause.
- [ ] `timestamp` refreshed on every edited file; every edited GitHub issue body updated
      to match its file.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-rewrite-the-pr-size-notes`, PR targets `develop`, merge commit.
      Commit e.g. `docs(epics): rewrite every PR size note against its own band`.

## Relevant files / areas

- All 73 issue files under `docs/epics/epic-*/issues/*.md` (71 today, plus the two
  [issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md)
  adds, plus the one
  [issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md) adds —
  re-count at the PR's base commit rather than trusting this number).
- All nine `docs/epics/epic-*/issues/index.md`.
- `docs/epics/epic-3-catalog-schema-and-export-tooling/EPIC_3.md` — `## Notes`.
- The existing good notes, as models: `epic-5 issue 04:197-207`, `epic-7 issue 04:257-260`,
  `epic-9 issue 04:143-150`, `epic-9 issue 06:112-116`, `epic-2 issue 08:174-183`,
  `epic-4 issue 08:152-158`.
- The pipeline's size bands (`S` ≈ under 200, `M` ≈ 200–500, `L` ≈ 500–1000, "L is the
  ceiling, not the target") as quoted in `EPIC_0.md`'s `## Scope` — the rule being
  applied. It lives in the skill contract, not in this repo.

## Dependencies

- **Blocked by**: [Issues 03–12](/epic-0-plan-remediation/issues/index.md) — every one of
  them edits issue bodies this PR also touches, and a sweep across 73 files racing ten
  content PRs is a guaranteed conflict. This is why it is second to last.
- **Blocks**: [Issue 14](/epic-0-plan-remediation/issues/14-normalize-links-depends-on-and-branch-notes.md)
  and [issue 15](/epic-0-plan-remediation/issues/15-reconcile-github-state-with-the-files.md).

## PR size note

`M` — ~350–500 changed lines: roughly five per issue file across 73 files, plus the
`size:` corrections, their index bullets and one `EPIC_3.md` paragraph. It is wide rather
than deep, and reviewable as a diffstat because every hunk has the same shape. Split past
~500, and the seam is per-epic: nine PRs of eight-ish files each, in epic order. Take the
seam if the first pass runs long — do not let it become a 700-line sweep nobody reads.

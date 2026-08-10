---
type: Issue
title: "Repair the Epic 9 issue set: the checker's file set, the classifier row count, and the release escape hatch"
description: "Derive the disposition checker's candidate set from the 232-file range instead of the whole upstream tree, label it an assumption, fix the unsatisfiable row-count criterion, and close the gap that lets an unported file ship as a recorded deviation."
tags: [epic-0]
timestamp: 2026-08-10T03:10:00Z
epic: 0
issue: 12
slug: repair-the-epic-9-issue-set
size: M
status: open
gh_issue: 210
resource: https://github.com/kern-ia/kern-link/issues/210
depends_on: [6]
---

# Repair the Epic 9 issue set: the checker's file set, the classifier row count, and the release escape hatch

## Summary

Epic 9 holds the program's two completion gates, and both are compromised by the issues
that implement them. The disposition checker enumerates
`git ls-tree -r --name-only <ref> -- packages/ai` — the whole upstream tree at the target
ref — where the epic's "232 files in range" means the *delta* between `244f1dea` and
`936aff00`. It therefore counts hundreds of files that never changed and, structurally,
cannot see the three files upstream deleted in range: exactly the ones whose Go
counterparts are now orphaned can never be reported `UNACCOUNTED`, and the sweep exits 0
with them undispositioned.

Then the classifier audit's one mechanically checkable criterion asks for a row count of
75 against code that holds 72, so a correct table fails and a padded one lies. And the
sweep's escape hatch — "record it as a row with the gap stated, and file a follow-up
issue" — is precisely the postponement `SCOPE.md:65-78` rules out by name, with nothing
in issues 05 or 06 blocking the release on an open porting follow-up.

Findings repaired: REPORT_9 P1, P2 ×3 (the unrequested checker, the row count, the escape
hatch), P3 ×5 (AC 2's DRIFT wording, the duplicated `httpretry` criterion, the
`classifier-parity.md` placement, the CHANGELOG footer). Epic 0 scope items 4, 5, 6, 7, 9
and 14; acceptance criteria 4, 5, 6, 7, 9 and 13.

## Scope

**1. The checker's candidate set (REPORT_9 P1).** Rewrite `epic-9 issue 03:41-44` and
`:55` to derive the set from
`git -C "$workdir" diff --name-status 244f1deaf1ae0fc1a242d9df5cddf457cf3d36a7..936aff00918de1187f085f123c2812d8f2d67745 -- packages/ai`,
keeping `D` entries as in-range paths that still need a disposition row, and make the
summary line print that count so `232` is checkable rather than asserted. If a whole-tree
sweep is wanted **as well**, state it as a deliberate superset and drop the "makes the 232
number observable" claim — but do not leave two definitions of "in range" in one epic.
Then fix the criterion that consumes it: `epic-9 issue 04:95-100`'s "if the real count
differs, the PR says so" turns a hard gate into a narrative one; with the enumeration
corrected, the count is checkable and the hedge goes.

**2. Label the checker an assumption (REPORT_9 P2).** `epic-9 issue 03` is a whole PR of
new shell tooling (`upstream/disposition_check.sh` + its offline test), a new step in
`.github/workflows/test.yml`, and a permanent addition to `docs/PORTING.md`'s documented
sync procedure — none of which `EPIC_9.md:34-36`, `:44-51` or `:68-69` asks for. Epic 0
has decided to **keep** it (`EPIC_0.md`, `## Notes`): dropping it would make the epic's
second completion gate a judgement call again. So state at the head of issue 03 that the
checker is an assumption added beyond `EPIC_9.md`, and reconcile the issue's own
contradiction — `03:64-67` wires the test script into `test.yml` while `03:76-79` asserts
"the three gates `CONVENTIONS.md` names stay the three gates". Say plainly that the
`test` job gains a fourth step while the number of CI *checks* is unchanged, and add the
authorizing line to `EPIC_9.md`'s `## Scope` so the tooling stops being derived rather
than requested.

**3. The row-count criterion (REPORT_9 P2).** `epic-9 issue 01:92-96` requires the row
count to be "at least `2 + 6 + 6 + 34 + 24 + 3`" = 75; counted against the tree, the six
groups the same issue lists at `:51-59` hold **8, 6, 28, 24, 3, 3** = 72, and no group
holds 34 or 2. Replace the arithmetic with a property — every regex in the six groups
appears exactly once, and `docs/classifier-parity.md`'s row count equals the count
produced by enumerating those six vars — or correct it to `8 + 6 + 28 + 24 + 3 + 3`,
re-derived at the PR's base commit. Related: `01:57-59` extends the epic's "every regex"
(`EPIC_9.md:26-30`, `:55-57`) to three non-regex detection modes; defensible, so say so
as an explicit extension rather than leaving it implied.

**4. Close the release escape hatch (REPORT_9 P2).** `epic-9 issue 04:79-82` allows an
upstream file that *should* have been ported to ship as "a row with the gap stated" plus
a follow-up issue. `SCOPE.md:65-78` rules that out by name — "a deviation is not a
postponement … Not admissible: 'large', 'hard to test'" — and `EPIC_9.md:34-36` carries it
as "no file left in a third, unaccounted state". Require every non-`ported` row to state
an **admissible** ground (a construct with no meaning in Go), and make a
"should-have-been-ported" finding a release blocker: add an acceptance criterion to issue
05 or 06 that no open porting follow-up exists at tag time. If the maintainer would
rather accept the deviation from `SCOPE.md:65-78`, say that explicitly in issue 04 — but
one of the two, not the current silence.

**5. `EPIC_9.md`'s AC 2 wording (REPORT_9 P3).** `EPIC_9.md:31-33` and `:58-59` make the
`httpretry`-vs-`provider-retry.ts` difference "a `docs/planning/DRIFT.md` entry", which no
Epic 9 PR writes — `epic-9 issue 02:69-75` correctly writes a drift record and leaves the
register to `close-epic` (`06:63-65` excludes the promotion). The issue is right; the epic
is the side to fix. Reword `:58-59` to "settled in writing — as code, or as a drift record
promoted to `docs/planning/DRIFT.md` at epic close". Note that
[issue 06](/epic-0-plan-remediation/issues/06-rein-in-epic-3-invented-scope.md) hands the
`provider-retry.ts` reconciliation to this issue's epic explicitly; make sure `epic-9
issue 02` reads as the accepting side.

**6. De-duplicate the `httpretry` criterion (REPORT_9 P3).** `epic-9 issue 01:115-118`
asserts `IsNonRetryableProviderLimitError` still wins "in both `IsRetryableAssistantError`
… and `httpretry.IsRetryable`, with a test" — reaching into a package its own `:76-78`
puts out of scope and duplicating `epic-9 issue 02:110-112` nearly verbatim. Keep only the
`IsRetryableAssistantError` half in issue 01 and note that the `httpretry` coupling is
asserted by issue 02.

**7. `docs/classifier-parity.md`'s home (REPORT_9 P3).** A regex-by-regex sync audit is a
dev-process artifact, and `epic-9 issue 01:45-50`, `:69-72` puts it in the consumer-facing
`docs/` bundle with its own `docs/index.md` bullet, against
`docs/planning/mapping/01-planning-bundle-nesting.md:69-74`. There is real
counter-precedent — `docs/PORTING.md` is equally dev-facing and sits at `docs/index.md:14`
— so either accept it explicitly as following the `PORTING.md` precedent (one sentence in
issue 01) or move it under `docs/planning/` and link it from `docs/PORTING.md:27-28`.
Decide; do not leave it unremarked.

**8. The CHANGELOG footer (REPORT_9 P3).** `epic-9 issue 05:68-69` says to "keep the
link-reference style the file already uses at its foot if it has one"; it does
(`CHANGELOG.md:63-65`), and it points at the **old** `julienlegoux` org that Epic 1 issue
02 moves away from. Add to issue 05's `## Scope`: refresh `CHANGELOG.md:63-65` onto
`kern-ia/kern-link` and add the `[0.2.0]` compare link in that form.

## Out of scope

- **Implementing Epic 9.** No Go code, no shell script, no workflow change.
- **Dropping issue 03.** Decided against in `EPIC_0.md`'s `## Notes`; item 2 keeps it and
  labels it.
- **Relocating `ai/apis/internal/httpretry`** — decided against, and enforced by
  [issue 06](/epic-0-plan-remediation/issues/06-rein-in-epic-3-invented-scope.md), which
  points the reconciliation here.
- **Writing `docs/planning/DRIFT.md`.** The register stays absent until an implementation
  epic discovers real drift; item 5 fixes the epic's *wording* about it.
- **Re-checking whether issue 01 should be `L`, and the four boilerplate PR size notes** —
  [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).
- **The `./` index links and the bundle-absolute links inside GitHub bodies** —
  [issue 14](/epic-0-plan-remediation/issues/14-normalize-links-depends-on-and-branch-notes.md)
  and [issue 15](/epic-0-plan-remediation/issues/15-reconcile-github-state-with-the-files.md).

## Acceptance criteria / Definition of done

- [ ] `epic-9 issue 03`'s enumeration is the in-range diff, `D` entries included, and its
      summary line prints a count that can be compared against 232; `epic-9 issue 04`'s
      acceptance criterion no longer hedges on "if the real count differs".
- [ ] `epic-9 issue 03` states at its head that the checker is an assumption beyond
      `EPIC_9.md`, `EPIC_9.md`'s `## Scope` authorizes it, and the issue no longer
      contradicts itself about the CI gates.
- [ ] `epic-9 issue 01`'s row-count criterion is satisfiable by a correct table — verified
      by counting the six groups in `ai/retry.go` and `ai/overflow.go` at the PR's base
      commit — and its three non-regex modes are labelled an explicit extension.
- [ ] No Epic 9 issue permits an unported-but-should-have-been file to ship: every
      non-`ported` disposition requires an admissible ground, and a release-blocking
      criterion exists in issue 05 or 06 — or issue 04 records the maintainer's explicit
      acceptance of the deviation from `SCOPE.md:65-78`.
- [ ] `EPIC_9.md` contains no acceptance criterion its own issues cannot meet, and each
      amendment names the issue that forced it.
- [ ] `httpretry.IsRetryable` is asserted by exactly one Epic 9 issue.
- [ ] `docs/classifier-parity.md`'s placement is decided in writing, with the precedent
      or the move stated.
- [ ] `epic-9 issue 05`'s `## Scope` names `CHANGELOG.md:63-65` and the `[0.2.0]` compare
      link on `kern-ia/kern-link`.
- [ ] Every `file:line` in a passage this PR touches is re-derived at the PR's base
      commit.
- [ ] `timestamp` refreshed on every edited file; GitHub bodies of #192–#197 and #126
      updated to match.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-repair-the-epic-9-issue-set`, PR targets `develop`, merge
      commit. Commit e.g. `docs(epics): point Epic 9's disposition gate at the 232-file range`.

## Relevant files / areas

- `docs/epics/epic-9-classifier-audit-and-release/EPIC_9.md:26-30`, `:31-33`, `:34-36`,
  `:44-51`, `:55-59`, `:60-61`, `:68-69`.
- `.../issues/01-classifier-parity-audit.md:45-50`, `:51-59`, `:57-59`, `:69-72`,
  `:76-78`, `:92-96`, `:115-118`.
- `.../issues/02-provider-retry-vs-httpretry.md:69-75`, `:79-80`, `:110-112`.
- `.../issues/03-disposition-checker.md:35-63`, `:41-44`, `:55`, `:64-67`, `:68-69`,
  `:76-79`.
- `.../issues/04-disposition-sweep.md:14`, `:79-82`, `:95-100`, `:103-105`, `:136-139`.
- `.../issues/05-upstream-lock-and-changelog.md:68-69`, `:83-103`, `:111-112`.
- `.../issues/06-cut-v0-2-0-release.md:63-65`, `:69-91`.
- Read-only: `ai/retry.go:16-33`, `:41-48`, `:53-97`, `ai/overflow.go:9-34`, `:39-43`,
  `:55-87`, `CHANGELOG.md:63-65`, `docs/planning/SCOPE.md:28-29`, `:65-78`,
  `docs/planning/mapping/01-planning-bundle-nesting.md:69-74`, `docs/index.md:11-14`,
  `docs/PORTING.md:27-28`, `upstream/sync.sh`, `.github/workflows/test.yml`.

## Dependencies

- **Blocked by**: [Issue 06](/epic-0-plan-remediation/issues/06-rein-in-epic-3-invented-scope.md)
  — it hands the `provider-retry.ts` reconciliation to Epic 9 issue 02 explicitly, and
  item 5 has to be written against that hand-off.
- **Blocks**: [Issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).

## PR size note

`M` — ~250–450 changed lines across eight files, concentrated in Epic 9 issues 01, 03 and
04. Split past ~500. The clean cut: items 1–2 (the checker) first, since they change what
the epic's second gate measures; items 3–8 second.

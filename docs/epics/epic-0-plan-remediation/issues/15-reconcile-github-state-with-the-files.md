---
type: Issue
title: "Reconcile GitHub state with the files, and account for all 97 findings"
description: "Restore the ten issue titles that dropped their colon, rewrite the bundle-absolute links that 404 inside GitHub bodies, verify the bundle is internally consistent, and record the disposition of every finding."
tags: [epic-0]
timestamp: 2026-08-11T21:00:00Z
epic: 0
issue: 15
slug: reconcile-github-state-with-the-files
size: S
status: in-progress
gh_issue: 213
resource: https://github.com/kern-ia/kern-link/issues/213
depends_on: ["13", "14"]
---

# Reconcile GitHub state with the files, and account for all 97 findings

## Summary

Two GitHub-side defects, and the epic's closing ledger.

Ten issue titles (#149–#158) dropped the colon their local files carry — `03:3` reads
`openai-completions: emit and stream grammar custom tools` while
`gh issue view 149 --json title` returns `openai-completions emit and stream grammar
custom tools`. Cosmetic, except that a title search from either side misses and later
reconciliation cannot match on title. And the bundle-absolute links the files correctly
use — `](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md)` — render
as `https://github.com/epic-9-…` once copied verbatim into a GitHub body, and 404 for
anyone reading the issue on GitHub.

This lands last by construction: titles are only worth reconciling once they have stopped
moving, and fourteen Epic 0 PRs have been editing them.

Findings repaired: REPORT_4 P3, REPORT_9 P3. Epic 0 scope item 17; acceptance criteria 15,
16 and 17.

## Scope

**1. The ten titles.** For each of #149–#158, run
`gh issue edit <n> --title "<exact frontmatter title>"` so the GitHub title matches its
file's `title:` byte for byte. Then sweep the whole bundle rather than trusting that only
those ten drifted — every issue's title is compared against its GitHub issue, and any
mismatch found (including in the three issues Epic 0 created and in Epic 0's own fifteen)
is fixed the same way. Worth a note in the PR body about *which* step stripped the colon,
since Epic 5's issues show the same shape and the next `create-issues` run will do it
again.

**2. The links inside GitHub bodies.** Rewrite every `](/epic-` link in a GitHub issue
body — and in the ten epic tracking issues' bodies — to a full
`https://github.com/kern-ia/kern-link/blob/develop/docs/epics/…` URL that resolves in a
browser. The **files keep the bundle-absolute form**: it is what the bundle rule requires
and what
[issue 14](/epic-0-plan-remediation/issues/14-normalize-links-depends-on-and-branch-notes.md)
normalizes the indexes onto. Only the copies pushed to GitHub are rewritten. Note this in
the PR body as a standing hazard of pushing bundle text to GitHub verbatim.

**3. Verify the bundle is internally consistent** after fifteen PRs of edits — the check
`close-epic` would otherwise discover the hard way:

- Every `issues/index.md` bullet agrees with its file's `title`, `size`, `status` and
  `gh_issue`; every listed file exists and every existing file is listed.
- `docs/epics/index.md` lists all ten epics with their real status and issue numbers.
- `docs/epics/log.md` has a `**Creation**` entry for each of the three issues Epic 0
  created plus Epic 0's own fifteen, and no entry for a file that does not exist.
- Every milestone holds exactly the issues its epic's files predict, every sub-issue is
  attached to its tracking issue, and no sub-issue carries the `epic` label.
- Every `resource:` URL resolves to the issue its `gh_issue` names.

**4. Account for all 97 findings.** Walk `docs/REPORT_1.md`–`REPORT_9.md` finding by
finding and record the disposition of each in `EPIC_0.md`'s `## Notes`: repaired (naming
the Epic 0 issue that repaired it), or not repaired with the reason. The reports are
**consumed, never amended** — `EPIC_0.md`'s `## Out of scope` says so, and they are the
record of what the bundle looked like when someone looked. This is the ledger that makes
the epic closable, and it is the acceptance criterion (`EPIC_0.md`, AC 16) with no other
owner.

## Out of scope

- **Editing `docs/REPORT_1.md`–`REPORT_9.md`.** Consumed, never amended.
- **Re-reviewing.** Severities come from the reports as written. A finding judged wrong
  is recorded as `won't-fix` in the ledger with its reason, not re-graded.
- **Any local content edit.** If item 3's verification finds a real inconsistency, fix
  the *bookkeeping* (an index bullet, a log entry, a milestone assignment); if it finds a
  content defect, record it in `EPIC_0.md`'s `## Notes` as unrepaired with a reason, and
  say so in the PR body. Epic 0 does not silently grow a sixteenth issue's worth of work
  into its last PR.
- **Closing the milestone or retiring Epic 0.** That is `close-epic`'s job, once every
  issue is `done`.
- **Rewriting the bundle's link form in the files** —
  [issue 14](/epic-0-plan-remediation/issues/14-normalize-links-depends-on-and-branch-notes.md).

## Acceptance criteria / Definition of done

- [ ] Every GitHub issue title in the `upstream-sync` program matches its file's `title:`
      frontmatter byte for byte — verified by a scripted comparison over every issue file
      in `docs/epics/`, not by checking the ten known ones.
- [ ] No GitHub issue body contains a link beginning `](/epic-`; every link in a GitHub
      body resolves in a browser.
- [ ] `git grep -n '](/epic-' docs/epics/` still returns the bundle-absolute links in the
      **files** — item 2 must not "fix" them on disk.
- [ ] Every `issues/index.md`, `docs/epics/index.md` and `docs/epics/log.md` agrees with
      the frontmatter of the files it indexes; every milestone, sub-issue link and label
      matches what the files predict.
- [ ] `EPIC_0.md`'s `## Notes` accounts for all 97 findings: each is marked repaired with
      the owning Epic 0 issue number, or unrepaired with a reason. The count in the ledger
      equals the count of findings in the nine reports, re-derived by walking them.
- [ ] `timestamp` refreshed on `EPIC_0.md` and on any file whose bookkeeping changed.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-reconcile-github-state-with-the-files`, PR targets `develop`,
      merge commit. Commit e.g. `docs(epics): reconcile GitHub state and close the finding ledger`.

## Relevant files / areas

- GitHub: issues #127–#197 and the three Epic 0 created, the ten tracking issues
  #118–#126 and #198, the umbrella #113, and milestones 18–27.
- `docs/epics/epic-0-plan-remediation/EPIC_0.md` — `## Notes`, the finding ledger.
- All ten `docs/epics/epic-*/issues/index.md`, `docs/epics/index.md`,
  `docs/epics/log.md`.
- `docs/REPORT_1.md`–`docs/REPORT_9.md` — read only, walked finding by finding.
- Useful commands: `gh issue list --milestone "<title>" --json number,title,labels`,
  `gh api repos/kern-ia/kern-link/issues/<n>/sub_issues`,
  `gh issue edit <n> --title "…" --body-file <file>`.

## Dependencies

- **Blocked by**: [Issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md)
  and [issue 14](/epic-0-plan-remediation/issues/14-normalize-links-depends-on-and-branch-notes.md),
  and transitively every other Epic 0 issue. Titles and bodies are only worth reconciling
  once they have stopped moving.
- **Blocks**: Nothing inside this epic. It is the last PR before `close-epic`.

## PR size note

`S` — near-zero local diff: `EPIC_0.md`'s `## Notes` plus whatever bookkeeping item 3
turns up. The work is GitHub-side and verification-side, so the diffstat understates it
by design. Split past ~200 local changed lines, which would itself be a signal that item
3 found something bigger than bookkeeping.

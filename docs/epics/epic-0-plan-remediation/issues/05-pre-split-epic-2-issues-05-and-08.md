---
type: Issue
title: "Pre-split Epic 2 issues 05 and 08 into reviewable halves"
description: "Cut the two Epic 2 issues that concede their own size ceiling into four, so the split is made now against the plan rather than mid-PR at the point of no return."
tags: [epic-0]
timestamp: 2026-08-10T03:10:00Z
epic: 0
issue: 05
slug: pre-split-epic-2-issues-05-and-08
size: M
status: open
gh_issue: 203
resource: https://github.com/kern-ia/kern-link/issues/203
depends_on: [4]
---

# Pre-split Epic 2 issues 05 and 08 into reviewable halves

## Summary

Two Epic 2 issues write their own split into their PR size notes and then leave the cut
to be made while the PR is being written. Issue 08 calls itself "the largest single
contract change in the sync" (`08:21`) and its size note (`:174-183`) already names the
seam; issue 05, once its 218 mandatory literal rewrites are honest (see
[issue 04](/epic-0-plan-remediation/issues/04-repair-the-epic-2-issue-set.md)), is well
past the `~1000` ceiling it sets for itself at `:156-157`.

A split decided mid-PR is decided under pressure, at the point of no return. Issue 08's
documented fallback is worse than the split: "the interim `publish` implemented as an
unconditional apply and the gap flagged in the PR body" ships a `RefreshModelsContext`
whose central guarantee is absent, contradicting the issue's own
`TestPublishReturnsFalseWhenProviderSuperseded` (`:126-128`).

Findings repaired: REPORT_2 P1 (the pre-split half) and REPORT_2 P2 (issue 08's
conceded ceiling). Epic 0 scope item 13's pre-split clause; acceptance criteria 6 and 12.

## Scope

**1. Split issue 05** into two issues along the seam REPORT_2 names:

- **05 (kept, rewritten)** — the struct split plus the mechanical literal rewrite:
  `ProviderRequestOptions`, the embedding into `StreamOptions`, the ~218 keyed-literal
  rewrites, and nothing else. Reviewable as a diffstat.
- **new issue** — `Fetch`/`FetchFunction`, `SamplingParams`, `DeferredFetchOptions` /
  `DeferredCancelOptions`, the `Model` field, and the two `docs/PORTING.md` rows.

  The two fields [issue 01](/epic-0-plan-remediation/issues/01-epic-4-core-type-contracts.md)
  assigns to Epic 2 issue 05 (`OpenAIToolChoice`, `OpenAIThinkingBudgets`) stay with the
  half that declares `StreamOptions` fields — the second one — and Epic 4 issues 04, 05,
  09 and 11 have their "Blocked by" line re-pointed at it.

**2. Split issue 08** along the seam its own size note writes:

- **08 (kept, rewritten)** — the `Provider`/`Models` contract types, the
  `createProvider` baseline-plus-overlay rework, and the `FetchModels` binding
  migration for the four dynamic bindings in `ai/providers`.
- **new issue** — generation counters, per-provider publication serialization, and
  cancellation, carrying `TestPublishReturnsFalseWhenProviderSuperseded` (`08:126-128`)
  and the criterion at `:134-135`. Delete the "unconditional apply plus a PR-body flag"
  fallback from the kept half — with the split made, it has nothing to fall back from.

**3. Number, wire and register the two new issues.** Append them as Epic 2 issues **13**
and **14** rather than renumbering 06–12: the schema requires `<nn>` two-digit numbers
in dependency order, and a backwards edge from a higher number is legal while
renumbering nine files and their `depends_on`, slugs, filenames and GitHub issues is
churn with a real chance of a mistake. Say so in one line in each new issue's Summary so
the numbering reads as deliberate.

- New issue 13 (`Fetch`/sampling/deferred options): `depends_on: [5]`.
- New issue 14 (generation and publication): `depends_on: [8]`.
- Rewire every intra-epic `depends_on` and `## Dependencies` prose that named the whole
  of 05 or 08 to the half it actually needs — at minimum issues 06 (`[1, 5]`), 08
  (`[5, 7]`), 09 (`[5, 8]`), 10 (`[1, 2, 5, 8, 9]` after issue 04) and 12 (`[1, 2, 10]`).
  Re-read each and repoint it; do not assume the old number still means the same work.
- Create both GitHub issues on milestone 19, label them as their siblings are
  (`enhancement`, `upstream-sync`, never `epic`), attach each as a native sub-issue of
  #119, add bullets to `issues/index.md`, and append a `**Creation**` entry per issue to
  `docs/epics/log.md`.

## Out of scope

- **Implementing any of the four halves.** No Go code.
- **Renumbering Epic 2 issues 06–12**, or renaming any existing slug, filename or
  GitHub issue.
- **Splitting anything else.** Epic 4 issues 09/11, Epic 6 issue 08, Epic 7 issue 03 and
  Epic 8 issues 01/02 are all flagged as under-declared sizes by their reports; those are
  `size:` corrections and per-issue size notes, which
  [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md) owns. Only
  Epic 2's two are split, because only those two concede the ceiling in their own bodies
  and name the seam.
- **The `## PR size note` wording** on the four resulting issues beyond what a new file
  needs — issue 13 rewrites all of them against their bands.

## Acceptance criteria / Definition of done

- [ ] `docs/epics/epic-2-core-types-and-models-contracts/issues/` holds 14 issue files;
      the two new ones carry complete frontmatter per the issue schema, all seven body
      sections in order, `size` matching their real content, and `status: open` with
      `gh_issue` + `resource` filled.
- [ ] Neither issue 05 nor issue 08 still contains a conditional split, a "decide
      mid-PR" sentence, or the unconditional-apply fallback: `git grep -n 'if this grows'
      docs/epics/epic-2-*/issues/05-*.md docs/epics/epic-2-*/issues/08-*.md` returns only
      the band-appropriate note.
- [ ] `TestPublishReturnsFalseWhenProviderSuperseded` appears in exactly one Epic 2
      issue.
- [ ] The Epic 2 dependency graph is acyclic and every `depends_on` agrees with its
      `## Dependencies` prose in both directions, re-verified by reading all 14 files —
      not by editing only the two split ones.
- [ ] Epic 4 issues 04, 05, 09 and 11 point their cross-epic blocker line at the Epic 2
      issue that actually declares `OpenAIToolChoice` / `OpenAIThinkingBudgets` after the
      split.
- [ ] Both new GitHub issues exist on milestone 19, are native sub-issues of #119, carry
      the sibling label set, and their titles match their files' `title:` byte for byte.
- [ ] `issues/index.md` lists all 14 in numeric order with matching size, status and
      issue number; `docs/epics/log.md` has a `**Creation**` entry for each new issue.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-pre-split-epic-2-issues-05-and-08`, PR targets `develop`, merge
      commit. Commit e.g. `docs(epics): pre-split Epic 2 issues 05 and 08`.

## Relevant files / areas

- `docs/epics/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md`
  — `:67-71` (SamplingParams), `:103-105` (the embedding), `:141-142` (the `// Ports:`
  instruction), `:156-163` (the size note).
- `.../issues/08-models-refresh-contract.md:21`, `:90-92`, `:126-128`, `:134-135`,
  `:174-183`.
- `.../issues/06-simple-options-and-lazy.md:14`, `.../issues/09-models-request-transforms.md:14`,
  `.../issues/10-deferred-response-dispatch.md:14`, `.../issues/12-faux-deferred-responses.md:14`
  — the `depends_on` lines to repoint.
- `.../issues/index.md`, `docs/epics/log.md`.
- `docs/epics/epic-4-openai-family-adapters/issues/04-*.md`, `05-*.md`, `09-*.md`,
  `11-*.md` — the cross-epic blocker lines.
- Read-only: `ai/provider.go` (489 lines, the surface issue 08 changes), `ai/options.go`.

## Dependencies

- **Blocked by**: [Issue 04](/epic-0-plan-remediation/issues/04-repair-the-epic-2-issue-set.md)
  — it rewrites issue 05's acceptance criteria and issue 10's edge; splitting first
  would mean distributing criteria that are still wrong. Transitively blocked by
  [issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md) and
  [issue 01](/epic-0-plan-remediation/issues/01-epic-4-core-type-contracts.md).
- **Blocks**: [Issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md)
  — the size sweep must count 73 issues, not 71, and
  [issue 15](/epic-0-plan-remediation/issues/15-reconcile-github-state-with-the-files.md),
  which reconciles titles for the two new GitHub issues too.

## PR size note

`M` — two new ~120-line issue files plus edits across ten existing ones, ~350–500
changed lines. Split past ~500, and the seam is obvious: issue 05's split in one PR,
issue 08's in another. Land them in that order if you take it, since issue 05's halves
are what most of the epic depends on.

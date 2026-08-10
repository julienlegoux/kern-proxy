---
type: Issue
title: "Repair the Epic 4 issue set: provenance, the undeclared cross-epic blockers, and the renamed section heading"
description: "Restore Epic 4's // Ports: and docs/PORTING.md requirement to issues 05, 07 and 08, declare the three missing cross-epic blockers, and fix the renamed heading and the two wrong file citations."
tags: [epic-0]
timestamp: 2026-08-10T03:10:00Z
epic: 0
issue: 07
slug: repair-the-epic-4-issue-set
size: S
status: open
gh_issue: 205
resource: https://github.com/kern-ia/kern-link/issues/205
depends_on: [1, 2]
---

# Repair the Epic 4 issue set: provenance, the undeclared cross-epic blockers, and the renamed section heading

## Summary

Epic 4's issue set is the strongest in the bundle — ~60 sampled `file:line` citations
landed on the named declaration, and nine of twelve issues carry the epic's provenance
requirement. What remains is the residue: three issues that add new upstream symbols to
already-ported files without requiring the `// Ports:` header to still describe them,
three that drop their cross-epic blocker from the one section that can hold it, one
renamed required heading, and two references nested under the wrong filename.

Small, but the provenance ones compound: `upstream/sync.sh` and Epic 9's disposition
sweep navigate by those headers, so a stale symbol list makes the next sync read ported
code as unported.

Findings repaired: REPORT_4 P2 ×2 (provenance, cross-epic blockers), REPORT_4 P3 ×2
(the renamed heading, the `codex.go`/`websocket.go` citations). Epic 0 scope items 8, 9,
12 and 14; acceptance criteria 8, 9 and 13.

## Scope

**1. Provenance in issues 05, 07 and 08 (REPORT_4 P2).** `EPIC_4.md:56-57` requires that
"every file touched carries a `// Ports:` provenance header, and its disposition is
reflected in `docs/PORTING.md`". Nine issues carry it (`01:96-98`, `:137-138`, `03:81-82`,
`04:78`, `06:77-79`, `09:70-71`, `10:57`, `11:87-90`, `12:78-81`, `:121-122`); issues 05,
07 and 08 do not, and all three add new upstream symbols to already-ported files —
`openaicompletions.go`, `openairesponses/messages.go`, `openairesponses/stream.go`. Add
the one-line "keep the `// Ports:` header's symbol list current" bullet to each issue's
`## Scope` — reuse the wording already at `04:78` and `10:57`, do not invent a third —
and an acceptance-criterion checkbox wherever a `docs/PORTING.md` row actually changes.

**2. The three missing cross-epic blockers (REPORT_4 P2).** `depends_on` holds
intra-epic numbers only, so `## Dependencies` prose is the only place a cross-epic
blocker can live, and three issues drop theirs:

- `epic-4 issue 03:139-143` lists issue 01 only, while `03:51-52` calls
  `compat.SupportsOpenAIGrammarTools` — Epic 2 issue 04's work, named at `03:132-134`
  under *Relevant files* instead. Add "**Blocked by**: Epic 2 issue 04".
- `epic-4 issue 10:100-104` lists issues 06 and 08 only, while `10:24-29` reads
  `supportsOpenAIGrammarTools` / `supportsStrictMode`. Same addition.
- `epic-4 issue 11:124-129` lists 06/07/08 only, while `11:26-29` uses
  `SplitDeferredTools` and `11:32-35` populates `EndTurn`. Add "Epic 2 issue 02
  (`EndTurn`) and issue 11 (`SplitDeferredTools`)".

Their nine siblings already do this correctly (`01:159-164`, `02:131-138`, `04:138-141`,
`05:142-147`, `07:131-135`, `08:140-147`, `09:134-140`) — match that form.

**3. The renamed heading (REPORT_4 P3).** `epic-4 issue 12:83` reads
`## Out of scope, recorded rather than ported`. The issue schema and all eleven siblings
use `## Out of scope` verbatim; any consumer matching headings exactly reads issue 12 as
missing the section. Restore the heading and move "recorded rather than ported" into a
lead sentence beneath it.

**4. The two mis-attributed references (REPORT_4 P3).** `epic-4 issue 12:60-67` nests
"the failure recorder (`codexRecordWebSocketFailure`, `:164`) and the SSE-fallback
recorder (`:151`)" under a `ai/apis/codex/codex.go:` bullet; both are in
`ai/apis/codex/websocket.go` (`codex.go:164` is a bare `return`; the call site is
`codex.go:228`). The issue's own `## Relevant files` gets it right at `:133-135`.
Qualify both as `websocket.go:151` / `websocket.go:164`. While in these files, re-derive
any other `file:line` in a passage this PR edits.

## Out of scope

- **Implementing Epic 4.** No Go code.
- **The tool-choice field, the thinking-budget carrier and `ConstrainedSamplingConfig`**
  — [issue 01](/epic-0-plan-remediation/issues/01-epic-4-core-type-contracts.md), which
  lands first and owns Epic 4 issues 01, 04, 05, 09 and 11's contract lines. Where this
  issue also edits 05 or 11, it edits only the provenance and dependency sections.
- **The `src/utils/uuid.ts` orphan** (`epic-4 issue 12:37-43`) —
  [issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md).
- **Issues 09, 10 and 11's `size:` values** (REPORT_4 P2 sizing) and the boilerplate PR
  size notes — [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).
  In particular, issue 09's escape hatch at `:145-148` ("land the highest-value cases and
  open a follow-up for the remainder"), which negotiates away `EPIC_4.md:54-55`, is
  closed there as part of the size correction, not here.
- **The ten GitHub titles that dropped their colon** (REPORT_4 P3) and the `./` index
  links — [issue 15](/epic-0-plan-remediation/issues/15-reconcile-github-state-with-the-files.md)
  and [issue 14](/epic-0-plan-remediation/issues/14-normalize-links-depends-on-and-branch-notes.md).

## Acceptance criteria / Definition of done

- [ ] Every Epic 4 issue that edits or creates a ported file requires the `// Ports:`
      header to still describe it:
      `git grep -L '// Ports:' docs/epics/epic-4-openai-family-adapters/issues/*.md`
      returns no file that touches an existing ported source file.
- [ ] Issues 05, 07 and 08 each carry a `docs/PORTING.md` acceptance criterion where a
      row changes, and state "unchanged" explicitly where none does.
- [ ] Issues 03, 10 and 11 each name their Epic 2 blocker in `## Dependencies`, by epic
      and issue number.
- [ ] All twelve Epic 4 issues carry exactly the seven required `##` headings, verbatim
      and in order.
- [ ] `git grep -n 'codex.go:151\|codex.go:164' docs/epics/epic-4-*/` returns nothing,
      and every `file:line` in a passage this PR touches is re-derived at the PR's base
      commit.
- [ ] `timestamp` refreshed on every edited file; the GitHub bodies of #149, #151, #153,
      #154, #156 and #158 updated to match.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-repair-the-epic-4-issue-set`, PR targets `develop`, merge
      commit. Commit e.g. `docs(epics): restore Epic 4's provenance and blocker declarations`.

## Relevant files / areas

- `docs/epics/epic-4-openai-family-adapters/EPIC_4.md:56-57` — the provenance criterion.
- `.../issues/03-completions-grammar-custom-tools.md:51-52`, `:132-134`, `:139-143`.
- `.../issues/05-completions-deferred-tools-and-finish-reason.md:51-82`, `:96-127`.
- `.../issues/07-responses-shared-namespace-and-deferred-tools.md:50-77`, `:90-115`.
- `.../issues/08-responses-shared-stream-decode.md:55-81`, `:94-125`.
- `.../issues/10-azure-responses-wiring.md:24-29`, `:57`, `:100-104`.
- `.../issues/11-codex-request-body-and-stop-reasons.md:26-29`, `:32-35`, `:124-129`.
- `.../issues/12-codex-session-ids-and-continuation-retry.md:60-67`, `:83`, `:133-135`.
- `.../issues/04-completions-thinking-formats.md:78` — the canonical wording to reuse.
- Read-only: `ai/apis/codex/codex.go:164`, `:228`, `ai/apis/codex/websocket.go:151`,
  `:164`, `docs/PORTING.md`.

## Dependencies

- **Blocked by**: [Issue 01](/epic-0-plan-remediation/issues/01-epic-4-core-type-contracts.md)
  (edits Epic 4 issues 05 and 11) and
  [issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md) (edits
  `EPIC_4.md` and issue 12).
- **Blocks**: [Issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).

## PR size note

`S` — ~120–190 changed lines across eight issue bodies, most of them single added
bullets. Split past ~200; if it grows, the seam is (provenance, item 1) and (the rest).

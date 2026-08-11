---
type: Issue
title: "Repair the Epic 8 issues: provenance on ported tests, the unsatisfiable grep, and the two missing scope files"
description: "Stop exempting ported test files from the // Ports: header, narrow issue 05's count sweep to the files it owns, add the two files carrying the same stale counts, and resolve issue 04's stop-work branch."
tags: [epic-0]
timestamp: 2026-08-11T10:05:00Z
epic: 0
issue: 11
slug: repair-the-epic-8-issue-set
size: S
status: done
gh_issue: 209
gh_pr: 214
resource: https://github.com/kern-ia/kern-link/issues/209
depends_on: []
---

# Repair the Epic 8 issues: provenance on ported tests, the unsatisfiable grep, and the two missing scope files

## Summary

Epic 8's acceptance criterion 4 requires `// Ports:` headers on **every ported file**;
issue 01 narrows that to "every new **non-test** file" — and the epic's single most
explicitly ported artifact is a test file, `stream_test.go`, which `02:119-121` calls
"the port of `test/pi-messages.test.ts` (248 lines)". Since
`docs/planning/CONVENTIONS.md:63-66` makes header *absence* meaningful — it marks
original code — an unheadered `stream_test.go` does not merely omit provenance, it
actively misreports it, to the exact sweep Epic 9 runs against these headers.

Beside it: an acceptance criterion whose grep hits frozen decision records and can never
pass, two reader-facing files that keep claiming 35 providers after the epic ships 36+,
and a stop-work branch whose question the issue's own dependency already answers.

Findings repaired: REPORT_8 P1, P2 ×2 (the unsatisfiable grep, the missing scope files),
P3 ×5 (the `docs/PORTING.md` citations, "fetched at provider setup", AC 2's split
ownership, the `events.go` layout departure, issue 04's stop-work branch). Epic 0 scope
items 5, 6, 7, 8, 9, 12 and 14; acceptance criteria 5, 6, 7, 8, 9 and 13.

## Scope

**1. Provenance on ported test files (REPORT_8 P1).** Replace
`epic-8 issue 01:173-174`'s "every new **non-test** file" with "every new file that ports
upstream code, tests included"; name `stream_test.go` alongside `errors.go`/`stream.go`
at `epic-8 issue 02:196` with `// Ports: packages/ai/test/pi-messages.test.ts`; check
`03:151` and `04:159-160` for the same narrowing. `ai/providers/radius_config_test.go`
(`03:95`) **is** original code and correctly needs no header — leave it headerless with a
one-line note saying why, so absence stays meaningful. Repo practice backs this: 33
`_test.go` files already carry one (e.g. `ai/apis/anthropic/anthropic_test.go:3`,
`ai/catalog/compat_test.go:3`).

**2. Narrow the count sweep (REPORT_8 P2).** `epic-8 issue 05:110-113` requires
`grep -rn "nine wire adapters\|35 providers" docs/ README.md ai/doc.go` to return
"nothing stale". Run today it also hits `docs/planning/scope/06-adapter-updates.md:18`
and `:43`, `docs/planning/scope/10-new-provider-bindings.md:30` — frozen decision records
that describe the repo as it was when the plan was written — plus the issue file's own
`:4` and `:111`. The criterion is unsatisfiable, and read literally it points the
implementer at artifacts they must not edit. Scope the grep to the files the issue owns
(`README.md ai/doc.go docs/architecture.md docs/auth.md docs/usage.md
docs/planning/SPECS.md docs/planning/index.md`) and add one line stating that
`docs/planning/scope/` and `docs/epics/` are historical and exempt.

**3. The two missing scope files (REPORT_8 P2).** `epic-8 issue 05:72-78` names
`docs/architecture.md`, `ai/doc.go`, `README.md` and `docs/planning/SPECS.md`, and misses
`docs/planning/index.md:13` — whose bullet copies `SPECS.md:4`'s description verbatim,
including "across 35 providers … nine wire adapters over net/http" — and
`docs/usage.md:213` ("catalog (~35 providers)"). Its own `## Out of scope` (`:86-88`)
excludes only *Radius examples* in `docs/usage.md`, not the provider count. Add both to
`## Scope` and to the narrowed grep from item 2.

**4. The `docs/PORTING.md` citations (REPORT_8 P3).** All three row citations at
`epic-8 issue 05:124-127` point at the wrong rows and the table extent is short by four
lines: the table runs `:21-64`, `src/api/*.lazy.ts` is `:38`, `src/auth/helpers.ts` is
`:42`, `src/providers/*.ts` is `:57` (`:48` is `anthropic-messages`, `:59` is
`env-api-keys.ts`). Re-derive all four at the PR's base commit.

**5. "Fetched at provider setup" (REPORT_8 P3).** `EPIC_8.md:31-33` says Radius fetches
its gateway config "at provider setup" and words acceptance criterion 3 around that;
the issues put the fetch inside the `RefreshModels` pipeline behind `AllowNetwork`
(`03:21-26`, `04:36-53`) so construction touches no network — almost certainly the right
reading of upstream and of Epic 2's refresh contract, but said nowhere. Add one line to
`epic-8 issue 04`'s Summary stating that "provider setup" means the refresh pipeline and
that construction is deliberately network-free, citing `EPIC_8.md:31-33`; and amend
`EPIC_8.md:50-52`'s wording so a reviewer checking AC 3 against `04`'s tests does not read
a met criterion as missed.

**6. Acceptance criterion 2's split ownership (REPORT_8 P3).**
`epic-8 issue 02:26-27` declares AC 2 ("upstream's 248 test lines ported and passing
offline") "settled here", while `epic-8 issue 01:129-134` ports part of the same file and
`01:200-201` cites it as issue 01's own upstream source. Change issue 02 to "is completed
here — issue 01 ports the converter cases, this PR ports the transport and error cases",
and add the matching half-sentence to issue 01, so no case falls between the two PRs.

**7. The `events.go` layout departure (REPORT_8 P3).**
`epic-8 issue 01:96-97` introduces `ai/apis/pimessages/events.go` where
`docs/planning/CONVENTIONS.md:37-40` assigns translation to `messages.go`, and no adapter
package in the repo has an `events.go`. The name may well be better here — the unit
converts events, not messages — but issue 01 does not flag it, while issue 03 flags its
own smaller departure explicitly at `:63-68`. Either use `messages.go` or add one
sentence to issue 01's `## Scope` in the shape issue 03 already uses.

**8. Issue 04's stop-work branch (REPORT_8 P3).** `epic-8 issue 04:56-64` opens with
"one thing to verify before writing code": whether the landed `RefreshModelsContext`
exposes `rc.Publish`, with a "stop, say so in the PR body, and record it as a drift
record" branch. The artifact it depends on already answers:
`epic-2 issue 08:30-35` carries `publish(publication) => Promise<boolean>`, and `:70-72`,
`:87` add `RefreshModelsContext` to `ai` and pass it into `FetchModels(ctx, rc)`. Replace
the hedge with those citations, keeping a one-line "if the landed signature differs,
report it" fallback. Note that Epic 2 issue 08 is split by
[issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md) — cite
the half that lands `RefreshModelsContext`.

## Out of scope

- **Implementing Epic 8.** No Go code.
- **Re-grading issues 01 and 02 to `L`, and the five boilerplate PR size notes** —
  [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).
- **`docs/planning/` edits.** Item 2 narrows a grep *away* from the frozen decision
  records; issue 05 keeps its `docs/planning/SPECS.md` and `docs/planning/index.md` edits,
  which Epic 9 issue 04 (`:87`) explicitly cedes to it — that is a coordinated hand-off,
  not an invented one, and it is not converted to a drift record.
- **Answering REPORT_8's open question** — whether `EPIC_8.md:47`'s "248 lines" is a
  provenance statement or a literal line budget. Item 6 makes the split ownership explicit
  either way.
- **`ai/providers/radius_config.go`'s underscore filename.** REPORT_8 rejected it as a
  finding — `ai/providers/cloudflare_auth.go` is the existing precedent and issue 03
  already flags the departure at `:63-68`.

## Acceptance criteria / Definition of done

- [ ] `git grep -n 'non-test' docs/epics/epic-8-pi-messages-and-radius/` returns nothing,
      and every Epic 8 issue that creates a ported file — test files included — requires
      its `// Ports:` header; the one original test file is explicitly exempted with a
      reason.
- [ ] `epic-8 issue 05`'s count-sweep criterion passes when run against the tree today
      for reasons other than luck: it names only files the issue owns, and states the
      `docs/planning/scope/` and `docs/epics/` exemption.
- [ ] `epic-8 issue 05`'s `## Scope` names `docs/planning/index.md:13` and
      `docs/usage.md:213`.
- [ ] All four `docs/PORTING.md` citations in issue 05 are re-derived and correct, as is
      every other `file:line` in a passage this PR touches.
- [ ] `EPIC_8.md` contains no line its own issues contradict — in particular AC 3's
      "at setup" wording — and each amendment names the issue that forced it.
- [ ] Acceptance criterion 2 is claimed by one issue and its halves are assigned by name
      in both issues 01 and 02.
- [ ] `epic-8 issue 01` either renames `events.go` to `messages.go` or flags the
      departure from `docs/planning/CONVENTIONS.md:37-40` with its reason.
- [ ] `epic-8 issue 04` no longer opens with a conditional stop: the `rc.Publish`
      question is answered with a citation into the Epic 2 issue that lands it.
- [ ] `timestamp` refreshed on every edited file; GitHub bodies of #187–#191 and #125
      updated to match.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-repair-the-epic-8-issue-set`, PR targets `develop`, merge
      commit. Commit e.g. `docs(epics): require provenance on Epic 8's ported test files`.

## Relevant files / areas

- `docs/epics/epic-8-pi-messages-and-radius/EPIC_8.md:31-33`, `:44-47`, `:50-52`.
- `.../issues/01-pimessages-wire-and-converter.md:96-97`, `:129-134`, `:173-174`, `:200-201`.
- `.../issues/02-pimessages-stream-entry.md:26-27`, `:119-121`, `:196`.
- `.../issues/03-radius-gateway-config.md:21-26`, `:63-68`, `:95`, `:151`.
- `.../issues/04-radius-provider-binding.md:36-53`, `:56-64`, `:130-131`, `:159-160`.
- `.../issues/05-porting-and-docs.md:36-40`, `:72-78`, `:86-88`, `:110-113`, `:124-127`.
- `docs/epics/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md:30-35`,
  `:70-72`, `:87` — read, for item 8's citation.
- Read-only: `docs/PORTING.md:21-64`, `:38`, `:42`, `:57`, `docs/planning/index.md:13`,
  `docs/usage.md:213`, `docs/planning/SPECS.md:4`,
  `docs/planning/CONVENTIONS.md:37-40`, `:63-66`,
  `docs/planning/scope/06-adapter-updates.md:18`, `:43`,
  `docs/planning/scope/10-new-provider-bindings.md:30`,
  `ai/apis/anthropic/anthropic_test.go:3`, `ai/catalog/compat_test.go:3`.

## Dependencies

- **Blocked by**: None. Epic 8's files are touched by no earlier Epic 0 issue. Item 8
  cites Epic 2 issue 08, which
  [issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md)
  splits — cite whichever half lands `RefreshModelsContext` at the time this PR is
  written, and re-check the citation if issue 05 merges afterwards.
- **Blocks**: [Issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).

## PR size note

`S` — ~150–200 changed lines across seven files, concentrated in issue 05. Split past
~200; the seam is item 1 (provenance) versus items 2–8, which are all single-passage
corrections.

---
type: Issue
title: "Repair the Epic 7 issues: the postForm contract, the callbackHost rename, and the picker order"
description: "Give postForm a signature that can produce the error strings two issues pin, put codex.go in the callbackHost rename's blast radius, fix the login-picker order, and require provenance headers on the four ported test files."
tags: [epic-0]
timestamp: 2026-08-11T13:10:00Z
epic: 0
issue: 10
slug: repair-the-epic-7-issue-set
size: M
status: pr-open
gh_issue: 208
gh_pr: 224
resource: https://github.com/kern-ia/kern-link/issues/208
depends_on: [9]
---

# Repair the Epic 7 issues: the postForm contract, the callbackHost rename, and the picker order

## Summary

Three Epic 7 issues cannot be executed as written. `postForm`'s specified signature
returns a decoded `map[string]any`, from which the raw response text is unrecoverable —
so the `[: <body>]` suffixes that two of its consumers pin in their acceptance criteria
cannot be produced, and the issue that defines the helper puts widening it out of scope
two lines after instructing "extend it there in a small, separate commit". The
`callbackHost` rename names one file when the function already has two callers in two
files, so a PR that follows Scope literally leaves a package that does not compile. And
issue 05 sorts the login list and then appends Radius, while its own acceptance
criterion asserts the fully alphabetical order — the implementer writes the code from
Scope and the test from the AC, and the test fails on the first run.

Findings repaired: REPORT_7 P2 ×4 (the picker order, `postForm`, `callbackHost`, the
provenance half of AC 5), P3 ×3 (the "epic 12" citation, the `IsSubscription` premise,
the unlabelled Radius gateway constant, the two citation slips). Epic 0 scope items 5, 6,
8, 10 and 14; acceptance criteria 5, 6, 8, 10 and 13.

## Scope

**1. `postForm`'s signature (REPORT_7 P2).** `epic-7 issue 01:75-81` specifies
`postForm(ctx, url string, fields url.Values) (int, map[string]any, error)` with "a
decoded body on **every** status. A body that is not a JSON object decodes to an empty
map". `epic-7 issue 02:74-75`, `:84-85`, `:89-90` then pin
`Kimi Code device authorization failed with status <status>[: <body>]` and its token-
request sibling, and `epic-7 issue 04:113-115` pins
`Could not load Radius OAuth config from <gateway>: <status> <body>` — none of which the
helper can produce. Respecify it in issue 01's `## Scope` as
`postForm(...) (int, []byte, map[string]any, error)` (or a small struct) returning the
raw body alongside the decoded map, and delete `epic-7 issue 02:121-124`'s
contradictory out-of-scope bullet — with the widening in issue 01, issue 02 never has to
reopen `token.go`.

**2. The `callbackHost` rename (REPORT_7 P2).** `epic-7 issue 03:128-129` calls the
rename mechanical, names only `ai/auth/oauth/anthropic.go` (repeated at `:206`), and
tells the implementer to "update its doc comment to name **both** callers". The function
has two callers *before* this issue — `ai/auth/oauth/anthropic.go:105` and
`ai/auth/oauth/codex.go:515` — and three after. Add `ai/auth/oauth/codex.go` to `## Scope`
and `## Relevant files`, reword to "all callers (`anthropic.go:105`, `codex.go:515`, and
the new `openrouter.go`)", and extend the sequencing note at `:225-226`: Epic 6 issue 08
rewrites **both** `anthropic.go` and `codex.go` (`epic-6 issue 08:21-23`), not just the
first.

**3. The login-picker order (REPORT_7 P2).** `epic-7 issue 05:71-79` derives the list
from the registry "sorted by id" and then **appends** Radius, producing
`… openrouter, xai, radius`; `:106-108` asserts `… openrouter, radius, xai`. Change the
Scope bullet to "append the Radius entry, then sort the whole list by id" — the fix that
survives Epic 8 removing the special case — rather than weakening the criterion.

**4. Provenance on the four ported test files (REPORT_7 P2).** `EPIC_7.md:54-55` requires
`// Ports:` headers on **every ported file**, and `EPIC_7.md:29-32` counts 1056 lines of
upstream tests; every existing test file in `ai/auth/oauth` already carries a header
(`copilot_test.go:3`, `devicecode_test.go:3`, and five others). The four new `*_test.go`
files are asked for none — issues 01–04 demand the header on the flow `.go` file only
(`01:179-180`, `02:167-168`, `03:190-191`, `04:220-222`), and `epic-7 issue 06:127-130`
greps `// Ports: packages/ai/src/auth/oauth`, which by construction cannot match a test
file. Add the criterion to issues 01–04 (`// Ports: packages/ai/test/<flow>-oauth.test.ts`
on the new `_test.go`) and widen issue 06's grep to
`git grep -c "// Ports: packages/ai/test/.*-oauth.test.ts" ai/auth/oauth`.

**5. The unlabelled Radius gateway constant (REPORT_7 P3).**
`epic-7 issue 05:41-52`, `:75-78` declares `defaultRadiusGateway = "https://radius.pi.dev"`
in `cmd/pi-ai/oauth.go`, while `EPIC_7.md:41-43` puts the radius provider binding "and
its gateway config" in Epic 8. The issue argues the case well — AC 4 is otherwise
unmeetable, Epic 8 is named as the removal trigger, no flag or env var is invented — but
never labels it. Add one line flagging it as an assumption against `EPIC_7.md:41-43`,
taken because `EPIC_7.md:52` cannot be met without it, so an auditor does not have to
re-derive why it is fine.

**6. The two false premises and the citation drift (REPORT_7 P3 ×2).**
`epic-7 issue 01:22-24` cites "`PollDeviceCodeFlow`, ported in epic 12" — a number from
the retired build program; cite `ai/auth/oauth/devicecode.go` instead, or qualify it as
"the original build program's epic 12". `epic-7 issue 01:114-117` calls
`IsSubscription` and `LoginLabel` "the **two fields** [Epic 6 issue 01] lands unused";
`epic-6 issue 01:60-64` sets `IsSubscription` on three existing flows, so only
`LoginLabel` lands unused — and the same sentence repeats at `epic-7 issue 05:54-59`, so
fix both. Then `01:191` cites `ai/auth/oauth/token.go:24` for `httpClient` (it is `:22`),
and `03:41-42` writes `oauthSuccessHtml`/`oauthErrorHtml` where the Go symbols are
`oauthSuccessHTML`/`oauthErrorHTML` (`ai/auth/oauth/page.go:98`, `:104`) — which the same
issue gets right at `:203`.

**7. `EPIC_7.md`'s wording where its issues are right (REPORT_7 P3).** Where item 5's
assumption or item 2's blast radius makes an epic sentence read as failed against work
that is correct, amend the epic and name the issue that forced it — the same treatment
Epic 0 applies to `EPIC_1.md`, `EPIC_3.md`, `EPIC_5.md`, `EPIC_6.md`, `EPIC_8.md` and
`EPIC_9.md`.

## Out of scope

- **Implementing Epic 7.** No Go code.
- **Answering REPORT_7's open question about `ai/images/builtin.go`** — whether upstream's
  `test/openrouter-oauth.test.ts` really asserts that both providers expose OAuth
  (`epic-7 issue 03:124-127`). It needs an upstream read at `936aff00`. If the implementer
  has that checkout open for another Epic 0 issue, resolving it here is welcome; it is not
  a blocker, and the claim is already flagged as an assumption in the issue.
- **Re-grading issues 01–03 from `M` to `L`, and the six boilerplate PR size notes** —
  [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).
- **The `./` index links** —
  [issue 14](/epic-0-plan-remediation/issues/14-normalize-links-depends-on-and-branch-notes.md).
- **`EPIC_7.md:53`'s "completes them" versus issue 05's stop-at-selection criteria.**
  REPORT_7 raises it as an open question rather than a finding, and issue 05's offline
  choice is right; if item 7's epic amendment makes the sentence easy to qualify, do so,
  but do not reopen the testing approach.

## Acceptance criteria / Definition of done

- [ ] `epic-7 issue 01`'s `postForm` signature can produce every error string issues 02
      and 04 pin — verified by reading the three files against each other — and
      `epic-7 issue 02` no longer both forbids and mandates widening it.
- [ ] `epic-7 issue 03` names `ai/auth/oauth/codex.go` in both `## Scope` and
      `## Relevant files`, states three post-rename callers, and sequences against Epic 6
      issue 08 for both files.
- [ ] `epic-7 issue 05`'s `## Scope` and its acceptance criterion produce the same
      picker order, and it is the fully sorted one.
- [ ] Issues 01–04 each require a `// Ports:` header on the new `_test.go` file, and
      issue 06's grep can match a test-file header:
      `git grep -n 'packages/ai/test/.*-oauth.test.ts' docs/epics/epic-7-*/issues/`
      returns all five.
- [ ] `epic-7 issue 05` flags `defaultRadiusGateway` as an assumption against
      `EPIC_7.md:41-43`, with the reason.
- [ ] `git grep -n 'epic 12' docs/epics/` returns nothing unqualified, and no Epic 7
      issue claims Epic 6 issue 01 lands two unused fields.
- [ ] Every `file:line` in a passage this PR touches is re-derived at the PR's base
      commit; `token.go:22` and `oauthSuccessHTML`/`oauthErrorHTML` read correctly.
- [ ] `EPIC_7.md` contains no line its own issues contradict, and each amendment names
      the issue that forced it.
- [ ] `timestamp` refreshed on every edited file; GitHub bodies of #181–#186 and #124
      updated to match.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-repair-the-epic-7-issue-set`, PR targets `develop`, merge
      commit. Commit e.g. `docs(epics): make Epic 7's three unexecutable issues executable`.

## Relevant files / areas

- `docs/epics/epic-7-four-new-oauth-flows/EPIC_7.md:29-32`, `:41-43`, `:52`, `:54-55`.
- `.../issues/01-xai-device-code-oauth.md:22-24`, `:75-81`, `:114-117`, `:179-180`, `:191`.
- `.../issues/02-kimi-coding-device-code-oauth.md:74-75`, `:84-85`, `:89-90`, `:121-124`,
  `:167-168`.
- `.../issues/03-openrouter-pkce-oauth.md:41-42`, `:128-129`, `:190-191`, `:203`, `:206`,
  `:225-226`.
- `.../issues/04-radius-gateway-oauth.md:113-115`, `:220-222`.
- `.../issues/05-cli-login-new-flows.md:41-52`, `:54-59`, `:71-79`, `:75-78`, `:106-108`.
- `.../issues/06-auth-docs-and-porting.md:127-130`.
- `docs/epics/epic-6-auth-core-and-env-api-key-bindings/issues/08-anthropic-codex-login-race.md:21-23`
  and `.../issues/01-auth-contract-surface.md:60-64` — read, for the sequencing note and
  the `IsSubscription` correction.
- Read-only: `ai/auth/oauth/anthropic.go:44`, `:105`, `ai/auth/oauth/codex.go:515`,
  `ai/auth/oauth/token.go:22`, `ai/auth/oauth/page.go:98`, `:104`,
  `ai/auth/oauth/devicecode.go:5`, `ai/auth/oauth/copilot_test.go:3`.

## Dependencies

- **Blocked by**: [Issue 09](/epic-0-plan-remediation/issues/09-amend-epic-6-and-close-its-gaps.md)
  — item 2's sequencing note points at Epic 6 issue 08, which issue 09 edits.
- **Blocks**: [Issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).

## PR size note

`M` — ~250–450 changed lines across eight files. Split past ~500. The clean cut: items
1–3 (the three unexecutable issues) first, items 4–7 (provenance, the assumption label,
the citations and the epic amendment) second — item 1 is the one to land early, since it
changes a shared helper's contract.

---
type: Issue
title: "Amend EPIC_6.md to absorb the work its issues build, and close Epic 6's ownership and dependency gaps"
description: "Add the four upstream files four Epic 6 issues build to EPIC_6.md's scope, declare the two undeclared conflicts, give acceptance criteria 2, 4 and 5 owners, and fix the three internal inconsistencies."
tags: [epic-0]
timestamp: 2026-08-11T16:10:00Z
epic: 0
issue: 09
slug: amend-epic-6-and-close-its-gaps
size: M
status: done
gh_issue: 207
gh_pr: 221
resource: https://github.com/kern-ia/kern-link/issues/207
depends_on: [2, 8]
---

# Amend EPIC_6.md to absorb the work its issues build, and close Epic 6's ownership and dependency gaps

## Summary

`EPIC_6.md:27-28` names exactly six upstream files. Four of the epic's eleven issues
build content from four more — `src/models.ts` (issues 05 and 06) and the three
`src/auth/oauth/*.ts` flows (issues 08 and 09) — and issue 08 opens by asserting the
opposite of `EPIC_6.md:41-42`: the epic says the directory move is "churn with no
content behind it", issue 08 says upstream moved the files "**with edits**" and then
deletes a branch, a helper and an options field from each flow.

Roughly a third of the epic's PRs therefore land work no epic acceptance criterion
verifies. Epic 0 has already decided the direction (`EPIC_0.md`, `## Notes`): the epic
**absorbs** the four files rather than moving issues 08 and 09 to Epic 7 — the work is
real, correctly placed and traceable, since Epic 2 issue 08 explicitly deferred the
`Models` auth surface here. It is the epic file that never recorded the hand-off.

Findings repaired: REPORT_6 P1, P2 ×5 (AC 4's catalog half, the two undeclared
conflicts, the #169 contract, AC 5's shared test file, `docs/auth.md`'s routing — the
last of which [issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md)
owns), P3 ×3 (AC 2 unverified, `// Ports:` in four issues, the issue 07/11 PORTING
mismatch, the three internal inconsistencies). Epic 0 scope items 5, 7, 8, 9, 10 and 14;
acceptance criteria 5, 7, 8, 9, 10 and 13.

## Scope

**1. Absorb the four files into `EPIC_6.md` (REPORT_6 P1).** Add `src/models.ts` and the
three `src/auth/oauth/*.ts` files to `## Scope`; add two acceptance criteria — "the
anthropic and codex login flows always race the manual-code prompt" and "the `Models`
auth surface Epic 2 deferred here is complete" — and rewrite `EPIC_6.md:41-42` to say
what it means: the *directory move* is a no-op, the *content edits inside the moved
files* are in scope. Cite Epic 2 issue 08 (`:96-106`), which is where
`filterModels`/`getAvailable`/`checkAuth`/`login` were deferred here, so the hand-off is
recorded. Name issues 08, 09, 05 and 06 as the amendment's cause.

**2. Acceptance criterion 4's catalog half (REPORT_6 P2).** `EPIC_6.md:52-53` requires
the four env-API-key bindings to resolve credentials **and each have a catalog entry**;
`epic-6 issue 10:43-50`, `:70-72` explicitly excludes catalog data, delegating to Epic 3
issue 04 (#144), and its criteria at `:85-97` check registration, env resolution and
base URLs only. Add `TestNewBindingsServeCatalogModels` to issue 10 — each of the four
ids returns a non-empty `catalog.BuiltinModels(id)`, skipped-and-named in the PR body if
#144 has not merged — or move the catalog clause into `EPIC_6.md`'s `## Dependencies` on
Epic 3. Pick one; do not leave the criterion claiming work the epic does not do.

**3. Declare the two conflicts `depends_on` under-states (REPORT_6 P2).** Issues 05 and
06 both rewrite the `Models` interface and `modelsImpl` in `ai/provider.go`
(`05:79-88`, `06:70-77`), and issue 06 says so in prose at `:155-156` — but `06:14` reads
`depends_on: [1, 3]`, so a supervisor runs them concurrently into a guaranteed conflict.
Issue 11 reconciles `docs/PORTING.md` after issues 03 and 09 amend it, says "**Land this
last**" at `:76`, and declares `depends_on: []`. Set `06` to `[1, 3, 5]` and `11` to
`[3, 9]` — both backwards references, so numeric order still holds.

**4. Acceptance criterion 5 and the shared upstream test file (REPORT_6 P2).**
`EPIC_6.md:54` requires ported upstream tests to come across. Only issue 08 turns it into
a requirement (`:84-87`); elsewhere the upstream test file is a *Relevant files* pointer
and the criteria list freshly invented Go tests instead. Worse,
`test/oauth-auth.test.ts` is named by three issues (`01:141`, `04:147`, `05:160`), so
each can assume another covers it. Add one acceptance criterion per issue naming its
upstream test file and requiring every case to be represented or explicitly
dispositioned, and split `test/oauth-auth.test.ts` explicitly — types/`Check` cases → 01,
expiry-window cases → 04, `checkAuth`/`getAvailable` cases → 05 — so no case is
everyone's and nobody's. Same treatment for `test/anthropic-auth-token.test.ts`
(`07:149-150`) and `test/github-copilot-oauth.test.ts` (`09:163`).

**5. Acceptance criterion 2's missing check (REPORT_6 P3).** `EPIC_6.md:49` ("no Go
package moved or renamed") is verified nowhere, and issues 08 and 09 are exactly the PRs
where "tidying" `ai/auth/oauth` to match upstream's new layout tempts. Add to issue 11
(which lands last and already runs grep-shaped checks):
`git diff --name-status <epic-base>..HEAD -- ai/auth/` shows no `R` entries.

**6. `// Ports:` maintenance in the four issues that lack it (REPORT_6 P3).** Required by
`01:120-125`, `05:144-146`, `07:133-135`, `08:80-83` and `:124`, `09:89-91`, `10:66`;
absent from issues 02 (edits `ai/auth.go`, `ai/credentialstore.go`,
`ai/auth/filestore.go`), 03 (`ai/auth.go`, `ai/resolve.go`, `ai/provider.go`,
`ai/providers/cloudflare_auth.go`), 04 (`ai/resolve.go`) and 06 (`ai/provider.go`,
`cmd/pi-ai/oauth.go`). Issue 03 changes what `ai/resolve.go` ports and issue 06 changes
what `cmd/pi-ai/oauth.go` does. Add the same one-line DoD item to all four.

**7. The #169/#172 contract, Epic 6's side (REPORT_6 P2).**
[Issue 08](/epic-0-plan-remediation/issues/08-reconcile-epic-5-with-its-issues.md) amends
Epic 5 issue 11's criterion; here, make `epic-6 issue 03:43-45`'s reciprocal action
explicit — it already names it — so both sides record the same contract in compatible
words.

**8. Issue 11's wrong attribution and the three small inconsistencies (REPORT_6 P3 ×2).**
`11:74-76` says issues 03, 07 and 09 each touch `docs/PORTING.md`; issue 07 explicitly
does not (`07:59-62`), and the `src/env-api-keys.ts` row (`docs/PORTING.md:59`) is the
one row issue 07 makes newly load-bearing. Change to "issues 03 and 09", and add the
`env-api-keys.ts` row rewrite to issue 11's own scope. Then: `01:23-24` says "the four
issues that consume it (05, 06, 08, 09)" while `:146-150` lists five including 03 — make
them agree; `10:47-48` claims `catalog.BuiltinModels(id)` "returns an empty slice" where
`ai/catalog/catalog.go:84-89` returns **nil** (its own doc comment says so) — a test
written to the stated claim would fail, so say nil; `01:43` reaches `docs/PORTING.md` as
`../../../../docs/PORTING.md` where its siblings use `../../../PORTING.md` — use the
sibling form.

## Out of scope

- **Implementing Epic 6.** No Go code.
- **Moving issues 08 and 09 to Epic 7.** Decided against in `EPIC_0.md`'s `## Notes`;
  item 1 takes the absorb path.
- **`docs/auth.md`'s env-key table and resolution-order prose** —
  [issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md) brings
  them into issue 11's scope, and lands first.
- **Issue 08's `size: M` → `L` (or its 08a/08b split), issue 02's `S`, and the eleven
  boilerplate PR size notes** —
  [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md). Epic 0
  splits only Epic 2's two issues, and only because those two name their own seam.
- **Answering REPORT_6's open questions** — whether `src/auth/helpers.ts` has an in-range
  delta beyond issue 07's one line, and whether the `FilterModels`-on-Copilot decision at
  `09:68-75` is binding. Both need an upstream read this issue does not require; if the
  implementer has `936aff00` open for item 1, answering them is welcome, and either
  answer belongs in the issue body it settles.

## Acceptance criteria / Definition of done

- [ ] `EPIC_6.md`'s `## Scope` names all ten upstream files its issues build, its
      `## Out of scope` no longer asserts the opposite of issue 08, and the two new
      acceptance criteria cover the OAuth-flow simplification and the `Models` auth
      surface. Each amendment names the issue that forced it.
- [ ] Every `EPIC_6.md` acceptance criterion has an owning issue that can satisfy it
      from Epic 6's own work, or is explicitly delegated in `## Dependencies` — checked
      criterion by criterion, 1 through 6.
- [ ] `epic-6 issue 06:14` reads `depends_on: [1, 3, 5]` and `issue 11:14` reads
      `[3, 9]`; `depends_on` and the `## Dependencies` prose agree in both directions for
      all eleven issues.
- [ ] Each of the three shared upstream test files has its cases assigned by issue
      number, and every issue naming an upstream test file carries a criterion requiring
      each case to be represented or dispositioned.
- [ ] Issues 02, 03, 04 and 06 each carry the `// Ports:`-still-describes-the-file DoD
      item.
- [ ] `epic-6 issue 11` carries the `ai/auth/` rename check and the
      `src/env-api-keys.ts` row rewrite, and no longer attributes a `docs/PORTING.md`
      edit to issue 07.
- [ ] The three inconsistencies are fixed: issue 01's consumer count agrees with its own
      blocked list, `catalog.BuiltinModels` is described as returning nil, and
      `01:43` uses `../../../PORTING.md`.
- [ ] Epic 5 issue 11 and Epic 6 issue 03 state the same contract about auth-time
      resolution, in both files.
- [ ] `timestamp` refreshed on every edited file; GitHub bodies of #170–#180 and #123
      updated to match.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-amend-epic-6-and-close-its-gaps`, PR targets `develop`, merge
      commit. Commit e.g. `docs(epics): absorb into EPIC_6 the work its issues build`.

## Relevant files / areas

- `docs/epics/epic-6-auth-core-and-env-api-key-bindings/EPIC_6.md:27-28`, `:40-42`,
  `:49`, `:52-54`, `:73-75`.
- `.../issues/01-auth-contract-surface.md:23-24`, `:43`, `:141`, `:146-150`.
- `.../issues/02-credential-store-list.md`, `.../issues/04-oauth-refresh-window.md:147`.
- `.../issues/03-provider-scoped-apikey-resolution.md:36-54`, `:85`, `:134-136`.
- `.../issues/05-models-availability.md:29-66`, `:79-88`, `:160`.
- `.../issues/06-models-login-logout.md:14`, `:68-86`, `:155-156`.
- `.../issues/07-anthropic-auth-token.md:59-62`, `:149-150`.
- `.../issues/08-anthropic-codex-login-race.md:21-22`, `:63-87`.
- `.../issues/09-copilot-model-availability.md:52-75`, `:163`.
- `.../issues/10-env-api-key-bindings.md:43-50`, `:47-48`, `:70-72`, `:85-97`.
- `.../issues/11-porting-paths-and-dispositions.md:14`, `:74-76`, `:76`, `:95-102`.
- `docs/epics/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md:96-106`
  (read, for the hand-off citation) and
  `docs/epics/epic-5-remaining-adapters/issues/11-cloudflare-stream-classification.md:118-119`.
- Read-only: `ai/provider.go`, `ai/resolve.go`, `ai/catalog/catalog.go:84-89`,
  `docs/PORTING.md:59`, `ai/providers/all.go`.

## Dependencies

- **Blocked by**: [Issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md)
  (edits Epic 6 issue 11's scope and out-of-scope) and
  [issue 08](/epic-0-plan-remediation/issues/08-reconcile-epic-5-with-its-issues.md) (writes
  Epic 5's half of the #169/#172 contract, which item 7 answers).
- **Blocks**: [Issue 10](/epic-0-plan-remediation/issues/10-repair-the-epic-7-issue-set.md)
  — Epic 7 issue 03's `callbackHost` rename must be sequenced against Epic 6 issue 08's
  rewrite of both `anthropic.go` and `codex.go`, and that sequencing note is written
  here first; and [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).

## PR size note

`M` — ~300–500 changed lines across thirteen files. Split past ~500. The clean cut:
items 1–3 (the epic amendment and the graph) first, items 4–8 (the per-issue criteria and
corrections) second.

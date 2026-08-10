---
type: Issue
title: "Reconcile EPIC_5.md with its issues, and settle the five criteria that pass whichever way the implementer decides"
description: "Record the Epic 4 dependency seven Epic 5 issues carry, make acceptance criterion 3 checkable, restore the docs/PORTING.md half to seven issues, and collapse the either-way criteria to one expected behaviour each."
tags: [epic-0]
timestamp: 2026-08-10T03:10:00Z
epic: 0
issue: 08
slug: reconcile-epic-5-with-its-issues
size: M
status: open
gh_issue: 206
resource: https://github.com/kern-ia/kern-link/issues/206
depends_on: []
---

# Reconcile EPIC_5.md with its issues, and settle the five criteria that pass whichever way the implementer decides

## Summary

`EPIC_5.md:69-70` declares Epic 5 "independent of Epic 4; the two can run concurrently
once epic 2 has merged". Seven of its eleven issues contradict that — four declare Epic
4 issue 01 (#147, which creates `ai/apis/internal/grammar`) a hard blocker and three
inherit it through the intra-epic edges 03→04, 05→06, 07→08. `epic-5 issue 03:160-163`
states the contradiction outright. `depends_on` cannot carry a cross-epic edge, so
nothing mechanical catches it: anyone scheduling from the epic starts Epic 5 alongside
Epic 4 and stalls on seven of eleven issues.

Beside it sit five acceptance criteria that are satisfied by either outcome — port or
deviate, honor or reject `Fetch` — which means the checkbox proves only that something
was written down. The sharpest is custom `Fetch`: Epic 4 issue 02 honors it for the
OpenAI family, so a "port the rejection" outcome in Epic 5 issue 06 leaves the library
with two different `Fetch` contracts across adapters.

Findings repaired: REPORT_5 P1, P2 ×3 (AC 3, the `docs/PORTING.md` half, the five
either-way criteria), P3 ×3 (the false "no epic owns it" claims, the SPECS.md edit, the
two unamended out-of-scope crossings, the three stale citations). Epic 0 scope items 5,
7, 8, 10, 11 and 14; acceptance criteria 5, 7, 8, 10, 11 and 13.

## Scope

**1. Record the Epic 4 dependency (REPORT_5 P1).** Amend `EPIC_5.md:65-70` to name Epic
4 issue 01 as a prerequisite for the seven issues that need
`ai/apis/internal/grammar` + `ResolveJSONSchemaStrictSampling`, keeping the "wire work
is independent" half that is true, and update the Epic 5 line in `docs/epics/index.md`
if it states an edge. The issue bodies (`03:157-163`, `05:159-163`, `07:155-161`,
`09:170-178`, and transitively `04:189-190`, `06:156-157`, `08:193-194`) are correct as
written — the epic is the side that is wrong. Name issue 03 as the amendment's cause.

**2. Make acceptance criterion 3 checkable (REPORT_5 P2).** `EPIC_5.md:57` requires
`cloudflare-stream.ts` to have "a recorded classification **and a Go home**", while
`epic-5 issue 11:72-73` allows "record it as a deviation" and prefixes its three
implementation tests with "If ported" (`:102-117`), leaving a `docs/PORTING.md` row as
the only unconditional criterion. #169 can then close green with AC 3 unmet — and Epic 6
issue 03 (#172) has already written down what that costs: Cloudflare requests shipping
with a literal `{CLOUDFLARE_ACCOUNT_ID}` in the URL, "a regression, not a deviation".
Make the port the required outcome in issue 11's `## Scope`, drop the "If ported —"
prefixes, and delete the deviation branch. Do not move the choice into the epic; decide
it.

**3. Restore the `docs/PORTING.md` half (REPORT_5 P2).** `EPIC_5.md:60-61` requires every
touched file to carry a `// Ports:` header **and** be dispositioned in
`docs/PORTING.md`. Seven issues close their `## Scope` with only the header half —
`02:109`, `03:96-97`, `04:110`, `05:100`, `07:94`, `09:94`, `10:107` — and none carries a
`docs/PORTING.md` line in its acceptance criteria. Add one criterion to each, naming its
upstream file, e.g. "`docs/PORTING.md`'s row for `src/api/anthropic-messages.ts` still
describes the Go code after this change", including where the expected outcome is
"unchanged".

**4. Collapse the five either-way criteria (REPORT_5 P2).** Decide each one, put the
deciding reason in `EPIC_5.md`'s `## Notes`, and rewrite the criterion to name the
single expected behaviour:

- `06:128-130` — `TestGoogleHonorsInjectedFetch` **or** `TestGoogleRejectsCustomFetch`.
  Decide: honor `Fetch`, matching Epic 4 issue 02, so the library has one `Fetch`
  contract. Issue 06 already recommends this at `:70-72`.
- `08:145-149` — adopt the 60s timeout **or** record a `PORTING.md` deviation.
- `09:88-91` — "leave a comment saying so, or match upstream's two-lookup shape".
- `02:85-88` — "decide, implement one way, and assert it in a test", with no stated
  expected outcome.
- `11:72-73` / `:99-117` — port **or** deviate; settled by item 2.

`EPIC_5.md:88-89`'s governing principle already rules the tie-break: a deviation must be
justified by structural non-portability, never by cost. Nothing in these five is
structurally non-portable.

**5. Correct the two false cross-epic claims and the unrecorded conflict (REPORT_5 P3).**
`02:116-121` says `ANTHROPIC_AUTH_TOKEN` bearer-header work has "no epic in this program
currently owns it" — Epic 6 issue 07 (#176) is exactly that work. `11:86-90` says nothing
claims `src/providers/*.ts` updates beyond Epic 6's four new bindings — Epic 6 issue 03
(#172) is exactly the `cloudflare-auth.ts` per-field credential/env merge. Both were true
when written (Epic 5's issues predate Epic 6's) and are false now. Replace each with a
pointer to the owning issue number. And `11:118-119` requires that "auth-time resolution
is not removed", which #172 explicitly removes: amend it to "auth-time resolution stays
until Epic 6 issue 03 (#172) removes it; the dispatch-time path must not depend on it",
so the conflict is recorded on both sides rather than only Epic 6's.

**6. Amend the two out-of-scope crossings and the invented planning edit (REPORT_5 P3
×2).** `EPIC_5.md:43` ("the OpenAI-family adapters — epic 4 owns those") and `:44-45`
(the classifier audit) both read as absolutes while `01:85-86`, `01:120-121` and
`08:80-83` cross them for good reasons. Qualify both: "except the Copilot header
builders, which issue 01 owns end to end", and "except where this epic's own change to
an error string forces a classifier check in the same PR". Separately, `01:89-90` asks
for an edit to `docs/planning/SPECS.md:340` that `EPIC_5.md:54-56` never authorized —
drop it, keeping the `docs/PORTING.md:46` row change that AC 2 does ask for. If the
SPECS line genuinely needs to move, it goes through a drift record, not an
implementation PR.

**7. Fix the three citations (REPORT_5 P3).** `01:134` cites
`ai/apis/anthropic/anthropic.go:1193` for `buildHeaders` (it is `:1192`); `10:165-168`
cites `ai/apis/bedrock/clientauth.go:123`, `:132`, `:147` (they are `:124`, `:133`,
`:148`); `01:138-139` calls `ai/apis/internal/grammar` an "existing precedent" when
`ai/apis/internal/` holds only `httpretry` — the package is created by Epic 4 issue 01.
Reword to "the package Epic 4 issue 01 creates", and re-derive every offset in a passage
this PR touches.

## Out of scope

- **Implementing Epic 5.** No Go code, no adapter change.
- **Moving `ai/apis/internal/grammar` into Epic 2.** REPORT_5's open question raises it;
  Epic 0's scope is the reports' findings, and the finding is the *undeclared* edge, not
  the package's home. Item 1 records the edge where it is.
- **Relocating `ai/apis/internal/httpretry`** — decided against in `EPIC_0.md`'s
  `## Notes` and enforced by
  [issue 06](/epic-0-plan-remediation/issues/06-rein-in-epic-3-invented-scope.md).
- **Issue 01's `size: S` → `M`, issues 09/10's possible `L`, and the ten boilerplate PR
  size notes** — [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).
- **Recording the branch and status-commit conventions in `EPIC_5.md`'s `## Notes`**
  (REPORT_5 P3) — [issue 14](/epic-0-plan-remediation/issues/14-normalize-links-depends-on-and-branch-notes.md)
  writes that line into all nine epics at once.
- **Epic 6 issue 03's side of the #169/#172 conflict** — this issue writes Epic 5's half
  and [issue 09](/epic-0-plan-remediation/issues/09-amend-epic-6-and-close-its-gaps.md)
  writes Epic 6's, in that order.

## Acceptance criteria / Definition of done

- [ ] `EPIC_5.md` contains no scope, out-of-scope or acceptance-criterion line its own
      issues contradict, and each amendment names the issue that forced it.
- [ ] `EPIC_5.md`'s `## Dependencies` names Epic 4 issue 01 and the seven issues that
      need it.
- [ ] No acceptance criterion in Epic 5's eleven issues is satisfied by two opposite
      outcomes: `git grep -nE '\bor\b.*(matching whichever|If ported|record it as a
      deviation)' docs/epics/epic-5-*/issues/` returns nothing, and each of the five
      decisions has its reason in `EPIC_5.md`'s `## Notes`.
- [ ] All eleven issues carry a `docs/PORTING.md` acceptance criterion naming their
      upstream file, including the seven that had only the `// Ports:` half.
- [ ] `git grep -n 'No epic in this program' docs/epics/` returns nothing that a live
      issue number contradicts, and `epic-5 issue 11:118-119` and Epic 6 issue 03 state
      the same contract in compatible terms.
- [ ] `git grep -n 'docs/planning/SPECS.md' docs/epics/epic-5-*/issues/` shows no
      merge-blocking edit criterion.
- [ ] Every `file:line` in a passage this PR touches is re-derived at the PR's base
      commit; `anthropic.go:1192` and `clientauth.go:124/:133/:148` read correctly, and
      `ai/apis/internal/grammar` is described as forthcoming, not existing.
- [ ] `timestamp` refreshed on every edited file; GitHub bodies of the edited issues
      (#159, #160, #162, #163, #164, #165, #166, #167, #168, #169) and #122 updated to
      match; the correction to #169's criterion posted as a comment on #169, as Epic 6
      issue 03 already promises reciprocally.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-reconcile-epic-5-with-its-issues`, PR targets `develop`, merge
      commit. Commit e.g. `docs(epics): reconcile EPIC_5 with its issues and settle its open criteria`.

## Relevant files / areas

- `docs/epics/epic-5-remaining-adapters/EPIC_5.md:43`, `:44-45`, `:52-61`, `:65-70`,
  `:88-89`, `## Notes`.
- `.../issues/01-copilot-dynamic-headers.md:85-91`, `:89-90`, `:120-121`, `:134`,
  `:138-139`.
- `.../issues/02-anthropic-stream-lifecycle.md:85-88`, `:109`, `:116-121`.
- `.../issues/03-anthropic-strict-tools-and-signed-thinking.md:96-97`, `:157-163`.
- `.../issues/04-anthropic-deferred-tools.md:110`, `:189-190`.
- `.../issues/05-google-shared-converters.md:100`, `:159-163`.
- `.../issues/06-google-and-vertex-stream-and-params.md:70-72`, `:128-130`, `:156-157`.
- `.../issues/07-mistral-stop-reasons-and-strict-tools.md:94`, `:155-161`.
- `.../issues/08-mistral-wire-and-header-parity.md:80-83`, `:145-149`, `:193-194`.
- `.../issues/09-bedrock-stop-reasons-strict-tools-and-claude-5.md:88-91`, `:94`, `:170-178`.
- `.../issues/10-bedrock-credentials-and-diagnostics.md:107`, `:165-168`.
- `.../issues/11-cloudflare-stream-classification.md:72-73`, `:86-90`, `:99-117`, `:118-119`.
- `docs/epics/index.md`, and `docs/epics/epic-6-.../issues/03-provider-scoped-apikey-resolution.md:36-54`
  (read, for the reciprocal wording).
- Read-only: `ai/apis/anthropic/anthropic.go:1192`, `ai/apis/bedrock/clientauth.go:124`,
  `:133`, `:148`, `ai/apis/internal/`, `docs/planning/SPECS.md:340`, `docs/PORTING.md:46`.

## Dependencies

- **Blocked by**: None. Epic 5's files are touched by no earlier Epic 0 issue.
- **Blocks**: [Issue 09](/epic-0-plan-remediation/issues/09-amend-epic-6-and-close-its-gaps.md)
  — the #169/#172 contract is written on Epic 5's side here and Epic 6's side there, and
  landing them in the other order leaves the pair disagreeing again; and
  [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).

## PR size note

`M` — ~300–500 changed lines across thirteen files; eleven issue bodies each take one to
three edits. Split past ~500. The clean cut: items 1–3 (the epic-level reconciliation)
first, items 4–7 (the per-issue criteria and citations) second.

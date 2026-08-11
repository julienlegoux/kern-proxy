---
type: Epic
title: "Plan remediation"
description: "Repair the 97 findings the nine issue-review reports raised against the epic and issue bundle, so the sync program implements against a plan that agrees with itself."
tags: [epic, remediation]
timestamp: 2026-08-11T21:30:00Z
epic: 0
slug: plan-remediation
status: open
gh_issue: 198
milestone: 27
resource: https://github.com/kern-ia/kern-link/issues/198
source: docs/REPORT_1.md, docs/REPORT_2.md, docs/REPORT_3.md, docs/REPORT_4.md, docs/REPORT_5.md, docs/REPORT_6.md, docs/REPORT_7.md, docs/REPORT_8.md, docs/REPORT_9.md
---

# Epic 0: Plan remediation

## Goal

Nine reviews read the 71 issues of epics 1–9 against their epics and the plan, and
raised 97 findings. None of them is about code — nothing has been implemented yet.
They are about the plan: acceptance criteria that cannot be satisfied in Go, public
API fields three issues assume exist and none creates, scope named in an epic that no
issue anywhere owns, epics whose own issues correctly contradict them, and one
boilerplate sentence copied into 70 of 71 issue bodies.

Implementing against that plan means discovering each defect mid-PR, at the point
where the cheap fix — editing a markdown file — has become an expensive one: an
invented public symbol in a released `v0.2.0`, a criterion ticked that is false, a
sibling epic rebased around a decision nobody recorded. This epic lands the whole
repair as documentation work, before Epic 1 opens its first branch.

It is scoped to the reports' own findings. It does not re-review, does not re-grade
severities, and does not implement any of the work the repaired issues describe.

## Scope

Seventeen repairs, ordered so that the ones changing a contract land before the ones
written against it.

**Contracts and ownership**

1. **Name and assign the two core-type fields three Epic 4 issues assume exist.**
   There is no OpenAI-family tool-choice field on `ai.StreamOptions` and no issue
   creates one, yet Epic 4 issues 05, 09 and 11 each wire it; `ThinkingBudgets` lives
   on `SimpleStreamOptions`, which the `buildParams` path Epic 4 issue 04 targets
   never sees. Decide each field's Go name and owning issue, then wire the
   dependencies. (REPORT_4 P1 ×2)
2. **Settle where `ConstrainedSamplingConfig` lands.** `EPIC_4.md:52-53` and
   `docs/planning/scope/12-constrained-sampling.md` say `ai.StreamOptions`; Epic 4
   issue 01 reads upstream `936aff00` as `Tool.constrainedSampling` and touches
   `ai.Tool`. Verify against upstream, then amend whichever side is wrong — including
   the scope decision, if it is the one that moves. (REPORT_4 P2)
3. **Adopt the six orphans.** The `ImagesOptions` → `ProviderRequestOptions` reshape
   is deferred in a circle between Epic 2 issue 05 and Epic 3 issues 05/06, so nobody
   builds it; `images-models.ts`'s `getAuth` reshape is deferred to an epic that
   disclaims it; deferred-response adapter work is pointed at Epics 4 and 5, which
   cover deferred *tools* only; `SimpleStreamOptions.deferred`, `src/utils/uuid.ts`
   and `docs/auth.md`'s env-key table have no owner at all. Give each an owner or an
   explicit, reasoned `## Out of scope` line. Two need a check against upstream
   `936aff00` first. (REPORT_2 P2 ×2, REPORT_3 P1+P2, REPORT_4 P2, REPORT_6 P2)
4. **Authorize or remove the invented scope.** Epic 3 issue 06 relocates
   `ai/apis/internal/httpretry` repo-wide — contradicting `CONVENTIONS.md:22-26` and
   colliding with the four adapter packages Epics 4 and 5 rewrite; Epic 9 issue 03 is
   a whole PR of shell tooling, a CI step and a permanent change to the documented
   sync procedure that `EPIC_9.md` never asks for. Plus a `MODEL_CATALOG_DIR`
   configuration surface, an unrequested invariant battery, one issue that makes
   rewriting its four successors part of its own definition of done, and a
   dev-process document placed in the consumer-facing bundle.
   (REPORT_3 P1+P3 ×3, REPORT_9 P2+P3)

**Epics and their issues**

5. **Amend the epic files their issues correctly diverged from.** `EPIC_5.md` declares
   independence from Epic 4 that seven of its eleven issues contradict; `EPIC_6.md`'s
   Scope omits four upstream files four of its issues build, and its Out of scope
   asserts the opposite of what issue 08 does; `EPIC_1.md`'s AC 2 and AC 4,
   `EPIC_3.md`'s "strictly in this order", and wordings in `EPIC_7.md`, `EPIC_8.md`
   and `EPIC_9.md` all read as failed against work that is right.
   (REPORT_1 P2 ×2, REPORT_3 P3, REPORT_5 P1+P3, REPORT_6 P1+P2, REPORT_7 P3,
   REPORT_8 P3, REPORT_9 P3)
6. **Rewrite the eight issues that cannot be executed as written.** Epic 2 issue 05's
   "compiles without churn" is impossible in Go — embedded fields promote for
   selectors, not for composite-literal keys, so 218 call sites break; Epic 9's
   disposition checker enumerates the whole upstream tree rather than the 232-file
   range, and structurally cannot see the three deleted files; Epic 7 issue 05 sorts
   then appends and its own acceptance criterion asserts the other order; `postForm`'s
   specified signature cannot produce the error strings two issues pin; the
   `callbackHost` rename names one file and has two callers, so the package does not
   build. (REPORT_2 P1+P2, REPORT_7 P2 ×3, REPORT_8 P2, REPORT_9 P1+P2)
7. **Settle the acceptance criteria that pass whichever way the implementer decides.**
   Port-or-deviate, honor-or-reject `Fetch`, "record the gap and file a follow-up" —
   which `SCOPE.md:65-78` rules out by name — plus two criteria nothing checks and one
   upstream test file three issues each assume another owns. (REPORT_5 P2 ×2,
   REPORT_6 P2+P3, REPORT_8 P3, REPORT_9 P2)
8. **Close the `// Ports:` and `docs/PORTING.md` provenance gaps.** Epic 8 narrows the
   epic's "every ported file" to "every non-test file" when its most explicitly ported
   artifact is a test file; Epic 5's criterion loses its `docs/PORTING.md` half in
   seven issues; Epic 4 issues 05/07/08 and Epic 6 issues 02/03/04/06 edit ported files
   with no requirement that the header still describes them; Epic 7's four ported test
   files are never asked for one. (REPORT_4 P2, REPORT_5 P2, REPORT_6 P3 ×2,
   REPORT_7 P2, REPORT_8 P1)
9. **Declare the missing dependencies and de-duplicate the shared ownership.** Epic 2
   issue 10 needs issue 09's decision and declares no edge; Epic 6 issue 06 conflicts
   with 05 in `ai/provider.go` and issue 11 must land last; four pairs of issues each
   promise the same test, the same merge or the same criterion. (REPORT_2 P2 ×3,
   REPORT_3 P2+P3, REPORT_4 P2, REPORT_6 P2, REPORT_8 P3, REPORT_9 P3)
10. **Correct the cross-epic claims that went false.** Two Epic 5 issues say "no epic
    in this program owns it" for work Epic 6 issues #172 and #176 now own; Epic 6
    issue 03 removes an acceptance criterion Epic 5 issue 11 promises to preserve,
    recorded on one side only; "epic 12" belongs to the retired numbering.
    (REPORT_5 P3, REPORT_6 P2, REPORT_7 P3 ×2)
11. **Route `docs/planning/` edits through the drift mechanism.** Epic 3 issues 02 and
    06 and Epic 5 issue 01 make un-triaged `SPECS.md` edits a merge-blocking
    acceptance criterion; Epic 3 issue 01 already does it correctly, as a drift record.
    (REPORT_3 P2, REPORT_5 P3)
12. **Fix the schema and convention violations**: a `build:` commit type the repo does
    not use, mandated as an acceptance criterion in the first PR of the program; a
    renamed required section heading; an unflagged adapter file-layout departure.
    (REPORT_1 P2, REPORT_4 P3, REPORT_8 P3)

**Sweeps**

13. **Rewrite the `## PR size note` in all 74 issues against its own band, and correct
    the mis-declared `size` values and their index bullets.** (71 at triage; the three
    issues this epic itself created bring the sweep to 74.) Seventy of seventy-one
    files carry one identical sentence — "Target ~500 changed lines; if this grows past
    ~1000, split it before opening the PR" — which describes an `L`, quoted verbatim
    at `S` and `M` issues alike. Includes pre-splitting Epic 2 issue 08 rather than
    leaving the cut to be made mid-PR, and stating once in `EPIC_3.md`'s `## Notes`
    that generated catalog output does not count toward the band.
    (REPORT_2 P2+P3, REPORT_3 P2 ×2+P3, REPORT_4 P2+P3, REPORT_5 P2 ×2, REPORT_6 P2 ×2,
    REPORT_7 P2, REPORT_8 P2, REPORT_9 P3)
14. **Correct the factual errors in the issue bodies**: `file:line` citations off by
    one or pointing at the wrong file, counts that do not reproduce from the commands
    that claim them (370 vs 369, 42 vs 44, 41 vs 39, 75 vs 72), three wrong
    `docs/PORTING.md` row citations, and two Scopes missing files they must touch.
    (REPORT_1 P3 ×2, REPORT_3 P3, REPORT_4 P3, REPORT_5 P3, REPORT_6 P3, REPORT_7 P3,
    REPORT_8 P2+P3, REPORT_9 P3)
15. **Record the branch, base-branch and status-commit conventions** once in each
    epic's `## Notes` — `CONVENTIONS.md:213-225` is reflected densely everywhere else
    in the bodies and nowhere for these three. (REPORT_1 P3, REPORT_2 P3, REPORT_5 P3)
16. **Normalize the in-bundle link form and the `depends_on` representation.** All nine
    `issues/index.md` files use `./NN-slug.md` where the bundle rule mandates the
    leading-slash form every issue body already uses; `depends_on: [1]` and
    `issue: 01` express the same identifier two ways.
    (REPORT_1 P3 ×2, REPORT_2 P3, REPORT_4 P3, REPORT_7 P3, REPORT_9 P3)
17. **Reconcile GitHub state with the files**, last, once the titles have settled: ten
    issue titles (#149–#158) dropped the colon their local files carry, and
    bundle-absolute links copied verbatim into issue bodies render as
    `https://github.com/epic-N-…` and 404 for anyone reading on GitHub.
    (REPORT_4 P3, REPORT_9 P3)

## Out of scope

- **Re-reviewing.** Severities come from the reports as written. A finding judged
  wrong is a `won't-fix` recorded here, not a re-grade — and this round produced none.
- **Editing the reports.** `docs/REPORT_1.md`–`REPORT_9.md` are the record of what the
  bundle looked like when someone looked. They are consumed, never amended.
- **Implementing any of the repaired issues.** This epic edits `docs/epics/**` and,
  where a repair says so, `docs/planning/**`. No Go code changes, and the three CI
  gates are unaffected.
- **`docs/planning/DRIFT.md`.** No finding was dispositioned as drift: every repair
  corrects an artifact rather than a standard. The register stays absent until an
  implementation epic discovers real drift.
- **The upstream port itself.** Nothing here changes what the program builds — only
  what its plan says about it.

## Acceptance criteria

1. Every OpenAI-family tool-choice and thinking-budget reference in Epic 4 issues 04,
   05, 09 and 11 names one field, with one owning issue, and the consuming issues
   declare that blocker.
2. `EPIC_4.md`, `docs/planning/scope/12-constrained-sampling.md` and Epic 4 issue 01
   agree on where `ConstrainedSamplingConfig` lands, with the upstream evidence cited.
3. Each of the six orphaned items either names an owning issue by number, or appears
   in an epic's `## Out of scope` with a reason — none is left pointing at an epic that
   disclaims it.
4. `ai/apis/internal/httpretry` stays where `CONVENTIONS.md:22-26` puts it for this
   program, Epic 3 issue 06 is scoped to the images adapter, and Epic 9 issue 03 states
   at its head that the checker is an assumption beyond `EPIC_9.md`.
5. No epic file contains an acceptance criterion, scope bullet or out-of-scope bullet
   that its own issues contradict. Each amendment names the issue that forced it.
6. Each of the eight unexecutable issues carries acceptance criteria that can be
   satisfied by the work the issue describes — verified by re-reading each against the
   Go tree or the upstream shape it cites.
7. No acceptance criterion in any epic is satisfied by two opposite outcomes; each
   names the single expected behaviour, with the deciding reason in the epic's
   `## Notes`.
8. Every issue that edits or creates a ported file requires the `// Ports:` header to
   still describe it, and every issue whose change alters a `docs/PORTING.md` row says
   so — test files included.
9. `depends_on` and the `## Dependencies` prose agree in both directions for all 74
   issues, and no behaviour, test or merge is claimed by two issues.
10. `git grep -n "No epic in this program"` over `docs/epics/` returns nothing that a
    live issue number contradicts, and every cross-epic contract conflict is recorded
    on both sides.
11. No issue makes an edit to `docs/planning/` a merge-blocking acceptance criterion;
    where the epic genuinely authorizes one, the epic says so.
12. `grep -rn 'Target ~500 changed lines' docs/epics/` returns only issues whose
    `size` is `L`, and every `size:` value matches its `issues/index.md` bullet and its
    own `## PR size note`.
13. Every `file:line` citation and every count that a repaired issue states is
    re-derived from the tree, with the producing command stated where the number will
    drift.
14. `grep -rn '](\./' docs/epics/*/issues/index.md` returns nothing, and all nine
    epics express `depends_on` in one form.
15. Every GitHub issue title matches its file's `title:` frontmatter byte for byte, and
    no issue body contains a link beginning `](/epic-`.
16. All 97 findings are accounted for: each is repaired, or recorded in this epic's
    `## Notes` with the reason it was not.
17. The `docs/epics/` bundle is internally consistent after the repairs — `index.md`,
    `log.md` and every `issues/index.md` agree with the frontmatter of the files they
    index.

## Dependencies

None inside this bundle, and this epic blocks
[Epic 1](/epic-1-repository-hygiene/EPIC_1.md) and every epic after it. The whole
point of the number is that it lands first: each repair here is a markdown edit today
and a mid-PR discovery, a rebase or a released public symbol tomorrow.

Two repairs — items 2 and 3 — need a check against upstream at `936aff00`, which is
not vendored in this tree. Verification is the first step of the issue that owns them,
not a blocker on the epic.

## Context

- [Technical specs](../../planning/SPECS.md)
- [Conventions](../../planning/CONVENTIONS.md)
- [Upstream sync scope](../../planning/SCOPE.md)
- [Report 1 — Epic 1 issues](../../REPORT_1.md)
- [Report 2 — Epic 2 issues](../../REPORT_2.md)
- [Report 3 — Epic 3 issues](../../REPORT_3.md)
- [Report 4 — Epic 4 issues](../../REPORT_4.md)
- [Report 5 — Epic 5 issues](../../REPORT_5.md)
- [Report 6 — Epic 6 issues](../../REPORT_6.md)
- [Report 7 — Epic 7 issues](../../REPORT_7.md)
- [Report 8 — Epic 8 issues](../../REPORT_8.md)
- [Report 9 — Epic 9 issues](../../REPORT_9.md)

## Notes

**The triage.** 97 findings extracted from nine reports, 0 dropped as stale, 0
recorded as `won't-fix`, 0 dispositioned as drift, 97 grouped into the seventeen
repairs above. Nothing was dropped because nothing had moved: the nine reports and the
71 issues they review were committed the same day, no implementation has started, no
GitHub issue tracked any finding, and `docs/planning/DRIFT.md` does not exist. All nine
are `review-issues` reports, so no finding carried a disposition decided at review time.

**Decisions taken during the triage**, recorded because a later reader will otherwise
re-open them:

- `ai/apis/internal/httpretry` is **not** relocated in this program. Epic 3 issue 06 is
  scoped to the OpenRouter images adapter; the reconciliation with upstream's
  `provider-retry.ts` stays with Epic 9, which already owns it. Moving a shared
  internal package while Epics 4 and 5 rewrite its four consumers buys nothing that
  waiting does not.
- Epic 9 issue 03 (the disposition checker) is **kept**, labelled an assumption beyond
  `EPIC_9.md`, and its file-set enumeration corrected. Dropping it would make the
  epic's second completion gate a judgement call again.
- `EPIC_6.md` **absorbs** the four upstream files its issues already build, rather than
  moving issues 08 and 09 to Epic 7. The work is real, correctly placed and traceable —
  Epic 2 issue 08 explicitly deferred the `Models` auth surface here; it is the epic
  file that never recorded the hand-off.
- Generated catalog output **does not** count toward the size bands. Epic 3 issues 04
  and 05 apply opposite rules to one situation; the hand-written rule is the one that
  measures reviewability.
- The `issues/index.md` link form is **normalized to the leading slash**, not recorded
  as drift. `bundle-interfaces.md` is ambiguous — its link table mandates the leading
  slash, its bullet template shows a bare filename — but every issue body in the bundle
  already uses the leading-slash form, so the indexes are the outlier rather than the
  convention.

**Why epic 0 and not epic 10.** Zero means *before continuing*. These repairs are not
the next increment of the plan; they are what makes the rest of the plan implementable.
This epic is temporary by design: once every issue is `done` and its milestone closed,
it is retired, and the record of what it fixed stays in
[the log](/log.md).

**Branch, base-branch and status-commit conventions** (`docs/planning/CONVENTIONS.md:213-224`).
Feature branches are named `issue-<NN>-<slug>` (`:216-218`). PRs target `develop` and
merge with a merge commit — no rebase, no squash (`:213-219`). Each status transition
gets its own `docs(epics): …` commit, separate from the implementation commit
(`:221-224`). `Closes #N` will not auto-close the issue, because PRs merge into
`develop` rather than the repo's default branch — Epic 1 is the first epic to exercise
that path; the explicit close at reconcile is the normal route, not a fallback.

### The finding ledger — all 97 accounted for

Written by [issue 15](/epic-0-plan-remediation/issues/15-reconcile-github-state-with-the-files.md)
at epic close, satisfying acceptance criterion 16. `R<n>.<k>` is the k-th `### P…`
heading in `docs/REPORT_<n>.md`, in file order; the count re-derives from
`grep -c '^### P' docs/REPORT_*.md` → 9 + 11 + 13 + 12 + 11 + 12 + 10 + 9 + 10 = **97**.
Every finding is repaired by the Epic 0 issue named; none is `won't-fix`, none was
dropped as stale, none was dispositioned as drift. Where two issues share a finding,
both are named.

| # | Sev | Finding | Repaired by |
|---|---|---|---|
| R1.1 | P2 | Epic AC 2's single exception widened by issue 02 to two directories | 03 |
| R1.2 | P2 | Epic AC 4 demands a single PR; the epic ships two | 03 |
| R1.3 | P2 | Issue 01 mandates a `build:` commit type the conventions do not use | 03 |
| R1.4 | P2 | Issue 02's `SPECS.md` rewrite is in Scope with no acceptance criterion | 03 |
| R1.5 | P3 | Issue 02's reference and file counts do not reproduce (370 vs 369, 42 vs 44) | 03 |
| R1.6 | P3 | Root `CONVENTIONS.md` line citation off by one | 03 |
| R1.7 | P3 | `issues/index.md` uses `./` links where the bundle rule wants a leading slash | 14 |
| R1.8 | P3 | `depends_on: [1]` does not match `issue: 01` | 14 |
| R1.9 | P3 | Branch and status-commit conventions absent from both bodies | 14 |
| R2.1 | P1 | Issue 05's "compiles without churn" is impossible in Go; the report's 218 broken call sites re-derived as 248 | 04, 05 |
| R2.2 | P2 | Issue 10 needs issue 09's decision but declares no edge | 04 |
| R2.3 | P2 | Issue 05 demands a test for a merge it assigns to issue 06 | 04 |
| R2.4 | P2 | Issue 07's deep copy goes shallow once issues 03 and 05 land | 04 |
| R2.5 | P2 | Issue 03's `ModelCost` embedding breaks twelve keyed literals | 04 |
| R2.6 | P2 | Issue 08 concedes the size ceiling and leaves the split to be made mid-PR | 05, 13 |
| R2.7 | P2 | Issue 10 points deferred-response adapter work at Epics 4 and 5 | 02 |
| R2.8 | P2 | `SimpleStreamOptions.deferred` is owned by no issue in any epic | 02 |
| R2.9 | P3 | Branch, base-branch and status-commit conventions appear in no body | 14 |
| R2.10 | P3 | `issues/index.md` links `./NN-slug.md` | 14 |
| R2.11 | P3 | Issue 07 is sized `S` but carries `M`'s work | 13 |
| R3.1 | P1 | The `ImagesOptions` → `ProviderRequestOptions` reshape is deferred in a circle | 02 |
| R3.2 | P1 | Issue 06 relocates `ai/apis/internal/httpretry` repo-wide | 06 |
| R3.3 | P2 | `images-models.ts`'s `getAuth` reshape is owned by no issue | 02 |
| R3.4 | P2 | Issues 02 and 06 mandate `SPECS.md` edits inside implementation PRs | 06 |
| R3.5 | P2 | Issues 04 and 05 apply opposite sizing rules to generated data | 06, 13 |
| R3.6 | P2 | Issue 04 has no criterion keeping the image catalog out of its diff | 06 |
| R3.7 | P3 | All six PR size notes are identical boilerplate | 13 |
| R3.8 | P3 | The images strict-decode test is owned twice | 06 |
| R3.9 | P3 | Issue 03 inverts the epic's "strictly in this order", correctly | 06 |
| R3.10 | P3 | Issue 01 makes rewriting its four successors part of its own DoD | 06 |
| R3.11 | P3 | Issue 02 invents a `MODEL_CATALOG_DIR` configuration surface | 06 |
| R3.12 | P3 | Issue 03's invariant battery exceeds the epic's criteria | 06 |
| R3.13 | P3 | The epic's "41 `*.models.ts`" is never reconciled with 39 provider files | 06 |
| R4.1 | P1 | Three issues wire an OpenAI `tool_choice` option that does not exist | 01 |
| R4.2 | P1 | Issue 04 references `StreamOptions.ThinkingBudgets`, which does not exist | 01 |
| R4.3 | P2 | Issue 01 reverses the epic's acceptance criterion 3 | 01 |
| R4.4 | P2 | Issues 03, 10 and 11 omit their cross-epic blockers | 07 |
| R4.5 | P2 | Sizing: issues 09 and 11 under-declared, issue 10 self-contradictory | 13 |
| R4.6 | P2 | Epic AC 5 (provenance + `docs/PORTING.md`) absent from issues 05, 07, 08 | 07 |
| R4.7 | P2 | Issue 12 adds `ai.UUIDv7`, a public API the epic never scoped | 02 |
| R4.8 | P3 | Ten GitHub issue titles drop the colon present in the local files | 15 |
| R4.9 | P3 | Issue 12 renames a required section heading | 07 |
| R4.10 | P3 | `issues/index.md` links are `./file.md` | 14 |
| R4.11 | P3 | The PR size note is identical boilerplate in eleven of twelve issues | 13 |
| R4.12 | P3 | Issue 12 attributes the WebSocket recorders to the wrong file | 07 |
| R5.1 | P1 | Seven of eleven issues are blocked by an Epic 4 issue the epic disclaims | 08 |
| R5.2 | P2 | Epic acceptance criterion 3 can be closed without a Go home | 08 |
| R5.3 | P2 | `## PR size note` boilerplate contradicts the declared `size` | 13 |
| R5.4 | P2 | Issue 01's `size: S` does not match its own scope | 13 |
| R5.5 | P2 | Epic AC 5's `docs/PORTING.md` half is dropped by seven issues | 08 |
| R5.6 | P2 | Five acceptance criteria pass whichever way the implementer decides | 08 |
| R5.7 | P3 | Two issues claim nobody owns work that Epic 6 owns | 08 |
| R5.8 | P3 | Issue 01 asks for an unrequested edit to a finalized planning doc | 08 |
| R5.9 | P3 | Two out-of-scope boundaries are crossed but never amended in the epic | 08 |
| R5.10 | P3 | Three stale or off-by-one code citations | 08 |
| R5.11 | P3 | Branch and status-commit conventions absent from the bodies | 14 |
| R6.1 | P1 | Four issues build upstream files the epic never names; issue 08 contradicts its Out of scope | 09 |
| R6.2 | P2 | Acceptance criterion 4's "catalog entry" half has no owner | 09 |
| R6.3 | P2 | `depends_on` under-declares two conflicts the bodies do declare | 09 |
| R6.4 | P2 | Issue 03 removes a criterion Epic 5 issue 11 (#169) promises to preserve | 09 |
| R6.5 | P2 | AC 5 is unenforced, and three issues share one upstream test file | 09 |
| R6.6 | P2 | `docs/auth.md` goes stale; issue 11 routes the fix to an epic that will not make it | 02, 09 |
| R6.7 | P2 | Issue 08 is sized `M` for work that reads as `L` | 13 |
| R6.8 | P2 | The PR size note is identical boilerplate in all eleven issues | 13 |
| R6.9 | P3 | Acceptance criterion 2 ("no Go package moved or renamed") is verified nowhere | 09 |
| R6.10 | P3 | `// Ports:` maintenance is a DoD item in six issues and absent from four | 09 |
| R6.11 | P3 | Issue 11 names issue 07 as amending `docs/PORTING.md`; issue 07 does not | 09 |
| R6.12 | P3 | Three small internal inconsistencies | 09 |
| R7.1 | P2 | Issue 05 sorts the login list, then appends Radius | 10 |
| R7.2 | P2 | `postForm`'s signature cannot produce the error strings issues 02 and 04 pin | 10 |
| R7.3 | P2 | The `callbackHost` rename omits `codex.go` and miscounts the callers | 10 |
| R7.4 | P2 | AC 5 half covered: four ported test files never require a `// Ports:` header | 10 |
| R7.5 | P2 | Sizing is internally inconsistent and every size note contradicts its `size` | 13 |
| R7.6 | P3 | "epic 12" has no counterpart in this bundle | 10 |
| R7.7 | P3 | Issue 01 claims Epic 6 lands two unused fields; it lands one | 10 |
| R7.8 | P3 | Radius's default gateway constant lands in `cmd/pi-ai` | 10 |
| R7.9 | P3 | Minor citation drift and a case-inconsistent symbol name | 10 |
| R7.10 | P3 | `issues/index.md` uses `./`-relative links | 14 |
| R8.1 | P1 | Ported test files are exempted from the `// Ports:` header the epic requires | 11 |
| R8.2 | P2 | `size:` is contradicted by an identical size note in all five issues | 13 |
| R8.3 | P2 | Issue 05's count-sweep criterion greps frozen artifacts and can never pass | 11 |
| R8.4 | P2 | Issue 05's Scope misses two files carrying the same stale counts | 11 |
| R8.5 | P3 | Every `docs/PORTING.md` line citation in issue 05 is wrong | 11 |
| R8.6 | P3 | "fetched at provider setup" becomes "gated refresh", unacknowledged | 11 |
| R8.7 | P3 | AC 2 ("248 test lines ported") is claimed by issue 02, half-executed in issue 01 | 11 |
| R8.8 | P3 | `events.go` departs from the named adapter file layout without a note | 11 |
| R8.9 | P3 | Issue 04 leaves a stop-work branch its own dependency already answers | 11 |
| R9.1 | P1 | The disposition checker measures a different file set than "232 files in range" | 12 |
| R9.2 | P2 | Issue 03 builds tooling the epic never asked for, unflagged | 12 |
| R9.3 | P2 | Issue 01's row-count criterion does not match the code it counts | 12 |
| R9.4 | P2 | Issue 04's escape hatch lets a genuinely unported file ship as a "gap" | 12 |
| R9.5 | P3 | Epic AC 2's `docs/planning/DRIFT.md` entry is written by no issue in the epic | 12 |
| R9.6 | P3 | Issues 01 and 02 both claim the same `httpretry` regression criterion | 12 |
| R9.7 | P3 | `docs/classifier-parity.md` lands in the consumer-facing docs bundle | 12 |
| R9.8 | P3 | PR size notes are boilerplate and disagree with the declared `size` | 13 |
| R9.9 | P3 | `./` bullets in `issues/index.md`, and bundle-absolute links inside GitHub bodies | 14, 15 |
| R9.10 | P3 | Issue 05 does not mention the CHANGELOG link-reference footer | 12 |

**Caveats recorded rather than repaired**, all noticed while building the ledger:

- Several issues' one-line `Findings repaired:` tallies undercount their own `## Scope`.
  Issue 03 says "REPORT_1 P2 ×3" for the four P2 findings its Scope items 1–4 repair;
  issue 06 says "P3 ×5" and then lists six; issues 08, 09, 10 and 12 each say "P3 ×3" or
  "P3 ×5" over a list of one more. The prose was left alone — the findings themselves are
  repaired, and rewriting fourteen merged issue bodies for a tally is churn. **This table
  is the authority on which issue repaired what.**
- Acceptance criterion 15's literal form ("no issue body contains a link beginning
  `](/epic-`") cannot be satisfied byte-for-byte: issues #212 and #213 are the two issues
  whose job is to describe that pattern, so their bodies must quote it. It is read as
  "no *link*" — four literal occurrences survive inside inline-code spans in #212 and
  #213, and every one of the 946 real links in the ten tracking issues and 89 sub-issue
  bodies is an absolute `https://github.com/kern-ia/kern-link/blob/develop/…` URL.
- Issues **#87–#97** are the closed sub-issues of a retired epic bundle
  (`epic-1-reliability-api-fixes`, `epic-2-release-process-hygiene`) that no longer exists
  on disk. Their bodies still carry bundle-absolute links. They are outside the
  `upstream-sync` program and were deliberately left untouched: rewriting their links to
  `blob/develop` URLs would point at files that do not exist.

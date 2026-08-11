---
type: Epic
title: "Plan remediation"
description: "Repair the 97 findings the nine issue-review reports raised against the epic and issue bundle, so the sync program implements against a plan that agrees with itself."
tags: [epic, remediation]
timestamp: 2026-08-11T20:10:00Z
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

13. **Rewrite the `## PR size note` in all 71 issues against its own band, and correct
    the mis-declared `size` values and their index bullets.** Seventy of seventy-one
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
9. `depends_on` and the `## Dependencies` prose agree in both directions for all 71
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

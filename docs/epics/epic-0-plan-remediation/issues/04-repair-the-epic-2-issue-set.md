---
type: Issue
title: "Repair the Epic 2 issue set: the criterion Go cannot satisfy, the missing edge, and the duplicated ownership"
description: "Rewrite Epic 2 issue 05's impossible embedded-field criterion and issue 03's blast radius, declare the 09→10 dependency, and de-duplicate the merge test and the deep-copy gap."
tags: [epic-0]
timestamp: 2026-08-11T13:10:00Z
epic: 0
issue: 04
slug: repair-the-epic-2-issue-set
size: M
status: in-progress
gh_issue: 202
resource: https://github.com/kern-ia/kern-link/issues/202
depends_on: [2]
---

# Repair the Epic 2 issue set: the criterion Go cannot satisfy, the missing edge, and the duplicated ownership

## Summary

Epic 2 is the core-types epic every other epic compiles against, and its issue set
carries the program's one genuinely impossible acceptance criterion: "existing call
sites still compile without churn" through embedded-field promotion. Go promotes
embedded fields for **selectors**, not for **composite-literal keys**, so the moment
the transport knobs move into an embedded `ProviderRequestOptions`, 218 keyed literals
stop compiling. The criterion can never be ticked as written; the implementer either
abandons the embedding the epic asked for, or ticks a box that is false.

Three smaller defects sit beside it: a `ModelCost` embedding with the same Go rule and
twelve unbudgeted literals, a dependency edge that exists in prose and not in
`depends_on`, and one behaviour rule promised by two issues at once.

Findings repaired: REPORT_2 P1, REPORT_2 P2 ×4 (issues 09/10 edge, issue 05's borrowed
test, issue 07's deep copy, issue 03's literals). Epic 0 scope items 6, 9 and 14;
acceptance criteria 6, 9 and 13.

## Scope

**1. The impossible criterion (REPORT_2 P1).** Rewrite
`epic-2 issue 05:107-109` from "compiles without churn" to the two claims that are
actually true: field *access* is unchanged by promotion (`opts.APIKey` still compiles —
a test asserts it), **and** every keyed `StreamOptions` composite literal is
mechanically rewritten to `ProviderRequestOptions: ai.ProviderRequestOptions{...}`.
Replace the ripple estimate at `:86-88` ("field promotion … usually means none") with
the real count and the command that produced it:
`grep -rn "StreamOptions{" --include=*.go .` filtered to the moved field names returns
**218** lines today, mostly `_test.go` (e.g. `ai/apis/anthropic/anthropic_oauth_test.go:39`,
`ai/apis/anthropic/anthropic_error_mapping_test.go:51`). Add the rewrite to `## Scope`
so it is planned work rather than a discovery.

**2. `ModelCost`'s twelve literals (REPORT_2 P2).** Same Go rule.
`grep -rn "ModelCost{" --include=*.go .` returns **12** keyed literals across eight
files — `ai/cost_test.go:16`, `ai/apis/anthropic/anthropic_test.go:944`,
`ai/apis/bedrock/stream_test.go:186`, `ai/apis/codex/stream_test.go:335`,
`ai/apis/openairesponses/stream_test.go:281`, `ai/catalog/fireworks_test.go:46`,
`ai/catalog/together_test.go:48`, `ai/example_stream_test.go:116` and others — while
`epic-2 issue 03:102-113` names only `ai/model.go`, `ai/cost.go`, `ai/cost_test.go`,
`ai/catalog/*` and `ai/estimate.go`. Add the rewrite to `## Scope` and the eight files
to `## Relevant files`, and re-derive the count at the PR's base commit.

**3. The undeclared 09→10 edge (REPORT_2 P2).** `epic-2 issue 10`'s Scope says it adds
the `Models`-level variants "in whatever shape [issue 09] settled on", and issue 09
leaves that shape explicitly to its implementer (`09:49-56`) — yet `10:14` reads
`depends_on: [1, 2, 5, 8]` and `09:134` asserts "**Blocks**: Nothing inside this epic."
`implement-epic` selects from `depends_on`, so issue 10 is dispatchable before 09 and
the two collide in `ai/options.go`. Set `10:14` to `depends_on: [1, 2, 5, 8, 9]`, add
issue 09 to its Blocked-by prose at `:135-138`, and name issue 10 under Blocks at
`09:134`.

**4. The merge test owned twice (REPORT_2 P2).** `epic-2 issue 05:116-117` requires
`TestSamplingParamsPerRequestOverridesModel` "at whichever layer this PR places it"
while `:67-71` defers the merge itself to issue 06, which asserts the identical
precedence in `TestBuildBaseOptionsMergesSamplingParams` (`06:74-76`) and depends on 05.
Delete `05:116-117`, leaving 05 with the `SamplingParams` fields and a marshalling
assertion; issue 06's test becomes the sole precedence proof, and the "never mutate
`Model.SamplingParams`" rule (`06:45-46`) keeps one home.

**5. The deep copy that goes shallow (REPORT_2 P2).** `epic-2 issue 07` is unblocked
(`depends_on: []`) and can land first; its hand-written `ModelsStoreEntry` deep copy
(`07:71-73`, tested at `:95-98`) becomes shallow for `Tiers` the moment issue 03 adds
`Tiers []ModelCostTier` (`03:49-53`) and for `SamplingParams` the moment issue 05 adds
it (`05:67-70`), and both tests keep passing because they were written against the old
shape. Add to issues 03 and 05 the criterion: extend `ModelsStoreEntry`'s deep copy and
`TestInMemoryModelsStoreReadReturnsCopy` to cover the field this PR adds to `Model`, or
state why it needs none — and add `ai/modelsstore.go` to both issues' Relevant files.

## Out of scope

- **Pre-splitting issues 05 and 08.** Both splits are real and both are
  [issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md)'s
  job — they create new issue files and new GitHub issues, which does not belong in a
  PR of body edits. This issue writes the corrected criteria that the split then
  distributes.
- **The two Epic 2 orphans** (the deferred-response adapter pointer at `10:79-82` and
  `SimpleStreamOptions.deferred` at `10:87-91`) —
  [issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md),
  which lands first.
- **Issue 07's `size: S` → `M`** (REPORT_2 P3) and every other size correction —
  [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).
- **The `./` index links, `depends_on` representation and the branch conventions**
  (REPORT_2 P3 ×2) —
  [issue 14](/epic-0-plan-remediation/issues/14-normalize-links-depends-on-and-branch-notes.md).
- **Re-litigating whether Epic 2 should embed at all.** REPORT_2's open question asks
  whether `EPIC_2.md:26-27` requires Go embedding or only reusable transport knobs. The
  epic asked for the reshape; this issue makes the issues describe it honestly rather
  than replacing the mechanism.

## Acceptance criteria / Definition of done

- [ ] No acceptance criterion in Epic 2's twelve issues asserts that keyed composite
      literals survive embedding. `git grep -n 'without churn' docs/epics/epic-2-*/`
      returns nothing.
- [ ] Issues 05 and 03 each carry the literal rewrite in `## Scope`, with the count and
      the exact `grep` that produced it, re-derived at the PR's base commit.
- [ ] `epic-2 issue 03`'s `## Relevant files` names the eight files carrying
      `ai.ModelCost{...}` literals.
- [ ] `epic-2 issue 10:14` reads `depends_on: [1, 2, 5, 8, 9]`, and issues 09 and 10
      state the edge in both directions in prose.
- [ ] Exactly one Epic 2 issue asserts the per-request-over-model `SamplingParams`
      precedence: `git grep -n 'SamplingParams' docs/epics/epic-2-*/issues/ | grep -i
      'precedence\|overrides'` names issue 06 only.
- [ ] Issues 03 and 05 each carry a `ModelsStoreEntry` deep-copy criterion naming the
      field they add, and both list `ai/modelsstore.go` under Relevant files.
- [ ] `depends_on` and the `## Dependencies` prose agree in both directions for all
      twelve Epic 2 issues.
- [ ] `timestamp` refreshed on every edited file; the GitHub bodies of the edited
      issues (#131, #133, #135, #137, #138) updated to match; `issues/index.md` still
      agrees with the frontmatter.
- [ ] `git diff --name-only` lists only files under `docs/`.
- [ ] CI green (three gates unaffected).
- [ ] Branch `issue-<NN>-repair-the-epic-2-issue-set`, PR targets `develop`, merge
      commit. Commit e.g. `docs(epics): make Epic 2's criteria satisfiable in Go`.

## Relevant files / areas

- `docs/epics/epic-2-core-types-and-models-contracts/issues/03-tiered-model-cost.md:49-53`,
  `:54-55`, `:102-113`.
- `.../issues/05-provider-request-options.md:67-71`, `:86-88`, `:103-105`, `:107-109`,
  `:116-117`.
- `.../issues/07-models-store.md:14`, `:71-73`, `:95-98`.
- `.../issues/09-models-request-transforms.md:49-56`, `:134`.
- `.../issues/10-deferred-response-dispatch.md:14`, `:49-54`, `:135-138`.
- `.../issues/06-simple-options-and-lazy.md:43-46`, `:74-76` — read to confirm the
  merge stays there.
- Read-only, to re-derive the counts: `ai/options.go`, `ai/model.go:36-42`, `:54-71`,
  `ai/modelsstore.go`, and the eight files carrying `ai.ModelCost{...}`.

## Dependencies

- **Blocked by**: [Issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md)
  — it edits Epic 2 issues 05 and 10, the same two files this issue rewrites.
- **Blocks**: [Issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md),
  which distributes issue 05's corrected criteria across the split halves, and
  [issue 13](/epic-0-plan-remediation/issues/13-rewrite-the-pr-size-notes.md).

## PR size note

`M` — ~200–400 changed lines across six issue bodies. Split past ~500. The clean cut
if it overruns: repairs 1–2 (the Go-rule rewrites) first, repairs 3–5 (the graph and
ownership edits) second.

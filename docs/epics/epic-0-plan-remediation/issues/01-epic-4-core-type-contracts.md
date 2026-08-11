---
type: Issue
title: "Settle the Epic 4 core-type contracts: the tool-choice field, the thinking-budget carrier, and ConstrainedSamplingConfig"
description: "Name and assign the two ai.StreamOptions fields three Epic 4 issues assume exist, and make EPIC_4.md, the scope decision and Epic 4 issue 01 agree on where ConstrainedSamplingConfig lands."
tags: [epic-0]
timestamp: 2026-08-11T09:40:00Z
epic: 0
issue: 01
slug: epic-4-core-type-contracts
size: M
status: pr-open
gh_issue: 199
gh_pr: 216
resource: https://github.com/kern-ia/kern-link/issues/199
depends_on: []
---

# Settle the Epic 4 core-type contracts: the tool-choice field, the thinking-budget carrier, and ConstrainedSamplingConfig

## Summary

Three Epic 4 issues wire an OpenAI-family tool-choice option onto `ai.StreamOptions`
that does not exist and that no issue in any epic creates; a fourth writes acceptance
criteria against `StreamOptions.ThinkingBudgets`, which lives on `SimpleStreamOptions`
and is invisible to the code path the issue targets. Separately, `EPIC_4.md`, the scope
decision and Epic 4 issue 01 disagree about where `ConstrainedSamplingConfig` lands.

All three are contract defects: the first implementer to reach them has to invent a
**public** symbol in package `ai` — the core Epic 2 owns — in the epic immediately
before Epic 9 cuts `v0.2.0`. Three PRs, three guesses at a released API surface. This
issue names each field once, gives it an owning issue, and wires the dependencies, so
it lands first in Epic 0: everything downstream is written against these names.

Findings repaired: REPORT_4 P1 ×2, REPORT_4 P2 (the criterion-3 reversal). Epic 0
scope items 1 and 2; acceptance criteria 1 and 2.

## Scope

**1. The OpenAI-family tool-choice field.** Declare it once as `OpenAIToolChoice` on
`ai.StreamOptions`, following the `GoogleToolChoice` (`ai/options.go:202`),
`MistralToolChoice` (`:227`) and `BedrockToolChoice` (`:265`) precedent, and give it
one owner:

- Add the field to the `## Scope` of
  [Epic 2 issue 05](/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md)
  with its exact Go name, type and JSON tag — that issue already owns the
  `StreamOptions` reshape, and a public `ai` field belongs to the core-types epic.
- Rewrite `epic-4 issue 05:47-49` and `:77-79` to name the field instead of hedging
  "or wherever the flat per-adapter field lives"; rewrite `epic-4 issue 09:45`, `:63`,
  `:106` and `epic-4 issue 11:30-31`, `:47-48`, `:94-95` to the same name.
- Add "**Blocked by**: Epic 2 issue 05 (`OpenAIToolChoice`)" to the `## Dependencies`
  of Epic 4 issues 05, 09 and 11. `depends_on` cannot carry the edge — it holds
  intra-epic numbers only — so the prose is the only place it can live.

**2. The thinking-budget carrier.** `epic-4 issue 04:68-73` claims
"`ai.ThinkingBudgets` already exists (`ai/options.go:325-326`)"; it is a field of
`SimpleStreamOptions`, and the work lands in
`buildParams(model, chat, opts *ai.StreamOptions)`
(`ai/apis/openaicompletions/openaicompletions.go:340`), which never sees it. Declare
`OpenAIThinkingBudgets` on `ai.StreamOptions`, mirroring `BedrockThinkingBudgets`
(`ai/options.go:276-278`, which exists for exactly this reason — see
`ai/apis/bedrock/bedrock.go:93-127`), add it to the same Epic 2 issue 05 scope bullet,
and fix `epic-4 issue 04:109-111` (`TestThinkingTokenBudgetHonorsPerLevelOverride`) to
name the real field. Add the same cross-epic blocker line to issue 04's
`## Dependencies`.

**3. `ConstrainedSamplingConfig`.** Verify against upstream at `936aff00` whether the
field is `Tool.constrainedSampling` (as `epic-4 issue 01:43-51` reads it) or reachable
through the request options (as `EPIC_4.md:35-36` and `:52-53`,
`docs/planning/scope/12-constrained-sampling.md`'s Verdict, and `SCOPE.md:200-201` all
say). The upstream checkout is **not** vendored in this tree — `upstream/` holds
`UPSTREAM.lock` and `sync.sh` only — so the verification is the first step of this
issue, not an assumption. Then amend whichever side is wrong:

- If issue 01 is right: amend `EPIC_4.md:35-36` and `:52-53`,
  `docs/planning/scope/12-constrained-sampling.md`'s Verdict and `SCOPE.md:200-201` to
  say `ai.Tool`, and add a sentence to `EPIC_4.md` recording that issue 01 therefore
  changes package `ai`'s public `Tool` struct (`ai/types.go:243-247`) — core-type work
  inside an adapter epic.
- If the epic is right: rewrite `epic-4 issue 01:43-51` and `:55-69` to `ai.StreamOptions`
  and drop its "no deviation is needed at all" reasoning.

Either way, cite the upstream evidence (the file and symbol at `936aff00`) in the
amended text, so the next reader does not re-open the question.

Editing `docs/planning/` is authorized **for this issue only**, by Epic 0 scope item 2
("including the scope decision, if it is the one that moves") — it is the exception to
the rule Epic 0 issue 06 enforces everywhere else.

## Out of scope

- **Implementing any of it.** No Go code changes. `ai/options.go`, `ai/types.go` and
  the adapter packages are read for verification and left untouched.
- **The `src/utils/uuid.ts` orphan** (`epic-4 issue 12:37-43`) and the other five
  orphaned items — [issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md).
- **Epic 4's provenance, dependency and schema repairs** — those are
  [issue 07](/epic-0-plan-remediation/issues/07-repair-the-epic-4-issue-set.md). This
  issue touches Epic 4 issues 01, 04, 05, 09 and 11 only, and only the contract lines
  named above.
- **`docs/planning/DRIFT.md`.** Nothing here is drift: no code exists yet to diverge
  from a standard. The planning edits above are amendments to a decision that was
  wrong when written, and Epic 0's `## Out of scope` records why the register stays
  absent.
- **The other 15 `docs/planning/` edits** the reports flag. Every one of them is
  either refused or converted to a drift record by
  [issue 06](/epic-0-plan-remediation/issues/06-rein-in-epic-3-invented-scope.md) and
  [issue 08](/epic-0-plan-remediation/issues/08-reconcile-epic-5-with-its-issues.md).

## Acceptance criteria / Definition of done

- [ ] `git grep -n 'ToolChoice' docs/epics/epic-4-openai-family-adapters/` returns one
      Go identifier only, and the same identifier appears in
      `docs/epics/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md`
      as a declared field.
- [ ] `git grep -n 'ThinkingBudgets' docs/epics/epic-4-openai-family-adapters/issues/04-completions-thinking-formats.md`
      names the `StreamOptions` carrier, and no line in that file claims the field
      "already exists".
- [ ] The `## Dependencies` sections of Epic 4 issues 04, 05, 09 and 11 each carry a
      "**Blocked by**: Epic 2 issue 05" line naming the field it waits on.
- [ ] `EPIC_4.md`, `docs/planning/scope/12-constrained-sampling.md`,
      `docs/planning/SCOPE.md` and Epic 4 issue 01 name the **same** Go home for
      `ConstrainedSamplingConfig`, and the amended text cites the upstream file and
      symbol at `936aff00` that settles it.
- [ ] Epic 4 acceptance criterion 3 (`EPIC_4.md:52-53`) is satisfiable by the work
      Epic 4 issue 01 describes — verified by reading the two against each other.
- [ ] Every `file:line` citation this PR writes or leaves in the edited passages is
      re-derived from the tree at the PR's base commit.
- [ ] `timestamp` is refreshed on every edited issue and epic file, and the GitHub
      bodies of the edited issues (#147, #150, #151, #155, #157 and Epic 2's #133) are
      updated to match.
- [ ] `git diff --name-only` lists only files under `docs/` — no `.go`, `go.mod`,
      `.github/` or `upstream/` change.
- [ ] CI green: the three gates (`go test ./... -race -v`,
      `bash upstream/sync_test.sh`, `golangci-lint` pinned `v2.12.2`) are unaffected
      and pass.
- [ ] Branch `issue-<NN>-epic-4-core-type-contracts`, PR targets `develop`, merge
      commit (no squash, no rebase). Commit follows Conventional Commits, e.g.
      `docs(epics): name the two Epic 4 core-type fields and settle constrained sampling`.

## Relevant files / areas

- `docs/epics/epic-4-openai-family-adapters/EPIC_4.md:35-36`, `:52-53` — the
  constrained-sampling scope bullet and acceptance criterion 3.
- `docs/epics/epic-4-openai-family-adapters/issues/01-constrained-sampling-core.md:43-51`,
  `:55-69` — the `ai.Tool` reading.
- `.../issues/04-completions-thinking-formats.md:68-73`, `:109-111`,
  `:138-140` — the thinking-budget claims and the stated blocker.
- `.../issues/05-completions-deferred-tools-and-finish-reason.md:47-49`, `:77-79`.
- `.../issues/09-openai-responses-compat-and-wiring.md:45`, `:63`, `:106`.
- `.../issues/11-codex-request-body-and-stop-reasons.md:30-31`, `:47-48`, `:94-95`.
- `docs/epics/epic-2-core-types-and-models-contracts/issues/05-provider-request-options.md`
  — the receiving `## Scope`. Note that
  [issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md)
  later splits this file; the two new fields belong to the half that keeps the
  `StreamOptions` field declarations.
- `docs/planning/scope/12-constrained-sampling.md` (Verdict), `docs/planning/SCOPE.md:200-201`.
- Read-only, for verification: `ai/options.go:196-269`, `:276-278`, `:321-326`;
  `ai/types.go:243-247`; `ai/apis/openaicompletions/openaicompletions.go:340`;
  `ai/apis/bedrock/bedrock.go:93-127`; `ai/apis/codex/params.go:47`, `:102`.

## Dependencies

- **Blocked by**: None. This is the first issue of Epic 0 — it changes contracts the
  rest of the epic's repairs are written against.
- **Blocks**: [Issue 02](/epic-0-plan-remediation/issues/02-adopt-the-orphaned-scope-items.md)
  and [issue 07](/epic-0-plan-remediation/issues/07-repair-the-epic-4-issue-set.md),
  both of which edit `EPIC_4.md` and Epic 4 issue bodies;
  [issue 05](/epic-0-plan-remediation/issues/05-pre-split-epic-2-issues-05-and-08.md),
  which splits the Epic 2 issue that receives the two new fields.
- **External**: the upstream tree at `936aff00`. It is not vendored here, so step 3
  begins with a clone or a `gh api` read of the frozen ref; record the command in the
  PR body.

## PR size note

`M` — roughly 200–400 changed lines of markdown across nine files, most of it in the
five Epic 4 issue bodies. Split past ~500. If the upstream verification in step 3
turns out to invalidate Epic 4 issue 01 wholesale rather than move one sentence, land
steps 1 and 2 first and open the rewrite as its own PR.

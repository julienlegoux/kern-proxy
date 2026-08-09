---
type: Decision
title: "Retry/overflow classifier and error-text sync"
description: "How the behavior-bearing error-string classifiers are resynchronized with upstream."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 07
slug: classifier-error-text-sync
status: decided
verdict: "Its own tracked unit in the final milestone, after the adapters move. Acceptance criterion is a pattern-by-pattern audit table pairing every regex in ai/retry.go and ai/overflow.go with its upstream counterpart at 936aff00. Also settles whether upstream's new provider-retry.ts supersedes ai/apis/internal/httpretry."
decided_via: triage
depends_on: [core-types-migration]
---

# Question

`src/utils/retry.ts` and `src/utils/overflow.ts` both changed in range, and
upstream added `src/utils/provider-retry.ts` and `src/utils/error-body.ts`
changes alongside.

This is the highest-risk-per-line area of the whole sync, for a reason recorded
in both planning documents. From `CONVENTIONS.md`:

> Error strings stay sentence-cased and upstream-verbatim ("Request was aborted",
> "No API key for provider: %s") because `ai/retry.go` and `ai/overflow.go`
> classify failures by matching that exact text and tests assert it. Rewriting
> error text is a behavior change.

`staticcheck`'s ST1005 is disabled specifically to protect this. And `retry.go`
carries its own warning that status-code patterns (`429`, `500`) are *text*
patterns, valid only against strings known to start with a status code — a 400
rejecting `max_tokens: 15000` contains the digits "500".

So an upstream change to a classifier regex or to an error string emitted by an
adapter is a silent behavior change on the Go side unless both move together.

# Options

- **Treat classifiers as their own tracked unit, ported with a diff-level
  audit** — enumerate every upstream pattern change and every error string the
  adapters emit, and verify each Go counterpart.
- **Port with the adapters, opportunistically** — the strings live in adapter
  code, so touch them as each adapter is ported. Cheaper, but spreads a
  cross-cutting invariant across several epics and several PRs.
- **Skip** — leave classifiers at 0.80.3. Not credible: the adapters they
  classify are moving to 0.84.1, so the patterns would be matching text that no
  longer gets emitted.

# Recommendation

**Its own tracked unit with a diff-level audit**, sequenced *after* the adapter
epics ([06](/scope/06-adapter-updates.md)) rather than before. The classifiers
match strings the adapters produce; auditing them before the adapters move means
auditing against text that is about to change.

Concretely, the acceptance criterion should be a table: every regex in
`ai/retry.go` and `ai/overflow.go` paired with the upstream pattern it mirrors at
`936aff00`, and every changed pattern justified. `retry_test.go` and
`overflow_test.go` are already table-driven (`CONVENTIONS.md` names them as two
of the 19 legitimate table-driven tests), so the new cases have an obvious home.

Also in this unit: upstream's new `utils/provider-retry.ts`. kern-link has
`ai/apis/internal/httpretry` — new code with no upstream counterpart, extracted
from the Codex adapter. Whether upstream's version supersedes it, or the two
coexist, is a real question the epic must answer rather than assume, and it is
the kind of thing that becomes a `docs/planning/DRIFT.md` entry if they diverge.

# Verdict

**Own tracked unit, sequenced after the adapter epics.** Accepted as
recommended; lands in milestone 6.

The acceptance criterion is concrete and deliberately mechanical: a table
pairing every regex in `ai/retry.go` and `ai/overflow.go` with the upstream
pattern it mirrors at `936aff00`, with every difference justified.
`retry_test.go` and `overflow_test.go` are already table-driven — two of the 19
legitimate table-driven tests `CONVENTIONS.md` names — so new cases have an
obvious home.

Sequencing after the adapters is the point of the decision: these patterns match
strings the adapters emit, and auditing them first would audit against text
about to change.

Also settled here: whether upstream's new `utils/provider-retry.ts` supersedes
kern-link's `ai/apis/internal/httpretry` (new code with no upstream counterpart,
extracted from the Codex adapter) or coexists with it. If they diverge, that is a
`docs/planning/DRIFT.md` entry rather than a silent difference.

This is risk 2 in [decision 21](/scope/21-risks-and-assumptions.md) — a missed
pattern produces a library that stops retrying something it used to retry, with
every test still green.

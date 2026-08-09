---
type: Decision
title: "Test port coverage"
description: "How much of the ~70 changed upstream test files is ported alongside the code."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 15
slug: test-port-coverage
status: decided
verdict: "Port tests for every ported behavior; skip tests for deferred features and non-portable idioms, each with a one-line reason in docs/PORTING.md. Tests land in the same PR as the behavior. No coverage percentage target."
decided_via: triage
depends_on: [parity-bar]
---

# Question

Roughly 70 files under `packages/ai/test/` changed or were added in range.
`docs/PORTING.md`'s sync procedure says, at step 3, to "port the change **and its
new tests**" — so the intent already exists; what needs deciding is the bar,
because 70 test files is a large fraction of the program's total work.

Relevant existing facts: kern-link has 102 test files against 126 non-test source
files. `CONVENTIONS.md` mandates standard-library `testing` only, discrete named
test functions over table-driven ones (28 `t.Run` calls across 102 files), no
`t.Parallel()`, no build tags, and `t.Skip`-gated live tests. So upstream tests
are never copied — they are *re-expressed* in a different idiom, which makes
per-file cost higher than a diff suggests.

Some upstream tests also aren't portable at all. `docs/PORTING.md` already records
one class: tests feeding `{ type: "unknown" }` content blocks, which Go's closed
content interfaces structurally cannot represent.

# Options

- **Port every changed upstream test** — maximum confidence, largest cost,
  includes tests that are meaningless in Go.
- **Port tests for every ported behavior; skip tests for deferred features and
  non-portable idioms** — coverage tracks the parity bar rather than the diff.
- **Port none; rely on the existing 102 files** — the existing suite tests
  0.80.3 behavior and would not catch the semantic changes being introduced.

# Recommendation

**Port tests for every ported behavior; skip the rest with a reason.** This is
the same shape as [decision 02](/scope/02-parity-bar.md)'s disposition rule, and
it keeps the two consistent: if a feature is deferred, its tests are deferred
with it; if a feature ships, it ships with tests.

Two rules to carry into the epics so this stays enforceable rather than
aspirational:

- **Tests land in the same PR as the behavior**, not a follow-up. `implement-issue`
  runs strict red-green TDD against acceptance criteria, so this is the pipeline's
  default anyway — the point is not to write acceptance criteria that let it slide.
- **A skipped upstream test gets one line in `docs/PORTING.md`**, in the same
  place the non-portable-idiom exclusions already live. That is what makes the
  next sync able to tell "we decided against this" from "we missed this".

Explicitly *not* recommended: a coverage percentage target. The repo has no
coverage configuration, threshold, or reporting today (`SPECS.md`), and
introducing one here would be a new project-wide standard smuggled in through a
sync program — out of scope for this decision, and better raised on its own.

# Verdict

**Coverage tracks the parity bar, not the diff.** Accepted as recommended.

Tests are ported for every ported behavior; tests for deferred features are
deferred with them, and non-portable idioms are skipped. Two rules make this
enforceable rather than aspirational:

- **Tests land in the same PR as the behavior.** `implement-issue` runs strict
  red-green TDD against acceptance criteria, so this is the pipeline's default —
  the discipline required is not writing acceptance criteria that let it slide.
- **Every skipped upstream test gets one line in `docs/PORTING.md`**, beside the
  non-portable-idiom exclusions already recorded there. That is what lets the
  next sync distinguish "decided against" from "missed".

Explicitly rejected: a coverage percentage target. The repo has no coverage
configuration, threshold, or reporting today, and adding one here would smuggle a
project-wide standards change in through a sync program — the same reasoning that
keeps the golangci-lint battery out under
[decision 18](/scope/18-non-goals.md).

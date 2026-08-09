---
type: Decision
title: "Risks and assumptions"
description: "What could sink this program, and what it assumes without proof."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 21
slug: risks-and-assumptions
status: decided
verdict: "Six risks and four assumptions recorded, each attached to the milestone that owns its mitigation rather than listed flat."
decided_via: triage
depends_on: [milestones-phasing]
---

# Question

Checklist area 11. The audit surfaced several things that are cheap to write
down now and expensive to discover in milestone 4.

# Options

- **Record them, with the mitigation baked into the milestone that owns each.**
- **Record them as a flat list in SCOPE.md** — visible, but nothing acts on them.

# Recommendation

**Record them against their owning milestone.** The list:

**Risks**

1. **`tools/export-catalog` may not run against `936aff00` at all.** Five
   generator scripts changed upstream. If the export tool reads their output
   shape, milestone 2 starts with a tooling repair of unknown size. *Mitigation:
   make "the export tool runs green against the frozen SHA" the first issue of
   milestone 2, so the unknown is priced before the rest of the milestone is
   planned.*
2. **Silent classifier regressions.** Retry and overflow behavior is matched on
   error *text*; a changed upstream string with no matching Go update produces a
   library that stops retrying something it used to retry, with every test still
   green. *Mitigation: milestone 6's pattern-by-pattern audit table.*
3. **Non-exhaustive `StopReason` switches.** `exhaustive` is deliberately off in
   `.golangci.yml`, so adding `"pending"`/`"deferred"` will not produce a single
   compiler or linter error at the sites that need updating. *Mitigation: a
   manual grep sweep as an explicit acceptance criterion in milestone 1.*
4. **A long-lived integration branch.** Six milestones of breaking work with no
   releasable artifact until the last one is a long time for `develop` to diverge
   from `main`. *Mitigation: keep merging each milestone into `develop` as it
   greens rather than holding a program-wide branch; `main` simply waits.*
5. **Upstream keeps moving.** By the time milestone 6 lands, `936aff00` will
   itself be behind. *Accepted, not mitigated —
   [decision 01](/scope/01-sync-target-and-freeze.md) chose a knowable scope over
   a moving one, and [17](/scope/17-upstream-lock-and-weekly-job.md) makes the
   next issue open automatically.*
6. **Deferred items accumulating into a second port debt.** Radius, pi-messages,
   and three OAuth flows are deferred here; deferring them again next sync turns
   a decision into a habit. *Mitigation: each deviation entry carries a concrete
   revisit trigger, per the drift-register format.*

**Assumptions**

- `936aff00` is a coherent upstream state, not mid-refactor. Not verified beyond
  it being `origin/main` at a point in time.
- Upstream's own test suite passes at `936aff00`; the ported tests inherit its
  correctness.
- No downstream consumer is pinned to kern-link v0.1.1 in a way that makes a
  breaking v0.2.0 costly — true today, since future Kern Lx is the only planned
  consumer and does not exist yet.
- The four deferred OAuth flows serve providers reachable by API key, so
  deferring them removes convenience, not access. Verified for `openrouter` and
  `xai` from the source; **not** verified for `kimi-coding`, which is worth a
  one-line check during milestone 5.

# Verdict

**Recorded as listed, each risk attached to its owning milestone.** Accepted as
recommended.

The mitigations that become real work rather than notes:

- Risk 1 → the tooling spike is the **first issue of milestone 2**
  ([decision 05](/scope/05-catalog-schema-and-tooling.md)).
- Risk 2 → the pattern audit table is milestone 6's acceptance criterion
  ([decision 07](/scope/07-classifier-error-text-sync.md)).
- Risk 3 → the manual grep sweep is an acceptance criterion in milestone 1
  ([decision 04](/scope/04-core-types-migration.md)).
- Risk 4 → merge each milestone into `develop` as it greens; `main` waits.
- Risk 6 → every deviation entry carries a concrete revisit trigger, per the
  drift-register format in the pipeline interfaces.

Risk 5 (upstream keeps moving) is **accepted, not mitigated** — the deliberate
consequence of [decision 01](/scope/01-sync-target-and-freeze.md).

The unverified assumption worth acting on: `kimi-coding` may have no API-key
path, which would make deferring its OAuth flow a loss of access rather than of
convenience. One line of checking during milestone 5 settles it.

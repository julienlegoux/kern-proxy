---
type: Decision
title: "Non-goals"
description: "What this sync program explicitly does not do, so it cannot creep back in via issues."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 18
slug: non-goals
status: decided
verdict: "Eight non-goals. The two deferral entries from the original list are struck after decisions 9, 10 and 13 reversed to full porting; what remains excludes only structurally non-portable upstream files, plus project-hygiene work that would be smuggled in through a sync."
decided_via: discussion
depends_on: [new-oauth-flows, new-provider-bindings, pi-messages-adapter, test-port-coverage]
---

# Question

A sync program touching 232 upstream files across every package in the module is
an unusually strong attractor for adjacent work. Without written non-goals, each
"while we're in here" is individually reasonable and collectively fatal.

This decision collects the exclusions the other decisions imply, plus the
adjacent temptations this particular codebase raises, so `create-issues` has an
explicit boundary.

# Options

- **Enumerate non-goals now, tied to the decisions that produced them.**
- **Leave it implicit** — rely on epics to be self-limiting. They are not; the
  audit found three plausible creep vectors before any epic exists.

# Recommendation

**Enumerate.** *(Revised 2026-08-09: the original list opened with the radius
and OAuth-flow deferrals as non-goals 2 and 3. Decisions
[09](/scope/09-new-oauth-flows.md), [10](/scope/10-new-provider-bindings.md) and
[13](/scope/13-pi-messages-adapter.md) reversed to full porting, so those two
entries are struck and the list renumbered.)*

Non-goals, each traceable to a decision or an existing document:

1. **Upstream commits after `936aff00`** — the target is frozen
   ([01](/scope/01-sync-target-and-freeze.md)). New upstream work is the next
   sync's issue.
2. **Bundler- and runtime-shim files** — `*.lazy.ts`, `bun-oauth.ts`,
   `compat/*`. These are the *only* admissible kind of exclusion under
   [02](/scope/02-parity-bar.md)'s tightened rule: structurally meaningless in
   Go, not merely expensive. Continues exclusions `docs/PORTING.md` already
   records.
3. **Compatibility shims for the breaking v0.2.0 changes**
   ([03](/scope/03-release-and-breaking-strategy.md)).
4. **New direct dependencies** — a hard constraint; one would reopen a scope
   decision rather than be absorbed by an epic
   ([19](/scope/19-constraints.md)).
5. **Enabling the deferred golangci-lint battery** — `.golangci.yml`'s own
   comment records revive/gocognit/exhaustive as acknowledged future work. The
   temptation is real here because [04](/scope/04-core-types-migration.md) notes
   `exhaustive` would have caught the `StopReason` switch sites. Turning it on is
   a project-wide standards change; it belongs in its own change, not smuggled in
   through a sync.
6. **Coverage tooling or thresholds** ([15](/scope/15-test-port-coverage.md)).
7. **Refactoring the testbed, the CLI, or `docs/` beyond what the sync forces.**
8. **Idiomatic-Go cleanups of ported code** — `CONVENTIONS.md`'s governing rule
   is that upstream's shape outranks Go idiom in ported files. A sync is the
   worst possible time to relitigate that, because it maximizes diff noise
   against the upstream the next sync must diff against.

# Verdict

**The eight non-goals above.** Accepted at triage 2026-08-09, then **refreshed
after its inputs landed** — which is why this doc was deliberately held open
through the deep-dives rather than finalized on the triage accept.

What changed between the triage accept and this verdict:

- **Struck: "radius as one deferred unit" and "the openrouter, kimi-coding and
  xai OAuth flows".** Decisions [09](/scope/09-new-oauth-flows.md),
  [10](/scope/10-new-provider-bindings.md) and
  [13](/scope/13-pi-messages-adapter.md) reversed to full porting, so these are
  now scope, not exclusions.
- **Added: new direct dependencies**, promoted from an implicit constraint to an
  explicit non-goal by [19](/scope/19-constraints.md)'s hard-constraint verdict.
- **Sharpened: the bundler/runtime-shim exclusion** now carries the reason it is
  admissible at all — structural non-portability, the only ground
  [02](/scope/02-parity-bar.md) accepts after its refresh.

The residual list is deliberately of one kind: things with no meaning in Go, and
project-hygiene changes that would ride in on a sync without being decided on
their own merits. Nothing is excluded for being large or slow.

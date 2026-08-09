---
type: Decision
title: "Core user journeys"
description: "Checklist area 4 — the library's API flows are unchanged in shape; new surfaces are covered per-feature."
tags: [decision, scope]
timestamp: 2026-08-09T01:16:29Z
phase: scope
decision: 25
slug: core-user-journeys
status: na
verdict: null
decided_via: na
depends_on: []
---

# Question

The 2–5 flows that must work end to end for this to mean anything.

# Options

N/A — see below.

# Recommendation

**N/A as a scoping decision, because the flows already exist and are already
tested.** `SPECS.md` documents the one request flow hop by hop (lookup → stream
entry → provider binding → auth → adapter selection → wire → events), and the
`faux` provider is described in its own docs as "the executable specification of
the event contract". A version sync does not introduce new journeys; it changes
what happens inside the existing one.

This is the checklist area most at risk of a *wrong* N/A, so the reasoning is
worth being explicit about: the sync does add new consumer-facing surface —
deferred tools, constrained sampling — but each is scoped as its own decision
([11](/scope/11-deferred-tools.md), [12](/scope/12-constrained-sampling.md))
where its acceptance criteria belong. Restating them as "journeys" here would
duplicate those decisions rather than add coverage.

What replaces this area's usual function — seeding acceptance criteria — is
[decision 02](/scope/02-parity-bar.md), which defines the program's observable
done condition.

# Verdict

N/A — flows are unchanged and already specified by `SPECS.md` and the `faux`
provider; new surfaces are scoped per-feature in decisions 11 and 12.

Confirmed as N/A by the user at triage, 2026-08-09, as part of an explicit
review of the N/A list separate from the open-decision triage. The user was
shown the alternative — reopening this to give deferred tools and constrained
sampling their own end-to-end journey criteria — and declined it.

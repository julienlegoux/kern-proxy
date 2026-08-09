---
type: Decision
title: "Target users"
description: "Checklist area 2 — inherited unchanged from the existing library."
tags: [decision, scope]
timestamp: 2026-08-09T01:16:29Z
phase: scope
decision: 23
slug: target-users
status: na
verdict: null
decided_via: na
depends_on: []
---

# Question

Who is this for, specifically?

# Options

N/A — see below.

# Recommendation

**N/A: inherited, not decided.** The audience is unchanged by a version sync: Go
developers consuming `github.com/julienlegoux/kern-link` via `go get`, with
future Kern Lx as the driving downstream consumer, and the maintainer as the
first user.

The one place audience *could* have bitten — whether Kern Lx's needs should
decide which upstream features are in scope — was raised at the premise gate and
explicitly declined in favour of upstream parity as the goal. The in/out calls
are therefore made per-feature in decisions
[09](/scope/09-new-oauth-flows.md)–[14](/scope/14-images-surface.md), on cost and
coherence rather than on audience.

# Verdict

N/A — unchanged by the sync; the audience-driven alternative was declined at the
premise gate.

Confirmed as N/A by the user at triage, 2026-08-09, as part of an explicit
review of the N/A list separate from the open-decision triage.

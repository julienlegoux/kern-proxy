---
type: Decision
title: "Delivery form"
description: "Checklist area 3 — unchanged: a Go module consumed with go get."
tags: [decision, scope]
timestamp: 2026-08-09T01:16:29Z
phase: scope
decision: 24
slug: delivery-form
status: na
verdict: null
decided_via: na
depends_on: []
---

# Question

What does the user actually receive — web app, CLI, library, service?

# Options

N/A — see below.

# Recommendation

**N/A: unchanged by this program.** `SPECS.md` opens by stating it: kern-link is
"a **library, not a service**", consumed with `go get`, with no server, no
database, and no deployed artifact. `cmd/pi-ai` (a three-command CLI) and the
gitignored `testbed/` are unchanged by the sync.

The one delivery-adjacent question this program *does* raise — how the breaking
changes are versioned and released — is not a form question and is decided
separately in [decision 03](/scope/03-release-and-breaking-strategy.md).

# Verdict

N/A — the delivery form is unchanged; versioning is handled by decision 03.

Confirmed as N/A by the user at triage, 2026-08-09, as part of an explicit
review of the N/A list separate from the open-decision triage.

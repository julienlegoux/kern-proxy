---
type: Decision
title: "Sync target and freeze"
description: "Which upstream revision this program targets, and whether that target is frozen for its duration."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 01
slug: sync-target-and-freeze
status: decided
verdict: "Freeze at 936aff00 (v0.84.1) for the program's duration; one program, no staged walk through intermediate minor tags."
decided_via: triage
depends_on: []
---

# Question

`upstream/UPSTREAM.lock` pins `244f1deaf1ae` (`@earendil-works/pi-ai` v0.80.3).
Upstream `origin/main` is `936aff00` (v0.84.1) as of 2026-08-09. The diff over
`packages/ai` is 232 files, +19531/-21423, across 207 commits.

Upstream is a live repository: it moved from the state GitHub issue #113
described on 2026-07-13 to a substantially larger diff by 2026-08-09, and it
will keep moving while this program runs. Every downstream decision — what is
in scope, how many epics, what "done" means — is defined against a specific
upstream revision. If that revision floats, the scope silently grows under the
implementation.

A second question rides along: one jump to 0.84.1, or a staged walk through the
intermediate minor tags (0.81, 0.82, 0.83, 0.84)?

# Options

- **Freeze at 936aff00 (v0.84.1), one program** — scope is fixed and knowable;
  new upstream commits are explicitly the *next* sync's problem.
- **Chase `origin/main` throughout** — ends closer to upstream, but the target
  moves under every epic and the program has no definable finish line.
- **Staged walk through minor tags** — each hop is smaller and independently
  verifiable, but four sequential catch-ups over the same files means porting
  some code two or three times as upstream refactored it mid-range.

# Recommendation

**Freeze at `936aff00` (v0.84.1), one program.** The whole reason `define-change`
handed this to `define-scope` is that the scope needs to be knowable before it
can be cut into epics, and it cannot be knowable against a moving ref. Record the
frozen SHA in the scope document itself so every epic inherits it.

Reject the staged walk on evidence from the diff: `src/utils/oauth/*` moved to
`src/auth/oauth/*` and `StreamOptions` was refactored onto
`ProviderRequestOptions` *within* the range. Replaying intermediate states means
porting code that upstream itself deleted before 0.84.1 — cost with no parity
benefit.

The weekly job will keep opening `upstream-sync` issues against the still-pinned
lock while this runs; [decision 17](/scope/17-upstream-lock-and-weekly-job.md)
handles that.

# Verdict

**Freeze at `936aff00` (v0.84.1), one program.** Accepted as recommended.

The frozen SHA is recorded in `SCOPE.md` so every epic inherits the same target.
The staged walk is rejected on the evidence cited above: upstream moved
`src/utils/oauth/*` to `src/auth/oauth/*` and refactored `StreamOptions` onto
`ProviderRequestOptions` *within* the range, so replaying intermediate states
would mean porting code upstream itself deleted before 0.84.1.

Upstream commits landing after `936aff00` are explicitly out of scope
([decision 18](/scope/18-non-goals.md)) and become the next sync's issue.

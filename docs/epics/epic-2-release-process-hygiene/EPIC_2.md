---
type: Epic
title: "Release & process hygiene"
description: "Adoption-readiness: golangci-lint in CI, a documented credential-mode ToS risk note, and a v0.1.0 tag that lands after everything else."
tags: [epic]
timestamp: 2026-07-09T16:30:00Z
resource: https://github.com/julienlegoux/kern-proxy/issues/86
epic: 2
slug: release-process-hygiene
status: done
gh_issue: 86
milestone: 17
source: docs/PLAN.md#epic-2--release--process-hygiene
---

# Epic 2: Release & process hygiene

## Goal

The adoption review flagged several adoption-readiness gaps that aren't code
bugs: no linter runs in CI, the README documents OAuth login without any
terms-of-service caveat, and the module has no tags so `go get` pins a
pseudo-version of `main`. This epic closes those gaps and cuts the first
tagged release.

## Scope

### 2.1 Add a linter to CI

No linter exists (no `.golangci.yml`, no lint step in
`.github/workflows/test.yml`). Add golangci-lint with a minimal config and a
lint job in `test.yml` (toolchain from `go.mod`, Go 1.25). Fix or explicitly
ignore whatever the first run flags — no blanket exclusions.

### 2.2 Document credential-mode risk

README documents OAuth login (`README.md`, Authentication section) with no
terms-of-service caveat. Add a short "credential modes" note to the README
Authentication section and the auth & credentials doc (`docs/auth.md`):

* API-key paths carry no ToS risk.
* Subscription-OAuth paths (Claude Pro/Max, ChatGPT Plus/Pro, GitHub Copilot) impersonate first-party clients — inherited upstream behavior, fine for personal use, account-revocation risk if shipped in a product.

### 2.3 Tag `v0.1.0`

No tags exist; `go get` currently pins a pseudo-version of `main`. Tag
`v0.1.0` with a minimal release note **after** the other items in this plan
land.

## Out of scope

The plan does not state Epic-2-specific exclusions; see Notes for the
project-wide non-goals that bound this epic.

## Acceptance criteria

The plan states approaches rather than formal criteria; done means:

- A lint job runs in `.github/workflows/test.yml` using golangci-lint with a minimal `.golangci.yml`, and its first-run findings are fixed or explicitly (not blanket) ignored.
- The credential-modes note (API-key = no ToS risk; subscription-OAuth = first-party impersonation with account-revocation risk in products) appears in both the README Authentication section and `docs/auth.md`.
- `v0.1.0` is tagged with a minimal release note, strictly after every other item in the plan (both epics) has landed.

## Dependencies

Depends on [Epic 1](/epic-1-reliability-api-fixes/EPIC_1.md): the plan orders
the epics and states that 2.3 (the tag) must land last, after all other items
in both epics. 2.1 and 2.2 have no hard dependency on Epic 1's code changes,
but the release itself gates on everything.

## Notes

Project-wide non-goals from the plan (not specific to this epic — do not
resurrect these when splitting issues):

* **Unbounded event queue on the `Result`-only path** — documented contract; at most a godoc sentence noting the memory behavior.
* **Restructuring `StreamOptions`, `map[string]*string` headers, int64-millisecond timestamps** — deliberate port-fidelity decisions.
* **Maturity concerns (stars, bus factor, usage history)** — not addressable by code; the plan notes they are mitigated over time and by 2.1 (lint CI) and 2.3 (tagged release), which is part of why this epic exists.

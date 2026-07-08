---
type: Epic
title: "CLI & upstream-sync tooling"
description: "cmd/pi-ai CLI (login/list/help), completed PORTING.md, upstream sync script + CI job, and full end-to-end verification."
tags: [epic]
timestamp: 2026-07-08T17:30:00Z
epic: 14
slug: cli-sync-tooling
status: done
gh_issue: 15
milestone: 14
resource: https://github.com/julienlegoux/kern-proxy/issues/15
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 14: CLI & upstream-sync tooling

## Goal

Finish the user-facing surface and the standing upstream-sync machinery: the `pi-ai` CLI, the completed porting map, and the automated diff-against-upstream workflow that keeps the Go port in sync with `@earendil-works/pi-ai` over time.

## Scope

- `cmd/pi-ai` CLI: `login` / `list` / `help`.
- PORTING.md completed: full TS-file → Go-package map + intentional deviations (including the skipped `compat.ts` / `legacy-api-aliases.ts` shims).
- `upstream/sync.sh` finished + a CI job (or Routine) that runs the diff weekly and opens an issue when non-empty.
- Standing sync procedure documented: run script → port each semantic change with its new tests → regen catalog JSON → bump `UPSTREAM.lock` SHA → full suite + gated live smokes. Go tags mirror upstream releases (`v0.80.3-go.N`).

## Out of scope

- New features beyond upstream parity.

## Acceptance criteria

- Full test suite + env-gated e2e green.
- End-to-end: `pi-ai list` works; a small example program streams a tool-call round-trip through faux and (if a key is present) a real provider.

## Dependencies

- Everything: `login` needs [Epic 12](/epic-12-oauth-flows/EPIC_12.md), `list` needs [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md), e2e needs the adapters (epics 5–10).

## Notes

- Size: S — deliberately kept as its own epic (one epic per plan phase, per project decision).
- `upstream/UPSTREAM.lock` and `upstream/sync.sh` already exist as scaffolding in the repo; this epic finishes and automates them.
- PORTING.md currently lives at `docs/PORTING.md` (moved from the repo root, uncommitted at time of writing) — reconcile the location when completing it.

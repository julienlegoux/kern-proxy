---
type: Issue
title: "Finish upstream-sync tooling and complete PORTING.md"
description: "Finish upstream/sync.sh, add the weekly diff CI job that opens an issue when non-empty, document the sync procedure, and complete the PORTING.md map."
tags: [epic-14]
timestamp: 2026-07-07T08:28:09Z
epic: 14
issue: 02
slug: upstream-sync-porting-md
size: M
status: open
gh_issue: 45
resource: https://github.com/julienlegoux/kern-proxy/issues/45
depends_on: []
blocked_by: ["#6", "#7", "#8", "#9", "#10", "#11", "#12", "#13", "#14"]
---

# Finish upstream-sync tooling and complete PORTING.md

## Summary

Finish the standing upstream-sync machinery of [Epic 14](/epic-14-cli-sync-tooling/EPIC_14.md): complete the existing `upstream/sync.sh` scaffolding, automate it as a weekly CI job that opens an issue when the diff is non-empty, document the sync procedure, and bring `docs/PORTING.md` to its final complete state.

## Scope

- `upstream/sync.sh` finished: diff pinned SHA (`upstream/UPSTREAM.lock`)..HEAD over `packages/ai`, bucketed by area.
- Weekly CI job (GitHub Actions schedule, extending the test workflow from Epic 3's #17, or a Routine) that runs the diff and opens a GitHub issue when non-empty.
- Standing sync procedure documented: run script → port each semantic change with its new tests → regen catalog JSON (Epic 11's export-catalog) → bump `UPSTREAM.lock` SHA → full suite + gated live smokes. Go tags mirror upstream releases (`v0.80.3-go.N`).
- PORTING.md completed: full TS-file → Go-package map, all statuses current, intentional deviations recorded (including the skipped `compat.ts` / `legacy-api-aliases.ts` shims); reconcile its location (currently `docs/PORTING.md`, moved from the repo root).

## Out of scope

- The CLI — [Issue 01](./01-pi-ai-cli.md).
- Porting any upstream changes the diff surfaces — those become their own issues via the documented procedure.

## Acceptance criteria / Definition of done

- `upstream/sync.sh` runs clean against the pinned checkout; the scheduled job is green and demonstrably opens an issue on a non-empty diff (test with a deliberately stale SHA).
- PORTING.md has no stale rows; every shipped package is `ported`, deviations documented.
- Full test suite + env-gated e2e green — the epic's acceptance criteria.

## Relevant files / areas

- Existing scaffolding: `upstream/sync.sh`, `upstream/UPSTREAM.lock`; `docs/PORTING.md`; `.github/workflows/` (from Epic 3's #17).

## Dependencies

None within this epic. Meaningful only near the end: "complete PORTING.md" implies epics 5–13 have landed their rows.

## PR size note

Target ~500 changed lines; mostly script + workflow + docs.

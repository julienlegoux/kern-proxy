---
type: Issue
title: "Run the test suite under -race in CI"
description: "Minimal GitHub Actions workflow running go test ./... -race on Linux so the concurrent-refresh race test actually executes."
tags: [epic-3]
timestamp: 2026-07-07T07:40:00Z
epic: 3
issue: 02
slug: run-race-tests-in-ci
size: S
status: open
gh_issue: 17
resource: https://github.com/julienlegoux/kern-proxy/issues/17
depends_on: []
---

# Run the test suite under -race in CI

## Summary

[Epic 3: Auth core](/epic-3-auth-core/EPIC_3.md)'s acceptance criteria require the concurrent-refresh test (`TestResolveExpiredOAuthRefreshesOnceUnderLock` in `ai/resolve_test.go`) to pass under `go test -race`. That is blocked on the Windows dev machine (race mode needs cgo plus a C compiler in `PATH`), and no CI exists yet. Add a minimal GitHub Actions workflow that runs the race-enabled suite on Linux, satisfying the plan's project-wide verification default (`go test ./... -race`).

## Scope

- `.github/workflows/test.yml`: on push/PR, `ubuntu-latest`, setup-go pinned to the module's Go version, `go test ./... -race`.
- Confirm the concurrent-refresh test passes under `-race` in the workflow run.

## Out of scope

- The upstream-sync CI job and any release/lint tooling — [Epic 14: CLI & upstream-sync tooling](/epic-14-cli-sync-tooling/EPIC_14.md). This workflow is the minimal test gate only; Epic 14 extends it rather than replacing it.
- Making `-race` work locally on Windows.

## Acceptance criteria / Definition of done

- Workflow runs green on this branch with `-race` enabled, no network needed (faux provider + httptest fixtures only).
- `TestResolveExpiredOAuthRefreshesOnceUnderLock` visibly executes and passes in the run log.

## Relevant files / areas

- New: `.github/workflows/test.yml` — no CI configuration exists in the repo yet; the path follows standard GitHub Actions convention.
- Read-only context: `go.mod` (Go version), `ai/resolve_test.go:135` (the race test).

## Dependencies

None — the race test already exists. Once [Issue 01](./01-flock-locked-file-credential-store.md) merges, its file-lock concurrency tests get race coverage from this same workflow for free.

## PR size note

Target ~500 changed lines; this one should be well under 100. If it grows, it's scope creep — keep it to the test workflow.

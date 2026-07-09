---
type: Issue
title: "Add golangci-lint to CI"
description: "Add a minimal .golangci.yml and a lint job to .github/workflows/test.yml, and fix or explicitly ignore whatever the first run flags."
tags: [epic-2]
timestamp: 2026-07-09T15:25:00Z
resource: https://github.com/julienlegoux/kern-proxy/issues/95
epic: 2
issue: 1
slug: golangci-lint-ci
size: M
status: done
gh_issue: 95
gh_pr: 106
depends_on: []
---

# Add golangci-lint to CI

## Summary

No linter runs anywhere: there is no `.golangci.yml` and no lint step in
`.github/workflows/test.yml`. The adoption review flagged this as an
adoption-readiness gap. Add golangci-lint with a minimal config and make it a
required CI job.

## Scope

- Add a minimal `.golangci.yml` — start from golangci-lint's default enabled set; don't enable a large custom battery in the first pass.
- Add a `lint` job to `.github/workflows/test.yml` using the official golangci-lint action, with the toolchain taken from `go.mod` (Go 1.25), matching how the existing test job sets up Go.
- Run it once locally and fix everything it flags. Where a finding is a deliberate port-fidelity decision, add a targeted `//nolint:<linter> // <reason>` or a scoped exclusion in `.golangci.yml` — **no blanket exclusions**.

## Out of scope

- Enabling aggressive/opinionated linters (exhaustive, gocognit, etc.) — minimal config first; tightening is future work.
- Any behavioral code change beyond what a lint fix mechanically requires.
- The v0.1.0 tag — [Issue 03](./03-tag-v0-1-0.md).

## Acceptance criteria / Definition of done

- `.golangci.yml` exists with a minimal config; `golangci-lint run ./...` passes locally.
- The lint job runs in `.github/workflows/test.yml` on the same triggers as the test job and passes on the PR itself.
- Every suppression is targeted and carries a reason; no blanket path or linter exclusions.
- `go test -race ./...` still clean after any lint-driven fixes.

## Relevant files / areas

- `.github/workflows/test.yml`, new `.golangci.yml`
- Whatever files the first lint run flags (unknown until it runs — if fixes balloon the diff past ~1000 lines, land config + CI job with targeted nolints first and split the fixes into a follow-up PR)

## Dependencies

None within this epic; blocks [Issue 03](./03-tag-v0-1-0.md). Part of
[Epic 2](/epic-2-release-process-hygiene/EPIC_2.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.

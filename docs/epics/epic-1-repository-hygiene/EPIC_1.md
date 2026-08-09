---
type: Epic
title: "Repository hygiene"
description: "Move the module path to github.com/kern-ia/kern-link and bump the go directive to 1.26, before any sync work opens branches against the old path."
tags: [epic]
timestamp: 2026-08-09T03:55:05Z
epic: 1
slug: repository-hygiene
status: open
gh_issue: 118
milestone: 18
resource: https://github.com/kern-ia/kern-link/issues/118
source: docs/planning/SCOPE.md#milestone-1-repository-hygiene
---

# Epic 1: Repository hygiene

## Goal

Mechanical, no semantic change, and first because it conflicts with every
in-flight branch. The repository, its tags and its releases already live at
`kern-ia`; only the Go module path still says `julienlegoux`. Doing this rename
after the sync epics have opened branches means resolving the same 355-reference
conflict once per branch.

## Scope

- Move the module path from `github.com/julienlegoux/kern-link` to
  `github.com/kern-ia/kern-link` — `go.mod`, every import, README, CHANGELOG,
  `docs/`. **355 references.**
- Bump the `go` directive from 1.25.0 to 1.26, to align with the other Kern
  packages. CI pins from `go-version-file: go.mod`, so the directive carries the
  workflow with it.
- Commit as `refactor!: move module path to github.com/kern-ia/kern-link`.

## Out of scope

- **No `/v2` suffix.** The project is v0.x, so the major-version suffix rule
  does not apply. Consumers pinned to `github.com/julienlegoux/kern-link@v0.1.1`
  keep working — module versions are immutable in `proxy.golang.org`.
- **Historical CHANGELOG entries for already-released versions are not
  rewritten.** They describe what was published under the old path, and that
  remains true.
- Any semantic change riding along with the rename.

## Acceptance criteria

1. `go.mod` declares `module github.com/kern-ia/kern-link` and `go 1.26`.
2. No reference to `github.com/julienlegoux/kern-link` remains anywhere in the
   tree, except in historical CHANGELOG entries for already-released versions.
3. CI is green: `go test ./... -race -v`, `bash upstream/sync_test.sh`, and
   `golangci-lint` (pinned v2.12.2).
4. The change lands as a single `refactor!:` commit-scoped PR, merged into
   `develop` before any other epic in this program opens a branch.

## Dependencies

None. This epic blocks every other epic in the program.

## Context

- [Technical specs](../../planning/SPECS.md)
- [Conventions](../../planning/CONVENTIONS.md)
- [Upstream sync scope](../../planning/SCOPE.md)
- [Decision 26 — module path migration](../../planning/scope/26-module-path-migration.md)
- [Decision 19 — constraints](../../planning/scope/19-constraints.md)

## Notes

Project-wide constraints that apply here rather than being this epic's own
boundary decisions:

- **`develop` is the integration trunk.** Each milestone merges into `develop`
  as it greens; one `develop → main` merge closes the program with the v0.2.0
  tag.
- **The race detector's verdict only ever arrives from CI** — there is no C
  toolchain locally, and `GOTMPDIR` must stay inside the repo. A green local run
  is not sufficient evidence.

The breaking module-path move is part of what makes the program's release a
breaking **v0.2.0**; no compatibility shim is planned for it.

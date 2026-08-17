---
type: Issue
title: "Bump the go directive from 1.25.0 to 1.26"
description: "Raise the go.mod language version to 1.26 to align with the other Kern packages, and prove CI is green on the new toolchain before the module-path rename lands."
tags: [epic-1]
timestamp: 2026-08-17T00:00:00Z
epic: 1
issue: 01
slug: bump-go-directive-to-1-26
size: S
status: in-progress
gh_issue: 127
resource: https://github.com/kern-ia/kern-link/issues/127
depends_on: []
---

# Bump the go directive from 1.25.0 to 1.26

## Summary

`go.mod` declares `go 1.25.0` while the other Kern packages are on Go 1.26
([decision 19](../../../planning/scope/19-constraints.md)). CI pins its toolchain
from `go-version-file: go.mod`, so this one line moves the whole build — the test
job, the lint job, and every later epic's compilation.

It ships first and alone, ahead of the module-path rename in
[issue 02](/epic-1-repository-hygiene/issues/02-move-module-path-to-kern-ia.md),
for one reason: a toolchain bump can surface real failures (a new vet check, a
`golangci-lint` v2.12.2 incompatibility, a changed stdlib behavior), and those
must not arrive tangled in a 170-file mechanical diff where nobody can tell
signal from noise.

## Scope

- `go.mod` line 3: `go 1.25.0` → `go 1.26`.
- Nothing else, unless CI on 1.26 fails: then fix exactly what it flags, in this
  PR, and say so in the PR body.

## Out of scope

- **The module path.** `module github.com/julienlegoux/kern-link` stays untouched
  here; that is issue 02's whole job.
- **A `toolchain` directive.** Do not add one — CI resolves the toolchain from
  the `go` directive, and pinning a second version in the same file gives two
  places to keep in sync.
- **Adopting Go 1.26 language or stdlib features.** This is a version bump, not a
  modernization pass.
- **`docs/planning/SPECS.md`'s "at Go **1.25.0**" line** (`docs/planning/SPECS.md:22`).
  That same line also names the old module path, so issue 02 rewrites it once
  with both facts corrected rather than the two PRs fighting over it.
- Widening `.golangci.yml`'s linter set, which
  [CONVENTIONS.md](../../../planning/CONVENTIONS.md) records as deliberate future
  work.

## Acceptance criteria / Definition of done

- [ ] `go.mod` line 3 reads exactly `go 1.26`, and `git diff --stat` on the
      branch lists `go.mod` as the only changed file (plus whatever a CI failure
      forced, if any).
- [ ] No `toolchain` directive appears in `go.mod`:
      `grep -c '^toolchain' go.mod` returns `0`.
- [ ] `go build ./...` and `go test ./...` pass locally, run as
      `GOTMPDIR=$PWD/.gotmp go test ./...` — `GOTMPDIR` must stay inside the repo
      (Application Control blocks test binaries built into the system temp dir),
      and `-race` is CI-only here since there is no local C toolchain
      ([CONVENTIONS.md, Tests](../../../planning/CONVENTIONS.md)).
- [ ] CI green on the PR — the `test` job (`go test ./... -race -v` and
      `bash upstream/sync_test.sh`) and the `lint` job (`golangci-lint` pinned
      `v2.12.2`) both pass. **The race detector's verdict only ever arrives from
      CI**; a green local run is not evidence.
- [ ] `gofmt -l .` prints nothing.
- [ ] The commit follows Conventional Commits with a scope, e.g.
      `chore(go): bump the go directive to 1.26`. No tool-signature trailer
      (`Co-Authored-By`, `Claude-Session`) — the git author is enough.

## Relevant files / areas

- `go.mod:3` — the `go` directive (`go 1.25.0` today).
- `.github/workflows/test.yml` — both jobs use
  `actions/setup-go@v6` with `go-version-file: go.mod`, so nothing in the
  workflow needs editing; it is the thing that proves the bump.
- `.golangci.yml` — `version: 2`, `linters.default: standard`. The pinned
  `golangci-lint v2.12.2` in `test.yml` is the likeliest source of a surprise on
  a new Go release; if it cannot parse 1.26 output, that is a finding for the PR
  body, not a silent version pin change.

## Dependencies

- **Blocked by**: None. This is the first issue of the first epic.
- **Blocks**: [Issue 02](/epic-1-repository-hygiene/issues/02-move-module-path-to-kern-ia.md)
  — both edit `go.mod`, and landing the one-line bump first keeps the rename PR's
  diff purely mechanical.

## PR size note

`S` — ~1 changed line: `go.mod:3`, `go 1.25.0` → `go 1.26`, with
`git diff --stat` expected to list that one file and nothing else. Split past
~200 — which can only happen if CI on 1.26 forces fixes, and a fix large enough
to approach that belongs in its own PR rather than riding along with the bump.

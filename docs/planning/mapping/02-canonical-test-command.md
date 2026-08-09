---
type: Decision
title: "Canonical test command"
description: "README and CI both say `go test ./... -race`, but that command cannot run on the maintainer's machine — which does CONVENTIONS.md record as the standard?"
tags: [decision, mapping]
timestamp: 2026-07-29T04:36:27Z
phase: mapping
decision: 02
slug: canonical-test-command
status: decided
verdict: "A — two-tier: `go test ./... -race` is the CI gate, local runs use `go test ./...` with an in-project GOTMPDIR; note added to README's Development section"
decided_via: triage
depends_on: []
---

# Question

The repository states one test command in two places:

- `README.md` → "## Development … `go test ./... -race`"
- `.github/workflows/test.yml` → `go test ./... -race -v`

Neither is runnable as-written on the maintainer's Windows development machine:

1. `-race` requires cgo and a C toolchain; there is no gcc installed, so the
   flag fails before any test runs.
2. Application Control blocks execution of freshly-built binaries from the
   system temp directory, so `go test` must be pointed at an in-project
   `GOTMPDIR` (the repo already gitignores `.gotmp/` for exactly this, but
   nothing explains why beyond that one-line comment).

Nothing in the repo documents either constraint. `CONVENTIONS.md` has to name a
canonical test command that a contributor — human or agent — can actually run.

# Options

**A — Two-tier: local command and CI command, both documented.** CONVENTIONS.md
records `-race` as the CI gate (authoritative, must be green before merge) and
records the local command as `go test ./...` with `GOTMPDIR` set inside the
repo, noting that `-race` is CI-only because it needs a C toolchain.

- Descriptive and honest: matches what the code and the machine actually do.
- An agent implementing an issue runs a command that works, and still knows the
  race detector is the real gate.

**B — Record only `-race`.** Keep the single documented command; treat the local
limitation as an environment problem outside the conventions doc.

- Matches README and CI verbatim, one command to remember.
- Cost: every local test run fails on first try, and the failure mode
  (toolchain error, or a blocked binary) looks like a broken repo rather than a
  known environment constraint. Downstream skills would repeatedly rediscover
  this.

**C — Drop `-race` from CI too.** Make the documented and runnable commands
identical by lowering the CI gate.

- Cost: throws away real coverage. `test.yml` carries an explicit comment that
  the race detector visibly executing
  `TestResolveExpiredOAuthRefreshesOnceUnderLock` was an acceptance criterion,
  and the credential store's cross-process locking is precisely the code a race
  detector is for. Not recommended.

# Recommendation

**A — two-tier.** The race detector is load-bearing for this codebase (OAuth
refresh under a double-checked lock, cross-process `flock` on the credential
store, concurrent `RefreshModels` sharing one in-flight fetch), so it stays the
CI gate. Recording the local command alongside it costs two lines and removes a
failure every contributor otherwise hits once. Also worth adding the same note
to `README.md`'s Development section, so the fact lives in the repo and not only
in the planning bundle.

# Verdict

**A — two-tier.** The race detector stays the authoritative gate and runs in CI
on every push; a green `-race` run is required before merge. Local runs use
`go test ./...` with `GOTMPDIR` pointed at the repo's gitignored `.gotmp/`,
because `-race` needs a C toolchain that is not installed and Application
Control blocks test binaries built into the system temp directory. Both tiers
are recorded in [CONVENTIONS](/CONVENTIONS.md), and the same note belongs in
`README.md`'s Development section so the fact lives in the repo itself.

Accepted at triage, 2026-08-08.

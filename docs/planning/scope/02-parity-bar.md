---
type: Decision
title: "Parity bar"
description: "What 'synced to 0.84.1' observably means — the acceptance bar for the whole program."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 02
slug: parity-bar
status: decided
verdict: "Behavioral parity plus a written exception list. Three gates: green CI (test -race -v, sync_test.sh, golangci-lint); every one of the 232 upstream files dispositioned as ported or deviated; UPSTREAM.lock reading 936aff00 / 0.84.1."
decided_via: triage
depends_on: [sync-target-and-freeze]
---

# Question

This is checklist area 7 (success criteria) in its brownfield form. kern-link
describes itself as a "full-parity Go port", but `docs/PORTING.md` already
carries an "Intentional deviations" section of eleven entries plus one
acknowledged gap (`src/api/github-copilot-headers.ts`, PORTING.md:46). So parity
has never meant file-for-file equivalence — it has meant *behavioral* parity with
written exceptions.

The program needs that bar stated precisely, because it is what
`create-issues` will turn into acceptance criteria and what tells the maintainer
when to stop.

# Options

- **Behavioral parity + written exception list** — every upstream semantic change
  in range is either ported or recorded in PORTING.md's deviations with a reason.
  Green CI (`go test ./... -race -v`, `sync_test.sh`, `golangci-lint`) is the gate.
- **File-for-file parity** — every upstream source file in range has a Go
  counterpart. Objective, but forces porting bundler shims (`*.lazy.ts`,
  `bun-oauth.ts`) that the port has always and correctly refused.
- **Consumer-driven parity** — port only what a downstream consumer asks for.
  Cheapest, but abandons the project's stated reason for existing and makes the
  next sync's diff even worse.

# Recommendation

**Behavioral parity plus a written exception list**, with three concrete gates:

1. `go test ./... -race -v` green in CI, plus `bash upstream/sync_test.sh` and
   `golangci-lint` — the three checks `docs/planning/CONVENTIONS.md` already
   names as the review bar.
2. Every upstream file in the 232-file range is *dispositioned*: ported, or
   listed in `docs/PORTING.md` under deviations with a reason. No file is left
   silently unaccounted for.
3. `upstream/UPSTREAM.lock` reads `commit=936aff00…` / `version=0.84.1`, which
   is the machine-checkable claim that the sync happened.

This is the bar the project already holds itself to; the value here is making
the *disposition* of all 232 files an explicit deliverable, so the next sync
starts from a known state rather than an assumption.

# Verdict

**Behavioral parity plus a written exception list.** Accepted as recommended,
with the three gates as stated:

1. `go test ./... -race -v` green in CI, plus `bash upstream/sync_test.sh` and
   `golangci-lint` — the review bar `CONVENTIONS.md` already names.
2. Every upstream file in the 232-file range is **dispositioned**: ported, or
   listed in `docs/PORTING.md` under deviations with a reason. No file left
   silently unaccounted for.
3. `upstream/UPSTREAM.lock` reads `commit=936aff00…` / `version=0.84.1`.

Gate 2 is delivered by the final milestone
([decision 20](/scope/20-milestones-phasing.md)).

**Refreshed 2026-08-09, after decisions 9, 10 and 13 reversed to full porting.**
This verdict originally read the disposition rule as making deferrals
"compatible with calling this parity". That was too permissive: it let *cost* be
a sufficient reason to enter the exception list, and the discussion on decision
09 established why that erodes the port — a deviation is not a postponement but
the loss of the Go base that the next sync's diff must apply against.

The exception list is therefore qualified: **an entry must be justified by
structural non-portability, never by cost.** Admissible: constructs with no
meaning in Go — the `*.lazy.ts` bundler shims, `bun-oauth.ts` (a Bun runtime
entry point), `compat/*` (back-compat surface upstream itself treats as
deprecated), and tests exercising shapes Go's closed content interfaces cannot
represent. Not admissible: "large", "hard to test", "no consumer asked for it".

Everything else in the 232-file range is ported.

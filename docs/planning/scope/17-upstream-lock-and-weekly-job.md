---
type: Decision
title: "UPSTREAM.lock advancement and the weekly sync job"
description: "When the pin advances, and what the weekly upstream-sync workflow does while the program runs."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 17
slug: upstream-lock-and-weekly-job
status: decided
verdict: "Advance UPSTREAM.lock once, in the final milestone, alongside the CHANGELOG entry and the v0.2.0 tag. Leave the weekly job running — its skip-if-an-issue-is-open guard keeps it quiet while #113 stays open as the umbrella tracking issue."
decided_via: triage
depends_on: [sync-target-and-freeze, milestones-phasing]
---

# Question

This is checklist area 9 (existing systems of record) in its concrete form. Three
pieces of machinery already own state this program will disturb:

1. **`upstream/UPSTREAM.lock`** — pins `commit=244f1deaf1ae`, `version=0.80.3`.
   `docs/PORTING.md` step 4 says to update it after porting. With the program
   split across several epics, "after porting" is ambiguous: after each epic, or
   once at the end?
2. **`.github/workflows/upstream-sync.yml`** — runs Mondays 06:00 UTC, diffs
   upstream against the lock, and opens an `upstream-sync`-labeled issue when the
   diff is non-empty **and no such issue is already open**. Issue **#113** is
   currently open and is what started this program.
3. **`upstream/sync_test.sh`** — runs on every push as a CI gate, offline-testing
   the diff-detection logic. Untouched by this program, but it will fail loudly
   if `sync.sh` or the lock format changes.

# Options

- **Advance the lock once, at the end** — the lock always describes a state the
  code actually reaches. But for the program's whole duration it says 0.80.3,
  and the weekly job keeps measuring against a pin the work has already passed.
- **Advance per epic** — the lock tracks progress. But it would then claim a
  version the port only partially implements, which is exactly the ambiguity
  [decision 02](/scope/02-parity-bar.md) exists to remove.
- **Advance once at the end, and pause the weekly job meanwhile** — as above,
  plus no redundant issues during the program.

# Recommendation

**Advance `UPSTREAM.lock` once, in the program's final epic, and leave the
weekly job running.**

The lock is a claim about what the port implements; a partially-synced tree that
claims 0.84.1 would mislead the next sync into skipping work. So it moves last,
together with the CHANGELOG entry and the v0.2.0 release from
[decision 03](/scope/03-release-and-breaking-strategy.md).

Leave the weekly job alone rather than pausing it — its `skip if an
upstream-sync issue is already open` guard means it will stay quiet on its own
while **#113** remains open, so pausing it would be changing CI to achieve
something CI already does. That guard also gives a useful signal: keep **#113**
open for the program's duration as the umbrella tracking issue, and close it when
the lock advances.

One consequence to state plainly in the scope document: because the target is
frozen at `936aff00` ([decision 01](/scope/01-sync-target-and-freeze.md)),
upstream commits landing during the program are **not** in scope, and the moment
#113 closes the weekly job will open a fresh issue for them. That is the intended
outcome, not a failure.

# Verdict

**Lock advances once, at the end; weekly job left running.** Accepted as
recommended.

`UPSTREAM.lock` is a claim about what the port implements, so a partially-synced
tree claiming 0.84.1 would mislead the next sync into skipping real work. It
moves in milestone 6 together with the CHANGELOG entry and the v0.2.0 tag from
[decision 03](/scope/03-release-and-breaking-strategy.md).

The weekly workflow is left untouched. Its existing "skip if an `upstream-sync`
issue is already open" guard means it stays quiet on its own while **#113**
remains open, so pausing it would be a CI change to achieve what CI already
does. **#113 therefore stays open for the program's duration as the umbrella
tracking issue**, and closes when the lock advances.

Stated plainly in `SCOPE.md` as an intended consequence, not a failure: because
the target is frozen at `936aff00`, upstream commits landing during the program
are out of scope, and the moment #113 closes the weekly job will open a fresh
issue for them.

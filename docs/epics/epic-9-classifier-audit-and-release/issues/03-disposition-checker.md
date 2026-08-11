---
type: Issue
title: "Add upstream/disposition_check.sh and its offline test"
description: "A checker that reads docs/PORTING.md's mapping table and reports any in-range upstream file with no disposition, plus any Go target that does not exist — so the 232-file criterion is verified mechanically rather than by hand."
tags: [epic-9]
timestamp: 2026-08-11T18:15:00Z
epic: 9
issue: 03
slug: disposition-checker
size: M
status: open
gh_issue: 194
resource: https://github.com/kern-ia/kern-link/issues/194
depends_on: []
---

# Add upstream/disposition_check.sh and its offline test

**Assumption beyond `EPIC_9.md`.** Neither `EPIC_9.md`'s `## Scope` nor its
`## Acceptance criteria` asks for this tooling — the epic only requires
criterion 3 (all 232 files dispositioned) to be true, not mechanically
checked. Epic 0's plan-remediation triage decided to keep this issue anyway
(`docs/epics/epic-0-plan-remediation/EPIC_0.md`, `## Notes`): without a
mechanical arbiter, "dispositioned" is a judgement call a reviewer cannot
falsify, which would make the epic's second completion gate a judgement call
again. `EPIC_9.md`'s `## Scope` now carries the authorizing line.

## Summary

Epic 9's acceptance criterion 3 says every one of the **232** in-range upstream
files is dispositioned in `docs/PORTING.md` — ported, or a deviation with a
reason. Checked by hand that criterion is unfalsifiable: a reader cannot tell a
complete table from a table that stops at 200 files, and the next sync inherits
the doubt rather than a known state.

This issue builds the check so
[issue 04](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md)
has a criterion that either passes or names the file it failed on. It follows
the precedent already in the repo: `upstream/sync.sh` does the network work,
`upstream/sync_test.sh` tests its logic offline and runs on every push.

## Scope

- **`upstream/disposition_check.sh`** — same shape and header-comment style as
  `upstream/sync.sh` (`#!/usr/bin/env bash`, `set -euo pipefail`, a usage and
  env-override block at the top):
  - Enumerate the **in-range diff**, not the whole upstream tree: the files
    that changed between the lock's pinned commit and a given target ref, via
    `git -C "$workdir" diff --name-status "$pinned..$ref" -- packages/ai`,
    mirroring `sync.sh`'s own diff (`sync.sh:39`) and reusing the clone
    `sync.sh` creates rather than cloning a second one. Keep every `D` entry as
    an in-range path still needing a disposition row — a file upstream deleted
    orphans its Go counterpart, and a whole-tree enumeration at the new ref
    alone can never produce that path to check, because it is no longer there
    to enumerate.
  - Parse the **first column** of the mapping table in `docs/PORTING.md` into
    the set of upstream patterns, treating each cell's backticked paths as
    glob patterns (`src/api/*.lazy.ts`, `src/providers/*.models.ts`, and the
    multi-path cells that list several files in one row).
  - Report every enumerated file matching no pattern as `UNACCOUNTED <path>`,
    and exit non-zero when there is at least one.
  - Second check, same run: every **Go** path named in the table's second column
    exists in the tree (`git ls-files`), reported as `MISSING <path>`. A row
    claiming `ported` to a file that does not exist is as broken as a missing
    row, and this catches renames the sweep would otherwise leave stale. Cells
    reading `n/a`, `none`, or naming a package directory are matched as
    directories or skipped explicitly.
  - Print a one-line summary (`N files in range, M unaccounted, K missing`),
    where `N` is the in-range diff's own file count — the same measurement
    the epic's **232** claims, not a second definition of "in range" — so the
    232 number is checkable against `N` rather than asserted separately.
  - Env overrides mirroring `sync.sh`: `UPSTREAM_LOCK_FILE`,
    `UPSTREAM_CLONE_DIR`, plus `UPSTREAM_PORTING_FILE` (which table to read) and
    `UPSTREAM_FILE_LIST` (read the in-range file list from a file instead of
    git, which is what makes the test offline).
- **`upstream/disposition_check_test.sh`** — offline, modeled on
  `upstream/sync_test.sh`: fixture `PORTING.md` tables and fixture file lists in
  a temp dir, asserting the accounted/unaccounted/missing verdicts and the exit
  codes. No network, no clone.
- Wire `bash upstream/disposition_check_test.sh` into the `test` job of
  `.github/workflows/test.yml` beside the existing `bash upstream/sync_test.sh`
  step — the `test` job's step list grows by one `run:` step (currently
  `go test ./... -race -v` then `sync_test.sh`; this adds a third alongside
  them). The *checker* itself (`disposition_check.sh`) stays manual — it needs
  the upstream clone, and CI runs offline — only its offline test is gated
  like `sync.sh`'s.
- `docs/PORTING.md`'s sync-procedure section (`:119-129`) gains a step naming
  the checker, so the next sync runs it instead of rediscovering it.

## Out of scope

- Actually dispositioning the missing files — that is
  [issue 04](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md).
  This PR may well leave the checker failing loudly; that is the expected state
  until 04 lands.
- Making the *checker* (`disposition_check.sh`) a required CI gate. It cannot
  be: it needs network access to an upstream clone. The three gates
  `CONVENTIONS.md` names by name — `go test ./... -race -v`,
  `bash upstream/sync_test.sh`, `golangci-lint` v2.12.2 — stay exactly those
  three; none is renamed, replaced, or joined by a fourth *named* gate. This
  does add one more offline, self-contained `run:` step to the `test` job
  (`disposition_check_test.sh`, next to `sync_test.sh`, above) — a step, not a
  gate, and not this bullet's subject.
- Restructuring the mapping table into a machine-readable format (YAML, TSV).
  The table is the human-facing artifact; the parser adapts to it, not the
  reverse.
- Any Go code.

## Acceptance criteria / Definition of done

- [ ] `bash upstream/disposition_check_test.sh` passes with no network and no
      `upstream/.upstream-clone` present, and prints a per-case pass line the
      way `sync_test.sh` does.
- [ ] The test covers, at minimum: a fixture where every path matches (exit 0,
      no `UNACCOUNTED` lines); one unmatched path (non-zero exit, the path named
      exactly once); a glob row matching several files; a Go target that does
      not exist (`MISSING`, non-zero exit); and a table row the parser must skip
      (the header separator `|---|---|---|`).
- [ ] The checker reads its patterns from `docs/PORTING.md` — no second
      hard-coded file list anywhere in the script.
- [ ] `bash upstream/disposition_check.sh` against the current tree runs to
      completion and prints the summary line with a file count; whether it exits
      0 is issue 04's business, not this one's.
- [ ] `.github/workflows/test.yml` runs the offline test, and the run is green.
- [ ] `docs/PORTING.md`'s sync procedure names the checker as a step.
- [ ] Both scripts use `set -euo pipefail`, keep LF endings per
      `.gitattributes`, and match `sync.sh`'s executable bit and header-comment
      conventions.
- [ ] CI green (`go test ./... -race -v`, `bash upstream/sync_test.sh`,
      `golangci-lint` v2.12.2).
- [ ] Conventional Commit, e.g.
      `ci: add an upstream disposition checker and its offline test`.

## Relevant files / areas

- `upstream/sync.sh` — the model to follow: env-override header block, lock
  parsing (`grep '^commit=' | cut -d= -f2`), clone reuse at
  `$here/.upstream-clone`, `bucket()`-style output.
- `upstream/sync_test.sh` — the offline-test pattern to mirror.
- `upstream/UPSTREAM.lock` — `commit=` supplies the default ref.
- `docs/PORTING.md:19-64` — the mapping table the parser consumes, including
  the glob rows (`:38`, `:58`) and multi-path cells (`:23`, `:40`, `:47`);
  `:111-131` the sync procedure.
- `.github/workflows/test.yml` — the step list where `sync_test.sh` runs.

## Dependencies

- **Blocked by**: None. Can run in parallel with issues 01 and 02.
- **Blocks**: [Issue 04](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md)
  — the sweep's acceptance criterion is this checker exiting 0.

## PR size note

`M` — ~350 changed lines of shell: `upstream/disposition_check.sh` (in-range
diff enumeration, glob matching over `docs/PORTING.md:19-64`'s first column, the
`MISSING` second pass, the summary line) and the offline
`upstream/disposition_check_test.sh` with its five fixture cases, plus one
`run:` step in `.github/workflows/test.yml` and one sync-procedure step. Split
past ~500, and the seam is the second check — the `MISSING` pass can follow the
`UNACCOUNTED` one in its own PR.

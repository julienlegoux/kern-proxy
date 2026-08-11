---
type: Issue
title: "Audit the retry and overflow classifiers against upstream 936aff00 and resync their patterns"
description: "Produce the pattern-by-pattern parity table for ai/retry.go and ai/overflow.go, port every upstream pattern change in range, and cover each one in the two table-driven test files."
tags: [epic-9]
timestamp: 2026-08-11T18:15:00Z
epic: 9
issue: 01
slug: classifier-parity-audit
size: M
status: open
gh_issue: 192
resource: https://github.com/kern-ia/kern-link/issues/192
depends_on: []
---

# Audit the retry and overflow classifiers against upstream 936aff00 and resync their patterns

## Summary

Epic 9's acceptance criterion 1, and the mitigation for the program's named
risk: **silent classifier regressions**. `ai/retry.go` and `ai/overflow.go`
decide whether a failed turn is retried or treated as context overflow by
matching provider error *text*. Every adapter that produces that text has just
moved to 0.84.1 (epics 4, 5, 8), so a pattern upstream added, reworded, or
dropped in range is a behavior change on the Go side that no test will report —
the suite stays green while the library quietly stops retrying something it used
to.

This issue is sequenced after the adapters deliberately
([decision 07](../../../planning/scope/07-classifier-error-text-sync.md)):
auditing before they moved would audit against text about to change.

The deliverable is two-sided. A **table** pairing every regex on both sides,
with every difference justified — that is what makes the parity claim checkable
by a later reader instead of re-derived. And the **code**: every upstream
pattern delta ported, each with a test case.

## Scope

- Fetch the upstream tree if it is not already present — `bash upstream/sync.sh`
  clones to `upstream/.upstream-clone` — then diff the two classifier files
  across the whole range:
  `git -C upstream/.upstream-clone diff 244f1deaf1ae0fc1a242d9df5cddf457cf3d36a7..936aff00918de1187f085f123c2812d8f2d67745 -- packages/ai/src/utils/retry.ts packages/ai/src/utils/overflow.ts`.
- New bundle doc **`docs/classifier-parity.md`** (`type: Reference`, following
  the frontmatter shape of `docs/PORTING.md:1-7`), holding one table with a row
  per pattern: upstream pattern (with `file:line` at `936aff00`), its Go
  counterpart (`ai/retry.go:<line>` / `ai/overflow.go:<line>`), a status of
  `same` / `adapted` / `go-only` / `upstream-only`, and a justification for
  every status that is not `same`. It stays in the consumer-facing `docs/`
  bundle rather than moving to `docs/planning/` — a deliberate choice, not an
  oversight: `docs/PORTING.md` is an equally dev-facing artifact already
  registered at `docs/index.md:14`, so `docs/planning/mapping/01-planning-bundle-nesting.md:69-74`'s
  dev-process/consumer split is read here as applying to planning documents,
  not to porting-audit docs that travel with the porting map they cross-link.
- The table covers every pattern group, not just the two obvious lists:
  - `nonRetryableProviderLimitErrorPattern` (`ai/retry.go:16-33`)
  - `retryableStatusCodePatterns` (`ai/retry.go:41-48`)
  - `transientProviderErrorPatterns` (`ai/retry.go:53-97`)
  - `overflowPatterns` (`ai/overflow.go:9-34`)
  - `nonOverflowPatterns` (`ai/overflow.go:39-43`)
  - the three non-text detection modes in `IsContextOverflow`
    (`ai/overflow.go:55-87`) — the `contextWindow > 0` guards and the `0.99`
    length-stop threshold are behavior too, and upstream may have moved them.
  - This is a deliberate extension of `EPIC_9.md`'s "every regex in
    `ai/retry.go` and `ai/overflow.go`" (`:26-30`, `:55-57`) to three modes
    that carry no regex at all — defensible because they carry the same
    silent-regression risk the audit exists to mitigate, stated here
    explicitly rather than left implied.
- Port every upstream pattern added, changed, or removed in range into the Go
  vars, keeping the file conventions: grouped patterns with a comment above each
  group explaining what family it covers (`retry.go`), and a trailing
  `// Provider` attribution comment per regex (`overflow.go`).
- Extend `ai/retry_test.go` and `ai/overflow_test.go` with a case per new or
  changed pattern, using realistic provider error text rather than the pattern
  echoed back. Both files are already table-driven and are two of the 19
  legitimate table-driven tests `CONVENTIONS.md` names — add cases to the
  existing tables rather than writing new discrete functions.
- Register the new doc: a bullet in `docs/index.md` (after the PORTING.md
  bullet), an entry in `docs/log.md` under a `## 2026-…` heading, and a link
  from `docs/PORTING.md`'s `src/utils/retry.ts` and `src/utils/overflow.ts` rows
  (`:27-28`) so a reader of the porting map finds the audit.

## Out of scope

- `utils/provider-retry.ts` versus `ai/apis/internal/httpretry` —
  [issue 02](/epic-9-classifier-audit-and-release/issues/02-provider-retry-vs-httpretry.md)
  settles that.
- The full 232-file disposition sweep —
  [issue 04](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md).
  This PR touches only the two classifier rows in the mapping table.
- **Changing error text that adapters emit.** If the audit turns up an adapter
  emitting text that no pattern matches (or matching one it should not), that is
  an adapter defect: record the mismatch as a row in the table and file a
  follow-up issue. Do not edit adapter error strings here — `staticcheck`'s
  ST1005 is disabled precisely because that text is load-bearing, and epics 4,
  5 and 8 own those files.
- `CHANGELOG.md` — [issue 05](/epic-9-classifier-audit-and-release/issues/05-upstream-lock-and-changelog.md).

## Acceptance criteria / Definition of done

- [ ] `docs/classifier-parity.md` exists with OKF frontmatter and a table whose
      rows cover **every** regex in the six groups listed under Scope, each
      appearing exactly once. The row count is a property, not a fixed number
      — it equals the sum of the six groups' entry counts in `ai/retry.go` and
      `ai/overflow.go`, re-counted at this PR's base commit — **72** as the
      groups stand today (`8 + 6 + 28 + 24 + 3 + 3`, counted directly from
      `nonRetryableProviderLimitErrorPattern`, `retryableStatusCodePatterns`,
      `transientProviderErrorPatterns`, `overflowPatterns`,
      `nonOverflowPatterns`, and `IsContextOverflow`'s three detection modes —
      not the unreachable 75 (`2 + 6 + 6 + 34 + 24 + 3`) this criterion
      previously asserted, which matched none of the six groups' real counts).
- [ ] Every upstream pattern present in `packages/ai/src/utils/retry.ts` and
      `packages/ai/src/utils/overflow.ts` at `936aff00` appears in the table
      exactly once, with a status of `same`, `adapted`, or
      `upstream-only`; every `upstream-only` row states why the Go port does not
      need it.
- [ ] Every Go-side pattern with no upstream counterpart is marked `go-only`
      with the reason it exists (the existing Cerebras, DS4, and z.ai entries in
      `ai/overflow.go` are examples of patterns that may not be upstream's).
- [ ] Each pattern added or changed in this PR has a case in `ai/retry_test.go`
      or `ai/overflow_test.go` asserting the classification against realistic
      error text, and
      `GOTMPDIR=$PWD/.gotmp go test ./ai/ -run 'Retry|Overflow' -v` passes with
      those cases visible by name.
- [ ] `ai/retry.go`'s warning comment (`:35-40`) about status-code patterns
      being text patterns survives verbatim in substance, and no status-code
      pattern has been added to `transientProviderErrorPatterns` — the two lists
      stay separate, because `IsTransientProviderErrorText` is applied to raw
      HTTP bodies (`:118-132`).
- [ ] `IsNonRetryableProviderLimitError` still wins over the transient
      classification in `IsRetryableAssistantError` (`ai/retry.go:145-148`),
      with a test asserting a quota message that also matches `rate.?limit` is
      classified terminal. The equivalent guarantee for `httpretry.IsRetryable`
      is out of scope here (`:76-78`) — it is
      [issue 02](/epic-9-classifier-audit-and-release/issues/02-provider-retry-vs-httpretry.md)'s
      acceptance criterion (`:110-112`), asserted there and not duplicated.
- [ ] `docs/index.md` lists the new doc; `docs/log.md` has an entry for it;
      `docs/PORTING.md`'s two classifier rows link to it.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] Conventional Commit, e.g.
      `fix(ai): resync the retry and overflow classifiers with upstream 0.84.1`.

## Relevant files / areas

- `ai/retry.go` — `buildProviderErrorPattern` (`:10-12`),
  `nonRetryableProviderLimitErrorPattern` (`:16-33`),
  `retryableStatusCodePatterns` (`:41-48`) and the warning above it (`:35-40`),
  `transientProviderErrorPatterns` (`:53-97`), and the three exported
  classifiers (`:109-149`).
- `ai/overflow.go` — `overflowPatterns` (`:9-34`), `nonOverflowPatterns`
  (`:39-43`), `IsContextOverflow`'s three modes (`:55-87`), `OverflowPatterns`
  (`:89-94`).
- `ai/retry_test.go`, `ai/overflow_test.go` — the two table-driven suites, each
  carrying a `// Ports: packages/ai/test/…` header.
- `ai/apis/internal/httpretry/httpretry.go:IsRetryable` — the second consumer of
  these classifiers; the audit must not break its documented coupling.
- `docs/PORTING.md:27-28`, `docs/index.md:11-14`, `docs/log.md`.
- Upstream at `936aff00`: `packages/ai/src/utils/retry.ts`,
  `packages/ai/src/utils/overflow.ts`, `packages/ai/test/retry.test.ts`,
  `packages/ai/test/overflow.test.ts`.

## Dependencies

- **Blocked by**: None within this epic. Depends on the adapter epics
  ([4](/epic-4-openai-family-adapters/EPIC_4.md),
  [5](/epic-5-remaining-adapters/EPIC_5.md),
  [8](/epic-8-pi-messages-and-radius/EPIC_8.md)) having merged — the point of
  the sequencing.
- **Blocks**: [Issue 02](/epic-9-classifier-audit-and-release/issues/02-provider-retry-vs-httpretry.md)
  (the retry comparison builds on this audit),
  [issue 04](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md).

## PR size note

`M` — ~400 changed lines: a 72-row `docs/classifier-parity.md` table
(`8 + 6 + 28 + 24 + 3 + 3` across the six groups), the upstream pattern deltas
ported into `ai/retry.go` and `ai/overflow.go`, a case each in the two existing
table-driven suites, and three registrations. Re-checked against REPORT_9's
flag and kept `M` because the table is one line per row and the ports edit
existing vars rather than adding files. Split past ~500, and the seam is the
doc: the parity table can land before the pattern ports it justifies.

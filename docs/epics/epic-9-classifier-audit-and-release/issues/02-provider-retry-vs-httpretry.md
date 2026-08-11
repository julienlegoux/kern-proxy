---
type: Issue
title: "Settle whether upstream's provider-retry.ts supersedes ai/apis/internal/httpretry"
description: "Compare upstream's new request-level retry helper against the Go port's own httpretry package, port what is portable, and record the rest as a drift record rather than a silent difference."
tags: [epic-9]
timestamp: 2026-08-11T20:00:00Z
epic: 9
issue: 02
slug: provider-retry-vs-httpretry
size: M
status: open
gh_issue: 193
resource: https://github.com/kern-ia/kern-link/issues/193
depends_on: ["01"]
---

# Settle whether upstream's provider-retry.ts supersedes ai/apis/internal/httpretry

## Summary

Epic 9's acceptance criterion 2. `ai/apis/internal/httpretry` is one of only two
non-test files in the repository with **no** `// Ports:` header — under
`CONVENTIONS.md` that absence is meaningful, marking it as original Go code with
no upstream counterpart. It was generalized from the Codex adapter's
`doRequestWithRetry` and is now the request-level retry loop every raw-`net/http`
adapter uses to honor `MaxRetries`.

Upstream 0.84.1 adds `src/utils/provider-retry.ts`. That may make httpretry a
*port* rather than original code, may leave the two coexisting with different
semantics, or may reveal upstream doing something the Go loop gets wrong. The
epic's requirement is that the answer is **written down** — as code if the
behaviors should converge, as a drift record if they diverge deliberately.
Either way, no silent difference survives this issue.

`ai/apis/internal/httpretry` itself is **not relocated** for this program —
[epic-0 issue 06](/epic-0-plan-remediation/issues/06-rein-in-epic-3-invented-scope.md)
hands this reconciliation here explicitly, having scoped Epic 3 issue 06 down
to the OpenRouter images adapter and confirmed the package stays where
`CONVENTIONS.md:22-26` puts it. This issue is the accepting side of that
hand-off: it settles `provider-retry.ts` against `httpretry.go` in place,
without moving the package.

## Scope

- Read `packages/ai/src/utils/provider-retry.ts` at `936aff00` in
  `upstream/.upstream-clone`, plus every call site
  (`git -C upstream/.upstream-clone grep -n 'provider-retry' 936aff00 -- packages/ai`),
  and establish what it actually is: a shared helper adapters call, or a
  provider-level policy layered above the request loop.
- Compare it against `httpretry.go` on the axes where a difference changes
  behavior:
  1. **Which statuses retry** — `IsRetryable`'s switch (`429/500/502/503/504`)
     versus upstream's list, and whether `524` (present in
     `retryableStatusCodePatterns`) belongs here too.
  2. **The text fallback** — `ai.IsTransientProviderErrorText` for non-listed
     statuses, and `ai.IsNonRetryableProviderLimitError` short-circuiting
     everything.
  3. **`Retry-After` parsing** — `retry-after-ms` winning over `retry-after`,
     which is parsed as seconds *or* an HTTP date (`RetryAfterDelay`), and the
     429-only cap through `EffectiveMaxRetryDelay` (`capRetryDelay`).
  4. **Backoff** — `ExponentialDelay` (`BaseDelay * 2**attempt`) versus
     upstream's, including any jitter upstream added.
  5. **The stream boundary** — httpretry deliberately retries only *before*
     streaming begins (documented at `httpretry.go:1-13`). If upstream now
     retries across a partially consumed stream, that is a genuine divergence,
     not a missing feature.
- Port the differences that are portable and are improvements, each with a named
  test in `ai/apis/internal/httpretry/httpretry_test.go`.
- Set the provenance header correctly for whatever the file becomes: if it now
  ports upstream, add
  `// Ports: packages/ai/src/utils/provider-retry.ts` after the package clause
  per `CONVENTIONS.md`; if it stays original, leave it headerless and say so
  explicitly in the porting map, so the next reader does not have to re-derive
  this.
- Add the single `docs/PORTING.md` row for `src/utils/provider-retry.ts` —
  `ported` with its Go target, or `not ported` with the reason.
- Where the two diverge deliberately, write the drift record at
  `docs/epics/epic-9-classifier-audit-and-release/drift/02-provider-retry-vs-httpretry.md`
  with the fields the register expects: **Decided** (the standard, linked),
  **Actual**, **Because** (the verified blocker or the reason convergence is
  wrong), **Disposition**, **Revisit when**, **Evidence**. `close-epic` promotes
  it into `docs/planning/DRIFT.md`; the epic's criterion 2 is satisfied through
  that path.

## Out of scope

- Writing `docs/planning/DRIFT.md` directly. The register is single-writer and
  written at epic close from the drift records issues leave behind.
- The classifier patterns themselves —
  [issue 01](/epic-9-classifier-audit-and-release/issues/01-classifier-parity-audit.md).
- Changing which adapters use httpretry, or moving Bedrock off the SDK retryer
  onto it. Adapter wiring belongs to epics 4 and 5.
- Retrying mid-stream. If the comparison concludes the Go boundary should move,
  file it — that is a design change with a blast radius across every adapter,
  not a line in this PR.
- The rest of the disposition sweep —
  [issue 04](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md).

## Acceptance criteria / Definition of done

- [ ] The comparison is settled in writing on all five axes above, as **either**
      a code change to `ai/apis/internal/httpretry` **or** a drift record at
      `docs/epics/epic-9-classifier-audit-and-release/drift/02-provider-retry-vs-httpretry.md`
      — in practice usually both, one per axis.
- [ ] Any drift record present carries all six fields (Decided / Actual /
      Because / Disposition / Revisit when / Evidence), with **Because** naming
      concrete evidence — an upstream line reference or a failing behavior — not
      a preference.
- [ ] `docs/PORTING.md` contains exactly one row for
      `src/utils/provider-retry.ts`, with a status and a reason.
- [ ] `ai/apis/internal/httpretry/httpretry.go` either carries a
      `// Ports: packages/ai/src/utils/provider-retry.ts` header, or carries
      none and `docs/PORTING.md` records that it is original Go code — the two
      states are mutually exclusive and one of them is true.
- [ ] Every behavior adopted from upstream has a named test in
      `ai/apis/internal/httpretry/httpretry_test.go` that fails without the
      change (discrete named functions, the package's existing style).
- [ ] `httpretry.IsRetryable` still consults
      `ai.IsNonRetryableProviderLimitError` first, with a test proving a quota
      message returns `false` even at HTTP 429.
- [ ] `GOTMPDIR=$PWD/.gotmp go test ./...` passes locally; CI green
      (`go test ./... -race -v`, `bash upstream/sync_test.sh`, `golangci-lint`
      v2.12.2).
- [ ] Conventional Commit, e.g.
      `refactor(apis): align httpretry with upstream provider-retry`.

## Relevant files / areas

- `ai/apis/internal/httpretry/httpretry.go` — the package doc comment
  (`:1-14`), `Do` (`:88-…`), `IsRetryable`, `RetryAfterDelay`,
  `ExponentialDelay`, `capRetryDelay`, `ErrAborted`, `DefaultMaxRetries` and
  Codex's deliberate `0`.
- `ai/apis/internal/httpretry/httpretry_test.go`.
- `ai/retry.go:109-132` — the two exported text classifiers httpretry depends
  on, and the comments documenting the coupling.
- `docs/PORTING.md` — the mapping table; note `src/utils/retry.ts` already maps
  to `ai/retry.go` (`:27`), so `provider-retry.ts` needs its own row.
- `docs/classifier-parity.md` from
  [issue 01](/epic-9-classifier-audit-and-release/issues/01-classifier-parity-audit.md)
  — cross-reference rather than duplicate its rows.
- Upstream at `936aff00`: `packages/ai/src/utils/provider-retry.ts`,
  `packages/ai/src/utils/error-body.ts`, `packages/ai/src/api/openai-codex-responses.ts`.

## Dependencies

- **Blocked by**: [Issue 01](/epic-9-classifier-audit-and-release/issues/01-classifier-parity-audit.md)
  — the retry classifiers are this loop's text fallback, and comparing retry
  policy against upstream is unsound while the patterns are still moving.
- **Blocks**: [Issue 04](/epic-9-classifier-audit-and-release/issues/04-disposition-sweep.md)
  — the sweep expects `provider-retry.ts` already dispositioned.

## PR size note

`M` — ~300 changed lines: the five-axis comparison lands as edits inside
`httpretry.go` (retryable statuses, `Retry-After` precedence, backoff, the
stream boundary), a named test per adopted behavior in `httpretry_test.go`, the
one `docs/PORTING.md` row for `src/utils/provider-retry.ts`, and a six-field
drift record for whatever stays divergent. Split past ~500 — but if the
comparison concludes the stream boundary should move, that is a follow-up with a
blast radius across every adapter, not more lines here.

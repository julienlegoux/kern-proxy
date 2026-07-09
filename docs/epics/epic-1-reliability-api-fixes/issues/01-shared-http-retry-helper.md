---
type: Issue
title: "Extract a shared HTTP retry helper from the Codex adapter"
description: "Generalize Codex's doRequestWithRetry into a shared helper package, unify its retry-classification regexes with ai/retry.go, migrate codex onto it, and document the default MaxRetries for other adapters."
tags: [epic-1]
timestamp: 2026-07-09T09:55:00Z
resource: https://github.com/julienlegoux/kern-proxy/issues/87
epic: 1
issue: 1
slug: shared-http-retry-helper
size: M
status: done
gh_issue: 87
gh_pr: 98
depends_on: []
---

# Extract a shared HTTP retry helper from the Codex adapter

## Summary

`StreamOptions.MaxRetries`/`MaxRetryDelay` (`ai/options.go:104`) are documented
but only the Codex adapter reads them (`doRequestWithRetry`,
`ai/apis/codex/codex.go:276-356`); every other adapter silently makes exactly
one attempt. This issue builds the shared machinery the other adapters will
adopt: generalize Codex's HTTP-request-level retry loop into a reusable helper
and migrate codex itself onto it.

## Scope

- New shared helper — `ai/apis/internal/httpretry` package (preferred; `ai/internal` already exists as a precedent for internal packages), or the `ai/apis` root alongside `simpleopts.go`/`transform.go` if import cycles force it. It retries the POST until 2xx *before* streaming begins.
- Carry over Codex's behaviors verbatim:
  - exponential backoff (`exponentialDelay`, base 1s, kept as a `var` so tests can shrink it)
  - `Retry-After` / `retry-after-ms` honoring, clamped via `opts.EffectiveMaxRetryDelay()` (`ai/options.go:305`)
  - non-retryable quota/billing short-circuit
  - context cancellation aborts the backoff sleep
- Unify the codex-local retry-classification regexes with the `ai/retry.go` patterns (`IsRetryableAssistantError`) where feasible — one copy, not two.
- Migrate the codex adapter to the helper. Codex keeps its deliberate per-adapter default of 0 (`ai/apis/codex/codex.go:49`); existing codex retry tests (`ai/apis/codex/stream_test.go`) must keep passing.
- Pick and document the default retry count for the other adapters in the helper and the `ai/options.go` doc comment — upstream-equivalent is 2.

## Out of scope

- Adopting the helper in any adapter other than codex — that's issues [02](./02-retry-anthropic-openaicompletions.md), [03](./03-retry-openairesponses-azure.md), [04](./04-retry-google-vertex-mistral.md), and [05](./05-bedrock-sdk-retries.md).
- Retrying after events have flowed to the consumer — a wrapper around the dispatched `*ai.Stream` (`ai/provider.go:449-462`) cannot transparently retry mid-stream; request-level-only is the deliberate design.
- The images API's own `MaxRetries` (`ai/images/types.go:98`) — not part of the plan.

## Acceptance criteria / Definition of done

- Helper unit tests (`httptest`): 429/500-then-success succeeds within `MaxRetries`; exhausted retries surface the failure to the caller.
- `Retry-After` and `retry-after-ms` are honored; `MaxRetryDelay` clamps them.
- Quota/billing errors are not retried.
- Cancelling the context aborts a pending backoff sleep promptly.
- Codex behavior is unchanged: default 0 retries, existing `ai/apis/codex/stream_test.go` retry tests pass, `go test -race ./...` is clean.

## Relevant files / areas

- `ai/apis/codex/codex.go` (`doRequestWithRetry`, `exponentialDelay`, retry regexes) — source of the generalization
- `ai/retry.go`, `ai/retry_test.go` — existing classifiers to unify with
- `ai/options.go` — `EffectiveMaxRetryDelay`, `MaxRetries` doc comment (document the default of 2)
- New: `ai/apis/internal/httpretry/` (or `ai/apis/httpretry.go`)

## Dependencies

None — this is the foundation for issues 02–05 in [Epic 1](/epic-1-reliability-api-fixes/EPIC_1.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.

---
type: Issue
title: "Adopt the retry helper in the anthropic and openaicompletions adapters"
description: "Switch the anthropic and openaicompletions request paths to the shared HTTP retry helper so MaxRetries/MaxRetryDelay are honored, with per-adapter httptest coverage."
tags: [epic-1]
timestamp: 2026-07-09T10:50:00Z
resource: https://github.com/julienlegoux/kern-proxy/issues/88
epic: 1
issue: 2
slug: retry-anthropic-openaicompletions
size: M
status: done
gh_issue: 88
gh_pr: 99
depends_on: [1]
---

# Adopt the retry helper in the anthropic and openaicompletions adapters

## Summary

With the shared retry helper in place (issue 01), the anthropic and
openaicompletions adapters currently still make exactly one attempt regardless
of `StreamOptions.MaxRetries`. Switch both request paths to the helper with the
documented default of 2 retries.

## Scope

- `ai/apis/anthropic/anthropic.go` (~line 259–272): replace the direct `client.Do(req)` request path with the shared helper.
- `ai/apis/openaicompletions/openaicompletions.go` (~line 123–136): same.
- Both adapters use the shared default (2) when `opts.MaxRetries` is nil.
- Per-adapter `httptest` tests mirroring the codex retry tests (`ai/apis/codex/stream_test.go` is the pattern to follow).

## Out of scope

- The other adapters — issues [03](./03-retry-openairesponses-azure.md), [04](./04-retry-google-vertex-mistral.md), [05](./05-bedrock-sdk-retries.md).
- Any change to the helper itself; if the helper needs a fix, do it in a follow-up to issue [01](./01-shared-http-retry-helper.md).
- Restructuring `StreamOptions` — deliberate port-fidelity non-goal.

## Acceptance criteria / Definition of done

For each of the two adapters, `httptest`-backed tests prove:

- 429/500-then-success succeeds within `MaxRetries`; exhausted retries surface the in-band error event.
- `Retry-After` honored; `MaxRetryDelay` clamps it.
- Quota/billing errors are not retried.
- Cancelling the context aborts a pending backoff sleep promptly.
- `go test -race ./...` clean.

## Relevant files / areas

- `ai/apis/anthropic/anthropic.go`, `ai/apis/anthropic/anthropic_test.go`
- `ai/apis/openaicompletions/openaicompletions.go`, `ai/apis/openaicompletions/openaicompletions_test.go`
- `ai/apis/internal/httpretry/` (consume only)

## Dependencies

Blocked by [Issue 01](./01-shared-http-retry-helper.md). Part of [Epic 1](/epic-1-reliability-api-fixes/EPIC_1.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.

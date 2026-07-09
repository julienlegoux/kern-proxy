---
type: Issue
title: "Adopt the retry helper in the google, vertex, and mistral adapters"
description: "Switch the google, google/vertex, and mistral request paths to the shared HTTP retry helper so MaxRetries/MaxRetryDelay are honored, with per-adapter httptest coverage."
tags: [epic-1]
timestamp: 2026-07-09T11:50:00Z
resource: https://github.com/julienlegoux/kern-proxy/issues/90
epic: 1
issue: 4
slug: retry-google-vertex-mistral
size: M
status: done
gh_issue: 90
gh_pr: 101
depends_on: [1]
---

# Adopt the retry helper in the google, vertex, and mistral adapters

## Summary

Final raw-HTTP adoption batch: google, google/vertex, and mistral currently
ignore `StreamOptions.MaxRetries`. Switch all three request paths to the shared
helper with the documented default of 2.

## Scope

- `ai/apis/google/google.go` (~line 122–135): replace the direct `client.Do(req)` request path with the shared helper.
- `ai/apis/google/vertex/vertex.go` (~line 174–187): same.
- `ai/apis/mistral/mistral.go` (~line 148–161): same.
- All three use the shared default (2) when `opts.MaxRetries` is nil.
- Per-adapter `httptest` tests (codex's `ai/apis/codex/stream_test.go` is the pattern).

## Out of scope

- The other adapters — issues [02](./02-retry-anthropic-openaicompletions.md), [03](./03-retry-openairesponses-azure.md), [05](./05-bedrock-sdk-retries.md).
- Any other google-adapter changes (e.g. URL construction) — this issue only touches the request/retry path.

## Acceptance criteria / Definition of done

For each of the three adapters, `httptest`-backed tests prove:

- 429/500-then-success succeeds within `MaxRetries`; exhausted retries surface the in-band error event.
- `Retry-After` honored; `MaxRetryDelay` clamps it.
- Quota/billing errors are not retried.
- Cancelling the context aborts a pending backoff sleep promptly.
- `go test -race ./...` clean.

If three adapters push the diff toward ~1000 lines, split mistral out into its
own PR rather than trimming tests.

## Relevant files / areas

- `ai/apis/google/google.go` (+ tests)
- `ai/apis/google/vertex/vertex.go` (+ tests)
- `ai/apis/mistral/mistral.go` (+ tests)
- `ai/apis/internal/httpretry/` (consume only)

## Dependencies

Blocked by [Issue 01](./01-shared-http-retry-helper.md). Part of [Epic 1](/epic-1-reliability-api-fixes/EPIC_1.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.

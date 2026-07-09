---
type: Issue
title: "Adopt the retry helper in the openairesponses and azure adapters"
description: "Switch the openairesponses and azure request paths to the shared HTTP retry helper so MaxRetries/MaxRetryDelay are honored, with per-adapter httptest coverage."
tags: [epic-1]
timestamp: 2026-07-09T11:20:00Z
resource: https://github.com/julienlegoux/kern-proxy/issues/89
epic: 1
issue: 3
slug: retry-openairesponses-azure
size: M
status: done
gh_issue: 89
gh_pr: 100
depends_on: [1]
---

# Adopt the retry helper in the openairesponses and azure adapters

## Summary

Same adoption as issue 02, for the responses family's raw-HTTP adapters:
openairesponses and azure currently ignore `StreamOptions.MaxRetries`. Switch
both request paths to the shared helper with the documented default of 2.

## Scope

- `ai/apis/openairesponses/openairesponses.go` (~line 126–139): replace the direct `client.Do(req)` request path with the shared helper.
- `ai/apis/azure/azure.go` (~line 142–155): same.
- Both use the shared default (2) when `opts.MaxRetries` is nil.
- Per-adapter `httptest` tests (codex's `ai/apis/codex/stream_test.go` is the pattern; openairesponses already has `stream_test.go` scaffolding to extend).

## Out of scope

- codex — it shares the responses wire format but migrated in issue [01](./01-shared-http-retry-helper.md) and keeps its deliberate default of 0.
- The other adapters — issues [02](./02-retry-anthropic-openaicompletions.md), [04](./04-retry-google-vertex-mistral.md), [05](./05-bedrock-sdk-retries.md).

## Acceptance criteria / Definition of done

For each of the two adapters, `httptest`-backed tests prove:

- 429/500-then-success succeeds within `MaxRetries`; exhausted retries surface the in-band error event.
- `Retry-After` honored; `MaxRetryDelay` clamps it.
- Quota/billing errors are not retried.
- Cancelling the context aborts a pending backoff sleep promptly.
- `go test -race ./...` clean.

## Relevant files / areas

- `ai/apis/openairesponses/openairesponses.go`, `ai/apis/openairesponses/stream_test.go`
- `ai/apis/azure/azure.go` (+ its test file)
- `ai/apis/internal/httpretry/` (consume only)

## Dependencies

Blocked by [Issue 01](./01-shared-http-retry-helper.md). Part of [Epic 1](/epic-1-reliability-api-fixes/EPIC_1.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.

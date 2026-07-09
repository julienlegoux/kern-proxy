---
type: Issue
title: "Map MaxRetries/MaxRetryDelay onto the bedrock AWS SDK retryer"
description: "Bedrock speaks the AWS SDK, not raw HTTP — configure the SDK retryer from StreamOptions instead of adopting the shared helper."
tags: [epic-1]
timestamp: 2026-07-09T12:05:00Z
resource: https://github.com/julienlegoux/kern-proxy/issues/91
epic: 1
issue: 5
slug: bedrock-sdk-retries
size: S
status: pr-open
gh_issue: 91
gh_pr: 102
depends_on: [1]
---

# Map MaxRetries/MaxRetryDelay onto the bedrock AWS SDK retryer

## Summary

The bedrock adapter uses the AWS SDK rather than raw `net/http`, so the shared
retry helper doesn't apply. Instead, map `StreamOptions.MaxRetries` (and
`MaxRetryDelay` where the SDK supports a backoff cap) onto the SDK's retryer
configuration so the documented options are honored here too.

## Scope

- `ai/apis/bedrock/bedrock.go`: configure the AWS SDK retryer (e.g. `retry.NewStandard` / `config.WithRetryMaxAttempts`) from `opts.MaxRetries`, using the shared default (2) when nil, and cap backoff with `opts.EffectiveMaxRetryDelay()` where the SDK allows.
- Test coverage proving the retryer config reflects the options (SDK middleware/config inspection or a stubbed HTTP client, matching however `ai/apis/bedrock/stream_test.go` currently fakes the service).

## Out of scope

- Adopting the shared `httpretry` helper here — the SDK owns the transport.
- Reimplementing the helper's quota/billing classification inside the SDK retryer beyond what the SDK's own retryable-error classes already cover; note any semantic difference in a doc comment instead.

## Acceptance criteria / Definition of done

- `opts.MaxRetries` controls the SDK's max attempts; nil means the shared default (2).
- `opts.MaxRetryDelay` caps the SDK backoff where supported; the deviation (if the SDK can't honor it exactly) is documented on the adapter.
- Tests demonstrate a retried-then-successful call within `MaxRetries` and that exhausted retries surface the in-band error event.
- `go test -race ./...` clean.

## Relevant files / areas

- `ai/apis/bedrock/bedrock.go`, `ai/apis/bedrock/stream_test.go`
- `ai/options.go` (`EffectiveMaxRetryDelay`, shared default — read-only)

## Dependencies

Blocked by [Issue 01](./01-shared-http-retry-helper.md) (it fixes the shared default and its documentation). Part of [Epic 1](/epic-1-reliability-api-fixes/EPIC_1.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.

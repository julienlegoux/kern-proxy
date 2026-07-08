---
type: Issue
title: "Wire retry/overflow classification and the live smoke into the Anthropic adapter"
description: "Retryable stream-truncation handling (ended before message_stop), overflow classification, and the env-gated live smoke test."
tags: [epic-5]
timestamp: 2026-07-08T05:00:00Z
epic: 5
issue: 04
slug: anthropic-retry-overflow-smoke
size: S
status: in-progress
gh_issue: 23
resource: https://github.com/julienlegoux/kern-proxy/issues/23
depends_on: [01]
---

# Wire retry/overflow classification and the live smoke into the Anthropic adapter

## Summary

Complete [Epic 5](/epic-5-anthropic-adapter/EPIC_5.md) by integrating Epic 1's retry/overflow classifiers with the adapter and adding the env-gated live smoke. Plan risk #5 lives here: SSE edge cases feed the retry classifier and must be bit-exact against the upstream fixtures.

## Scope

- Stream "ended before `message_stop`" classified as **retryable** (early-EOF detection from `ai/internal/sse`).
- Overflow (context-length) classification wired to the adapter's error mapping (`ai/overflow.go`, `ai/retry.go` from Epic 1 — reuse, don't duplicate).
- Env-gated live smoke test mirroring upstream `skipIf`: runs only with `ANTHROPIC_API_KEY` set.
- TDD gate: port the remaining upstream `anthropic-*` error/retry tests, fixtures verbatim.

## Out of scope

- Changes to the classifiers themselves — they shipped in Epic 1; this issue only wires and fixture-locks them.

## Acceptance criteria / Definition of done

- Error/retry `anthropic-*` test ports pass bit-exact against verbatim SSE fixtures (plan risk #5).
- With `ANTHROPIC_API_KEY` set, the live smoke streams a real response; without it, the test skips.
- All ~12 upstream `anthropic-*` suites now pass, closing the epic's acceptance criteria.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- `ai/apis/anthropic/` (from Issue 01), `ai/retry.go`, `ai/overflow.go`, `ai/internal/sse` (read-only reuse).

## Dependencies

Blocked by [Issue 01](./01-anthropic-adapter-core.md).

## PR size note

Target ~500 changed lines; this should land well under — it's wiring plus test ports.

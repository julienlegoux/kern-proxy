---
type: Issue
title: "Add the anthropic-messages adapter core"
description: "Request building and SSE stream decoding for the Anthropic adapter, including the message_start/message_delta usage-seeding contract."
tags: [epic-5]
timestamp: 2026-07-08T03:00:00Z
epic: 5
issue: 01
slug: anthropic-adapter-core
size: L
status: done
gh_issue: 20
gh_pr: 50
resource: https://github.com/julienlegoux/kern-proxy/issues/20
depends_on: []
blocked_by: ["#18", "#19"]
---

# Add the anthropic-messages adapter core

## Summary

Port the core of the anthropic-messages adapter — the first real provider adapter ([Epic 5](/epic-5-anthropic-adapter/EPIC_5.md)): outgoing request construction and SSE stream decoding into the unified event stream, over raw `net/http` and the hand-rolled SSE parser from Epic 1. Follow-up issues layer thinking/cache_control, OAuth impersonation, and retry integration on top of this core.

## Scope

- New `ai/apis/anthropic` package: request building (messages, system, tools, sampling params) and streaming response decode to unified events.
- Fidelity-critical stream mechanics: usage seeded at `message_start` and only non-null-overwritten at `message_delta`; stop-reason mapping; tool-call block assembly.
- API-key auth path via the resolved `ModelAuth` (Epic 3 core, already shipped).
- TDD gate: port the base upstream `anthropic-*` tests first, copying SSE fixtures **verbatim**; golden-request snapshots captured via `OnPayload` must match TS-captured goldens.
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- Adaptive thinking and `cache_control` — [Issue 02](./02-anthropic-thinking-cache-control.md).
- OAuth Claude Code impersonation — [Issue 03](./03-anthropic-oauth-impersonation.md).
- Retry/overflow classifier integration and the live smoke — [Issue 04](./04-anthropic-retry-overflow-smoke.md).
- The OAuth login flow — [Epic 12](/epic-12-oauth-flows/EPIC_12.md); provider bindings — [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Acceptance criteria / Definition of done

- Base `anthropic-*` test ports pass against httptest fixtures (no network).
- Golden-request snapshots match the TS goldens byte-for-byte where upstream asserts them.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `ai/apis/anthropic/` — no adapter code exists yet; the path follows the plan's layout.
- Read-only context: `ai/internal/sse` (Epic 1 parser), `ai/events.go`, `ai/stream.go`, `ai/provider.go`, `ai/providers/faux` (the event-contract reference), `ai/cost.go`.

## Dependencies

Blocks [Issue 02](./02-anthropic-thinking-cache-control.md), [Issue 03](./03-anthropic-oauth-impersonation.md), [Issue 04](./04-anthropic-retry-overflow-smoke.md). Epic-level: needs epics 1–4.

## PR size note

Sized L: the request builder, stream decoder, and their verbatim fixture ports form one coherent, separately-unreviewable unit — splitting encode from decode would leave a PR with no runnable adapter. Keep it as close to ~500 lines of non-fixture code as possible; fixtures/goldens don't count against the budget, but if hand-written code approaches ~1000, split.

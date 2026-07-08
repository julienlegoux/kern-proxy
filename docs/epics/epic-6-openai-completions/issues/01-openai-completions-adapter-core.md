---
type: Issue
title: "Add the openai-completions adapter core"
description: "Chat-completions request/stream core with dual-map tool-call correlation, partial tool-arg JSON re-parsing, and the cached/cache-write usage math."
tags: [epic-6]
timestamp: 2026-07-08T06:15:00Z
epic: 6
issue: 01
slug: openai-completions-adapter-core
size: L
status: in-progress
gh_issue: 24
resource: https://github.com/julienlegoux/kern-proxy/issues/24
depends_on: []
blocked_by: ["#18", "#19"]
---

# Add the openai-completions adapter core

## Summary

Port the core of the openai-completions adapter ([Epic 6](/epic-6-openai-completions/EPIC_6.md)) — the compatibility workhorse that ultimately serves ~15 vendors: request construction and SSE stream decoding over raw `net/http`, with the fidelity-critical delta-correlation and usage mechanics. Follow-up issues add the compat matrix, thinking formats, and caching/affinity.

## Scope

- New `ai/apis/openaicompletions` package: chat-completions request building and streaming decode to unified events.
- Tool-call delta correlation by **both index and id** (dual maps) — deltas may arrive keyed either way.
- Partial tool-arg JSON re-parsed on every delta via Epic 1's strict → repair → partial → `{}` cascade (`ai/internal/partialjson`); scratch buffers stripped before persistence.
- Usage math: `prompt_tokens − cached − cache_write`.
- TDD gate: port the base upstream `openai-completions-*` tests first; goldens via `OnPayload` match TS-captured goldens.
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- Compat auto-detection matrix — [Issue 02](./02-openai-compat-matrix.md).
- Thinking-format encodings — [Issue 03](./03-openai-thinking-formats.md).
- cache_control replication, session affinity, live smoke — [Issue 04](./04-openai-cache-affinity-smoke.md).
- The Responses API family — [Epic 7](/epic-7-responses-family/EPIC_7.md).

## Acceptance criteria / Definition of done

- Base `openai-completions-*` test ports pass against httptest fixtures; tool-call correlation covered for both index-keyed and id-keyed delta streams.
- Persisted messages carry no partial-JSON scratch buffers.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `ai/apis/openaicompletions/` — path follows the plan's layout; no adapter code exists yet.
- Read-only context: `ai/internal/sse`, `ai/internal/partialjson`, `ai/events.go`, `ai/stream.go`, `ai/providers/faux` (event-contract reference).

## Dependencies

Blocks [Issue 02](./02-openai-compat-matrix.md), [Issue 03](./03-openai-thinking-formats.md), [Issue 04](./04-openai-cache-affinity-smoke.md). Epic-level: needs epics 1–4.

## PR size note

Sized L: request builder + stream decoder + dual-map correlation are one unreviewable-apart unit. Fixtures/goldens excluded from the budget; if hand-written code approaches ~1000 lines, split.

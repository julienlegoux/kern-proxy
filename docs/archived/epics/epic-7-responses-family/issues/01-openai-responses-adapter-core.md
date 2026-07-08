---
type: Issue
title: "Add the openai-responses adapter core"
description: "Responses API request/stream core plus the shared code the Azure and Codex variants build on."
tags: [epic-7]
timestamp: 2026-07-08T00:00:00Z
epic: 7
issue: 01
slug: openai-responses-adapter-core
size: L
status: done
gh_issue: 28
gh_pr: 58
resource: https://github.com/julienlegoux/kern-proxy/issues/28
depends_on: []
blocked_by: ["#18", "#19"]
---

# Add the openai-responses adapter core

## Summary

Port the shared openai-responses adapter — the base of the Responses family ([Epic 7](/epic-7-responses-family/EPIC_7.md)): Responses API request construction and SSE stream decoding to unified events, structured so the Azure and Codex variants ([Issue 02](./02-azure-responses-variant.md), [Issue 03](./03-codex-adapter.md)) are thin layers over shared code.

## Scope

- New `ai/apis/openairesponses` package: request building (input items, tools, reasoning config) and streaming decode over raw `net/http` + own SSE.
- Shared code factored for reuse by the Azure and Codex variants (base URLs, headers, request shaping hooks).
- TDD gate: port the base upstream responses tests first; goldens via `OnPayload` match TS-captured goldens.
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- Azure variant — [Issue 02](./02-azure-responses-variant.md).
- Codex adapter and its WebSocket transport — [Issue 03](./03-codex-adapter.md) / [Issue 04](./04-codex-websocket-transport.md).

## Acceptance criteria / Definition of done

- Base responses test ports pass against httptest fixtures with matching goldens.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `ai/apis/openairesponses/` — path follows the plan's layout; no adapter code exists yet.
- Read-only context: `ai/internal/sse`, `ai/events.go`, `ai/stream.go`, `ai/providers/faux` (event-contract reference).

## Dependencies

Blocks [Issue 02](./02-azure-responses-variant.md), [Issue 03](./03-codex-adapter.md). Epic-level: needs epics 1–4; shares protocol ground with [Epic 6](/epic-6-openai-completions/EPIC_6.md).

## PR size note

Sized L: the request builder + stream decoder + shared-variant scaffolding form one unit. Fixtures/goldens excluded from the budget; if hand-written code approaches ~1000 lines, split.

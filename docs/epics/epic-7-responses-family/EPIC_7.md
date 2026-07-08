---
type: Epic
title: "Responses family (OpenAI responses, Azure, Codex)"
description: "openai-responses adapter plus Azure variant and the Codex adapter with WebSocket transport, zstd SSE fallback, and JWT accountId."
tags: [epic]
timestamp: 2026-07-08T07:28:37Z
epic: 7
slug: responses-family
status: done
gh_issue: 8
milestone: 7
resource: https://github.com/julienlegoux/kern-proxy/issues/8
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 7: Responses family (OpenAI responses, Azure, Codex)

## Goal

Port the OpenAI Responses API family: the shared openai-responses adapter, its Azure variant, and the Codex adapter — the plan's highest-fidelity-risk transport.

## Scope

- `ai/apis/openairesponses` (+ shared code), `azure/`, `codex/`.
- Codex transport (verified against upstream): **WebSocket transport** (`coder/websocket`) **with SSE fallback that requires zstd Content-Encoding decoding** (`klauspost/compress/zstd`), per-session fallback memory, connection-limit retry, JWT accountId extraction.
- Codex OAuth credential consumption (PKCE :1455 / device flows themselves are Epic 12).

## Out of scope

- The Codex OAuth login flows — [Epic 12: OAuth flows](/epic-12-oauth-flows/EPIC_12.md).

## Acceptance criteria

- Upstream responses/codex test ports pass with golden-request snapshots.
- WebSocket integration test against a local ws server passes.

## Dependencies

- [Epic 1](/epic-1-foundation/EPIC_1.md), [Epic 2](/epic-2-faux-provider-registry/EPIC_2.md), [Epic 3](/epic-3-auth-core/EPIC_3.md), [Epic 4](/epic-4-transform-validation/EPIC_4.md). Shares protocol ground with [Epic 6](/epic-6-openai-completions/EPIC_6.md).

## Notes

- Size: L–XL — the largest single epic. Part of the 45%-effort adapter trio (epics 5–7).
- **Key risk #2 from the plan:** Codex transport (WS + zstd SSE fallback + per-session fallback memory) is the highest-fidelity-risk adapter. Note `Transport`/`WebsocketConnectTimeout` are in base options (verified), so no per-api generics are needed.
- Project-wide: TDD gate; `// Ports:` headers; PORTING.md mapping discipline.

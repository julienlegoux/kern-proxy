---
type: Issue
title: "Add the Codex WebSocket transport with zstd SSE fallback"
description: "WebSocket transport (coder/websocket), SSE fallback requiring zstd Content-Encoding decode, per-session fallback memory, and connection-limit retry."
tags: [epic-7]
timestamp: 2026-07-08T07:28:37Z
epic: 7
issue: 04
slug: codex-websocket-transport
size: L
status: done
gh_issue: 31
gh_pr: 61
resource: https://github.com/julienlegoux/kern-proxy/issues/31
depends_on: [03]
---

# Add the Codex WebSocket transport with zstd SSE fallback

## Summary

Port the Codex transport — **key risk #2 from the plan**, the highest-fidelity-risk adapter piece: a WebSocket transport (`coder/websocket`) with an SSE fallback that requires zstd Content-Encoding decoding (`klauspost/compress/zstd`), per-session fallback memory, and connection-limit retry. Builds on [Issue 03](./03-codex-adapter.md)'s request/auth layer.

## Scope

- WebSocket transport via `coder/websocket`; note `Transport`/`WebsocketConnectTimeout` already live in base options (verified against upstream) — no per-api generics needed.
- SSE fallback path decoding zstd `Content-Encoding` via `klauspost/compress/zstd`.
- Per-session fallback memory (once a session falls back to SSE it stays there).
- Connection-limit retry behavior.
- TDD gate: port the upstream WS/fallback Codex tests first; add a WebSocket integration test against a local ws server (the epic's second acceptance criterion).
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- Codex request shaping/auth — landed in [Issue 03](./03-codex-adapter.md).
- OAuth login flows — [Epic 12](/epic-12-oauth-flows/EPIC_12.md).

## Acceptance criteria / Definition of done

- WS/fallback test ports pass; the local-ws-server integration test passes.
- Fallback memory verified: after an SSE fallback, subsequent requests in the session skip the WS attempt.
- zstd-encoded SSE fixture decodes correctly.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- `ai/apis/openairesponses/codex/` (from Issue 03); new deps `coder/websocket`, `klauspost/compress/zstd` in `go.mod`.

## Dependencies

Blocked by [Issue 03](./03-codex-adapter.md).

## PR size note

Sized L: the transport, fallback state machine, and integration test are one safety-critical unit (plan risk #2) that would be riskier reviewed apart. If hand-written code approaches ~1000 lines, split the integration-test harness into a preparatory PR.

---
type: Issue
title: "Add the Codex adapter over the responses protocol"
description: "Codex request/auth layer: OAuth credential consumption, JWT accountId extraction, and Codex-specific request shaping over the shared responses core (HTTP/SSE path only)."
tags: [epic-7]
timestamp: 2026-07-08T07:30:00Z
epic: 7
issue: 03
slug: codex-adapter
size: M
status: pr-open
gh_issue: 30
gh_pr: 60
resource: https://github.com/julienlegoux/kern-proxy/issues/30
depends_on: [01]
---

# Add the Codex adapter over the responses protocol

## Summary

Port the Codex adapter's request/auth layer over [Issue 01](./01-openai-responses-adapter-core.md)'s shared responses core, on the plain HTTP/SSE path: OAuth credential consumption, **JWT accountId extraction**, and Codex-specific request shaping. The WebSocket transport and zstd SSE fallback land separately in [Issue 04](./04-codex-websocket-transport.md) — the plan's highest-fidelity-risk piece deserves its own review.

## Scope

- `ai/apis/openairesponses/codex`: Codex request shaping and headers over the shared core.
- OAuth credential consumption (stored credential via Epic 3's resolution; note Codex is the exception to the 5-minute `expires` margin).
- JWT accountId extraction from the access token.
- TDD gate: port the upstream Codex tests that don't require the WS transport first, goldens via `OnPayload`.
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- WebSocket transport, zstd SSE fallback, per-session fallback memory, connection-limit retry — [Issue 04](./04-codex-websocket-transport.md).
- The Codex OAuth login flows (PKCE :1455 / device) — [Epic 12](/epic-12-oauth-flows/EPIC_12.md).

## Acceptance criteria / Definition of done

- Codex HTTP-path test ports pass; accountId extraction covered against fixture JWTs.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `ai/apis/openairesponses/codex/`; shared core from Issue 01; `ai/auth.go` / `ai/resolve.go` (credential consumption, read-only).

## Dependencies

Blocked by [Issue 01](./01-openai-responses-adapter-core.md). Blocks [Issue 04](./04-codex-websocket-transport.md).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.

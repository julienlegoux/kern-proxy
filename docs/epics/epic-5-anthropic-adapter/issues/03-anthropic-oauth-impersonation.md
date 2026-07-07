---
type: Issue
title: "Add OAuth Claude Code impersonation mode to the Anthropic adapter"
description: "Identity system block, beta headers, claude-cli user-agent, and tool-name remap when running on an OAuth credential."
tags: [epic-5]
timestamp: 2026-07-08T02:00:00Z
epic: 5
issue: 03
slug: anthropic-oauth-impersonation
size: M
status: pr-open
gh_issue: 22
gh_pr: 52
resource: https://github.com/julienlegoux/kern-proxy/issues/22
depends_on: [01]
---

# Add OAuth Claude Code impersonation mode to the Anthropic adapter

## Summary

When the resolved credential is OAuth (Claude Pro/Max), the upstream adapter impersonates Claude Code on the wire. Port that mode faithfully on top of [Issue 01](./01-anthropic-adapter-core.md)'s core: identity system block, beta headers, `user-agent: claude-cli/…`, and the tool-name remap.

## Scope

- OAuth request mode: inject the identity system block; set the required beta headers and `user-agent: claude-cli/…`; remap tool names per upstream.
- Credential consumption only — the resolved OAuth credential comes from Epic 3's auth core (`ai/resolve.go`); the login flow itself is Epic 12.
- TDD gate: port the upstream impersonation `anthropic-*` tests first; goldens must show the exact headers/system block.

## Out of scope

- The Anthropic PKCE login flow — [Epic 12](/epic-12-oauth-flows/EPIC_12.md).
- Thinking/cache mechanics — [Issue 02](./02-anthropic-thinking-cache-control.md).

## Acceptance criteria / Definition of done

- Impersonation test ports pass; golden requests match TS goldens for headers, user-agent, system block, and remapped tool names.
- API-key requests are untouched (mode only activates on OAuth credentials).
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- `ai/apis/anthropic/` (from Issue 01), `ai/auth.go` / `ai/resolve.go` (OAuth credential + resolution, read-only), `ai/headers.go`.

## Dependencies

Blocked by [Issue 01](./01-anthropic-adapter-core.md). The credential this consumes is produced by [Epic 12](/epic-12-oauth-flows/EPIC_12.md)'s login flow, but tests use stored fixtures so there is no build-order dependency.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.

---
type: Issue
title: "Add the Codex dual login flow"
description: "Codex OAuth: PKCE on callback port :1455 plus the device-code alternative, with Codex's no-margin expires semantics."
tags: [epic-12]
timestamp: 2026-07-08T19:15:00Z
epic: 12
issue: 03
slug: codex-dual-flow
size: M
status: in-progress
gh_issue: 42
resource: https://github.com/julienlegoux/kern-proxy/issues/42
depends_on: [01]
---

# Add the Codex dual login flow

## Summary

Port the Codex login flow over the shared scaffolding from [Issue 01](./01-anthropic-pkce-flow.md): a **dual flow** — PKCE on callback port `:1455` or device code, per upstream's selection logic. Codex is the documented exception to the 5-minute `expires` margin, and its credential carries `accountId` in `Extra` for the adapter's JWT-derived account routing (Epic 7).

## Scope

- Codex PKCE flow on `:1455` and the device-code alternative, with upstream's flow selection.
- Token exchange + refresh; `expires` **without** the 5-minute margin (the Codex exception).
- `accountId` (and other Codex-specific fields) preserved in `OAuthCredential.Extra`.
- TDD gate: port the upstream codex oauth tests first (httptest endpoints).
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- Anthropic/Copilot flows — [Issue 01](./01-anthropic-pkce-flow.md) / [Issue 02](./02-copilot-device-code-flow.md).
- The Codex adapter's consumption of the credential — [Epic 7](/epic-7-responses-family/EPIC_7.md).

## Acceptance criteria / Definition of done

- Codex oauth test ports pass for both flow variants; `expires` asserted margin-free; `Extra` round-trips `accountId`.
- Manual live login check performed once and noted in the PR.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- `ai/auth/oauth/` (scaffolding from Issue 01); `ai/auth.go` (`OAuthCredential.Extra`, read-only).

## Dependencies

Blocked by [Issue 01](./01-anthropic-pkce-flow.md) (shared scaffolding). Closes the epic together with issues 01–02.

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.

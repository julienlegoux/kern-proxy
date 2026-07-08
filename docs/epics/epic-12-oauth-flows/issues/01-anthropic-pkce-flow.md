---
type: Issue
title: "Add the Anthropic PKCE login flow"
description: "ai/auth/oauth scaffolding plus the Anthropic PKCE flow on callback port :53692 with the callback vs manual-code race handled."
tags: [epic-12]
timestamp: 2026-07-08T17:15:00Z
epic: 12
issue: 01
slug: anthropic-pkce-flow
size: M
status: in-progress
gh_issue: 40
resource: https://github.com/julienlegoux/kern-proxy/issues/40
depends_on: []
blocked_by: ["#16"]
---

# Add the Anthropic PKCE login flow

## Summary

Port the first interactive OAuth login flow of [Epic 12](/epic-12-oauth-flows/EPIC_12.md) and the shared `ai/auth/oauth` scaffolding it establishes: Anthropic PKCE on callback port `:53692`, including the **callback vs manual-code race** — the user can either complete the browser callback or paste a code manually, and whichever resolves first wins (the pending `AuthPrompt` is cancelled via its `Ctx`).

## Scope

- `ai/auth/oauth` package scaffolding shared by all three flows: PKCE helpers, local callback server, token exchange plumbing (ports upstream `src/utils/oauth/*`).
- Anthropic flow: PKCE authorize URL, callback server on `:53692`, manual-code fallback prompt, the race between them, token exchange, and refresh.
- `expires` stored as epoch-ms **minus the 5-minute margin**.
- Credentials persist through the `CredentialStore` (`AuthLoginCallbacks` / `OAuthAuth.Login` contracts in `ai/auth.go` are already defined — implement against them).
- TDD gate: port the upstream anthropic oauth tests first (httptest token endpoints; no live browser in CI).
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- Copilot and Codex flows — [Issue 02](./02-copilot-device-code-flow.md) / [Issue 03](./03-codex-dual-flow.md).
- Credential storage itself — Epic 3 (the flock file store is [its issue #16](/epic-3-auth-core/issues/01-flock-locked-file-credential-store.md)).
- The adapter's impersonation mode (consumes this credential) — Epic 5; CLI `login` wiring — Epic 14.

## Acceptance criteria / Definition of done

- Anthropic oauth test ports pass, including a test where the manual-code prompt resolves while the callback server is pending (and vice versa).
- Manual live login check performed once and noted in the PR.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- New: `ai/auth/oauth/` — path per the plan; see Epic 3's layout note if `ai/auth` ends up elsewhere.
- Read-only context: `ai/auth.go` (`OAuthAuth`, `AuthLoginCallbacks`, `AuthPrompt` with `Ctx`), `ai/credentialstore.go`.

## Dependencies

Blocks [Issue 02](./02-copilot-device-code-flow.md) and [Issue 03](./03-codex-dual-flow.md) (they reuse the scaffolding). Cross-epic: persisted logins need Epic 3's file store (#16).

## PR size note

Target ~500 changed lines; if the shared scaffolding pushes this past ~1000, land the scaffolding as its own preparatory PR.

---
type: Issue
title: "Add the Copilot device-code login flow"
description: "GitHub Copilot device code + token exchange with per-credential baseUrl (enterpriseUrl) support."
tags: [epic-12]
timestamp: 2026-07-08T18:30:00Z
epic: 12
issue: 02
slug: copilot-device-code-flow
size: M
status: pr-open
gh_issue: 41
gh_pr: 71
resource: https://github.com/julienlegoux/kern-proxy/issues/41
depends_on: [01]
---

# Add the Copilot device-code login flow

## Summary

Port the GitHub Copilot login flow over the shared scaffolding from [Issue 01](./01-anthropic-pkce-flow.md): device-code grant, the GitHub-token → Copilot-token exchange, and **per-credential baseUrl** (enterprise URLs ride in the credential's `Extra`, and `OAuthAuth.ToAuth` derives request auth including baseUrl from whatever is stored).

## Scope

- Copilot flow: device-code initiation (`AuthEventDeviceCode` with user code/verification URI/interval), polling, token exchange, refresh.
- Per-credential baseUrl: enterpriseUrl captured at login, surfaced via `ToAuth` (`ModelAuth.BaseURL`).
- `expires` epoch-ms minus the 5-minute margin.
- TDD gate: port the upstream `oauth-device-code` and copilot oauth tests first (httptest endpoints).
- `// Ports:` headers; flip the matching PORTING.md rows.

## Out of scope

- Anthropic/Codex flows — [Issue 01](./01-anthropic-pkce-flow.md) / [Issue 03](./03-codex-dual-flow.md).
- The Copilot provider binding and policy `RefreshModels` — [Epic 11](/epic-11-catalog-all-providers/EPIC_11.md).

## Acceptance criteria / Definition of done

- `oauth-device-code` and copilot oauth test ports pass, including an enterpriseUrl case where `ToAuth` yields the per-credential baseUrl.
- Manual live login check performed once and noted in the PR.
- `go test ./...` green; `// Ports:` headers present; PORTING.md updated.

## Relevant files / areas

- `ai/auth/oauth/` (scaffolding from Issue 01); `ai/auth.go` (`OAuthCredential.Extra`, `AuthEvent` device-code fields, read-only).

## Dependencies

Blocked by [Issue 01](./01-anthropic-pkce-flow.md) (shared scaffolding).

## PR size note

Target ~500 changed lines; if this grows past ~1000, split it before opening the PR.

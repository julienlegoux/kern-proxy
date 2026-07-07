---
type: Epic
title: "OAuth flows"
description: "Interactive OAuth login flows: Anthropic PKCE with callback/manual-code race, Copilot device code + token exchange, Codex dual-flow."
tags: [epic]
timestamp: 2026-07-07T05:34:54Z
epic: 12
slug: oauth-flows
status: open
gh_issue: 13
milestone: 12
resource: https://github.com/julienlegoux/kern-proxy/issues/13
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 12: OAuth flows

## Goal

Port the three interactive OAuth login flows so users can authenticate without API keys — feeding credentials into the auth core's stores for the adapters (and the CLI's `login` command in Epic 14) to consume.

## Scope

- `ai/auth/oauth` (or equivalent under the actual auth layout — see Epic 3's layout note):
  - **Anthropic**: PKCE on callback port :53692, with the callback vs manual-code race handled.
  - **Copilot**: device code + token exchange + per-credential baseUrl.
  - **Codex**: dual-flow — PKCE on :1455 / device code.
- Token refresh semantics respected: `expires` = epoch-ms minus 5-min margin (except Codex).

## Out of scope

- Credential storage/resolution — [Epic 3: Auth core](/epic-3-auth-core/EPIC_3.md).
- The CLI `login` command wiring — [Epic 14](/epic-14-cli-sync-tooling/EPIC_14.md).

## Acceptance criteria

- Ports of upstream `oauth-device-code` and copilot/codex oauth tests pass.
- Manual live login check performed per flow.

## Dependencies

- [Epic 3: Auth core](/epic-3-auth-core/EPIC_3.md) — including its remaining flock file store work, which this epic's persisted logins need. Blocks [Epic 14](/epic-14-cli-sync-tooling/EPIC_14.md)'s `login` command.

## Notes

- Size: M.
- The Anthropic OAuth credential also drives the adapter's Claude Code impersonation mode ([Epic 5](/epic-5-anthropic-adapter/EPIC_5.md)).
- Project-wide: TDD gate; `// Ports:` headers; PORTING.md mapping discipline.

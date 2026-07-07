---
type: Epic
title: "Auth core"
description: "Credential types, in-memory and flock-locked file stores, resolve precedence, and env-key mapping with ambient sentinels."
tags: [epic]
timestamp: 2026-07-07T05:34:54Z
epic: 3
slug: auth-core
status: open
gh_issue: 4
milestone: 3
resource: https://github.com/julienlegoux/kern-proxy/issues/4
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 3: Auth core

## Goal

Provide the credential storage and resolution layer every provider adapter authenticates through: credential types, pluggable stores (in-memory and on-disk), a strict resolution precedence, and the env-var key map with ambient sentinels.

## Scope

- Credential types.
- `InMemoryCredentialStore` and a flock-locked file store at `~/.pi/agent/auth.json` (file mode 0600, dir 0700).
- Resolve precedence: explicit override → stored credential (OAuth refresh with **double-checked locking under `CredentialStore.Modify`**) → ambient env only when nothing stored.
- Env-key map + ambient sentinels for Vertex ADC and the Bedrock AWS auth matrix.
- `expires` semantics: epoch-ms minus a 5-minute margin (except Codex).

### Already completed (delivered early in commit `61ce7d6`, see Epic 2)

- Credential types (`ai/auth.go`), `InMemoryCredentialStore` (`ai/credentialstore.go`), resolve precedence (`ai/resolve.go` + tests), env-key map, auth context.

### Remaining

- The flock-locked file store (`~/.pi/agent/auth.json`, 0600/0700 + flock).
- Verify/complete double-checked locking on OAuth refresh under `CredentialStore.Modify`.
- Concurrent-refresh race test under `-race`.

## Out of scope

- The interactive OAuth login flows themselves (PKCE, device code) — [Epic 12: OAuth flows](/epic-12-oauth-flows/EPIC_12.md).

## Acceptance criteria

- Ports of upstream `oauth-auth` and `env-api-keys` tests pass.
- Concurrent-refresh race test passes under `go test -race`.

## Dependencies

- [Epic 1: Foundation](/epic-1-foundation/EPIC_1.md). Blocks [Epic 12: OAuth flows](/epic-12-oauth-flows/EPIC_12.md) (login flows persist through the file store) and all real provider adapters (epics 5–10) that resolve auth.

## Notes

- **Partially done** — the plan was started before this epic split; the bulk of this epic shipped inside the Phase 2 commit. Only the file store and race-test work remains, so issues created from this epic should cover just the "Remaining" list.
- **Layout deviation:** the plan called for an `ai/auth/` package; the implementation put auth files in the `ai` root (`auth.go`, `authcontext.go`, `credentialstore.go`, `resolve.go`). Keep that layout unless deliberately refactored.
- Project-wide: TDD gate (port upstream tests first); `// Ports:` headers; PORTING.md mapping discipline.

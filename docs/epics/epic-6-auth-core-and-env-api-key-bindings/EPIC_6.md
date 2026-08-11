---
type: Epic
title: "Auth core and env-API-key bindings"
description: "Port the content of upstream's restructured auth tree without moving any Go package, refresh PORTING.md's upstream paths, and add four env-API-key provider bindings."
tags: [epic]
timestamp: 2026-08-11T13:05:00Z
epic: 6
slug: auth-core-and-env-api-key-bindings
status: open
gh_issue: 123
milestone: 23
resource: https://github.com/kern-ia/kern-link/issues/123
source: docs/planning/SCOPE.md#milestone-6-auth-core-and-env-api-key-bindings
---

# Epic 6: Auth core and env-API-key bindings

## Goal

Upstream reorganised its auth tree between v0.80.3 and v0.84.1. The content
changes have to land; the *moves* mostly do not, because kern-link's packages
already sit where upstream moved to. Getting that distinction right is what
keeps the next sync navigable.

## Scope

- Port the content of `auth/credential-store.ts`, `auth/helpers.ts`,
  `auth/resolve.ts`, `auth/types.ts`, `env-api-keys.ts`, `oauth.ts`.
- Port the content of `src/models.ts`'s auth surface —
  `Provider.filterModels`, `Models.checkAuth`, `Models.getAvailable`,
  `Models.login`, `Models.logout`, and the provider-id `GetAuth` overload —
  that [Epic 2 issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
  explicitly deferred here (`:96-106`). Built by issues 05 and 06.
- Port the content edits `src/auth/oauth/anthropic.ts`,
  `src/auth/oauth/openai-codex.ts` and `src/auth/oauth/github-copilot.ts`
  carry at their new upstream location: the always-racing manual-code prompt
  and the Copilot policy-state model fallback. Built by issues 08 and 09. The
  *directory move* upstream made is a no-op for this port (see `## Out of
  scope`); the *content* upstream changed inside the moved files is not.
- **Update `docs/PORTING.md`'s upstream paths.** Upstream moved
  `src/utils/oauth/*` to `src/auth/oauth/*`. The mapping table is what the whole
  sync procedure navigates by; leaving it pointing at the deleted
  `src/utils/oauth/*` would break the next sync's ability to resolve `sync.sh`
  output against Go packages.
- Four new env-API-key bindings, each an `envApiKeyAuth` one-liner plus its
  catalog entry: `baseten`, `qwen-token-plan`, `qwen-token-plan-cn`,
  `qwen-token-plan-individual`.

## Out of scope

- **No Go package *moves*.** `ai/auth/oauth` already sits at upstream's new
  destination — a call the port made independently and earlier. Mirroring the
  *directory move* would be churn with no content behind it. The *content*
  upstream edited inside the moved files (issues 08, 09) is in scope, per
  `## Scope` above.
- The four new OAuth flows — epic 7.
- The radius provider binding, which needs its OAuth flow first — epic 8.

## Acceptance criteria

1. The content of all six upstream auth files is ported, matching `936aff00`.
2. No Go package under `ai/auth/` has been moved or renamed.
3. `docs/PORTING.md`'s mapping table points at upstream's **current** paths
   (`src/auth/oauth/*`), with no dangling `src/utils/oauth/*` entries.
4. The four env-API-key bindings resolve credentials from their environment
   variables, and each has a catalog entry.
5. Ported upstream tests come across; new coverage is offline and stdlib-only.
6. CI green: `go test ./... -race -v`, `bash upstream/sync_test.sh`,
   `golangci-lint` v2.12.2.
7. The Anthropic and Codex OAuth login flows always race the manual-code
   prompt against the callback server — no optional fallback path remains
   (issue 08).
8. The `Models` auth surface [Epic 2 issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
   deferred here — `filterModels`, `checkAuth`, `getAvailable`, `login`,
   `logout`, and the provider-id `GetAuth` overload — is complete (issues 05,
   06).

## Dependencies

- [Epic 2: Core types and Models contracts](/epic-2-core-types-and-models-contracts/EPIC_2.md)

Blocks [Epic 7: Four new OAuth flows](/epic-7-four-new-oauth-flows/EPIC_7.md).

## Context

- [Technical specs](../../planning/SPECS.md)
- [Conventions](../../planning/CONVENTIONS.md)
- [Upstream sync scope](../../planning/SCOPE.md)
- [Decision 08 — auth restructure](../../planning/scope/08-auth-restructure.md)
- [Decision 10 — new provider bindings](../../planning/scope/10-new-provider-bindings.md)

## Notes

- Project-wide, not this epic's own boundary: no new direct dependencies, no
  logger, tests offline and stdlib-only, `// Ports:` headers on ported files and
  none on original code — absence is meaningful.
- The governing principle: a deviation must be justified by structural
  non-portability, never by cost.
- Upstream target frozen at `936aff00`.
- **Amendment (Epic 0 issue 09, 2026-08-11).** `## Scope` and `## Acceptance
  criteria` originally named only the six auth files from the epic's own
  goal, but four of the epic's eleven issues always built more than that:
  issues 05 and 06 complete the `Models` auth surface
  [Epic 2 issue 08](/epic-2-core-types-and-models-contracts/issues/08-models-refresh-contract.md)
  deferred here, and issues 08 and 09 carry real content edits from
  `src/auth/oauth/anthropic.ts`, `openai-codex.ts` and `github-copilot.ts`'s
  move — not the no-op directory move itself. This amendment records the
  hand-off and the work that was always there; it does not change what any
  issue does.

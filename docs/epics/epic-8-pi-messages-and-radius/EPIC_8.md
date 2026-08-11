---
type: Epic
title: "pi-messages and radius"
description: "Add pi's own protocol as the tenth ai/apis package and land the radius provider binding, whose gateway config is fetched over the network at provider setup."
tags: [epic]
timestamp: 2026-08-10T14:00:00Z
epic: 8
slug: pi-messages-and-radius
status: open
gh_issue: 125
milestone: 25
resource: https://github.com/kern-ia/kern-link/issues/125
source: docs/planning/SCOPE.md#milestone-8-pi-messages-and-radius
---

# Epic 8: pi-messages and radius

## Goal

Two new surfaces rather than updates to existing ones: pi's own wire protocol,
and a provider binding whose setup does something none of the existing 35 do.
They are grouped because radius is the one provider that needs both.

## Scope

- **`api/pi-messages.ts` (433 lines + 248 test lines)** becomes the **tenth**
  package under `ai/apis/`, with its `ai.Api` constant and registration. It is
  pi's own protocol — a POST of `{model, context, options}` to
  `<baseUrl>/messages`, SSE back — so it maps almost one-to-one onto types
  kern-link already has.
- **`providers/radius.ts` (82 lines) and `radius-config.ts` (96 lines).** Radius
  fetches its gateway config at provider setup, a network dependency none of the
  existing 35 bindings has; **that is the design point to get right**, not a
  detail to discover during implementation.

## Out of scope

- The radius OAuth flow itself — epic 7.
- The other three new env-API-key bindings — epic 6.
- Compatibility shims for the v0.2.0 breaks.

## Acceptance criteria

1. `ai/apis/pimessages` (or the chosen package name) exists as the tenth
   adapter, with its `ai.Api` constant and registration, matching upstream at
   `936aff00`.
2. Upstream's 248 lines of pi-messages tests are ported and pass offline.
3. The radius provider binding resolves, and its gateway-config fetch is
   deliberately designed — the network call happens inside the `RefreshModels`
   pipeline (behind `AllowNetwork`), not at provider construction, and is
   explicit in the code and covered by an offline `httptest` case, including
   its failure path.
4. `// Ports:` headers on every ported file; each dispositioned in
   `docs/PORTING.md`.
5. CI green: `go test ./... -race -v`, `bash upstream/sync_test.sh`,
   `golangci-lint` v2.12.2.

## Dependencies

- [Epic 7: Four new OAuth flows](/epic-7-four-new-oauth-flows/EPIC_7.md) — the
  radius OAuth flow.
- [Epic 2: Core types and Models contracts](/epic-2-core-types-and-models-contracts/EPIC_2.md)

## Context

- [Technical specs](../../planning/SPECS.md)
- [Conventions](../../planning/CONVENTIONS.md)
- [Upstream sync scope](../../planning/SCOPE.md)
- [Decision 13 — pi-messages adapter](../../planning/scope/13-pi-messages-adapter.md)
- [Decision 10 — new provider bindings](../../planning/scope/10-new-provider-bindings.md)

## Notes

- Risk owned here: **radius's runtime gateway-config fetch** is a network
  dependency at provider setup that no existing binding has. The mitigation is
  to design for it explicitly rather than discover it.
- pi-messages is another decision the governing principle **reversed** during
  scoping: it was provisionally deferred on cost, and cost is not admissible
  grounds for a deviation.
- Project-wide, not this epic's own boundary: no new direct dependencies (raw
  `net/http` + `ai/internal/sse`), no logger, tests offline and stdlib-only.
- Upstream target frozen at `936aff00`.
- **AC 3's "gateway-config fetch" wording** was amended by
  [Epic 0 issue 11](/epic-0-plan-remediation/issues/11-repair-the-epic-8-issue-set.md)
  to say the network call happens inside `RefreshModels` (behind
  `AllowNetwork`), not at provider construction — issues 03 and 04 already
  build it that way; the epic file just never said so, and a reviewer checking
  AC 3 against 04's tests could otherwise read a met criterion as missed.

---
type: Epic
title: "Faux provider & registry"
description: "The faux provider as executable spec of the event contract, plus the provider/models registry, lazy stream, and simple options."
tags: [epic]
timestamp: 2026-07-07T05:34:54Z
epic: 2
slug: faux-provider-registry
status: done
gh_issue: 3
milestone: 2
resource: https://github.com/julienlegoux/kern-proxy/issues/3
source: docs/PLAN.md#phases-dependency-ordered-tdd-gates
---

# Epic 2: Faux provider & registry

## Goal

Freeze the event contract with an executable specification (the faux provider) and stand up the machinery that composes providers: the `Provider`/`Models` registry (`createProvider`/`createModels`), the lazy-stream pattern, and simple streaming options.

## Scope

- `providers/faux`: the faux provider — the executable spec of the event contract, enabling the whole test suite to run with no network.
- Provider/Models registry (`createProvider`/`createModels`).
- `lazy.go`: `Stream()` returns synchronously; auth resolution/setup runs in a goroutine; setup failure becomes an in-band `error` event with a zero-usage AssistantMessage.
- `simpleopts.go`: `SimpleStreamOptions` embedding `StreamOptions`.

## Out of scope

- Real provider adapters (epics 5–10) and the ~35 thin provider bindings (epic 11).

## Acceptance criteria

- Ports of upstream `faux-provider`, `stream`, and `abort` tests pass — event contract frozen.
- `go test ./... -race` green with no network.

## Dependencies

- [Epic 1: Foundation](/epic-1-foundation/EPIC_1.md) — domain types, events, stream.

## Notes

- **Status: DONE.** Delivered in commit `61ce7d6` ("Phase 2: provider registry, auth resolution core, lazy stream, faux provider"). This epic exists for the record; its GitHub issue is created closed.
- That commit also delivered **auth resolution core ahead of schedule** — part of [Epic 3: Auth core](/epic-3-auth-core/EPIC_3.md)'s scope (credential types, in-memory store, resolve precedence, env-key map). Epic 3 records what remains.
- Project-wide: TDD gate (port upstream tests first, then implement, then goldens); `// Ports:` headers; PORTING.md mapping discipline.

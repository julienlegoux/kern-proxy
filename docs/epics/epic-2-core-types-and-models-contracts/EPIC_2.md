---
type: Epic
title: "Core types and Models contracts"
description: "Port the types.ts and models.ts surface, the new core catalog modules, and deferred tools in full — the foundation every other file in the sync compiles against."
tags: [epic]
timestamp: 2026-08-09T03:55:05Z
epic: 2
slug: core-types-and-models-contracts
status: open
gh_issue: 119
milestone: 19
resource: https://github.com/kern-ia/kern-link/issues/119
source: docs/planning/SCOPE.md#milestone-2-core-types-and-models-contracts
---

# Epic 2: Core types and Models contracts

## Goal

`types.ts` (+303) and `models.ts` (+685) are the surface every adapter compiles
against. Upstream changed them structurally between v0.80.3 and v0.84.1, so no
adapter epic can land until the Go core matches. Getting this wrong once is
paid for in every subsequent epic.

## Scope

- **`types.ts` and `models.ts`**: the `ProviderRequestOptions<TModel>` refactor,
  tiered `ModelCost` / `ModelCostRates` / `ModelCostTier`, `StopReason` gaining
  `pending` / `deferred`, `ThinkingLevel` gaining `max`, `FetchFunction`,
  `SessionAffinityFormat`, `JsonValue`, `BedrockCompat`, the
  `ModelsPublication` / `RefreshModelsContext` / `ModelsRefreshOptions` /
  `ModelsRequestTransforms` contracts, and the removal of the `AuthModel`
  export.
- **New core modules** `model-catalog.ts` and `models-store.ts`. Their Go
  mapping is an implementer call, but `docs/PORTING.md` records the outcome
  either way.
- **`simple-options` and `lazy`** land here, not with the adapters — every
  adapter compiles against them.
- **Deferred tools in full**: `utils/deferred-tools.ts`, `DeferredHandle`, the
  deferred fetch/cancel options, and the ported test. The type half is already
  mandatory, so shipping the enum values without the behaviour would leave
  `StopReason` cases the library can never emit.

## Out of scope

- Adapter-side consumption of the new contracts — that is epics 4 and 5.
- Regenerating the model catalog JSON, which epic 3 owns. This epic ships the Go
  structs that *read* the catalog; the regenerated tree fails validation at load
  if it arrives first.
- Idiomatic-Go cleanups of the ported shapes. Upstream's shape outranks Go idiom
  in ported files.

## Acceptance criteria

1. Every type and contract listed in Scope exists in the Go core with the
   upstream shape, and the tree compiles.
2. **Deferred tools are proven end to end through the `faux` provider** — the
   new stop reasons (`pending`, `deferred`) are actually emitted and observed by
   a test, not merely declared.
3. **A manual grep sweep for `StopReason` switch sites has been performed and
   its result recorded.** `exhaustive` is deliberately absent from
   `.golangci.yml`, so neither compiler nor linter will flag a switch that
   silently stops being exhaustive. This criterion is easy to miss and is the
   mitigation for a named program risk.
4. `docs/PORTING.md` records the Go mapping chosen for `model-catalog.ts` and
   `models-store.ts`, whatever it is.
5. CI green: `go test ./... -race -v`, `bash upstream/sync_test.sh`,
   `golangci-lint` v2.12.2.

## Dependencies

- [Epic 1: Repository hygiene](/epic-1-repository-hygiene/EPIC_1.md) — the
  module path must move before this epic opens branches.

This epic blocks epics 3, 4, 5, 6 and 8.

## Context

- [Technical specs](../../planning/SPECS.md)
- [Conventions](../../planning/CONVENTIONS.md)
- [Upstream sync scope](../../planning/SCOPE.md)
- [Decision 04 — core types migration](../../planning/scope/04-core-types-migration.md)
- [Decision 11 — deferred tools](../../planning/scope/11-deferred-tools.md)
- [Decision 02 — parity bar](../../planning/scope/02-parity-bar.md)

## Notes

- The upstream target is **frozen at `936aff00`**
  (`936aff00918de1187f085f123c2812d8f2d67745`, measured 2026-08-09). This epic
  is defined against that revision and no other.
- Project-wide, not this epic's own boundary: **no new direct dependencies**,
  **no logger** (non-fatal problems attach as `AssistantMessageDiagnostic`),
  tests stay offline and standard-library-only (`testing` +
  `net/http/httptest`, no `testify`, no build tags, no `t.Parallel()`), and
  every ported file carries a `// Ports:` provenance header.
- The governing principle for the whole program: **a deviation must be
  justified by structural non-portability, never by cost.**
- Risk owned here: non-exhaustive `StopReason` switches produce no compiler or
  linter error. The grep sweep is the mitigation.

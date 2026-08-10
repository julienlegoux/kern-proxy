---
type: Epic
title: "OpenAI-family adapters"
description: "Sync the four adapters built on the shared OpenAI responses core — completions, responses, Azure and Codex — together with constrained sampling."
tags: [epic]
timestamp: 2026-08-10T09:20:00Z
epic: 4
slug: openai-family-adapters
status: open
gh_issue: 121
milestone: 21
resource: https://github.com/kern-ia/kern-link/issues/121
source: docs/planning/SCOPE.md#milestone-4-openai-family-adapters
---

# Epic 4: OpenAI-family adapters

## Goal

These four adapters share the `openai-responses-shared` core. They move together
or that shared layer gets ported twice, once per adapter, with two chances to
diverge from upstream.

## Scope

- `openai-completions.ts` (+396) → `ai/apis/openaicompletions`
- `openai-responses.ts` and `openai-responses-shared.ts` (+412) →
  `ai/apis/openairesponses` — the shared core beneath Azure and Codex, whose
  converters `docs/PORTING.md` records as exported for exactly that reason
- `azure-openai-responses.ts` → `ai/apis/azure`
- `openai-codex-responses.ts` (+220) → `ai/apis/codex`
- **Constrained sampling**: `api/constrained-sampling.ts` (+148),
  `GrammarFormat`, `GrammarVariants`, `ConstrainedSamplingConfig`. The formats
  are OpenAI-specific and the request builders are being rewritten here anyway.
  `ConstrainedSamplingConfig` lands on `ai.Tool`, **not** in `ai.StreamOptions`:
  upstream at `936aff00` declares
  `constrainedSampling?: false | ConstrainedSamplingConfig` on the `Tool`
  interface (`packages/ai/src/types.ts:506`, inside `:501-507`; the config type
  itself is `:492-500`), and `StreamOptions` (`:175-219`) carries no such field.
  The configuration is per **tool**, not per request, so no per-adapter-options
  deviation is needed for it at all. Issue 01 therefore changes package `ai`'s
  public `Tool` struct (`ai/types.go:243-247`) — core-type work landing inside an
  adapter epic, by the hand-off
  [Epic 2 issue 02](/epic-2-core-types-and-models-contracts/issues/02-message-model-deferred-fields.md)
  already recorded in its `## Out of scope`.

## Out of scope

- The non-OpenAI adapters — epic 5 owns those.
- The classifier audit of `ai/retry.go` and `ai/overflow.go`: these adapters emit
  the strings the classifiers match, so auditing before they have moved would
  audit text about to change. Epic 9 owns it.
- Idiomatic-Go cleanups of ported code.

## Acceptance criteria

1. All four adapters compile against the epic-2 core and match upstream at
   `936aff00` in behaviour.
2. The shared responses core is ported once and consumed by responses, Azure and
   Codex — not duplicated.
3. Constrained sampling is ported, with `ConstrainedSamplingConfig` reachable
   through `ai.Tool` — mirroring upstream's `Tool.constrainedSampling`
   (`packages/ai/src/types.ts:506` at `936aff00`) — and **not** through
   `ai.StreamOptions`.
4. Ported upstream tests come across with the code; new coverage is offline
   `httptest`, never live-gated only.
5. Every file touched carries a `// Ports:` provenance header, and its
   disposition is reflected in `docs/PORTING.md`.
6. CI green: `go test ./... -race -v`, `bash upstream/sync_test.sh`,
   `golangci-lint` v2.12.2.

## Dependencies

- [Epic 2: Core types and Models contracts](/epic-2-core-types-and-models-contracts/EPIC_2.md)
  — these adapters compile against the new core surface.

## Context

- [Technical specs](../../planning/SPECS.md)
- [Conventions](../../planning/CONVENTIONS.md)
- [Upstream sync scope](../../planning/SCOPE.md)
- [Decision 06 — adapter updates](../../planning/scope/06-adapter-updates.md)
- [Decision 12 — constrained sampling](../../planning/scope/12-constrained-sampling.md)

## Notes

- Project-wide, not this epic's own boundary: **no new direct dependencies**
  (raw `net/http` + `ai/internal/sse` everywhere), no logger, offline
  stdlib-only tests, error text stays upstream-verbatim because `ai/retry.go`
  and `ai/overflow.go` classify failures by matching it.
- The governing principle: **a deviation must be justified by structural
  non-portability, never by cost.** "Large" or "no consumer asked for it" are
  not admissible grounds.
- Upstream target frozen at `936aff00`.

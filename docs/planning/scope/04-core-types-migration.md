---
type: Decision
title: "Core types and Models-contract migration"
description: "Whether the breaking core-type and Models-registry changes are ported, and whether they land before everything else."
tags: [decision, scope]
timestamp: 2026-08-09T01:30:23Z
phase: scope
decision: 04
slug: core-types-migration
status: decided
verdict: "Port the core type and Models-registry surface in full, as the program's first milestone. Includes a manual grep sweep for non-exhaustive StopReason switches, since golangci-lint's exhaustive check is deliberately off."
decided_via: triage
depends_on: [release-and-breaking-strategy]
---

# Question

`packages/ai/src/types.ts` (+303) and `src/models.ts` (+685) are the foundation
every other file in the range builds on. Upstream also added two new core
modules with no Go counterpart: `src/model-catalog.ts` and `src/models-store.ts`.

In kern-link these map to `ai/types.go`, `ai/model.go`, `ai/options.go`,
`ai/events.go`, `ai/provider.go`, `ai/lazy.go`, `ai/cost.go` — per the mapping
table in `docs/PORTING.md`. Nothing in `ai/apis/*` or `ai/providers` compiles
against the new shapes until these land.

Is this in scope, and does it gate the rest?

# Options

- **Port in full, first** — the whole type and registry surface moves to 0.84.1
  before any adapter work starts.
- **Port the minimum the adapters need, defer the rest** — e.g. take
  `ThinkingLevel: "max"` and tiered cost but skip `ProviderRequestOptions` and
  the `Models*` refresh contracts. Smaller, but leaves the core permanently
  half-migrated and makes the next sync harder.
- **Out of scope** — not credible; without it nothing else in the range ports.

# Recommendation

**Port in full, and make it the program's first milestone.** It is a hard
dependency for adapters ([06](/scope/06-adapter-updates.md)), the catalog
([05](/scope/05-catalog-schema-and-tooling.md)), and both new capabilities
([11](/scope/11-deferred-tools.md), [12](/scope/12-constrained-sampling.md)) —
`StopReason`'s new `"pending"`/`"deferred"` values exist *for* deferred tools,
and `ConstrainedSamplingConfig` hangs off the options refactor.

Two Go-specific notes worth carrying into the epic:

- `StopReason` and `ThinkingLevel` are string-typed constants in `ai`; adding
  values is additive in Go but breaks any consumer `switch` that assumed
  exhaustiveness. `CONVENTIONS.md` records that `exhaustive` is deliberately not
  enabled in `.golangci.yml`, so the compiler will not find those sites — the
  epic must grep for them by hand.
- `model-catalog.ts` / `models-store.ts` may or may not deserve separate Go
  files; the existing port already distributes `models.ts` across
  `ai/provider.go` and `ai/lazy.go`, so the mapping is a judgment call for the
  implementer, and `docs/PORTING.md`'s table needs a row either way.

# Verdict

**Port in full, first milestone.** Accepted as recommended.

Two items carry into the epic as explicit acceptance criteria rather than
implementation detail:

- **A manual grep sweep for `StopReason` switch sites.** Adding `"pending"` and
  `"deferred"` is additive in Go, so neither the compiler nor the linter will
  flag a switch that silently stops being exhaustive — `exhaustive` is
  deliberately absent from `.golangci.yml`. This is risk 3 in
  [decision 21](/scope/21-risks-and-assumptions.md).
- **A `docs/PORTING.md` mapping row for `model-catalog.ts` and
  `models-store.ts`**, whichever Go files they land in. The existing port already
  distributes `models.ts` across `ai/provider.go` and `ai/lazy.go`, so the target
  is an implementer judgment call — but the table must record the outcome either
  way.

`simple-options` and `lazy.ts` belong to this milestone rather than the adapter
epics ([decision 06](/scope/06-adapter-updates.md)), because every adapter
compiles against them.
